package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("SendDingDingMessage", handleSendDingDingMessage)
	registerToolHandler("SendFeishuMessage", handleSendFeishuMessage)

	registerToolHandler("GetDailyDimensionStats", handleGetDailyDimensionStats)
	registerToolHandler("GetTypeStatsByDate", handleGetTypeStatsByDate)
	registerToolHandler("GetUplimitPlateStocks", handleGetUplimitPlateStocks)
	registerToolHandler("GetHolidayYear", handleGetHolidayYear)
	registerToolHandler("GetHolidayBatch", handleGetHolidayBatch)

	for _, name := range []string{
		"QueryBasicInfo",
		"QueryFinance",
		"QueryIndustry",
		"QueryFutures",
		"SelectETF",
		"QueryManagement",
		"QueryStockConnect",
		"SelectFundManager",
		"SelectConvertibleBond",
		"SelectFundCompany",
		"SelectFund",
		"SelectFuturesOption",
		"SelectHKStock",
		"SelectUSStock",
		"QueryFundFinance",
		"QueryBusinessData",
	} {
		registerToolHandler(name, handleAgentParityIwencaiQuery)
	}

	registerToolHandler("SearchAnnouncement", handleSearchAnnouncement)
	registerToolHandler("StockEarningsReview", handleStockEarningsReview)
	registerToolHandler("IndustryResearch", handleIndustryResearch)
	registerToolHandler("TrackingReport", handleTrackingReport)
	registerToolHandler("FinanceDataQuery", handleFinanceDataQuery)
}

func handleSendDingDingMessage(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleSendToDingDing(o, funcArguments, ctx)
}

func handleSendFeishuMessage(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	return handleSendToFeishu(o, funcArguments, ctx)
}

func handleAgentParityIwencaiQuery(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, ctx.FuncName, funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	page := int(gjson.Get(funcArguments, "page").Int())
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入查询语句")
		return nil
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	result := NewIwencaiAPI().QueryToMarkdown(query, page, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleSearchAnnouncement(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SearchAnnouncement", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入搜索关键词")
		return nil
	}
	result := NewIwencaiAPI().SearchAnnouncementToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleStockEarningsReview(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "StockEarningsReview", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	reportDate := gjson.Get(funcArguments, "reportDate").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入股票名称或代码")
		return nil
	}
	result := NewEmAPI().EarningsReviewToMarkdown(query, reportDate)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleIndustryResearch(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "IndustryResearch", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入行业关键词")
		return nil
	}
	result := NewEmAPI().IndustryResearchToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleTrackingReport(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "TrackingReport", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入股票名称/代码或行业关键词")
		return nil
	}
	result := NewEmAPI().TrackingReportToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleFinanceDataQuery(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "FinanceDataQuery", funcArguments)
	query := gjson.Get(funcArguments, "query").String()
	if query == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请输入查询内容")
		return nil
	}
	result := NewEmAPI().FinanceDataQueryToMarkdown(query)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleGetDailyDimensionStats(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetDailyDimensionStats", funcArguments)
	dimension := gjson.Get(funcArguments, "dimension").String()
	name := gjson.Get(funcArguments, "name").String()
	days := int(gjson.Get(funcArguments, "days").Int())
	if dimension == "" || name == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供dimension和name参数")
		return nil
	}
	if days <= 0 {
		days = 30
	}
	result, err := NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "查询失败: "+err.Error())
		return nil
	}
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetTypeStatsByDate(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTypeStatsByDate", funcArguments)
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供date参数")
		return nil
	}
	result, err := NewStockChangeHistoryService().GetTypeStatsByDate(date)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "查询失败: "+err.Error())
		return nil
	}
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func handleGetUplimitPlateStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetUplimitPlateStocks", funcArguments)
	plateName := gjson.Get(funcArguments, "plate_name").String()
	if plateName == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供板块名称参数 plate_name")
		return nil
	}
	date := gjson.Get(funcArguments, "date").String()
	if date == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		date = time.Now().In(loc).Format("2006-01-02")
	}
	dataMap, err := getUplimitDataMap(date)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, err.Error())
		return nil
	}
	result := renderUplimitPlateStocks(date, plateName, dataMap)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

