package tools

import (
	"strings"

	"github.com/cloudwego/eino/components/tool"
)

type ToolGroup string

const (
	GroupBase          ToolGroup = "base"
	GroupStockAnalysis ToolGroup = "stock_analysis"
	GroupMarket        ToolGroup = "market"
	GroupScreening     ToolGroup = "screening"
	GroupMoneyFlow     ToolGroup = "money_flow"
	GroupNewsResearch  ToolGroup = "news_research"
	GroupAIAnalysis    ToolGroup = "ai_analysis"
	GroupOperations    ToolGroup = "operations"
)

var toolGroupMap = map[string]ToolGroup{
	"QueryStockCodeInfo":           GroupBase,
	"QueryBKDictInfo":              GroupBase,
	"GetCurrentTime":               GroupBase,
	"GetFollowedStocks":            GroupBase,
	"CleanupStockCodes":            GroupBase,
	"GetHolidayInfo":               GroupBase,
	"GetHolidayYear":               GroupBase,
	"GetHolidayBatch":              GroupBase,
	"IsTradingDay":                 GroupBase,
	"GetNextTradingDay":            GroupBase,
	"CreateAiRecommendStocks":      GroupBase,
	"BatchCreateAiRecommendStocks": GroupBase,

	"GetStockInfo":            GroupStockAnalysis,
	"GetStockKLine":           GroupStockAnalysis,
	"GetEastMoneyKLine":       GroupStockAnalysis,
	"GetEastMoneyKLineWithMA": GroupStockAnalysis,
	"GetStockMinuteData":      GroupStockAnalysis,
	"GetStockFinancialInfo":   GroupStockAnalysis,
	"GetStockHolderNum":       GroupStockAnalysis,
	"GetStockRZRQInfo":        GroupStockAnalysis,
	"GetStockConceptInfo":     GroupStockAnalysis,

	"QueryEvent": GroupStockAnalysis,

	"QueryBasicInfo": GroupStockAnalysis,

	"QueryFinance": GroupStockAnalysis,

	"QueryIndustry": GroupStockAnalysis,

	"QueryManagement": GroupStockAnalysis,

	"QueryFundFinance":  GroupStockAnalysis,
	"QueryBusinessData": GroupStockAnalysis,

	"GetMarketData":              GroupMarket,
	"GlobalStockIndexesReadable": GroupMarket,
	"GetStockChanges":            GroupMarket,
	"GetStockChangeHistoryList":  GroupMarket,
	"GetDailyChangeStats":        GroupMarket,
	"GetChangeRank":              GroupMarket,
	"GetDailyDimensionStats":     GroupMarket,
	"GetTypeStatsByDate":         GroupMarket,
	"QueryIwencai":               GroupMarket,

	"QueryZhishu": GroupMarket,

	"QueryMacro": GroupMarket,

	"QueryFutures": GroupMarket,

	"QueryStockConnect": GroupMarket,

	"StockEarningsReview": GroupStockAnalysis,

	"FinancialQA": GroupAIAnalysis,

	"GetStockLatestFinance":       GroupStockAnalysis,
	"GetHKStockLatestFinance":     GroupStockAnalysis,
	"GetStockQtrMainFinance":      GroupStockAnalysis,
	"GetStockOrgPredict":          GroupStockAnalysis,
	"GetStockPredictSummary":      GroupStockAnalysis,
	"GetStockValuationPercentile": GroupStockAnalysis,
	"GetStockMarginTrading":       GroupStockAnalysis,
	"GetStockBlockTrade":          GroupStockAnalysis,
	"GetRzrqRank":                 GroupStockAnalysis,
	"GetRzrqTrend":                GroupStockAnalysis,
	"GetStockHolderTrend":         GroupStockAnalysis,
	"GetStockBillboard":           GroupStockAnalysis,
	"GetStockOperationDeptTrade":  GroupStockAnalysis,
	"GetStockOrgBasicInfo":        GroupStockAnalysis,
	"ComparableCompanyAnalysis":   GroupStockAnalysis,
	"HotspotDiscovery":            GroupMarket,

	"IndustryResearch": GroupStockAnalysis,

	"TrackingReport": GroupStockAnalysis,

	"FinanceDataQuery": GroupStockAnalysis,

	"FinanceSearch": GroupNewsResearch,

	"SearchReport": GroupNewsResearch,

	"QueryInsResearch": GroupNewsResearch,

	"SearchNews": GroupNewsResearch,

	"SearchInvestor": GroupNewsResearch,

	"SearchAnnouncement": GroupNewsResearch,

	"FilterStocks":            GroupScreening,
	"SearchStockByIndicators": GroupScreening,

	"SelectAStock": GroupScreening,

	"SelectSector": GroupScreening,

	"SelectETF":             GroupScreening,
	"SelectFundManager":     GroupScreening,
	"SelectConvertibleBond": GroupScreening,
	"SelectFundCompany":     GroupScreening,
	"SelectFund":            GroupScreening,
	"SelectFuturesOption":   GroupScreening,
	"SelectHKStock":         GroupScreening,
	"SelectUSStock":         GroupScreening,
	"SearchBk":              GroupScreening,
	"SearchETF":             GroupScreening,
	"HotStrategyTable":      GroupScreening,
	"HotStockTable":         GroupScreening,

	"GetStockMoneyData":        GroupMoneyFlow,
	"GetMutualTop10Deal":       GroupMoneyFlow,
	"GetStockHistoryMoneyData": GroupMoneyFlow,
	"GetIndustryMoneyRank":     GroupMoneyFlow,
	"GetMACCapitalFlow":        GroupMoneyFlow,

	"QueryStockNewsTool":          GroupNewsResearch,
	"GetNewsListData":             GroupNewsResearch,
	"GetStockResearchReport":      GroupNewsResearch,
	"GetIndustryResearchReport":   GroupNewsResearch,
	"GetSecuritiesCompanyOpinion": GroupNewsResearch,
	"StockNotice":                 GroupNewsResearch,
	"GetStockNotice":              GroupNewsResearch,
	"InteractiveAnswer":           GroupNewsResearch,
	"GetInvestCalendar":           GroupNewsResearch,
	"GetClsCalendar":              GroupNewsResearch,
	"GetLongTigerList":            GroupNewsResearch,
	"GetHotStockList":             GroupNewsResearch,
	"GetHotEventList":             GroupNewsResearch,
	"GetUplimitLadder":            GroupNewsResearch,
	"GetUplimitHotPlates":         GroupNewsResearch,
	"GetUplimitHotStocks":         GroupNewsResearch,
	"GetUplimitExplodedStocks":    GroupNewsResearch,
	"GetUplimitPlateStocks":       GroupNewsResearch,

	"GetWallstreetcnLives":      GroupNewsResearch,
	"GetWallstreetcnMarketReal": GroupMarket,
	"GetWallstreetcnKline":      GroupMarket,
	"GetWallstreetcnCalendar":   GroupNewsResearch,

	"AiRecommendStocks":    GroupAIAnalysis,
	"GetAIAnalysisHistory": GroupAIAnalysis,
	"GetAIAnalysisDetail":  GroupAIAnalysis,
	"GetAIAnalysisContent": GroupAIAnalysis,

	"SetTradingPrice":                GroupOperations,
	"FollowStock":                    GroupOperations,
	"AddDailyOperationPlan":          GroupOperations,
	"GetDailyOperationPlanList":      GroupOperations,
	"UpdateDailyOperationPlan":       GroupOperations,
	"UpdateDailyOperationPlanStatus": GroupOperations,
	"SendDingDingMessage":            GroupOperations,
	"SendToDingDing":                 GroupOperations,
	"SendFeishuMessage":              GroupOperations,
	"SendToFeishu":                   GroupOperations,
	"SearchFund":                     GroupOperations,
	"GetFundInfo":                    GroupOperations,
	"GetEconomicData":                GroupOperations,
	"GetTradingRecordList":           GroupOperations,
	"GetTradingRecordStatistics":     GroupOperations,

	// 分组与概念标签管理（16 个工具）
	"GetStockGroups":          GroupOperations,
	"CreateStockGroup":        GroupOperations,
	"UpdateStockGroup":        GroupOperations,
	"DeleteStockGroup":        GroupOperations,
	"AddStockToGroup":         GroupOperations,
	"RemoveStockFromGroup":    GroupOperations,
	"BatchMoveStocksToGroup":  GroupOperations,
	"GetStockConcepts":        GroupOperations,
	"CreateStockConcept":      GroupOperations,
	"UpdateStockConcept":      GroupOperations,
	"DeleteStockConcept":      GroupOperations,
	"AddStockToConcept":       GroupOperations,
	"RemoveStockFromConcept":  GroupOperations,
	"BatchAddStocksToConcept": GroupOperations,
	"MergeStockConcepts":      GroupOperations,
	"ReorganizeStockGroups":   GroupOperations,

	"ListPromptTemplates":  GroupBase,
	"GetPromptTemplate":    GroupBase,
	"SavePromptTemplate":   GroupBase,
	"DeletePromptTemplate": GroupBase,
}

