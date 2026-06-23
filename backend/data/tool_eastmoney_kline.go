package data

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("GetEastMoneyKLine", handleGetEastMoneyKLine)
	registerToolHandler("GetEastMoneyKLineWithMA", handleGetEastMoneyKLineWithMA)
}

// normalizeKLineType 将前端/自然语言 K 线类型转为东方财富 API 参数
func normalizeKLineType(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "day", "日", "101", "日k", "日k线":
		return "101"
	case "week", "周", "102", "周k", "周k线":
		return "102"
	case "month", "月", "103", "月k", "月k线":
		return "103"
	case "quarter", "季", "104", "季k", "季k线":
		return "104"
	case "halfyear", "半年", "105", "半年k", "半年k线", "半年k线图":
		return "105"
	case "year", "年", "106", "年k", "年k线":
		return "106"
	case "1", "1min", "1分钟":
		return "1"
	case "5", "5min", "5分钟":
		return "5"
	case "15", "15min", "15分钟":
		return "15"
	case "30", "30min", "30分钟":
		return "30"
	case "60", "60min", "60分钟":
		return "60"
	case "120", "120min", "120分钟", "2h", "两小时", "2小时":
		return "120"
	default:
		return s
	}
}

func EastMoneyKLineSection(api *EastMoneyKLineApi, stockCode, kLineType, adjustFlag string, limit int) string {
	if !api.ValidateStockCode(stockCode) {
		return stockCode + "：股票代码无效，请使用正确格式（如 000001.SZ、600000.SH、00700.HK）。"
	}
	kType := normalizeKLineType(kLineType)
	var list *[]KLineData
	if adjustFlag != "" && (kType == "101" || kType == "day") {
		adj := strings.TrimSpace(strings.ToLower(adjustFlag))
		if adj != "qfq" && adj != "hfq" {
			adj = "qfq"
		}
		list = api.GetAdjustedKLine(stockCode, adj, int(limit))
	} else {
		list = api.GetKLineData(stockCode, kType, strings.TrimSpace(adjustFlag), int(limit))
	}
	var sourceLabel string
	if list == nil || len(*list) == 0 {
		fallbackResult := FetchKLineWithFallback(stockCode, "", kType, limit, "")
		if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
			list = fallbackResult.Data
			sourceLabel = fallbackResult.Source
		}
	}
	if list == nil || len(*list) == 0 {
		return stockCode + "：未获取到 K 线数据，请检查股票代码与类型。"
	}
	rows := make([]map[string]any, 0, len(*list))
	for _, k := range *list {
		vol, _ := convertor.ToFloat(k.Volume)
		rows = append(rows, map[string]any{
			"日期":      k.Day,
			"开盘价":     k.Open,
			"收盘价":     k.Close,
			"最高价":     k.High,
			"最低价":     k.Low,
			"成交量(万手)": vol / 10000 / 100,
			"涨跌幅(%)":  k.ChangePercent,
			"涨跌额":     k.ChangeValue,
			"振幅(%)":   k.Amplitude,
			"换手率(%)":  k.TurnoverRate,
		})
	}
	jsonData, _ := json.Marshal(rows)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		markdownTable = string(jsonData)
	}
	typeLabel := kLineType
	if typeLabel == "" {
		typeLabel = kType
	}
	sourceInfo := ""
	if sourceLabel != "" {
		sourceInfo = "（数据源：" + sourceLabel + "）"
	}
	return "\r\n### " + stockCode + " " + typeLabel + " K线（共 " + convertor.ToString(len(*list)) + " 条）" + sourceInfo + "\r\n" + markdownTable + "\r\n"
}

func handleGetEastMoneyKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	kLineType := gjson.Get(funcArguments, "kLineType").String()
	adjustFlag := gjson.Get(funcArguments, "adjustFlag").String()
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 60
	}
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。")
		return nil
	}

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetEastMoneyKLine，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	kType := normalizeKLineType(kLineType)
	res := parallelStockToolSections(codes, func(stockCode string) string {
		// A股优先使用 FetchKLineWithFallback（MAC→东方财富→新浪→腾讯→通达信）
		if IsAStockCode(stockCode) {
			return FetchKLineWithFallbackAsSection(stockCode, kType, limit)
		}
		api := NewEastMoneyKLineApi(GetSettingConfig())
		return EastMoneyKLineSection(api, stockCode, kLineType, adjustFlag, limit)
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

// sortMALabels 按 MA 周期数字排序，如 MA5,MA10,MA20,MA60
func sortMALabels(labels []string) []string {
	if len(labels) <= 1 {
		return labels
	}
	nums := make([]int, len(labels))
	for i, l := range labels {
		n, _ := strconv.Atoi(strings.TrimPrefix(l, "MA"))
		nums[i] = n
	}
	for i := 0; i < len(nums); i++ {
		for j := i + 1; j < len(nums); j++ {
			if nums[i] > nums[j] {
				nums[i], nums[j] = nums[j], nums[i]
				labels[i], labels[j] = labels[j], labels[i]
			}
		}
	}
	return labels
}

// parseMaPeriods 解析 "5,10,20,60" 为 []int
func parseMaPeriods(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func EastMoneyKLineWithMASection(api *EastMoneyKLineApi, stockCode, kLineType string, limit int, maPeriodsStr string) string {
	if !api.ValidateStockCode(stockCode) {
		return stockCode + "：股票代码无效，请使用正确格式（如 000001.SZ、600000.SH、00700.HK）。"
	}
	kType := normalizeKLineType(kLineType)
	maPeriods := parseMaPeriods(maPeriodsStr)
	list, err := api.GetKLineWithMA(stockCode, kType, int(limit), maPeriods...)
	var sourceLabel string
	if err != nil || list == nil || len(*list) == 0 {
		fallbackResult := FetchKLineWithFallback(stockCode, "", kType, limit, "")
		if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
			list = fallbackResult.Data
			sourceLabel = fallbackResult.Source
		}
	}
	if list == nil || len(*list) == 0 {
		return stockCode + "：未获取到带均线的 K 线数据，请检查股票代码与参数。"
	}
	maLabels := make([]string, 0, len(maPeriods))
	if len(maPeriods) > 0 {
		for _, p := range maPeriods {
			maLabels = append(maLabels, "MA"+strconv.Itoa(p))
		}
	} else if len(*list) > 0 && (*list)[0].MA != nil {
		for p := range (*list)[0].MA {
			maLabels = append(maLabels, "MA"+p)
		}
		maLabels = sortMALabels(maLabels)
	}
	rows := make([]map[string]any, 0, len(*list))
	for _, k := range *list {
		vol, _ := convertor.ToFloat(k.Volume)
		row := map[string]any{
			"日期":      k.Day,
			"开盘价":     k.Open,
			"收盘价":     k.Close,
			"最高价":     k.High,
			"最低价":     k.Low,
			"成交量(万手)": vol / 10000 / 100,
			"涨跌幅(%)":  k.ChangePercent,
			"涨跌额":     k.ChangeValue,
			"振幅(%)":   k.Amplitude,
			"换手率(%)":  k.TurnoverRate,
		}
		for _, label := range maLabels {
			p := strings.TrimPrefix(label, "MA")
			if v, ok := k.MA[p]; ok && v != "" {
				row[label] = v
			}
		}
		rows = append(rows, row)
	}
	jsonData, _ := json.Marshal(rows)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		markdownTable = string(jsonData)
	}
	typeLabel := kLineType
	if typeLabel == "" {
		typeLabel = kType
	}
	sourceInfo := ""
	if sourceLabel != "" {
		sourceInfo = "（数据源：" + sourceLabel + "）"
	}
	return "\r\n### " + stockCode + " " + typeLabel + " K线+均线（共 " + convertor.ToString(len(*list)) + " 条）" + sourceInfo + "\r\n" + markdownTable + "\r\n"
}

func handleGetEastMoneyKLineWithMA(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	kLineType := gjson.Get(funcArguments, "kLineType").String()
	limit := int(gjson.Get(funcArguments, "limit").Int())
	maPeriodsStr := gjson.Get(funcArguments, "maPeriods").String()
	if limit <= 0 {
		limit = 60
	}
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。")
		return nil
	}

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetEastMoneyKLineWithMA，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	res := parallelStockToolSections(codes, func(stockCode string) string {
		// A股优先使用 FetchKLineWithFallback + 均线计算
		if IsAStockCode(stockCode) {
			return FetchKLineWithMASection(stockCode, normalizeKLineType(kLineType), limit, maPeriodsStr)
		}
		api := NewEastMoneyKLineApi(GetSettingConfig())
		return EastMoneyKLineWithMASection(api, stockCode, kLineType, limit, maPeriodsStr)
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}

// IsAStockCode 判断股票代码是否为A股（沪深京市场）
func IsAStockCode(code string) bool {
	return strings.HasSuffix(code, ".SZ") || strings.HasSuffix(code, ".SH") || strings.HasSuffix(code, ".BJ")
}

// IsHKStockCode 判断股票代码是否为港股
func IsHKStockCode(code string) bool {
	upper := strings.ToUpper(code)
	return strings.HasSuffix(upper, ".HK") || strings.HasPrefix(upper, "HK")
}

// IsUSStockCode 判断股票代码是否为美股
func IsUSStockCode(code string) bool {
	upper := strings.ToUpper(code)
	return strings.HasSuffix(upper, ".US") || strings.HasPrefix(upper, "US") || strings.HasPrefix(upper, "GB_")
}

// NormalizeKLineType 导出 normalizeKLineType 供外部包使用
func NormalizeKLineType(s string) string {
	return normalizeKLineType(s)
}

// FetchKLineWithFallbackAsSection 使用 FetchKLineWithFallback 获取K线数据并格式化为 markdown section
func FetchKLineWithFallbackAsSection(stockCode, klt string, limit int) string {
	kType := normalizeKLineType(klt)
	fallbackResult := FetchKLineWithFallback(stockCode, "", kType, limit, "")
	if fallbackResult.Data == nil || len(*fallbackResult.Data) == 0 {
		return stockCode + "：未获取到 K 线数据，请检查股票代码与类型。"
	}
	list := fallbackResult.Data
	rows := make([]map[string]any, 0, len(*list))
	for _, k := range *list {
		vol, _ := convertor.ToFloat(k.Volume)
		rows = append(rows, map[string]any{
			"日期":      k.Day,
			"开盘价":     k.Open,
			"收盘价":     k.Close,
			"最高价":     k.High,
			"最低价":     k.Low,
			"成交量(万手)": vol / 10000 / 100,
			"涨跌幅(%)":  k.ChangePercent,
			"涨跌额":     k.ChangeValue,
			"振幅(%)":   k.Amplitude,
			"换手率(%)":  k.TurnoverRate,
		})
	}
	jsonData, _ := json.Marshal(rows)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		markdownTable = string(jsonData)
	}
	sourceInfo := ""
	if fallbackResult.Source != "" {
		sourceInfo = "（数据源：" + fallbackResult.Source + "）"
	}
	return "\r\n### " + stockCode + " " + klt + " K线（共 " + convertor.ToString(len(*list)) + " 条）" + sourceInfo + "\r\n" + markdownTable + "\r\n"
}

// FetchKLineWithMASection 使用 FetchKLineWithFallback 获取K线数据并附均线，格式化为 markdown section
func FetchKLineWithMASection(stockCode, klt string, limit int, maPeriodsStr string) string {
	kType := normalizeKLineType(klt)
	fallbackResult := FetchKLineWithFallback(stockCode, "", kType, limit, "")
	if fallbackResult.Data == nil || len(*fallbackResult.Data) == 0 {
		return stockCode + "：未获取到带均线的 K 线数据，请检查股票代码与参数。"
	}
	list := fallbackResult.Data

	// 计算均线
	maPeriods := parseMaPeriods(maPeriodsStr)
	if len(maPeriods) == 0 {
		maPeriods = []int{5, 10, 20, 60, 120}
	}
	calculateSMA(list, maPeriods)

	maLabels := make([]string, 0, len(maPeriods))
	for _, p := range maPeriods {
		maLabels = append(maLabels, "MA"+strconv.Itoa(p))
	}

	rows := make([]map[string]any, 0, len(*list))
	for _, k := range *list {
		vol, _ := convertor.ToFloat(k.Volume)
		row := map[string]any{
			"日期":      k.Day,
			"开盘价":     k.Open,
			"收盘价":     k.Close,
			"最高价":     k.High,
			"最低价":     k.Low,
			"成交量(万手)": vol / 10000 / 100,
			"涨跌幅(%)":  k.ChangePercent,
			"涨跌额":     k.ChangeValue,
			"振幅(%)":   k.Amplitude,
			"换手率(%)":  k.TurnoverRate,
		}
		for _, label := range maLabels {
			p := strings.TrimPrefix(label, "MA")
			if v, ok := k.MA[p]; ok && v != "" {
				row[label] = v
			}
		}
		rows = append(rows, row)
	}
	jsonData, _ := json.Marshal(rows)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		markdownTable = string(jsonData)
	}
	sourceInfo := ""
	if fallbackResult.Source != "" {
		sourceInfo = "（数据源：" + fallbackResult.Source + "）"
	}
	return "\r\n### " + stockCode + " " + klt + " K线+均线（共 " + convertor.ToString(len(*list)) + " 条）" + sourceInfo + "\r\n" + markdownTable + "\r\n"
}

// calculateSMA 按收盘价计算简单移动均线，写入 KLineData.MA
func calculateSMA(list *[]KLineData, periods []int) {
	if list == nil || len(*list) == 0 {
		return
	}
	n := len(*list)
	closes := make([]float64, n)
	for i, k := range *list {
		closes[i], _ = strconv.ParseFloat(k.Close, 64)
	}
	for _, p := range periods {
		if p > n {
			continue
		}
		for i := p - 1; i < n; i++ {
			sum := 0.0
			for j := i - p + 1; j <= i; j++ {
				sum += closes[j]
			}
			avg := sum / float64(p)
			if (*list)[i].MA == nil {
				(*list)[i].MA = make(map[string]string)
			}
			(*list)[i].MA[strconv.Itoa(p)] = fmt.Sprintf("%.2f", avg)
		}
	}
}
