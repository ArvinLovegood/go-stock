package agent

import (
	"strings"
	"testing"
	"time"
)

func TestBuildAgentResumePromptIncludesRecoveryConstraints(t *testing.T) {
	prompt := BuildAgentResumePrompt(AgentRunSnapshot{
		ID:        "run-1",
		Phase:     "tool_calling",
		UpdatedAt: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		Events: []AgentRunEvent{
			{Type: "phase", Name: "planning"},
			{Type: "tool", Name: "GetStockQuote", Status: "ok", ArgPreview: "600519", ResultPreview: "价格=100"},
		},
	})
	for _, expected := range []string{"未完成任务", "GetStockQuote", "价格=100", "不要把以下摘要当作最新数据", "实时行情"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("resume prompt missing %q: %s", expected, prompt)
		}
	}
}
