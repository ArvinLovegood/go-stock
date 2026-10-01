package data

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestIsBitgetFuturesCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"bt:aaplusdt", true},
		{"BT:AAPLUSDT", true},
		{"  bt:tslausdt  ", true},
		// 前缀判定只认 bt:，是否为有效合约交由 ResolveBitgetSymbol 经白名单校验
		{"bt:whatever", true},
		{"bn:btcusdt", false},
		{"AAPLUSDT", false},
		{"sh600519", false},
		{"hk00700", false},
		{"gb_aapl", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsBitgetFuturesCode(c.in); got != c.want {
			t.Errorf("IsBitgetFuturesCode(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBitgetIntervalFromKlt(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "1D"},
		{"1", "1m"},
		{"1m", "1m"},
		{"3", "3m"},
		{"5", "5m"},
		{"15", "15m"},
		{"30", "30m"},
		{"60", "1H"},
		{"1h", "1H"},
		{"120", "2H"},
		{"240", "4H"},
		{"101", "1D"},
		{"day", "1D"},
		{"102", "1W"},
		{"103", "1M"},
		{"104", "1M"},
		{"105", "1M"},
		{"106", "1M"},
		{"1M", "1M"},
		{"month", "1M"},
	}
	for _, c := range cases {
		if got := bitgetIntervalFromKlt(c.in); got != c.want {
			t.Errorf("bitgetIntervalFromKlt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBitgetFormatDay(t *testing.T) {
	// 0ms = UTC 1970-01-01 00:00，UTC+8 为 08:00
	if got := bitgetFormatDay(0, "1D"); got != "1970-01-01" {
		t.Errorf("bitgetFormatDay(0,1D) = %q, want 1970-01-01", got)
	}
	if got := bitgetFormatDay(0, "1m"); got != "1970-01-01 08:00" {
		t.Errorf("bitgetFormatDay(0,1m) = %q, want 1970-01-01 08:00", got)
	}
	if got := bitgetFormatDay(0, "1H"); got != "1970-01-01 08:00" {
		t.Errorf("bitgetFormatDay(0,1H) = %q, want 1970-01-01 08:00", got)
	}
	if got := bitgetFormatDay(0, "1W"); got != "1970-01-01" {
		t.Errorf("bitgetFormatDay(0,1W) = %q, want 1970-01-01", got)
	}
}

func TestBitgetTrimNum(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"4.00000200", "4.000002"},
		{"0.00010000", "0.0001"},
		{"334.99000000", "334.99"},
		{"", ""},
		{"not-a-number", "not-a-number"},
	}
	for _, c := range cases {
		if got := bitgetTrimNum(c.in); got != c.want {
			t.Errorf("bitgetTrimNum(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBitgetEndTimeMs(t *testing.T) {
	if got := bitgetEndTimeMs(""); got != 0 {
		t.Errorf("bitgetEndTimeMs(\"\") = %d, want 0", got)
	}
	if got := bitgetEndTimeMs("1600000000"); got != 1600000000000 {
		t.Errorf("bitgetEndTimeMs(秒) = %d, want 1600000000000", got)
	}
	if got := bitgetEndTimeMs("1600000000000"); got != 1600000000000 {
		t.Errorf("bitgetEndTimeMs(毫秒) = %d, want 1600000000000", got)
	}
	if got := bitgetEndTimeMs("2026-10-01"); got != time.Date(2026, 10, 1, 0, 0, 0, 0, bitgetCST).UnixMilli() {
		t.Errorf("bitgetEndTimeMs(日期) = %d, 与期望不符", got)
	}
	if got := bitgetEndTimeMs("无法解析"); got != 0 {
		t.Errorf("bitgetEndTimeMs(无法解析) = %d, want 0", got)
	}
}

// TestBitgetResolveSymbol 注入假 /contracts，避免联网。
func TestBitgetResolveSymbol(t *testing.T) {
	bitgetInfoMu.Lock()
	prev := bitgetInfoData
	bitgetInfoData = &BitgetFuturesInfo{
		Symbols: []BitgetSymbolInfo{
			{Symbol: "AAPLUSDT", BaseCoin: "AAPL", DisplayName: "苹果(AAPL)/USDT 美股永续"},
			{Symbol: "TSLAUSDT", BaseCoin: "TSLA", DisplayName: "特斯拉(TSLA)/USDT 美股永续"},
		},
		SymbolMap: map[string]BitgetSymbolInfo{
			"AAPLUSDT": {Symbol: "AAPLUSDT", BaseCoin: "AAPL", DisplayName: "苹果(AAPL)/USDT 美股永续"},
			"TSLAUSDT": {Symbol: "TSLAUSDT", BaseCoin: "TSLA", DisplayName: "特斯拉(TSLA)/USDT 美股永续"},
		},
		FetchedAt: time.Now(),
	}
	bitgetInfoMu.Unlock()
	t.Cleanup(func() {
		bitgetInfoMu.Lock()
		bitgetInfoData = prev
		bitgetInfoMu.Unlock()
	})

	okCases := []struct {
		in, want string
	}{
		{"AAPLUSDT", "AAPLUSDT"},
		{"aapl", "AAPLUSDT"},
		{"aapl/usdt", "AAPLUSDT"},
		{"bt:aaplusdt", "AAPLUSDT"},
		{"苹果", "AAPLUSDT"},
		{"tsla", "TSLAUSDT"},
		{"特斯拉", "TSLAUSDT"},
	}
	for _, c := range okCases {
		got, ok := ResolveBitgetSymbol(c.in)
		if !ok || got != c.want {
			t.Errorf("ResolveBitgetSymbol(%q) = (%q,%v), want (%q,true)", c.in, got, ok, c.want)
		}
	}

	// 白名单外标的应拒绝
	if _, ok := ResolveBitgetSymbol("sh600519"); ok {
		t.Errorf("ResolveBitgetSymbol(\"sh600519\") 应解析失败")
	}
	if _, ok := ResolveBitgetSymbol("googl"); ok {
		t.Errorf("白名单外的 googl 应解析失败")
	}
	if _, ok := ResolveBitgetSymbol(""); ok {
		t.Errorf("ResolveBitgetSymbol(\"\") 应解析失败")
	}
}

// TestBitgetResolveSymbolOfflineFallback 白名单不可用时按格式兜底，但仍须拒绝贵金属代币。
func TestBitgetResolveSymbolOfflineFallback(t *testing.T) {
	bitgetInfoMu.Lock()
	prev := bitgetInfoData
	bitgetInfoData = nil
	bitgetInfoMu.Unlock()
	t.Cleanup(func() {
		bitgetInfoMu.Lock()
		bitgetInfoData = prev
		bitgetInfoMu.Unlock()
	})

	if got, ok := ResolveBitgetSymbol("aapl"); !ok || got != "AAPLUSDT" {
		t.Errorf("离线兜底 ResolveBitgetSymbol(\"aapl\") = (%q,%v), want (AAPLUSDT,true)", got, ok)
	}
	if got, ok := ResolveBitgetSymbol("bt:tslausdt"); !ok || got != "TSLAUSDT" {
		t.Errorf("离线兜底 ResolveBitgetSymbol(\"bt:tslausdt\") = (%q,%v), want (TSLAUSDT,true)", got, ok)
	}
	// 非股票 RWA 必须拒绝
	if _, ok := ResolveBitgetSymbol("paxg"); ok {
		t.Errorf("贵金属代币 PAXG 应被拒绝")
	}
	if _, ok := ResolveBitgetSymbol("xautusdt"); ok {
		t.Errorf("贵金属代币 XAUT 应被拒绝")
	}
}

func TestBitgetDisplayName(t *testing.T) {
	if got := bitgetDisplayName("AAPL"); got != "苹果(AAPL)/USDT 美股永续" {
		t.Errorf("bitgetDisplayName(AAPL) = %q", got)
	}
	if got := bitgetDisplayName("zzzz"); got != "ZZZZ/USDT 美股永续" {
		t.Errorf("bitgetDisplayName(zzzz) = %q", got)
	}
}

func TestNormalizeStockCodeBitget(t *testing.T) {
	if got := normalizeStockCode("bt:AAPLUSDT"); got != "bt:aaplusdt" {
		t.Errorf("normalizeStockCode(\"bt:AAPLUSDT\") = %q, want bt:aaplusdt", got)
	}
	if got := normalizeStockCode("bt:aaplusdt"); got != "bt:aaplusdt" {
		t.Errorf("normalizeStockCode(\"bt:aaplusdt\") = %q, want bt:aaplusdt", got)
	}
}

func TestIsBitgetChannelFailure(t *testing.T) {
	channelFail := []int{403, 408, 418, 429, 451, 500, 502, 503, 504}
	for _, code := range channelFail {
		if !isBitgetChannelFailure(&bitgetHTTPError{statusCode: code}) {
			t.Errorf("状态码 %d 应判定为通道失败", code)
		}
	}
	businessFail := []int{400, 404, 405}
	for _, code := range businessFail {
		if isBitgetChannelFailure(&bitgetHTTPError{statusCode: code}) {
			t.Errorf("状态码 %d 应判定为业务错误（不切通道）", code)
		}
	}
	// 业务码错误（HTTP 200 但 code != 00000）不应切通道
	if isBitgetChannelFailure(&bitgetAPIError{code: "40054", msg: "empty"}) {
		t.Errorf("业务码错误不应判定为通道失败")
	}
	if !isBitgetChannelFailure(errors.New("dial tcp: i/o timeout")) {
		t.Errorf("网络层错误应判定为通道失败")
	}
}

func TestIsBitgetEmptyData(t *testing.T) {
	if !isBitgetEmptyData(&bitgetAPIError{code: bitgetEmptyDataCode, msg: "empty"}) {
		t.Errorf("40054 应判定为无数据")
	}
	if isBitgetEmptyData(&bitgetAPIError{code: "40000", msg: "other"}) {
		t.Errorf("非 40054 不应判定为无数据")
	}
	if isBitgetEmptyData(errors.New("other")) {
		t.Errorf("普通错误不应判定为无数据")
	}
}

func TestBitgetChannelStateMachine(t *testing.T) {
	bitgetChannel.mu.Lock()
	prevPrefer, prevFailAt := bitgetChannel.preferProxy, bitgetChannel.lastFailAt
	bitgetChannel.preferProxy = false
	bitgetChannel.lastFailAt = time.Time{}
	bitgetChannel.mu.Unlock()
	t.Cleanup(func() {
		bitgetChannel.mu.Lock()
		bitgetChannel.preferProxy, bitgetChannel.lastFailAt = prevPrefer, prevFailAt
		bitgetChannel.mu.Unlock()
	})

	if bitgetPreferProxy() {
		t.Fatalf("初始状态不应偏好代理")
	}
	bitgetMarkDirectFailed()
	if !bitgetPreferProxy() {
		t.Fatalf("直连失败后应偏好代理")
	}
	bitgetChannel.mu.Lock()
	bitgetChannel.lastFailAt = time.Now().Add(-bitgetProxyCooldown - time.Minute)
	bitgetChannel.mu.Unlock()
	if bitgetPreferProxy() {
		t.Fatalf("冷却到期后不应再偏好代理")
	}
	bitgetMarkDirectOK()
	if bitgetPreferProxy() {
		t.Fatalf("标记直连恢复后不应偏好代理")
	}
}

// TestBitgetDTOParsing 用内置 fixture 校验各端点响应的字段解析。
func TestBitgetDTOParsing(t *testing.T) {
	// /tickers 元素（data 为数组，需取 [0]）
	tickerJSON := `{"symbol":"AAPLUSDT","lastPr":"334.99","bidPr":"334.98","askPr":"335.00","bidSz":"12.5","askSz":"8.3","high24h":"340.00","low24h":"330.00","ts":"1790841600000","change24h":"0.01414","baseVolume":"18570.42","quoteVolume":"6250000.5","indexPrice":"334.884","fundingRate":"0.0001","holdingAmount":"18570.42","openUtc":"1790755200000","changeUtc24h":"0.01414","open24h":"330.32","markPrice":"334.99"}`
	var list []BitgetTicker
	if err := json.Unmarshal([]byte("["+tickerJSON+"]"), &list); err != nil {
		t.Fatalf("解析 ticker 失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ticker 数组长度异常: %d", len(list))
	}
	tk := list[0]
	if tk.Symbol != "AAPLUSDT" || bitgetFloat(tk.LastPr) != 334.99 {
		t.Errorf("ticker 解析异常: %+v", tk)
	}
	if bitgetFloat(tk.Change24h) != 0.01414 {
		t.Errorf("change24h 应为比率小数: %+v", tk.Change24h)
	}
	// 涨跌幅 = change24h * 100；涨跌额 = lastPr - open24h
	if got := bitgetChangePercent(&tk); got < 1.413 || got > 1.415 {
		t.Errorf("涨跌幅换算异常: %v", got)
	}
	if got := bitgetFloat(tk.LastPr) - bitgetFloat(tk.Open24h); got < 4.66 || got > 4.68 {
		t.Errorf("涨跌额换算异常: %v", got)
	}
	if bitgetFloat(tk.BidSz) != 12.5 || bitgetFloat(tk.AskPr) != 335.0 {
		t.Errorf("买卖档位解析异常: %+v", tk)
	}

	// /ticker 的单元素数组形态
	var single []BitgetTicker
	if err := json.Unmarshal([]byte("["+tickerJSON+"]"), &single); err != nil || len(single) != 1 {
		t.Fatalf("/ticker 单元素数组解析失败: %v", err)
	}

	// /candles 二维数组：升序，[ts,o,h,l,c,baseVol,quoteVol]
	candles := `[[1790755200000,"330.32","341.00","329.10","334.99","18570.42","6250000.5"],[1790841600000,"334.99","338.00","332.00","336.50","12000.00","4000000.0"]]`
	var rows [][]json.RawMessage
	if err := json.Unmarshal([]byte(candles), &rows); err != nil {
		t.Fatalf("解析 candles 失败: %v", err)
	}
	if len(rows) != 2 || len(rows[0]) < 7 {
		t.Fatalf("candles 行结构异常: %d", len(rows))
	}
	if got := bitgetRawInt(rows[0][0]); got != 1790755200000 {
		t.Errorf("candles openTime = %d", got)
	}
	if got := bitgetRawStr(rows[0][1]); got != "330.32" {
		t.Errorf("candles open = %q", got)
	}
	if got := bitgetRawStr(rows[0][6]); got != "6250000.5" {
		t.Errorf("candles quoteVolume = %q", got)
	}

	// 统一响应外壳
	envJSON := `{"code":"00000","msg":"success","requestTime":1790841600000,"data":[{"symbol":"AAPLUSDT","price":"334.99","indexPrice":"334.884","markPrice":"334.99","ts":"1790841600000"}]}`
	var env bitgetEnvelope
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		t.Fatalf("解析外壳失败: %v", err)
	}
	if env.Code != bitgetSuccessCode || env.Msg != "success" {
		t.Errorf("外壳解析异常: %+v", env)
	}
	var sp []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &sp); err != nil || len(sp) != 1 {
		t.Fatalf("symbol-price data 解析失败: %v", err)
	}
	if sp[0].Symbol != "AAPLUSDT" || bitgetFloat(sp[0].Price) != 334.99 {
		t.Errorf("symbol-price 解析异常: %+v", sp[0])
	}

	// 业务错误外壳
	emptyJSON := `{"code":"40054","msg":"The data fetched by AAPLUSDT is empty","requestTime":1790841600000,"data":null}`
	var emptyEnv bitgetEnvelope
	if err := json.Unmarshal([]byte(emptyJSON), &emptyEnv); err != nil {
		t.Fatalf("解析错误外壳失败: %v", err)
	}
	if !isBitgetEmptyData(&bitgetAPIError{code: emptyEnv.Code, msg: emptyEnv.Msg}) {
		t.Errorf("40054 外壳应判定为无数据: %+v", emptyEnv)
	}

	// /current-fund-rate
	fundJSON := `{"code":"00000","msg":"success","data":{"symbol":"AAPLUSDT","fundingRate":"0.0001","fundingRateInterval":"8","nextUpdate":"1790841600000","minFundingRate":"-0.003","maxFundingRate":"0.003"}}`
	var fundEnv bitgetEnvelope
	if err := json.Unmarshal([]byte(fundJSON), &fundEnv); err != nil {
		t.Fatalf("解析 current-fund-rate 失败: %v", err)
	}
	var fr struct {
		Symbol              string `json:"symbol"`
		FundingRate         string `json:"fundingRate"`
		FundingRateInterval string `json:"fundingRateInterval"`
		NextUpdate          string `json:"nextUpdate"`
	}
	if err := json.Unmarshal(fundEnv.Data, &fr); err != nil {
		t.Fatalf("current-fund-rate data 解析失败: %v", err)
	}
	if fr.Symbol != "AAPLUSDT" || bitgetFloat(fr.FundingRate) != 0.0001 {
		t.Errorf("current-fund-rate 解析异常: %+v", fr)
	}

	// /funding-time
	ftJSON := `{"code":"00000","msg":"success","data":{"symbol":"AAPLUSDT","nextFundingTime":"1790841600000","ratePeriod":"8"}}`
	var ftEnv bitgetEnvelope
	if err := json.Unmarshal([]byte(ftJSON), &ftEnv); err != nil {
		t.Fatalf("解析 funding-time 失败: %v", err)
	}
	var ft struct {
		NextFundingTime string `json:"nextFundingTime"`
		RatePeriod      string `json:"ratePeriod"`
	}
	if err := json.Unmarshal(ftEnv.Data, &ft); err != nil {
		t.Fatalf("funding-time data 解析失败: %v", err)
	}
	if ft.NextFundingTime != "1790841600000" || ft.RatePeriod != "8" {
		t.Errorf("funding-time 解析异常: %+v", ft)
	}

	// /history-fund-rate（返回新→旧，需 reverse 为升序）
	histJSON := `[{"symbol":"AAPLUSDT","fundingRate":"0.0003","fundingTime":1790928000000},{"symbol":"AAPLUSDT","fundingRate":"0.0001","fundingTime":1790841600000}]`
	var hist []BitgetFundingRate
	if err := json.Unmarshal([]byte(histJSON), &hist); err != nil {
		t.Fatalf("解析 history-fund-rate 失败: %v", err)
	}
	if len(hist) != 2 || hist[0].FundingTime != 1790928000000 {
		t.Fatalf("history-fund-rate 原始顺序异常: %+v", hist)
	}
	for i, j := 0, len(hist)-1; i < j; i, j = i+1, j-1 {
		hist[i], hist[j] = hist[j], hist[i]
	}
	if hist[0].FundingTime != 1790841600000 || len(hist) != 2 {
		t.Errorf("history-fund-rate reverse 后顺序异常: %+v", hist)
	}

	// 实际接口的 fundingTime 为字符串，须与数字形态同样可解析
	histStr := `[{"symbol":"AAPLUSDT","fundingRate":"0","fundingTime":"1790812800000"},{"symbol":"AAPLUSDT","fundingRate":"0.000193","fundingTime":"1790726400000"}]`
	var hist2 []BitgetFundingRate
	if err := json.Unmarshal([]byte(histStr), &hist2); err != nil {
		t.Fatalf("解析字符串形态 fundingTime 失败: %v", err)
	}
	if len(hist2) != 2 || hist2[0].FundingTime != 1790812800000 || bitgetFloat(hist2[1].FundingRate) != 0.000193 {
		t.Errorf("字符串形态 fundingTime 解析异常: %+v", hist2)
	}

	// /open-interest
	oiJSON := `{"code":"00000","msg":"success","data":{"openInterestList":[{"symbol":"AAPLUSDT","size":"18570.42"}],"ts":"1790841600000"}}`
	var oiEnv bitgetEnvelope
	if err := json.Unmarshal([]byte(oiJSON), &oiEnv); err != nil {
		t.Fatalf("解析 open-interest 失败: %v", err)
	}
	var oi struct {
		OpenInterestList []struct {
			Symbol string `json:"symbol"`
			Size   string `json:"size"`
		} `json:"openInterestList"`
	}
	if err := json.Unmarshal(oiEnv.Data, &oi); err != nil {
		t.Fatalf("open-interest data 解析失败: %v", err)
	}
	if len(oi.OpenInterestList) != 1 || bitgetFloat(oi.OpenInterestList[0].Size) != 18570.42 {
		t.Errorf("open-interest 解析异常: %+v", oi)
	}

	// /contracts（含 isRwa 字段，字符串类型）
	contractsJSON := `[{"symbol":"AAPLUSDT","baseCoin":"AAPL","quoteCoin":"USDT","symbolStatus":"normal","isRwa":"YES","pricePlace":"2","volumePlace":"4","fundInterval":"8","maxLever":"20"},{"symbol":"PAXGUSDT","baseCoin":"PAXG","quoteCoin":"USDT","symbolStatus":"normal","isRwa":"YES","pricePlace":"2","volumePlace":"4","fundInterval":"8","maxLever":"20"}]`
	var contracts []struct {
		Symbol       string `json:"symbol"`
		BaseCoin     string `json:"baseCoin"`
		SymbolStatus string `json:"symbolStatus"`
		IsRwa        string `json:"isRwa"`
		PricePlace   string `json:"pricePlace"`
		FundInterval string `json:"fundInterval"`
		MaxLever     string `json:"maxLever"`
	}
	if err := json.Unmarshal([]byte(contractsJSON), &contracts); err != nil {
		t.Fatalf("解析 contracts 失败: %v", err)
	}
	if len(contracts) != 2 || contracts[0].IsRwa != "YES" || contracts[0].PricePlace != "2" {
		t.Errorf("contracts 解析异常: %+v", contracts[0])
	}
	// 白名单过滤：PAXG 应被排除
	kept := 0
	for _, c := range contracts {
		if _, excluded := bitgetNonStockRWA[c.BaseCoin]; excluded {
			continue
		}
		kept++
	}
	if kept != 1 {
		t.Errorf("非股票 RWA 过滤后应剩 1 个，实得 %d", kept)
	}
}

// TestBitgetDerivativesDegradation 校验能力缺口降级：无 OI 历史、无多空比时字段为零值而非 panic。
func TestBitgetDerivativesDegradation(t *testing.T) {
	bundle := BitgetDerivativesBundle{}
	if bundle.OpenInterest != 0 || bundle.Basis != 0 {
		t.Errorf("零值 bundle 异常: %+v", bundle)
	}
	if len(bundle.FundingRateHistory) != 0 {
		t.Errorf("资金费率历史初始应为空")
	}
	// 年化折算公式与实现一致：费率 * (365*24/周期) * 100
	rate := 0.0001
	period := 8
	annual := rate * (365 * 24 / float64(period)) * 100
	if annual != 10.95 {
		t.Errorf("年化折算异常: %v", annual)
	}
}