package data

import (
	"testing"
	"time"

	gotdx "github.com/bensema/gotdx"
)

// TestTdxTimeoutSecNotFollowCrawlTimeout 回归守护：TDX 超时必须是短超时，
// 不得沿用 CrawlTimeOut（默认 60s，曾导致网络不佳用户 AI 工具调用卡数分钟）。
func TestTdxTimeoutSecNotFollowCrawlTimeout(t *testing.T) {
	if sec := tdxTimeoutSec(); sec > 15 {
		t.Fatalf("TDX 超时 %ds 过大：TCP 行情协议应保持短超时（<=15s）", sec)
	}
}

// TestTdxReachableAddressesCache 验证拨测缓存：预置缓存后应直接命中，不触发网络拨测。
func TestTdxReachableAddressesCache(t *testing.T) {
	saved := tdxProbeCache["test-cache"]
	defer func() {
		if saved != nil {
			tdxProbeCache["test-cache"] = saved
		} else {
			delete(tdxProbeCache, "test-cache")
		}
	}()

	// 预置一个 2030 年才过期的缓存，主机列表指向不可达地址：
	// 若缓存未命中会真实拨测（3s 超时 × 并发）并走 fallback，命中则瞬时返回
	tdxProbeCache["test-cache"] = &tdxProbeResult{
		addresses: []string{"1.2.3.4:7709", "5.6.7.8:7709"},
		expireAt:  time.Now().Add(time.Hour),
	}

	start := time.Now()
	addr, pool := tdxReachableAddresses("test-cache", []gotdx.HostInfo{
		{Name: "x", IP: "192.0.2.1", Port: 7709}, // TEST-NET 不可达地址
	})
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("缓存未命中，走了真实拨测（耗时 %v）", elapsed)
	}
	if addr != "1.2.3.4:7709" || len(pool) != 1 || pool[0] != "5.6.7.8:7709" {
		t.Fatalf("缓存命中结果不符: addr=%q pool=%v", addr, pool)
	}
}

// TestTdxReachableAddressesAllDownFallback 全部主站不可达时应截断地址池（≤3），
// 让 gotdx 逐地址连接快速失败，而非遍历全部地址。
func TestTdxReachableAddressesAllDownFallback(t *testing.T) {
	kind := "test-all-down"
	delete(tdxProbeCache, kind)
	defer delete(tdxProbeCache, kind)

	// TEST-NET 地址保证不可达
	hosts := make([]gotdx.HostInfo, 0, 10)
	for i := 0; i < 10; i++ {
		hosts = append(hosts, gotdx.HostInfo{Name: "x", IP: "192.0.2.1", Port: 7709})
	}

	start := time.Now()
	addr, pool := tdxReachableAddresses(kind, hosts)
	elapsed := time.Since(start)

	if elapsed > tdxProbeTimeout+2*time.Second {
		t.Fatalf("全挂拨测耗时 %v 超过单轮拨测上限", elapsed)
	}
	total := 1 + len(pool)
	if total > tdxAllDownFallbackCount {
		t.Fatalf("全挂 fallback 地址数 %d 超过上限 %d", total, tdxAllDownFallbackCount)
	}
	if addr == "" {
		t.Fatal("全挂 fallback 应返回非空首地址")
	}
	// 缓存应记录 allDown 标记
	if entry, ok := tdxProbeCache[kind]; !ok || !entry.allDown {
		t.Fatal("全挂结果应写入缓存并标记 allDown")
	}
}

// TestTdxReachableAddressesExpiredCache 缓存过期后应重新拨测。
func TestTdxReachableAddressesExpiredCache(t *testing.T) {
	kind := "test-expired"
	delete(tdxProbeCache, kind)
	defer delete(tdxProbeCache, kind)

	// 预置已过期缓存
	tdxProbeCache[kind] = &tdxProbeResult{
		addresses: []string{"9.9.9.9:7709"},
		expireAt:  time.Now().Add(-time.Minute),
	}

	_, pool := tdxReachableAddresses(kind, []gotdx.HostInfo{
		{Name: "x", IP: "192.0.2.1", Port: 7709},
	})
	// 过期缓存被替换为真实拨测结果（不可达 → fallback，地址不是 9.9.9.9）
	if entry := tdxProbeCache[kind]; entry.addresses[0] == "9.9.9.9:7709" && time.Now().Before(entry.expireAt) {
		t.Fatal("过期缓存未被重新拨测替换")
	}
	_ = pool
}
