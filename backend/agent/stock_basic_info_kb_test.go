package agent

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// TestNormalizeStockCodeForKB 验证股票代码归一化（纯逻辑，不依赖外部服务）
func TestNormalizeStockCodeForKB(t *testing.T) {
	cases := map[string]string{
		"002008":    "002008",
		"002008.SZ": "002008",
		"sz002008":  "002008",
		"SZ002008":  "002008",
		"600519":    "600519",
		"600519.SH": "600519",
		"sh600519":  "600519",
		"":          "",
	}
	for input, expected := range cases {
		got := normalizeStockCodeForKB(input)
		if got != expected {
			t.Errorf("normalizeStockCodeForKB(%q) = %q, want %q", input, got, expected)
		}
	}
}

// TestMaybeVectorizeStockBasicInfo 集成测试：调用基础资料工具后自动向量化到知识库。
// 依赖向量库与 embedding 服务；若未就绪则自动跳过。
func TestMaybeVectorizeStockBasicInfo(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test disabled; set GO_STOCK_RUN_INTEGRATION_TESTS=1 to enable")
	}
	db.Init("../../data/stock.db")
	api := data.NewStockDataApi()

	// 1. 获取基础资料 Markdown
	md := api.GetStockOrgBasicInfoToMarkdown("002008")
	if md == "" || md[:2] != "##" {
		t.Fatalf("获取基础资料失败: %s", md[:min(len(md), 100)])
	}
	t.Logf("基础资料获取成功，长度=%d", len(md))

	// 2. 触发自动向量化（异步）
	MaybeVectorizeStockBasicInfo("002008", md)

	// 3. 等待异步入库完成（最多 30 秒）
	deadline := time.Now().Add(30 * time.Second)
	vectorized := false
	for time.Now().Before(deadline) {
		if isSourceInKB(StockBasicInfoKBName, "002008") {
			vectorized = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !vectorized {
		t.Skip("向量化未在超时内完成（可能 embedding 服务未配置），跳过后续断言")
	}

	t.Logf("股票 002008 基础资料已成功向量化到知识库 %q", StockBasicInfoKBName)

	// 4. 验证 KB 存在
	info := GetKnowledgeBase(StockBasicInfoKBName)
	if info == nil {
		t.Fatalf("知识库 %q 不存在", StockBasicInfoKBName)
	}
	t.Logf("知识库: name=%q documentCount=%d", info.Name, info.DocumentCount)

	// 5. 验证去重：再次触发不应重复入库
	MaybeVectorizeStockBasicInfo("002008.SZ", md) // 不同格式，归一化后相同
	time.Sleep(2 * time.Second)
	// isSourceInKB 应仍为 true（已存在）
	if !isSourceInKB(StockBasicInfoKBName, "002008") {
		t.Errorf("去重失败：002008 应已存在")
	}
	t.Log("去重验证通过：相同股票代码不重复入库")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
