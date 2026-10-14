package data

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

// OKX（欧易）REST 客户端。
//
// 网络策略与币安一致：直连优先，失败自动降级到设置里的「合约代理」（Settings.BinanceProxy，
// 与币安/Bitget 共用同一设置项）。私有接口需 HMAC-SHA256 签名：
//
//	预签名字符串 = timestamp + method + requestPath + body（GET 的 requestPath 含 query）
//	OK-ACCESS-SIGN = base64(hmac_sha256(secretKey, 预签名字符串))
//	OK-ACCESS-TIMESTAMP 为 ISO 8601 UTC（如 2020-12-08T09:08:57.715Z）
//
// 凭证（APIKey/SecretKey/Passphrase）与模拟盘开关存于 Settings 表，Passphrase 是创建
// 密钥时设置的「口令」而非备注名。API 密钥需绑定出口 IP 白名单。

const (
	okxBaseURL        = "https://www.okx.com"
	okxCodePrefix     = "ok:"
	okxDirectTimeout  = 5 * time.Second
	okxProxyTimeout   = 12 * time.Second
	okxProxyCooldown  = 5 * time.Minute
	okxMaxConcurrency = 4
)

// okxISO8601 OKX 签名时间戳格式（UTC，毫秒精度，字面 Z）。
const okxISO8601 = "2006-01-02T15:04:05.000Z"

// okxCodePrefixOf OKX 合约代码前缀（ok:btcusdt-swap 形式）。
func IsOKXCode(code string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(code)), "OK:")
}

// ===== 凭证 =====

type okxCredentials struct {
	apiKey     string
	secretKey  string
	passphrase string
	simulated  bool
}

// okxGetCredentials 读取设置中的 OKX 凭证；未配置时返回 nil。
// 模拟盘与实盘密钥是两套（OKX 模拟环境创建的密钥仅模拟环境可用），按「模拟盘」开关自动选用。
func okxGetCredentials() *okxCredentials {
	simulated := true
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		simulated = cfg.OKXSimulated
	}
	return okxGetCredentialsForEnv(simulated)
}

// okxGetCredentialsForEnv 按指定环境读取凭证：simulated=true 取模拟盘密钥，false 取实盘密钥。
// 模拟盘与实盘密钥是两套（OKX 模拟环境创建的密钥仅模拟环境可用）；
// 信号自动交易双环境并行时，按环境各取各的密钥。未配置时返回 nil。
func okxGetCredentialsForEnv(simulated bool) *okxCredentials {
	if db.Dao == nil {
		return nil
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return nil
	}
	key := strings.TrimSpace(cfg.OKXApiKey)
	secret := strings.TrimSpace(cfg.OKXSecretKey)
	pass := strings.TrimSpace(cfg.OKXPassphrase)
	if !simulated { // 实盘：使用实盘密钥
		key = strings.TrimSpace(cfg.OKXLiveApiKey)
		secret = strings.TrimSpace(cfg.OKXLiveSecretKey)
		pass = strings.TrimSpace(cfg.OKXLivePassphrase)
	}
	if key == "" || secret == "" || pass == "" {
		return nil
	}
	return &okxCredentials{apiKey: key, secretKey: secret, passphrase: pass, simulated: simulated}
}

// okxCredMissingError 按当前环境给出针对性的凭证缺失提示。
func okxCredMissingError() error {
	simulated := true
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		simulated = cfg.OKXSimulated
	}
	return okxCredMissingErrorForEnv(simulated)
}

// okxCredMissingErrorForEnv 按指定环境给出针对性的凭证缺失提示。
func okxCredMissingErrorForEnv(simulated bool) error {
	if !simulated {
		return errors.New("实盘环境未配置实盘 API 凭证，请在设置页填写「实盘 API Key/SecretKey/口令」")
	}
	return okxMissingCredError
}

// okxTradeEnabled 交易开关：关闭时 AI 不允许下单/撤单（查询不受限）。
func okxTradeEnabled() bool {
	if db.Dao == nil {
		return false
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return false
	}
	return cfg.OKXEnableTrade
}

// ===== 双通道 HTTP =====

