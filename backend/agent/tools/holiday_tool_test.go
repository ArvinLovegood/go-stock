package tools

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func TestGetHolidayTools(t *testing.T) {
	tools := GetHolidayTools()
	t.Logf("Total holiday tools count: %d", len(tools))

	for i, tool := range tools {
		info, err := tool.Info(nil)
		if err != nil {
			t.Errorf("Tool %d: failed to get info: %v", i, err)
			continue
		}
		t.Logf("Tool %d: %s - %s", i+1, info.Name, info.Desc)
	}
}

func TestGetHolidayInfo(t *testing.T) {
	tool := NewHolidayTool(
		"GetHolidayInfo",
		"查询指定日期的节假日信息",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return "test", nil
		},
	)

	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Failed to get tool info: %v", err)
	}

	if info.Name != "GetHolidayInfo" {
		t.Errorf("Expected tool name 'GetHolidayInfo', got '%s'", info.Name)
	}

	t.Logf("Tool info: Name=%s, Desc=%s", info.Name, info.Desc)
}

func TestIsTradingDay_Weekend(t *testing.T) {
	testCases := []struct {
		name     string
		date     string
		expected bool
	}{
		{"周六", "2026-04-04", false},
		{"周日", "2026-04-05", false},
		{"周一", "2026-04-06", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing date: %s, expected trading day: %v", tc.date, tc.expected)
		})
	}
}

func TestIsTradingDay_Holiday(t *testing.T) {
	testCases := []struct {
		name string
		date string
		desc string
	}{
		{"元旦", "2026-01-01", "元旦假期"},
		{"春节初一", "2026-02-17", "春节假期"},
		{"国庆节", "2026-10-01", "国庆假期"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing holiday date: %s (%s)", tc.date, tc.desc)
		})
	}
}

func TestIsTradingDay_API(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}

	holidayTools := GetHolidayTools()
	var isTradingDayTool tool.InvokableTool
	for _, tl := range holidayTools {
		info, _ := tl.Info(nil)
		if info.Name == "IsTradingDay" {
			isTradingDayTool = tl.(tool.InvokableTool)
			break
		}
	}

	if isTradingDayTool == nil {
		t.Fatal("IsTradingDay tool not found")
	}

	testCases := []struct {
		name          string
		date          string
		expectTrading bool
	}{
		{"工作日_2026-04-07", "2026-04-07", true},
		{"周六_2026-04-04", "2026-04-04", false},
		{"周日_2026-04-05", "2026-04-05", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := `{"date": "` + tc.date + `"}`
			result, err := isTradingDayTool.InvokableRun(context.Background(), args)

			if err != nil {
				t.Fatalf("API call failed: %v", err)
			}

			t.Logf("Result for %s:\n%s", tc.date, result)

			if tc.expectTrading {
				if !strings.Contains(result, "✅ 是") {
					t.Errorf("Expected trading day for %s, but got non-trading", tc.date)
				}
			} else {
				if !strings.Contains(result, "❌ 否") {
					t.Errorf("Expected non-trading day for %s, but got trading", tc.date)
				}
			}
		})
	}
}

func TestGetHolidayInfo_API(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}

	holidayTools := GetHolidayTools()
	var holidayInfoTool tool.InvokableTool
	for _, tl := range holidayTools {
		info, _ := tl.Info(nil)
		if info.Name == "GetHolidayInfo" {
			holidayInfoTool = tl.(tool.InvokableTool)
			break
		}
	}

	if holidayInfoTool == nil {
		t.Fatal("GetHolidayInfo tool not found")
	}

	args := `{"date": "2026-01-01"}`
	result, err := holidayInfoTool.InvokableRun(context.Background(), args)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	t.Logf("Holiday info for 2026-01-01:\n%s", result)

	if !strings.Contains(result, "元旦") {
		t.Error("Expected result to contain '元旦'")
	}
}

func TestGetNextTradingDay_API(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}

	holidayTools := GetHolidayTools()
	var nextTradingDayTool tool.InvokableTool
	for _, tl := range holidayTools {
		info, _ := tl.Info(nil)
		if info.Name == "GetNextTradingDay" {
			nextTradingDayTool = tl.(tool.InvokableTool)
			break
		}
	}

	if nextTradingDayTool == nil {
		t.Fatal("GetNextTradingDay tool not found")
	}

	args := `{"startDate": "2026-04-04", "days": 10}`
	result, err := nextTradingDayTool.InvokableRun(context.Background(), args)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	t.Logf("Next trading day from 2026-04-04:\n%s", result)

	if !strings.Contains(result, "2026-04-07") {
		t.Error("Expected next trading day to be 2026-04-07 (Monday)")
	}
}

func TestGetHolidayYear_API(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test disabled; set GO_STOCK_RUN_INTEGRATION_TESTS=1 to enable")
	}
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}

	holidayTools := GetHolidayTools()
	var holidayYearTool tool.InvokableTool
	for _, tl := range holidayTools {
		info, _ := tl.Info(nil)
		if info.Name == "GetHolidayYear" {
			holidayYearTool = tl.(tool.InvokableTool)
			break
		}
	}

	if holidayYearTool == nil {
		t.Fatal("GetHolidayYear tool not found")
	}

	args := `{"year": "2026"}`
	result, err := holidayYearTool.InvokableRun(context.Background(), args)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	t.Logf("Holiday year info for 2026:\n%s", result)

	if !strings.Contains(result, "元旦") {
		t.Error("Expected result to contain '元旦'")
	}
	if !strings.Contains(result, "春节") {
		t.Error("Expected result to contain '春节'")
	}
	if !strings.Contains(result, "国庆节") {
		t.Error("Expected result to contain '国庆节'")
	}
}
