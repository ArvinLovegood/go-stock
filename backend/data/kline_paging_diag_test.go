//go:build live

package data

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bensema/gotdx/types"

	"go-stock/backend/db"
)

// 临时诊断：模拟前端 StockLightweightKlineChart 的「拖动到最早 → 加载更早历史」分页序列，
// 验证后端各数据源的 end 分页语义是否正确（返回严格早于当前最旧一根的数据）。
//
//	TEST_DB_SKIP_INIT=1 go test -tags live ./backend/data/ -run TestKlinePagingDiag -v

func klinePagingDiagInitDB(t *testing.T) {
	t.Helper()
	if db.Dao != nil {
		return
	}
	dbPath := filepath.Join(os.TempDir(), "go-stock-kline-diag.db")
	_ = os.Remove(dbPath)
	db.Init(dbPath)
	_ = db.Dao.AutoMigrate(&Settings{})
}

func klinePagingDiagEnd(oldestDay, klt string) string {
	day := strings.TrimSpace(oldestDay)
	switch klt {
	case "101", "102", "103":
		datePart := day
		if i := strings.Index(day, " "); i > 0 {
			datePart = day[:i]
		}
		t, err := time.ParseInLocation("2006-01-02", datePart, time.Local)
		if err != nil {
			return ""
		}
		return t.AddDate(0, 0, -1).Format("20060102")
	default:
		sec := 60
		switch klt {
		case "5":
			sec = 300
		case "15":
			sec = 900
		case "30":
			sec = 1800
		case "60":
			sec = 3600
		case "120":
			sec = 7200
		}
		for _, layout := range []string{"2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, day, time.Local); err == nil {
				return t.Add(-time.Duration(sec) * time.Second).Format("20060102150405")
			}
		}
		return ""
	}
}

func klinePagingDiagCheck(t *testing.T, code, klt string, initialLimit int) {
	t.Helper()
	klinePagingDiagInitDB(t)
	r1 := FetchKLineWithFallback(code, "", klt, initialLimit, "")
	if r1 == nil || r1.Data == nil || len(*r1.Data) == 0 {
		t.Fatalf("[%s klt=%s] 首屏无数据", code, klt)
	}
	src1 := r1.Source
	oldest := (*r1.Data)[0].Day
	newest := (*r1.Data)[len(*r1.Data)-1].Day
	end := klinePagingDiagEnd(oldest, klt)
	t.Logf("[%s klt=%s] 首屏 source=%s %d根 %s ~ %s → end=%s", code, klt, src1, len(*r1.Data), oldest, newest, end)
	if end == "" {
		t.Fatalf("[%s klt=%s] end 计算失败 oldest=%q", code, klt, oldest)
	}

	r2 := FetchKLineWithFallback(code, "", klt, 400, end, "")
	if r2 == nil || r2.Data == nil {
		t.Fatalf("[%s klt=%s] 分页返回 nil", code, klt)
	}
	src2 := r2.Source
	got := *r2.Data
	t.Logf("[%s klt=%s] 分页 source=%s %d根", code, klt, src2, len(got))
	if len(got) == 0 {
		t.Logf("[%s klt=%s] 分页为空（可能已到数据起点）", code, klt)
		return
	}
	pagedOldest := got[0].Day
	pagedNewest := got[len(got)-1].Day
	t.Logf("[%s klt=%s] 分页区间 %s ~ %s", code, klt, pagedOldest, pagedNewest)

	// 断言：分页结果必须全部严格早于首屏最旧一根
	exist := map[string]bool{}
	for _, k := range *r1.Data {
		exist[k.Day] = true
	}
	newAdded, dup := 0, 0
	for _, k := range got {
		if exist[k.Day] {
			dup++
		} else {
			newAdded++
		}
	}
	t.Logf("[%s klt=%s] 合并判定：新增=%d 重叠=%d", code, klt, newAdded, dup)
	if pagedNewest >= oldest {
		t.Errorf("[%s klt=%s] 分页未返回更早数据：分页最新=%s 应 < 首屏最旧=%s（重叠=%d 新增=%d）",
			code, klt, pagedNewest, oldest, dup, newAdded)
	}
	if newAdded == 0 {
		t.Errorf("[%s klt=%s] 分页与已有数据完全重叠：end=%s 返回 %d 根全部重复", code, klt, end, len(got))
	}
}

