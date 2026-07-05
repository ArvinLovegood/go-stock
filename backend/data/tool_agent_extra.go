package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("FilterStocks", handleFilterStocks)
	registerToolHandler("QueryStockCodeInfo", handleQueryStockCodeInfo)
	registerToolHandler("QueryStockNews", handleQueryStockNews)
	registerToolHandler("GetStockInfo", handleGetStockInfoTool)
	registerToolHandler("GetStockMinuteData", handleGetStockMinuteData)
	registerToolHandler("GetStockChanges", handleGetStockChanges)
	registerToolHandler("GetStockChangeHistoryList", handleGetStockChangeHistoryList)
	registerToolHandler("GetFollowedStocks", handleGetFollowedStocks)
	registerToolHandler("GetAIAnalysisHistory", handleGetAIAnalysisHistory)
	registerToolHandler("GetAIAnalysisDetail", handleGetAIAnalysisDetail)
	registerToolHandler("GetAIAnalysisContent", handleGetAIAnalysisContent)
	registerToolHandler("GetHotStockList", handleGetHotStockList)
	registerToolHandler("GetHotEventList", handleGetHotEventList)
	registerToolHandler("GetIndustryMoneyRank", handleGetIndustryMoneyRank)
	registerToolHandler("GetLongTigerList", handleGetLongTigerList)
	registerToolHandler("GetEconomicData", handleGetEconomicData)
	registerToolHandler("GetInvestCalendar", handleGetInvestCalendar)
	registerToolHandler("GetStockNotice", handleGetStockNoticeTool)
	registerToolHandler("SearchFund", handleSearchFund)
	registerToolHandler("GetFundInfo", handleGetFundInfo)
	registerToolHandler("GetFundKLine", handleGetFundKLine)
	registerToolHandler("GetFundHistoryNetValue", handleGetFundHistoryNetValue)
	registerToolHandler("GetFundTop10Holdings", handleGetFundTop10Holdings)
	registerToolHandler("QueryIwencai", handleQueryIwencai)
	registerToolHandler("SelectAStock", handleSelectAStock)
	registerToolHandler("SelectSector", handleSelectSector)
	registerToolHandler("QueryMacro", handleQueryMacro)
	registerToolHandler("QueryZhishu", handleQueryZhishu)
	registerToolHandler("QueryEvent", handleQueryEvent)
	registerToolHandler("SearchNews", handleSearchNews)
	registerToolHandler("SearchInvestor", handleSearchInvestor)
	registerToolHandler("SearchReport", handleSearchReport)
	registerToolHandler("QueryInsResearch", handleQueryInsResearch)
	registerToolHandler("FinanceSearch", handleFinanceSearch)
	registerToolHandler("FinancialQA", handleFinancialQA)
	registerToolHandler("GetStockLatestFinance", handleGetStockLatestFinance)
	registerToolHandler("GetStockQtrMainFinance", handleGetStockQtrMainFinance)
	registerToolHandler("GetStockOrgPredict", handleGetStockOrgPredict)
	registerToolHandler("GetStockPredictSummary", handleGetStockPredictSummary)
	registerToolHandler("GetStockValuationPercentile", handleGetStockValuationPercentile)
	registerToolHandler("GetStockMarginTrading", handleGetStockMarginTrading)
	registerToolHandler("GetStockBlockTrade", handleGetStockBlockTrade)
	registerToolHandler("GetStockHolderTrend", handleGetStockHolderTrend)
	registerToolHandler("GetStockBillboard", handleGetStockBillboard)
	registerToolHandler("GetStockOperationDeptTrade", handleGetStockOperationDeptTrade)
	registerToolHandler("ComparableCompanyAnalysis", handleComparableCompanyAnalysis)
	registerToolHandler("HotspotDiscovery", handleHotspotDiscovery)
	registerToolHandler("GetUplimitLadder", handleGetUplimitLadder)
	registerToolHandler("GetUplimitHotPlates", handleGetUplimitHotPlates)
	registerToolHandler("GetUplimitHotStocks", handleGetUplimitHotStocks)
	registerToolHandler("GetUplimitExplodedStocks", handleGetUplimitExplodedStocks)
	registerToolHandler("GetDailyChangeStats", handleGetDailyChangeStats)
	registerToolHandler("GetChangeRank", handleGetChangeRank)
	registerToolHandler("GetHolidayInfo", handleGetHolidayInfo)
	registerToolHandler("IsTradingDay", handleIsTradingDay)
	registerToolHandler("GetNextTradingDay", handleGetNextTradingDay)
}

