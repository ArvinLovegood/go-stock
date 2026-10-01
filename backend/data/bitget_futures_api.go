package data

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 本文件是 Bitget 美股永续合约各 REST 端点的门面（facade），
// 统一走 bitget_futures_client.go 的直连/代理双通道 + TTL 缓存。
// 行情形态复用项目既有的 KLineData / MinuteData / StockInfo，
// 永续衍生指标走 BitgetDerivativesBundle。

// 各端点缓存时长。行情类取较短 TTL 兼顾新鲜度与配额；历史类放宽。
const (
	bitgetTTLTickers      = 5 * time.Second
	bitgetTTLTicker       = 3 * time.Second
	bitgetTTLKline        = 3 * time.Second
	bitgetTTLMarkPrice    = 10 * time.Second
	bitgetTTLFunding      = 10 * time.Second
	bitgetTTLFundingTime  = 10 * time.Second
	bitgetTTLFundingHist  = 60 * time.Second
	bitgetTTLOpenInterest = 10 * time.Second
)

// BitgetFuturesApi Bitget 美股永续合约数据源门面。
type BitgetFuturesApi struct{}

// NewBitgetFuturesApi 创建 Bitget 美股永续合约数据源。
func NewBitgetFuturesApi() *BitgetFuturesApi {
	return &BitgetFuturesApi{}
}

// ===== 数值/参数辅助 =====

func bitgetFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// bitgetTrimNum 把 Bitget 返回的高精度小数字符串归一为最简形式，避免图表标签出现冗余尾零。
func bitgetTrimNum(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func bitgetQuery(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			v.Set(kv[i], kv[i+1])
		}
	}
	return v
}

// bitgetBaseParams 所有行情端点都必须携带 productType。
func bitgetBaseParams(extra ...string) url.Values {
	v := bitgetQuery(extra...)
	v.Set("productType", bitgetProductType)
	return v
}

// ===== 实时行情 =====

// GetTicker 获取单个合约 24h 行情。
func (b *BitgetFuturesApi) GetTicker(symbol string) *BitgetTicker {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	data, err := bitgetFetch(ctx, "ticker:"+sym, bitgetTTLTicker, "/ticker",
		bitgetBaseParams("symbol", sym))
	if err != nil {
		logger.SugaredLogger.Warnf("获取 Bitget 24h 行情失败: symbol=%s err=%v", sym, err)
		return nil
	}
	// /ticker 的 data 为单元素数组
	var list []BitgetTicker
	if err := json.Unmarshal(data, &list); err != nil || len(list) == 0 {
		return nil
	}
	return &list[0]
}