func TestKlinePagingDiagAShareDaily(t *testing.T) {
	klinePagingDiagCheck(t, "000001", "101", 800)
}

func TestKlinePagingDiagAShare60Min(t *testing.T) {
	klinePagingDiagCheck(t, "000001", "60", 500)
}

func TestKlinePagingDiagBinanceDaily(t *testing.T) {
	setupLiveProxy(t)
	klinePagingDiagCheck(t, "bn:btcusdt", "101", 500)
}

func TestKlinePagingDiagBinance15Min(t *testing.T) {
	setupLiveProxy(t)
	klinePagingDiagCheck(t, "bn:btcusdt", "15", 500)
}

// TestKlinePagingSourceDiag 底层数据源实验：定位 A股日K分页失败根因。
// TEST_DB_SKIP_INIT=1 go test -tags live ./backend/data/ -run TestKlinePagingSourceDiag -v
func TestKlinePagingSourceDiag(t *testing.T) {
	klinePagingDiagInitDB(t)
	api := NewTdxKLineApi()
	market, code := tdxMarketFromStockCode("000001")

	// A) MAC 日线 QFQ 手动分页：验证服务器是否支持 start 偏移取更早数据
	_ = api.ensureMACClient()
	for _, tc := range []struct{ start, count uint32 }{{0, 700}, {700, 700}, {1400, 700}} {
		bars, err := api.macClient.MACSymbolBars(market, code, 4, 1, tc.start, tc.count, types.AdjustQFQ)
		if err != nil {
			t.Logf("[MAC手动] start=%d count=%d err=%v", tc.start, tc.count, err)
			continue
		}
		if len(bars) == 0 {
			t.Logf("[MAC手动] start=%d count=%d → 0根", tc.start, tc.count)
			continue
		}
		t.Logf("[MAC手动] start=%d count=%d → %d根 %s ~ %s",
			tc.start, tc.count, len(bars),
			bars[0].DateTime.Format("2006-01-02"), bars[len(bars)-1].DateTime.Format("2006-01-02"))
	}

	// B) MAC 深取 8000（库内自动分页）：QFQ vs 不复权
	for _, adj := range []uint16{types.AdjustQFQ, types.AdjustNone} {
		bars, err := api.macClient.MACSymbolBars(market, code, 4, 1, 0, 8000, adj)
		if err != nil {
			t.Logf("[MAC深取 adj=%d] err=%v", adj, err)
			continue
		}
		if len(bars) == 0 {
			t.Logf("[MAC深取 adj=%d] → 0根", adj)
			continue
		}
		t.Logf("[MAC深取 adj=%d] → %d根 %s ~ %s", adj, len(bars),
			bars[0].DateTime.Format("2006-01-02"), bars[len(bars)-1].DateTime.Format("2006-01-02"))
	}

	// C) 普通通达信主站 StockKLine 日线：QFQ vs 不复权（复现 invalid kline item 0）
	_ = api.ensureClient()
	for _, adj := range []uint16{types.AdjustQFQ, types.AdjustNone} {
		bars, err := api.client.StockKLine(4, market, code, 0, 800, 0, adj)
		if err != nil {
			t.Logf("[TDX主站 adj=%d] err=%v", adj, err)
			continue
		}
		if len(bars) == 0 {
			t.Logf("[TDX主站 adj=%d] → 0根", adj)
			continue
		}
		t.Logf("[TDX主站 adj=%d] → %d根 %s ~ %s", adj, len(bars),
			bars[0].DateTime.Format("2006-01-02"), bars[len(bars)-1].DateTime.Format("2006-01-02"))
	}

	// D) 东财：分页 vs 首屏对照
	em := NewEastMoneyKLineApi(GetSettingConfig())
	d1 := em.GetKLineDataBefore("000001", "101", "1", 400, "20231218")
	t.Logf("[东财] Before end=20231218 lmt=400 → %d根", len(*d1))
	if len(*d1) > 0 {
		t.Logf("[东财] 区间 %s ~ %s", (*d1)[0].Day, (*d1)[len(*d1)-1].Day)
	}
	d2 := em.GetKLineData("000001", "101", "1", 10)
	t.Logf("[东财] 首屏 10根 → %d根", len(*d2))
}