func sendToolCallLog(ctx *ToolContext, toolName, args string) {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": fmt.Sprintf("\r\n```\r\n🔧 调用工具：%s\n参数：%s\r\n```\r\n", toolName, args),
		"time":              time.Now().Format(time.DateTime),
	}
}

func handleFilterStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "FilterStocks", funcArguments)
	page := gjson.Get(funcArguments, "page").Int()
	pageSize := gjson.Get(funcArguments, "pageSize").Int()
	keyword := gjson.Get(funcArguments, "keyword").String()
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	indicators := models.TechnicalIndicators{}
	indicators.MACDGOLDENFORK = gjson.Get(funcArguments, "macdGoldenFork").Bool()
	indicators.KDJGOLDENFORK = gjson.Get(funcArguments, "kdjGoldenFork").Bool()
	indicators.BREAKTHROUGH = gjson.Get(funcArguments, "breakThrough").Bool()
	indicators.LOWFUNDSINFLOW = gjson.Get(funcArguments, "lowFundsInflow").Bool()
	indicators.HIGHFUNDSOUTFLOW = gjson.Get(funcArguments, "highFundsOutflow").Bool()
	indicators.BREAKUPMA5DAYS = gjson.Get(funcArguments, "breakUpMa5Days").Bool()
	indicators.LONGAVGARRAY = gjson.Get(funcArguments, "longAvgArray").Bool()
	indicators.SHORTAVGARRAY = gjson.Get(funcArguments, "shortAvgArray").Bool()
	indicators.UPPERLARGEVOLUME = gjson.Get(funcArguments, "upperLargeVolume").Bool()
	indicators.DOWNNARROWVOLUME = gjson.Get(funcArguments, "downNarrowVolume").Bool()
	indicators.MORNINGSTAR = gjson.Get(funcArguments, "morningStar").Bool()
	indicators.EVENINGSTAR = gjson.Get(funcArguments, "eveningStar").Bool()
	indicators.UPNDAY = int(gjson.Get(funcArguments, "upNday").Int())
	indicators.DOWNNDAY = int(gjson.Get(funcArguments, "downNday").Int())
	res := NewStockDataApi().GetAllStocks(int(page), int(pageSize), keyword, indicators)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleQueryStockCodeInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryStockCodeInfo", funcArguments)
	searchWord := gjson.Get(funcArguments, "searchWord").String()
	result := NewStockDataApi().GetStockList(searchWord)
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleQueryStockNews(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryStockNews", funcArguments)
	searchWords := gjson.Get(funcArguments, "searchWords").String()
	result := NewMarketNewsApi().CailianpressWeb(searchWords)
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetStockInfoTool(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockInfo", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		codes = strings.Split(stockCode, ",")
	}
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 stockCode 不能为空")
		return nil
	}
	md := parallelStockToolSections(codes, func(code string) string {
		res, _ := NewStockDataApi().GetStockCodeRealTimeData(code)
		jsonBytes, _ := json.Marshal(res)
		return string(jsonBytes)
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetStockMinuteData(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockMinuteData", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	res, _ := NewStockDataApi().GetStockMinutePriceData(stockCode)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetStockChanges(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockChanges", funcArguments)
	changeTypesStr := gjson.Get(funcArguments, "changeTypes").String()
	pageSize := gjson.Get(funcArguments, "pageSize").Int()
	if pageSize <= 0 {
		pageSize = 20
	}
	var changeTypes []int
	typeMap := map[string]int{
		"火箭发射": 8201, "快速反弹": 8202, "大笔买入": 8203, "封涨停板": 8204,
		"加速下跌": 8205, "高台跳水": 8206, "大笔卖出": 8207, "封跌停板": 8208,
	}
	if changeTypesStr != "" {
		for _, t := range strings.Split(changeTypesStr, ",") {
			t = strings.TrimSpace(t)
			if code, ok := typeMap[t]; ok {
				changeTypes = append(changeTypes, code)
			} else if n, err := strconv.Atoi(t); err == nil {
				changeTypes = append(changeTypes, n)
			}
		}
	}
	res := NewStockChangesApi().GetStockChangesReadable(changeTypes, 0, int(pageSize))
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetStockChangeHistoryList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockChangeHistoryList", funcArguments)
	res, err := NewStockChangeHistoryService().GetHistoryList(models.StockChangeHistoryQuery{
		StockCode:  gjson.Get(funcArguments, "stockCode").String(),
		ChangeType: int(gjson.Get(funcArguments, "changeType").Int()),
		StartDate:  gjson.Get(funcArguments, "startDate").String(),
		EndDate:    gjson.Get(funcArguments, "endDate").String(),
		Page:       int(gjson.Get(funcArguments, "page").Int()),
		PageSize:   int(gjson.Get(funcArguments, "pageSize").Int()),
	})
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "查询失败: "+err.Error())
		return nil
	}
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetFollowedStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetFollowedStocks", funcArguments)
	groupId := gjson.Get(funcArguments, "groupId").Int()
	if groupId > 0 {
		groupStocks := NewStockGroupApi(db.Dao).GetGroupStockByGroupId(int(groupId))
		var results []string
		for _, gs := range groupStocks {
			stocks := NewStockDataApi().GetFollowedStockByStockCode(gs.StockCode)
			jsonBytes, _ := json.Marshal(stocks)
			results = append(results, string(jsonBytes))
		}
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, strings.Join(results, "\n"))
	} else {
		res := NewStockDataApi().GetFollowList(0)
		jsonBytes, _ := json.Marshal(res)
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	}
	return nil
}

