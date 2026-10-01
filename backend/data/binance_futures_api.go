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

// 本文件是币安 USDT-M 永续合约各 REST 端点的门面（facade），
// 统一走 binance_futures_client.go 的直连/代理双通道 + TTL 缓存。
// 行情形态复用项目既有的 KLineData / MinuteData / StockInfo，
// 永续衍生指标走 BinanceDerivativesBundle。

// binanceCST 币安 K 线以 UTC 为界，展示时统一转 UTC+8，与 A 股/港股用户习惯一致。
var binanceCST = time.FixedZone("CST", 8*60*60)

// 各端点缓存时长。行情类取较短 TTL 兼顾新鲜度与币安 weight 配额；
// 历史类数据变化慢，TTL 放宽以减少请求。
const (
	binanceTTLTicker      = 3 * time.Second
	binanceTTLBook        = 3 * time.Second
	binanceTTLPremium     = 10 * time.Second
	binanceTTLKline       = 3 * time.Second
	binanceTTLOpenInterest = 10 * time.Second
	binanceTTLFundingHist = 60 * time.Second
	binanceTTLOIHist      = 60 * time.Second
	binanceTTLRatioHist   = 60 * time.Second
)

// ===== 端点响应 DTO（仅用于解析，与业务结构体解耦）=====

// BinanceTicker24h /fapi/v1/ticker/24hr 单个合约的 24 小时统计。
type BinanceTicker24h struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	OpenPrice          string `json:"openPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	OpenTime           int64  `json:"openTime"`
	CloseTime          int64  `json:"closeTime"`
	Count              int64  `json:"count"`
}

// BinanceBookTicker /fapi/v1/ticker/bookTicker 最优挂单（一档买卖）。
type BinanceBookTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	BidQty   string `json:"bidQty"`
	AskPrice string `json:"askPrice"`
	AskQty   string `json:"askQty"`
	Time     int64  `json:"time"`
}

// BinancePremiumIndex /fapi/v1/premiumIndex 标记价、指数价与当期资金费率。
type BinancePremiumIndex struct {
	Symbol          string `json:"symbol"`
	MarkPrice       string `json:"markPrice"`
	IndexPrice      string `json:"indexPrice"`
	LastFundingRate string `json:"lastFundingRate"`
	NextFundingTime int64  `json:"nextFundingTime"`
	Time            int64  `json:"time"`
}

// BinanceOpenInterest /fapi/v1/openInterest 当前未平仓量。
type BinanceOpenInterest struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"`
	Time         int64  `json:"time"`
}

// BinanceFuturesApi 币安 USDT-M 永续合约数据源门面。
type BinanceFuturesApi struct{}

// NewBinanceFuturesApi 创建币安永续合约数据源。
func NewBinanceFuturesApi() *BinanceFuturesApi {
	return &BinanceFuturesApi{}
}

// ===== 数值/参数辅助 =====

// binanceTrimNum 把币安返回的高精度小数字符串归一为最简形式，
// 如 "4.00000200"→"4.000002"，"0.00010000"→"0.0001"，避免图表标签出现冗余尾零。
func binanceTrimNum(s string) string {
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

func binanceFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func binanceQuery(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			v.Set(kv[i], kv[i+1])
		}
	}
	return v
}

