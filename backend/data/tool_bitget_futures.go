package data

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/tidwall/gjson"
)

// Bitget 美股永续合约的 AI 工具 handler。
// 复用 tool_kline.go / tool_binance_futures.go 的通用写法：
// gjson 解析参数 → ctx.Ch 推进度 → JSONToMarkdownTable 组装 → appendToolMessages。

func init() {
	registerToolHandler("GetBitgetFuturesMarket", handleGetBitgetFuturesMarket)
	registerToolHandler("GetBitgetFuturesKLine", handleGetBitgetFuturesKLine)
	registerToolHandler("GetBitgetFuturesDerivatives", handleGetBitgetFuturesDerivatives)
}

func bitgetToolProgress(ctx *ToolContext, funcArguments string, tip string) {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：" + ctx.FuncName + "，" + tip + "，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}
}

// bitgetItemToMarkdown 把一组 map 转成 markdown 表格。
func bitgetItemToMarkdown(items []map[string]any) (string, error) {
	jsonData, err := json.Marshal(&items)
	if err != nil {
		return "", err
	}
	return JSONToMarkdownTable(jsonData)
}

// bitgetResolveOrError 解析合约标识，失败时直接回写错误信息。ok=false 表示已处理完毕。
func bitgetResolveOrError(ctx *ToolContext, funcArguments string, symbol string) (string, bool) {
	if strings.TrimSpace(symbol) == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 symbol 不能为空，请传入合约标识（如 aapl、苹果、aapl/usdt、bt:aaplusdt）。")
		return "", false
	}
	resolved, ok := ResolveBitgetSymbol(symbol)
	if !ok {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			fmt.Sprintf("%s：未在 Bitget 美股永续合约中找到该品种，请确认标的是美股（如 aapl、tsla、nvda）。注意 Bitget 美股永续不含加密币与贵金属代币。", symbol))
		return "", false
	}
	return resolved, true
}

// ===== GetBitgetFuturesMarket =====

func handleGetBitgetFuturesMarket(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sortBy := strings.ToLower(strings.TrimSpace(gjson.Get(funcArguments, "sort").String()))
	if sortBy == "" {
		sortBy = "percent"
	}
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	symbols := strings.TrimSpace(gjson.Get(funcArguments, "symbols").String())

	bitgetToolProgress(ctx, funcArguments, "获取 Bitget 美股永续合约行情")

	api := NewBitgetFuturesApi()

	var rows []BitgetTicker
	if symbols != "" {
		for _, raw := range strings.Split(symbols, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if resolved, ok := ResolveBitgetSymbol(raw); ok {
				if t := api.GetTicker(resolved); t != nil {
					rows = append(rows, *t)
				}
			}
		}
	} else {
		rows = bitgetSortTickers(api.GetAllTickers(), sortBy, limit)
	}

	if len(rows) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"未获取到美股永续合约行情数据。若在国内网络环境，请检查设置中的「合约代理」配置。")
		return nil
	}

	items := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		sym := strings.ToUpper(t.Symbol)
		item := map[string]any{
			"合约":         BitgetSymbolName(sym),
			"标识":         sym,
			"最新价":        bitgetTrimNum(t.LastPr),
			"24h涨跌幅(%)":  convertor.ToString(round2(bitgetChangePercent(&t))),
			"24h成交额(万U)": convertor.ToString(round2(bitgetFloat(t.QuoteVolume) / 10000)),
			"当期资金费率(%)":  convertor.ToString(round4(bitgetFloat(t.FundingRate) * 100)),
		}
		items = append(items, item)
	}

	table, err := bitgetItemToMarkdown(items)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "行情数据格式化失败："+err.Error())
		return nil
	}

	title := "Bitget 美股永续合约行情"
	switch sortBy {
	case "amount":
		title += "（按24h成交额）"
	case "funding":
		title += "（按资金费率绝对值）"
	default:
		title += "（按24h涨跌幅）"
	}
	res := fmt.Sprintf("\r\n ### %s（共 %d 条）：\r\n%s\r\n", title, len(items), table)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

