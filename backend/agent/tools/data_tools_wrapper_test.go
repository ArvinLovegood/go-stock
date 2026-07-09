package tools

import (
	"context"
	"encoding/json"
	"testing"

	"go-stock/backend/models"
)

func TestGetAllDataTools(t *testing.T) {
	tools := GetAllDataTools()
	t.Logf("Total tools count: %d", len(tools))

	toolNames := make(map[string]int)
	for i, tool := range tools {
		info, err := tool.Info(nil)
		if err != nil {
			t.Errorf("Tool %d: failed to get info: %v", i, err)
			continue
		}
		t.Logf("Tool %d: %s - %s", i+1, info.Name, info.Desc)

		if count, exists := toolNames[info.Name]; exists {
			t.Errorf("Duplicate tool name found: %s (count: %d)", info.Name, count+1)
		}
		toolNames[info.Name]++
	}

	t.Log("\n=== Tool List ===")
	for name, count := range toolNames {
		if count > 1 {
			t.Errorf("Duplicate tool: %s (count: %d)", name, count)
		}
	}
}

func TestWithAgentMeta_AgentMetaFromCtx(t *testing.T) {
	// 空 context 取不到
	if _, ok := AgentMetaFromCtx(context.Background()); ok {
		t.Fatalf("AgentMetaFromCtx should return ok=false for empty context")
	}

	// 注入后能完整取出
	meta := AgentMeta{
		ModelName:    "glm-5.2",
		SystemPrompt: "你是股票分析大师",
		UserPrompt:   "分析 600519",
	}
	ctx := WithAgentMeta(context.Background(), meta)
	got, ok := AgentMetaFromCtx(ctx)
	if !ok {
		t.Fatalf("AgentMetaFromCtx should return ok=true after WithAgentMeta")
	}
	if got != meta {
		t.Fatalf("AgentMeta round-trip mismatch: got %+v, want %+v", got, meta)
	}
}

func TestInjectRecommendMeta_Single(t *testing.T) {
	// AI 自填 modelName，应被实际值覆盖；系统/用户提示词应被填充
	args := `{"modelName":"ai-fake-name","stockCode":"600519.SH","stockName":"贵州茅台","rating":"买入"}`
	meta := AgentMeta{
		ModelName:    "glm-5.2",
		SystemPrompt: "你是顶级股票投资大师",
		UserPrompt:   "请分析贵州茅台",
	}

	out := injectRecommendMeta("CreateAiRecommendStocks", args, meta)
	if out == "" {
		t.Fatalf("injectRecommendMeta returned empty for valid single args")
	}

	var rec models.AiRecommendStocks
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("unmarshal injected result failed: %v", err)
	}
	if rec.ModelName != meta.ModelName {
		t.Errorf("ModelName not overridden: got %q, want %q", rec.ModelName, meta.ModelName)
	}
	if rec.SystemPrompt != meta.SystemPrompt {
		t.Errorf("SystemPrompt not filled: got %q, want %q", rec.SystemPrompt, meta.SystemPrompt)
	}
	if rec.UserPrompt != meta.UserPrompt {
		t.Errorf("UserPrompt not filled: got %q, want %q", rec.UserPrompt, meta.UserPrompt)
	}
	// 原有字段应保留
	if rec.StockCode != "600519.SH" {
		t.Errorf("StockCode should be preserved: got %q", rec.StockCode)
	}
	if rec.Rating != "买入" {
		t.Errorf("Rating should be preserved: got %q", rec.Rating)
	}
}

func TestInjectRecommendMeta_Batch(t *testing.T) {
	args := `{"stocks":[{"modelName":"fake-1","stockCode":"600519.SH"},{"modelName":"fake-2","stockCode":"000001.SZ"}]}`
	meta := AgentMeta{
		ModelName:    "deepseek-chat",
		SystemPrompt: "系统提示",
		UserPrompt:   "用户提问",
	}

	out := injectRecommendMeta("BatchCreateAiRecommendStocks", args, meta)
	if out == "" {
		t.Fatalf("injectRecommendMeta returned empty for valid batch args")
	}

	// 校验外层仍是 {"stocks":[...]} 结构
	if !isValidJSON(out) {
		t.Fatalf("injected output is not valid JSON: %s", out)
	}

	var wrapper struct {
		Stocks []*models.AiRecommendStocks `json:"stocks"`
	}
	if err := json.Unmarshal([]byte(out), &wrapper); err != nil {
		t.Fatalf("unmarshal wrapper failed: %v", err)
	}
	if len(wrapper.Stocks) != 2 {
		t.Fatalf("expected 2 stocks, got %d", len(wrapper.Stocks))
	}
	for i, rec := range wrapper.Stocks {
		if rec.ModelName != meta.ModelName {
			t.Errorf("stock[%d] ModelName not overridden: got %q, want %q", i, rec.ModelName, meta.ModelName)
		}
		if rec.SystemPrompt != meta.SystemPrompt {
			t.Errorf("stock[%d] SystemPrompt not filled: got %q, want %q", i, rec.SystemPrompt, meta.SystemPrompt)
		}
		if rec.UserPrompt != meta.UserPrompt {
			t.Errorf("stock[%d] UserPrompt not filled: got %q, want %q", i, rec.UserPrompt, meta.UserPrompt)
		}
	}
	// 原有字段保留
	if wrapper.Stocks[0].StockCode != "600519.SH" {
		t.Errorf("stock[0] StockCode should be preserved: got %q", wrapper.Stocks[0].StockCode)
	}
}

func TestInjectRecommendMeta_InvalidJSON(t *testing.T) {
	// 非法 JSON 应返回空字符串（调用方保留原 args）
	out := injectRecommendMeta("CreateAiRecommendStocks", "{not valid json", AgentMeta{ModelName: "x"})
	if out != "" {
		t.Errorf("injectRecommendMeta should return empty for invalid JSON, got %q", out)
	}
}

func isValidJSON(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}
