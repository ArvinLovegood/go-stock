package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/go-resty/resty/v2"
)

// Bitget 美股永续合约（RWA）客户端。
//
// 网络策略与币安完全同构但**相互独立**：直连优先，失败自动降级到用户在设置里配置的
// 「Bitget 合约代理」（Settings.BitgetProxy），不使用全局 HttpProxy，也不复用币安的
// BinanceProxy / client。
//
// 注意：不能直接复用 CreateHTTPClientWithTimeout —— ConfigureFromSettings 会把用户的
// HttpProxy 写进全局 sharedTransport.Proxy，直接复用会导致「直连实际走了代理」。
// 因此直连 client 取 sharedTransport.Clone() 后显式置 Proxy=nil。

const (
	bitgetMixBase        = "https://api.bitget.com/api/v2/mix/market"
	bitgetProductType    = "usdt-futures"
	bitgetCodePrefix     = "bt:"
	bitgetDirectTimeout  = 5 * time.Second
	bitgetProxyTimeout   = 12 * time.Second
	bitgetProxyCooldown  = 5 * time.Minute
	bitgetMaxConcurrency = 4
	bitgetSuccessCode    = "00000"
	bitgetEmptyDataCode  = "40054" // 该 symbol 无数据（如美股永续的多空比）
)

// IsBitgetFuturesCode 判断代码是否为 Bitget 美股永续合约（bt: 前缀，大小写不敏感）。
// 与 sh/sz/bj、hk、gb_、.US、.CSI、100.XXX 及币安 bn: 前缀均不冲突。
func IsBitgetFuturesCode(code string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(code)), "BT:")
}

// bitgetStockAliases 主流美股中文别名，用于展示名与 AI 参数容错。
var bitgetStockAliases = map[string]string{
	"AAPL": "苹果", "TSLA": "特斯拉", "NVDA": "英伟达", "MSFT": "微软",
	"AMZN": "亚马逊", "GOOGL": "谷歌", "GOOG": "谷歌", "META": "脸书",
	"NFLX": "奈飞", "TSM": "台积电", "AMD": "超微", "INTC": "英特尔",
	"AVGO": "博通", "ORCL": "甲骨文", "CRM": "赛富时", "ADBE": "Adobe",
	"QCOM": "高通", "MU": "美光", "PLTR": "Palantir", "COIN": "Coinbase",
	"MSTR": "微策略", "CRCL": "Circle", "HOOD": "罗宾汉", "UBER": "优步",
	"DIS": "迪士尼", "BA": "波音", "JPM": "摩根大通", "V": "Visa",
	"MA": "万事达", "WMT": "沃尔玛", "KO": "可口可乐", "PEP": "百事",
	"XOM": "埃克森美孚", "CVX": "雪佛龙", "PFE": "辉瑞", "MRNA": "Moderna",
	"LLY": "礼来", "UNH": "联合健康", "GS": "高盛", "MS": "摩根士丹利",
	"BABA": "阿里巴巴", "JD": "京东", "PDD": "拼多多", "NIO": "蔚来",
	"XPEV": "小鹏", "LI": "理想", "BIDU": "百度", "ARM": "Arm",
	"SMCI": "超微电脑", "APP": "AppLovin", "SHOP": "Shopify", "SQ": "Block",
}

// bitgetNonStockRWA 非股票 RWA（贵金属代币），需从美股列表中排除。
var bitgetNonStockRWA = map[string]struct{}{
	"PAXG": {},
	"XAUT": {},
}

// ===== 双通道 HTTP =====

type bitgetChannelState struct {
	mu          sync.Mutex
	preferProxy bool
	lastFailAt  time.Time
}

var bitgetChannel bitgetChannelState

var (
	bitgetDirectOnce sync.Once
	bitgetDirectCli  *resty.Client

	bitgetProxyMu  sync.Mutex
	bitgetProxyCli *resty.Client
	bitgetProxyURL string
)

var bitgetSemaphore = make(chan struct{}, bitgetMaxConcurrency)

// bitgetHTTPError Bitget 返回的非 200 响应。
type bitgetHTTPError struct {
	statusCode int
	body       string
}

