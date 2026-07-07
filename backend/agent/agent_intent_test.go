package agent

import (
	"strings"
	"testing"
)

func TestSanitizeAssistantHistoryForContext(t *testing.T) {
	raw := "贵州茅台当前股价 1850.50 元，涨幅 +2.35%，代码 600519"
	out := sanitizeAssistantHistoryForContext(raw)
	if strings.Contains(out, "1850.50") || strings.Contains(out, "+2.35%") {
		t.Fatalf("expected numeric redaction: %q", out)
	}
	if !strings.Contains(out, "600519") {
		t.Fatalf("expected stock code preserved: %q", out)
	}
	if !strings.Contains(out, "历史回复摘要") {
		t.Fatalf("expected history warning prefix: %q", out)
	}
	// 占位符应改短为 [旧值]，不再使用冗长的旧占位符
	if strings.Contains(out, "历史数值已省略") {
		t.Fatalf("should not use verbose placeholder: %q", out)
	}
	if !strings.Contains(out, "[旧值]") {
		t.Fatalf("expected [旧值] placeholder: %q", out)
	}
}

func TestClassifyComplexityIntentRouting(t *testing.T) {
	tests := []struct {
		question string
		expected AgentMode
	}{
		{"今天茅台股价多少", AgentModeReact},
		{"查询一下平安银行的代码", AgentModeReact},
		{"全面分析贵州茅台的投资价值", AgentModePlanExecute},
		{"综合分析当前市场热点和投资机会", AgentModePlanExecute},
		{"帮我查一下今天大盘行情", AgentModeReact},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", AgentModePlanExecute},
	}
	for _, tt := range tests {
		got := classifyComplexity(tt.question)
		if got != tt.expected {
			t.Errorf("classifyComplexity(%q) = %s, want %s", tt.question, got, tt.expected)
		}
	}
}
