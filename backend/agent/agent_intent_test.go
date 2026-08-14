package agent

import (
	"testing"
)

func TestClassifyComplexityIntentRouting(t *testing.T) {
	tests := []struct {
		question string
		expected Mode
	}{
		{"今天茅台股价多少", React},
		{"查询一下平安银行的代码", React},
		{"全面分析贵州茅台的投资价值", PlanExecute},
		{"综合分析当前市场热点和投资机会", PlanExecute},
		{"帮我查一下今天大盘行情", React},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", PlanExecute},
	}
	for _, tt := range tests {
		got := classifyComplexity(tt.question)
		if got != tt.expected {
			t.Errorf("classifyComplexity(%q) = %s, want %s", tt.question, got, tt.expected)
		}
	}
}