type okxChannelState struct {
	mu          sync.Mutex
	preferProxy bool
	lastFailAt  time.Time
}

var okxChannel okxChannelState

var (
	okxDirectOnce sync.Once
	okxDirectCli  *resty.Client

	okxProxyMu  sync.Mutex
	okxProxyCli *resty.Client
	okxProxyURL string
)

var okxSemaphore = make(chan struct{}, okxMaxConcurrency)

// okxHTTPError OKX 返回的非 200 响应。
type okxHTTPError struct {
	statusCode int
	body       string
}

func (e *okxHTTPError) Error() string {
	return fmt.Sprintf("okx http %d: %s", e.statusCode, truncateForLog(e.body))
}

// isOKXChannelFailure 判断错误是否应触发通道切换；业务错误（签名失败、参数错误）不切换。
func isOKXChannelFailure(err error) bool {
	var he *okxHTTPError
	if errors.As(err, &he) {
		switch he.statusCode {
		case http.StatusRequestTimeout, 418, http.StatusTooManyRequests,
			http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	return true
}

func okxDirectClient() *resty.Client {
	okxDirectOnce.Do(func() {
		tr := GetSharedTransport().Clone()
		tr.Proxy = nil
		hc := &http.Client{Transport: tr, Timeout: okxDirectTimeout}
		okxDirectCli = resty.NewWithClient(hc).SetTimeout(okxDirectTimeout).SetRetryCount(0)
	})
	return okxDirectCli
}

// okxProxyClient 返回 OKX 专用代理 client；未配置时返回 nil。
// 与币安合约共用「合约代理」设置项（BinanceProxy），币圈数据源统一走这一份代理配置。
func okxProxyClient() *resty.Client {
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

	okxProxyMu.Lock()
	defer okxProxyMu.Unlock()
	if okxProxyCli != nil && okxProxyURL == proxyURL {
		return okxProxyCli
	}
	pu, err := url.Parse(proxyURL)
	if err != nil || pu.Scheme == "" || pu.Host == "" {
		return nil
	}
	tr := GetSharedTransport().Clone()
	tr.Proxy = http.ProxyURL(pu)
	hc := &http.Client{Transport: tr, Timeout: okxProxyTimeout}
	okxProxyCli = resty.NewWithClient(hc).SetTimeout(okxProxyTimeout).SetRetryCount(0)
	okxProxyURL = proxyURL
	return okxProxyCli
}

func okxPreferProxy() bool {
	okxChannel.mu.Lock()
	defer okxChannel.mu.Unlock()
	if !okxChannel.preferProxy {
		return false
	}
	return time.Since(okxChannel.lastFailAt) <= okxProxyCooldown
}

func okxMarkDirectFailed() {
	okxChannel.mu.Lock()
	okxChannel.preferProxy = true
	okxChannel.lastFailAt = time.Now()
	okxChannel.mu.Unlock()
}

func okxMarkDirectOK() {
	okxChannel.mu.Lock()
	okxChannel.preferProxy = false
	okxChannel.mu.Unlock()
}

// okxSign 计算 OK-ACCESS-SIGN。
func okxSign(secretKey, timestamp, method, requestPath, body string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(timestamp + method + requestPath + body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// okxDoOnce 通过指定通道发起一次 OKX 请求并校验业务码。
// params 仅用于 GET；bodyObj 仅用于 POST。
func okxDoOnce(ctx context.Context, cli *resty.Client, cred *okxCredentials, method, path string, params url.Values, bodyObj any) ([]byte, error) {
	if cli == nil {
		return nil, errors.New("OKX HTTP 客户端不可用")
	}

	bodyStr := ""
	if bodyObj != nil {
		raw, err := json.Marshal(bodyObj)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyStr = string(raw)
	}

	requestPath := path
	query := ""
	if method == http.MethodGet && len(params) > 0 {
		query = params.Encode()
		requestPath = path + "?" + query
	}

	req := cli.R().SetContext(ctx)
	// GET 查询串已并入 requestPath（第 258 行最终请求也使用 requestPath），
	// 此处不可再 SetQueryParams，否则 URL 重复拼接导致签名不匹配（OKX 50113）。
	if bodyStr != "" {
		req.SetBody(bodyStr)
	}
	req.SetHeader("Content-Type", "application/json")
	// 模拟盘标记（公共行情接口带此头也无副作用）
	if cred != nil && cred.simulated {
		req.SetHeader("x-simulated-trading", "1")
	}
	// 私有接口签名
	if cred != nil {
		ts := time.Now().UTC().Format(okxISO8601)
		req.SetHeader("OK-ACCESS-KEY", cred.apiKey)
		req.SetHeader("OK-ACCESS-SIGN", okxSign(cred.secretKey, ts, method, requestPath, bodyStr))
		req.SetHeader("OK-ACCESS-TIMESTAMP", ts)
		req.SetHeader("OK-ACCESS-PASSPHRASE", cred.passphrase)
	}

	var resp *resty.Response
	var err error
	switch method {
	case http.MethodPost:
		resp, err = req.Post(okxBaseURL + path)
	default:
		resp, err = req.Get(okxBaseURL + requestPath)
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, &okxHTTPError{statusCode: resp.StatusCode(), body: truncateForLog(resp.String())}
	}

	// 业务码校验：code=="0" 成功；否则把 OKX 错误信息带回
	var env struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &env); err != nil {
		return nil, fmt.Errorf("解析 OKX 响应失败: %w", err)
	}
	if env.Code != "0" {
		// 下单等接口失败时，明细错误在 data[0].sCode/sMsg（顶层 code=1 "All operations failed"）
		if len(env.Data) > 0 {
			var d struct {
				SCode string `json:"sCode"`
				SMsg  string `json:"sMsg"`
			}
			if json.Unmarshal(env.Data[0], &d) == nil && (d.SMsg != "" || d.SCode != "") {
				return nil, &okxBizError{code: d.SCode, msg: d.SMsg}
			}
		}
		return nil, &okxBizError{code: env.Code, msg: env.Msg}
	}
	return resp.Body(), nil
}

// okxBizError OKX 业务错误（HTTP 200 但 code != "0"）。
type okxBizError struct {
	code string
	msg  string
}

func (e *okxBizError) Error() string {
	return fmt.Sprintf("OKX 错误 %s: %s", e.code, e.msg)
}

// okxMissingCredError 凭证未配置。
var okxMissingCredError = errors.New("未配置 OKX API 凭证，请在设置页填写 APIKey/SecretKey/Passphrase（口令）")

// doOKXRequest 带双通道降级的 OKX 请求。private=true 时要求凭证，未配置直接报错。
func doOKXRequest(ctx context.Context, method, path string, params url.Values, bodyObj any, private bool) ([]byte, error) {
	var cred *okxCredentials
	if private {
		cred = okxGetCredentials()
		if cred == nil {
			return nil, okxCredMissingError()
		}
	}
	return doOKXRequestWithCred(ctx, cred, method, path, params, bodyObj)
}

// doOKXRequestWithCred 带双通道降级的 OKX 请求；cred 为 nil 表示公共接口。
// 允许调用方传入定制凭证（如信号模拟交易强制 simulated 标记）。
func doOKXRequestWithCred(ctx context.Context, cred *okxCredentials, method, path string, params url.Values, bodyObj any) ([]byte, error) {
	okxSemaphore <- struct{}{}
	defer func() { <-okxSemaphore }()

	attemptDirectFirst := !okxPreferProxy()
	var lastErr error
	var proxyErr, directErr error

	if attemptDirectFirst {
		body, err := okxDoOnce(ctx, okxDirectClient(), cred, method, path, params, bodyObj)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isOKXChannelFailure(err) {
			return nil, err
		}
		okxMarkDirectFailed()
		directErr = err
		logger.SugaredLogger.Warnf("OKX直连失败，尝试代理通道: path=%s err=%v", path, err)
	}

	if proxyCli := okxProxyClient(); proxyCli != nil {
		body, err := okxDoOnce(ctx, proxyCli, cred, method, path, params, bodyObj)
		if err == nil {
			return body, nil
		}
		lastErr = err
		proxyErr = err
		logger.SugaredLogger.Warnf("OKX代理通道失败: path=%s err=%v", path, err)
	}

	if !attemptDirectFirst {
		// 代理优先期间仍失败，兜底回试直连
		body, err := okxDoOnce(ctx, okxDirectClient(), cred, method, path, params, bodyObj)
		if err == nil {
			okxMarkDirectOK()
			return body, nil
		}
		lastErr = err
		directErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("OKX 接口不可达，请在设置中配置合约代理")
	}
	// 双通道均失败时合并报错，避免代理错误被直连错误掩盖
	if proxyErr != nil && directErr != nil {
		lastErr = fmt.Errorf("代理通道: %v；直连通道: %w", proxyErr, directErr)
	}
	return nil, lastErr
}

// ===== 结果缓存 =====

type okxCacheEntry struct {
	body      []byte
	expiresAt time.Time
}

var (
	okxCacheMu sync.Mutex
	okxCache   = map[string]okxCacheEntry{}

	okxFlightMu sync.Mutex
	okxFlights  = map[string]*sync.Mutex{}
)

func okxFlightLock(key string) *sync.Mutex {
	okxFlightMu.Lock()
	defer okxFlightMu.Unlock()
	m, ok := okxFlights[key]
	if !ok {
		m = &sync.Mutex{}
		okxFlights[key] = m
	}
	return m
}

// okxCachedRequest 带 TTL 缓存与同 key 请求去重的请求入口（仅用于公共只读端点）。
func okxCachedRequest(ctx context.Context, key string, ttl time.Duration, method, path string, params url.Values) ([]byte, error) {
	return okxCachedRequestPriv(ctx, key, ttl, method, path, params, false)
}

// okxCachedRequestPriv 带缓存与单飞锁的 OKX 请求；private=true 走签名通道
// （缓存键需自带维度区分，如 "account-instruments:instType=SWAP"）。
func okxCachedRequestPriv(ctx context.Context, key string, ttl time.Duration, method, path string, params url.Values, private bool) ([]byte, error) {
	if ttl > 0 {
		okxCacheMu.Lock()
		if e, ok := okxCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			okxCacheMu.Unlock()
			return body, nil
		}
		okxCacheMu.Unlock()
	}

	lock := okxFlightLock(key)
	lock.Lock()
	defer lock.Unlock()

	if ttl > 0 {
		okxCacheMu.Lock()
		if e, ok := okxCache[key]; ok && time.Now().Before(e.expiresAt) {
			body := e.body
			okxCacheMu.Unlock()
			return body, nil
		}
		okxCacheMu.Unlock()
	}

	body, err := doOKXRequest(ctx, method, path, params, nil, private)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		okxCacheMu.Lock()
		okxCache[key] = okxCacheEntry{body: body, expiresAt: time.Now().Add(ttl)}
		okxCacheMu.Unlock()
	}
	return body, nil
}

// okxFloatNum 解析 OKX 返回的小数字符串；失败返回 0。
func okxFloatNum(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// okxCellFloat OKX 数组行内单元格转数值（兼容字符串与数字混排）。
func okxCellFloat(v any) float64 {
	switch x := v.(type) {
	case string:
		return okxFloatNum(x)
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

// okxTrimNum 归一 OKX 小数字符串（去冗余尾零）。
func okxTrimNum(s string) string {
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

// okxData 解析业务数据数组中的第一个元素。
func okxData(body []byte, v any) error {
	var env struct {
		Code string            `json:"code"`
		Msg  string            `json:"msg"`
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("解析 OKX 响应失败: %w", err)
	}
	if env.Code != "" && env.Code != "0" {
		return &okxBizError{code: env.Code, msg: env.Msg}
	}
	if len(env.Data) == 0 {
		return errors.New("OKX 返回数据为空")
	}
	if err := json.Unmarshal(env.Data[0], v); err != nil {
		return fmt.Errorf("解析 OKX 业务数据失败: %w（响应片段: %.200s）", err, string(body))
	}
	return nil
}

// okxDataList 解析业务数据数组。
func okxDataList(body []byte, v any) error {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("解析 OKX 响应失败: %w", err)
	}
	return json.Unmarshal(env.Data, v)
}
