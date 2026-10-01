package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/go-resty/resty/v2"
)

// 币安 USDT-M 永续合约客户端。
//
// 网络策略：直连优先，失败自动降级到用户在设置里配置的 HTTP 代理（双通道）。
// 注意：不能直接复用 CreateHTTPClientWithTimeout —— ConfigureFromSettings 会把用户的
// HttpProxy 写进全局 sharedTransport.Proxy，直接复用会导致「直连实际走了代理」。
// 因此直连 client 取 sharedTransport.Clone() 后显式置 Proxy=nil。

const (
	binanceFapiBase       = "https://fapi.binance.com"
	binanceCodePrefix     = "bn:"
	binanceDirectTimeout  = 5 * time.Second
	binanceProxyTimeout   = 12 * time.Second
	binanceProxyCooldown  = 5 * time.Minute
	binanceMaxConcurrency = 4
)

// IsBinanceFuturesCode 判断代码是否为币安永续合约（bn: 前缀，大小写不敏感）。
// 与现有 sh/sz/bj、hk、gb_、.US、.CSI、100.XXX 前缀均不冲突。
func IsBinanceFuturesCode(code string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(code)), "BN:")
}

// binanceBaseAliases 主流币种中文别名，用于展示名与 AI 参数容错。
var binanceBaseAliases = map[string]string{
	"BTC": "比特币", "ETH": "以太坊", "SOL": "Solana", "BNB": "币安币",
	"XRP": "瑞波币", "DOGE": "狗狗币", "ADA": "艾达币", "TRX": "波场",
	"AVAX": "雪崩", "LINK": "Chainlink", "DOT": "波卡", "MATIC": "Polygon",
	"LTC": "莱特币", "BCH": "比特现金", "UNI": "Uniswap", "ATOM": "Cosmos",
	"ETC": "以太经典", "FIL": "Filecoin", "APT": "Aptos", "ARB": "Arbitrum",
	"OP": "Optimism", "SUI": "Sui", "NEAR": "NEAR", "ICP": "ICP",
	"PEPE": "PEPE", "SHIB": "柴犬币", "TON": "Toncoin", "SEI": "Sei",
	"TIA": "Celestia", "INJ": "Injective", "AAVE": "Aave", "MKR": "Maker",
	"CRV": "Curve", "LDO": "Lido", "RUNE": "Thorchain", "GALA": "Gala",
	"FTM": "Fantom", "ALGO": "Algorand", "XLM": "恒星币", "EOS": "柚子币",
	"ZEC": "Zcash", "DASH": "达世币", "XTZ": "Tezos", "SAND": "The Sandbox",
	"MANA": "Decentraland", "AXS": "Axie", "GMT": "STEPN", "WLD": "Worldcoin",
	"ORDI": "Ordinals", "1000PEPE": "PEPE(1000)", "1000SHIB": "柴犬币(1000)",
}

// ===== 双通道 HTTP =====

type binanceChannelState struct {
	mu          sync.Mutex
	preferProxy bool
	lastFailAt  time.Time
}

var binanceChannel binanceChannelState

var (
	binanceDirectOnce sync.Once
	binanceDirectCli  *resty.Client

	binanceProxyMu  sync.Mutex
	binanceProxyCli *resty.Client
	binanceProxyURL string
)

// binanceSemaphore 限制币安请求并发，控制 weight 消耗。
var binanceSemaphore = make(chan struct{}, binanceMaxConcurrency)

// binanceHTTPError 币安返回的非 200 响应。
type binanceHTTPError struct {
	statusCode int
	body       string
}

func (e *binanceHTTPError) Error() string {
	return fmt.Sprintf("binance http %d: %s", e.statusCode, e.body)
}

