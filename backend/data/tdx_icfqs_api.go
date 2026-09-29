package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"

	gotdx "github.com/bensema/gotdx"
	"github.com/tidwall/gjson"
)

// IcfqsApi 封装 ICFQS（通达信 TQLEX HTTP）接口，提供龙虎榜/游资与主题投资数据。
// ICFQS 为无状态 HTTP 接口，直接使用默认节点（121.37.193.4:7615 与 hot.icfqs.com:7615），
// 无需连接管理；失败时返回错误由上层提示，不做降级。
type IcfqsApi struct{}

// NewIcfqsApi 创建 ICFQS 数据接口实例
func NewIcfqsApi() *IcfqsApi { return &IcfqsApi{} }

const icfqsHTTPTimeout = 10 * time.Second

func (api *IcfqsApi) newClient() *gotdx.ICFQSClient {
	return gotdx.NewICFQS(gotdx.WithICFQSTimeout(icfqsHTTPTimeout))
}

// newHotClient 龙虎榜/游资类接口默认使用 hot.icfqs.com 节点
func (api *IcfqsApi) newHotClient() *gotdx.ICFQSClient {
	return gotdx.NewICFQSHot(gotdx.WithICFQSTimeout(icfqsHTTPTimeout))
}

// icfqsColumnsOf 解析 ResultSets 中的列名，兼容 ColName 数组、ColName 空格分隔字符串、ColDes 数组三种返回。
func icfqsColumnsOf(resultSet map[string]any) []string {
	if rawColumns, ok := resultSet["ColName"].([]any); ok && len(rawColumns) > 0 {
		columns := make([]string, 0, len(rawColumns))
		for _, rawColumn := range rawColumns {
			if name := strings.TrimSpace(fmt.Sprint(rawColumn)); name != "" {
				columns = append(columns, name)
			}
		}
		if len(columns) > 0 {
			return columns
		}
	}
	if rawName, ok := resultSet["ColName"].(string); ok && strings.TrimSpace(rawName) != "" {
		if fields := strings.Fields(rawName); len(fields) > 0 {
			return fields
		}
	}
	rawColumnDefs, _ := resultSet["ColDes"].([]any)
	columns := make([]string, 0, len(rawColumnDefs))
	for _, rawColumnDef := range rawColumnDefs {
		columnDef, _ := rawColumnDef.(map[string]any)
		if name := strings.TrimSpace(fmt.Sprint(columnDef["Name"])); name != "" {
			columns = append(columns, name)
		}
	}
	return columns
}

// icfqsDataTable 从 ICFQS 响应中取出数据表。
// 部分接口（如主题轮动）会返回多张 ResultSets：行数表、日期表、真实数据表，
// 这里统一选取列数最多的一张，避免误取前置的辅助表。
func icfqsDataTable(raw map[string]any) ([]string, [][]any) {
	resultSets, _ := raw["ResultSets"].([]any)
	if len(resultSets) == 0 {
		return nil, nil
	}
	bestIndex := 0
	bestColumns := 0
	for index, item := range resultSets {
		resultSet, _ := item.(map[string]any)
		if columnCount, ok := resultSet["ColNum"].(float64); ok && int(columnCount) > bestColumns {
			bestColumns = int(columnCount)
			bestIndex = index
		}
	}
	if bestColumns == 0 {
		return icfqsTable(resultSets[0])
	}
	return icfqsTable(resultSets[bestIndex])
}

// icfqsTable 解析单张 ResultSet 的列名与行数据
func icfqsTable(resultSet any) ([]string, [][]any) {
	table, _ := resultSet.(map[string]any)
	if table == nil {
		return nil, nil
	}
	columns := icfqsColumnsOf(table)
	content, _ := table["Content"].([]any)
	rows := make([][]any, 0, len(content))
	for _, item := range content {
		values, _ := item.([]any)
		if len(values) == 0 {
			continue
		}
		rows = append(rows, values)
	}
	if len(columns) == 0 && len(rows) > 0 {
		columns = make([]string, len(rows[0]))
		for i := range columns {
			columns[i] = fmt.Sprintf("col_%d", i)
		}
	}
	return columns, rows
}

// icfqsError 从 ICFQS 响应中提取错误信息（ErrorCode 非 0 表示失败）
func icfqsError(raw map[string]any) error {
	code, _ := raw["ErrorCode"].(float64)
	if code == 0 {
		return nil
	}
	return fmt.Errorf("ICFQS 返回错误(ErrorCode=%d)：%v", int(code), raw["ErrorInfo"])
}

// icfqsLabel 返回列的中文表头，未配置时回退原始列名
func icfqsLabel(labels map[string]string, column string) string {
	if labels != nil {
		if label, ok := labels[column]; ok {
			return label
		}
	}
	return column
}

