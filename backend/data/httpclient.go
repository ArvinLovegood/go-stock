package data

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// 代理策略总览：
//   - 全局 HttpProxy（爬虫代理）仅作用于抓取网页数据与调非 AI 大模型接口的海外场景
//     （海外新闻源、图床、GitHub 下载、TradingView/Reuters 等），作用范围由
//     HttpProxyMode 控制：overseas（默认）=仅国外站点与数据通过 GlobalProxyURL()
//     显式构建代理客户端，sharedTransport 恒直连，国内数据源不受影响；global=所有请求走代理。
//   - AI/模型请求（chat/embeddings/Agent）不使用全局代理，只看 AI 配置项自身的代理
//     开关与地址（见 agent.resolveAIProxyForConfig：单项关闭恒直连，单项地址不回退全局）。
//   - BinanceProxy（合约代理）服务币圈数据源（币安合约、OKX、Bitget 及其公告/快讯），
//     通过各自客户端的直连/代理双通道实现，与全局 HttpProxy 相互独立。

var (
	sharedTransport  *http.Transport
	sharedHTTPClient *http.Client
	SharedHTTPClient *resty.Client
	httpConfigMutex  sync.RWMutex
)

func init() {
	sharedTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 nil,
	}

	sharedHTTPClient = &http.Client{
		Transport: sharedTransport,
		Timeout:   300 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(sharedHTTPClient).
		SetRetryCount(0).
		SetTimeout(300 * time.Second)
}

func UpdateHTTPClientTimeout(timeout time.Duration) {
	sharedHTTPClient.Timeout = timeout
	SharedHTTPClient.SetTimeout(timeout)
}

func parseProxyURL(proxyURL string) *url.URL {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	return u
}

// GlobalProxyURL 返回全局「国外站点」（爬虫）代理地址；未启用或为空时返回空串（表示直连）。
// 仅用于抓取网页数据与调非 AI 大模型接口的海外场景（海外新闻源、图床、GitHub 下载等）。
// AI/模型请求不使用该值——AI 请求只看 AI 配置项的代理开关（见 agent.resolveAIProxyForConfig）。
func GlobalProxyURL() string {
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil || !cfg.HttpProxyEnabled {
		return ""
	}
	return strings.TrimSpace(cfg.HttpProxy)
}

// EnvProxyFallback 返回标准环境变量代理地址（HTTPS_PROXY/HTTP_PROXY，含小写），未设置时返回空串。
// 供海外数据源在全局设置代理未启用时回退使用：被墙站点（如 TradingView）直连会挂起超时，
// 而 Go 默认 HTTP 客户端本就遵循环境变量代理，这里保持一致的兜底行为。
func EnvProxyFallback() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// domesticHostSuffixes 已知国内站点域名后缀（国内 AI 平台、国内数据源、国内图床等）。
// overseas 抓取场景下这些域名直连，避免国内端点被代理节点断连（EOF/TLS 超时）。
var domesticHostSuffixes = []string{
	"volces.com",        // 火山引擎（Ark / 数据源）
	"aliyuncs.com",      // 阿里云（百炼 / OSS 图床）
	"aliyuncs.com.cn",   // 阿里云
	"taobao.com",        // 淘宝
	"alicdn.com",        // 阿里图床 CDN
	"taobaocdn.com",     // 淘宝 CDN
	"tencentmaas.com",   // 腾讯云 TokenHub / Token Plan
	"cloud.tencent.com", // 腾讯云（混元 / lkeap）
	"myqcloud.com",      // 腾讯云 COS 图床
	"qpic.cn",           // 腾讯图片 CDN
	"gtimg.cn",          // 腾讯图片/静态资源 CDN
	"deepseek.com",      // DeepSeek
	"bigmodel.cn",       // 智谱 AI
	"moonshot.cn",       // 月之暗面
	"minimax.cn",        // MiniMax
	"minimaxi.com",      // MiniMax 国内平台
	"xf-yun.com",        // 讯飞开放平台
	"iflytek.com",       // 讯飞
	"xiaomimimo.com",    // 小米 MiMo
	"modelscope.cn",     // 魔搭社区
	"siliconflow.cn",    // 硅基流动
	"baidu.com",         // 百度
	"bdstatic.com",      // 百度静态资源
	"eastmoney.com",     // 东方财富
	"10jqka.com.cn",     // 同花顺
	"sina.com.cn",       // 新浪
	"sinaimg.cn",        // 新浪图床
	"sohu.com",          // 搜狐
	"netease.com",       // 网易
	"126.net",           // 网易静态资源
	"jin10.com",         // 金十数据
	"cls.cn",            // 财联社
	"wallstreetcn.com",  // 华尔街见闻
	"xueqiu.com",        // 雪球
	"futunn.com",        // 富途
	"iwencai.com",       // 问财
}

