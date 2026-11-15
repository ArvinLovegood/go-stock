package tools

import (
	"context"
	"testing"

	"go-stock/backend/data"
)

func TestPublicMacroToolsRegistered(t *testing.T) {
	initToolGroupsTestDB(t)
	want := map[string]bool{"GetWorldBankIndicator": false, "GetTreasuryExchangeRates": false}
	for _, tool := range FilterToolsByGroups(GetAllDataTools(), ClassifyQuestion("查询美元汇率和世界银行通胀")) {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if _, ok := want[info.Name]; ok {
			want[info.Name] = true
			if !data.IsToolKeyConfigured(info.Name) {
				t.Fatalf("%s unexpectedly requires a key", info.Name)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s not available to the market group", name)
		}
	}
}

func TestPublicMacroToolsBlockedInBacktest(t *testing.T) {
	ctx := WithAgentMeta(context.Background(), AgentMeta{IsPromptBacktest: true})
	for _, tool := range getPublicMacroTools() {
		wrapper := tool.(*DataToolWrapper)
		if _, err := wrapper.InvokableRun(ctx, `{}`); err == nil {
			t.Errorf("%s should reject point-in-time backtests", wrapper.name)
		}
	}
}
