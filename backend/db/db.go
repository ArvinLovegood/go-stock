package db

import (
	"log"
	"os"
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
	var openDb *gorm.DB
	var err error
	if sqlitePath == "" {
		// modernc.org/sqlite 的 DSN 参数风格为 _pragma=name(value)，与 mattn 风格（_busy_timeout=...）不兼容
		sqlitePath = "data/stock.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-524288)"
	}
	openDb, err = gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: sqlitePath}), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}

	// 兜底：确保 busy_timeout / WAL / synchronous 生效（不同驱动/DSN 参数支持可能存在差异）
	_ = openDb.Exec("PRAGMA busy_timeout=10000").Error
	_ = openDb.Exec("PRAGMA journal_mode=WAL").Error
	_ = openDb.Exec("PRAGMA synchronous=NORMAL").Error

	dbCon, err := openDb.DB()
	if err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	// SQLite 写入是串行锁模型：连接开太多会放大锁竞争导致 SQLITE_BUSY
	dbCon.SetMaxIdleConns(1)
	dbCon.SetMaxOpenConns(5)
	dbCon.SetConnMaxLifetime(time.Hour)
	Dao = openDb
	AutoMigrate()
	// 启动时异步清理过期缓存（保留最近 1 天），避免数据库无限增长
	go ClearExpiredStockTransactionCache()
}
