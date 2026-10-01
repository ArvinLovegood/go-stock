package data

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestIsBinanceFuturesCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"bn:BTCUSDT", true},
		{"BN:btcusdt", true},
		{"bn:ethusdt", true},
		{"  bn:solusdt  ", true},
		// 前缀判定只认 bn:，是否为有效合约交由 ResolveBinanceSymbol 经 exchangeInfo 校验
		{"bn:BTCUSDT.P", true},
		{"BTCUSDT", false},
		{"sh600519", false},
		{"hk00700", false},
		{"gb_aapl", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsBinanceFuturesCode(c.in); got != c.want {
			t.Errorf("IsBinanceFuturesCode(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBinanceIntervalFromKlt(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "1d"},
		{"1", "1m"},
		{"1m", "1m"},
		{"5", "5m"},
		{"15", "15m"},
		{"30", "30m"},
		{"60", "1h"},
		{"120", "2h"},
		{"240", "4h"},
		{"101", "1d"},
		{"day", "1d"},
		{"102", "1w"},
		{"103", "1M"},
		{"104", "1M"},
		{"105", "1M"},
		{"106", "1M"},
		{"1M", "1M"},
		{"month", "1M"},
	}
	for _, c := range cases {
		if got := binanceIntervalFromKlt(c.in); got != c.want {
			t.Errorf("binanceIntervalFromKlt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBinanceFormatDay(t *testing.T) {
	// 0ms = UTC 1970-01-01 00:00，UTC+8 为 08:00
	if got := binanceFormatDayCST(0, "1d"); got != "1970-01-01" {
		t.Errorf("binanceFormatDayCST(0,1d) = %q, want 1970-01-01", got)
	}
	if got := binanceFormatDayCST(0, "1h"); got != "1970-01-01 08:00" {
		t.Errorf("binanceFormatDayCST(0,1h) = %q, want 1970-01-01 08:00", got)
	}
	if got := binanceFormatDay(0, "1d"); got != "1970-01-01" {
		t.Errorf("binanceFormatDay(0,1d) = %q, want 1970-01-01", got)
	}
}

func TestBinanceTrimNum(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"4.00000200", "4.000002"},
		{"0.00010000", "0.0001"},
		{"11793.63104562", "11793.63104562"},
		{"", ""},
		{"not-a-number", "not-a-number"},
	}
	for _, c := range cases {
		if got := binanceTrimNum(c.in); got != c.want {
			t.Errorf("binanceTrimNum(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBinanceEndTimeMs(t *testing.T) {
	if got := binanceEndTimeMs(""); got != 0 {
		t.Errorf("binanceEndTimeMs(\"\") = %d, want 0", got)
	}
	// 秒级时间戳
	if got := binanceEndTimeMs("1600000000"); got != 1600000000000 {
		t.Errorf("binanceEndTimeMs(秒) = %d, want 1600000000000", got)
	}
	// 毫秒级时间戳原样返回
	if got := binanceEndTimeMs("1600000000000"); got != 1600000000000 {
		t.Errorf("binanceEndTimeMs(毫秒) = %d, want 1600000000000", got)
	}
	// 日期串按 UTC+8 解析
	if got := binanceEndTimeMs("2026-10-01"); got != time.Date(2026, 10, 1, 0, 0, 0, 0, binanceCST).UnixMilli() {
		t.Errorf("binanceEndTimeMs(日期) = %d, 与期望不符", got)
	}
	if got := binanceEndTimeMs("无法解析"); got != 0 {
		t.Errorf("binanceEndTimeMs(无法解析) = %d, want 0", got)
	}
}

// TestBinanceResolveSymbol 注入假 exchangeInfo，避免联网。
func TestBinanceResolveSymbol(t *testing.T) {
	binanceExchangeInfoMu.Lock()
	prev := binanceExchangeInfoData
	binanceExchangeInfoData = &BinanceExchangeInfo{
		Symbols: []BinanceSymbolInfo{
			{Symbol: "BTCUSDT", BaseAsset: "BTC", DisplayName: "比特币/USDT 永续"},
			{Symbol: "ETHUSDT", BaseAsset: "ETH", DisplayName: "以太坊/USDT 永续"},
		},
		SymbolMap: map[string]BinanceSymbolInfo{
			"BTCUSDT": {Symbol: "BTCUSDT", BaseAsset: "BTC", DisplayName: "比特币/USDT 永续"},
			"ETHUSDT": {Symbol: "ETHUSDT", BaseAsset: "ETH", DisplayName: "以太坊/USDT 永续"},
		},
		FetchedAt: time.Now(),
	}
	binanceExchangeInfoMu.Unlock()
	t.Cleanup(func() {
		binanceExchangeInfoMu.Lock()
		binanceExchangeInfoData = prev
		binanceExchangeInfoMu.Unlock()
	})

	okCases := []struct {
		in, want string
	}{
		{"BTCUSDT", "BTCUSDT"},
		{"btc", "BTCUSDT"},
		{"btc/usdt", "BTCUSDT"},
		{"比特币", "BTCUSDT"},
		{"bn:btcusdt", "BTCUSDT"},
		{"eth", "ETHUSDT"},
	}
	for _, c := range okCases {
		got, ok := ResolveBinanceSymbol(c.in)
		if !ok || got != c.want {
			t.Errorf("ResolveBinanceSymbol(%q) = (%q,%v), want (%q,true)", c.in, got, ok, c.want)
		}
	}

	if _, ok := ResolveBinanceSymbol("sh600519"); ok {
		t.Errorf("ResolveBinanceSymbol(\"sh600519\") 应解析失败")
	}
	if _, ok := ResolveBinanceSymbol(""); ok {
		t.Errorf("ResolveBinanceSymbol(\"\") 应解析失败")
	}
}

func TestNormalizeStockCodeBinance(t *testing.T) {
	if got := normalizeStockCode("bn:BTCUSDT"); got != "bn:btcusdt" {
		t.Errorf("normalizeStockCode(\"bn:BTCUSDT\") = %q, want bn:btcusdt", got)
	}
	if got := normalizeStockCode("bn:btcusdt"); got != "bn:btcusdt" {
		t.Errorf("normalizeStockCode(\"bn:btcusdt\") = %q, want bn:btcusdt", got)
	}
}

func TestIsBinanceChannelFailure(t *testing.T) {
	channelFail := []int{403, 408, 418, 429, 451, 500, 502, 503, 504}
	for _, code := range channelFail {
		if !isBinanceChannelFailure(&binanceHTTPError{statusCode: code}) {
			t.Errorf("状态码 %d 应判定为通道失败", code)
		}
	}
	businessFail := []int{400, 404, 405}
	for _, code := range businessFail {
		if isBinanceChannelFailure(&binanceHTTPError{statusCode: code}) {
			t.Errorf("状态码 %d 应判定为业务错误（不切通道）", code)
		}
	}
	if !isBinanceChannelFailure(errors.New("dial tcp: i/o timeout")) {
		t.Errorf("网络层错误应判定为通道失败")
	}
}

func TestBinanceChannelStateMachine(t *testing.T) {
	binanceChannel.mu.Lock()
	prevPrefer, prevFailAt := binanceChannel.preferProxy, binanceChannel.lastFailAt
	binanceChannel.preferProxy = false
	binanceChannel.lastFailAt = time.Time{}
	binanceChannel.mu.Unlock()
	t.Cleanup(func() {
		binanceChannel.mu.Lock()
		binanceChannel.preferProxy, binanceChannel.lastFailAt = prevPrefer, prevFailAt
		binanceChannel.mu.Unlock()
	})

	if binancePreferProxy() {
		t.Fatalf("初始状态不应偏好代理")
	}
	binanceMarkDirectFailed()
	if !binancePreferProxy() {
		t.Fatalf("直连失败后应偏好代理")
	}
	// 冷却到期后应恢复尝试直连
	binanceChannel.mu.Lock()
	binanceChannel.lastFailAt = time.Now().Add(-binanceProxyCooldown - time.Minute)
	binanceChannel.mu.Unlock()
	if binancePreferProxy() {
		t.Fatalf("冷却到期后不应再偏好代理")
	}
	binanceMarkDirectOK()
	if binancePreferProxy() {
		t.Fatalf("标记直连恢复后不应偏好代理")
	}
}

// TestBinanceDTOParsing 用内置 fixture 校验各端点响应的字段解析。
func TestBinanceDTOParsing(t *testing.T) {
	klines := `[[1499040000000,"0.01634790","0.80000000","0.01575800","0.01577100","148976.11427815",1499644799999,"2434.19055334",308,"1756.87402397","28.46694368","0"]]`
	var rows [][]json.RawMessage
	if err := json.Unmarshal([]byte(klines), &rows); err != nil {
		t.Fatalf("解析 klines 失败: %v", err)
	}
	if len(rows) != 1 || len(rows[0]) < 8 {
		t.Fatalf("klines 行结构异常: %d", len(rows))
	}
	if got := binanceRawStr(rows[0][1]); got != "0.01634790" {
		t.Errorf("klines open = %q", got)
	}
	if got := binanceRawInt(rows[0][0]); got != 1499040000000 {
		t.Errorf("klines openTime = %d", got)
	}
	if got := binanceRawStr(rows[0][7]); got != "2434.19055334" {
		t.Errorf("klines quoteVolume = %q", got)
	}

	ticker := `{"symbol":"BTCUSDT","priceChange":"-94.99","priceChangePercent":"-95.960","lastPrice":"4.00000200","openPrice":"99.00000000","highPrice":"100.00000000","lowPrice":"0.10000000","volume":"8913.30000000","quoteVolume":"15.30000000"}`
	var tk BinanceTicker24h
	if err := json.Unmarshal([]byte(ticker), &tk); err != nil {
		t.Fatalf("解析 ticker 失败: %v", err)
	}
	if tk.Symbol != "BTCUSDT" || binanceFloat(tk.LastPrice) != 4.000002 {
		t.Errorf("ticker 解析异常: %+v", tk)
	}

	premium := `{"symbol":"BTCUSDT","markPrice":"11793.63104562","indexPrice":"11781.80495970","lastFundingRate":"0.00010000","nextFundingTime":1597392000000}`
	var pi BinancePremiumIndex
	if err := json.Unmarshal([]byte(premium), &pi); err != nil {
		t.Fatalf("解析 premiumIndex 失败: %v", err)
	}
	if pi.Symbol != "BTCUSDT" || binanceFloat(pi.LastFundingRate) != 0.0001 {
		t.Errorf("premiumIndex 解析异常: %+v", pi)
	}
	if pi.NextFundingTime != 1597392000000 {
		t.Errorf("premiumIndex nextFundingTime 异常: %d", pi.NextFundingTime)
	}

	funding := `[{"symbol":"BTCUSDT","fundingTime":1597392000000,"fundingRate":"0.00010000"}]`
	var fr []BinanceFundingRate
	if err := json.Unmarshal([]byte(funding), &fr); err != nil {
		t.Fatalf("解析 fundingRate 失败: %v", err)
	}
	if len(fr) != 1 || fr[0].FundingTime != 1597392000000 {
		t.Errorf("fundingRate 解析异常: %+v", fr)
	}

	ratio := `[{"symbol":"BTCUSDT","longShortRatio":"1.8105","longAccount":"0.6442","shortAccount":"0.3558","timestamp":1583139600000}]`
	var ls []BinanceLongShortRatio
	if err := json.Unmarshal([]byte(ratio), &ls); err != nil {
		t.Fatalf("解析多空比失败: %v", err)
	}
	if len(ls) != 1 || binanceFloat(ls[0].LongShortRatio) != 1.8105 {
		t.Errorf("多空比解析异常: %+v", ls)
	}

	oiHist := `[{"symbol":"BTCUSDT","sumOpenInterest":"20403.63700000","sumOpenInterestValue":"150570784.07809979","timestamp":1583127900000}]`
	var oi []BinanceOpenInterestHist
	if err := json.Unmarshal([]byte(oiHist), &oi); err != nil {
		t.Fatalf("解析 OI 历史失败: %v", err)
	}
	if len(oi) != 1 || binanceFloat(oi[0].SumOpenInterest) != 20403.637 {
		t.Errorf("OI 历史解析异常: %+v", oi)
	}
}

func TestBinanceHistPeriod(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "1h"},
		{"5m", "5m"},
		{"1H", "1h"},
		{"1d", "1d"},
		{"3m", "1h"}, // 不受支持的周期回落默认值
		{"weekly", "1h"},
	}
	for _, c := range cases {
		if got := binanceHistPeriod(c.in); got != c.want {
			t.Errorf("binanceHistPeriod(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}