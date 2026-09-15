package agent

// prompt_backtest_engine_test.go — 主动回测引擎纯函数守护测试（不依赖 DB）。

import (
	"testing"

	"go-stock/backend/models"
)

func TestExtractPicksJSON(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"裸JSON数组", `[{"code":"600000","name":"浦发银行","rating":"看好","reason":"r"}]`, 1},
		{"代码块包裹", "```json\n[{\"code\":\"000001\",\"name\":\"平安银行\",\"rating\":\"强烈看好\"}]\n```", 1},
		{"前后杂文", "以下是分析：\n[{\"code\":\"600000\"},{\"code\":\"000001\"}]\n以上。",
			2},
		{"空数组", "[]", 0},
		{"无JSON", "抱歉，我无法完成。", 0},
	}
	for _, c := range cases {
		got := extractPicksJSON(c.in)
		if len(got) != c.want {
			t.Errorf("%s：解析出 %d 条，want %d", c.name, len(got), c.want)
		}
	}
	// 字段解析
	picks := extractPicksJSON(`[{"code":"600000","name":"浦发银行","rating":"看好","reason":"r"}]`)
	if len(picks) != 1 || picks[0].Code != "600000" || picks[0].Name != "浦发银行" {
		t.Errorf("字段解析错误：%+v", picks)
	}
}

func TestNormalizePickCode(t *testing.T) {
	cases := map[string]string{
		"600000":    "600000",
		"sh600000":  "600000",
		"sz000001":  "000001",
		"600000.SH": "600000",
		"000001.SZ": "000001",
		" 600519 ":  "600519",
		"bk0475":    "", // 非纯数字长度不对
		"60000":     "",
		"abc":       "",
		"":          "",
	}
	for in, want := range cases {
		if got := normalizePickCode(in); got != want {
			t.Errorf("normalizePickCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildPickEquityCurve(t *testing.T) {
	picks := []models.PromptBacktestPick{
		{TradeDate: "2026-01-05", ReturnPct: 10},
		{TradeDate: "2026-01-05", ReturnPct: -6}, // 同日等权 → 2%
		{TradeDate: "2026-01-06", ReturnPct: -20},
	}
	curve := buildPickEquityCurve(picks)
	if len(curve) != 2 {
		t.Fatalf("曲线点数 = %d, want 2", len(curve))
	}
	if curve[0].DailyPct != 2 {
		t.Errorf("首日组合收益 = %v, want 2", curve[0].DailyPct)
	}
	if curve[1].Equity < 0.815 || curve[1].Equity > 0.817 {
		t.Errorf("次日净值 = %v, want ≈0.816", curve[1].Equity)
	}
	if buildPickEquityCurve(nil) != nil {
		t.Errorf("空入参应返回 nil")
	}
}

func TestComputePickJaccard(t *testing.T) {
	picks := []models.PromptBacktestPick{
		// 同日 run1 与 run2 完全一致 → Jaccard=1
		{TradeDate: "2026-01-05", RunIndex: 1, StockCode: "600000"},
		{TradeDate: "2026-01-05", RunIndex: 1, StockCode: "000001"},
		{TradeDate: "2026-01-05", RunIndex: 2, StockCode: "600000"},
		{TradeDate: "2026-01-05", RunIndex: 2, StockCode: "000001"},
		// 次日 run1 与 run2 无重合 → Jaccard=0
		{TradeDate: "2026-01-06", RunIndex: 1, StockCode: "600519"},
		{TradeDate: "2026-01-06", RunIndex: 2, StockCode: "300750"},
	}
	j := computePickJaccard(picks)
	if j != 0.5 { // (1 + 0) / 2
		t.Errorf("Jaccard = %v, want 0.5", j)
	}
	// 无重复跑 → -1
	if j := computePickJaccard(picks[:2]); j != -1 {
		t.Errorf("单 run 应返回 -1，实际 %v", j)
	}
	// 一半重合：{A,B} vs {B,C} → 1/3
	mixed := []models.PromptBacktestPick{
		{TradeDate: "2026-01-05", RunIndex: 1, StockCode: "A"},
		{TradeDate: "2026-01-05", RunIndex: 1, StockCode: "B"},
		{TradeDate: "2026-01-05", RunIndex: 2, StockCode: "B"},
		{TradeDate: "2026-01-05", RunIndex: 2, StockCode: "C"},
	}
	if j := computePickJaccard(mixed); j != 0.33 {
		t.Errorf("Jaccard = %v, want 0.33", j)
	}
}

func TestParseTemplateIds(t *testing.T) {
	got := parseTemplateIds("1, 3,5,,x,0,-7")
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Errorf("parseTemplateIds = %v, want [1 3 5]", got)
	}
	if got := parseTemplateIds("1, 3, 5, 7"); len(got) != 4 || got[3] != 7 {
		t.Errorf("parseTemplateIds = %v, want [1 3 5 7]", got)
	}
}