// binanceEndTimeMs 把外部传入的 end 解析为毫秒时间戳；无法识别时返回 0（表示取最近数据）。
func binanceEndTimeMs(end string) int64 {
	raw := strings.TrimSpace(end)
	if raw == "" {
		return 0
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if n > 1e12 { // 毫秒
			return n
		}
		return n * 1000 // 秒
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02", "2006-01-02", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.ParseInLocation(layout, raw, binanceCST); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

// ===== K 线 =====

// GetKLine 获取合约 K 线，形态与项目其它数据源一致（复用 KLineData）。
// klt 取项目内标识（1/5/15/30/60/120/240/101/102/103/104/105/106）。
// end 为可选结束时间（日期/时间/时间戳），为空则取最近数据。
func (b *BinanceFuturesApi) GetKLine(symbol, klt string, limit int, end string) *[]KLineData {
	result := &[]KLineData{}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1500 {
		limit = 1500
	}
	interval := binanceIntervalFromKlt(klt)
	endMs := binanceEndTimeMs(end)

	params := binanceQuery(
		"symbol", strings.ToUpper(symbol),
		"interval", interval,
		"limit", strconv.Itoa(limit),
	)
	if endMs > 0 {
		params.Set("endTime", strconv.FormatInt(endMs, 10))
	}

	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("kline:%s:%s:%d:%d", strings.ToUpper(symbol), interval, limit, endMs)
	body, err := binanceCachedRequest(ctx, key, binanceTTLKline, "/fapi/v1/klines", params)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安K线失败: symbol=%s interval=%s err=%v", symbol, interval, err)
		return result
	}

	var rows [][]json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		logger.SugaredLogger.Warnf("解析币安K线失败: %v", err)
		return result
	}

	out := make([]KLineData, 0, len(rows))
	for i, row := range rows {
		if len(row) < 7 {
			continue
		}
		openTime := binanceRawInt(row[0])
		open := binanceTrimNum(binanceRawStr(row[1]))
		high := binanceTrimNum(binanceRawStr(row[2]))
		low := binanceTrimNum(binanceRawStr(row[3]))
		closeP := binanceTrimNum(binanceRawStr(row[4]))
		volume := binanceTrimNum(binanceRawStr(row[5]))
		amount := binanceTrimNum(binanceRawStr(row[7]))

		kd := KLineData{
			Day:    binanceFormatDayCST(openTime, interval),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeP,
			Volume: volume,
			Amount: amount,
		}
		if i > 0 {
			prevClose := binanceFloat(binanceRawStr(rows[i-1][4]))
			curClose := binanceFloat(binanceRawStr(row[4]))
			curHigh := binanceFloat(binanceRawStr(row[2]))
			curLow := binanceFloat(binanceRawStr(row[3]))
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = binanceTrimNum(strconv.FormatFloat(curClose-prevClose, 'f', -1, 64))
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		out = append(out, kd)
	}
	*result = out
	return result
}

// binanceFormatDayCST 把开盘时间（毫秒）按 UTC+8 格式化为 KLineData.Day。
func binanceFormatDayCST(openTimeMs int64, interval string) string {
	t := time.UnixMilli(openTimeMs).In(binanceCST)
	if strings.HasSuffix(interval, "m") || strings.HasSuffix(interval, "h") {
		return t.Format("2006-01-02 15:04")
	}
	return t.Format("2006-01-02")
}

func binanceRawStr(r json.RawMessage) string {
	s := strings.TrimSpace(string(r))
	s = strings.Trim(s, `"`)
	return s
}

func binanceRawInt(r json.RawMessage) int64 {
	n, _ := strconv.ParseInt(binanceRawStr(r), 10, 64)
	return n
}

// ===== 实时行情 =====

// GetTicker24h 获取单个合约 24 小时行情。
func (b *BinanceFuturesApi) GetTicker24h(symbol string) *BinanceTicker24h {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "ticker24h:"+sym, binanceTTLTicker,
		"/fapi/v1/ticker/24hr", binanceQuery("symbol", sym))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安24h行情失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var t BinanceTicker24h
	if err := json.Unmarshal(body, &t); err != nil {
		logger.SugaredLogger.Warnf("解析币安24h行情失败: %v", err)
		return nil
	}
	return &t
}

