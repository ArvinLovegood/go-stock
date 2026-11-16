package data

import (
	"fmt"
	"go-stock/backend/logger"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("GetBloombergMarketNews", handleBloombergMarketNews)
}

func handleBloombergMarketNews(o *OpenAi, args string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetBloombergMarketNews", args)
	limit := 0
	value := gjson.Get(args, "limit")
	if value.Exists() {
		limit = int(value.Int())
		if limit == 0 {
			appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, args, "新闻条数必须在 1 至 20 之间")
			return nil
		}
	}
	result, err := NewBloombergNewsAPI().MarketNewsForAI(limit)
	if err != nil {
		logger.SugaredLogger.Warnf("获取彭博市场新闻失败: %v", err)
		result = fmt.Sprintf("获取彭博市场新闻失败: %v", err)
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, args, result)
	return nil
}