// isBinanceChannelFailure 判断错误是否应触发通道切换。
// 限流/封禁/服务端错误与网络层错误视为通道失败；业务错误（如 -1121 invalid symbol）不切换。
func isBinanceChannelFailure(err error) bool {
	var he *binanceHTTPError
	if errors.As(err, &he) {
		switch he.statusCode {
		case http.StatusForbidden, http.StatusRequestTimeout, 418, http.StatusTooManyRequests,
			http.StatusUnavailableForLegalReasons, http.StatusInternalServerError,
			http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	return true
}

func binanceDirectClient() *resty.Client {
	binanceDirectOnce.Do(func() {
		tr := GetSharedTransport().Clone()
		tr.Proxy = nil
		hc := &http.Client{Transport: tr, Timeout: binanceDirectTimeout}
		binanceDirectCli = resty.NewWithClient(hc).SetTimeout(binanceDirectTimeout).SetRetryCount(0)
	})
	return binanceDirectCli
}

// binanceProxyClient 返回币安专用代理 client；未配置「币安合约代理」时返回 nil。
// 只读取 Settings.BinanceProxy（与全局 HttpProxy 隔离），避免影响国内数据源的直连行为。
// 代理地址变化时会重建 client。
func binanceProxyClient() *resty.Client {
	// DB 未初始化时（如单元测试）无法读取设置，退化为仅直连通道
	if db.Dao == nil {
		return nil
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return nil
	}
	proxyURL := strings.TrimSpace(cfg.BinanceProxy)
	if proxyURL == "" {
		return nil
	}

	binanceProxyMu.Lock()
	defer binanceProxyMu.Unlock()
	if binanceProxyCli != nil && binanceProxyURL == proxyURL {
		return binanceProxyCli
	}
	pu, err := url.Parse(proxyURL)
	if err != nil || pu.Scheme == "" || pu.Host == "" {
		return nil
	}
	tr := GetSharedTransport().Clone()
	tr.Proxy = http.ProxyURL(pu)
	hc := &http.Client{Transport: tr, Timeout: binanceProxyTimeout}
	binanceProxyCli = resty.NewWithClient(hc).SetTimeout(binanceProxyTimeout).SetRetryCount(0)
	binanceProxyURL = proxyURL
	return binanceProxyCli
}

func binancePreferProxy() bool {
	binanceChannel.mu.Lock()
	defer binanceChannel.mu.Unlock()
	if !binanceChannel.preferProxy {
		return false
	}
	// 冷却到期后重新尝试直连
	return time.Since(binanceChannel.lastFailAt) <= binanceProxyCooldown
}

func binanceMarkDirectFailed() {
	binanceChannel.mu.Lock()
	binanceChannel.preferProxy = true
	binanceChannel.lastFailAt = time.Now()
	binanceChannel.mu.Unlock()
}

func binanceMarkDirectOK() {
	binanceChannel.mu.Lock()
	binanceChannel.preferProxy = false
	binanceChannel.mu.Unlock()
}

func binanceDoOnce(ctx context.Context, cli *resty.Client, path string, params url.Values) ([]byte, error) {
	if cli == nil {
		return nil, errors.New("币安 HTTP 客户端不可用")
	}
	req := cli.R().SetContext(ctx)
	if len(params) > 0 {
		req.SetQueryParamsFromValues(params)
	}
	resp, err := req.Get(binanceFapiBase + path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, &binanceHTTPError{statusCode: resp.StatusCode(), body: truncateForLog(resp.String())}
	}
	return resp.Body(), nil
}

func truncateForLog(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// doBinanceRequest 带双通道降级的币安 GET 请求。
// 直连失败（网络错误或限流/封禁类状态码）时自动改用设置中的代理重试；
// 代理通道优先期间若代理也失败，会兜底回试一次直连。双通道均失败时返回原始错误。
func doBinanceRequest(ctx context.Context, path string, params url.Values) ([]byte, error) {
	binanceSemaphore <- struct{}{}
	defer func() { <-binanceSemaphore }()

	attemptDirectFirst := !binancePreferProxy()
	var lastErr error

	if attemptDirectFirst {
		body, err := binanceDoOnce(ctx, binanceDirectClient(), path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isBinanceChannelFailure(err) {
			return nil, err
		}
		binanceMarkDirectFailed()
		logger.SugaredLogger.Warnf("币安直连失败，尝试代理通道: path=%s err=%v", path, err)
	}

	if proxyCli := binanceProxyClient(); proxyCli != nil {
		body, err := binanceDoOnce(ctx, proxyCli, path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}

	if !attemptDirectFirst {
		// 代理优先期间仍失败，兜底回试直连，成功则恢复直连优先
		body, err := binanceDoOnce(ctx, binanceDirectClient(), path, params)
		if err == nil {
			binanceMarkDirectOK()
			return body, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("币安接口不可达，请在设置中配置 HTTP 代理")
	}
	return nil, lastErr
}

// ===== 结果缓存 =====

type binanceCacheEntry struct {
	body      []byte
	expiresAt time.Time
}

var (
	binanceCacheMu sync.Mutex
	binanceCache   = map[string]binanceCacheEntry{}

	binanceFlightMu sync.Mutex
	binanceFlights  = map[string]*sync.Mutex{}
)

func binanceFlightLock(key string) *sync.Mutex {
	binanceFlightMu.Lock()
	defer binanceFlightMu.Unlock()
	m, ok := binanceFlights[key]
	if !ok {
		m = &sync.Mutex{}
		binanceFlights[key] = m
	}
	return m
}

// binanceCachedRequest 带 TTL 缓存与同 key 请求去重的请求入口。
func binanceCachedRequest(ctx context.Context, key string, ttl time.Duration, path string, params url.Values) ([]byte, error) {
	if ttl > 0 {
		binanceCacheMu.Lock()
		if e, ok := binanceCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			binanceCacheMu.Unlock()
			return body, nil
		}
		binanceCacheMu.Unlock()
	}

	lock := binanceFlightLock(key)
	lock.Lock()
	defer lock.Unlock()

	// 等锁期间可能已被其它 goroutine 填充
	if ttl > 0 {
		binanceCacheMu.Lock()
		if e, ok := binanceCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			binanceCacheMu.Unlock()
			return body, nil
		}
		binanceCacheMu.Unlock()
	}

	body, err := doBinanceRequest(ctx, path, params)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		binanceCacheMu.Lock()
		binanceCache[key] = binanceCacheEntry{body: body, expiresAt: time.Now().Add(ttl)}
		binanceCacheMu.Unlock()
	}
	return body, nil
}

// ===== exchangeInfo =====

const binanceExchangeInfoTTL = 6 * time.Hour

var (
	binanceExchangeInfoMu   sync.RWMutex
	binanceExchangeInfoData *BinanceExchangeInfo
)

// GetBinanceExchangeInfo 获取 USDT-M 永续合约基础信息（6 小时缓存）。
// 拉取失败时沿用旧缓存；两者皆无时返回 nil，调用方需降级处理。
func GetBinanceExchangeInfo() *BinanceExchangeInfo {
	binanceExchangeInfoMu.RLock()
	cached := binanceExchangeInfoData
	binanceExchangeInfoMu.RUnlock()
	if cached != nil && time.Since(cached.FetchedAt) < binanceExchangeInfoTTL {
		return cached
	}

	info := loadBinanceExchangeInfo()
	if info == nil {
		return cached
	}
	binanceExchangeInfoMu.Lock()
	binanceExchangeInfoData = info
	binanceExchangeInfoMu.Unlock()
	return info
}

func loadBinanceExchangeInfo() *BinanceExchangeInfo {
	ctx, cancel := context.WithTimeout(context.Background(), binanceProxyTimeout)
	defer cancel()

	body, err := doBinanceRequest(ctx, "/fapi/v1/exchangeInfo", nil)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安 exchangeInfo 失败: %v", err)
		return nil
	}

	var raw struct {
		Symbols []struct {
			Symbol         string `json:"symbol"`
			ContractType   string `json:"contractType"`
			Status         string `json:"status"`
			BaseAsset      string `json:"baseAsset"`
			QuoteAsset     string `json:"quoteAsset"`
			PricePrecision int    `json:"pricePrecision"`
			OnboardDate    int64  `json:"onboardDate"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		logger.SugaredLogger.Warnf("解析币安 exchangeInfo 失败: %v", err)
		return nil
	}

	info := &BinanceExchangeInfo{
		Symbols:   make([]BinanceSymbolInfo, 0, len(raw.Symbols)),
		SymbolMap: make(map[string]BinanceSymbolInfo, len(raw.Symbols)),
		FetchedAt: time.Now(),
	}
	for _, s := range raw.Symbols {
		contractType := strings.ToUpper(strings.TrimSpace(s.ContractType))
		// 接纳普通永续（PERPETUAL）与 TradFi 永续（TRADIFI_PERPETUAL，含美股/ETF/港股/大宗商品等）
		if (contractType != "PERPETUAL" && contractType != "TRADIFI_PERPETUAL") ||
			!strings.EqualFold(s.Status, "TRADING") ||
			!strings.EqualFold(s.QuoteAsset, "USDT") {
			continue
		}
		base := strings.ToUpper(s.BaseAsset)
		isTradFi := contractType == "TRADIFI_PERPETUAL"
		si := BinanceSymbolInfo{
			Symbol:         s.Symbol,
			BaseAsset:      base,
			DisplayName:    binanceDisplayName(base, isTradFi),
			PricePrecision: s.PricePrecision,
			OnboardDate:    s.OnboardDate,
			IsTradFi:       isTradFi,
		}
		info.Symbols = append(info.Symbols, si)
		info.SymbolMap[strings.ToUpper(s.Symbol)] = si
	}
	return info
}

// binanceDisplayName 生成展示名。加密永续用「币种/USDT 永续」；
// TradFi 永续（美股/ETF/大宗商品等）标注「美股永续」并用中文别名区分。
func binanceDisplayName(baseAsset string, isTradFi bool) string {
	base := strings.ToUpper(baseAsset)
	alias, ok := binanceBaseAliases[base]
	if isTradFi {
		if !ok || alias == base {
			alias, ok = bitgetStockAliases[base]
		}
		if ok && alias != base {
			return fmt.Sprintf("%s(%s)/USDT 美股永续", alias, base)
		}
		return fmt.Sprintf("%s/USDT 美股永续", base)
	}
	if ok {
		return fmt.Sprintf("%s/USDT 永续", alias)
	}
	return fmt.Sprintf("%s/USDT 永续", base)
}

// SymbolName 返回合约展示名；exchangeInfo 不可用时回落到 symbol 本身。
func SymbolName(symbol string) string {
	if info := GetBinanceExchangeInfo(); info != nil {
		if si, ok := info.SymbolMap[strings.ToUpper(symbol)]; ok {
			return si.DisplayName
		}
	}
	return strings.ToUpper(symbol)
}

// ResolveBinanceSymbol 把用户/AI 输入的各种写法归一到币安合约 symbol。
// 支持：btc / 比特币 / btc/usdt / BTCUSDT / bn:btcusdt。
// 正常路径会经 exchangeInfo 校验；exchangeInfo 不可用（离线）时按格式拼接兜底。
func ResolveBinanceSymbol(input string) (string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	aliasBase := binanceBaseFromAlias(raw)
	if IsBinanceFuturesCode(raw) {
		raw = raw[len(binanceCodePrefix):]
	}
	normalized := strings.ToUpper(raw)
	for _, sep := range []string{"/", "-", "_", " "} {
		normalized = strings.ReplaceAll(normalized, sep, "")
	}

	info := GetBinanceExchangeInfo()
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

	// exchangeInfo 不可用时的兜底
	if aliasBase != "" {
		return aliasBase + "USDT", true
	}
	if strings.HasSuffix(normalized, "USDT") || strings.HasSuffix(normalized, "USDC") {
		return normalized, true
	}
	if normalized == "" {
		return "", false
	}
	return normalized + "USDT", true
}

// binanceBaseFromAlias 通过中文别名或币种名反查基础币种。
func binanceBaseFromAlias(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	if _, ok := binanceBaseAliases[upper]; ok {
		return upper
	}
	// TradFi 美股/ETF 的 base 也接受直接输入（如 AAPL、TSLA）
	if _, ok := bitgetStockAliases[upper]; ok {
		return upper
	}
	for base, alias := range binanceBaseAliases {
		if alias == trimmed {
			return base
		}
	}
	for base, alias := range bitgetStockAliases {
		if alias == trimmed {
			return base
		}
	}
	return ""
}

// binanceIntervalFromKlt 把项目内 klt 映射为币安 interval。
// 1/5/15/30/60/120/240 为分钟线；101/102/103/104/105/106 为日/周/月以上。
func binanceIntervalFromKlt(klt string) string {
	raw := strings.TrimSpace(klt)
	// 先处理大小写敏感的大写 M（月），避免被 ToLower 误判为分钟
	if raw == "1M" {
		return "1M"
	}
	switch strings.ToLower(raw) {
	case "":
		return "1d"
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
		return "1h"
	case "120", "2h":
		return "2h"
	case "240", "4h":
		return "4h"
	case "101", "day", "日", "日k", "日k线", "1d":
		return "1d"
	case "102", "week", "周", "周k", "周k线", "1w":
		return "1w"
	case "103", "104", "105", "106", "month", "月", "月k", "季", "季k", "半年", "年", "年k":
		return "1M"
	default:
		return raw
	}
}

// binanceFormatDay 把开盘时间（毫秒）格式化为 KLineData.Day。
// 分钟/小时级带时间部分，日及以上仅日期；统一按 UTC（币安日K 以 UTC 00:00 为界）。
func binanceFormatDay(openTimeMs int64, interval string) string {
	t := time.UnixMilli(openTimeMs).UTC()
	if strings.HasSuffix(interval, "m") || strings.HasSuffix(interval, "h") {
		return t.Format("2006-01-02 15:04")
	}
	return t.Format("2006-01-02")
}