// IsDomesticHost 判断主机名是否为国内/本地站点：
// localhost、无点主机名（内网服务名）、私有/环回 IP、.cn 后缀、已知国内域名后缀。
func IsDomesticHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return false
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return true
	}
	// 私有/环回/链路本地 IP
	if strings.HasPrefix(host, "127.") || strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "169.254.") {
		return true
	}
	if strings.HasSuffix(host, ".cn") {
		return true
	}
	for _, suffix := range domesticHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// ResolveProxyURLForEndpoint 按目标 URL 解析抓取类请求应使用的代理地址（空串表示直连）。
// 爬虫代理（全局）仅作用于海外目标：国内/本地目标恒直连，海外目标在全局代理启用时走代理。
func ResolveProxyURLForEndpoint(rawURL string) string {
	proxy := GlobalProxyURL()
	if proxy == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if IsDomesticHost(u.Hostname()) {
		return ""
	}
	return proxy
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	// 全局 HttpProxy 作用范围（HttpProxyMode）：
	//   - overseas（默认）：共享客户端恒直连，仅海外抓取场景（海外新闻源、图床、
	//     GitHub 下载、TradingView/Reuters 等）通过 GlobalProxyURL() + CreateProxiedHTTPClient
	//     显式走代理；AI/模型请求不在此列（只看 AI 配置项的代理开关）。
	//   - global：所有经 sharedTransport 的请求都走代理（兼容旧行为，用户显式选择）。
	proxyURL := GlobalProxyURL()
	httpConfigMutex.Lock()
	if proxyURL != "" && config.HttpProxyMode == "global" {
		if pu := parseProxyURL(proxyURL); pu != nil {
			sharedTransport.Proxy = http.ProxyURL(pu)
		} else {
			sharedTransport.Proxy = nil
		}
	} else {
		sharedTransport.Proxy = nil
	}
	httpConfigMutex.Unlock()

	if config.CrawlTimeOut > 0 {
		UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		UpdateHTTPClientTimeout(300 * time.Second)
	}
}

func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return resty.NewWithClient(httpClient).
		SetTimeout(timeout).
		SetRetryCount(0)
}

// CreateDirectHTTPClientWithTimeout 创建显式直连客户端（克隆共享 transport 但强制清除代理）。
// 供 AI/模型请求在单项代理关闭时使用：即使全局代理为「全局所有请求」模式（sharedTransport
// 带代理），AI 请求也保持直连，不被爬虫代理劫持。
func CreateDirectHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	tr := GetSharedTransport().Clone()
	tr.Proxy = nil
	httpClient := &http.Client{Transport: tr, Timeout: timeout}
	return resty.NewWithClient(httpClient).
		SetTimeout(timeout).
		SetRetryCount(0)
}

// CreateProxiedHTTPClient 创建带可选代理的独立 HTTP 客户端。
// transport 克隆自共享 sharedTransport（保留连接池等参数），代理只作用于本 client，
// 不会修改全局共享客户端。proxyURL 为空时不启用代理。
func CreateProxiedHTTPClient(timeout time.Duration, proxyURL string) *resty.Client {
	tr := GetSharedTransport().Clone()
	if proxyURL != "" {
		if pu := parseProxyURL(proxyURL); pu != nil {
			tr.Proxy = http.ProxyURL(pu)
		}
	}
	httpClient := &http.Client{Transport: tr, Timeout: timeout}
	return resty.NewWithClient(httpClient).
		SetTimeout(timeout).
		SetRetryCount(0)
}

// CreateGlobalProxiedHTTPClient 创建「国外站点」客户端：启用全局 HttpProxy 时走代理，否则直连。
func CreateGlobalProxiedHTTPClient(timeout time.Duration) *resty.Client {
	return CreateProxiedHTTPClient(timeout, GlobalProxyURL())
}

func CreateDownloadClient() *resty.Client {
	downloadTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		// 下载走 GitHub 镜像源（国内可达），保持直连，不使用配置的代理
		Proxy: nil,
	}

	downloadHTTPClient := &http.Client{
		Transport: downloadTransport,
		Timeout:   0,
	}

	return resty.NewWithClient(downloadHTTPClient).
		SetTimeout(0).
		SetRetryCount(2).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(30 * time.Second)
}

// GetSharedTransport 返回共享 HTTP Transport（恒直连，不含代理），
// 供各模块克隆使用；需要代理的场景请用 CreateProxiedHTTPClient。
func GetSharedTransport() *http.Transport {
	httpConfigMutex.RLock()
	defer httpConfigMutex.RUnlock()
	return sharedTransport
}
