package data

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// ProgressCallback 周期性回调（每 500ms 或下载完成时调用）。
// downloaded: 已下载字节数; total: 总字节数;
// percentage: 0-100; currentSpeed: 瞬时速度(bytes/s); avgSpeed: 平均速度(bytes/s)。
type ProgressCallback func(downloaded, total int64, percentage, currentSpeed, avgSpeed float64)

const progressEmitInterval = 500 * time.Millisecond

// progressReader 包装 io.ReadCloser，在读取过程中追踪进度并周期性调用回调。
type progressReader struct {
	reader        io.ReadCloser
	total         int64
	downloaded    int64
	callback      ProgressCallback
	startTime     time.Time
	lastEmitTime  time.Time
	lastEmitBytes int64
}

func newProgressReader(reader io.ReadCloser, total int64, cb ProgressCallback) *progressReader {
	now := time.Now()
	return &progressReader{
		reader:       reader,
		total:        total,
		callback:     cb,
		startTime:    now,
		lastEmitTime: now,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
	}

	now := time.Now()
	shouldEmit := now.Sub(pr.lastEmitTime) >= progressEmitInterval || err != nil
	if shouldEmit && pr.callback != nil {
		elapsed := now.Sub(pr.lastEmitTime).Seconds()
		currentSpeed := float64(0)
		if elapsed > 0 {
			currentSpeed = float64(pr.downloaded-pr.lastEmitBytes) / elapsed
		}
		avgElapsed := now.Sub(pr.startTime).Seconds()
		avgSpeed := float64(0)
		if avgElapsed > 0 {
			avgSpeed = float64(pr.downloaded) / avgElapsed
		}
		percentage := float64(0)
		if pr.total > 0 {
			percentage = float64(pr.downloaded) * 100 / float64(pr.total)
			if percentage > 100 {
				percentage = 100
			}
		}
		pr.callback(pr.downloaded, pr.total, percentage, currentSpeed, avgSpeed)
		pr.lastEmitTime = now
		pr.lastEmitBytes = pr.downloaded
	}

	return n, err
}

func (pr *progressReader) Close() error {
	return pr.reader.Close()
}

// DownloadWithProgress 从 url 下载到 tmpPath，每 500ms 调用 progressCB。
// totalSize 为预期大小（来自 GitHub API asset.Size），为 0 时用 Content-Length。
// 使用 net/http 直接请求（非 resty），避免 resty 重试干扰进度追踪。
func DownloadWithProgress(ctx context.Context, url string, tmpPath string, totalSize int64, progressCB ProgressCallback) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "go-stock-updater")

	shared := GetSharedTransport()
	transport := &http.Transport{
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
		Proxy:                 shared.Proxy,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("下载失败, HTTP状态: %d", resp.StatusCode)
	}

	total := totalSize
	if total <= 0 {
		total = resp.ContentLength
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer out.Close()

	pr := newProgressReader(resp.Body, total, progressCB)
	_, err = io.Copy(out, pr)
	if err != nil {
		return fmt.Errorf("下载写入失败: %w", err)
	}

	// 发射最终进度事件确保用户看到 100%
	if progressCB != nil {
		avgElapsed := time.Since(pr.startTime).Seconds()
		avgSpeed := float64(0)
		if avgElapsed > 0 {
			avgSpeed = float64(pr.downloaded) / avgElapsed
		}
		percentage := float64(100)
		if total > 0 && pr.downloaded < total {
			percentage = float64(pr.downloaded) * 100 / float64(total)
		}
		progressCB(pr.downloaded, total, percentage, 0, avgSpeed)
	}

	return nil
}
