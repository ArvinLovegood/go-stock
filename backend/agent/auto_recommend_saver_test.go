package agent

// auto_recommend_saver_test.go — 推荐记录自动保存纯函数守护测试（不依赖 DB/网络）。

import (
	"testing"
)

func TestExtractPicksLoose(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"裸JSON数组", `[{"code":"600000","name":"浦发银行","rating":"看好","reason":"r"}]`, 1},
		{"代码块包裹", "```json\n[{\"code\":\"000001\",\"name\":\"平安银行\",\"rating\":\"强烈看好\",\"reason\":\"r\"}]\n```", 1},
		{"正文含Markdown链接后接JSON", "参考 [东方财富](https://www.eastmoney.com) 数据。\n[{\"code\":\"600519\",\"name\":\"贵州茅台\",\"rating\":\"看好\",\"reason\":\"龙头\"}]", 1},
		{"字符串内含方括号", `[{"code":"600000","name":"浦发[银行]","rating":"看好","reason":"括号[1]测试"}]`, 1},
		{"首个数组非推荐结构继续扫描", "表格列 [[列A,列B]]\n[{\"code\":\"300750\",\"name\":\"宁德时代\",\"rating\":\"中性\",\"reason\":\"r\"}]", 1},
		{"首尾方括号干扰（整体跨度解析必失败）", "选出如下[1]：\n```json\n[{\"code\":\"600000\",\"name\":\"浦发银行\",\"rating\":\"看好\",\"reason\":\"r\"}]\n```\n注[1]", 1},
		{"无JSON", "本轮未发现值得推荐的标的。", 0},
		{"只有非对象数组", "[[1,2,3]]", 0},
	}
	for _, c := range cases {
		got := extractPicksLoose(c.in)
		if len(got) != c.want {
			t.Errorf("%s：解析出 %d 条，want %d", c.name, len(got), c.want)
		}
	}

	// 字段解析
	picks := extractPicksLoose("结论如下：\n[{\"code\":\"600519\",\"name\":\"贵州茅台\",\"rating\":\"看好\",\"reason\":\"业绩稳健\"}]")
	if len(picks) != 1 || picks[0].Code != "600519" || picks[0].Name != "贵州茅台" ||
		picks[0].Rating != "看好" || picks[0].Reason != "业绩稳健" {
		t.Errorf("字段解析错误：%+v", picks)
	}

	// 未闭合数组返回空
	if got := extractPicksLoose(`[{"code":"600000"`); got != nil {
		t.Errorf("未闭合数组应返回 nil，实际 %v", got)
	}
}

func TestMatchJSONArray(t *testing.T) {
	cases := []struct {
		name, in string
		start   int
		want    int // 期望配对 ']' 的下标，-1 表示无配对
	}{
		{"简单数组", `[1,2]`, 0, 4},
		{"嵌套数组", `[[1],[2]]`, 0, 8},
		{"字符串内括号不计入", `["a]b",1]`, 0, 8},
		{"转义引号", `["a\"b]",1]`, 0, 10},
		{"未闭合", `[1,2`, 0, -1},
		{"起始非左括号", `abc`, 0, -1},
	}
	for _, c := range cases {
		if got := matchJSONArray(c.in, c.start); got != c.want {
			t.Errorf("%s：matchJSONArray(%q,%d) = %d, want %d", c.name, c.in, c.start, got, c.want)
		}
	}
	if got := matchJSONArray("abc", -1); got != -1 {
		t.Errorf("非法 start 应返回 -1，实际 %d", got)
	}
}

func TestAStockSuffixedCode(t *testing.T) {
	cases := map[string]string{
		"600519": "600519.SH",
		"688981": "688981.SH",
		"000001": "000001.SZ",
		"300750": "300750.SZ",
		"832000": "832000.BJ",
		"430047": "430047.BJ",
		"":       "",
	}
	for in, want := range cases {
		if got := aStockSuffixedCode(in); got != want {
			t.Errorf("aStockSuffixedCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsPromptBacktestCall(t *testing.T) {
	// 提问含场景标记
	if !isPromptBacktestCall("今天是 2026-01-05（A股交易日，模拟回测场景：忽略素材之外的时间）。\n...", "") {
		t.Errorf("含场景标记的提问应识别为回测调用")
	}
	// sysPrompt 含输出契约标记
	if !isPromptBacktestCall("", "你是选股大师\n【输出格式要求（必须严格遵守，优先级高于上文）】...") {
		t.Errorf("含输出契约标记的 sysPrompt 应识别为回测调用")
	}
	// 普通分析调用
	if isPromptBacktestCall("帮我分析贵州茅台的投资价值", "你是顶级股票投资大师...") {
		t.Errorf("普通分析调用不应误判为回测")
	}
}
