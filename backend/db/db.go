package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// 纯 Go sqlite 驱动，与依赖 gotdx 共用同一实现，注册 database/sql 驱动名 "sqlite"。
	// 注意：不要同时引入 glebarez/go-sqlite 或 mattn/go-sqlite3（已用空壳包顶替），
	// 否则会重复注册驱动名导致启动 panic。
	_ "modernc.org/sqlite"
)

var Dao *gorm.DB

// dbMaxConns 连接池上限。VACUUM 前后需要按此值临时排空/恢复空闲连接。
const dbMaxConns = 8

// dbFilePath 实际使用的数据库文件路径，供 VACUUM 等维护操作定位文件
var dbFilePath string

// vacuumRunning 防止并发触发 VACUUM：VACUUM 全程独占写锁，并发执行只会相互等待并失败
var vacuumRunning atomic.Bool

// resolveDBPath 解析数据库文件路径（空值取默认路径），并去掉 DSN 查询参数只保留文件路径。
func resolveDBPath(path string) string {
	if path == "" {
		path = "data/stock.db"
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	return path
}

// txLockClause 显式事务的加锁模式。
// WAL 下 deferred（驱动默认）事务若"先读后写"，升级写锁时会立即返回 busy 而**不排队等待**，
// 造成偶发写失败；immediate 在 BEGIN 时就取写锁，把冲突交给 busy_timeout 排队。
// 项目现有事务均为"写在前"（先 DELETE 再批量写），此项属防御性配置，
// 也覆盖未来可能新增的"先查后写"事务。基准见 BenchmarkTxLock。
const txLockClause = "&_txlock=immediate"

// sqliteDSN 拼装 modernc.org/sqlite 风格 DSN。
// busy_timeout/cache_size/mmap_size 等均为连接级 PRAGMA，必须通过 DSN 下发才能对
// 连接池中每个新建连接生效（对单个连接 Exec 只影响那一个连接）。
func sqliteDSN(path string) string {
	if path == "" {
		path = "data/stock.db"
	}
	if strings.Contains(path, "_pragma=") {
		return path // 调用方已自带参数，尊重原样
	}
	return path + "?_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=cache_size(-131072)" + // 每连接页缓存上限 128MB（池扩容后按连接摊薄）
		"&_pragma=mmap_size(268435456)" + // 256MB 内存映射 IO，加速读且多连接共享 OS 页缓存
		"&_pragma=temp_store(MEMORY)" + // 临时表/排序走内存
		"&_pragma=wal_autocheckpoint(2000)" + // WAL 超过约 8MB 才自动 checkpoint，降低交易时段高频写的卡顿
		txLockClause
}

func Init(sqlitePath string) {
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second * 3,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			LogLevel:                  logger.Silent,
		},
	)
	dbFilePath = resolveDBPath(sqlitePath)
	openDb, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: sqliteDSN(sqlitePath)}), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}

	// 兜底：确保 busy_timeout / WAL / synchronous 生效（DSN 参数未生效时的保险）
	_ = openDb.Exec("PRAGMA busy_timeout=10000").Error
	_ = openDb.Exec("PRAGMA journal_mode=WAL").Error
	_ = openDb.Exec("PRAGMA synchronous=NORMAL").Error

	dbCon, err := openDb.DB()
	if err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	// WAL 模式下读不阻塞写、写不阻塞读，多个连接可并发读
	// （板块/概念资金流向页一次并发拉取 40+ 条曲线）；写者由 busy_timeout 串行等待。
	// 之前 MaxIdleConns=1 会导致并发高峰后连接被反复关闭重建，预编译语句缓存随之失效。
	// SQLite 写是单写者，连接数过高无益于写吞吐，反而加剧写锁竞争与每连接页缓存内存占用；
	// 8 个连接足够覆盖行情页并发读，索引生效后单条查询为毫秒级，8 连接可快速轮转。
	dbCon.SetMaxIdleConns(dbMaxConns)
	dbCon.SetMaxOpenConns(dbMaxConns)
	// SQLite 为嵌入式库，连接无服务端状态可刷新，不设生命周期可避免
	// 周期性换连接后 database/sql 内部按连接缓存的预编译语句全部作废重建
	dbCon.SetConnMaxLifetime(0)
	// 空闲超过 5 分钟回收连接：每个 idle 连接各自持有独立的页缓存（cache_size 上限 128MB/连接），
	// 长期空闲会空占内存；交易时段连接会持续复用，不受影响，非交易时段空闲连接自动释放。
	dbCon.SetConnMaxIdleTime(5 * time.Minute)
	Dao = openDb
	AutoMigrate()
	// 启动时异步清理过期缓存（保留最近 1 天），避免数据库无限增长
	go ClearExpiredStockTransactionCache()
}