// GetAllTickers 获取全部 USDT-M 永续合约 24 小时行情（按 exchangeInfo 过滤交割合约）。
func (b *BinanceFuturesApi) GetAllTickers() []BinanceTicker24h {
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "ticker24h:all", binanceTTLTicker, "/fapi/v1/ticker/24hr", nil)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安全部24h行情失败: %v", err)
		return nil
	}
	var list []BinanceTicker24h
	if err := json.Unmarshal(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析币安全部24h行情失败: %v", err)
		return nil
	}
	if info := GetBinanceExchangeInfo(); info != nil && len(info.SymbolMap) > 0 {
		filtered := make([]BinanceTicker24h, 0, len(list))
		for _, t := range list {
			if _, ok := info.SymbolMap[strings.ToUpper(t.Symbol)]; ok {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}
	return list
}

// GetBookTicker 获取单个合约最优挂单（一档买卖）。
func (b *BinanceFuturesApi) GetBookTicker(symbol string) *BinanceBookTicker {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "book:"+sym, binanceTTLBook,
		"/fapi/v1/ticker/bookTicker", binanceQuery("symbol", sym))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安盘口失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var bt BinanceBookTicker
	if err := json.Unmarshal(body, &bt); err != nil {
		logger.SugaredLogger.Warnf("解析币安盘口失败: %v", err)
		return nil
	}
	return &bt
}

// ===== 衍生指标端点 =====

// GetPremiumIndex 获取标记价/指数价/当期资金费率/下次结算时间。
func (b *BinanceFuturesApi) GetPremiumIndex(symbol string) *BinancePremiumIndex {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "premium:"+sym, binanceTTLPremium,
		"/fapi/v1/premiumIndex", binanceQuery("symbol", sym))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安标记价失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var p BinancePremiumIndex
	if err := json.Unmarshal(body, &p); err != nil {
		logger.SugaredLogger.Warnf("解析币安标记价失败: %v", err)
		return nil
	}
	return &p
}

// GetAllPremiumIndex 获取全部合约的标记价与当期资金费率（单一请求，weight 低）。
func (b *BinanceFuturesApi) GetAllPremiumIndex() []BinancePremiumIndex {
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "premium:all", binanceTTLPremium, "/fapi/v1/premiumIndex", nil)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安全量标记价失败: %v", err)
		return nil
	}
	var list []BinancePremiumIndex
	if err := json.Unmarshal(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析币安全量标记价失败: %v", err)
		return nil
	}
	if info := GetBinanceExchangeInfo(); info != nil && len(info.SymbolMap) > 0 {
		filtered := make([]BinancePremiumIndex, 0, len(list))
		for _, p := range list {
			if _, ok := info.SymbolMap[strings.ToUpper(p.Symbol)]; ok {
				filtered = append(filtered, p)
			}
		}
		return filtered
	}
	return list
}

// GetFundingRateHistory 获取资金费率历史，按时间升序返回。
func (b *BinanceFuturesApi) GetFundingRateHistory(symbol string, limit int) []BinanceFundingRate {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	if limit <= 0 {
		limit = 24
	}
	if limit > 1000 {
		limit = 1000
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("funding:%s:%d", sym, limit)
	body, err := binanceCachedRequest(ctx, key, binanceTTLFundingHist, "/fapi/v1/fundingRate",
		binanceQuery("symbol", sym, "limit", strconv.Itoa(limit)))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安资金费率历史失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var list []BinanceFundingRate
	if err := json.Unmarshal(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析币安资金费率历史失败: %v", err)
		return nil
	}
	return list
}

// GetOpenInterest 获取当前未平仓量。
func (b *BinanceFuturesApi) GetOpenInterest(symbol string) *BinanceOpenInterest {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	body, err := binanceCachedRequest(ctx, "oi:"+sym, binanceTTLOpenInterest,
		"/fapi/v1/openInterest", binanceQuery("symbol", sym))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安未平仓量失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var oi BinanceOpenInterest
	if err := json.Unmarshal(body, &oi); err != nil {
		logger.SugaredLogger.Warnf("解析币安未平仓量失败: %v", err)
		return nil
	}
	return &oi
}

// binanceHistPeriod 校验并归一 futures/data 系列端点的 period 参数。
func binanceHistPeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d":
		return strings.ToLower(strings.TrimSpace(period))
	default:
		return "1h"
	}
}

