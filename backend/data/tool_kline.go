package data

import (
	"encoding/json"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("GetStockKLine", handleGetStockKLine)
}

func buildStockKLineSection(o *OpenAi, stockCode string, toIntDay int64) string {
	if !strutil.HasPrefixAny(stockCode, []string{"sz", "sh", "hk", "us", "gb_"}) && !IsBinanceFuturesCode(stockCode) && !IsBitgetFuturesCode(stockCode) {
		return stockCode + "：无数据，可能股票代码错误。（A股：sh,sz开头;港股hk开头,美股：us开头;币安永续：bn:开头，如bn:btcusdt;美股永续：bt:开头，如bt:aaplusdt）"
	}
	var K *[]KLineData
	if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
		K = NewStockDataApi().GetKLineData(stockCode, "240", o.KDays)
	}
	if strutil.HasPrefixAny(stockCode, []string{"hk", "us", "gb_"}) {
		K = NewStockDataApi().GetHK_KLineData(stockCode, "day", o.KDays)
	}
	var sourceLabel string
	if K == nil || len(*K) == 0 {
		fallbackResult := FetchKLineWithFallback(stockCode, "", "101", int(toIntDay), "")
		if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
			K = fallbackResult.Data
			sourceLabel = fallbackResult.Source
		}
	}
	if K == nil || len(*K) == 0 {
		return stockCode + "：未获取到 K 线数据。"
	}
	Kmap := &[]map[string]any{}
	for _, kline := range *K {
		mapk := make(map[string]any, 6)
		mapk["日期"] = kline.Day
		mapk["开盘价"] = kline.Open
		mapk["最高价"] = kline.High
		mapk["最低价"] = kline.Low
		mapk["收盘价"] = kline.Close
		Volume, _ := convertor.ToFloat(kline.Volume)
		mapk["成交量(万手)"] = Volume / 10000.00 / 100.00
		*Kmap = append(*Kmap, mapk)
	}
	jsonData, _ := json.Marshal(Kmap)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	sourceInfo := ""
	if sourceLabel != "" {
		sourceInfo = "（数据源：" + sourceLabel + "）"
	}
	return "\r\n ### " + stockCode + convertor.ToString(toIntDay) + "日K线数据" + sourceInfo + "：\r\n" + markdownTable + "\r\n"
}

// handleGetStockKLine 处理 GetStockKLine 工具调用
func handleGetStockKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	days := gjson.Get(funcArguments, "days").String()
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetStockKLine，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	toIntDay, convErr := convertor.ToInt(days)
	if convErr != nil {
		toIntDay = 90
	}

	res := parallelStockToolSections(codes, func(stockCode string) string {
		return buildStockKLineSection(o, stockCode, int64(toIntDay))
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		res,
	)
	return nil
}
