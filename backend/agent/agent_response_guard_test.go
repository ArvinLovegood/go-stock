package agent

import "testing"

func TestEvaluateResponseGuard(t *testing.T) {
	traceWithData := &AgentTurnTrace{
		ToolCalls: []ToolCallRecord{
			{Name: "GetStockInfo", Status: "ok"},
		},
	}
	traceNoData := &AgentTurnTrace{}

	tests := []struct {
		name     string
		question string
		answer   string
		trace    *AgentTurnTrace
		wantOK   bool
	}{
		{
			name:     "conceptual without numbers",
			question: "什么是MACD",
			answer:   "MACD 是趋势指标，由快线慢线构成。",
			trace:    traceNoData,
			wantOK:   true,
		},
		{
			name:     "price question verified",
			question: "今天茅台股价多少",
			answer:   "贵州茅台现价 1850.50 元，涨幅 +2.1%",
			trace:    traceWithData,
			wantOK:   true,
		},
		{
			name:     "price question unverified",
			question: "今天茅台股价多少",
			answer:   "贵州茅台现价 1850.50 元，涨幅 +2.1%",
			trace:    traceNoData,
			wantOK:   false,
		},
		{
			name:     "price question admits unavailable",
			question: "今天茅台股价多少",
			answer:   "当前未能获取到最新数据，请稍后重试。",
			trace:    traceNoData,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateResponseGuard(tt.question, tt.answer, tt.trace)
			if got.OK != tt.wantOK {
				t.Fatalf("evaluateResponseGuard() OK=%v want=%v reason=%s", got.OK, tt.wantOK, got.Reason)
			}
		})
	}
}

func TestIsDataFetchingTool(t *testing.T) {
	if isDataFetchingTool("GetCurrentTime") {
		t.Fatal("GetCurrentTime should not be data tool")
	}
	if !isDataFetchingTool("GetStockInfo") {
		t.Fatal("GetStockInfo should be data tool")
	}
}

func TestResponseHasNumericFactualClaims(t *testing.T) {
	if !responseHasNumericFactualClaims("涨幅 +2.35%") {
		t.Fatal("expected percent claim")
	}
	if responseHasNumericFactualClaims("MACD 由快慢线构成") {
		t.Fatal("should not treat indicator name as numeric claim")
	}
}

func TestAgentTurnTraceRecord(t *testing.T) {
	ctx, trace := NewAgentTurnTrace(t.Context(), "测试")
	if AgentTurnTraceFromContext(ctx) != trace {
		t.Fatal("trace not in context")
	}
	trace.RecordToolCall("GetStockInfo", "ok", `{"code":"600519"}`)
	if !trace.HasSuccessfulDataToolCall() {
		t.Fatal("expected successful data tool call")
	}
}
