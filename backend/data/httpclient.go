package data

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// 代理策略总览：
//   - 全局 HttpProxy 作用范围由 HttpProxyMode 控制：overseas（默认）=仅国外站点与数据
//     （AI/模型请求、图床上传、GitHub 下载、TradingView/Reuters 等）通过 GlobalProxyURL()
//     显式构建代理客户端，sharedTransport 恒直连，国内数据源不受影响；global=所有请求走代理。
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

// GlobalProxyURL 返回全局「国外站点」代理地址；未启用或为空时返回空串（表示直连）。
// 仅海外场景（AI 请求、图床、GitHub 下载、海外新闻源等）应使用该值构建代理客户端。
func GlobalProxyURL() string {
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil || !cfg.HttpProxyEnabled {
		return ""
	}
	return strings.TrimSpace(cfg.HttpProxy)
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	// 全局 HttpProxy 作用范围（HttpProxyMode）：
	//   - overseas（默认）：共享客户端恒直连，仅海外场景（AI、图床、GitHub 下载、
	//     TradingView/Reuters 等）通过 GlobalProxyURL() + CreateProxiedHTTPClient 显式走代理；
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