func handleGetHolidayYear(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHolidayYear", funcArguments)
	year := gjson.Get(funcArguments, "year").String()
	if year == "" {
		year = time.Now().Format("2006")
	}
	apiURL := fmt.Sprintf("https://timor.tech/api/holiday/year/%s/", year)
	var result map[string]any
	resp, err := restyForHoliday().R().SetResult(&result).Get(apiURL)
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

func handleGetHolidayBatch(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHolidayBatch", funcArguments)
	datesStr := gjson.Get(funcArguments, "dates").String()
	if datesStr == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供要查询的日期列表")
		return nil
	}
	client := restyForHoliday()
	results := make(map[string]any)
	for _, date := range strings.Split(datesStr, ",") {
		date = strings.TrimSpace(date)
		if date == "" {
			continue
		}
		apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date)
		var result map[string]any
		resp, err := client.R().SetResult(&result).Get(apiURL)
		if err != nil {
			results[date] = map[string]string{"error": err.Error()}
			continue
		}
		if resp.StatusCode() != 200 {
			results[date] = map[string]any{"error": fmt.Sprintf("HTTP状态码: %d", resp.StatusCode())}
			continue
		}
		results[date] = result
	}
	jsonBytes, _ := json.Marshal(results)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

func getUplimitDataMap(date string) (map[string]any, error) {
	result := NewMarketNewsApi().GetUplimitHot(date, 20)
	if result == nil || result["code"] == nil {
		return nil, fmt.Errorf("获取涨停梯队数据失败")
	}
	code, _ := result["code"].(float64)
	if int(code) != 20000 {
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("获取涨停梯队数据失败: %s", msg)
	}
	dataMap, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("涨停梯队数据格式异常")
	}
	return dataMap, nil
}

func renderUplimitPlateStocks(date, plateName string, dataMap map[string]any) string {
	plateInfo, _ := dataMap["plate_info"].(map[string]any)
	plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
	stockInfo, _ := dataMap["stock_info"].(map[string]any)

	var targetCode string
	for pCode, pi := range plateInfo {
		if piMap, ok := pi.(map[string]any); ok {
			name, _ := piMap["name"].(string)
			if name == plateName {
				targetCode = pCode
				break
			}
		}
	}
	if targetCode == "" {
		plateArr, _ := dataMap["plate"].([]any)
		for _, p := range plateArr {
			if arr, ok := p.([]any); ok && len(arr) >= 2 {
				name, _ := arr[0].(string)
				pCode, _ := arr[1].(string)
				if name == plateName {
					targetCode = pCode
					break
				}
			}
		}
	}
	if targetCode == "" {
		return fmt.Sprintf("未找到板块【%s】，请检查板块名称是否正确", plateName)
	}
	stocks, ok := plateStocks[targetCode].([]any)
	if !ok || len(stocks) == 0 {
		return fmt.Sprintf("%s 板块【%s】暂无涨停股", date, plateName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 板块【%s】涨停股详情\n\n", date, plateName))
	sb.WriteString(fmt.Sprintf("共%d只涨停股\n\n", len(stocks)))
	sb.WriteString("| 代码 | 名称 | 连板 | 类型 | 描述 | 时间 | 封单比 | 收盘封单 | 成交额 | 市值 | 概念板块 |\n")
	sb.WriteString("|---|---|---:|---|---|---|---:|---:|---:|---:|---|\n")
	for _, s := range stocks {
		sm, _ := s.(map[string]any)
		sCode, _ := sm["stock_code"].(string)
		sName, _ := sm["stock_name"].(string)
		keepTimes, _ := sm["up_limit_keep_times"].(float64)
		upType, _ := sm["up_limit_type"].(string)
		upDesc, _ := sm["up_limit_desc"].(string)
		upTime, _ := sm["up_limit_time"].(string)
		platesStr := getUplimitStockPlates(stockInfo, sCode)
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %.2f%% | %.2f%% | %.2f亿 | %.2f亿 | %s |\n",
			sCode, sName, int(keepTimes), upType, upDesc, upTime,
			floatValue(sm["fd_max"]), floatValue(sm["fd_close"]), floatValue(sm["amount"]), floatValue(sm["market_c"]), platesStr))
	}
	return sb.String()
}

func floatValue(val any) float64 {
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func getUplimitStockPlates(stockInfo map[string]any, code string) string {
	if si, ok := stockInfo[code].(map[string]any); ok {
		if pa, ok := si["plates"].([]any); ok {
			plates := make([]string, 0, len(pa))
			for _, p := range pa {
				plates = append(plates, fmt.Sprintf("%v", p))
			}
			return strings.Join(plates, ",")
		}
	}
	return ""
}
