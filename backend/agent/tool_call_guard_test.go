package agent

// tool_call_guard_test.go — 工具调用守卫（瞬态重试判定、去重安全名单、
// 去重键、单工具上限、同轮缓存）的纯函数/状态测试，不依赖网络与数据库。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestIsTransientToolError(t *testing.T) {
	transient := []string{
		"context deadline exceeded",
		"Get \"https://x\": dial tcp: i/o timeout",
		"read: connection reset by peer",
		"connect: connection refused",
		"unexpected EOF",
		"TLS handshake timeout",
		"HTTP 503 Service Unavailable",
		"http 429 too many requests",
		"502 Bad Gateway",
	}
	for _, msg := range transient {
		if !isTransientToolError(errors.New(msg)) {
			t.Errorf("应识别为瞬态错误: %q", msg)
		}
	}

	nonTransient := []string{
		"参数错误: stockCode 不能为空",
		"no such host",           // DNS 永久失败，重试无意义
		"invalid character 'x'",  // 数据格式错误
		"未找到符合条件的数据",      // 业务空结果
		"permission denied",      // 权限问题非瞬态
	}
	for _, msg := range nonTransient {
		if isTransientToolError(errors.New(msg)) {
			t.Errorf("不应识别为瞬态错误: %q", msg)
		}
	}
	if isTransientToolError(nil) {
		t.Error("nil 错误不应识别为瞬态")
	}
}

func TestIsDedupSafeTool(t *testing.T) {
	// 只读查询工具：允许同轮去重（取自 tools.go 真实工具名）
	safe := []string{
		"GetStockKLine", "GetEastMoneyKLine", "SearchStockByIndicators",
		"GetStockMoneyData", "QueryStockNews", "FilterStocks",
		"HotStockTable", "HotStrategyTable", "GetHotTopics", "TopTopics",
		"GlobalStockIndexesReadable", "GetFollowedStocks", // Get 前缀优先判定，不受 follow 影响
		"AiRecommendStocks", // 未命中任何名单 → 保守不去重，见下方 unsafe 断言修正
	}
	// AiRecommendStocks 不以只读前缀开头，按保守策略不去重
	for _, name := range safe[:len(safe)-1] {
		if !isDedupSafeTool(name) {
			t.Errorf("只读工具应允许去重: %s", name)
		}
	}

	// 有副作用工具：绝不参与去重
	unsafe := []string{
		"CreateAiRecommendStocks", "BatchCreateAiRecommendStocks",
		"SendToDingDing", "SendToFeishu", "AddDailyOperationPlan",
		"UpdateDailyOperationPlan", "DeleteStockGroup", "SavePromptTemplate",
		"SetTradingPrice", "FollowStock", "CleanupStockCodes",
		"MarkdownToImage", "InteractiveAnswer",
		"write_file", "edit_file", "execute", // DeepAgents 内置
		"AiRecommendStocks",                  // 未命中名单，保守不去重
		"some_mcp_tool",                      // 未知 MCP 工具保守不去重
		"",
	}
	for _, name := range unsafe {
		if isDedupSafeTool(name) {
			t.Errorf("有副作用/未知工具不应去重: %q", name)
		}
	}
}

func TestToolDedupKey(t *testing.T) {
	k1 := toolDedupKey("GetStockKLine", `{"stockCode":"sh600519"}`)
	k2 := toolDedupKey("GetStockKLine", `{"stockCode":"sh600519"}`)
	k3 := toolDedupKey("GetStockKLine", `{"stockCode":"sz000001"}`)
	k4 := toolDedupKey("GetEastMoneyKLine", `{"stockCode":"sh600519"}`)
	if k1 != k2 {
		t.Error("相同工具+参数应生成相同键")
	}
	if k1 == k3 {
		t.Error("参数不同应生成不同键")
	}
	if k1 == k4 {
		t.Error("工具不同应生成不同键")
	}
	// 大小写与首尾空白归一
	k5 := toolDedupKey("getstockkline", "  "+`{"stockCode":"sh600519"}`+"\n")
	if k1 != k5 {
		t.Error("键应归一化大小写与空白")
	}
}