// VacuumResult 数据库压缩结果，字节数便于前端自行换算展示
type VacuumResult struct {
	FilePath    string  `json:"filePath"`    // 数据库文件路径
	BeforeBytes int64   `json:"beforeBytes"` // 压缩前文件大小
	AfterBytes  int64   `json:"afterBytes"`  // 压缩后文件大小
	FreedBytes  int64   `json:"freedBytes"`  // 释放的空间
	DurationSec float64 `json:"durationSec"` // 本次耗时（秒）
}

// Vacuum 执行 SQLite VACUUM：重建数据库文件，回收 DELETE/清理后未归还操作系统的空闲页。
//
// 为什么需要：SQLite 的 DELETE 只把页面标记为空闲并放入空闲链表，文件大小不会缩小。
// 板块/概念资金流向、分笔成交等高频写入表长期清理后，文件会远大于实际数据量，
// 查询要扫描的页面数、磁盘 IO 与 WAL checkpoint 耗时都会成倍增加，进而拖慢其它查询。
//
// 调用方需注意：
//  1. VACUUM 全程独占写锁，期间所有写入排队等待，务必在非交易时段执行；
//  2. 需要约等于当前库大小的临时磁盘空间（内部会重建出一份完整副本），空间不足会失败；
//  3. 耗时与库大小成正比，数 GB 的库可能持续数分钟。
func Vacuum() (*VacuumResult, error) {
	if !vacuumRunning.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("数据库压缩正在进行中，请等待本次执行完成")
	}
	defer vacuumRunning.Store(false)

	start := time.Now()

	// 关键前置步骤：排空连接池中的空闲连接。
	// SQLite 只有在没有其它连接持读标记时，checkpoint 才能截断主库文件；
	// 池中空闲连接会持有读标记，导致出现"页数已被 VACUUM 压缩、但磁盘文件大小不变"的现象
	// （实测：不排空时 freed=0，排空后 18.9MB → 1.0MB）。
	if sqlDB, dbErr := Dao.DB(); dbErr == nil {
		sqlDB.SetMaxIdleConns(0)
		// 给在途查询留出归还连接的时间，否则该连接仍会挡住文件截断
		time.Sleep(300 * time.Millisecond)
		defer sqlDB.SetMaxIdleConns(dbMaxConns)
	}

	// 用独立连接执行，绕开 GORM 的预编译语句缓存：
	// VACUUM 会重建整个库，连接上已缓存的语句会全部失效。
	conn, err := sql.Open("sqlite", sqliteDSN(dbFilePath))
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetMaxOpenConns(1)

	// 先把 WAL 内容合并回主库：一是让大小统计真实反映数据量，
	// 二是避免后续 VACUUM 与未合并的 WAL 争抢资源。
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("vacuum: pre wal_checkpoint failed: %v", err)
	}

	beforeBytes, err := dbFileSize()
	if err != nil {
		return nil, err
	}
	// 覆盖 DSN 中的 busy_timeout(10s)：压缩期间定时采集等写入者会排队，
	// 需要更长的等待窗口，否则 VACUUM 容易因取不到锁而失败。
	if _, err := conn.Exec("PRAGMA busy_timeout=120000"); err != nil {
		log.Printf("vacuum: set busy_timeout failed: %v", err)
	}

	if _, err := conn.Exec("VACUUM"); err != nil {
		return nil, fmt.Errorf("VACUUM 执行失败（可能有写入尚未释放，或磁盘空间不足）: %w", err)
	}
	// WAL 模式下 VACUUM 重建后的页面先写入 WAL，主库文件的收缩要到 checkpoint 时才落地，
	// 因此必须再 checkpoint 一次，文件大小才真正反映压缩结果（否则会看到"没变小"甚至变大）。
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("vacuum: post wal_checkpoint failed: %v", err)
	}

	afterBytes, err := dbFileSize()
	if err != nil {
		return nil, err
	}

	return &VacuumResult{
		FilePath:    dbFilePath,
		BeforeBytes: beforeBytes,
		AfterBytes:  afterBytes,
		FreedBytes:  beforeBytes - afterBytes,
		DurationSec: time.Since(start).Seconds(),
	}, nil
}

// dbFileSize 返回当前数据库主文件大小
func dbFileSize() (int64, error) {
	fi, err := os.Stat(dbFilePath)
	if err != nil {
		return 0, fmt.Errorf("读取数据库文件失败: %w", err)
	}
	return fi.Size(), nil
}