// GetAllTickers 获取全部美股永续合约 24h 行情（按白名单过滤）。
func (b *BitgetFuturesApi) GetAllTickers() []BitgetTicker {
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	data, err := bitgetFetch(ctx, "tickers:all", bitgetTTLTickers, "/tickers",
		bitgetBaseParams())
	if err != nil {
		logger.SugaredLogger.Warnf("获取 Bitget 全量 24h 行情失败: %v", err)
		return nil
	}
	var list []BitgetTicker
	if err := json.Unmarshal(data, &list); err != nil {
		logger.SugaredLogger.Warnf("解析 Bitget 全量 24h 行情失败: %v", err)
		return nil
	}
	info := GetBitgetFuturesInfo()
	if info == nil || len(info.SymbolMap) == 0 {
		return nil
	}
	filtered := make([]BitgetTicker, 0, len(list))
	for _, t := range list {
		if _, ok := info.SymbolMap[strings.ToUpper(t.Symbol)]; ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// bitgetChangePercent /ticker 只返回 ratio 形式的 change24h（小数），此处换算为百分数。
func bitgetChangePercent(t *BitgetTicker) float64 {
	return bitgetFloat(t.Change24h) * 100
}

// ===== K 线 =====

// GetKLine 获取合约 K 线，形态与项目其它数据源一致（复用 KLineData）。
// klt 取项目内标识（1/5/15/30/60/120/240/101/102/103/104/105/106）。
func (b *BitgetFuturesApi) GetKLine(symbol, klt string, limit int, end string) *[]KLineData {
	result := &[]KLineData{}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	interval := bitgetIntervalFromKlt(klt)
	endMs := bitgetEndTimeMs(end)

	params := bitgetBaseParams(
		"symbol", strings.ToUpper(symbol),
		"granularity", interval,
		"limit", strconv.Itoa(limit),
	)
	if endMs > 0 {
		params.Set("endTime", strconv.FormatInt(endMs, 10))
	}

	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("kline:%s:%s:%d:%d", strings.ToUpper(symbol), interval, limit, endMs)
	data, err := bitgetFetch(ctx, key, bitgetTTLKline, "/candles", params)
	if err != nil {
		logger.SugaredLogger.Warnf("获取 Bitget K线失败: symbol=%s granularity=%s err=%v", symbol, interval, err)
		return result
	}

	var rows [][]json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		logger.SugaredLogger.Warnf("解析 Bitget K线失败: %v", err)
		return result
	}

	out := make([]KLineData, 0, len(rows))
	for i, row := range rows {
		if len(row) < 7 {
			continue
		}
		openTime := bitgetRawInt(row[0])
		open := bitgetTrimNum(bitgetRawStr(row[1]))
		high := bitgetTrimNum(bitgetRawStr(row[2]))
		low := bitgetTrimNum(bitgetRawStr(row[3]))
		closeP := bitgetTrimNum(bitgetRawStr(row[4]))
		volume := bitgetTrimNum(bitgetRawStr(row[5]))
		amount := bitgetTrimNum(bitgetRawStr(row[6]))

		kd := KLineData{
			Day:    bitgetFormatDay(openTime, interval),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeP,
			Volume: volume,
			Amount: amount,
		}
		if i > 0 {
			prevClose := bitgetFloat(bitgetRawStr(rows[i-1][4]))
			curClose := bitgetFloat(bitgetRawStr(row[4]))
			curHigh := bitgetFloat(bitgetRawStr(row[2]))
			curLow := bitgetFloat(bitgetRawStr(row[3]))
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = bitgetTrimNum(strconv.FormatFloat(curClose-prevClose, 'f', -1, 64))
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		out = append(out, kd)
	}
	*result = out
	return result
}

func bitgetRawStr(r json.RawMessage) string {
	s := strings.TrimSpace(string(r))
	return strings.Trim(s, `"`)
}

func bitgetRawInt(r json.RawMessage) int64 {
	n, _ := strconv.ParseInt(bitgetRawStr(r), 10, 64)
	return n
}

// ===== 资金费率 =====

// BitgetFundingNow 当期资金费率与结算信息。
type BitgetFundingNow struct {
	Symbol             string  `json:"symbol"`
	FundingRate        float64 `json:"fundingRate"`
	FundingRateInterval int    `json:"fundingRateInterval"`
	NextUpdate          int64   `json:"nextUpdate"`
}

// GetFundingNow 获取当期资金费率（/current-fund-rate）。
func (b *BitgetFuturesApi) GetFundingNow(symbol string) *BitgetFundingNow {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	data, err := bitgetFetch(ctx, "fundnow:"+sym, bitgetTTLFunding, "/current-fund-rate",
		bitgetBaseParams("symbol", sym))
	if err != nil {
		if !isBitgetEmptyData(err) {
			logger.SugaredLogger.Warnf("获取 Bitget 当期资金费率失败: symbol=%s err=%v", sym, err)
		}
		return nil
	}
	var raw struct {
		Symbol              string `json:"symbol"`
		FundingRate         string `json:"fundingRate"`
		FundingRateInterval string `json:"fundingRateInterval"`
		NextUpdate          string `json:"nextUpdate"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	n, _ := strconv.Atoi(raw.FundingRateInterval)
	nu, _ := strconv.ParseInt(raw.NextUpdate, 10, 64)
	return &BitgetFundingNow{
		Symbol:              raw.Symbol,
		FundingRate:         bitgetFloat(raw.FundingRate),
		FundingRateInterval: n,
		NextUpdate:          nu,
	}
}

// GetFundingTime 获取结算周期与下次结算时间（/funding-time）。
// 返回 (nextFundingTime 毫秒, ratePeriod 小时)。
func (b *BitgetFuturesApi) GetFundingTime(symbol string) (int64, int) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	data, err := bitgetFetch(ctx, "fundtime:"+sym, bitgetTTLFundingTime, "/funding-time",
		bitgetBaseParams("symbol", sym))
	if err != nil {
		if !isBitgetEmptyData(err) {
			logger.SugaredLogger.Warnf("获取 Bitget 结算周期失败: symbol=%s err=%v", sym, err)
		}
		return 0, 0
	}
	var raw struct {
		NextFundingTime string `json:"nextFundingTime"`
		RatePeriod      string `json:"ratePeriod"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, 0
	}
	nft, _ := strconv.ParseInt(raw.NextFundingTime, 10, 64)
	rp, _ := strconv.Atoi(raw.RatePeriod)
	return nft, rp
}

// GetFundingRateHistory 获取资金费率历史，按时间升序返回。
func (b *BitgetFuturesApi) GetFundingRateHistory(symbol string, limit int) []BitgetFundingRate {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100 // Bitget 单页上限
	}
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("fundhist:%s:%d", sym, limit)
	data, err := bitgetFetch(ctx, key, bitgetTTLFundingHist, "/history-fund-rate",
		bitgetBaseParams("symbol", sym, "pageSize", strconv.Itoa(limit)))
	if err != nil {
		if !isBitgetEmptyData(err) {
			logger.SugaredLogger.Warnf("获取 Bitget 资金费率历史失败: symbol=%s err=%v", sym, err)
		}
		return nil
	}
	var list []BitgetFundingRate
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	// 接口返回新→旧，反转为时间升序
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list
}

// ===== 未平仓量 =====

// GetOpenInterest 获取当前未平仓量（/open-interest）。Bitget 无 OI 历史端点。
func (b *BitgetFuturesApi) GetOpenInterest(symbol string) float64 {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()
	data, err := bitgetFetch(ctx, "oi:"+sym, bitgetTTLOpenInterest, "/open-interest",
		bitgetBaseParams("symbol", sym))
	if err != nil {
		if !isBitgetEmptyData(err) {
			logger.SugaredLogger.Warnf("获取 Bitget 未平仓量失败: symbol=%s err=%v", sym, err)
		}
		return 0
	}
	var raw struct {
		OpenInterestList []struct {
			Symbol string `json:"symbol"`
			Size   string `json:"size"`
		} `json:"openInterestList"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0
	}
	for _, it := range raw.OpenInterestList {
		if strings.EqualFold(it.Symbol, sym) {
			return bitgetFloat(it.Size)
		}
	}
	if len(raw.OpenInterestList) > 0 {
		return bitgetFloat(raw.OpenInterestList[0].Size)
	}
	return 0
}

// ===== 衍生指标汇总 =====

// GetDerivatives 汇总单个合约的衍生指标（实时值；Bitget 美股永续无多空比、无 OI 历史）。
func (b *BitgetFuturesApi) GetDerivatives(symbol string) *BitgetDerivativesBundle {
	resolved, ok := ResolveBitgetSymbol(symbol)
	if !ok {
		return nil
	}

	bundle := &BitgetDerivativesBundle{
		Symbol: resolved,
		Name:   BitgetSymbolName(resolved),
	}

	ticker := b.GetTicker(resolved)
	if ticker == nil {
		return nil
	}
	bundle.LastPrice = bitgetFloat(ticker.LastPr)
	bundle.MarkPrice = bitgetFloat(ticker.MarkPrice)
	bundle.IndexPrice = bitgetFloat(ticker.IndexPrice)
	bundle.FundingRate = bitgetFloat(ticker.FundingRate)
	if bundle.IndexPrice > 0 {
		bundle.Basis = (bundle.MarkPrice - bundle.IndexPrice) / bundle.IndexPrice * 100
	}

	// 结算周期与下次结算时间（若 funding-time 不可用，回落到 ticker 的 fundingRate 上下文）
	if nft, rp := b.GetFundingTime(resolved); nft > 0 || rp > 0 {
		bundle.NextFundingTime = nft
		bundle.RatePeriod = rp
	} else if now := b.GetFundingNow(resolved); now != nil {
		bundle.NextFundingTime = now.NextUpdate
		bundle.RatePeriod = now.FundingRateInterval
	}
	if bundle.RatePeriod == 0 {
		bundle.RatePeriod = 8
	}
	// 年化 = 费率 * (365*24/周期小时)
	bundle.AnnualizedRate = bundle.FundingRate * (365 * 24 / float64(bundle.RatePeriod)) * 100

	bundle.OpenInterest = b.GetOpenInterest(resolved)
	if bundle.LastPrice > 0 {
		bundle.OpenInterestUSD = bundle.OpenInterest * bundle.LastPrice
	}

	bundle.FundingRateHistory = b.GetFundingRateHistory(resolved, 24)
	return bundle
}

// ===== 与既有分发对接的封装 =====

// Bitget 数据源标识。前端据此区分「正常」与两类失败，避免静默返回空数据。
const (
	bitgetSourceOK            = "bitget-futures"
	bitgetSourceUnreachable   = "bitget-futures-unreachable"
	bitgetSourceInvalidSymbol = "bitget-futures-invalid-symbol"
)

// fetchFromBitgetFutures 供 FetchKLineWithFallback 调用，Source 标记实际结果。
func fetchFromBitgetFutures(stockCode, klt string, limit int, end string) *KLineSourceResult {
	symbol, ok := ResolveBitgetSymbol(stockCode)
	if !ok {
		return &KLineSourceResult{Data: &[]KLineData{}, Source: bitgetSourceInvalidSymbol}
	}
	data := NewBitgetFuturesApi().GetKLine(symbol, klt, limit, end)
	if data == nil || len(*data) == 0 {
		return &KLineSourceResult{Data: &[]KLineData{}, Source: bitgetSourceUnreachable}
	}
	return &KLineSourceResult{Data: data, Source: bitgetSourceOK}
}

// BitgetMinutePriceData 供 GetStockMinutePriceData 调用，返回最近一日分时（1 分钟粒度）。
func BitgetMinutePriceData(stockCode string) *[]MinuteData {
	result := &[]MinuteData{}
	symbol, ok := ResolveBitgetSymbol(stockCode)
	if !ok {
		return result
	}
	data := NewBitgetFuturesApi().GetKLine(symbol, "1", 1440, "")
	if data == nil {
		return result
	}
	out := make([]MinuteData, 0, len(*data))
	for _, k := range *data {
		hhmm := k.Day
		if idx := strings.Index(k.Day, " "); idx >= 0 {
			hhmm = k.Day[idx+1:]
		}
		out = append(out, MinuteData{
			Time:   hhmm,
			Price:  bitgetFloat(k.Close),
			Volume: bitgetFloat(k.Volume),
			Amount: bitgetFloat(k.Amount),
		})
	}
	*result = out
	return result
}

// BitgetRealtimeStockInfo 供 GetStockCodeRealTimeData 调用，把 24h 行情映射为 StockInfo。
func BitgetRealtimeStockInfo(stockCode string) *StockInfo {
	symbol, ok := ResolveBitgetSymbol(stockCode)
	if !ok {
		return nil
	}
	ticker := NewBitgetFuturesApi().GetTicker(symbol)
	if ticker == nil {
		return nil
	}
	info := &StockInfo{
		Code:     bitgetCodePrefix + strings.ToLower(symbol),
		Name:     BitgetSymbolName(symbol),
		Price:    bitgetTrimNum(ticker.LastPr),
		Open:     bitgetTrimNum(ticker.Open24h),
		High:     bitgetTrimNum(ticker.High24h),
		Low:      bitgetTrimNum(ticker.Low24h),
		Volume:   bitgetTrimNum(ticker.BaseVolume),
		Amount:   bitgetTrimNum(ticker.QuoteVolume),
		Market:   "Bitget美股永续",
		Date:     time.Now().In(bitgetCST).Format("2006-01-02"),
		Time:     time.Now().In(bitgetCST).Format("15:04:05"),
	}
	info.ChangePercent = bitgetChangePercent(ticker)
	info.ChangePrice = bitgetFloat(ticker.LastPr) - bitgetFloat(ticker.Open24h)
	info.PrePrice = bitgetFloat(ticker.Open24h)
	info.PreClose = bitgetTrimNum(ticker.Open24h)

	info.B1P = bitgetTrimNum(ticker.BidPr)
	info.B1V = bitgetTrimNum(ticker.BidSz)
	info.A1P = bitgetTrimNum(ticker.AskPr)
	info.A1V = bitgetTrimNum(ticker.AskSz)
	info.Bid = info.B1P
	info.Ask = info.A1P
	return info
}

// BitgetHotStock 返回美股永续合约热门榜单（供 HotStock 的 marketType 分支复用）。
// sortBy："percent" 按 24h 涨跌幅降序、"amount" 按 24h 成交额降序、"funding" 按当期资金费率绝对值降序。
func BitgetHotStock(size int, sortBy string) *[]models.HotItem {
	result := &[]models.HotItem{}
	api := NewBitgetFuturesApi()
	tickers := api.GetAllTickers()
	if len(tickers) == 0 {
		return result
	}
	if size <= 0 || size > len(tickers) {
		size = len(tickers)
	}

	type rankRow struct {
		item       models.HotItem
		amount     float64
		fundingAbs float64
	}
	rows := make([]rankRow, 0, len(tickers))
	for _, t := range tickers {
		sym := strings.ToUpper(t.Symbol)
		rate := bitgetFloat(t.FundingRate)
		rows = append(rows, rankRow{
			item: models.HotItem{
				Code:     bitgetCodePrefix + strings.ToLower(sym),
				Name:     BitgetSymbolName(sym),
				Value:    bitgetFloat(t.QuoteVolume),
				Percent:  bitgetChangePercent(&t),
				Current:  bitgetFloat(t.LastPr),
				Chg:      bitgetFloat(t.LastPr) - bitgetFloat(t.Open24h),
				Exchange: "BITGET",
			},
			amount:     bitgetFloat(t.QuoteVolume),
			fundingAbs: math.Abs(rate),
		})
	}

	switch sortBy {
	case "amount":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].amount > rows[j].amount })
	case "funding":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].fundingAbs > rows[j].fundingAbs })
	default:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].item.Percent > rows[j].item.Percent })
	}

	if size > len(rows) {
		size = len(rows)
	}
	items := make([]models.HotItem, 0, size)
	for _, r := range rows[:size] {
		items = append(items, r.item)
	}
	*result = items
	return result
}