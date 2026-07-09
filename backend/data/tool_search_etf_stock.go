package data

import (
	"encoding/json"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("SearchETF", handleSearchETF)
	registerToolHandler("SearchStockByIndicators", handleSearchStockByIndicators)
	registerToolHandler("AiRecommendStocks", handleAiRecommendStocks)
	registerToolHandler("CreateAiRecommendStocks", handleCreateAiRecommendStocks)
	registerToolHandler("BatchCreateAiRecommendStocks", handleBatchCreateAiRecommendStocks)
}

// handleSearchETF 处理 SearchETF 工具调用
func handleSearchETF(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	words := gjson.Get(funcArguments, "words").String()
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：SearchETF，\n参数：" + words + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	res := NewSearchStockApi(words).SearchETF(random.RandInt(50, 120))
	content := thsResultToMarkdown(res, "工具筛选出的相关ETF数据")
	//logger.SugaredLogger.Infof("SearchETF:words:%s  --> \n%s", words, content)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)
	return nil
}

// handleSearchStockByIndicators 处理 SearchStockByIndicators 工具调用
func handleSearchStockByIndicators(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	words := gjson.Get(funcArguments, "words").String()

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：SearchStockByIndicators，\n参数：" + words + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	res := NewSearchStockApi(words).SearchStock(random.RandInt(50, 120))
	content := thsResultToMarkdown(res, "工具筛选出的相关股票数据")
	//logger.SugaredLogger.Infof("SearchStockByIndicators:words:%s  --> \n%s", words, content)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)
	return nil
}

// handleAiRecommendStocks 处理 AiRecommendStocks 工具调用
func handleAiRecommendStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	page := gjson.Get(funcArguments, "page").String()
	pageSize := gjson.Get(funcArguments, "pageSize").String()
	keyWord := gjson.Get(funcArguments, "keyWord").String()
	startDate := gjson.Get(funcArguments, "startDate").String()
	endDate := gjson.Get(funcArguments, "endDate").String()

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：AiRecommendStocks，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	pageNo, convErr := convertor.ToInt(page)
	if convErr != nil {
		pageNo = 1
	}
	pageSizeNum, convErr := convertor.ToInt(pageSize)
	if convErr != nil {
		pageSizeNum = 50
	}

	pageData, svcErr := NewAiRecommendStocksService().GetAiRecommendStocksList(&models.AiRecommendStocksQuery{
		StartDate: startDate,
		EndDate:   endDate,
		Page:      int(pageNo),
		PageSize:  int(pageSizeNum),
		StockCode: keyWord,
		StockName: keyWord,
		BkName:    keyWord,
	})
	if svcErr != nil {
		pageData = &models.AiRecommendStocksPageData{}
	}

	var dataExport []models.AiRecommendStocksMdExport
	for _, v := range pageData.List {
		dataExport = append(dataExport, v.ToMdExportStruct())
	}
	content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)
	return nil
}

// handleCreateAiRecommendStocks 处理 CreateAiRecommendStocks 工具调用
func handleCreateAiRecommendStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：CreateAiRecommendStocks，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	recommend := models.AiRecommendStocks{}
	if err := json.Unmarshal([]byte(funcArguments), &recommend); err != nil {
		//logger.SugaredLogger.Infof("CreateAiRecommendStocks error : %s", err.Error())
		return err
	}

	// 用实际使用的模型名覆盖 AI 自填值，并关联系统/用户提示词
	recommend.ModelName = ctx.Model
	recommend.SystemPrompt = ctx.SystemPrompt
	recommend.UserPrompt = ctx.Question

	svcErr := NewAiRecommendStocksService().CreateAiRecommendStocks(&recommend)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		func() string {
			if svcErr != nil {
				//logger.SugaredLogger.Infof("CreateAiRecommendStocks error : %s", svcErr.Error())
				ctx.Ch <- map[string]any{
					"code":     0,
					"question": ctx.Question,
					"content":  "保存股票推荐失败:" + svcErr.Error(),
				}
				return "保存股票推荐失败:" + svcErr.Error()
			}
			return "保存股票推荐成功"
		}(),
	)

	return svcErr
}

// handleBatchCreateAiRecommendStocks 处理 BatchCreateAiRecommendStocks 工具调用
func handleBatchCreateAiRecommendStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：BatchCreateAiRecommendStocks，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	stocks := gjson.Get(funcArguments, "stocks").String()
	var recommends []*models.AiRecommendStocks
	if err := json.Unmarshal([]byte(stocks), &recommends); err != nil {
		//logger.SugaredLogger.Infof("BatchCreateAiRecommendStocks error : %s", err.Error())
		return err
	}

	// 用实际使用的模型名覆盖 AI 自填值，并关联系统/用户提示词
	for _, r := range recommends {
		if r == nil {
			continue
		}
		r.ModelName = ctx.Model
		r.SystemPrompt = ctx.SystemPrompt
		r.UserPrompt = ctx.Question
	}

	svcErr := NewAiRecommendStocksService().BatchCreateAiRecommendStocks(recommends)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		func() string {
			if svcErr != nil {
				//logger.SugaredLogger.Infof("BatchCreateAiRecommendStocks error : %s", svcErr.Error())
				ctx.Ch <- map[string]any{
					"code":     0,
					"question": ctx.Question,
					"content":  "批量保存股票推荐失败:" + svcErr.Error(),
				}
				return "批量保存股票推荐失败:" + svcErr.Error()
			}
			return "批量保存股票推荐成功"
		}(),
	)

	return svcErr
}
