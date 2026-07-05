package tools

import "strings"

// QuestionIntent 用户问题意图，用于 Agent 模式选择与工具组兜底。
type QuestionIntent int

const (
	IntentGeneral QuestionIntent = iota
	IntentQuoteLookup
	IntentCodeLookup
	IntentScreening
	IntentMarketOverview
	IntentNewsResearch
	IntentMoneyFlow
	IntentComprehensiveReport
)

func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if kw != "" && strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// DetectQuestionIntent 基于关键词识别问题意图（规则路由，非 LLM）。
func DetectQuestionIntent(question string) QuestionIntent {
	lowerQ := strings.ToLower(question)

	if containsAny(lowerQ,
		"全面分析", "综合分析", "深度分析", "详细分析", "系统分析",
		"多维度", "全方位",
		"投资建议", "操作建议", "买卖建议",
		"对比分析", "比较分析", "横向对比",
		"投资组合", "资产配置", "仓位管理",
		"风险评估", "风险分析",
		"研究报告", "投资报告",
		"产业链", "赛道分析",
	) {
		return IntentComprehensiveReport
	}

	if containsAny(lowerQ,
		"筛选", "选股", "条件选股", "指标选股", "智能选股",
		"选板块", "选etf", "选基金", "选港股", "选美股", "选可转债",
		"形态选股", "macd金叉", "kdj金叉", "放量突破",
		"热门策略", "连涨", "连跌", "多头排列", "空头排列",
	) {
		return IntentScreening
	}

	if containsAny(lowerQ,
		"新闻", "资讯", "公告", "研报", "研究报告", "券商观点",
		"龙虎榜", "互动易", "投资者互动", "华尔街见闻",
	) {
		return IntentNewsResearch
	}

	if containsAny(lowerQ,
		"资金", "流入", "流出", "净流入", "净流出", "主力", "北向", "南向",
	) {
		return IntentMoneyFlow
	}

	if containsAny(lowerQ,
		"大盘", "市场", "指数", "涨跌分布", "涨停", "跌停", "全球指数",
		"宏观", "gdp", "cpi", "ppi", "pmi", "期货", "期权", "问财",
	) {
		return IntentMarketOverview
	}

	if containsAny(lowerQ,
		"代码", "简称", "叫什么", "是哪只", "股票名称",
	) && !containsAny(lowerQ, "股价", "价格", "涨跌", "行情", "多少钱") {
		return IntentCodeLookup
	}

	if containsAny(lowerQ,
		"股价", "价格", "多少钱", "涨跌", "涨幅", "跌幅", "行情",
		"开盘价", "收盘价", "最高价", "最低价", "实时", "最新",
		"pe", "pb", "roe", "财务", "营收", "利润", "k线", "分时",
	) {
		return IntentQuoteLookup
	}

	if containsAny(lowerQ, "分析", "诊断", "评估", "怎么样") {
		wordCount := len([]rune(question))
		if wordCount > 60 {
			return IntentComprehensiveReport
		}
		return IntentQuoteLookup
	}

	return IntentGeneral
}

// DefaultToolGroupsForIntent 模糊问题时的最小工具组兜底。
func DefaultToolGroupsForIntent(intent QuestionIntent) []ToolGroup {
	switch intent {
	case IntentMarketOverview:
		return []ToolGroup{GroupMarket}
	case IntentNewsResearch:
		return []ToolGroup{GroupNewsResearch}
	case IntentScreening:
		return []ToolGroup{GroupScreening, GroupStockAnalysis}
	case IntentMoneyFlow:
		return []ToolGroup{GroupMoneyFlow, GroupStockAnalysis}
	case IntentComprehensiveReport:
		return []ToolGroup{GroupStockAnalysis, GroupMarket, GroupNewsResearch}
	case IntentCodeLookup:
		return []ToolGroup{GroupBase}
	default:
		return []ToolGroup{GroupStockAnalysis}
	}
}
