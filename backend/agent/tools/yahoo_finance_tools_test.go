package tools

import (
	"context"
	"strings"
	"testing"
)

func TestYahooNewsToolRegisteredAndBlockedInBacktest(t *testing.T) {
	initToolGroupsTestDB(t)
	found := false
	for _, item := range FilterToolsByGroups(GetAllDataTools(), ClassifyQuestion("查询苹果的美股新闻")) {
		info, err := item.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Name == "SearchYahooStockNews" {
			found = true
			ctx := WithAgentMeta(context.Background(), AgentMeta{IsPromptBacktest: true})
			if _, err := item.(*DataToolWrapper).InvokableRun(ctx, `{"stockCode":"AAPL.US"}`); err == nil || !strings.Contains(err.Error(), "不支持历史时点查询") {
				t.Error("Yahoo news should be unavailable in point-in-time backtests")
			}
		}
	}
	if !found {
		t.Error("Yahoo news tool is not registered in the news group")
	}
}
