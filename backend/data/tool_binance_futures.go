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

// 币安 USDT-M 永续合约的 AI 工具 handler。
// 复用 tool_kline.go 的通用写法：gjson 解析参数 → ctx.Ch 推进度 → JSONToMarkdownTable 组装 → appendToolMessages。

func init() {
	registerToolHandler("GetBinanceFuturesMarket", handleGetBinanceFuturesMarket)
	registerToolHandler("GetBinanceFuturesKLine", handleGetBinanceFuturesKLine)
	registerToolHandler("GetBinanceFuturesDerivatives", handleGetBinanceFuturesDerivatives)
}

func binanceToolProgress(ctx *ToolContext, funcArguments string, tip string) {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：" + ctx.FuncName + "，" + tip + "，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}
}

// binanceItemToMarkdown 把一组 map 转成 markdown 表格。
func binanceItemToMarkdown(items []map[string]any) (string, error) {
	jsonData, err := json.Marshal(&items)
	if err != nil {
		return "", err
	}
	return JSONToMarkdownTable(jsonData)
}

// ===== GetBinanceFuturesMarket =====

// BinanceFuturesMarketMarkdown 生成币安 USDT-M 永续合约行情 markdown（入参为工具 JSON 参数）。
// 供 OpenAI 直连工具与 Agent 工具共用，保证两条链路的输出一致。
func BinanceFuturesMarketMarkdown(funcArguments string) (string, error) {
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

	api := NewBinanceFuturesApi()
	funding := map[string]float64{}
	for _, p := range api.GetAllPremiumIndex() {
		funding[strings.ToUpper(p.Symbol)] = binanceFloat(p.LastFundingRate)
	}

	var rows []BinanceTicker24h
	if symbols != "" {
		for _, raw := range strings.Split(symbols, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if resolved, ok := ResolveBinanceSymbol(raw); ok {
				if t := api.GetTicker24h(resolved); t != nil {
					rows = append(rows, *t)
				}
			}
		}
	} else {
		all := api.GetAllTickers()
		rows = binanceSortTickers(all, funding, sortBy, limit)
	}

	if len(rows) == 0 {
		return "未获取到币安永续合约行情数据。若在国内网络环境，请检查设置中的 HTTP 代理配置（币安接口可能需要代理）。", nil
	}

	items := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		sym := strings.ToUpper(t.Symbol)
		rate := funding[sym]
		item := map[string]any{
			"合约":         SymbolName(sym),
			"标识":         sym,
			"最新价":        binanceTrimNum(t.LastPrice),
			"24h涨跌幅(%)":  t.PriceChangePercent,
			"24h成交额(万U)": convertor.ToString(round2(binanceFloat(t.QuoteVolume) / 10000)),
			"当期资金费率(%)":  convertor.ToString(round4(rate * 100)),
		}
		items = append(items, item)
	}

	table, err := binanceItemToMarkdown(items)
	if err != nil {
		return "", fmt.Errorf("行情数据格式化失败：%w", err)
	}

	title := "币安 USDT-M 永续合约行情"
	switch sortBy {
	case "amount":
		title += "（按24h成交额）"
	case "funding":
		title += "（按资金费率绝对值）"
	default:
		title += "（按24h涨跌幅）"
	}
	return fmt.Sprintf("\r\n ### %s（共 %d 条）：\r\n%s\r\n", title, len(items), table), nil
}

func handleGetBinanceFuturesMarket(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	binanceToolProgress(ctx, funcArguments, "获取币安永续合约行情")
	res, err := BinanceFuturesMarketMarkdown(funcArguments)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, err.Error())
		return nil
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

