package agent

import (
	"context"
	"testing"
)

func TestVerifyFinancialAnswerRequiresToolEvidence(t *testing.T) {
	traceCtx, trace := NewAgentTurnTrace(context.Background(), "查询行情")
	_ = traceCtx
	warning := VerifyFinancialAnswer("600519 当前股价 123.45 元，涨跌幅 2.3%", trace)
	if warning == "" {
		t.Fatal("expected warning when financial facts have no successful tool call")
	}

	trace.RecordToolCall("GetStockInfo", "ok", `{"stockCode":"600519"}`)
	if warning := VerifyFinancialAnswer("600519 当前股价 123.45 元，涨跌幅 2.3%", trace); warning != "" {
		t.Fatalf("unexpected warning after successful tool call: %s", warning)
	}
}

func TestVerifyFinancialAnswerIgnoresGeneralText(t *testing.T) {
	if warning := VerifyFinancialAnswer("建议关注估值变化，具体数据需要进一步确认。", nil); warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
}
