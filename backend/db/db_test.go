package db

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/models"
)

// 并发读写冒烟测试：验证连接池扩容 + WAL + busy_timeout 配置下，
// 多 goroutine 混合读写不会出现 database is locked / 死锁 / 断言失败。
// 使用独立临时库，不触碰主库 stock.db。
func TestConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	Init(filepath.Join(tmpDir, "concurrent_test.db"))
	defer func() {
		if dbCon, err := Dao.DB(); err == nil {
			_ = dbCon.Close()
		}
	}()
	// bk_fund_flow 表由 main.go 启动时迁移，测试库需自行建表
	if err := Dao.AutoMigrate(&models.BKFundFlow{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	// 并发规模参照真实负载：资金流向页并发拉取 40+ 曲线 + 采集任务批量写
	const (
		writers       = 4
		readers       = 8
		itersPerGo    = 30
	)
	var wg sync.WaitGroup
	errCh := make(chan error, writers+readers)

	start := time.Now()
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < itersPerGo; i++ {
				rec := &models.BKFundFlow{
					Code:      fmt.Sprintf("BKTEST%02d", w),
					Name:      "并发测试板块",
					NetInflow: int64(i*w - 10),
					SnapTime:  time.Now().Format("2006-01-02 15:04:05.000000000"),
				}
				if err := Dao.Create(rec).Error; err != nil {
					errCh <- fmt.Errorf("writer %d: %w", w, err)
					return
				}
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for i := 0; i < itersPerGo; i++ {
				var cnt int64
				if err := Dao.Model(&models.BKFundFlow{}).Where("code LIKE ?", "BKTEST%").Count(&cnt).Error; err != nil {
					errCh <- fmt.Errorf("reader %d: %w", r, err)
					return
				}
				var list []models.BKFundFlow
				if err := Dao.Where("code LIKE ?", "BKTEST%").Order("net_inflow DESC").Limit(20).Find(&list).Error; err != nil {
					errCh <- fmt.Errorf("reader %d list: %w", r, err)
					return
				}
			}
		}(r)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("并发读写出现错误: %v", err)
	}

	// 校验写入完整性
	var total int64
	if err := Dao.Model(&models.BKFundFlow{}).Where("code LIKE ?", "BKTEST%").Count(&total).Error; err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if want := int64(writers * itersPerGo); total != want {
		t.Errorf("写入不完整: got %d, want %d", total, want)
	}
	t.Logf("%d 写者 × %d 条 + %d 读者 × %d 轮 混合并发完成，耗时 %s，落库 %d 条",
		writers, itersPerGo, readers, itersPerGo, time.Since(start), total)

	// 验证 PRAGMA 确实生效
	var mode string
	if err := Dao.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil || mode != "wal" {
		t.Errorf("journal_mode 应为 wal，实际 %q (err=%v)", mode, err)
	}
	var busyTimeout int
	if err := Dao.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil || busyTimeout <= 0 {
		t.Errorf("busy_timeout 应 > 0，实际 %d (err=%v)", busyTimeout, err)
	}
	var mmapSize int64
	if err := Dao.Raw("PRAGMA mmap_size").Scan(&mmapSize).Error; err != nil || mmapSize <= 0 {
		t.Errorf("mmap_size 应 > 0，实际 %d (err=%v)", mmapSize, err)
	}
}

// 验证 DSN 拼装：裸路径追加并发 PRAGMA，已带参数的原样保留
func TestSqliteDSN(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "data/stock.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-131072)&_pragma=mmap_size(268435456)&_pragma=temp_store(MEMORY)&_pragma=wal_autocheckpoint(2000)"},
		{"../../data/stock.db", "../../data/stock.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-131072)&_pragma=mmap_size(268435456)&_pragma=temp_store(MEMORY)&_pragma=wal_autocheckpoint(2000)"},
		{"D:/go-stock/data/stock.db", "D:/go-stock/data/stock.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-131072)&_pragma=mmap_size(268435456)&_pragma=temp_store(MEMORY)&_pragma=wal_autocheckpoint(2000)"},
		{"data/stock.db?_pragma=busy_timeout(5000)", "data/stock.db?_pragma=busy_timeout(5000)"},
	}
	for _, c := range cases {
		if got := sqliteDSN(c.in); got != c.want {
			t.Errorf("sqliteDSN(%q)\n got  %s\n want %s", c.in, got, c.want)
		}
	}
}

// 池配置对比基准：模拟资金流向页并发读 + 采集写负载，对比旧配置(5连接/1空闲)与新配置(16/16)
func BenchmarkPoolConfig(b *testing.B) {
	runCase := func(b *testing.B, maxOpen, maxIdle int) {
		gdb, err := gorm.Open(sqlite.New(sqlite.Config{
			DriverName: "sqlite",
			DSN:        sqliteDSN(filepath.Join(b.TempDir(), "bench.db")),
		}), &gorm.Config{SkipDefaultTransaction: true, PrepareStmt: true})
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			if c, e := gdb.DB(); e == nil {
				_ = c.Close()
			}
		}()
		if err = gdb.AutoMigrate(&models.BKFundFlow{}); err != nil {
			b.Fatal(err)
		}
		dbCon, _ := gdb.DB()
		dbCon.SetMaxOpenConns(maxOpen)
		dbCon.SetMaxIdleConns(maxIdle)
		dbCon.SetConnMaxLifetime(0)

		// 预置数据
		for i := 0; i < 200; i++ {
			if err = gdb.Create(&models.BKFundFlow{
				Code: fmt.Sprintf("BK%04d", i), Name: "bench", NetInflow: int64(i),
				SnapTime: time.Now().Format("2006-01-02 15:04:05"),
			}).Error; err != nil {
				b.Fatal(err)
			}
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			// 8 并发读 + 2 并发写，模拟页面拉曲线 + 后台采集
			var wg sync.WaitGroup
			errCh := make(chan error, 10)
			for r := 0; r < 8; r++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var list []models.BKFundFlow
					if e := gdb.Where("code LIKE ?", "BK%").Order("net_inflow DESC").Limit(20).Find(&list).Error; e != nil {
						errCh <- e
					}
				}()
			}
			for w := 0; w < 2; w++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					if e := gdb.Create(&models.BKFundFlow{
						Code: fmt.Sprintf("BKW%d", w), Name: "bench", NetInflow: int64(w),
						SnapTime: time.Now().Format("2006-01-02 15:04:05.000000000"),
					}).Error; e != nil {
						errCh <- e
					}
				}(w)
			}
			wg.Wait()
			close(errCh)
			for e := range errCh {
				b.Fatal(e)
			}
		}
	}
	b.Run("old_5conns_1idle", func(b *testing.B) { runCase(b, 5, 1) })
	b.Run("new_16conns_16idle", func(b *testing.B) { runCase(b, 16, 16) })
}