func handleGetAIAnalysisHistory(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetAIAnalysisHistory", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	stockName := gjson.Get(funcArguments, "stockName").String()
	question := gjson.Get(funcArguments, "question").String()
	startDate := gjson.Get(funcArguments, "startDate").String()
	endDate := gjson.Get(funcArguments, "endDate").String()
	page := gjson.Get(funcArguments, "page").Int()
	pageSize := gjson.Get(funcArguments, "pageSize").Int()
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	res, _ := NewAIResponseResultService().GetAIResponseResultList(models.AIResponseResultQuery{
		StockCode: stockCode,
		StockName: stockName,
		Question:  question,
		StartDate: startDate,
		EndDate:   endDate,
		Page:      int(page),
		PageSize:  int(pageSize),
	})
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetAIAnalysisDetail(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetAIAnalysisDetail", funcArguments)
	id := gjson.Get(funcArguments, "id").Int()
	var result models.AIResponseResult
	db.Dao.First(&result, id)
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetAIAnalysisContent(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetAIAnalysisContent", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	var result models.AIResponseResult
	db.Dao.Where("stock_code = ?", stockCode).Order("created_at DESC").First(&result)
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetHotStockList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHotStockList", funcArguments)
	marketType := gjson.Get(funcArguments, "marketType").String()
	if marketType == "" {
		marketType = "10"
	}
	size := gjson.Get(funcArguments, "size").Int()
	if size <= 0 {
		size = 20
	}
	res := NewMarketNewsApi().XUEQIUHotStock(int(size), marketType)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetHotEventList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHotEventList", funcArguments)
	size := gjson.Get(funcArguments, "size").Int()
	if size <= 0 {
		size = 20
	}
	res := NewMarketNewsApi().HotEvent(int(size))
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetIndustryMoneyRank(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetIndustryMoneyRank", funcArguments)
	fenlei := gjson.Get(funcArguments, "fenlei").String()
	if fenlei == "" {
		fenlei = "1"
	}
	sort := gjson.Get(funcArguments, "sort").String()
	if sort == "" {
		sort = "netamount"
	}
	res := NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetLongTigerList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetLongTigerList", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	res := NewMarketNewsApi().LongTiger(date)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetEconomicData(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetEconomicData", funcArguments)
	dataType := gjson.Get(funcArguments, "dataType").String()
	api := NewMarketNewsApi()
	var result interface{}
	switch strings.ToLower(dataType) {
	case "gdp":
		result = api.GetGDP()
	case "cpi":
		result = api.GetCPI()
	case "ppi":
		result = api.GetPPI()
	case "pmi":
		result = api.GetPMI()
	default:
		result = map[string]string{"error": "未知数据类型: " + dataType}
	}
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetInvestCalendar(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetInvestCalendar", funcArguments)
	yearMonth := gjson.Get(funcArguments, "yearMonth").String()
	if yearMonth == "" {
		yearMonth = time.Now().Format("2006-01")
	}
	res := NewMarketNewsApi().InvestCalendar(yearMonth)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetStockNoticeTool(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockNotice", funcArguments)
	stockCodes := gjson.Get(funcArguments, "stockCodes").String()
	if stockCodes == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 stockCodes 不能为空")
		return nil
	}
	res := NewMarketNewsApi().StockNotice(stockCodes)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleSearchFund(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SearchFund", funcArguments)
	keyword := gjson.Get(funcArguments, "keyword").String()
	res := NewFundApi().GetFundList(keyword)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetFundInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetFundInfo", funcArguments)
	fundCode := gjson.Get(funcArguments, "fundCode").String()
	res, _ := NewFundApi().CrawlFundBasic(fundCode)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetFundKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetFundKLine", funcArguments)
	fundCode := gjson.Get(funcArguments, "fundCode").String()
	klt := gjson.Get(funcArguments, "klt").String()
	limit := gjson.Get(funcArguments, "limit").Int()
	if klt == "" {
		klt = "101"
	}
	if limit <= 0 {
		limit = 100
	}
	res := NewFundKLineApi().GetFundKLine(fundCode, klt, int(limit))
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetFundHistoryNetValue(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetFundHistoryNetValue", funcArguments)
	fundCode := gjson.Get(funcArguments, "fundCode").String()
	pageIndex := gjson.Get(funcArguments, "pageIndex").Int()
	pageSize := gjson.Get(funcArguments, "pageSize").Int()
	startDate := gjson.Get(funcArguments, "startDate").String()
	endDate := gjson.Get(funcArguments, "endDate").String()
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	res, err := NewFundApi().GetFundHistoryNetValue(fundCode, int(pageIndex), int(pageSize), startDate, endDate)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("获取基金历史净值失败: %v", err))
		return nil
	}
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetFundTop10Holdings(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetFundTop10Holdings", funcArguments)
	fundCode := gjson.Get(funcArguments, "fundCode").String()
	res, err := NewFundApi().GetFundTop10Holdings(fundCode)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("获取基金十大持仓股失败: %v", err))
		return nil
	}
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleQueryIwencai(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryIwencai", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleSelectAStock(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SelectAStock", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleSelectSector(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SelectSector", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleQueryMacro(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryMacro", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleQueryZhishu(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryZhishu", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleQueryEvent(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryEvent", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleSearchNews(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SearchNews", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入搜索关键词")
		return nil
	}
	md := NewIwencaiAPI().SearchNewsToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleSearchInvestor(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SearchInvestor", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入搜索关键词")
		return nil
	}
	md := NewIwencaiAPI().SearchInvestorToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleSearchReport(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SearchReport", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入搜索关键词")
		return nil
	}
	md := NewIwencaiAPI().SearchReportToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleQueryInsResearch(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "QueryInsResearch", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := gjson.Get(funcArguments, "page").Int()
	limit := gjson.Get(funcArguments, "limit").Int()
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	md := NewIwencaiAPI().QueryToMarkdown(query, int(page), int(limit))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleFinanceSearch(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "FinanceSearch", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	md := NewIwencaiAPI().QueryToMarkdown(query, 1, 10)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleFinancialQA(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "FinancialQA", funcArguments)
	question := gjson.Get(funcArguments, "question").String()
	md := NewIwencaiAPI().QueryToMarkdown(question, 1, 10)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleF10ToolCall(funcName string, funcArguments string, ctx *ToolContext, fn func(string) string) error {
	sendToolCallLog(ctx, funcName, funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	if stockCode == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入股票代码")
		return nil
	}
	md := fn(stockCode)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetStockLatestFinance(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockLatestFinance", funcArguments, ctx, NewStockDataApi().GetStockLatestFinanceToMarkdown)
}

func handleGetStockQtrMainFinance(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockQtrMainFinance", funcArguments, ctx, NewStockDataApi().GetStockQtrMainFinanceToMarkdown)
}

func handleGetStockOrgPredict(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockOrgPredict", funcArguments, ctx, NewStockDataApi().GetStockOrgPredictToMarkdown)
}

func handleGetStockPredictSummary(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockPredictSummary", funcArguments, ctx, NewStockDataApi().GetStockPredictSummaryToMarkdown)
}

func handleGetStockValuationPercentile(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockValuationPercentile", funcArguments, ctx, NewStockDataApi().GetStockValuationPercentileToMarkdown)
}

func handleGetStockMarginTrading(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockMarginTrading", funcArguments, ctx, NewStockDataApi().GetStockMarginTradingToMarkdown)
}

func handleGetStockBlockTrade(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockBlockTrade", funcArguments, ctx, NewStockDataApi().GetStockBlockTradeToMarkdown)
}

func handleGetStockHolderTrend(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockHolderTrend", funcArguments, ctx, NewStockDataApi().GetStockHolderTrendToMarkdown)
}

func handleGetStockBillboard(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockBillboard", funcArguments, ctx, NewStockDataApi().GetStockBillboardToMarkdown)
}

func handleGetStockOperationDeptTrade(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleF10ToolCall("GetStockOperationDeptTrade", funcArguments, ctx, NewStockDataApi().GetStockOperationDeptTradeToMarkdown)
}

func handleComparableCompanyAnalysis(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "ComparableCompanyAnalysis", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入公司名称或股票代码")
		return nil
	}
	md := NewEmAPI().ComparableCompanyAnalysisToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleHotspotDiscovery(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "HotspotDiscovery", funcArguments)
	question := gjson.Get(funcArguments, "question").String()
	if question == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入热点描述")
		return nil
	}
	md := NewEmAPI().HotspotDiscoveryToMarkdown(question)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetUplimitLadder(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetUplimitLadder", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	res := NewMarketNewsApi().GetUplimitHot(date, 20)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetUplimitHotPlates(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetUplimitHotPlates", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	res := NewMarketNewsApi().GetUplimitHot(date, 20)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetUplimitHotStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetUplimitHotStocks", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	res := NewMarketNewsApi().GetUplimitHot(date, 20)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetUplimitExplodedStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetUplimitExplodedStocks", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	res := NewMarketNewsApi().GetUplimitHot(date, 20)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetDailyChangeStats(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetDailyChangeStats", funcArguments)
	days := gjson.Get(funcArguments, "days").Int()
	if days <= 0 {
		days = 30
	}
	res, _ := NewStockChangeHistoryService().GetDailyChangeStats(int(days))
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetChangeRank(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetChangeRank", funcArguments)
	tradeDate := gjson.Get(funcArguments, "tradeDate").String()
	_ = tradeDate
	days := 1
	topN := 20
	res, _ := NewStockChangeHistoryService().GetChangeRank(days, topN)
	jsonBytes, _ := json.Marshal(res)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetHolidayInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHolidayInfo", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	client := restyForHoliday()
	apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date)
	var result map[string]interface{}
	resp, err := client.R().SetResult(&result).Get(apiURL)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "查询失败: "+err.Error())
		return nil
	}
	if resp.StatusCode() != 200 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("查询失败，HTTP状态码: %d", resp.StatusCode()))
		return nil
	}
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleIsTradingDay(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "IsTradingDay", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	client := restyForHoliday()
	apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date)
	var result map[string]interface{}
	resp, err := client.R().SetResult(&result).Get(apiURL)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "查询失败: "+err.Error())
		return nil
	}
	isHoliday := false
	if resp.StatusCode() == 200 && result != nil {
		if code, ok := result["code"].(float64); ok && code == 0 {
			if holiday, ok := result["holiday"].(map[string]interface{}); ok {
				if holiday["holiday"] == true {
					isHoliday = true
				}
			}
		}
	}
	isWeekend := false
	t, _ := time.Parse("2006-01-02", date)
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		isWeekend = true
	}
	isTradingDay := !isHoliday && !isWeekend
	resultStr := fmt.Sprintf("日期: %s, 是否交易日: %v (周末: %v, 节假日: %v)", date, isTradingDay, isWeekend, isHoliday)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, resultStr)
	return nil
}

func handleGetNextTradingDay(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetNextTradingDay", funcArguments)
	startDate := gjson.Get(funcArguments, "startDate").String()
	days := gjson.Get(funcArguments, "days").Int()
	if days <= 0 {
		days = 1
	}
	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	t, _ := time.Parse("2006-01-02", startDate)
	count := 0
	for count < int(days) {
		t = t.AddDate(0, 0, 1)
		weekday := t.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			continue
		}
		count++
	}
	result := fmt.Sprintf("从 %s 起第 %d 个交易日: %s", startDate, days, t.Format("2006-01-02"))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func restyForHoliday() *resty.Client {
	return resty.New()
}