type groupKeywords struct {
	group    ToolGroup
	keywords []string
}

var groupKeywordsList = []groupKeywords{
	{GroupStockAnalysis, []string{
		"股票", "股价", "行情", "K线", "k线", "日K", "周K", "月K", "分钟线",
		"分时", "实时", "价格", "涨跌", "成交量", "成交额",
		"集合竞价", "竞价", "开盘竞价", "收盘竞价", "竞价明细",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS",
		"毛利率", "净利率", "现金流", "负债率",
		"行业估值", "行业排名", "行业盈利",
		"股东", "持股", "融资融券", "融券", "融资", "杠杆", "两融", "走势",
		"股本结构", "股东户数", "实控人", "前十大股东",
		"概念", "板块归属", "所属概念",
		"业绩预告", "增发", "配股", "质押", "解禁", "调研", "监管函",
		"基本资料", "上市日期", "基金资料", "费率", "合约信息",
		"主营业务", "主要客户", "供应商", "参控股", "股权投资", "重大合同",
		"基金业绩", "基金持仓", "基金风险", "基金评级", "基金获奖",
		"业绩点评", "财报分析", "业绩报告", "营收分析", "利润分析", "季报", "年报", "中报",
		"行业研究", "行业报告", "产业分析", "行业深度", "行业趋势",
		"跟踪报告", "个股跟踪", "行业跟踪", "动态跟踪", "最新动态跟踪",
		"查数", "金融数据查询", "数据查询", "指标查询", "估值数据", "行情数据查询",
		"诊断", "评估", "估值",
		"技术面", "基本面", "MACD", "KDJ", "RSI", "布林", "BOLL",
		"均线", "MA5", "MA10", "MA20", "MA60", "MA120",
		"前复权", "后复权", "复权",
		"可比公司", "对标公司", "同行对比", "行业对标",
		"机构预测", "券商预测", "目标价", "一致性预期",
		"分析一下", "分析",
	}},
	{GroupMarket, []string{
		"大盘", "市场", "指数", "行情", "涨跌分布", "涨停", "跌停",
		"上涨家数", "下跌家数", "申购", "新股",
		"异动", "火箭发射", "快速反弹", "大笔买入", "封涨停",
		"加速下跌", "高台跳水", "大笔卖出", "封跌停",
		"全球指数", "道琼斯", "纳斯达克", "标普", "恒生", "日经",
		"异动统计", "异动趋势", "异动排行", "异动排名", "异动次数",
		"利好", "利空", "异动类型", "异动分布",
		"问财", "同花顺", "行情查询", "行情数据",
		"热点", "题材", "市场热点", "热点发现", "热点板块",
		"指数行情", "指数点位", "沪深300", "创业板指", "中证500",
		"宏观", "GDP", "CPI", "PPI", "PMI", "社融", "M2", "LPR",
		"期货", "期权", "波动率", "持仓", "行权",
		"北向资金", "南向资金", "沪深港通", "沪股通", "深股通", "港股通", "AH溢价",
	}},
	{GroupScreening, []string{
		"筛选", "选股", "过滤", "条件选股", "指标选股",
		"智能选股",
		"选板块", "板块排行", "板块筛选",
		"选ETF", "ETF筛选",
		"选基金经理", "基金经理筛选", "基金经理排名",
		"选可转债", "可转债筛选", "可转债溢价率",
		"选基金公司", "基金公司筛选", "基金公司排名",
		"选基金", "基金筛选", "基金排名",
		"选期货期权", "期货筛选", "期权筛选",
		"选港股", "港股筛选", "港股排行",
		"选美股", "美股筛选", "美股排行",
		"形态选股", "MACD金叉", "KDJ金叉", "放量突破",
		"连涨", "连跌", "多头排列", "空头排列",
		"板块", "概念", "ETF",
		"热门策略", "热门股票", "策略",
	}},
	{GroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出",
		"北向", "南向", "沪股通", "深股通", "港股通",
		"主力", "机构", "外资",
		"行业资金", "板块资金",
	}},
	{GroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "研报", "研究报告",
		"最新动态", "政策动态", "行业趋势",
		"券商", "机构观点", "分析师", "评级",
		"投资评级", "目标价", "行业分析", "深度分析",
		"ESG", "信用评级", "主体评级", "基金评级", "券商金股", "业绩预测",
		"互动", "问答", "投资者互动",
		"投资者关系", "业绩说明会", "路演", "投资者调研", "分析师会议", "投关活动",
		"公告搜索", "分红公告", "回购公告", "重组公告", "定期报告",
		"资讯搜索", "金融资讯", "舆情监控", "热点捕捉", "研报速览", "公告精读",
		"日历", "财报日", "股东大会", "IPO",
		"龙虎榜", "营业部",
		"涨停", "连板", "梯队", "涨停复盘", "涨停板块", "炸板", "封板",
		"热门板块", "板块热度", "板块轮动", "主线题材", "接力板块",
		"个股热度", "热门个股", "人气股", "关注度",
		"封板失败", "开板", "破板",
		"热门话题", "热点事件", "雪球",
		"华尔街见闻", "见闻快讯", "全球快讯", "7x24",
		"美元指数", "非农", "美联储", "降息", "加息", "通胀数据",
		"财经日历", "经济数据公布", "重要数据",
	}},
	{GroupAIAnalysis, []string{
		"AI分析", "AI推荐", "AI 分析", "AI 推荐",
		"历史分析", "分析报告", "分析记录",
		"推荐股票", "买入评级", "增持评级", "减持评级",
		"金融问答", "智能问答", "FinancialQA",
		"GetAIAnalysis", "AiRecommendStocks",
	}},
	{GroupOperations, []string{
		"预警", "价位", "开仓", "止盈价", "止损价", "成本价",
		"钉钉", "飞书", "QQ", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
		"GDP", "CPI", "PPI", "PMI", "宏观经济",
		"关注", "自选", "加自选", "加入分组", "设置概念", "概念标签", "归类", "持仓", "持仓量",
		"交易日志", "交易记录", "盈亏",
		"操作计划", "每日计划", "操作方案", "明日操作", "明天操作", "盘中预警",
	}},
}

func ClassifyQuestion(question string) map[ToolGroup]bool {
	matched := map[ToolGroup]bool{
		GroupBase: true,
	}

	lowerQ := strings.ToLower(question)

	for _, gk := range groupKeywordsList {
		for _, kw := range gk.keywords {
			if strings.Contains(lowerQ, strings.ToLower(kw)) {
				matched[gk.group] = true
				break
			}
		}
	}

	if len(matched) <= 1 {
		for _, g := range DefaultToolGroupsForIntent(DetectQuestionIntent(question)) {
			matched[g] = true
		}
	}

	return matched
}

func FilterToolsByGroups(allTools []tool.BaseTool, groups map[ToolGroup]bool) []tool.BaseTool {
	var filtered []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(nil)
		if err != nil {
			filtered = append(filtered, t)
			continue
		}
		group, exists := toolGroupMap[info.Name]
		if !exists || groups[group] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
