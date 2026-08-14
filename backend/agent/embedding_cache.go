package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/philippgille/chromem-go"
	"go-stock/backend/logger"
)

// embedding 查询缓存：避免对相同 query 重复调用 embedding API。
//
// 背景：knowledge_base.go 的 SearchKnowledgeBase 每次检索都会调用 embedding 函数
// 将 query 转为向量。同一会话内用户可能反复问相似问题（如"茅台最新财报"），
// 此时 query 的 embedding 完全相同，重复调用 API 是浪费。
//
// 设计：
//   - 缓存 key = modelKey + "\x00" + text，区分不同 embedding 模型
//   - TTL = 1h（embedding 不随时间变化）
//   - 容量限制 = 1000 条，超限时清空（简单策略，避免内存泄漏）
//   - 返回 []float32 副本，避免调用方修改缓存内容
//
// 统计：通过 atomic 计数器记录命中/未命中，便于监控。

const (
	embeddingCacheTTL     = time.Hour
	embeddingCacheMaxSize = 1000
)

type embeddingCacheEntry struct {
	embedding []float32
	expireAt  time.Time
}

var (
	embeddingCache   = make(map[string]embeddingCacheEntry)
	embeddingCacheMu sync.RWMutex
	embeddingHits    int64
	embeddingMisses  int64
)

// wrapEmbedFuncWithCache 包装 chromem.EmbeddingFunc，增加查询缓存层。
//
// 参数：
//   - fn: 原始 embedding 函数（不能为 nil）
//   - modelKey: 模型标识（如 "aic3:text-embedding-3-small"），用于区分不同模型的缓存
//
// 缓存命中时返回副本（避免调用方修改缓存）；未命中时调用 fn 并缓存结果。
// fn 返回 error 时不缓存，直接返回错误。
func wrapEmbedFuncWithCache(fn chromem.EmbeddingFunc, modelKey string) chromem.EmbeddingFunc {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, text string) ([]float32, error) {
		// 空文本不缓存（chromem-go 对空文本可能有特殊处理）
		if text == "" {
			return fn(ctx, text)
		}

		cacheKey := modelKey + "\x00" + text

		// 快路径：RLock 查缓存
		embeddingCacheMu.RLock()
		if e, ok := embeddingCache[cacheKey]; ok && time.Now().Before(e.expireAt) {
			embeddingCacheMu.RUnlock()
			atomic.AddInt64(&embeddingHits, 1)
			return copyFloat32Slice(e.embedding), nil
		}
		embeddingCacheMu.RUnlock()

		// 慢路径：调用原始 embedding 函数
		emb, err := fn(ctx, text)
		if err != nil {
			return nil, err
		}

		// 写缓存
		embeddingCacheMu.Lock()
		// 容量限制：超限时清空（简单策略，避免内存无限增长）
		if len(embeddingCache) >= embeddingCacheMaxSize {
			embeddingCache = make(map[string]embeddingCacheEntry)
			logger.SugaredLogger.Infof("embedding cache reached max size %d, cleared", embeddingCacheMaxSize)
		}
		embeddingCache[cacheKey] = embeddingCacheEntry{
			embedding: copyFloat32Slice(emb),
			expireAt:  time.Now().Add(embeddingCacheTTL),
		}
		embeddingCacheMu.Unlock()

		atomic.AddInt64(&embeddingMisses, 1)
		return emb, nil
	}
}

// copyFloat32Slice 返回 []float32 的副本。
// 避免调用方修改切片底层数组导致缓存内容被污染。
func copyFloat32Slice(s []float32) []float32 {
	if s == nil {
		return nil
	}
	cp := make([]float32, len(s))
	copy(cp, s)
	return cp
}

// EmbeddingCacheStats 返回 embedding 缓存的命中/未命中统计。
func EmbeddingCacheStats() (hits, misses int64) {
	return atomic.LoadInt64(&embeddingHits), atomic.LoadInt64(&embeddingMisses)
}

// InvalidateEmbeddingCache 清空 embedding 缓存（配置变更时调用）。
func InvalidateEmbeddingCache() {
	embeddingCacheMu.Lock()
	embeddingCache = make(map[string]embeddingCacheEntry)
	embeddingCacheMu.Unlock()
}