// GetOpenInterestHist 获取未平仓量历史序列（币数）。
func (b *BinanceFuturesApi) GetOpenInterestHist(symbol, period string, limit int) []BinanceOpenInterestHist {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	period = binanceHistPeriod(period)
	if limit <= 0 {
		limit = 48
	}
	if limit > 500 {
		limit = 500
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("oihist:%s:%s:%d", sym, period, limit)
	body, err := binanceCachedRequest(ctx, key, binanceTTLOIHist, "/futures/data/openInterestHist",
		binanceQuery("symbol", sym, "period", period, "limit", strconv.Itoa(limit)))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安未平仓量历史失败: symbol=%s err=%v", sym, err)
		return nil
	}
	var list []BinanceOpenInterestHist
	if err := json.Unmarshal(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析币安未平仓量历史失败: %v", err)
		return nil
	}
	return list
}

// GetLongShortAccountRatio 获取全局多空账户数比历史。
func (b *BinanceFuturesApi) GetLongShortAccountRatio(symbol, period string, limit int) []BinanceLongShortRatio {
	return b.fetchLongShortRatio("/futures/data/globalLongShortAccountRatio", symbol, period, limit)
}

// GetTopLongShortPositionRatio 获取大户持仓多空比历史。
func (b *BinanceFuturesApi) GetTopLongShortPositionRatio(symbol, period string, limit int) []BinanceLongShortRatio {
	return b.fetchLongShortRatio("/futures/data/topLongShortPositionRatio", symbol, period, limit)
}

// GetTakerLongShortRatio 获取主动买卖量比历史。
func (b *BinanceFuturesApi) GetTakerLongShortRatio(symbol, period string, limit int) []BinanceLongShortRatio {
	return b.fetchLongShortRatio("/futures/data/takerlongshortRatio", symbol, period, limit)
}

func (b *BinanceFuturesApi) fetchLongShortRatio(path, symbol, period string, limit int) []BinanceLongShortRatio {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil
	}
	period = binanceHistPeriod(period)
	if limit <= 0 {
		limit = 48
	}
	if limit > 500 {
		limit = 500
	}
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()
	key := fmt.Sprintf("ls:%s:%s:%s:%d", path, sym, period, limit)
	body, err := binanceCachedRequest(ctx, key, binanceTTLRatioHist, path,
		binanceQuery("symbol", sym, "period", period, "limit", strconv.Itoa(limit)))
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安多空比失败: path=%s symbol=%s err=%v", path, sym, err)
		return nil
	}
	var list []BinanceLongShortRatio
	if err := json.Unmarshal(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析币安多空比失败: %v", err)
		return nil
	}
	return list
}

