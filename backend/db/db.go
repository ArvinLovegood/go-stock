package db

import (
	"log"
	"os"
	"strings"
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
		"&_pragma=wal_autocheckpoint(2000)" // WAL 超过约 8MB 才自动 checkpoint，降低交易时段高频写的卡顿
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
	dbCon.SetMaxIdleConns(16)
	dbCon.SetMaxOpenConns(16)
	// SQLite 为嵌入式库，连接无服务端状态可刷新，不设生命周期可避免
	// 周期性换连接后 database/sql 内部按连接缓存的预编译语句全部作废重建
	dbCon.SetConnMaxLifetime(0)
	Dao = openDb
	AutoMigrate()
	// 启动时异步清理过期缓存（保留最近 1 天），避免数据库无限增长
	go ClearExpiredStockTransactionCache()
}
