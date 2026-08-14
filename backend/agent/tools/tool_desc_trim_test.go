package tools

import (
	"strings"
	"testing"
)

// TestCacheTTLForToolPrecise 验证精确配置优先于组级默认。
func TestCacheTTLForToolPrecise(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantTTL  int
	}{
		// 精确配置（覆盖组级默认）
		{"F10低频数据1h", "GetStockOrgBasicInfo", 3600},
		{"每日变化10s", "GetStockLatestFinance", 10},
		{"节假日24h", "IsTradingDay", 86400},
		{"研报5min", "GetStockResearchReport", 300},
		{"不缓存-AI生成", "CreateAiRecommendStocks", 0},
		{"不缓存-实时时间", "GetCurrentTime", 0},
		{"不缓存-分时数据", "GetStockMinuteData", 0},
		{"操作类查询覆盖为60s", "GetTradingRecordList", 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheTTLForTool(tt.toolName)
			if got != tt.wantTTL {
				t.Errorf("cacheTTLForTool(%q) = %d, want %d", tt.toolName, got, tt.wantTTL)
			}
		})
	}
}

// TestCacheTTLForToolGroupDefault 验证未精确配置的工具走组级默认。
func TestCacheTTLForToolGroupDefault(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantTTL  int
	}{
		// 走组级默认（未在 cacheTTLByTool 中配置）
		{"GroupBase默认60s", "GetFollowedStocks", 60},
		{"GroupStockAnalysis默认30s", "GetStockInfo", 30},
		{"GroupMarket默认10s", "GetMarketData", 10},
		{"GroupScreening默认300s", "FilterStocks", 300},
		{"GroupMoneyFlow默认10s", "GetStockMoneyData", 10},
		{"GroupNewsResearch默认60s", "SearchNews", 60},
		{"GroupAIAnalysis默认0不缓存", "FinancialQA", 0},
		{"GroupOperations默认0不缓存", "FollowStock", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheTTLForTool(tt.toolName)
			if got != tt.wantTTL {
				t.Errorf("cacheTTLForTool(%q) = %d, want %d", tt.toolName, got, tt.wantTTL)
			}
		})
	}
}

// TestCacheTTLForToolUnknown 验证未知工具不缓存。
func TestCacheTTLForToolUnknown(t *testing.T) {
	got := cacheTTLForTool("NonExistentTool12345")
	if got != 0 {
		t.Errorf("cacheTTLForTool(unknown) = %d, want 0", got)
	}
}

// TestBuildCacheKey 验证缓存 key 构造。
func TestBuildCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		argsJSON string
		want     string
	}{
		{"空参数", "GetStockInfo", "", "GetStockInfo|"},
		{"有参数", "GetStockInfo", `{"stockCode":"600519"}`, `GetStockInfo|{"stockCode":"600519"}`},
		{"参数TrimSpace", "GetStockInfo", `  {"stockCode":"600519"}  `, `GetStockInfo|{"stockCode":"600519"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCacheKey(tt.toolName, tt.argsJSON)
			if got != tt.want {
				t.Errorf("buildCacheKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSetAndGetCachedToolResult 验证缓存写入和读取。
func TestSetAndGetCachedToolResult(t *testing.T) {
	// 清空缓存避免干扰
	resetToolResultCache()

	toolName := "GetStockOrgBasicInfo" // TTL=3600
	argsJSON := `{"stockCode":"600519"}`
	result := "测试缓存结果"

	// 初始状态应未命中
	if _, ok := getCachedToolResult(toolName, argsJSON); ok {
		t.Fatal("expected cache miss before set")
	}

	// 写入缓存
	setCachedToolResult(toolName, argsJSON, result)

	// 应命中
	got, ok := getCachedToolResult(toolName, argsJSON)
	if !ok {
		t.Fatal("expected cache hit after set")
	}
	if got != result {
		t.Errorf("cached result = %q, want %q", got, result)
	}
}

// TestSetCachedToolResultSkipInvalid 验证错误/空结果不缓存。
func TestSetCachedToolResultSkipInvalid(t *testing.T) {
	resetToolResultCache()

	toolName := "GetStockOrgBasicInfo"
	argsJSON := `{"stockCode":"600519"}`

	// 空结果不缓存
	setCachedToolResult(toolName, argsJSON, "")
	if _, ok := getCachedToolResult(toolName, argsJSON); ok {
		t.Error("empty result should not be cached")
	}

	// 错误消息不缓存
	setCachedToolResult(toolName, argsJSON, "工具调用出错：连接超时")
	if _, ok := getCachedToolResult(toolName, argsJSON); ok {
		t.Error("error result should not be cached")
	}

	// 正常结果缓存
	setCachedToolResult(toolName, argsJSON, "正常结果")
	if _, ok := getCachedToolResult(toolName, argsJSON); !ok {
		t.Error("normal result should be cached")
	}
}

// TestSetCachedToolResultSkipNoTTL 验证 TTL=0 的工具不缓存。
func TestSetCachedToolResultSkipNoTTL(t *testing.T) {
	resetToolResultCache()

	toolName := "GetCurrentTime" // TTL=0
	argsJSON := `{}`
	result := "2026-08-11 12:00:00"

	setCachedToolResult(toolName, argsJSON, result)
	if _, ok := getCachedToolResult(toolName, argsJSON); ok {
		t.Error("TTL=0 tool should not be cached")
	}
}

// TestSetCachedToolResultSkipTooLarge 验证超大结果不缓存。
func TestSetCachedToolResultSkipTooLarge(t *testing.T) {
	resetToolResultCache()

	toolName := "GetStockOrgBasicInfo" // TTL=3600
	argsJSON := `{"stockCode":"600519"}`
	// 生成超过 64KB 的结果
	largeResult := strings.Repeat("x", 65*1024)

	setCachedToolResult(toolName, argsJSON, largeResult)
	if _, ok := getCachedToolResult(toolName, argsJSON); ok {
		t.Error("result > 64KB should not be cached")
	}
}