// GetDerivatives 汇总单个合约的衍生指标（实时 + 历史序列）。
// periods 控制历史序列长度，period 为历史采样周期（默认 1h）。
func (b *BinanceFuturesApi) GetDerivatives(symbol, period string, periods int) *BinanceDerivativesBundle {
	resolved, ok := ResolveBinanceSymbol(symbol)
	if !ok {
		return nil
	}
	if periods <= 0 {
		periods = 48
	}
	if periods > 500 {
		periods = 500
	}

	bundle := &BinanceDerivativesBundle{
		Symbol: resolved,
		Name:   SymbolName(resolved),
	}

	if ticker := b.GetTicker24h(resolved); ticker != nil {
		bundle.LastPrice = binanceFloat(ticker.LastPrice)
	}
	if premium := b.GetPremiumIndex(resolved); premium != nil {
		bundle.MarkPrice = binanceFloat(premium.MarkPrice)
		bundle.IndexPrice = binanceFloat(premium.IndexPrice)
		bundle.LastFundingRate = binanceFloat(premium.LastFundingRate)
		bundle.NextFundingTime = premium.NextFundingTime
		if bundle.IndexPrice > 0 {
			bundle.Basis = (bundle.MarkPrice - bundle.IndexPrice) / bundle.IndexPrice * 100
		}
		// 资金费率按 8 小时一期，年化 = 费率 * 3 * 365
		bundle.AnnualizedRate = bundle.LastFundingRate * 3 * 365 * 100
	}

	if oi := b.GetOpenInterest(resolved); oi != nil {
		bundle.OpenInterest = binanceFloat(oi.OpenInterest)
	}
	if oiHist := b.GetOpenInterestHist(resolved, period, periods); len(oiHist) > 0 {
		points := make([]BinanceDerivativesPoint, 0, len(oiHist))
		for _, p := range oiHist {
			points = append(points, BinanceDerivativesPoint{
				Timestamp: p.Timestamp,
				Value:     binanceFloat(p.SumOpenInterest),
			})
		}
		bundle.OpenInterestHistory = points
		// 用最新一点的未平仓名义价值兜底 OpenInterestValue
		last := oiHist[len(oiHist)-1]
		bundle.OpenInterestValue = binanceFloat(last.SumOpenInterestValue)
		if len(points) >= 2 {
			first := points[0].Value
			if first > 0 {
				bundle.OpenInterestChange = (points[len(points)-1].Value - first) / first * 100
			}
		}
	}

	if ratios := b.GetLongShortAccountRatio(resolved, period, periods); len(ratios) > 0 {
		points := make([]BinanceDerivativesPoint, 0, len(ratios))
		for _, r := range ratios {
			points = append(points, BinanceDerivativesPoint{
				Timestamp: r.Timestamp,
				Value:     binanceFloat(r.LongShortRatio),
			})
		}
		bundle.LongShortHistory = points
		last := ratios[len(ratios)-1]
		bundle.LongShortRatio = binanceFloat(last.LongShortRatio)
		bundle.LongAccount = binanceFloat(last.LongAccount)
		bundle.ShortAccount = binanceFloat(last.ShortAccount)
	}

	if takers := b.GetTakerLongShortRatio(resolved, period, periods); len(takers) > 0 {
		bundle.TakerBuySellRatio = binanceFloat(takers[len(takers)-1].BuySellRatio)
	}

	bundle.FundingRateHistory = b.GetFundingRateHistory(resolved, periods)
	return bundle
}

// ===== 与既有分发对接的封装 =====

// 币安数据源标识。前端据此区分「正常」与两类失败，避免静默返回空数据。
const (
	binanceSourceOK            = "binance-futures"
	binanceSourceUnreachable   = "binance-futures-unreachable"    // 接口不可达（国内网络未配代理）
	binanceSourceInvalidSymbol = "binance-futures-invalid-symbol" // 合约标识无效
)

// fetchFromBinanceFutures 供 FetchKLineWithFallback 调用，Source 标记实际结果。
func fetchFromBinanceFutures(stockCode, klt string, limit int, end string) *KLineSourceResult {
	symbol, ok := ResolveBinanceSymbol(stockCode)
	if !ok {
		return &KLineSourceResult{Data: &[]KLineData{}, Source: binanceSourceInvalidSymbol}
	}
	data := NewBinanceFuturesApi().GetKLine(symbol, klt, limit, end)
	if data == nil || len(*data) == 0 {
		return &KLineSourceResult{Data: &[]KLineData{}, Source: binanceSourceUnreachable}
	}
	return &KLineSourceResult{Data: data, Source: binanceSourceOK}
}

// BinanceMinutePriceData 供 GetStockMinutePriceData 调用，返回最近一日分时（1 分钟粒度）。
func BinanceMinutePriceData(stockCode string) *[]MinuteData {
	result := &[]MinuteData{}
	symbol, ok := ResolveBinanceSymbol(stockCode)
	if !ok {
		return result
	}
	data := NewBinanceFuturesApi().GetKLine(symbol, "1", 1440, "")
	if data == nil {
		return result
	}
	out := make([]MinuteData, 0, len(*data))
	for _, k := range *data {
		// Day 形如 "2026-10-01 14:30"，取时间部分作为分时 X 轴
		hhmm := k.Day
		if idx := strings.Index(k.Day, " "); idx >= 0 {
			hhmm = k.Day[idx+1:]
		}
		out = append(out, MinuteData{
			Time:   hhmm,
			Price:  binanceFloat(k.Close),
			Volume: binanceFloat(k.Volume),
			Amount: binanceFloat(k.Amount),
		})
	}
	*result = out
	return result
}

