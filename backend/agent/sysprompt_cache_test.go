package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestFileContentCacheHitAndMiss 验证文件级 mtime 缓存的命中/未命中。
func TestFileContentCacheHitAndMiss(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")
	content := "测试缓存内容"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 清空缓存
	InvalidateFileCache(path)

	// 第一次读取：未命中
	got, _, hit := globalFileCache.readCachedFile(path)
	if hit {
		t.Error("first read should be cache miss")
	}
	if got != content {
		t.Errorf("first read content = %q, want %q", got, content)
	}

	// 第二次读取：命中
	got, _, hit = globalFileCache.readCachedFile(path)
	if !hit {
		t.Error("second read should be cache hit")
	}
	if got != content {
		t.Errorf("second read content = %q, want %q", got, content)
	}
}

// TestFileContentCacheMTimeInvalidation 验证文件修改后缓存失效。
func TestFileContentCacheMTimeInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test2.md")
	oldContent := "旧内容"
	if err := os.WriteFile(path, []byte(oldContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	InvalidateFileCache(path)

	// 第一次读取：缓存
	got, _, _ := globalFileCache.readCachedFile(path)
	if got != oldContent {
		t.Fatalf("first read = %q, want %q", got, oldContent)
	}

	// 修改文件（确保 mtime 变化）
	time.Sleep(10 * time.Millisecond)
	newContent := "新内容"
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 第二次读取：应未命中（mtime 变了），返回新内容
	got, _, hit := globalFileCache.readCachedFile(path)
	if hit {
		t.Error("read after mtime change should be cache miss")
	}
	if got != newContent {
		t.Errorf("content after change = %q, want %q", got, newContent)
	}
}

// TestFileContentCacheNonExistent 验证文件不存在时返回空。
func TestFileContentCacheNonExistent(t *testing.T) {
	got, _, hit := globalFileCache.readCachedFile("/nonexistent/path/file.md")
	if hit {
		t.Error("non-existent file should not be cache hit")
	}
	if got != "" {
		t.Errorf("non-existent file content = %q, want empty", got)
	}
}

// TestFileContentCacheEmptyPath 验证空路径返回空。
func TestFileContentCacheEmptyPath(t *testing.T) {
	got, _, hit := globalFileCache.readCachedFile("")
	if hit {
		t.Error("empty path should not be cache hit")
	}
	if got != "" {
		t.Errorf("empty path content = %q, want empty", got)
	}
}

// TestInvalidateFileCache 验证手动失效缓存。
func TestInvalidateFileCache(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test3.md")
	content := "手动失效测试"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 第一次读取：缓存
	globalFileCache.readCachedFile(path)

	// 手动失效
	InvalidateFileCache(path)

	// 第二次读取：应未命中
	_, _, hit := globalFileCache.readCachedFile(path)
	if hit {
		t.Error("should be cache miss after InvalidateFileCache")
	}
}

// TestFileCacheStats 验证缓存统计计数。
func TestFileCacheStats(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stats_test.md")
	if err := os.WriteFile(path, []byte("stats"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	InvalidateFileCache(path)

	// 记录初始统计
	initHits, initMisses := FileCacheStats()

	// 第一次读取：miss
	globalFileCache.readCachedFile(path)
	misses1 := atomic.LoadInt64(&globalFileCache.misses)
	if misses1 != initMisses+1 {
		t.Errorf("after first read: misses = %d, want %d", misses1, initMisses+1)
	}

	// 第二次读取：hit
	globalFileCache.readCachedFile(path)
	hits2 := atomic.LoadInt64(&globalFileCache.hits)
	if hits2 != initHits+1 {
		t.Errorf("after second read: hits = %d, want %d", hits2, initHits+1)
	}
}

// TestEmbeddingCacheHitAndMiss 验证 embedding 缓存的命中/未命中。
func TestEmbeddingCacheHitAndMiss(t *testing.T) {
	InvalidateEmbeddingCache()

	callCount := int64(0)
	mockEmbedFunc := func(ctx context.Context, text string) ([]float32, error) {
		atomic.AddInt64(&callCount, 1)
		return []float32{0.1, 0.2, 0.3}, nil
	}

	wrapped := wrapEmbedFuncWithCache(mockEmbedFunc, "test-model")

	// 第一次调用：未命中，调用原始函数
	result1, err := wrapped(context.Background(), "茅台财报")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if len(result1) != 3 {
		t.Errorf("first call result len = %d, want 3", len(result1))
	}
	if atomic.LoadInt64(&callCount) != 1 {
		t.Errorf("after first call: callCount = %d, want 1", callCount)
	}

	// 第二次调用：命中，不调用原始函数
	result2, err := wrapped(context.Background(), "茅台财报")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if len(result2) != 3 {
		t.Errorf("second call result len = %d, want 3", len(result2))
	}
	if atomic.LoadInt64(&callCount) != 1 {
		t.Errorf("after second call: callCount = %d, want 1 (should be cached)", callCount)
	}

	// 验证返回的是副本（修改 result2 不影响缓存）
	result2[0] = 999.0
	result3, _ := wrapped(context.Background(), "茅台财报")
	if result3[0] == 999.0 {
		t.Error("cache returned same slice reference, should return copy")
	}
}

// TestEmbeddingCacheDifferentModels 验证不同模型的缓存隔离。
func TestEmbeddingCacheDifferentModels(t *testing.T) {
	InvalidateEmbeddingCache()

	callCount := int64(0)
	mockEmbedFunc := func(ctx context.Context, text string) ([]float32, error) {
		atomic.AddInt64(&callCount, 1)
		return []float32{float32(callCount)}, nil // 每次返回不同值
	}

	wrapped1 := wrapEmbedFuncWithCache(mockEmbedFunc, "model-A")
	wrapped2 := wrapEmbedFuncWithCache(mockEmbedFunc, "model-B")

	// model-A 调用
	r1, _ := wrapped1(context.Background(), "same text")
	// model-B 调用相同 text（应未命中，因为模型不同）
	r2, _ := wrapped2(context.Background(), "same text")

	if r1[0] == r2[0] {
		t.Error("different models should have separate cache entries")
	}
	if atomic.LoadInt64(&callCount) != 2 {
		t.Errorf("callCount = %d, want 2 (different models = 2 calls)", callCount)
	}
}

// TestEmbeddingCacheEmptyText 验证空文本不缓存。
func TestEmbeddingCacheEmptyText(t *testing.T) {
	InvalidateEmbeddingCache()

	callCount := int64(0)
	mockEmbedFunc := func(ctx context.Context, text string) ([]float32, error) {
		atomic.AddInt64(&callCount, 1)
		return []float32{0.0}, nil
	}

	wrapped := wrapEmbedFuncWithCache(mockEmbedFunc, "test-model")

	// 空文本不缓存，每次都调用原始函数
	wrapped(context.Background(), "")
	wrapped(context.Background(), "")

	if atomic.LoadInt64(&callCount) != 2 {
		t.Errorf("empty text callCount = %d, want 2 (should not cache)", callCount)
	}
}

// TestEmbeddingCacheNilFunc 验证 nil 函数返回 nil。
func TestEmbeddingCacheNilFunc(t *testing.T) {
	wrapped := wrapEmbedFuncWithCache(nil, "test-model")
	if wrapped != nil {
		t.Error("wrapEmbedFuncWithCache(nil, ...) should return nil")
	}
}

// TestEmbeddingCacheStats 验证缓存统计。
func TestEmbeddingCacheStats(t *testing.T) {
	InvalidateEmbeddingCache()

	mockEmbedFunc := func(ctx context.Context, text string) ([]float32, error) {
		return []float32{0.1}, nil
	}
	wrapped := wrapEmbedFuncWithCache(mockEmbedFunc, "stats-model")

	initHits, initMisses := EmbeddingCacheStats()

	// miss
	wrapped(context.Background(), "query1")
	// hit
	wrapped(context.Background(), "query1")
	// miss
	wrapped(context.Background(), "query2")

	hits, misses := EmbeddingCacheStats()
	if hits != initHits+1 {
		t.Errorf("hits = %d, want %d", hits, initHits+1)
	}
	if misses != initMisses+2 {
		t.Errorf("misses = %d, want %d", misses, initMisses+2)
	}
}

// TestCopyFloat32Slice 验证切片深拷贝。
func TestCopyFloat32Slice(t *testing.T) {
	original := []float32{1.0, 2.0, 3.0}
	cp := copyFloat32Slice(original)

	// 修改副本不影响原始
	cp[0] = 999.0
	if original[0] == 999.0 {
		t.Error("copy should not affect original")
	}

	// nil 处理
	if cp2 := copyFloat32Slice(nil); cp2 != nil {
		t.Error("copyFloat32Slice(nil) should return nil")
	}

	// 空切片
	if cp3 := copyFloat32Slice([]float32{}); len(cp3) != 0 {
		t.Error("copyFloat32Slice(empty) should return empty slice")
	}
}

// TestStaticRulesConstNonEmpty 验证静态规则常量非空。
func TestStaticRulesConstNonEmpty(t *testing.T) {
	if staticRulesHead == "" {
		t.Error("staticRulesHead should not be empty")
	}
	if staticRulesCompliance == "" {
		t.Error("staticRulesCompliance should not be empty")
	}
	if staticRulesTail == "" {
		t.Error("staticRulesTail should not be empty")
	}
	if staticRulesParallel == "" {
		t.Error("staticRulesParallel should not be empty")
	}
	if staticRulesRetrieval == "" {
		t.Error("staticRulesRetrieval should not be empty")
	}
	if staticRulesPlanExecute == "" {
		t.Error("staticRulesPlanExecute should not be empty")
	}
	if staticRulesThinking == "" {
		t.Error("staticRulesThinking should not be empty")
	}
}

// TestStaticRulesContainKeywords 验证静态规则包含关键标识。
func TestStaticRulesContainKeywords(t *testing.T) {
	if !strings.Contains(staticRulesHead, "【强制规则】") {
		t.Error("staticRulesHead should contain 【强制规则】")
	}
	if !strings.Contains(staticRulesCompliance, "【合规边界】") {
		t.Error("staticRulesCompliance should contain 【合规边界】")
	}
	if !strings.Contains(staticRulesTail, "【错误恢复策略】") {
		t.Error("staticRulesTail should contain 【错误恢复策略】")
	}
	if !strings.Contains(staticRulesParallel, "【并行工具调用】") {
		t.Error("staticRulesParallel should contain 【并行工具调用】")
	}
	if !strings.Contains(staticRulesRetrieval, "【检索结果引用规范】") {
		t.Error("staticRulesRetrieval should contain 【检索结果引用规范】")
	}
	if !strings.Contains(staticRulesPlanExecute, "【任务规划要求】") {
		t.Error("staticRulesPlanExecute should contain 【任务规划要求】")
	}
	if !strings.Contains(staticRulesThinking, "【思考要求】") {
		t.Error("staticRulesThinking should contain 【思考要求】")
	}
}

// TestGetStaticXPrompt 验证静态规则缓存函数返回正确内容。
func TestGetStaticXPrompt(t *testing.T) {
	head, headTokens := getStaticHeadPrompt()
	if head == "" {
		t.Error("getStaticHeadPrompt should return non-empty string")
	}
	if headTokens <= 0 {
		t.Error("getStaticHeadPrompt should return positive tokens")
	}
	// 验证内容包含两段
	if !strings.Contains(head, "【强制规则】") {
		t.Error("static head should contain 【强制规则】")
	}
	if !strings.Contains(head, "【合规边界】") {
		t.Error("static head should contain 【合规边界】")
	}

	tail, tailTokens := getStaticTailPrompt()
	if tail == "" {
		t.Error("getStaticTailPrompt should return non-empty string")
	}
	if tailTokens <= 0 {
		t.Error("getStaticTailPrompt should return positive tokens")
	}

	pe, peTokens := getStaticPlanExecutePrompt()
	if pe == "" || peTokens <= 0 {
		t.Error("getStaticPlanExecutePrompt should return non-empty string and positive tokens")
	}

	th, thTokens := getStaticThinkingPrompt()
	if th == "" || thTokens <= 0 {
		t.Error("getStaticThinkingPrompt should return non-empty string and positive tokens")
	}
}
