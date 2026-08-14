package tools

import (
	"fmt"
	"strings"

	"github.com/coocood/freecache"
)

// toolResultCache 工具结果内存缓存。
//
// 用于缓存低频变化的工具返回（F10 基础资料、季度财务、机构预测等），
// 减少对东方财富 F10 接口的重复请求，降低 Agent 多步推理时的网络时延。
// 行情类、新闻类等高频变化数据走短 TTL（10s）或不缓存。
//
// 缓存 key = 工具名 + "|" + 规范化参数 JSON，按工具名分别设置 TTL。
var toolResultCache = freecache.NewCache(8 * 1024 * 1024) // 8MB

// cacheTTLByTool 各工具的精确缓存 TTL（秒），覆盖组级默认。
// 0 表示不缓存（写操作、AI 生成、实时时间等）。
// 未在此 map 中的工具，按其所属工具组的默认 TTL 处理（见 groupDefaultTTL）。
var cacheTTLByTool = map[string]int{
	// ==== 低频变化（F10 公司资料/季度财务/机构预测/户均持股/财务信息）——1 小时 ====
	"GetStockOrgBasicInfo":      3600,
	"GetStockQtrMainFinance":    3600,
	"GetStockOrgPredict":        3600,
	"GetStockPredictSummary":    3600,
	"GetStockHolderTrend":       3600,
	"GetHKStockLatestFinance":   3600, // 港股财报季度更新
	"GetStockFinancialInfo":     3600, // 财务信息
	"GetStockHolderNum":         3600, // 股东户数
	"GetStockConceptInfo":       3600, // 概念信息
	"StockEarningsReview":       3600, // 业绩点评
	"ComparableCompanyAnalysis": 3600, // 可比公司分析
	"IndustryResearch":          3600, // 行业研究
	"TrackingReport":            3600, // 跟踪报告
	"FinanceDataQuery":          3600, // 财务数据查询

	// ==== 每日变化（最新财务/估值百分位/融资融券/大宗交易/龙虎榜/营业部）——10 秒 ====
	"GetStockLatestFinance":       10,
	"GetStockValuationPercentile": 10,
	"GetStockMarginTrading":       10,
	"GetStockBlockTrade":          10,
	"GetStockBillboard":           10,
	"GetStockOperationDeptTrade":  10,

	// ==== 节假日/交易日——24 小时（年度更新）====
	"GetHolidayInfo":    86400,
	"GetHolidayYear":    86400,
	"GetHolidayBatch":   86400,
	"IsTradingDay":      86400,
	"GetNextTradingDay": 86400,

	// ==== 研报/日历——5 分钟 ~ 1 小时 ====
	"GetStockResearchReport":      300,
	"GetIndustryResearchReport":   300,
	"GetSecuritiesCompanyOpinion": 300,
	"InteractiveAnswer":           300,
	"GetInvestCalendar":           3600,
	"GetClsCalendar":              3600,

	// ==== GroupOperations 中的查询类工具（组级默认 0 不缓存，这里覆盖为可缓存）====
	"GetTradingRecordList":       60,
	"GetTradingRecordStatistics": 60,
	"GetDailyOperationPlanList":  60,
	"SearchFund":                 300,
	"GetFundInfo":                300,
	"GetEconomicData":            300,
	"ListPromptTemplates":        300,
	"GetPromptTemplate":          300,

	// ==== 不缓存（写操作/AI 生成/实时时间/分时数据等）====
	"GetCurrentTime":               0,
	"CleanupStockCodes":            0,
	"CreateAiRecommendStocks":      0,
	"BatchCreateAiRecommendStocks": 0,
	"SavePromptTemplate":           0,
	"DeletePromptTemplate":         0,
	"GetStockMinuteData":           0, // 分时数据实时性要求高
	"AiRecommendStocks":            0, // AI 每次可能不同
	"FinancialQA":                  0, // AI 问答每次可能不同
}

// groupDefaultTTL 按工具组的默认缓存 TTL（秒）。
// 未在 cacheTTLByTool 中显式配置的工具，按其所属组返回默认 TTL。
// 这样无需逐个工具配置，即可覆盖全部 110+ 工具。
var groupDefaultTTL = map[ToolGroup]int{
	GroupBase:          60,  // 基础工具（时间/假日/关注列表等）——60s
	GroupStockAnalysis: 30,  // 股票分析（行情/K线/财务等）——30s
	GroupMarket:        10,  // 市场行情（指数/涨跌/统计等）——10s
	GroupScreening:     300, // 选股筛选（选股/搜索/热门表格等）——5min
	GroupMoneyFlow:     10,  // 资金流（个股/行业/主力资金等）——10s
	GroupNewsResearch:  60,  // 新闻研报（新闻/公告/研报/日历等）——60s
	GroupAIAnalysis:    0,   // AI 分析（每次可能不同）——不缓存
	GroupOperations:    0,   // 操作类（写操作/消息发送/分组管理等）——不缓存
}

// cacheTTLForTool 返回工具的缓存 TTL。
// 查找顺序：1. cacheTTLByTool 精确配置 → 2. groupDefaultTTL 组级默认 → 3. 返回 0（不缓存）。
func cacheTTLForTool(name string) int {
	// 1. 精确配置优先（覆盖组级默认）
	if ttl, ok := cacheTTLByTool[name]; ok {
		return ttl
	}
	// 2. 查工具组默认 TTL
	if group, ok := toolGroupMap[name]; ok {
		if ttl, ok := groupDefaultTTL[group]; ok {
			return ttl
		}
	}
	// 3. 未知工具不缓存
	return 0
}

// buildCacheKey 构造工具结果缓存 key：工具名 + 规范化参数。
// 规范化：去除参数 JSON 中首尾空白与多余空格，避免相同语义参数产生不同 key。
func buildCacheKey(toolName, argsJSON string) string {
	args := strings.TrimSpace(argsJSON)
	if args == "" {
		return toolName + "|"
	}
	return fmt.Sprintf("%s|%s", toolName, args)
}

// getCachedToolResult 查询缓存，命中返回 (结果, true)，否则返回 ("", false)。
func getCachedToolResult(toolName, argsJSON string) (string, bool) {
	ttl := cacheTTLForTool(toolName)
	if ttl <= 0 {
		return "", false
	}
	key := buildCacheKey(toolName, argsJSON)
	data, err := toolResultCache.Get([]byte(key))
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// setCachedToolResult 写入缓存（按工具 TTL）。
// result 为空或为错误消息时不缓存，避免缓存失败结果。
func setCachedToolResult(toolName, argsJSON, result string) {
	ttl := cacheTTLForTool(toolName)
	if ttl <= 0 || strings.TrimSpace(result) == "" {
		return
	}
	// 错误/空结果不缓存
	if strings.Contains(result, "工具调用出错") || strings.Contains(result, "工具调用异常") {
		return
	}
	key := buildCacheKey(toolName, argsJSON)
	// freecache 入参超过 1/1024 总容量会失败，这里 result 不会超过 8MB/1024=8KB
	// 单条 F10 返回最大约 30KB，为避免 entry 过大失败，超过 64KB 不缓存
	if len(result) > 64*1024 {
		return
	}
	_ = toolResultCache.Set([]byte(key), []byte(result), ttl)
}

// resetToolResultCache 清空工具结果缓存（仅用于测试或显式重置）。
func resetToolResultCache() {
	toolResultCache.Clear()
}