func binanceSortTickers(list []BinanceTicker24h, funding map[string]float64, sortBy string, limit int) []BinanceTicker24h {
	rows := make([]BinanceTicker24h, len(list))
	copy(rows, list)
	switch sortBy {
	case "amount":
		sort.SliceStable(rows, func(i, j int) bool {
			return binanceFloat(rows[i].QuoteVolume) > binanceFloat(rows[j].QuoteVolume)
		})
	case "funding":
		sort.SliceStable(rows, func(i, j int) bool {
			return math.Abs(funding[strings.ToUpper(rows[i].Symbol)]) > math.Abs(funding[strings.ToUpper(rows[j].Symbol)])
		})
	default:
		sort.SliceStable(rows, func(i, j int) bool {
			return binanceFloat(rows[i].PriceChangePercent) > binanceFloat(rows[j].PriceChangePercent)
		})
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// ===== GetBinanceFuturesKLine =====

// BinanceFuturesKLineMarkdown 生成币安 USDT-M 永续合约 K 线 markdown（入参为工具 JSON 参数）。
func BinanceFuturesKLineMarkdown(funcArguments string) (string, error) {
	symbol := gjson.Get(funcArguments, "symbol").String()
	if strings.TrimSpace(symbol) == "" {
		return "参数 symbol 不能为空，请传入合约标识（如 btc、比特币、btc/usdt、bn:btcusdt）。", nil
	}
	resolved, ok := ResolveBinanceSymbol(symbol)
	if !ok {
		return fmt.Sprintf("%s：未在币安 USDT-M 永续合约中找到该品种，请确认合约标识（如 btc、eth、sol）。", symbol), nil
	}
	kLineType := gjson.Get(funcArguments, "interval").String()
	if strings.TrimSpace(kLineType) == "" {
		kLineType = "101"
	}
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 90
	}
	if limit > 1500 {
		limit = 1500
	}

	kType := normalizeKLineType(kLineType)
	interval := binanceIntervalFromKlt(kType)
	data := NewBinanceFuturesApi().GetKLine(resolved, kType, limit, "")
	if data == nil || len(*data) == 0 {
		return SymbolName(resolved) + "：未获取到 K 线数据。若在国内网络环境，请检查设置中的 HTTP 代理配置。", nil
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

	table, err := binanceItemToMarkdown(items)
	if err != nil {
		return "", fmt.Errorf("K 线数据格式化失败：%w", err)
	}

	axisNote := "（时间轴为 UTC+8，币安日K以 UTC 00:00 为界）"
	title := fmt.Sprintf("%s %s [%s] K线数据%s", SymbolName(resolved), convertor.ToString(len(items)), interval, axisNote)
	return fmt.Sprintf("\r\n ### %s：\r\n%s\r\n", title, table), nil
}

func handleGetBinanceFuturesKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	binanceToolProgress(ctx, funcArguments, "获取币安永续合约K线")
	res, err := BinanceFuturesKLineMarkdown(funcArguments)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, err.Error())
		return nil
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

// ===== GetBinanceFuturesDerivatives =====

// BinanceFuturesDerivativesMarkdown 生成币安 USDT-M 永续合约衍生指标 markdown（入参为工具 JSON 参数）。
func BinanceFuturesDerivativesMarkdown(funcArguments string) (string, error) {
	symbol := gjson.Get(funcArguments, "symbol").String()
	if strings.TrimSpace(symbol) == "" {
		return "参数 symbol 不能为空，请传入合约标识（如 btc、比特币、btc/usdt、bn:btcusdt）。", nil
	}
	resolved, ok := ResolveBinanceSymbol(symbol)
	if !ok {
		return fmt.Sprintf("%s：未在币安 USDT-M 永续合约中找到该品种，请确认合约标识（如 btc、eth、sol）。", symbol), nil
	}
	period := strings.TrimSpace(gjson.Get(funcArguments, "period").String())
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 48
	}
	if limit > 120 {
		limit = 120
	}

	bundle := NewBinanceFuturesApi().GetDerivatives(resolved, period, limit)
	if bundle == nil {
		return SymbolName(resolved) + "：未获取到衍生指标数据。若在国内网络环境，请检查设置中的 HTTP 代理配置。", nil
	}

	var sb strings.Builder
	sb.WriteString("\r\n ### " + bundle.Name + "（" + bundle.Symbol + "）永续合约衍生指标：\r\n")

	// 1) 价格与资金费率
	fundingAvg := 0.0
	if n := len(bundle.FundingRateHistory); n > 0 {
		sum := 0.0
		for _, f := range bundle.FundingRateHistory {
			sum += binanceFloat(f.FundingRate)
		}
		fundingAvg = sum / float64(n)
	}
	sb.WriteString("\r\n**资金费率**\r\n")
	sb.WriteString(fmt.Sprintf("- 标记价：%s / 指数价：%s / 基差：%.4f%%\r\n",
		convertor.ToString(bundle.MarkPrice), convertor.ToString(bundle.IndexPrice), bundle.Basis))
	sb.WriteString(fmt.Sprintf("- 当期资金费率：%.4f%%（年化约 %.2f%%）\r\n",
		bundle.LastFundingRate*100, bundle.AnnualizedRate))
	if fundingAvg != 0 {
		sb.WriteString(fmt.Sprintf("- 近 %d 期平均费率：%.4f%%\r\n", len(bundle.FundingRateHistory), fundingAvg*100))
	}
	if bundle.NextFundingTime > 0 {
		next := time.UnixMilli(bundle.NextFundingTime).In(binanceCST)
		remain := time.Until(next)
		if remain < 0 {
			remain = 0
		}
		sb.WriteString(fmt.Sprintf("- 下次结算：%s（约 %s 后）\r\n",
			next.Format("2006-01-02 15:04"), formatBinanceDuration(remain)))
	}

	// 2) 未平仓量
	sb.WriteString("\r\n**未平仓量 OI**\r\n")
	sb.WriteString(fmt.Sprintf("- 当前 OI：%s（名义价值约 %s USDT）\r\n",
		convertor.ToString(bundle.OpenInterest), convertor.ToString(round2(bundle.OpenInterestValue))))
	sb.WriteString(fmt.Sprintf("- 区间变化率：%.2f%%\r\n", bundle.OpenInterestChange))

	// 3) 多空比
	sb.WriteString("\r\n**多空持仓比**\r\n")
	sb.WriteString(fmt.Sprintf("- 全局账户多空比：%.4f（多头账户 %.2f%% / 空头账户 %.2f%%）\r\n",
		bundle.LongShortRatio, bundle.LongAccount*100, bundle.ShortAccount*100))
	sb.WriteString(fmt.Sprintf("- 主动买卖量比：%.4f\r\n", bundle.TakerBuySellRatio))
	if n := len(bundle.LongShortHistory); n >= 2 {
		first := bundle.LongShortHistory[0].Value
		last := bundle.LongShortHistory[n-1].Value
		trend := "走平"
		if last > first*1.02 {
			trend = "多头增仓（偏多）"
		} else if last < first*0.98 {
			trend = "空头增仓（偏空）"
		}
		sb.WriteString(fmt.Sprintf("- 近 %d 期多空比趋势：%s → %s，%s\r\n",
			n, convertor.ToString(round4(first)), convertor.ToString(round4(last)), trend))
	}

	return sb.String(), nil
}

func handleGetBinanceFuturesDerivatives(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	binanceToolProgress(ctx, funcArguments, "获取币安永续合约衍生指标")
	res, err := BinanceFuturesDerivativesMarkdown(funcArguments)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, err.Error())
		return nil
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

// formatBinanceDuration 把时长格式化为「x小时y分」。
func formatBinanceDuration(d time.Duration) string {
	if d <= 0 {
		return "0分"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%d小时%d分", h, m)
	}
	return fmt.Sprintf("%d分", m)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func round4(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
