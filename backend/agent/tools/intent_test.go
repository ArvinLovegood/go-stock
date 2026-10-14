package tools

import "testing"

func TestDetectQuestionIntent(t *testing.T) {
	tests := []struct {
		question string
		want     QuestionIntent
	}{
		{"今天茅台股价多少", IntentQuoteLookup},
		{"查询一下平安银行的代码", IntentCodeLookup},
		{"全面分析贵州茅台的投资价值", IntentComprehensiveReport},
		{"帮我筛选MACD金叉的股票", IntentScreening},
		{"今天大盘怎么样", IntentMarketOverview},
		{"最近有什么新闻", IntentNewsResearch},
		{"北向资金流入情况", IntentMoneyFlow},
		{"你好", IntentGeneral},
	}
	for _, tt := range tests {
		got := DetectQuestionIntent(tt.question)
		if got != tt.want {
			t.Errorf("DetectQuestionIntent(%q) = %v, want %v", tt.question, got, tt.want)
		}
	}
}

// 核心五组（个股分析/行情/资讯/选股/资金流）对任何「正事问题」常驻，与措辞无关；
// 但「运营」（自选/持仓/操作计划/分组与 MCP 管理）与「加密」这类需明确指令的分组不该被带上。
func TestClassifyQuestionCoreGroupsAlwaysOn(t *testing.T) {
	questions := []string{
		"现在有什么机会",
		"今天大盘指数点位怎么样",
		"贵州茅台股价多少",
	}
	for _, q := range questions {
		groups := ClassifyQuestion(q)
		if !groups[GroupBase] {
			t.Errorf("%q: base 组应常驻", q)
		}
		for _, g := range coreResearchGroups {
			if !groups[g] {
				t.Errorf("%q: 核心组 %s 应常驻", q, g)
			}
		}
		if groups[GroupOperations] {
			t.Errorf("%q: 运营组需用户明确指令，不应注入: %v", q, groups)
		}
		if groups[GroupCrypto] {
			t.Errorf("%q: 加密组需用户明确指令，不应注入: %v", q, groups)
		}
		if groups[GroupAIAnalysis] {
			t.Errorf("%q: AI 分析组需用户明确指令，不应注入: %v", q, groups)
		}
	}
}

// 纯闲聊/问候只保留 base 组，不注入核心研究组：
// 「你好」没必要背着 180+ 工具 schema，闲聊会显著拖慢首 token。
func TestClassifyQuestionChitchatGetsBaseOnly(t *testing.T) {
	questions := []string{
		"你好",
		"你好，用一句话介绍你自己",
		"你是谁",
		"hello",
		"谢谢",
	}
	for _, q := range questions {
		groups := ClassifyQuestion(q)
		if !groups[GroupBase] {
			t.Errorf("%q: base 组应保留", q)
		}
		for _, g := range coreResearchGroups {
			if groups[g] {
				t.Errorf("%q: 闲聊不应注入核心组 %s: %v", q, g, groups)
			}
		}
	}
}

// 问候+正事的混合输入不受闲聊判定影响：命中领域关键词即走正常分组路径。
func TestChitchatWithRealRequestKeepsCoreGroups(t *testing.T) {
	groups := ClassifyQuestion("你好，帮我分析一下贵州茅台的最新行情")
	if !groups[GroupStockAnalysis] || !groups[GroupMarket] {
		t.Errorf("混合输入应命中领域组: %v", groups)
	}
	if !IsChitchatQuestion("你好") || IsChitchatQuestion("现在有什么机会") {
		t.Errorf("闲聊判定不准确")
	}
}

// 关键词只做增量：命中相应关键词时才额外放开运营/加密等分组。
func TestClassifyQuestionKeywordAddsExtraGroups(t *testing.T) {
	groups := ClassifyQuestion("帮我看看币安BTC永续合约的资金费率，再把这只股票加入自选")
	if !groups[GroupCrypto] {
		t.Errorf("命中「币安/BTC/永续」应放开加密组: %v", groups)
	}
	if !groups[GroupOperations] {
		t.Errorf("命中「加入自选」应放开运营组: %v", groups)
	}
	// 增量不该挤掉常驻核心组。
	for _, g := range coreResearchGroups {
		if !groups[g] {
			t.Errorf("放开了增量分组后核心组 %s 仍应常驻", g)
		}
	}
}