func bitgetSortTickers(list []BitgetTicker, sortBy string, limit int) []BitgetTicker {
	rows := make([]BitgetTicker, len(list))
	copy(rows, list)
	switch sortBy {
	case "amount":
		sort.SliceStable(rows, func(i, j int) bool {
			return bitgetFloat(rows[i].QuoteVolume) > bitgetFloat(rows[j].QuoteVolume)
		})
	case "funding":
		sort.SliceStable(rows, func(i, j int) bool {
			return math.Abs(bitgetFloat(rows[i].FundingRate)) > math.Abs(bitgetFloat(rows[j].FundingRate))
		})
	default:
		sort.SliceStable(rows, func(i, j int) bool {
			return bitgetChangePercent(&rows[i]) > bitgetChangePercent(&rows[j])
		})
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// ===== GetBitgetFuturesKLine =====

func handleGetBitgetFuturesKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	symbol := gjson.Get(funcArguments, "symbol").String()
	resolved, ok := bitgetResolveOrError(ctx, funcArguments, symbol)
	if !ok {
		return nil
	}
	kLineType := gjson.Get(funcArguments, "interval").String()
	if strings.TrimSpace(kLineType) == "" {
		kLineType = "101"
	}
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 90
	}
	if limit > 1000 {
		limit = 1000
	}

	bitgetToolProgress(ctx, funcArguments, "获取 Bitget 美股永续合约K线")

	kType := normalizeKLineType(kLineType)
	interval := bitgetIntervalFromKlt(kType)
	data := NewBitgetFuturesApi().GetKLine(resolved, kType, limit, "")
	if data == nil || len(*data) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			BitgetSymbolName(resolved)+"：未获取到 K 线数据。若在国内网络环境，请检查设置中的「合约代理」配置。")
		return nil
	}

	items := make([]map[string]any, 0, len(*data))
	for _, kline := range *data {
		items = append(items, map[string]any{
			"时间":     kline.Day,
			"开盘价":    kline.Open,
			"最高价":    kline.High,
			"最低价":    kline.Low,
			"收盘价":    kline.Close,
			"成交量":    kline.Volume,
			"成交额(U)": kline.Amount,
			"涨跌幅(%)": kline.ChangePercent,
		})
	}

	table, err := bitgetItemToMarkdown(items)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "K 线数据格式化失败："+err.Error())
		return nil
	}

	axisNote := "（时间轴为 UTC+8，Bitget 日K以 UTC 00:00 为界）"
	title := fmt.Sprintf("%s %s [%s] K线数据%s", BitgetSymbolName(resolved), convertor.ToString(len(items)), interval, axisNote)
	res := fmt.Sprintf("\r\n ### %s：\r\n%s\r\n", title, table)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

// ===== GetBitgetFuturesDerivatives =====

func handleGetBitgetFuturesDerivatives(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	symbol := gjson.Get(funcArguments, "symbol").String()
	resolved, ok := bitgetResolveOrError(ctx, funcArguments, symbol)
	if !ok {
		return nil
	}

	bitgetToolProgress(ctx, funcArguments, "获取 Bitget 美股永续合约衍生指标")

	bundle := NewBitgetFuturesApi().GetDerivatives(resolved)
	if bundle == nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			BitgetSymbolName(resolved)+"：未获取到衍生指标数据。若在国内网络环境，请检查设置中的「合约代理」配置。")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("\r\n ### " + bundle.Name + "（" + bundle.Symbol + "）美股永续合约衍生指标：\r\n")

	fundingAvg := 0.0
	if n := len(bundle.FundingRateHistory); n > 0 {
		sum := 0.0
		for _, f := range bundle.FundingRateHistory {
			sum += bitgetFloat(f.FundingRate)
		}
		fundingAvg = sum / float64(n)
	}
	sb.WriteString("\r\n**资金费率**\r\n")
	sb.WriteString(fmt.Sprintf("- 标记价：%s / 指数价：%s / 基差：%.4f%%\r\n",
		convertor.ToString(bundle.MarkPrice), convertor.ToString(bundle.IndexPrice), bundle.Basis))
	sb.WriteString(fmt.Sprintf("- 当期资金费率：%.4f%%（结算周期 %d 小时，年化约 %.2f%%）\r\n",
		bundle.FundingRate*100, bundle.RatePeriod, bundle.AnnualizedRate))
	if fundingAvg != 0 {
		sb.WriteString(fmt.Sprintf("- 近 %d 期平均费率：%.4f%%\r\n", len(bundle.FundingRateHistory), fundingAvg*100))
	}
	if bundle.NextFundingTime > 0 {
		next := time.UnixMilli(bundle.NextFundingTime).In(bitgetCST)
		remain := time.Until(next)
		if remain < 0 {
			remain = 0
		}
		sb.WriteString(fmt.Sprintf("- 下次结算：%s（约 %s 后）\r\n",
			next.Format("2006-01-02 15:04"), formatBinanceDuration(remain)))
	}

	sb.WriteString("\r\n**未平仓量 OI**\r\n")
	sb.WriteString(fmt.Sprintf("- 当前 OI：%s（名义价值约 %s USDT）\r\n",
		convertor.ToString(bundle.OpenInterest), convertor.ToString(round2(bundle.OpenInterestUSD))))
	sb.WriteString("- 说明：Bitget 未提供 OI 历史端点，只能取当前值；亦不提供多空持仓比。\r\n")

	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, sb.String())
	return nil
}