func (e *bitgetHTTPError) Error() string {
	return fmt.Sprintf("bitget http %d: %s", e.statusCode, e.body)
}

// bitgetAPIError Bitget 返回 HTTP 200 但业务码非 00000。
// 业务错误**不**触发通道切换（属请求本身问题，如 symbol 无数据）。
type bitgetAPIError struct {
	code string
	msg  string
}

func (e *bitgetAPIError) Error() string {
	return fmt.Sprintf("bitget api %s: %s", e.code, e.msg)
}

// isBitgetEmptyData 判断错误是否为「该 symbol 无数据」（如美股永续无多空比）。
func isBitgetEmptyData(err error) bool {
	var ae *bitgetAPIError
	return errors.As(err, &ae) && ae.code == bitgetEmptyDataCode
}

// isBitgetChannelFailure 仅 HTTP 层异常（网络错误 / 限流 / 封禁 / 5xx）触发通道切换。
func isBitgetChannelFailure(err error) bool {
	var he *bitgetHTTPError
	if errors.As(err, &he) {
		switch he.statusCode {
		case http.StatusForbidden, http.StatusRequestTimeout, 418, http.StatusTooManyRequests,
			http.StatusUnavailableForLegalReasons, http.StatusInternalServerError,
			http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	var ae *bitgetAPIError
	if errors.As(err, &ae) {
		return false
	}
	return true
}

func bitgetDirectClient() *resty.Client {
	bitgetDirectOnce.Do(func() {
		tr := GetSharedTransport().Clone()
		tr.Proxy = nil
		hc := &http.Client{Transport: tr, Timeout: bitgetDirectTimeout}
		bitgetDirectCli = resty.NewWithClient(hc).SetTimeout(bitgetDirectTimeout).SetRetryCount(0)
	})
	return bitgetDirectCli
}

// bitgetProxyClient 返回 Bitget 专用代理 client；未配置代理时返回 nil。
// 界面已统一为「合约代理」单一设置项（对应 Settings.BinanceProxy），故 Settings.BitgetProxy
// 为空时回退使用它，保证统一设置项对币安与 Bitget 两条合约通道都生效。
func bitgetProxyClient() *resty.Client {
	if db.Dao == nil {
		return nil
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return nil
	}
	proxyURL := strings.TrimSpace(cfg.BitgetProxy)
	if proxyURL == "" {
		proxyURL = strings.TrimSpace(cfg.BinanceProxy)
	}
	if proxyURL == "" {
		return nil
	}

	bitgetProxyMu.Lock()
	defer bitgetProxyMu.Unlock()
	if bitgetProxyCli != nil && bitgetProxyURL == proxyURL {
		return bitgetProxyCli
	}
	pu, err := url.Parse(proxyURL)
	if err != nil || pu.Scheme == "" || pu.Host == "" {
		return nil
	}
	tr := GetSharedTransport().Clone()
	tr.Proxy = http.ProxyURL(pu)
	hc := &http.Client{Transport: tr, Timeout: bitgetProxyTimeout}
	bitgetProxyCli = resty.NewWithClient(hc).SetTimeout(bitgetProxyTimeout).SetRetryCount(0)
	bitgetProxyURL = proxyURL
	return bitgetProxyCli
}

func bitgetPreferProxy() bool {
	bitgetChannel.mu.Lock()
	defer bitgetChannel.mu.Unlock()
	if !bitgetChannel.preferProxy {
		return false
	}
	return time.Since(bitgetChannel.lastFailAt) <= bitgetProxyCooldown
}

func bitgetMarkDirectFailed() {
	bitgetChannel.mu.Lock()
	bitgetChannel.preferProxy = true
	bitgetChannel.lastFailAt = time.Now()
	bitgetChannel.mu.Unlock()
}

func bitgetMarkDirectOK() {
	bitgetChannel.mu.Lock()
	bitgetChannel.preferProxy = false
	bitgetChannel.mu.Unlock()
}

func bitgetDoOnce(ctx context.Context, cli *resty.Client, path string, params url.Values) ([]byte, error) {
	if cli == nil {
		return nil, errors.New("Bitget HTTP 客户端不可用")
	}
	req := cli.R().SetContext(ctx)
	if len(params) > 0 {
		req.SetQueryParamsFromValues(params)
	}
	resp, err := req.Get(bitgetMixBase + path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, &bitgetHTTPError{statusCode: resp.StatusCode(), body: truncateForLog(resp.String())}
	}
	return resp.Body(), nil
}

// doBitgetRequest 带双通道降级的 Bitget GET 请求（仅处理 HTTP 层，业务码由上层解析）。
func doBitgetRequest(ctx context.Context, path string, params url.Values) ([]byte, error) {
	bitgetSemaphore <- struct{}{}
	defer func() { <-bitgetSemaphore }()

	attemptDirectFirst := !bitgetPreferProxy()
	var lastErr error

	if attemptDirectFirst {
		body, err := bitgetDoOnce(ctx, bitgetDirectClient(), path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isBitgetChannelFailure(err) {
			return nil, err
		}
		bitgetMarkDirectFailed()
		logger.SugaredLogger.Warnf("Bitget 直连失败，尝试代理通道: path=%s err=%v", path, err)
	}

	if proxyCli := bitgetProxyClient(); proxyCli != nil {
		body, err := bitgetDoOnce(ctx, proxyCli, path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}

	if !attemptDirectFirst {
		body, err := bitgetDoOnce(ctx, bitgetDirectClient(), path, params)
		if err == nil {
			bitgetMarkDirectOK()
			return body, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("合约接口不可达，请在设置中配置「合约代理」")
	}
	return nil, lastErr
}

// ===== 结果缓存 =====

type bitgetCacheEntry struct {
	body      []byte
	expiresAt time.Time
}

var (
	bitgetCacheMu sync.Mutex
	bitgetCache   = map[string]bitgetCacheEntry{}

	bitgetFlightMu sync.Mutex
	bitgetFlights  = map[string]*sync.Mutex{}
)

func bitgetFlightLock(key string) *sync.Mutex {
	bitgetFlightMu.Lock()
	defer bitgetFlightMu.Unlock()
	m, ok := bitgetFlights[key]
	if !ok {
		m = &sync.Mutex{}
		bitgetFlights[key] = m
	}
	return m
}

func bitgetCachedRequest(ctx context.Context, key string, ttl time.Duration, path string, params url.Values) ([]byte, error) {
	if ttl > 0 {
		bitgetCacheMu.Lock()
		if e, ok := bitgetCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			bitgetCacheMu.Unlock()
			return body, nil
		}
		bitgetCacheMu.Unlock()
	}

	lock := bitgetFlightLock(key)
	lock.Lock()
	defer lock.Unlock()

	if ttl > 0 {
		bitgetCacheMu.Lock()
		if e, ok := bitgetCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			bitgetCacheMu.Unlock()
			return body, nil
		}
		bitgetCacheMu.Unlock()
	}

	body, err := doBitgetRequest(ctx, path, params)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		bitgetCacheMu.Lock()
		bitgetCache[key] = bitgetCacheEntry{body: body, expiresAt: time.Now().Add(ttl)}
		bitgetCacheMu.Unlock()
	}
	return body, nil
}

// bitgetEnvelope Bitget v2 统一响应外壳。
type bitgetEnvelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// bitgetFetch 拉取并解出业务数据；业务码非 00000 时返回 *bitgetAPIError。
func bitgetFetch(ctx context.Context, key string, ttl time.Duration, path string, params url.Values) (json.RawMessage, error) {
	body, err := bitgetCachedRequest(ctx, key, ttl, path, params)
	if err != nil {
		return nil, err
	}
	var env bitgetEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("解析 Bitget 响应失败: %w", err)
	}
	if env.Code != bitgetSuccessCode {
		return nil, &bitgetAPIError{code: env.Code, msg: env.Msg}
	}
	return env.Data, nil
}

// ===== /contracts 合约信息（美股 RWA 白名单）=====

const bitgetContractsTTL = 6 * time.Hour

var (
	bitgetInfoMu   sync.RWMutex
	bitgetInfoData *BitgetFuturesInfo
)

// GetBitgetFuturesInfo 获取美股永续合约基础信息（6 小时缓存）。
// 仅保留 isRwa=="YES" 且非贵金属代币且 symbolStatus=="normal" 的合约。
// 拉取失败时沿用旧缓存；两者皆无时返回 nil，调用方需降级处理。
func GetBitgetFuturesInfo() *BitgetFuturesInfo {
	bitgetInfoMu.RLock()
	cached := bitgetInfoData
	bitgetInfoMu.RUnlock()
	if cached != nil && time.Since(cached.FetchedAt) < bitgetContractsTTL {
		return cached
	}

	info := loadBitgetFuturesInfo()
	if info == nil {
		return cached
	}
	bitgetInfoMu.Lock()
	bitgetInfoData = info
	bitgetInfoMu.Unlock()
	return info
}

func loadBitgetFuturesInfo() *BitgetFuturesInfo {
	ctx, cancel := context.WithTimeout(context.Background(), bitgetProxyTimeout)
	defer cancel()

	data, err := bitgetFetch(ctx, "contracts", bitgetContractsTTL, "/contracts",
		url.Values{"productType": {bitgetProductType}})
	if err != nil {
		logger.SugaredLogger.Warnf("获取 Bitget /contracts 失败: %v", err)
		return nil
	}

	var raw []struct {
		Symbol       string `json:"symbol"`
		BaseCoin     string `json:"baseCoin"`
		QuoteCoin    string `json:"quoteCoin"`
		SymbolStatus string `json:"symbolStatus"`
		IsRwa        string `json:"isRwa"`
		PricePlace   string `json:"pricePlace"`
		VolumePlace  string `json:"volumePlace"`
		FundInterval string `json:"fundInterval"`
		MaxLever     string `json:"maxLever"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		logger.SugaredLogger.Warnf("解析 Bitget /contracts 失败: %v", err)
		return nil
	}

	info := &BitgetFuturesInfo{
		Symbols:   make([]BitgetSymbolInfo, 0, len(raw)),
		SymbolMap: make(map[string]BitgetSymbolInfo, len(raw)),
		FetchedAt: time.Now(),
	}
	for _, s := range raw {
		if !strings.EqualFold(s.IsRwa, "YES") || !strings.EqualFold(s.SymbolStatus, "normal") {
			continue
		}
		base := strings.ToUpper(s.BaseCoin)
		if _, excluded := bitgetNonStockRWA[base]; excluded {
			continue
		}
		si := BitgetSymbolInfo{
			Symbol:       s.Symbol,
			BaseCoin:     base,
			DisplayName:  bitgetDisplayName(base),
			PricePlace:   s.PricePlace,
			VolumePlace:  s.VolumePlace,
			FundInterval: s.FundInterval,
			MaxLever:     s.MaxLever,
		}
		info.Symbols = append(info.Symbols, si)
		info.SymbolMap[strings.ToUpper(s.Symbol)] = si
	}
	return info
}

// bitgetDisplayName 生成展示名，有中文别名时用别名。
func bitgetDisplayName(baseCoin string) string {
	base := strings.ToUpper(baseCoin)
	if alias, ok := bitgetStockAliases[base]; ok {
		return fmt.Sprintf("%s(%s)/USDT 美股永续", alias, base)
	}
	return fmt.Sprintf("%s/USDT 美股永续", base)
}

// BitgetSymbolName 返回合约展示名；/contracts 不可用时回落到 symbol 本身。
func BitgetSymbolName(symbol string) string {
	if info := GetBitgetFuturesInfo(); info != nil {
		if si, ok := info.SymbolMap[strings.ToUpper(symbol)]; ok {
			return si.DisplayName
		}
	}
	return strings.ToUpper(symbol)
}

// BitgetSymbolList 返回全部美股永续合约基础信息（品种列表用）。
func BitgetSymbolList() []BitgetSymbolInfo {
	info := GetBitgetFuturesInfo()
	if info == nil {
		return nil
	}
	return info.Symbols
}

// ResolveBitgetSymbol 把用户/AI 输入的各种写法归一到 Bitget 合约 symbol。
// 支持：aapl / 苹果 / aapl/usdt / AAPLUSDT / bt:aaplusdt。
// 只在美股 RWA 白名单内解析（防止把黄金等非股票 RWA 当美股）；
// 白名单不可用（离线）时按格式拼接兜底，但会拒绝黑名单标的。
func ResolveBitgetSymbol(input string) (string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	aliasBase := bitgetBaseFromAlias(raw)
	if IsBitgetFuturesCode(raw) {
		raw = raw[len(bitgetCodePrefix):]
	}
	normalized := strings.ToUpper(raw)
	for _, sep := range []string{"/", "-", "_", " "} {
		normalized = strings.ReplaceAll(normalized, sep, "")
	}

	info := GetBitgetFuturesInfo()
	if info != nil && len(info.SymbolMap) > 0 {
		if _, ok := info.SymbolMap[normalized]; ok {
			return normalized, true
		}
		if aliasBase != "" {
			if _, ok := info.SymbolMap[aliasBase+"USDT"]; ok {
				return aliasBase + "USDT", true
			}
		}
		if !strings.HasSuffix(normalized, "USDT") {
			if _, ok := info.SymbolMap[normalized+"USDT"]; ok {
				return normalized + "USDT", true
			}
		}
		return "", false
	}

	// 白名单不可用时的兜底
	base := aliasBase
	if base == "" {
		base = strings.TrimSuffix(normalized, "USDT")
	}
	if _, excluded := bitgetNonStockRWA[base]; excluded {
		return "", false
	}
	if base == "" {
		return "", false
	}
	return base + "USDT", true
}

// bitgetBaseFromAlias 通过中文别名或股票代码反查基础标的。
func bitgetBaseFromAlias(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	if _, ok := bitgetStockAliases[upper]; ok {
		return upper
	}
	for base, alias := range bitgetStockAliases {
		if alias == trimmed {
			return base
		}
	}
	return ""
}

// bitgetIntervalFromKlt 把项目内 klt 映射为 Bitget granularity。
// Bitget 分钟用小写（1m/5m...），小时/日/周/月用大写（1H/1D/1W/1M）。
func bitgetIntervalFromKlt(klt string) string {
	raw := strings.TrimSpace(klt)
	// 先处理大小写敏感的大写 M（月），避免被 ToLower 误判为分钟
	if raw == "1M" {
		return "1M"
	}
	switch strings.ToLower(raw) {
	case "":
		return "1D"
	case "1", "1m":
		return "1m"
	case "3", "3m":
		return "3m"
	case "5", "5m":
		return "5m"
	case "15", "15m":
		return "15m"
	case "30", "30m":
		return "30m"
	case "60", "60m", "1h":
		return "1H"
	case "120", "2h":
		return "2H"
	case "240", "4h":
		return "4H"
	case "101", "day", "日", "日k", "日k线", "1d":
		return "1D"
	case "102", "week", "周", "周k", "周k线", "1w":
		return "1W"
	case "103", "104", "105", "106", "month", "月", "月k", "季", "季k", "半年", "年", "年k":
		return "1M"
	default:
		return raw
	}
}

// bitgetCST Bitget 时间戳以 UTC 为界，展示时统一转 UTC+8，与 A 股/港股用户习惯一致。
var bitgetCST = time.FixedZone("CST", 8*60*60)

// bitgetFormatDay 把开盘时间（毫秒）格式化为 KLineData.Day。
// 分钟/小时级带时间部分，日及以上仅日期。
func bitgetFormatDay(openTimeMs int64, interval string) string {
	t := time.UnixMilli(openTimeMs).In(bitgetCST)
	if strings.HasSuffix(interval, "m") || strings.HasSuffix(interval, "H") {
		return t.Format("2006-01-02 15:04")
	}
	return t.Format("2006-01-02")
}

// bitgetEndTimeMs 把外部传入的 end 解析为毫秒时间戳；无法识别时返回 0（取最近数据）。
func bitgetEndTimeMs(end string) int64 {
	raw := strings.TrimSpace(end)
	if raw == "" {
		return 0
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if n > 1e12 {
			return n
		}
		return n * 1000
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02", "2006-01-02", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.ParseInLocation(layout, raw, bitgetCST); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}