// BinanceRealtimeStockInfo 供 GetStockCodeRealTimeData 调用，把 24h 行情+一档盘口映射为 StockInfo。
func BinanceRealtimeStockInfo(stockCode string) *StockInfo {
	symbol, ok := ResolveBinanceSymbol(stockCode)
	if !ok {
		return nil
	}
	api := NewBinanceFuturesApi()
	ticker := api.GetTicker24h(symbol)
	if ticker == nil {
		return nil
	}
	info := &StockInfo{
		Code:     binanceCodePrefix + strings.ToLower(symbol),
		Name:     SymbolName(symbol),
		Price:    binanceTrimNum(ticker.LastPrice),
		Open:     binanceTrimNum(ticker.OpenPrice),
		High:     binanceTrimNum(ticker.HighPrice),
		Low:      binanceTrimNum(ticker.LowPrice),
		Volume:   binanceTrimNum(ticker.Volume),
		Amount:   binanceTrimNum(ticker.QuoteVolume),
		Market:   "币安永续",
		Date:     time.Now().In(binanceCST).Format("2006-01-02"),
		Time:     time.Now().In(binanceCST).Format("15:04:05"),
	}
	// 24h 涨幅/涨跌额由币安直接给出
	info.ChangePercent = binanceFloat(ticker.PriceChangePercent)
	info.ChangePrice = binanceFloat(ticker.PriceChange)
	// 24h 统计窗口的「开盘价」等价于昨收基准，供其它模块复用
	info.PrePrice = binanceFloat(ticker.OpenPrice)
	info.PreClose = binanceTrimNum(ticker.OpenPrice)

	if book := api.GetBookTicker(symbol); book != nil {
		info.B1P = binanceTrimNum(book.BidPrice)
		info.B1V = binanceTrimNum(book.BidQty)
		info.A1P = binanceTrimNum(book.AskPrice)
		info.A1V = binanceTrimNum(book.AskQty)
		info.Bid = info.B1P
		info.Ask = info.A1P
	}
	return info
}

// BinanceSymbolList 返回全部 USDT-M 永续合约基础信息（品种列表用）。
func BinanceSymbolList() []BinanceSymbolInfo {
	info := GetBinanceExchangeInfo()
	if info == nil {
		return nil
	}
	return info.Symbols
}

// BinanceHotStock 返回币安永续合约热门榜单（供 HotStock 的 marketType 分支复用）。
// sortBy："percent" 按 24h 涨跌幅降序、"amount" 按 24h 成交额降序、"funding" 按当期资金费率绝对值降序。
func BinanceHotStock(size int, sortBy string) *[]models.HotItem {
	result := &[]models.HotItem{}
	api := NewBinanceFuturesApi()
	tickers := api.GetAllTickers()
	if len(tickers) == 0 {
		return result
	}
	if size <= 0 || size > len(tickers) {
		size = len(tickers)
	}

	funding := map[string]float64{}
	for _, p := range api.GetAllPremiumIndex() {
		funding[strings.ToUpper(p.Symbol)] = binanceFloat(p.LastFundingRate)
	}

	type rankRow struct {
		item       models.HotItem
		amount     float64
		fundingAbs float64
	}
	rows := make([]rankRow, 0, len(tickers))
	for _, t := range tickers {
		sym := strings.ToUpper(t.Symbol)
		rate := funding[sym]
		rows = append(rows, rankRow{
			item: models.HotItem{
				Code:     binanceCodePrefix + strings.ToLower(sym),
				Name:     SymbolName(sym),
				Value:    binanceFloat(t.QuoteVolume),
				Percent:  binanceFloat(t.PriceChangePercent),
				Current:  binanceFloat(t.LastPrice),
				Chg:      binanceFloat(t.PriceChange),
				Exchange: "BINANCE",
			},
			amount:     binanceFloat(t.QuoteVolume),
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