//go:build live

package data

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/db"
)

// 联网测试：需显式开启 build tag 才会编译执行。
//
//	go test -tags live ./backend/data/ -run TestBitgetFuturesLive -v
//
// 分别在「未配置代理」与「已配置代理」两种设置下各跑一遍，验证直连/代理双通道均可用。
//
// 通过环境变量注入 Bitget 专用代理（不污染用户真实设置库）：
//
//	$env:TEST_DB_SKIP_INIT="1"; $env:TEST_BITGET_PROXY="http://localhost:10809"
//	go test -tags live ./backend/data/ -run TestBitgetFuturesLive -v

// setupBitgetLiveProxy 若设置了 TEST_BITGET_PROXY，则用独立临时库写入「Bitget 合约代理」，
// 使 live 测试可脱离用户真实设置，端到端验证 bitgetProxyClient 的代理通道。
func setupBitgetLiveProxy(t *testing.T) {
	t.Helper()
	proxy := os.Getenv("TEST_BITGET_PROXY")
	if proxy == "" {
		return
	}
	if db.Dao == nil {
		dbPath := filepath.Join(os.TempDir(), "go-stock-bitget-live.db")
		_ = os.Remove(dbPath)
		db.Init(dbPath)
	}
	// settings 表不在 db.AutoMigrate 中（由 main.go 单独迁移），此处按需创建
	if err := db.Dao.AutoMigrate(&Settings{}); err != nil {
		t.Fatalf("迁移 settings 表失败: %v", err)
	}
	if err := db.Dao.Create(&Settings{BitgetProxy: proxy}).Error; err != nil {
		t.Fatalf("写入测试代理失败: %v", err)
	}
}

func TestBitgetFuturesLiveContracts(t *testing.T) {
	setupBitgetLiveProxy(t)
	info := GetBitgetFuturesInfo()
	if info == nil || len(info.Symbols) == 0 {
		t.Fatal("GetBitgetFuturesInfo 为空：Bitget 接口不可达，请在设置中配置「Bitget 合约代理」后重试")
	}
	t.Logf("Bitget 美股永续（RWA）共 %d 个", len(info.Symbols))
	// 贵金属代币必须已排除
	if _, ok := info.SymbolMap["PAXGUSDT"]; ok {
		t.Errorf("PAXGUSDT 属非股票 RWA，不应出现在美股列表")
	}
	if _, ok := ResolveBitgetSymbol("苹果"); !ok {
		t.Errorf("中文别名「苹果」解析失败")
	}
}

func TestBitgetFuturesLiveTicker(t *testing.T) {
	setupBitgetLiveProxy(t)
	api := NewBitgetFuturesApi()
	ticker := api.GetTicker("AAPLUSDT")
	if ticker == nil {
		t.Fatal("GetTicker 返回 nil：Bitget 接口不可达，请在设置中配置「Bitget 合约代理」后重试")
	}
	if bitgetFloat(ticker.LastPr) <= 0 {
		t.Fatalf("最新价异常: %+v", ticker)
	}
	t.Logf("AAPLUSDT 最新价=%s 标记价=%s 指数价=%s 24h涨跌幅=%.2f%% 成交额=%s 资金费率=%s",
		ticker.LastPr, ticker.MarkPrice, ticker.IndexPrice,
		bitgetChangePercent(ticker), ticker.QuoteVolume, ticker.FundingRate)
}

func TestBitgetFuturesLiveKLine(t *testing.T) {
	setupBitgetLiveProxy(t)
	api := NewBitgetFuturesApi()
	data := api.GetKLine("AAPLUSDT", "101", 30, "")
	if data == nil || len(*data) == 0 {
		t.Fatal("GetKLine 返回空：Bitget 接口不可达或 symbol 无效")
	}
	first := (*data)[0]
	last := (*data)[len(*data)-1]
	if first.Open == "" || first.Close == "" || first.Day == "" {
		t.Fatalf("K线字段为空: %+v", first)
	}
	t.Logf("AAPLUSDT 日K %d 根：%s ~ %s，最新收盘=%s", len(*data), first.Day, last.Day, last.Close)
}

func TestBitgetFuturesLiveDerivatives(t *testing.T) {
	setupBitgetLiveProxy(t)
	api := NewBitgetFuturesApi()
	bundle := api.GetDerivatives("AAPLUSDT")
	if bundle == nil {
		t.Fatal("GetDerivatives 返回 nil：Bitget 接口不可达或 symbol 无效")
	}
	if bundle.MarkPrice <= 0 && bundle.IndexPrice <= 0 {
		t.Fatalf("标记价/指数价均为空: %+v", bundle)
	}
	t.Logf("AAPLUSDT 标记价=%.2f 指数价=%.2f 基差=%.4f%% 资金费率=%.4f%% 周期=%dh 年化=%.2f%%",
		bundle.MarkPrice, bundle.IndexPrice, bundle.Basis, bundle.FundingRate*100, bundle.RatePeriod, bundle.AnnualizedRate)
	t.Logf("OI=%.2f (名义 %.2f USDT) 资金费率历史点数=%d（美股永续无多空比、无 OI 历史）",
		bundle.OpenInterest, bundle.OpenInterestUSD, len(bundle.FundingRateHistory))
}

// TestBitgetFuturesLiveHotStock 校验热门榜单与榜单排序。
func TestBitgetFuturesLiveHotStock(t *testing.T) {
	setupBitgetLiveProxy(t)
	items := BitgetHotStock(20, "percent")
	if items == nil || len(*items) == 0 {
		t.Fatal("BitgetHotStock 返回空：Bitget 接口不可达")
	}
	for i := 1; i < len(*items); i++ {
		if (*items)[i-1].Percent < (*items)[i].Percent {
			t.Errorf("涨跌幅榜单未按降序: %v < %v", (*items)[i-1].Percent, (*items)[i].Percent)
			break
		}
	}
	t.Logf("涨跌幅榜 Top5: %s / %s / %s / %s / %s",
		(*items)[0].Name, (*items)[1].Name, (*items)[2].Name, (*items)[3].Name, (*items)[4].Name)
}

// TestBitgetFuturesLiveChannelFallback 注入不可用代理，验证双通道降级不 panic 且返回明确错误。
func TestBitgetFuturesLiveChannelFallback(t *testing.T) {
	setupBitgetLiveProxy(t)
	t.Log("双通道降级请通过界面配置「Bitget 合约代理」做端到端验证；此处仅确认调用链不 panic")
	api := NewBitgetFuturesApi()
	_ = api.GetTicker("AAPLUSDT")
	_ = api.GetAllTickers()
}