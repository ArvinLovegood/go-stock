package data

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

const (
	nodesSourceURL   = "https://raw.githubusercontent.com/oopsunix/ghproxy-next/main/components/nodes.ts"
	proxyTestBytes   = 2 * 1024 * 1024
	proxyTestTimeout = 15 * time.Second
	proxyTestTopN    = 20
	proxyConcurrency = 10
)

var hardcodedProxies = []string{
	"github.starrlzy.cn", "ghproxy.net", "gh-proxy.com",
	"github-proxy.memory-echoes.cn", "github.dpik.top", "gh.tryxd.cn",
	"github.tbap.top", "j.1win.ggff.net", "ghfile.geekertao.top",
	"gh-proxy.net", "cdn.gh-proxy.com", "gh.acmsz.top", "ghfast.top",
	"gh.idayer.com", "git.yylx.win", "ghproxy.1888866.xyz",
	"gh.chjina.com", "gh.noki.icu",
}

var nodesValueRe = regexp.MustCompile(`value:\s*"([^"]+)"`)

// FetchProxyCandidates 从 oopsunix/ghproxy-next 的 nodes.ts 抓取代理节点列表，
// 过滤非 ASCII 域名（IDN 域名无法直接用于 URL），合并硬编码列表（硬编码优先）。
// 抓取失败时回退到纯硬编码列表。
func FetchProxyCandidates() []string {
	candidates := []string{}
	seen := map[string]bool{}

	// 1) 从 nodes.ts 抓取
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nodesSourceURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.SugaredLogger.Warnf("抓取代理节点列表失败: %s, 使用内置列表", err.Error())
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			for _, m := range nodesValueRe.FindAllStringSubmatch(line, -1) {
				domain := strings.TrimSpace(m[1])
				if domain == "" || !isASCII(domain) || seen[domain] {
					continue
				}
				seen[domain] = true
				candidates = append(candidates, domain)
			}
		}
		logger.SugaredLogger.Infof("从 nodes.ts 抓取到 %d 个代理节点", len(candidates))
	}()

	// 2) 合并硬编码列表（硬编码优先排前面）
	merged := []string{}
	mergedSeen := map[string]bool{}
	for _, d := range hardcodedProxies {
		if !mergedSeen[d] {
			mergedSeen[d] = true
			merged = append(merged, d)
		}
	}
	for _, d := range candidates {
		if !mergedSeen[d] {
			mergedSeen[d] = true
			merged = append(merged, d)
		}
	}

	if len(merged) > proxyTestTopN {
		merged = merged[:proxyTestTopN]
	}
	return merged
}

// TestProxySpeed 下载最多 proxyTestBytes 字节测速，使用 Range 头。
// proxy == "" 表示直连。返回 (speed_bytes_per_sec, ok)。
func TestProxySpeed(ctx context.Context, githubURL string, proxy string) (float64, bool) {
	url := ProxyDownloadURL(githubURL, proxy)

	testCtx, cancel := context.WithTimeout(ctx, proxyTestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(testCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", "go-stock-updater")
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", proxyTestBytes-1))

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, false
	}

	downloaded := int64(0)
	buf := make([]byte, 64*1024)
	t0 := time.Now()
	for {
		select {
		case <-testCtx.Done():
			elapsed := time.Since(t0).Seconds()
			if downloaded > 0 && elapsed > 0 {
				return float64(downloaded) / elapsed, true
			}
			return 0, false
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			downloaded += int64(n)
		}
		if downloaded >= int64(proxyTestBytes) {
			break
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			elapsed := time.Since(t0).Seconds()
			if downloaded > 0 && elapsed > 0 {
				return float64(downloaded) / elapsed, true
			}
			return 0, false
		}
	}
	elapsed := time.Since(t0).Seconds()
	if downloaded <= 0 || elapsed <= 0 {
		return 0, false
	}
	return float64(downloaded) / elapsed, true
}

// SelectFastestProxy 并发测速直连 + 所有候选代理，返回最快的。
// bestProxy == "" 表示直连最快（或所有代理均失败）。
func SelectFastestProxy(ctx context.Context, githubURL string) (bestProxy string, bestSpeed float64) {
	candidates := FetchProxyCandidates()
	logger.SugaredLogger.Infof("开始并发测速（直连 + %d 个代理）...", len(candidates))

	type speedResult struct {
		proxy string
		speed float64
		ok    bool
	}

	resultsCh := make(chan speedResult, len(candidates)+1)
	var wg sync.WaitGroup
	sem := make(chan struct{}, proxyConcurrency)

	// 测直连
	wg.Add(1)
	go func() {
		defer wg.Done()
		speed, ok := TestProxySpeed(ctx, githubURL, "")
		resultsCh <- speedResult{"", speed, ok}
		if ok {
			logger.SugaredLogger.Infof("  (直连) 速度: %s", fmtSpeed(speed))
		} else {
			logger.SugaredLogger.Infof("  (直连) 失败")
		}
	}()

	// 测代理
	for _, proxy := range candidates {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			speed, ok := TestProxySpeed(ctx, githubURL, p)
			resultsCh <- speedResult{p, speed, ok}
			if ok {
				logger.SugaredLogger.Infof("  %s 速度: %s", p, fmtSpeed(speed))
			} else {
				logger.SugaredLogger.Infof("  %s 失败", p)
			}
		}(proxy)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	for r := range resultsCh {
		if r.ok && r.speed > bestSpeed {
			bestSpeed = r.speed
			bestProxy = r.proxy
		}
	}

	label := bestProxy
	if label == "" {
		label = "(直连)"
	}
	logger.SugaredLogger.Infof("测速完成，最快: %s 速度: %s", label, fmtSpeed(bestSpeed))
	return bestProxy, bestSpeed
}

// ProxyDownloadURL 构造代理下载 URL: https://<proxy>/<原始github_url>
// proxy 为空时返回原始 URL。
func ProxyDownloadURL(githubURL string, proxy string) string {
	if proxy == "" {
		return githubURL
	}
	return fmt.Sprintf("https://%s/%s", proxy, githubURL)
}

// IsGitHubURL 判断 URL 是否为 GitHub 地址（VIP CDN URL 返回 false，跳过代理加速）。
func IsGitHubURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.HasPrefix(lower, "https://github.com/") ||
		strings.HasPrefix(lower, "http://github.com/") ||
		strings.HasPrefix(lower, "https://objects.githubusercontent.com/") ||
		strings.HasPrefix(lower, "https://release.github.com/")
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

func fmtSpeed(bps float64) string {
	for _, unit := range []string{"B/s", "KB/s", "MB/s", "GB/s"} {
		if bps < 1024 || unit == "GB/s" {
			return fmt.Sprintf("%.2f %s", bps, unit)
		}
		bps /= 1024
	}
	return fmt.Sprintf("%.2f B/s", bps)
}