// icfqsCellText 清洗单元格文本：去换行、去竖线，避免破坏 Markdown 表格
func icfqsCellText(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "|", "/")
	if len(text) > 120 {
		text = text[:120] + "…"
	}
	return text
}

// IcfqsMarkdown 将 ICFQS 行列数据渲染为 Markdown 表格（limit>0 时只输出前 limit 行）。
func IcfqsMarkdown(title string, columns []string, rows [][]any, labels map[string]string, limit int) string {
	if len(columns) == 0 || len(rows) == 0 {
		return fmt.Sprintf("\n## %s\n无数据\n", title)
	}
	total := len(rows)
	if limit > 0 && total > limit {
		rows = rows[:limit]
	}
	var sb strings.Builder
	sb.WriteString("\n## " + title + "\n")
	sb.WriteString("|")
	for _, column := range columns {
		sb.WriteString(" " + icfqsLabel(labels, column) + " |")
	}
	sb.WriteString("\n|")
	for range columns {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for _, row := range rows {
		sb.WriteString("|")
		for i := range columns {
			value := ""
			if i < len(row) {
				value = icfqsCellText(row[i])
			}
			sb.WriteString(" " + value + " |")
		}
		sb.WriteString("\n")
	}
	if total > len(rows) {
		sb.WriteString(fmt.Sprintf("\n> 共 %d 条，已展示前 %d 条。\n", total, len(rows)))
	}
	return sb.String()
}

// icfqsNormalizeDate 把 YYYY-MM-DD / YYYY/MM/DD 归一化为 YYYYMMDD，空值原样返回
func icfqsNormalizeDate(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return ""
	}
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")
	return date
}

// icfqsPlainStockCode 去掉 .SH/.SZ/.BJ/.HK 后缀与 sh/sz/bj 前缀，得到纯代码
func icfqsPlainStockCode(stockCode string) string {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	for _, suffix := range []string{".SH", ".SZ", ".BJ", ".HK", ".US"} {
		code = strings.TrimSuffix(code, suffix)
	}
	for _, prefix := range []string{"SH", "SZ", "BJ", "HK"} {
		if len(code) > len(prefix)+3 {
			code = strings.TrimPrefix(code, prefix)
		}
	}
	return code
}

// ---------- 龙虎榜 / 游资 ----------

// IcfqsLHBLabels 龙虎榜类接口列名中文对照
var IcfqsLHBLabels = map[string]string{
	"sc":        "市场",
	"gpmc":      "股票名称",
	"gpdm":      "股票代码",
	"yzmc":      "游资名称",
	"yyb":       "营业部",
	"sblx":      "上榜类型码",
	"dealtype":  "买卖方向(B买/S卖)",
	"deallevel": "排名",
	"mrje":      "买入金额(元)",
	"mcje":      "卖出金额(元)",
	"mmbg":      "买卖净额(元)",
	"rq":        "上榜日期",
}