// TestEastMoneyProdPathDiag 决定性实验：同一进程、同一 client 内交替执行
// A) 受控直连 em.client.R()  B) 生产 helper fetchKLineJSONBytesByHTTP  C) 直连复测  D) 生产函数 GetKLineData，
// 隔离 rc=102 由哪条路径触发；并程序化比对生产 URL 构造结果与受控 URL 是否逐字节一致。
// TEST_DB_SKIP_INIT=1 go test -tags live ./backend/data/ -run TestEastMoneyProdPathDiag -v
func TestEastMoneyProdPathDiag(t *testing.T) {
	klinePagingDiagInitDB(t)
	em := NewEastMoneyKLineApi(GetSettingConfig())
	t.Logf("config.CrawlTimeOut=%d BrowserPath=%q", em.config.CrawlTimeOut, em.config.BrowserPath)

	controlledURL := func() string {
		params := url.Values{}
		params.Set("secid", "0.000001")
		params.Set("klt", "101")
		params.Set("fqt", "1")
		params.Set("end", "20500101")
		params.Set("lmt", "10")
		params.Set("fields1", "f1,f2,f3,f4,f5,f6")
		params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116,f113,f114,f115")
		params.Set("wbp2u", "|0|0|0|web")
		params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
		return "https://push2his.eastmoney.com/api/qt/stock/kline/get?" + params.Encode()
	}

	rcOf := func(body string) string {
		if j := strings.Index(body, `"rc":`); j >= 0 {
			return body[j:min(j+12, len(body))]
		}
		return "rc=?(无rc字段)"
	}

	for round := 1; round <= 2; round++ {
		// A) 受控直连
		req := em.client.R()
		setEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
		resp, err := req.Get(controlledURL())
		if err != nil {
			t.Logf("[R%d A-直连] err=%v", round, err)
		} else {
			t.Logf("[R%d A-直连] HTTP=%d %s", round, resp.StatusCode(), rcOf(string(resp.Body())))
		}

		// B) 生产 helper，但喂受控 URL（helper 内部含 SetTimeout 突变）
		body, ferr := em.fetchKLineJSONBytesByHTTP(controlledURL())
		if ferr != nil {
			t.Logf("[R%d B-helper受控URL] err=%v", round, ferr)
		} else {
			t.Logf("[R%d B-helper受控URL] %s", round, rcOf(string(body)))
		}

		// C) 直连复测（验证 B 是否污染连接/状态）
		req2 := em.client.R()
		setEastMoneyKlineBrowserHeaders(req2, "https://quote.eastmoney.com")
		resp2, err2 := req2.Get(controlledURL())
		if err2 != nil {
			t.Logf("[R%d C-直连复测] err=%v", round, err2)
		} else {
			t.Logf("[R%d C-直连复测] HTTP=%d %s", round, resp2.StatusCode(), rcOf(string(resp2.Body())))
		}

		// D) 生产函数（内部 URL 已通过 Infof 日志打印）
		d := em.GetKLineData("000001", "101", "1", 10)
		t.Logf("[R%d D-生产函数] → %d根", round, len(*d))

		time.Sleep(2 * time.Second)
	}

	// E) 程序化 URL 比对（固定 _ 时间戳，逐字节 diff）
	fixedTS := "1762755000000"
	prodParams := url.Values{}
	fields := "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116"
	if adjustFlag := "1"; adjustFlag != "" {
		fields = "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116,f113,f114,f115"
	}
	prodParams.Set("secid", em.convertStockCode("000001"))
	prodParams.Set("klt", "101")
	prodParams.Set("fqt", "1")
	prodParams.Set("end", "20500101")
	prodParams.Set("lmt", strconv.Itoa(10))
	prodParams.Set("fields1", "f1,f2,f3,f4,f5,f6")
	prodParams.Set("fields2", fields)
	prodParams.Set("wbp2u", "|0|0|0|web")
	prodParams.Set("_", fixedTS)
	prodURL := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?%s", prodParams.Encode())
	t.Logf("生产构造 URL = %s", prodURL)
	t.Logf("受控 URL    = %s", controlledURL())
}
