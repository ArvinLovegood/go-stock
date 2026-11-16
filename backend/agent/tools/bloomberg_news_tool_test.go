package tools

import (
	"context"
	"strings"
	"testing"

	"go-stock/backend/data"
)

func TestBloombergNewsToolAvailabilityAndBacktest(t *testing.T) {
	initToolGroupsTestDB(t)
	var found *DataToolWrapper
	for _, item := range FilterToolsByGroups(GetAllDataTools(), ClassifyQuestion("彭博市场新闻")) {
		info, err := item.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Name == "GetBloombergMarketNews" {
			found = item.(*DataToolWrapper)
		}
	}
	if found == nil || !data.IsToolKeyConfigured("GetBloombergMarketNews") {
		t.Fatal("Bloomberg news tool must be registered without an API key")
	}
	ctx := WithAgentMeta(context.Background(), AgentMeta{IsPromptBacktest: true})
	if _, err := found.InvokableRun(ctx, `{}`); err == nil || !strings.Contains(err.Error(), "不支持历史时点查询") {
		t.Fatalf("backtest must be rejected, got %v", err)
	}
	for _, args := range []string{`{"limit":0}`, `{"limit":-1}`, `{"limit":21}`} {
		if _, err := found.InvokableRun(context.Background(), args); err == nil {
			t.Errorf("expected invalid limit error for %s", args)
		}
	}
}
