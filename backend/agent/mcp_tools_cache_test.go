package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/cloudwego/eino/components/tool"
)

// withFakeLoader 替换包级加载函数并在测试结束后恢复。
func withFakeLoader(t *testing.T, fn func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer)) {
	t.Helper()
	orig := loadMCPToolsForServerFn
	loadMCPToolsForServerFn = fn
	t.Cleanup(func() { loadMCPToolsForServerFn = orig })
}

func resetMCPToolsCacheForTest() {
	mcpToolsCacheMu.Lock()
	mcpToolsCache = make(map[uint]*mcpToolsCacheEntry)
	mcpToolsCacheMu.Unlock()
}

func TestMCPToolsCacheHitWithinTTL(t *testing.T) {
	resetMCPToolsCacheForTest()

	var calls int32
	withFakeLoader(t, func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		calls++
		return make([]tool.BaseTool, 3), nil
	})

	server := &models.MCPServer{ID: 1, URL: "http://x", Name: "s1", UpdatedAt: time.Now()}

	// 第一次：未命中，加载
	tools1 := getMCPToolsForServer(context.Background(), server)
	if len(tools1) != 3 {
		t.Fatalf("首次加载应返回 3 个工具, got %d", len(tools1))
	}
	if calls != 1 {
		t.Fatalf("首次应调用加载器 1 次, got %d", calls)
	}

	// 第二/三次：TTL 内指纹一致，应命中缓存不再加载
	for i := 0; i < 2; i++ {
		toolsN := getMCPToolsForServer(context.Background(), server)
		if len(toolsN) != 3 {
			t.Fatalf("缓存命中应返回相同工具数 3, got %d", len(toolsN))
		}
	}
	if calls != 1 {
		t.Fatalf("TTL 内重复请求不应触发加载, 期望 1 次, got %d", calls)
	}
}

func TestMCPToolsCacheInvalidatedByFingerprint(t *testing.T) {
	resetMCPToolsCacheForTest()

	var calls int32
	withFakeLoader(t, func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		calls++
		return make([]tool.BaseTool, 1), nil
	})

	server := &models.MCPServer{ID: 2, URL: "http://y", Name: "s2", UpdatedAt: time.Now()}
	getMCPToolsForServer(context.Background(), server) // 首次加载

	// 模拟配置变更：gorm 写操作会刷新 UpdatedAt → 指纹变化 → 失效
	server.UpdatedAt = server.UpdatedAt.Add(time.Second)
	server.Headers = "{\"Authorization\":\"Bearer new\"}"
	getMCPToolsForServer(context.Background(), server)

	if calls != 2 {
		t.Fatalf("指纹变化应触发重新加载, 期望 2 次, got %d", calls)
	}
}

func TestMCPToolsCacheInvalidatedByTTL(t *testing.T) {
	resetMCPToolsCacheForTest()

	var calls int32
	withFakeLoader(t, func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		calls++
		return make([]tool.BaseTool, 1), nil
	})

	server := &models.MCPServer{ID: 3, URL: "http://z", Name: "s3", UpdatedAt: time.Now()}
	getMCPToolsForServer(context.Background(), server) // 首次加载

	// 手动把 fetchedAt 回拨超过 TTL，模拟过期
	mcpToolsCacheMu.Lock()
	if e, ok := mcpToolsCache[3]; ok {
		e.fetchedAt = time.Now().Add(-mcpToolsCacheTTL - time.Minute)
	} else {
		t.Fatalf("缓存条目应存在")
	}
	mcpToolsCacheMu.Unlock()

	getMCPToolsForServer(context.Background(), server)
	if calls != 2 {
		t.Fatalf("TTL 过期应触发重新加载, 期望 2 次, got %d", calls)
	}
}

func TestInvalidateMCPToolsCache(t *testing.T) {
	resetMCPToolsCacheForTest()

	var calls int32
	withFakeLoader(t, func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		calls++
		return make([]tool.BaseTool, 1), nil
	})

	server := &models.MCPServer{ID: 4, URL: "http://w", Name: "s4", UpdatedAt: time.Now()}
	getMCPToolsForServer(context.Background(), server)

	InvalidateMCPToolsCache(4) // 指定 ID 失效
	getMCPToolsForServer(context.Background(), server)
	if calls != 2 {
		t.Fatalf("主动失效后应重新加载, 期望 2 次, got %d", calls)
	}

	InvalidateMCPToolsCache(0) // 全部清空
	mcpToolsCacheMu.RLock()
	_, ok := mcpToolsCache[4]
	mcpToolsCacheMu.RUnlock()
	if ok {
		t.Fatalf("全量失效后缓存应为空")
	}
}

func TestSweepStaleMCPToolsCache(t *testing.T) {
	resetMCPToolsCacheForTest()

	// 条目 5：不在活跃列表且闲置超阈值 → 应被清理
	mcpToolsCacheMu.Lock()
	mcpToolsCache[5] = &mcpToolsCacheEntry{
		tools:      nil,
		lastUsedAt: atomic.Int64{},
	}
	mcpToolsCache[5].lastUsedAt.Store(time.Now().Add(-mcpToolsStaleCloseDelay - time.Minute).UnixNano())
	// 条目 6：在活跃列表 → 保留
	mcpToolsCache[6] = &mcpToolsCacheEntry{}
	mcpToolsCache[6].lastUsedAt.Store(time.Now().Add(-mcpToolsStaleCloseDelay - time.Minute).UnixNano())
	mcpToolsCacheMu.Unlock()

	sweepStaleMCPToolsCache(map[uint]bool{6: true})

	mcpToolsCacheMu.RLock()
	_, has5 := mcpToolsCache[5]
	_, has6 := mcpToolsCache[6]
	mcpToolsCacheMu.RUnlock()

	if has5 {
		t.Fatalf("闲置的非活跃条目应被清理")
	}
	if !has6 {
		t.Fatalf("活跃条目不应被清理")
	}
}

func TestMCPToolsCacheConcurrentSingleFlight(t *testing.T) {
	resetMCPToolsCacheForTest()

	var calls int32
	withFakeLoader(t, func(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		calls++ // 写锁内串行执行，无竞争
		time.Sleep(50 * time.Millisecond)
		return make([]tool.BaseTool, 2), nil
	})

	server := &models.MCPServer{ID: 7, URL: "http://c", Name: "s7", UpdatedAt: time.Now()}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tools := getMCPToolsForServer(context.Background(), server)
			if len(tools) != 2 {
				t.Errorf("并发请求应返回 2 个工具, got %d", len(tools))
			}
		}()
	}
	wg.Wait()

	if calls != 1 {
		t.Fatalf("并发未命中应只加载一次(singleflight), 期望 1 次, got %d", calls)
	}
}
