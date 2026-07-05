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

func TestClassifyQuestionAmbiguousFallback(t *testing.T) {
	groups := ClassifyQuestion("你好")
	if !groups[GroupBase] {
		t.Fatal("expected base group")
	}
	if groups[GroupStockAnalysis] && groups[GroupMarket] && groups[GroupNewsResearch] {
		t.Fatal("ambiguous question should not expand to all major groups")
	}
	if !groups[GroupStockAnalysis] {
		t.Fatal("expected stock analysis fallback for general intent")
	}
}

func TestClassifyQuestionMarketFallback(t *testing.T) {
	groups := ClassifyQuestion("看看")
	if groups[GroupMarket] && groups[GroupNewsResearch] && groups[GroupStockAnalysis] {
		t.Fatal("vague question should not load all groups")
	}
}