func TestReserveToolNamedPerToolLimit(t *testing.T) {
	_, run := NewAgentRunContext(context.Background(), "test", "s1", AgentRunBudget{MaxToolCalls: 100})
	defer run.Finish(nil)
	limit := maxPerToolCalls(run.Budget) // 100/4 = 25
	for i := 0; i < limit; i++ {
		if err := run.ReserveToolNamed("GetStockKLine"); err != nil {
			t.Fatalf("第 %d 次调用不应被单工具上限拦截: %v", i+1, err)
		}
	}
	err := run.ReserveToolNamed("GetStockKLine")
	if err == nil || !strings.Contains(err.Error(), "单工具上限") {
		t.Fatalf("第 %d 次调用应被单工具上限拦截: %v", limit+1, err)
	}
	// 其他工具不受单工具上限影响
	if err := run.ReserveToolNamed("GetStockMoneyData"); err != nil {
		t.Fatalf("其他工具不应被 GetStockKLine 的上限拦截: %v", err)
	}
}

func TestMaxPerToolCallsFloor(t *testing.T) {
	if n := maxPerToolCalls(AgentRunBudget{MaxToolCalls: 8}); n != 15 {
		t.Errorf("小预算应保底 15 次, got %d", n)
	}
	if n := maxPerToolCalls(AgentRunBudget{MaxToolCalls: 200}); n != 50 {
		t.Errorf("预算 200 单工具上限应为 50, got %d", n)
	}
}

func TestToolCacheFIFO(t *testing.T) {
	_, run := NewAgentRunContext(context.Background(), "test", "s1", AgentRunBudget{MaxToolCalls: 200})
	defer run.Finish(nil)

	// 存取
	run.StoreToolCache("k1", "result-1")
	if v, ok := run.LookupToolCache("k1"); !ok || v != "result-1" {
		t.Fatalf("缓存存取失败: ok=%v v=%q", ok, v)
	}
	if _, ok := run.LookupToolCache("missing"); ok {
		t.Fatal("未缓存的键不应命中")
	}

	// FIFO 淘汰：写满上限后最旧条目被移除
	for i := 0; i < toolCacheMaxEntries+10; i++ {
		run.StoreToolCache(fmt.Sprintf("flood-%d", i), "v")
	}
	if _, ok := run.LookupToolCache("flood-0"); ok {
		t.Error("超出上限后最旧条目应被淘汰")
	}
	if _, ok := run.LookupToolCache(fmt.Sprintf("flood-%d", toolCacheMaxEntries+9)); !ok {
		t.Error("最新条目应保留")
	}

	// 同键覆盖不增加条数、不影响淘汰顺序之外的行为
	run.StoreToolCache("k1", "result-1-new")
	if v, _ := run.LookupToolCache("k1"); v != "result-1-new" {
		t.Errorf("同键覆盖应更新值, got %q", v)
	}
}

// TestTrimToolResultLayeredThreshold 验证分层截断阈值：
// 中度超限（≤2×预算）走规则压缩不调 LLM；极度超限（>2×预算）才调 LLM 摘要。
func TestTrimToolResultLayeredThreshold(t *testing.T) {
	called := 0
	m := &fakeSummaryModel{resp: "LLM摘要：价格 12.34 元"}
	ctx := WithSummaryModel(context.Background(), &countingSummaryModel{inner: m, called: &called})

	// 中度超限：正文略超 4000 token 预算（约 7000 汉字 ≈ 5400 token）
	moderate := "数据：" + repeatRune("数据说明", 1700)
	out := trimToolResult(ctx, moderate, 4000)
	if called != 0 {
		t.Errorf("中度超限不应调用 LLM 摘要, called=%d", called)
	}
	if out == "" {
		t.Error("中度超限应返回规则压缩结果")
	}

	// 极度超限：longBody ≈ 9200+ token > 2×4000 预算 → 调 LLM 摘要
	out = trimToolResult(ctx, longBody, 4000)
	if called != 1 {
		t.Errorf("极度超限应调用 LLM 摘要一次, called=%d", called)
	}
	if !strings.Contains(out, "LLM摘要") {
		t.Errorf("极度超限应返回 LLM 摘要, got %.100q", out)
	}
}

// countingSummaryModel 包装 fakeSummaryModel 统计调用次数。
type countingSummaryModel struct {
	inner *fakeSummaryModel
	called *int
}

func (c *countingSummaryModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	*c.called++
	return c.inner.Generate(ctx, msgs, opts...)
}

func (c *countingSummaryModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.inner.Stream(ctx, msgs, opts...)
}

func (c *countingSummaryModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return c, nil
}
