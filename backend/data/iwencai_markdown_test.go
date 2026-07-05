package data

import (
	"strings"
	"testing"
)

func TestCollectIwencaiColumnKeysUnionAllRows(t *testing.T) {
	datas := []map[string]any{
		{"股票代码": "001", "A": "1", "最新价": 10.0},
		{"股票代码": "002", "B": "2", "解禁日期": "2026-07-10"},
	}
	cols := collectIwencaiColumnKeys(datas)
	if len(cols) != 5 {
		t.Fatalf("expected 5 columns, got %d: %v", len(cols), cols)
	}
	if cols[0] != "股票代码" {
		t.Fatalf("expected 股票代码 first, got %v", cols)
	}
}

func TestRenderIwencaiSingleEntityUsesKVTable(t *testing.T) {
	row := map[string]any{
		"股票代码":          "600519.SH",
		"股票简称":          "贵州茅台",
		"最新价":           1194.45,
		"最新涨跌幅":         -0.710723,
		"收盘价[20260703]": 1194.45,
		"开盘价[20260702]": 1200.0,
		"开盘价[20260703]": 1205.24,
	}
	out := renderIwencaiSingleEntityMarkdown(row)
	if !strings.Contains(out, "| 字段 | 值 |") {
		t.Fatalf("expected kv table: %s", out)
	}
	if !strings.Contains(out, "#### 贵州茅台 (600519.SH)") {
		t.Fatalf("expected entity title: %s", out)
	}
	if !strings.Contains(out, "-0.71%") {
		t.Fatalf("expected formatted percent: %s", out)
	}
	if !strings.Contains(out, "开盘价（按日期）") {
		t.Fatalf("expected date series section: %s", out)
	}
}

func TestRenderIwencaiMultiRowIncludesUnionColumns(t *testing.T) {
	datas := []map[string]any{
		{"股票代码": "001", "股票简称": "测试A", "字段1": "v1"},
		{"股票代码": "002", "股票简称": "测试B", "字段2": "v2"},
	}
	out := renderIwencaiMultiRowMarkdown(datas)
	if !strings.Contains(out, "字段1") || !strings.Contains(out, "字段2") {
		t.Fatalf("expected union columns: %s", out)
	}
}

func TestBuildIwencaiQuerySummaryWithChunks(t *testing.T) {
	result := &IwencaiResponse{
		CodeCount:  42,
		ChunksInfo: []any{"变动日期等于1周后 (42)"},
		Datas:      []map[string]any{{"股票代码": "1"}},
	}
	summary := buildIwencaiQuerySummary(result, 1, 5, 13, iwencaiKindStock)
	if !strings.Contains(summary, "42 只标的") {
		t.Fatalf("missing count label: %s", summary)
	}
	if !strings.Contains(summary, "匹配条件") {
		t.Fatalf("missing chunks info: %s", summary)
	}
}

func TestRenderIwencaiSearchMarkdown(t *testing.T) {
	items := []IwencaiSearchItem{
		{Title: "新闻A", PublishDate: "2026-07-05", Summary: strings.Repeat("摘要", 200), URL: "https://example.com/a"},
		{Title: "新闻B", PublishDate: "2026-07-04", Summary: "短摘要", URL: "https://example.com/b"},
		{Title: "新闻C", PublishDate: "2026-07-03"},
		{Title: "新闻D", PublishDate: "2026-07-02"},
	}
	out := renderIwencaiSearchMarkdown("贵州茅台", "新闻", items)
	if !strings.Contains(out, "| # | 标题 | 发布时间 |") {
		t.Fatalf("expected overview table for many results: %s", out)
	}
	if !strings.Contains(out, "...(省略") && len([]rune(strings.Repeat("摘要", 200))) > iwencaiSearchSummaryMaxRunes {
		// truncate uses ... suffix
	}
	if !strings.Contains(out, "https://example.com/a") {
		t.Fatalf("expected detail links: %s", out)
	}
	longSummary := strings.Repeat("摘要", 200)
	if !strings.Contains(truncateIwencaiText(longSummary, iwencaiSearchSummaryMaxRunes), "...") {
		t.Fatalf("expected truncated summary suffix")
	}
}

func TestFormatIwencaiCellEscapesPipe(t *testing.T) {
	got := formatIwencaiCell("a|b")
	if got != `a\|b` {
		t.Fatalf("expected escaped pipe, got %q", got)
	}
}

func TestFormatIwencaiColumnHeader(t *testing.T) {
	got := formatIwencaiColumnHeader("涨跌幅[20260703]", "20260703")
	if got != "涨跌幅" {
		t.Fatalf("expected stripped date header, got %q", got)
	}
	got = formatIwencaiColumnHeader("涨跌幅[20260703]", "")
	if got != "涨跌幅（2026-07-03）" {
		t.Fatalf("expected formatted date header, got %q", got)
	}
}

func TestRenderIwencaiMacroMarkdown(t *testing.T) {
	row := map[string]any{
		"macro_id":   "M002826938",
		"macro_name": "GDP:同比",
		"指标":         "GDP同比增长率",
		"指标值":        5,
		"单位":         "%",
		"时间":         "20251231",
		"国家":         "中国",
		"周期":         "年",
		"地区级别":       []string{"国家"},
	}
	out := renderIwencaiMacroMarkdown(row)
	if strings.Contains(out, "macro_id") {
		t.Fatalf("macro internal fields should be hidden: %s", out)
	}
	if !strings.Contains(out, "#### GDP同比增长率") {
		t.Fatalf("expected macro title: %s", out)
	}
	if !strings.Contains(out, "2025-12-31") {
		t.Fatalf("expected formatted date: %s", out)
	}
	if !strings.Contains(out, "国家") {
		t.Fatalf("expected region level: %s", out)
	}
}

func TestRenderIwencaiLongTextSeparated(t *testing.T) {
	row := map[string]any{
		"股票代码": "002475.SZ",
		"股票简称": "立讯精密",
		"最新价":  64.44,
		"内容":   strings.Repeat("调研", 400),
		"机构家数": 446,
	}
	out := renderIwencaiSingleEntityMarkdown(row)
	if strings.Contains(out, strings.Repeat("调研", 400)) {
		t.Fatalf("long text should be truncated: %s", out[:200])
	}
	if !strings.Contains(out, "**内容**") {
		t.Fatalf("expected long text section: %s", out)
	}
	if !strings.Contains(out, "| 机构家数 | 446 |") {
		t.Fatalf("expected normal field in table: %s", out)
	}
}

func TestFormatIwencaiBooleanAndLargeNumber(t *testing.T) {
	if got := formatIwencaiCellByColumn("涨停", true); got != "是" {
		t.Fatalf("expected 是, got %q", got)
	}
	if got := formatIwencaiCellByColumn("流通a股", 126289068); got != "1.26亿" {
		t.Fatalf("expected 1.26亿, got %q", got)
	}
	if got := formatIwencaiCellByColumn("变动日期", "20260706"); got != "2026-07-06" {
		t.Fatalf("expected formatted date value, got %q", got)
	}
}

func TestIwencaiResultCountLabelByKind(t *testing.T) {
	result := &IwencaiResponse{CodeCount: 3, Datas: make([]map[string]any, 3)}
	if got := iwencaiResultCountLabel(result, iwencaiKindIndex); got != "3 个指数" {
		t.Fatalf("unexpected index label: %s", got)
	}
	if got := iwencaiResultCountLabel(result, iwencaiKindMacro); got != "3 条记录" {
		t.Fatalf("unexpected macro label: %s", got)
	}
}
