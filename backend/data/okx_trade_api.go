package data

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// OKX 交易数据源门面：公共行情（免签）+ 私有账户/交易接口（HMAC 签名）。
// 支持 SPOT（币币现货）与 SWAP（USDT 永续合约）两类品种。

// ===== 类型 =====

// OKXInstrument /public/instruments 合约/交易对规格。
type OKXInstrument struct {
	InstType  string `json:"instType"` // SPOT / SWAP
	InstId    string `json:"instId"`   // BTC-USDT / BTC-USDT-SWAP
	BaseCcy   string `json:"baseCcy"`
	QuoteCcy  string `json:"quoteCcy"`
	SettleCcy string `json:"settleCcy"`
	CtVal     string `json:"ctVal"` // SWAP 一张合约对应的基础币数量
	CtValCcy  string `json:"ctValCcy"`
	MinSz     string `json:"minSz"` // 最小下单量
	LotSz     string `json:"lotSz"` // 下单量步进
	TickSz    string `json:"tickSz"`
	Lever     string `json:"lever"` // 最大可设杠杆（SWAP，如 "100"；SPOT 为空）
	State     string `json:"state"`
}

// OKXTicker /market/tickers 24h 行情。
type OKXTicker struct {
	InstId    string `json:"instId"`
	Last      string `json:"last"`
	AskPx     string `json:"askPx"`
	BidPx     string `json:"bidPx"`
	Open24h   string `json:"open24h"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	Vol24h    string `json:"vol24h"`    // 成交量（张/币）
	VolCcy24h string `json:"volCcy24h"` // 成交额（计价币）
	Ts        string `json:"ts"`
}

// OKXBalanceDetail 单币种余额。
type OKXBalanceDetail struct {
	Ccy       string `json:"ccy"`
	Eq        string `json:"eq"`        // 币种总权益（折 USD）
	AvailEq   string `json:"availEq"`   // 可用权益
	AvailBal  string `json:"availBal"`  // 可用余额
	FrozenBal string `json:"frozenBal"` // 冻结
	Imr       string `json:"imr"`       // 币种初始保证金（统一账户）
	Mmr       string `json:"mmr"`       // 币种维持保证金（统一账户，防爆仓风控用）
}

// OKXBalance 账户余额。
type OKXBalance struct {
	TotalEq string             `json:"totalEq"` // 总权益（USD）
	Details []OKXBalanceDetail `json:"details"`
}

// OKXAccountConfig /account/config 账户配置。
type OKXAccountConfig struct {
	UID     string `json:"uid"`
	AcctLv  string `json:"acctLv"`  // 账户等级
	PosMode string `json:"posMode"` // long_short_mode / net_mode
}

// OKXPosition /account/positions 持仓。
type OKXPosition struct {
	InstId      string `json:"instId"`
	PosSide     string `json:"posSide"` // long / short / net
	Pos         string `json:"pos"`     // 持仓数量（SWAP 为张）
	PosCcy      string `json:"posCcy"`
	AvgPx       string `json:"avgPx"`
	MarkPx      string `json:"markPx"`
	Upl         string `json:"upl"`      // 未实现盈亏
	UplRatio    string `json:"uplRatio"` // 未实现收益率（小数）
	Lever       string `json:"lever"`
	MgnMode     string `json:"mgnMode"` // cross / isolated
	Margin      string `json:"margin"`
	LiqPx       string `json:"liqPx"`
	NotionalUsd string `json:"notionalUsd"`
	MTime       string `json:"uTime"`
}

// OKXOrder /trade/orders-pending / /trade/orders-history 订单。
type OKXOrder struct {
	InstId     string `json:"instId"`
	OrdId      string `json:"ordId"`
	Side       string `json:"side"` // buy / sell
	PosSide    string `json:"posSide"`
	TdMode     string `json:"tdMode"`
	OrdType    string `json:"ordType"`
	Sz         string `json:"sz"`
	Px         string `json:"px"`
	State      string `json:"state"` // live / partially_filled / filled / canceled
	AvgPx      string `json:"avgPx"`
	AccFillSz  string `json:"accFillSz"`
	FillFee    string `json:"fillFee"`
	ReduceOnly string `json:"reduceOnly"`
	Source     string `json:"source"`
	CTime      string `json:"cTime"`
	UTime      string `json:"uTime"`
}

// OKXPlaceOrderResult /trade/order 下单/撤单结果。
type OKXPlaceOrderResult struct {
	OrdId   string `json:"ordId"`
	ClOrdId string `json:"clOrdId"`
	SCode   string `json:"sCode"`
	SMsg    string `json:"sMsg"`
}

// OKXOrderRequest 下单请求。
type OKXOrderRequest struct {
	InstId         string             `json:"instId"`
	TdMode         string             `json:"tdMode"`            // cash / cross / isolated
	Side           string             `json:"side"`              // buy / sell
	PosSide        string             `json:"posSide,omitempty"` // net / long / short
	OrdType        string             `json:"ordType"`           // limit / market
	Sz             string             `json:"sz"`
	Px             string             `json:"px,omitempty"`
	TgtCcy         string             `json:"tgtCcy,omitempty"`
	ReduceOnly     *bool              `json:"reduceOnly,omitempty"`
	AttachAlgoOrds []OKXAttachAlgoOrd `json:"attachAlgoOrds,omitempty"`
}

// OKXAttachAlgoOrd 附带止盈止损。
type OKXAttachAlgoOrd struct {
	SlTriggerPx string `json:"slTriggerPx,omitempty"`
	SlOrdPx     string `json:"slOrdPx,omitempty"` // -1 表示市价
	TpTriggerPx string `json:"tpTriggerPx,omitempty"`
	TpOrdPx     string `json:"tpOrdPx,omitempty"`
}

// ===== 合约规格缓存 =====

const okxInstrumentsTTL = 6 * time.Hour

type okxInstrumentsCache struct {
	mu        sync.RWMutex
	fetchedAt time.Time
	byType    map[string][]OKXInstrument // instType -> list
	byInstId  map[string]OKXInstrument   // instId -> spec
}

var okxInstruments okxInstrumentsCache

func okxInstrumentsSnapshot() (map[string][]OKXInstrument, map[string]OKXInstrument) {
	okxInstruments.mu.RLock()
	defer okxInstruments.mu.RUnlock()
	if okxInstruments.byType == nil || time.Since(okxInstruments.fetchedAt) >= okxInstrumentsTTL {
		return nil, nil
	}
	return okxInstruments.byType, okxInstruments.byInstId
}

// GetOKXInstruments 拉取并缓存合约/交易对规格（6 小时）；instType 为空拉 SPOT+SWAP 两类。
func GetOKXInstruments(instType string) []OKXInstrument {
	types := []string{}
	switch strings.ToUpper(strings.TrimSpace(instType)) {
	case "SPOT":
		types = []string{"SPOT"}
	case "SWAP":
		types = []string{"SWAP"}
	case "":
		types = []string{"SPOT", "SWAP"}
	default:
		types = []string{strings.ToUpper(strings.TrimSpace(instType))}
	}

	// 全部命中缓存时直接返回
	if len(types) == 2 {
		byType, _ := okxInstrumentsSnapshot()
		if byType != nil {
			out := make([]OKXInstrument, 0, len(byType["SPOT"])+len(byType["SWAP"]))
			out = append(out, byType["SPOT"]...)
			out = append(out, byType["SWAP"]...)
			return out
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()

	freshType := map[string][]OKXInstrument{}
	freshId := map[string]OKXInstrument{}
	for _, t := range types {
		params := url.Values{}
		params.Set("instType", t)
		body, err := okxCachedRequest(ctx, "instruments:"+t, okxInstrumentsTTL, http.MethodGet, "/api/v5/public/instruments", params)
		if err != nil {
			logger.SugaredLogger.Warnf("获取OKX合约规格失败: instType=%s err=%v", t, err)
			continue
		}
		var list []OKXInstrument
		if err := okxDataList(body, &list); err != nil {
			logger.SugaredLogger.Warnf("解析OKX合约规格失败: instType=%s err=%v", t, err)
			continue
		}
		freshType[t] = list
		for _, ins := range list {
			freshId[ins.InstId] = ins
		}
	}
	if len(freshType) == 0 {
		return nil
	}

	okxInstruments.mu.Lock()
	// 合并旧缓存（避免单类拉取失败丢失另一类）
	if okxInstruments.byType != nil && time.Since(okxInstruments.fetchedAt) < okxInstrumentsTTL {
		for t, list := range okxInstruments.byType {
			if _, ok := freshType[t]; !ok {
				freshType[t] = list
				for _, ins := range list {
					freshId[ins.InstId] = ins
				}
			}
		}
	}
	if okxInstruments.byType == nil {
		okxInstruments.byType = map[string][]OKXInstrument{}
		okxInstruments.byInstId = map[string]OKXInstrument{}
	}
	for t, list := range freshType {
		okxInstruments.byType[t] = list
	}
	for id, ins := range freshId {
		okxInstruments.byInstId[id] = ins
	}
	okxInstruments.fetchedAt = time.Now()
	okxInstruments.mu.Unlock()

	out := make([]OKXInstrument, 0, len(freshType["SPOT"])+len(freshType["SWAP"]))
	for _, t := range types {
		out = append(out, freshType[t]...)
	}
	return out
}

// okxLookupInstrument 查询单个合约规格；缓存为空时尝试拉取一次。
func okxLookupInstrument(instId string) *OKXInstrument {
	_, byId := okxInstrumentsSnapshot()
	if byId == nil {
		GetOKXInstruments("")
		_, byId = okxInstrumentsSnapshot()
	}
	if byId == nil {
		return nil
	}
	if ins, ok := byId[instId]; ok {
		return &ins
	}
	return nil
}

// okxBaseFromAlias 通过中文别名/币种名反查基础币种（复用币安别名表）。
func okxBaseFromAlias(input string) string {
	if base := binanceBaseFromAlias(input); base != "" {
		return base
	}
	trimmed := strings.ToUpper(strings.TrimSpace(input))
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	return trimmed
}

// ResolveOKXInstId 把用户/AI 输入归一为 OKX instId。
// 支持：btc / 比特币 / BTC-USDT / BTC-USDT-SWAP / btcusdt / ok:btcusdt(-swap)。
// instType 为空时自动判断：显式 SWAP 后缀→SWAP；显式 BASE-QUOTE→SPOT；否则默认 SWAP。
func ResolveOKXInstId(input, instType string) (string, string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", false
	}
	if IsOKXCode(raw) {
		raw = raw[len(okxCodePrefix):]
	}
	raw = strings.ToUpper(strings.TrimSpace(raw))
	for _, sep := range []string{" ", "/", "_"} {
		raw = strings.ReplaceAll(raw, sep, "-")
	}

	// 期望类型判断：显式 SWAP 后缀→SWAP；显式 BASE-QUOTE 结构→SPOT；否则默认 SWAP（交易场景主用永续）
	explicitSwap := strings.HasSuffix(raw, "SWAP")
	explicitSpot := strings.Contains(raw, "-") && !explicitSwap
	wantType := strings.ToUpper(strings.TrimSpace(instType))
	if wantType != "SPOT" && wantType != "SWAP" {
		if explicitSwap {
			wantType = "SWAP"
		} else if explicitSpot {
			wantType = "SPOT"
		} else {
			wantType = "SWAP"
		}
	}

	// 提取基础币种：中文别名优先（比特币→BTC），否则用去分隔后的原串再剥离计价币后缀
	base := okxBaseFromAlias(strings.ReplaceAll(raw, "-", ""))
	if base == "" {
		return "", "", false
	}
	base = strings.TrimSuffix(base, "SWAP")

	if wantType == "SWAP" {
		quote := "USDT"
		for _, q := range []string{"USDT", "USD"} {
			if strings.HasSuffix(base, q) && len(base) > len(q) {
				quote = q
				base = strings.TrimSuffix(base, q)
				break
			}
		}
		return base + "-" + quote + "-SWAP", "SWAP", true
	}

	quote := "USDT"
	for _, q := range []string{"USDT", "USDC", "USD"} {
		if strings.HasSuffix(base, q) && len(base) > len(q) {
			quote = q
			base = strings.TrimSuffix(base, q)
			break
		}
	}
	return base + "-" + quote, "SPOT", true
}

// ===== 公共行情 =====

// GetOKXTickers 获取某类品种 24h 行情列表。
func GetOKXTickers(instType string) []OKXTicker {
	t := strings.ToUpper(strings.TrimSpace(instType))
	if t != "SPOT" && t != "SWAP" {
		t = "SWAP"
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := okxCachedRequest(ctx, "tickers:"+t, 3*time.Second, http.MethodGet, "/api/v5/market/tickers", url.Values{"instType": {t}})
	if err != nil {
		logger.SugaredLogger.Warnf("获取OKX行情失败: instType=%s err=%v", t, err)
		return nil
	}
	var list []OKXTicker
	if err := okxDataList(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析OKX行情失败: %v", err)
		return nil
	}
	return list
}

// GetOKXTicker 获取单个品种 24h 行情。
func GetOKXTicker(instId string) *OKXTicker {
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := okxCachedRequest(ctx, "ticker:"+instId, 3*time.Second, http.MethodGet, "/api/v5/market/ticker", url.Values{"instId": {instId}})
	if err != nil {
		logger.SugaredLogger.Warnf("获取OKX行情失败: instId=%s err=%v", instId, err)
		return nil
	}
	var t OKXTicker
	if err := okxData(body, &t); err != nil {
		return nil
	}
	return &t
}

// OKXCandle K 线数据（已换算为数值类型，时间戳毫秒）。
type OKXCandle struct {
	Ts      int64   `json:"ts"` // 开盘时间（毫秒）
	Open    float64 `json:"open"`
	High    float64 `json:"high"`
	Low     float64 `json:"low"`
	Close   float64 `json:"close"`
	Vol     float64 `json:"vol"`    // 成交量（张/币）
	VolCcy  float64 `json:"volCcy"` // 成交额（计价币）
	Confirm bool    `json:"confirm"`
}

// GetOKXCandles 拉取 K 线（公共接口）。
// bar 支持：1m/3m/5m/15m/30m/1H/2H/4H/6H/12H/1D/1W/1M；limit 1~300，默认 200。
// OKX 返回按时间倒序，这里转为正序（旧→新）。
func GetOKXCandles(instId, bar string, limit int) []OKXCandle {
	instId = strings.TrimSpace(instId)
	if instId == "" {
		return []OKXCandle{}
	}
	bar = strings.TrimSpace(bar)
	if bar == "" {
		bar = "15m"
	}
	if limit <= 0 || limit > 300 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	params := url.Values{"instId": {instId}, "bar": {bar}, "limit": {strconv.Itoa(limit)}}
	body, err := okxCachedRequest(ctx, "candles:"+instId+":"+bar+":"+strconv.Itoa(limit), 3*time.Second, http.MethodGet, "/api/v5/market/candles", params)
	if err != nil {
		logger.SugaredLogger.Warnf("获取OKX K线失败: instId=%s bar=%s err=%v", instId, bar, err)
		return []OKXCandle{}
	}
	// OKX candles 的 data 是多行数组（每行一根K线），须整体解析；行内 confirm 为数字混排
	var rows [][]any
	if err := okxDataList(body, &rows); err != nil {
		logger.SugaredLogger.Warnf("解析OKX K线失败: %v", err)
		return []OKXCandle{}
	}
	out := make([]OKXCandle, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if len(r) < 6 {
			continue
		}
		out = append(out, OKXCandle{
			Ts:      int64(okxCellFloat(r[0])),
			Open:    okxCellFloat(r[1]),
			High:    okxCellFloat(r[2]),
			Low:     okxCellFloat(r[3]),
			Close:   okxCellFloat(r[4]),
			Vol:     okxCellFloat(r[5]),
			VolCcy:  okxCellFloat(r[6]),
			Confirm: len(r) > 8 && okxCellFloat(r[8]) == 1,
		})
	}
	return out
}

// ===== K 线取数（信号监控 / K 线弹窗复用） =====

// okxCST 东八区：K 线 Day 字段与币安/东财口径保持一致（UTC+8 无夏令时）。
var okxCST = time.FixedZone("CST", 8*3600)

// okxBarFromKlt 项目内 klt 标识 → OKX bar。OKX 不支持的周期返回空串。
func okxBarFromKlt(klt string) string {
	switch strings.TrimSpace(klt) {
	case "1":
		return "1m"
	case "3":
		return "3m"
	case "5":
		return "5m"
	case "15":
		return "15m"
	case "30":
		return "30m"
	case "60":
		return "1H"
	case "120":
		return "2H"
	case "240":
		return "4H"
	case "101":
		return "1D"
	case "102":
		return "1W"
	case "103":
		return "1M"
	}
	return ""
}

// fetchFromOKX OKX 合约 K 线 → KLineData（与币安/Bitget 同构：Day 按 UTC+8 格式化，
// 分时周期带 HH:mm，日及以上只带日期）。买点门控需要 ≥300 根，OKX 单次上限 300，
// limit 一律收紧到 300；ok: 与 bn:/bt: 代码都会被信号监控与图表用到。
func fetchFromOKX(stockCode, klt string, limit int) *KLineSourceResult {
	empty := &KLineSourceResult{Data: &[]KLineData{}, Source: "okx"}
	instId, _, ok := ResolveOKXInstId(stockCode, "")
	if !ok {
		return empty
	}
	bar := okxBarFromKlt(klt)
	if bar == "" {
		return empty
	}
	if limit > 300 {
		limit = 300
	}
	candles := GetOKXCandles(instId, bar, limit)
	if len(candles) == 0 {
		return empty
	}
	intraday := strings.HasSuffix(bar, "m") || strings.HasSuffix(bar, "H")
	out := make([]KLineData, 0, len(candles))
	for i, c := range candles {
		if c.Open <= 0 || c.High <= 0 || c.Low <= 0 || c.Close <= 0 {
			continue
		}
		t := time.UnixMilli(c.Ts).In(okxCST)
		day := t.Format("2006-01-02")
		if intraday {
			day = t.Format("2006-01-02 15:04")
		}
		kd := KLineData{
			Day:    day,
			Open:   strconv.FormatFloat(c.Open, 'f', -1, 64),
			High:   strconv.FormatFloat(c.High, 'f', -1, 64),
			Low:    strconv.FormatFloat(c.Low, 'f', -1, 64),
			Close:  strconv.FormatFloat(c.Close, 'f', -1, 64),
			Volume: strconv.FormatFloat(c.Vol, 'f', -1, 64),
			Amount: strconv.FormatFloat(c.VolCcy, 'f', -1, 64),
		}
		if i > 0 {
			prevClose := candles[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (c.Close-prevClose)/prevClose*100)
				kd.ChangeValue = strconv.FormatFloat(c.Close-prevClose, 'f', -1, 64)
				kd.Amplitude = fmt.Sprintf("%.2f", (c.High-c.Low)/prevClose*100)
			}
		}
		out = append(out, kd)
	}
	if len(out) == 0 {
		return empty
	}
	return &KLineSourceResult{Data: &out, Source: "okx"}
}

// GetOKXAccountInstruments 账户维度可交易品种列表（私有接口 /api/v5/account/instruments）。
// 返回当前账户（含模拟盘账户）实际可交易的产品，比公共合约表更准确；
// instType 传空返回全部类型，由调用方按需过滤。
// 凭证未配置或请求失败返回 nil，调用方应回退公共接口 /api/v5/public/instruments。
func GetOKXAccountInstruments(instType string) []OKXInstrument {
	if okxGetCredentials() == nil {
		return nil
	}
	// 注意：/api/v5/account/instruments 的 instType 为必填（错误码 50014），
	// 与公共接口"传空返回 SPOT+SWAP"不同，这里空值拆分为分组拉取后合并。
	t := strings.ToUpper(strings.TrimSpace(instType))
	if t == "" {
		merged := GetOKXAccountInstruments("SPOT")
		merged = append(merged, GetOKXAccountInstruments("SWAP")...)
		return merged
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	params := url.Values{"instType": {t}}
	key := "account-instruments:" + t
	body, err := okxCachedRequestPriv(ctx, key, okxInstrumentsTTL, http.MethodGet, "/api/v5/account/instruments", params, true)
	if err != nil {
		logger.SugaredLogger.Warnf("获取OKX账户可交易品种失败（回退公共合约表）: err=%v", err)
		return nil
	}
	var list []OKXInstrument
	if err := okxDataList(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析OKX账户可交易品种失败: %v", err)
		return nil
	}
	return list
}

// GetOKXAccountInstrumentsForEnv 按指定环境的账户可交易品种表（simulated=true 模拟盘，false 实盘）。
// 模拟盘与实盘是两个独立账户、品种表不同：模拟盘只有主流币，实盘为全量市场；
// 交易面板双环境并行时各取各的品种表。缓存键带环境维度，避免互相污染。
func GetOKXAccountInstrumentsForEnv(simulated bool, instType string) []OKXInstrument {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil
	}
	t := strings.ToUpper(strings.TrimSpace(instType))
	if t == "" {
		merged := GetOKXAccountInstrumentsForEnv(simulated, "SPOT")
		merged = append(merged, GetOKXAccountInstrumentsForEnv(simulated, "SWAP")...)
		return merged
	}
	envTag := "live"
	if simulated {
		envTag = "sim"
	}
	cacheKey := "account-instruments:env=" + envTag + ":" + t
	okxCacheMu.Lock()
	if e, ok := okxCache[cacheKey]; ok && time.Now().Before(e.expiresAt) {
		body := e.body
		okxCacheMu.Unlock()
		var cached []OKXInstrument
		if err := okxDataList(body, &cached); err == nil {
			return cached
		}
	} else {
		okxCacheMu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	params := url.Values{"instType": {t}}
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/instruments", params, nil)
	if err != nil {
		logger.SugaredLogger.Warnf("获取OKX账户可交易品种失败(sim=%v，回退公共合约表): err=%v", simulated, err)
		return nil
	}
	okxCacheMu.Lock()
	okxCache[cacheKey] = okxCacheEntry{body: body, expiresAt: time.Now().Add(okxInstrumentsTTL)}
	okxCacheMu.Unlock()
	var list []OKXInstrument
	if err := okxDataList(body, &list); err != nil {
		logger.SugaredLogger.Warnf("解析OKX账户可交易品种失败: %v", err)
		return nil
	}
	return list
}

// ===== 私有接口 =====

// GetOKXBalance 查询账户余额。环境跟随「模拟盘」全局开关。
func GetOKXBalance() (*OKXBalance, error) {
	cred := okxGetCredentials()
	if cred == nil {
		return nil, okxCredMissingError()
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	return getOKXBalanceWithCred(ctx, cred)
}

// GetOKXBalanceForEnv 按指定环境查询账户余额：simulated=true 模拟盘，false 实盘。
// 模拟盘与实盘是两个独立账户，交易页签双环境并行查看时各查各的。
func GetOKXBalanceForEnv(simulated bool) (*OKXBalance, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	return getOKXBalanceWithCred(ctx, cred)
}

// getOKXBalanceWithCred 余额查询核心：凭证由调用方给定（全局开关或显式环境）。
func getOKXBalanceWithCred(ctx context.Context, cred *okxCredentials) (*OKXBalance, error) {
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/balance", nil, nil)
	if err != nil {
		return nil, err
	}
	var bal OKXBalance
	if err := okxData(body, &bal); err != nil {
		return nil, err
	}
	return &bal, nil
}

// GetOKXAccountConfig 查询账户配置（持仓模式等，供下单参数判断）。环境跟随「模拟盘」全局开关。
func GetOKXAccountConfig() (*OKXAccountConfig, error) {
	cred := okxGetCredentials()
	if cred == nil {
		return nil, okxCredMissingError()
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	return getOKXAccountConfigWithCred(ctx, cred)
}

// GetOKXAccountConfigForEnv 按指定环境查询账户配置：simulated=true 模拟盘，false 实盘。
func GetOKXAccountConfigForEnv(simulated bool) (*OKXAccountConfig, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	return getOKXAccountConfigWithCred(ctx, cred)
}

// getOKXAccountConfigWithCred 账户配置查询核心：凭证由调用方给定。
func getOKXAccountConfigWithCred(ctx context.Context, cred *okxCredentials) (*OKXAccountConfig, error) {
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/config", nil, nil)
	if err != nil {
		return nil, err
	}
	var cfg OKXAccountConfig
	if err := okxData(body, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetOKXPositions 查询持仓；instId 为空查全部。环境跟随「模拟盘」全局开关。
func GetOKXPositions(instId string) ([]OKXPosition, error) {
	simulated := true
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		simulated = cfg.OKXSimulated
	}
	return okxPositionsForEnv(instId, simulated)
}

// okxPositionsForEnv 按指定环境查询持仓：模拟盘与实盘是两个独立账户，持仓互不可见，
// 信号自动交易双环境并行时必须按环境各查各的。
func okxPositionsForEnv(instId string, simulated bool) ([]OKXPosition, error) {
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	params := url.Values{}
	if strings.TrimSpace(instId) != "" {
		params.Set("instId", strings.TrimSpace(instId))
	}
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/positions", params, nil)
	if err != nil {
		return nil, err
	}
	var list []OKXPosition
	if err := okxDataList(body, &list); err != nil {
		return nil, err
	}
	// 过滤零仓位
	out := make([]OKXPosition, 0, len(list))
	for _, p := range list {
		if okxFloatNum(p.Pos) != 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

// GetOKXPositionListForEnv 按指定环境查询持仓：simulated=true 模拟盘，false 实盘。
// 模拟盘与实盘是两个独立账户，持仓互不可见，instId 为空查全部。
func GetOKXPositionListForEnv(simulated bool, instId string) ([]OKXPosition, error) {
	return okxPositionsForEnv(instId, simulated)
}

// okxOrdersByAPI 拉取订单列表。pending=true 走 orders-pending，否则走 orders-history。
// 环境跟随「模拟盘」全局开关。
func okxOrdersByAPI(pending bool, instType, instId string, limit int) ([]OKXOrder, error) {
	cred := okxGetCredentials()
	if cred == nil {
		return nil, okxCredMissingError()
	}
	return okxOrdersByAPIWithCred(pending, instType, instId, limit, cred)
}

// okxOrdersByAPIWithCred 订单列表核心：凭证由调用方给定（全局开关或显式环境）。
func okxOrdersByAPIWithCred(pending bool, instType, instId string, limit int, cred *okxCredentials) ([]OKXOrder, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	types := []string{}
	it := strings.ToUpper(strings.TrimSpace(instType))
	switch it {
	case "SPOT", "SWAP":
		types = []string{it}
	default:
		types = []string{"SWAP", "SPOT"} // 未指定时两类都查
	}

	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()

	out := make([]OKXOrder, 0, limit)
	for _, t := range types {
		params := url.Values{}
		params.Set("instType", t)
		if strings.TrimSpace(instId) != "" {
			params.Set("instId", strings.TrimSpace(instId))
		}
		path := "/api/v5/trade/orders-history"
		if pending {
			path = "/api/v5/trade/orders-pending"
		} else {
			params.Set("limit", strconv.Itoa(limit))
		}
		body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, path, params, nil)
		if err != nil {
			return nil, err
		}
		var list []OKXOrder
		if err := okxDataList(body, &list); err != nil {
			return nil, err
		}
		out = append(out, list...)
		if len(out) >= limit {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return okxFloatNum(out[i].UTime) > okxFloatNum(out[j].UTime)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetOKXPendingOrders 待成交订单。
func GetOKXPendingOrders(instType, instId string, limit int) ([]OKXOrder, error) {
	return okxOrdersByAPI(true, instType, instId, limit)
}

// GetOKXOrderHistory 历史订单。
func GetOKXOrderHistory(instType, instId string, limit int) ([]OKXOrder, error) {
	return okxOrdersByAPI(false, instType, instId, limit)
}

// GetOKXOpenOrdersForEnv 按指定环境查询当前委托（SWAP 50 条）：simulated=true 模拟盘，false 实盘。
func GetOKXOpenOrdersForEnv(simulated bool) ([]OKXOrder, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	return okxOrdersByAPIWithCred(true, "SWAP", "", 50, cred)
}

// GetOKXHistoryOrdersForEnv 按指定环境查询历史委托（SWAP 100 条）。
func GetOKXHistoryOrdersForEnv(simulated bool) ([]OKXOrder, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	return okxOrdersByAPIWithCred(false, "SWAP", "", 100, cred)
}

// OKXTradeFill /trade/fills-history 成交明细。fillPnl 为该笔成交的已实现盈亏（不含手续费，
// 仅合约/杠杆类成交有值）；fillFee 手续费（OKX 返回负数）。
type OKXTradeFill struct {
	InstId  string `json:"instId"`
	TradeId string `json:"tradeId"`
	OrdId   string `json:"ordId"`
	Side    string `json:"side"`
	PosSide string `json:"posSide"`
	FillPx  string `json:"fillPx"`
	FillSz  string `json:"fillSz"`
	FillFee string `json:"fillFee"`
	Fee     string `json:"fee"` // fills-history 接口的费用字段名是 fee（fills 才是 fillFee），解析后并回 FillFee
	FillPnl string `json:"fillPnl"`
	Ts      string `json:"ts"`
}

// GetOKXFillsHistory 拉取近期成交明细（单页 100，按 tradeId 游标向前翻页，上限 300）。
// fills-history 的 instType 归类与订单接口不同：SWAP 合约对应 MARGINSWAP。环境跟随「模拟盘」全局开关。
func GetOKXFillsHistory(instType, instId string, limit int) ([]OKXTradeFill, error) {
	cred := okxGetCredentials()
	if cred == nil {
		return nil, okxCredMissingError()
	}
	return okxFillsHistoryWithCred(instType, instId, limit, cred)
}

// GetOKXFillsHistoryForEnv 按指定环境拉取近期成交明细：simulated=true 模拟盘，false 实盘。
func GetOKXFillsHistoryForEnv(simulated bool, instType, instId string, limit int) ([]OKXTradeFill, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	return okxFillsHistoryWithCred(instType, instId, limit, cred)
}

// okxFillsHistoryWithCred 成交明细核心：凭证由调用方给定（全局开关或显式环境）。
func okxFillsHistoryWithCred(instType, instId string, limit int, cred *okxCredentials) ([]OKXTradeFill, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	it := strings.ToUpper(strings.TrimSpace(instType))
	// fills-history 的合约类目兼容：优先 MARGINSWAP（部分产品线归此类），被 OKX 以
	// 51000「Parameter instType error」拒绝时自动回退 SWAP 重试；SPOT 固定不变。
	pathTypes := []string{"MARGINSWAP", "SWAP"}
	if it == "SPOT" {
		pathTypes = []string{"SPOT"}
	}
	workType := "" // 已确认可用的类目，翻页时直接复用避免每页试错

	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()

	fetchPage := func(after string) ([]OKXTradeFill, error) {
		params := url.Values{}
		params.Set("limit", "100")
		if strings.TrimSpace(instId) != "" {
			params.Set("instId", strings.TrimSpace(instId))
		}
		if after != "" {
			params.Set("after", after)
		}
		var lastErr error
		for _, pt := range pathTypes {
			if workType != "" && pt != workType {
				continue
			}
			p := params.Clone()
			p.Set("instType", pt)
			body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/trade/fills-history", p, nil)
			if err == nil {
				workType = pt
				var list []OKXTradeFill
				if err := okxDataList(body, &list); err != nil {
					return nil, err
				}
				// fills-history 的费用字段名为 fee，统一并回 FillFee 供上层盈亏口径使用
				for i := range list {
					if list[i].FillFee == "" {
						list[i].FillFee = list[i].Fee
					}
				}
				return list, nil
			}
			lastErr = err
			if strings.Contains(err.Error(), "51000") || strings.Contains(err.Error(), "instType error") {
				continue // 类目被拒，换下一个候选
			}
			return nil, err
		}
		return nil, lastErr
	}

	out := make([]OKXTradeFill, 0, limit)
	after := ""
	for len(out) < limit {
		list, err := fetchPage(after)
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
		if len(list) < 100 {
			break
		}
		after = list[len(list)-1].TradeId // 取更早一页
	}
	sort.SliceStable(out, func(i, j int) bool {
		return okxFloatNum(out[i].Ts) > okxFloatNum(out[j].Ts)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PlaceOKXOrder 下单。返回结果与订单号；交易开关关闭时报错。环境跟随「模拟盘」开关。
func PlaceOKXOrder(req *OKXOrderRequest) (*OKXPlaceOrderResult, error) {
	if !okxTradeEnabled() {
		return nil, errors.New("OKX 交易开关未开启：请在设置页开启「OKX 交易」后再执行下单操作")
	}
	return placeOKXOrderCore(req)
}

// SubmitOKXOrderForEnv 显式指定环境下单：simulated=true 模拟盘，false 实盘（真实资金）。
// 与全局手动路径的区别是不读取「模拟盘」全局开关（交易页签双环境各自固定环境），
// 「OKX 交易」总开关仍照常拦截。
func SubmitOKXOrderForEnv(simulated bool, req *OKXOrderRequest) (*OKXPlaceOrderResult, error) {
	if !okxTradeEnabled() {
		return nil, errors.New("OKX 交易开关未开启：请在设置页开启「OKX 交易」后再执行下单操作")
	}
	return placeOKXOrderWithEnv(req, simulated)
}

// CloseOKXPositionMarket 市价平仓（reduceOnly 只减仓，不反向开仓）。
// net 模式按 pos 正负判方向；双向持仓（long_short_mode）按 posSide 显式判方向。
// 供手动平仓（App 层）与止盈止损监控共用。环境跟随「模拟盘」全局开关。
func CloseOKXPositionMarket(instId, posSide, pos string) (*OKXPlaceOrderResult, error) {
	req, err := okxClosePositionRequest(instId, posSide, pos)
	if err != nil {
		return nil, err
	}
	return PlaceOKXOrder(req)
}

// CloseOKXPositionForEnv 按指定环境市价平仓：simulated=true 模拟盘，false 实盘。
// 模拟盘与实盘持仓互不可见，必须用对应环境凭证才能平掉对应账户的仓。
func CloseOKXPositionForEnv(simulated bool, instId, posSide, pos string) (*OKXPlaceOrderResult, error) {
	req, err := okxClosePositionRequest(instId, posSide, pos)
	if err != nil {
		return nil, err
	}
	return SubmitOKXOrderForEnv(simulated, req)
}

// okxClosePositionRequest 构建市价 reduceOnly 平仓请求（只减仓，不反向开仓）。
// net 模式按 pos 正负判方向；双向持仓（long_short_mode）按 posSide 显式判方向。
func okxClosePositionRequest(instId, posSide, pos string) (*OKXOrderRequest, error) {
	posClean := strings.TrimSpace(pos)
	if strings.TrimSpace(instId) == "" || posClean == "" {
		return nil, errors.New("平仓参数不完整")
	}
	posVal, err := strconv.ParseFloat(posClean, 64)
	if err != nil || posVal == 0 {
		return nil, errors.New("持仓数量无效: " + posClean)
	}
	if posVal < 0 {
		posVal = -posVal
	}
	req := OKXOrderRequest{
		InstId:     strings.TrimSpace(instId),
		TdMode:     "cross",
		OrdType:    "market",
		Sz:         strconv.FormatFloat(posVal, 'f', -1, 64),
		ReduceOnly: boolPtr(true),
	}
	switch posSide {
	case "long":
		req.PosSide = "long"
		req.Side = "sell"
	case "short":
		req.PosSide = "short"
		req.Side = "buy"
	default: // net：pos 为负 = 空头，平仓买回
		if strings.HasPrefix(posClean, "-") {
			req.Side = "buy"
		} else {
			req.Side = "sell"
		}
	}
	return &req, nil
}

// boolPtr 返回 bool 指针（reduceOnly 等 omitempty 指针字段用）。
func boolPtr(v bool) *bool {
	return &v
}

// PlaceOKXSignalOrder 信号自动交易下单：simulated=true 走 OKX 模拟环境（模拟盘密钥 +
// x-simulated-trading:1），false 走实盘（实盘密钥与真实资金）。按环境分别校验对应开关，
// 与手动交易开关无关。
func PlaceOKXSignalOrder(req *OKXOrderRequest, simulated bool) (*OKXPlaceOrderResult, error) {
	if db.Dao == nil {
		return nil, errors.New("数据库不可用")
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return nil, errors.New("配置不可用")
	}
	if simulated && !cfg.OKXSignalTradeEnable {
		return nil, errors.New("OKX 信号自动交易·模拟盘开关未开启：请在设置页 OKX 区块开启「模拟盘自动下单」")
	}
	if !simulated && !cfg.OKXSignalTradeLive {
		return nil, errors.New("OKX 信号自动交易·实盘开关未开启：请在设置页 OKX 区块开启「实盘自动下单」")
	}
	return placeOKXOrderWithEnv(req, simulated)
}

// placeOKXOrderCore 下单核心。凭证与环境由 okxGetCredentials 按「模拟盘」全局开关决定。
func placeOKXOrderCore(req *OKXOrderRequest) (*OKXPlaceOrderResult, error) {
	simulated := true
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		simulated = cfg.OKXSimulated
	}
	return placeOKXOrderWithEnv(req, simulated)
}

// placeOKXOrderWithEnv 按指定环境下单：simulated=true 用模拟盘密钥与模拟环境头，
// false 用实盘密钥与真实资金。规格校验、参数补齐与手动路径完全一致。
func placeOKXOrderWithEnv(req *OKXOrderRequest, simulated bool) (*OKXPlaceOrderResult, error) {
	if req == nil || req.InstId == "" || req.Side == "" || strings.TrimSpace(req.Sz) == "" {
		return nil, errors.New("下单参数不完整：instId/side/sz 必填")
	}
	req.Side = strings.ToLower(req.Side)
	if req.Side != "buy" && req.Side != "sell" {
		return nil, fmt.Errorf("无效的 side: %s（仅支持 buy/sell）", req.Side)
	}
	instType := "SWAP"
	if strings.HasSuffix(req.InstId, "-SWAP") {
		instType = "SWAP"
	} else if ins := okxLookupInstrument(req.InstId); ins != nil {
		instType = ins.InstType
	}

	// 规格校验与默认值
	if ins := okxLookupInstrument(req.InstId); ins != nil {
		sz := okxFloatNum(req.Sz)
		if sz <= 0 {
			return nil, fmt.Errorf("下单数量无效: %s", req.Sz)
		}
		if minSz := okxFloatNum(ins.MinSz); minSz > 0 && sz < minSz {
			return nil, fmt.Errorf("下单数量 %s 低于最小下单量 %s（instId=%s）", req.Sz, ins.MinSz, ins.InstId)
		}
		if lotSz := okxFloatNum(ins.LotSz); lotSz > 0 {
			steps := sz / lotSz
			nearest := math.Round(steps)
			// 浮点容差：0.07/0.01=6.999999... 属合法整数步，不能直接比较整除
			if math.Abs(steps-nearest) > 1e-6 {
				return nil, fmt.Errorf("下单数量 %s 不符合步进 %s（instId=%s）", req.Sz, ins.LotSz, ins.InstId)
			}
		}
	}

	if strings.TrimSpace(req.TdMode) == "" {
		if instType == "SPOT" {
			req.TdMode = "cash"
		} else {
			req.TdMode = "cross"
		}
	}
	if strings.TrimSpace(req.OrdType) == "" {
		req.OrdType = "limit"
	}
	req.OrdType = strings.ToLower(req.OrdType)
	if req.OrdType == "limit" && strings.TrimSpace(req.Px) == "" {
		return nil, errors.New("限价单必须提供价格 px（或改用 ordType=market）")
	}
	// 现货市价单统一按基础币数量（tgtCcy=base_ccy），避免 buy 默认按计价币金额的歧义
	if instType == "SPOT" && req.OrdType == "market" && strings.TrimSpace(req.TgtCcy) == "" {
		req.TgtCcy = "base_ccy"
	}
	// 双向持仓账户必须显式 posSide（OKX 51010）；net 模式与现货不需要。
	// 按本次下单环境（simulated=false 走实盘凭证）查询持仓模式，勿用全局开关错配。
	okxAttachPosSide(req, instType, simulated)

	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodPost, "/api/v5/trade/order", nil, req)
	if err != nil {
		if strings.Contains(err.Error(), "51010") {
			return nil, errors.New("当前 OKX 账户模式（现货模式）不支持合约交易（51010）：请在 OKX 网页/App 的「账户模式」中升级为单币种保证金后重试")
		}
		return nil, err
	}
	var res OKXPlaceOrderResult
	if err := okxData(body, &res); err != nil {
		return nil, err
	}
	if res.SCode != "0" && res.SCode != "" {
		return nil, fmt.Errorf("下单失败（%s）: %s", res.SCode, res.SMsg)
	}
	return &res, nil
}

// OKXSetLeverageResult /account/set-leverage 响应。
type OKXSetLeverageResult struct {
	InstId  string `json:"instId"`
	Lever   string `json:"lever"`
	MgnMode string `json:"mgnMode"`
	SCode   string `json:"sCode"`
	SMsg    string `json:"sMsg"`
}

// SetOKXLeverageEnv 按环境设置合约杠杆（cross 全仓）：simulated=true 模拟盘，false 实盘。
// 仅对 SWAP 永续有效（现货无杠杆概念）；cross 模式无需 posSide；重复设置同一杠杆幂等无害。
// lever 取值 "1"~"125"。
func SetOKXLeverageEnv(simulated bool, instId, lever string) (*OKXSetLeverageResult, error) {
	instId = strings.TrimSpace(instId)
	lever = strings.TrimSpace(lever)
	if instId == "" || !strings.HasSuffix(instId, "-SWAP") {
		return nil, errors.New("杠杆设置仅支持永续合约（instId 需以 -SWAP 结尾）")
	}
	if n, err := strconv.ParseFloat(lever, 64); err != nil || n < 1 || n > 125 {
		return nil, fmt.Errorf("无效杠杆值: %s（支持 1~125）", lever)
	}
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodPost, "/api/v5/account/set-leverage", nil, map[string]string{
		"instId":  instId,
		"lever":   lever,
		"mgnMode": "cross",
	})
	if err != nil {
		return nil, err
	}
	var res OKXSetLeverageResult
	if err := okxData(body, &res); err != nil {
		return nil, err
	}
	if res.SCode != "0" && res.SCode != "" {
		return nil, fmt.Errorf("杠杆设置失败（%s）: %s", res.SCode, res.SMsg)
	}
	return &res, nil
}

// ===== 交易所侧条件单（止盈止损 algo）查询与撤销 =====
// 信号自动开仓附带 attachAlgoOrds 后，交易所侧持有未触发条件单；软件侧监控市价平仓后
// 必须撤销这些「孤儿单」，否则同合约再开仓时旧条件单误触发会平掉新仓。

// OKXAlgoOrder /trade/orders-algo-pending 条件单（止盈止损）。
type OKXAlgoOrder struct {
	AlgoId      string `json:"algoId"`
	InstId      string `json:"instId"`
	OrdType     string `json:"ordType"` // conditional（单边）/ oco（双向）
	Side        string `json:"side"`
	PosSide     string `json:"posSide"`
	Sz          string `json:"sz"`
	TpTriggerPx string `json:"tpTriggerPx"`
	SlTriggerPx string `json:"slTriggerPx"`
	State       string `json:"state"` // live = 未触发
	CTime       string `json:"cTime"`
}

// okxListPendingAlgoByType 按环境与条件单类型查询未触发挂单。
func okxListPendingAlgoByType(simulated bool, instId, ordType string) ([]OKXAlgoOrder, error) {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/trade/orders-algo-pending", url.Values{
		"instId":  {instId},
		"ordType": {ordType},
	}, nil)
	if err != nil {
		return nil, err
	}
	var list []OKXAlgoOrder
	// 用 okxDataList 而非 okxData：无未触发条件单时 OKX 返回 data=[]，
	// okxData 会把空数组当错误（"OKX 返回数据为空"），导致「没有条件单」与「查询失败」无法区分。
	if err := okxDataList(body, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// GetOKXPendingAlgoOrdersEnv 查询某合约全部未触发条件单（conditional + oco 两类合并）。
func GetOKXPendingAlgoOrdersEnv(simulated bool, instId string) ([]OKXAlgoOrder, error) {
	var out []OKXAlgoOrder
	for _, ordType := range []string{"conditional", "oco"} {
		list, err := okxListPendingAlgoByType(simulated, instId, ordType)
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
	}
	return out, nil
}

// CancelOKXAlgoOrderEnv 撤销指定条件单。
func CancelOKXAlgoOrderEnv(simulated bool, algoId, instId string) error {
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return okxCredMissingErrorForEnv(simulated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodPost, "/api/v5/trade/cancel-algos", nil, []map[string]string{
		{"algoId": algoId, "instId": instId},
	})
	if err != nil {
		return err
	}
	var res []OKXPlaceOrderResult
	if err := okxData(body, &res); err != nil {
		return err
	}
	if len(res) > 0 && res[0].SCode != "0" && res[0].SCode != "" {
		return fmt.Errorf("撤销条件单失败（%s）: %s", res[0].SCode, res[0].SMsg)
	}
	return nil
}

// okxPosModeCaches 账户持仓模式缓存（long_short_mode / net_mode，极少变更，进程内长期缓存）。
// 模拟盘与实盘是两个独立账户、持仓模式可不同，必须按环境分开缓存：
// 否则全局开关=模拟盘时给实盘下单会误用模拟盘的 long_short_mode 补 posSide，实盘 net 模式拒单 51000。
var okxPosModeCaches = [2]atomic.Value{} // [0]=实盘, [1]=模拟盘

// okxAccountPosModeCached 按指定环境查询账户持仓模式，失败返回空串（按 net 模式处理）。
func okxAccountPosModeCached(simulated bool) string {
	idx := 0
	if simulated {
		idx = 1
	}
	if v := okxPosModeCaches[idx].Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	cfg, err := GetOKXAccountConfigForEnv(simulated)
	if err != nil || cfg == nil || cfg.PosMode == "" {
		return ""
	}
	okxPosModeCaches[idx].Store(cfg.PosMode)
	return cfg.PosMode
}

// okxAttachPosSide 双向持仓账户自动补 posSide：buy→long / sell→short（开仓语义）。
// 仅当请求未显式指定时填充；平仓单（reduceOnly）应自带与持仓一致的 posSide。
// 持仓模式必须按本次下单的环境查询（simulated 与实盘账户模式可不同）。
func okxAttachPosSide(req *OKXOrderRequest, instType string, simulated bool) {
	if req == nil || instType == "SPOT" || strings.TrimSpace(req.PosSide) != "" {
		return
	}
	if okxAccountPosModeCached(simulated) == "long_short_mode" {
		if req.Side == "buy" {
			req.PosSide = "long"
		} else if req.Side == "sell" {
			req.PosSide = "short"
		}
	}
}

// CancelOKXOrder 撤单。环境跟随「模拟盘」全局开关。
func CancelOKXOrder(instId, ordId string) (*OKXPlaceOrderResult, error) {
	if !okxTradeEnabled() {
		return nil, errors.New("OKX 交易开关未开启：请在设置页开启「OKX 交易」后再执行撤单操作")
	}
	cred := okxGetCredentials()
	if cred == nil {
		return nil, okxCredMissingError()
	}
	return cancelOKXOrderWithCred(cred, instId, ordId)
}

// RevokeOKXOrderForEnv 按指定环境撤单：simulated=true 模拟盘，false 实盘。
// 模拟盘与实盘委托互不可见，必须用对应环境凭证才能撤掉对应账户的委托。
func RevokeOKXOrderForEnv(simulated bool, instId, ordId string) (*OKXPlaceOrderResult, error) {
	if !okxTradeEnabled() {
		return nil, errors.New("OKX 交易开关未开启：请在设置页开启「OKX 交易」后再执行撤单操作")
	}
	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return nil, okxCredMissingErrorForEnv(simulated)
	}
	return cancelOKXOrderWithCred(cred, instId, ordId)
}

// cancelOKXOrderWithCred 撤单核心：凭证由调用方给定（全局开关或显式环境）。
func cancelOKXOrderWithCred(cred *okxCredentials, instId, ordId string) (*OKXPlaceOrderResult, error) {
	if strings.TrimSpace(instId) == "" || strings.TrimSpace(ordId) == "" {
		return nil, errors.New("撤单参数不完整：instId 与 ordId 必填")
	}
	payload := map[string]string{"instId": strings.TrimSpace(instId), "ordId": strings.TrimSpace(ordId)}
	ctx, cancel := context.WithTimeout(context.Background(), okxProxyTimeout)
	defer cancel()
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodPost, "/api/v5/trade/cancel-order", nil, payload)
	if err != nil {
		return nil, err
	}
	var res OKXPlaceOrderResult
	if err := okxData(body, &res); err != nil {
		return nil, err
	}
	if res.SCode != "0" && res.SCode != "" {
		return nil, fmt.Errorf("撤单失败（%s）: %s", res.SCode, res.SMsg)
	}
	return &res, nil
}

// ===== 展示辅助 =====

// OKXInstDisplayName 生成展示名（如 BTC-USDT-SWAP → 比特币/USDT 永续；BTC-USDT → 比特币/USDT 现货）。
func OKXInstDisplayName(instId string) string {
	parts := strings.Split(strings.ToUpper(instId), "-")
	if len(parts) < 2 {
		return instId
	}
	base := parts[0]
	if alias, ok := binanceBaseAliases[base]; ok {
		base = alias
	}
	if strings.HasSuffix(instId, "-SWAP") {
		return fmt.Sprintf("%s/%s 永续", base, parts[1])
	}
	return fmt.Sprintf("%s/%s 现货", base, parts[1])
}

// okxStateText 订单状态中文。
func okxStateText(state string) string {
	switch state {
	case "live":
		return "待成交"
	case "partially_filled":
		return "部分成交"
	case "filled":
		return "已成交"
	case "canceled":
		return "已撤销"
	}
	return state
}

// okxPosSideText 持仓方向中文。
func okxPosSideText(posSide string) string {
	switch posSide {
	case "long":
		return "多"
	case "short":
		return "空"
	case "net":
		return "净持仓"
	}
	return posSide
}

// okxPosModeText 持仓模式中文。
func okxPosModeText(posMode string) string {
	switch posMode {
	case "long_short_mode":
		return "双向持仓（开平需指定 long/short）"
	case "net_mode":
		return "单向持仓（net）"
	}
	return posMode
}

// okxMsToTime 毫秒时间戳转展示时间（UTC+8）。
func okxMsToTime(ms string) string {
	n, err := strconv.ParseInt(strings.TrimSpace(ms), 10, 64)
	if err != nil || n <= 0 {
		return "-"
	}
	if n < 1e12 {
		n *= 1000
	}
	return time.UnixMilli(n).In(binanceCST).Format("01-02 15:04:05")
}