// GetStockLHBDetail 个股龙虎榜明细（按席位展示买卖金额）
func (api *IcfqsApi) GetStockLHBDetail(symbol, startDate, endDate string) ([]string, [][]any, error) {
	raw, err := api.newHotClient().ICFQSLHBDetailRaw(context.Background(), icfqsPlainStockCode(symbol), icfqsNormalizeDate(startDate), icfqsNormalizeDate(endDate))
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS LHBDetail error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetYYBLHBDetail 营业部龙虎榜明细（按营业部名称查询其上榜记录）
func (api *IcfqsApi) GetYYBLHBDetail(yybName, startDate, endDate string) ([]string, [][]any, error) {
	raw, err := api.newHotClient().ICFQSYYBDetailRaw(context.Background(), strings.TrimSpace(yybName), icfqsNormalizeDate(startDate), icfqsNormalizeDate(endDate))
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS YYBDetail error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetActiveCapitalDetail 游资席位详情（按游资/席位代码查询买卖明细）
func (api *IcfqsApi) GetActiveCapitalDetail(code, startDate, endDate string) ([]string, [][]any, error) {
	raw, err := api.newHotClient().ICFQSYZDetailRaw(context.Background(), strings.TrimSpace(code), icfqsNormalizeDate(startDate), icfqsNormalizeDate(endDate))
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS YZDetail error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// ---------- 主题投资 / 轮动 ----------

// IcfqsTopicRotationLabels 主题轮动列名中文对照
var IcfqsTopicRotationLabels = map[string]string{
	"configType": "板块类别",
	"configCode": "主题代码",
	"topicName":  "主题名称",
	"topicType":  "主题类型",
	"dataType":   "涨跌幅",
	"dataDate":   "日期",
	"dataRank":   "排名",
}

// IcfqsHotTopicsLabels 热门主题列名中文对照
var IcfqsHotTopicsLabels = map[string]string{
	"N001": "排名",
	"N002": "类型代码",
	"N003": "主题代码",
	"N004": "主题名称",
	"N005": "事件日期",
	"N006": "备注",
	"N007": "事件描述",
	"N008": "详情链接",
}

// IcfqsTopTopicsLabels 领涨主题列名中文对照
var IcfqsTopTopicsLabels = map[string]string{
	"N001": "类型代码",
	"N002": "主题代码",
	"N003": "主题名称",
}

// IcfqsTopicStocksLabels 主题成分股列名中文对照
var IcfqsTopicStocksLabels = map[string]string{
	"N001": "类型代码",
	"N002": "主题代码",
	"N003": "分类代码",
	"N004": "股票代码",
	"N005": "股票名称",
	"N006": "标记",
	"N007": "关联度",
	"N008": "入选说明",
	"N009": "收录日期",
}

// IcfqsTopicKLineLabels 主题走势列名中文对照
var IcfqsTopicKLineLabels = map[string]string{
	"N001": "日期",
	"N002": "收盘/点位",
	"N003": "涨跌幅",
	"N004": "成交额",
	"N005": "上涨家数",
	"N006": "下跌家数",
}

// GetTopicRotation 主题轮动排行（按涨跌幅排名）
// dataNum: 1=前后10 2=前10 3=前20 4=前30 5=后20 6=后30
func (api *IcfqsApi) GetTopicRotation(dataNum, dataType, dataDate int, themeType string) ([]string, [][]any, error) {
	raw, err := api.newClient().ICFQSTopicRotationRaw(context.Background(), dataNum, dataType, dataDate, themeType)
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS TopicRotation error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetHotTopics 热门主题列表（含事件驱动说明与详情链接）
func (api *IcfqsApi) GetHotTopics() ([]string, [][]any, error) {
	raw, err := api.newClient().ICFQSHotTopicsRaw(context.Background())
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS HotTopics error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetTopTopics 领涨主题排行
func (api *IcfqsApi) GetTopTopics(topN int) ([]string, [][]any, error) {
	raw, err := api.newClient().ICFQSTopTopicsRaw(context.Background(), topN)
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS TopTopics error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetTopicStocks 主题关联成分股列表
func (api *IcfqsApi) GetTopicStocks(code, setcode string, page, size int) ([]string, [][]any, error) {
	raw, err := api.newClient().ICFQSTopicStocksRaw(context.Background(), strings.TrimSpace(code), strings.TrimSpace(setcode), page, size)
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS TopicStocks error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// GetTopicKLine 主题历史走势（日线，含点位/涨跌幅/成交额）
func (api *IcfqsApi) GetTopicKLine(code, setcode string) ([]string, [][]any, error) {
	raw, err := api.newClient().ICFQSTopicKLineRaw(context.Background(), strings.TrimSpace(code), strings.TrimSpace(setcode))
	if err != nil {
		logger.SugaredLogger.Warnf("ICFQS TopicKLine error: %v", err)
		return nil, nil, err
	}
	if err := icfqsError(raw); err != nil {
		return nil, nil, err
	}
	columns, rows := icfqsDataTable(raw)
	return columns, rows, nil
}

// ---------- 工具处理函数 ----------

func init() {
	registerToolHandler("GetStockLHBDetail", handleGetStockLHBDetail)
	registerToolHandler("GetYYBLHBDetail", handleGetYYBLHBDetail)
	registerToolHandler("GetActiveCapitalDetail", handleGetActiveCapitalDetail)
	registerToolHandler("GetTopicRotation", handleGetTopicRotation)
	registerToolHandler("GetHotTopics", handleGetHotTopics)
	registerToolHandler("GetTopTopics", handleGetTopTopics)
	registerToolHandler("GetTopicStocks", handleGetTopicStocks)
	registerToolHandler("GetTopicKLine", handleGetTopicKLine)
}

func icfqsIntArg(funcArguments, key string, defaultValue, maxValue int) int {
	value := int(gjson.Get(funcArguments, key).Int())
	if value <= 0 {
		value = defaultValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func handleGetStockLHBDetail(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockLHBDetail", funcArguments)
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	startDate := gjson.Get(funcArguments, "startDate").String()
	endDate := gjson.Get(funcArguments, "endDate").String()
	limit := icfqsIntArg(funcArguments, "limit", 40, 500)

	api := NewIcfqsApi()
	md := parallelStockToolSections(codes, func(code string) string {
		columns, rows, err := api.GetStockLHBDetail(code, startDate, endDate)
		if err != nil {
			return fmt.Sprintf("\n## %s 龙虎榜\n获取失败：%v\n", code, err)
		}
		return IcfqsMarkdown(code+" 龙虎榜席位明细（ICFQS）", columns, rows, IcfqsLHBLabels, limit)
	})
	if strings.TrimSpace(md) == "" {
		md = "参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。"
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetYYBLHBDetail(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetYYBLHBDetail", funcArguments)
	yybName := strings.TrimSpace(gjson.Get(funcArguments, "yybName").String())
	if yybName == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 yybName 不能为空，请传入营业部名称。")
		return nil
	}
	limit := icfqsIntArg(funcArguments, "limit", 40, 500)
	columns, rows, err := NewIcfqsApi().GetYYBLHBDetail(yybName, gjson.Get(funcArguments, "startDate").String(), gjson.Get(funcArguments, "endDate").String())
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "营业部龙虎榜获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown(yybName+" 营业部龙虎榜（ICFQS）", columns, rows, IcfqsLHBLabels, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetActiveCapitalDetail(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetActiveCapitalDetail", funcArguments)
	code := strings.TrimSpace(gjson.Get(funcArguments, "code").String())
	if code == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 code 不能为空，请传入游资/席位代码。")
		return nil
	}
	limit := icfqsIntArg(funcArguments, "limit", 40, 500)
	columns, rows, err := NewIcfqsApi().GetActiveCapitalDetail(code, gjson.Get(funcArguments, "startDate").String(), gjson.Get(funcArguments, "endDate").String())
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "游资席位列明细获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown(code+" 游资席位明细（ICFQS）", columns, rows, IcfqsLHBLabels, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetTopicRotation(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTopicRotation", funcArguments)
	dataNum := icfqsIntArg(funcArguments, "dataNum", 2, 6)
	dataType := icfqsIntArg(funcArguments, "dataType", 1, 10)
	dataDate := icfqsIntArg(funcArguments, "dataDate", 2, 10)
	themeType := strings.TrimSpace(gjson.Get(funcArguments, "themeType").String())
	limit := icfqsIntArg(funcArguments, "limit", 30, 200)

	columns, rows, err := NewIcfqsApi().GetTopicRotation(dataNum, dataType, dataDate, themeType)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "主题轮动数据获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown("主题轮动排行（ICFQS）", columns, rows, IcfqsTopicRotationLabels, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetHotTopics(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetHotTopics", funcArguments)
	limit := icfqsIntArg(funcArguments, "limit", 20, 100)
	columns, rows, err := NewIcfqsApi().GetHotTopics()
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "热门主题数据获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown("热门主题（ICFQS）", columns, rows, IcfqsHotTopicsLabels, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetTopTopics(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTopTopics", funcArguments)
	topN := icfqsIntArg(funcArguments, "topN", 10, 100)
	columns, rows, err := NewIcfqsApi().GetTopTopics(topN)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "领涨主题数据获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown("领涨主题排行（ICFQS）", columns, rows, IcfqsTopTopicsLabels, topN)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetTopicStocks(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTopicStocks", funcArguments)
	code := strings.TrimSpace(gjson.Get(funcArguments, "code").String())
	if code == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 code 不能为空，请传入主题代码（如 880904）。")
		return nil
	}
	setcode := strings.TrimSpace(gjson.Get(funcArguments, "setcode").String())
	if setcode == "" {
		setcode = "2"
	}
	page := icfqsIntArg(funcArguments, "page", 1, 0)
	size := icfqsIntArg(funcArguments, "size", 30, 200)

	columns, rows, err := NewIcfqsApi().GetTopicStocks(code, setcode, page, size)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "主题成分股获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown(code+" 主题成分股（ICFQS）", columns, rows, IcfqsTopicStocksLabels, size)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetTopicKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTopicKLine", funcArguments)
	code := strings.TrimSpace(gjson.Get(funcArguments, "code").String())
	if code == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 code 不能为空，请传入主题代码（如 880904）。")
		return nil
	}
	setcode := strings.TrimSpace(gjson.Get(funcArguments, "setcode").String())
	if setcode == "" {
		setcode = "2"
	}
	limit := icfqsIntArg(funcArguments, "limit", 30, 250)

	columns, rows, err := NewIcfqsApi().GetTopicKLine(code, setcode)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "主题走势获取失败："+err.Error())
		return nil
	}
	md := IcfqsMarkdown(code+" 主题历史走势（ICFQS）", columns, rows, IcfqsTopicKLineLabels, limit)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}
