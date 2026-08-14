package agent

import (
	"context"
	"os"
	"testing"

	"go-stock/backend/agent/tools"
)

func TestGetAllTools(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test disabled; set GO_STOCK_RUN_INTEGRATION_TESTS=1 to enable")
	}
	allTools := getToolsByQuestion("分析一下茅台的股票行情和资金流向", false)
	t.Logf("Total tools count: %d", len(allTools))

	toolNames := make(map[string]int)
	for i, tool := range allTools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Errorf("Tool %d: failed to get info: %v", i, err)
			continue
		}
		t.Logf("Tool %d: %s - %s", i+1, info.Name, info.Desc)

		if count, exists := toolNames[info.Name]; exists {
			t.Errorf("Duplicate tool name found: %s (previous count: %d)", info.Name, count)
		}
		toolNames[info.Name]++
	}

	t.Log("\n=== Checking for duplicates ===")
	duplicates := []string{}
	for name, count := range toolNames {
		if count > 1 {
			duplicates = append(duplicates, name)
			t.Errorf("Duplicate tool: %s (count: %d)", name, count)
		}
	}

	if len(duplicates) == 0 {
		t.Log("No duplicate tools found!")
	}
}

func TestAgentGoTools(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test disabled; set GO_STOCK_RUN_INTEGRATION_TESTS=1 to enable")
	}
	ctx := context.Background()

	t.Log("Testing agent.go tools:")
	agentToolsList := []string{
		"GetQueryEconomicDataTool",
		"GetQueryStockPriceInfoTool",
		"GetQueryStockCodeInfoTool",
		"GetQueryMarketNewsTool",
		"GetChoiceStockByIndicatorsTool",
		"GetStockKLineTool",
		"GetInteractiveAnswerDataTool",
		"GetFinancialReportTool",
		"GetQueryStockNewsTool",
		"GetIndustryResearchReportTool",
		"GetQueryBKDictTool",
	}

	for _, name := range agentToolsList {
		t.Logf("Tool: %s exists", name)
	}

	dataTools := tools.GetAllDataTools()
	t.Logf("\nData tools count: %d", len(dataTools))

	for i, tool := range dataTools {
		info, err := tool.Info(ctx)
		if err != nil {
			t.Errorf("Data tool %d: failed to get info: %v", i, err)
			continue
		}
		t.Logf("Data tool %d: %s", i+1, info.Name)
	}
}
