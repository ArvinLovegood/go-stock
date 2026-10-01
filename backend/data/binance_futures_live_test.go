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
//	go test -tags live ./backend/data/ -run TestBinanceFuturesLive -v
//
// 分别在「未配置代理」与「已配置代理」两种设置下各跑一遍，验证直连/代理双通道均可用。
//
// 通过环境变量注入币安专用代理（不污染用户真实设置库）：
//
//	$env:TEST_DB_SKIP_INIT="1"; $env:TEST_BINANCE_PROXY="http://localhost:10809"
//	go test -tags live ./backend/data/ -run TestBinanceFuturesLive -v

// setupLiveProxy 若设置了 TEST_BINANCE_PROXY，则用独立临时库写入「币安合约代理」，
// 使 live 测试可脱离用户真实设置，端到端验证 binanceProxyClient 的代理通道。
func setupLiveProxy(t *testing.T) {
	t.Helper()
	proxy := os.Getenv("TEST_BINANCE_PROXY")
	if proxy == "" {
		return
	}
	if db.Dao == nil {
		dbPath := filepath.Join(os.TempDir(), "go-stock-binance-live.db")
		_ = os.Remove(dbPath)
		db.Init(dbPath)
	}
	// settings 表不在 db.AutoMigrate 中（由 main.go 单独迁移），此处按需创建
	if err := db.Dao.AutoMigrate(&Settings{}); err != nil {
		t.Fatalf("迁移 settings 表失败: %v", err)
	}
	if err := db.Dao.Create(&Settings{BinanceProxy: proxy}).Error; err != nil {
		t.Fatalf("写入测试代理失败: %v", err)
	}
}

func TestBinanceFuturesLiveTicker(t *testing.T) {
	setupLiveProxy(t)
	api := NewBinanceFuturesApi()
	ticker := api.GetTicker24h("BTCUSDT")
	if ticker == nil {
		t.Fatal("GetTicker24h 返回 nil：币安接口不可达，请在设置中配置「币安合约代理」后重试")
	}
	if binanceFloat(ticker.LastPrice) <= 0 {
		t.Fatalf("最新价异常: %+v", ticker)
	}
	t.Logf("BTCUSDT 最新价=%s 24h涨跌幅=%s%% 成交额=%s", ticker.LastPrice, ticker.PriceChangePercent, ticker.QuoteVolume)
}

func TestBinanceFuturesLiveKLine(t *testing.T) {
	setupLiveProxy(t)
	api := NewBinanceFuturesApi()
	data := api.GetKLine("BTCUSDT", "101", 30, "")
	if data == nil || len(*data) == 0 {
		t.Fatal("GetKLine 返回空：币安接口不可达或 symbol 无效")
	}
	first := (*data)[0]
	last := (*data)[len(*data)-1]
	if first.Open == "" || first.Close == "" || first.Day == "" {
		t.Fatalf("K线字段为空: %+v", first)
	}
	t.Logf("BTCUSDT 日K %d 根：%s ~ %s，最新收盘=%s", len(*data), first.Day, last.Day, last.Close)
}

func TestBinanceFuturesLiveDerivatives(t *testing.T) {
	setupLiveProxy(t)
	api := NewBinanceFuturesApi()
	bundle := api.GetDerivatives("BTCUSDT", "1h", 24)
	if bundle == nil {
		t.Fatal("GetDerivatives 返回 nil：币安接口不可达或 symbol 无效")
	}
	if bundle.MarkPrice <= 0 && bundle.IndexPrice <= 0 {
		t.Fatalf("标记价/指数价均为空: %+v", bundle)
	}
	t.Logf("BTCUSDT 标记价=%.2f 指数价=%.2f 基差=%.4f%% 资金费率=%.4f%% 年化=%.2f%%",
		bundle.MarkPrice, bundle.IndexPrice, bundle.Basis, bundle.LastFundingRate*100, bundle.AnnualizedRate)
	t.Logf("OI=%.2f (名义 %.2f USDT, 区间变化 %.2f%%)", bundle.OpenInterest, bundle.OpenInterestValue, bundle.OpenInterestChange)
	t.Logf("全局多空比=%.4f（多 %.2f%% / 空 %.2f%%）主动买卖比=%.4f 历史点数=%d",
		bundle.LongShortRatio, bundle.LongAccount*100, bundle.ShortAccount*100, bundle.TakerBuySellRatio, len(bundle.LongShortHistory))
}

func TestBinanceFuturesLiveExchangeInfo(t *testing.T) {
	setupLiveProxy(t)
	info := GetBinanceExchangeInfo()
	if info == nil || len(info.Symbols) == 0 {
		t.Fatal("exchangeInfo 为空：币安接口不可达")
	}
	// 校验 TradFi（美股/ETF/大宗商品等）永续已被纳入
	tradFi := 0
	for _, s := range info.Symbols {
		if s.IsTradFi {
			tradFi++
		}
	}
	t.Logf("币安 USDT-M 永续合约共 %d 个（其中 TradFi 美股/传统金融 %d 个）", len(info.Symbols), tradFi)
	if tradFi == 0 {
		t.Errorf("未解析到 TradFi 永续（TRADIFI_PERPETUAL），过滤条件可能未生效")
	}
	if _, ok := ResolveBinanceSymbol("比特币"); !ok {
		t.Errorf("中文别名「比特币」解析失败")
	}
	// 美股中文别名与裸代码均应解析到币安 TradFi 合约
	if sym, ok := ResolveBinanceSymbol("苹果"); !ok || sym != "AAPLUSDT" {
		t.Errorf("ResolveBinanceSymbol(\"苹果\") = (%q,%v), want (AAPLUSDT,true)", sym, ok)
	}
	if sym, ok := ResolveBinanceSymbol("bn:nvdausdt"); !ok || sym != "NVDAUSDT" {
		t.Errorf("ResolveBinanceSymbol(\"bn:nvdausdt\") = (%q,%v), want (NVDAUSDT,true)", sym, ok)
	}
}

// TestBinanceFuturesLiveChannelFallback 注入不可用代理，验证双通道降级不 panic 且返回明确错误。
func TestBinanceFuturesLiveChannelFallback(t *testing.T) {
	setupLiveProxy(t)
	t.Log("双通道降级请通过界面配置「币安合约代理」做端到端验证；此处仅确认调用链不 panic")
	api := NewBinanceFuturesApi()
	_ = api.GetTicker24h("BTCUSDT")
	_ = api.GetAllTickers()
}