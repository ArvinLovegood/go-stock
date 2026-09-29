package data

import (
	"fmt"
	"strings"

	"go-stock/backend/logger"
	"go-stock/backend/util"

	gotdx "github.com/bensema/gotdx"
	"github.com/bensema/gotdx/types"
	"github.com/tidwall/gjson"
)

// macCallWithRetry 在 MAC 主站或扩展行情客户端上执行一次调用，失败自动重连后重试一次。
// useEx=true 时使用 macExClient（港美股/扩展板块），否则使用 macClient（A股）。
func macCallWithRetry[T any](t *TdxKLineApi, useEx bool, fn func(client *gotdx.Client) (T, error)) (T, error) {
	var zero T
	ensure := t.ensureMACClient
	reconnect := t.reconnectMAC
	mu := &t.macMu
	if useEx {
		ensure = t.ensureMACExClient
		reconnect = t.reconnectMACEx
		mu = &t.macExMu
	}
	if err := ensure(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return zero, err
	}
	current := func() *gotdx.Client {
		if useEx {
			return t.macExClient
		}
		return t.macClient
	}

	mu.Lock()
	result, err := fn(current())
	mu.Unlock()
	if err == nil {
		return result, nil
	}

	logger.SugaredLogger.Warnf("TdxKLine MAC call error: %v, reconnecting...", err)
	if reconnectErr := reconnect(); reconnectErr != nil {
		logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
		return zero, err
	}
	mu.Lock()
	result, err = fn(current())
	mu.Unlock()
	if err != nil {
		logger.SugaredLogger.Errorf("TdxKLine MAC call retry error: %v", err)
		return zero, err
	}
	return result, nil
}

// MACBoardListItem MAC 板块列表项（含板块指数与代表个股）
type MACBoardListItem struct {
	Market      uint16  `md:"-"`
	Code        string  `md:"板块代码"`
	Name        string  `md:"板块名称"`
	Price       float64 `md:"板块指数"`
	RiseSpeed   float64 `md:"涨速"`
	SymbolCode  string  `md:"代表股代码"`
	SymbolName  string  `md:"代表股名称"`
	SymbolPrice float64 `md:"代表股价格"`
}

// GetMACBoardList 获取 MAC 板块列表（板块指数、涨速与代表个股）。
// useEx=true 时走扩展行情节点（港股/美股板块），否则为 A 股板块。
func (t *TdxKLineApi) GetMACBoardList(boardType uint16, count uint32, useEx bool) *[]MACBoardListItem {
	result := &[]MACBoardListItem{}
	items, err := macCallWithRetry(t, useEx, func(client *gotdx.Client) ([]MACBoardListItem, error) {
		list, callErr := client.MACBoardList(boardType, count)
		if callErr != nil {
			return nil, callErr
		}
		converted := make([]MACBoardListItem, 0, len(list))
		for _, item := range list {
			converted = append(converted, MACBoardListItem{
				Market:      item.Market,
				Code:        item.Code,
				Name:        item.Name,
				Price:       item.Price,
				RiseSpeed:   item.RiseSpeed,
				SymbolCode:  item.SymbolCode,
				SymbolName:  item.SymbolName,
				SymbolPrice: item.SymbolPrice,
			})
		}
		return converted, nil
	})
	if err != nil {
		return result
	}
	*result = items
	return result
}

// MACBoardMemberQuote MAC 板块成分股报价
type MACBoardMemberQuote struct {
	Market       uint16  `md:"-"`
	Symbol       string  `md:"股票代码"`
	Name         string  `md:"股票名称"`
	Close        float64 `md:"最新价"`
	ChangePct    float64 `md:"涨跌幅(%)"`
	TurnoverRate float64 `md:"换手率(%)"`
	VolumeRatio  float64 `md:"量比"`
	PETTM        float64 `md:"PE(TTM)"`
	Amount       float64 `md:"成交额(元)"`
}

// GetMACBoardMemberQuotes 获取板块成分股报价（默认按涨速降序）。港股等扩展板块需用 useEx。
func (t *TdxKLineApi) GetMACBoardMemberQuotes(boardSymbol string, count uint32, sortType uint16, sortOrder uint8, useEx bool) *[]MACBoardMemberQuote {
	result := &[]MACBoardMemberQuote{}
	items, err := macCallWithRetry(t, useEx, func(client *gotdx.Client) ([]MACBoardMemberQuote, error) {
		list, callErr := client.MACBoardMembersQuotesWithSort(boardSymbol, count, sortType, sortOrder)
		if callErr != nil {
			return nil, callErr
		}
		converted := make([]MACBoardMemberQuote, 0, len(list))
		for _, item := range list {
			changePct := 0.0
			if item.PreClose > 0 {
				changePct = (item.Close - item.PreClose) / item.PreClose * 100
			}
			converted = append(converted, MACBoardMemberQuote{
				Market:       item.Market,
				Symbol:       item.Symbol,
				Name:         item.Name,
				Close:        item.Close,
				ChangePct:    changePct,
				TurnoverRate: item.TurnoverRate,
				VolumeRatio:  item.VolumeRatio,
				PETTM:        item.PETTM,
				Amount:       item.Amount,
			})
		}
		return converted, nil
	})
	if err != nil {
		return result
	}
	*result = items
	return result
}

// MACMarketMonitorItem MAC 市场监控（异动）条目
type MACMarketMonitorItem struct {
	Index       uint16 `md:"序号"`
	Market      uint16 `md:"-"`
	Code        string `md:"代码"`
	Name        string `md:"名称"`
	Time        string `md:"时间"`
	Desc        string `md:"异动描述"`
	Value       string `md:"异动值"`
	UnusualType uint8  `md:"异动类型码"`
}

// GetMACMarketMonitorData 获取 MAC 市场监控（实时异动）列表，market 取值见 types.MarketXX。
func (t *TdxKLineApi) GetMACMarketMonitorData(market uint8, count uint32) *[]MACMarketMonitorItem {
	result := &[]MACMarketMonitorItem{}
	items, err := macCallWithRetry(t, false, func(client *gotdx.Client) ([]MACMarketMonitorItem, error) {
		list, callErr := client.MACMarketMonitor(market, 0, count)
		if callErr != nil {
			return nil, callErr
		}
		converted := make([]MACMarketMonitorItem, 0, len(list))
		for _, item := range list {
			converted = append(converted, MACMarketMonitorItem{
				Index:       item.Index,
				Market:      item.Market,
				Code:        item.Code,
				Name:        item.Name,
				Time:        item.Time,
				Desc:        item.Desc,
				Value:       item.Value,
				UnusualType: item.UnusualType,
			})
		}
		return converted, nil
	})
	if err != nil {
		return result
	}
	*result = items
	return result
}

// MACSymbolInfoData MAC 单只股票盘口摘要
type MACSymbolInfoData struct {
	StockCode     string  `md:"股票代码"`
	Name          string  `md:"股票名称"`
	DateTime      string  `md:"数据时间"`
	PreClose      float64 `md:"昨收"`
	Open          float64 `md:"今开"`
	High          float64 `md:"最高"`
	Low           float64 `md:"最低"`
	Close         float64 `md:"最新"`
	ChangePct     float64 `md:"涨跌幅(%)"`
	Momentum      float64 `md:"涨速"`
	Vol           uint32  `md:"成交量"`
	Amount        float64 `md:"成交额(元)"`
	InsideVolume  uint32  `md:"内盘"`
	OutsideVolume uint32  `md:"外盘"`
	Turnover      float64 `md:"换手率(%)"`
	VR            float64 `md:"量比"`
	Avg           float64 `md:"均价"`
}

// GetMACSymbolInfoData 获取 MAC 股票摘要（含内外盘、量比、换手率等盘口指标）
func (t *TdxKLineApi) GetMACSymbolInfoData(stockCode string) *MACSymbolInfoData {
	market, code := tdxMarketFromStockCode(stockCode)
	reply, err := macCallWithRetry(t, false, func(client *gotdx.Client) (*MACSymbolInfoData, error) {
		info, callErr := client.MACSymbolInfo(market, code)
		if callErr != nil {
			return nil, callErr
		}
		if info == nil {
			return nil, fmt.Errorf("empty mac symbol info")
		}
		changePct := 0.0
		if info.PreClose > 0 {
			changePct = (info.Close - info.PreClose) / info.PreClose * 100
		}
		return &MACSymbolInfoData{
			StockCode:     stockCode,
			Name:          info.Name,
			DateTime:      info.DateTime.Format("2006-01-02 15:04:05"),
			PreClose:      info.PreClose,
			Open:          info.Open,
			High:          info.High,
			Low:           info.Low,
			Close:         info.Close,
			ChangePct:     changePct,
			Momentum:      info.Momentum,
			Vol:           info.Vol,
			Amount:        info.Amount,
			InsideVolume:  info.InsideVolume,
			OutsideVolume: info.OutsideVolume,
			Turnover:      info.Turnover,
			VR:            info.VR,
			Avg:           info.Avg,
		}, nil
	})
	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetMACSymbolInfoData(%s) error: %v", stockCode, err)
		return nil
	}
	return reply
}

// MACMarketCodes 把 sz/sh/bj/all 等市场名称映射为 MAC 市场码；返回 nil 表示取值不合法
func MACMarketCodes(name string) []uint8 {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "sz", "深圳", "0":
		return []uint8{uint8(types.MarketSZ)}
	case "sh", "上海", "1":
		return []uint8{uint8(types.MarketSH)}
	case "bj", "北京", "2":
		return []uint8{uint8(types.MarketBJ)}
	case "all", "全部", "":
		return []uint8{uint8(types.MarketSZ), uint8(types.MarketSH)}
	}
	return nil
}

// ---------- 工具处理函数 ----------

func init() {
	registerToolHandler("GetMACBoardList", handleGetMACBoardList)
	registerToolHandler("GetMACBoardMembers", handleGetMACBoardMembers)
	registerToolHandler("GetMACMarketMonitor", handleGetMACMarketMonitor)
	registerToolHandler("GetMACSymbolInfo", handleGetMACSymbolInfo)
	registerToolHandler("GetTdxTickData", handleGetTdxTickData)
	registerToolHandler("GetTdxMinuteTrend", handleGetTdxMinuteTrend)
}

func macIntArg(funcArguments, key string, defaultValue, maxValue int) int {
	value := int(gjson.Get(funcArguments, key).Int())
	if value <= 0 {
		value = defaultValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func handleGetMACBoardList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetMACBoardList", funcArguments)
	boardType := macIntArg(funcArguments, "boardType", 0, 65535)
	count := macIntArg(funcArguments, "count", 50, 300)
	market := strings.TrimSpace(gjson.Get(funcArguments, "market").String())
	useEx := strings.HasPrefix(strings.ToLower(market), "hk") || strings.HasPrefix(strings.ToLower(market), "us")

	items := NewTdxKLineApi().GetMACBoardList(uint16(boardType), uint32(count), useEx)
	if items == nil || len(*items) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"未获取到板块列表数据，请确认 boardType 与 market 参数（A股默认 boardType=0、market=a）。")
		return nil
	}
	marketName := "A股"
	if useEx {
		marketName = "扩展行情（港股/美股）"
	}
	md := util.MarkdownTableWithTitle(fmt.Sprintf("%s 板块列表（通达信MAC，共%d个）", marketName, len(*items)), *items)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetMACBoardMembers(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetMACBoardMembers", funcArguments)
	boardSymbol := strings.TrimSpace(gjson.Get(funcArguments, "boardSymbol").String())
	if boardSymbol == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 boardSymbol 不能为空，请传入板块代码（如 880761）。")
		return nil
	}
	count := macIntArg(funcArguments, "count", 30, 200)
	sortType := macIntArg(funcArguments, "sortType", 14, 65535)
	sortOrder := macIntArg(funcArguments, "sortOrder", 1, 255)
	market := strings.TrimSpace(gjson.Get(funcArguments, "market").String())
	useEx := strings.HasPrefix(strings.ToLower(market), "hk") || strings.HasPrefix(strings.ToLower(market), "us")

	items := NewTdxKLineApi().GetMACBoardMemberQuotes(boardSymbol, uint32(count), uint16(sortType), uint8(sortOrder), useEx)
	if items == nil || len(*items) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			fmt.Sprintf("未获取到板块 %s 的成分股报价，请确认板块代码是否正确（如 880761）。", boardSymbol))
		return nil
	}
	md := util.MarkdownTableWithTitle(fmt.Sprintf("板块 %s 成分股报价（通达信MAC，共%d只）", boardSymbol, len(*items)), *items)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetMACMarketMonitor(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetMACMarketMonitor", funcArguments)
	count := macIntArg(funcArguments, "count", 30, 200)
	market := strings.TrimSpace(gjson.Get(funcArguments, "market").String())
	markets := MACMarketCodes(market)
	if markets == nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"参数 market 取值不合法，可选：all（默认，深市+沪市）、sz、sh、bj。")
		return nil
	}

	api := NewTdxKLineApi()
	merged := make([]MACMarketMonitorItem, 0, count*len(markets))
	for _, m := range markets {
		items := api.GetMACMarketMonitorData(m, uint32(count))
		if items == nil {
			continue
		}
		merged = append(merged, *items...)
	}
	if len(merged) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"未获取到市场异动数据（可能为非交易时段或无符合条件的异动）。")
		return nil
	}
	md := util.MarkdownTableWithTitle(fmt.Sprintf("市场实时异动监控（通达信MAC，共%d条）", len(merged)), merged)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

func handleGetMACSymbolInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetMACSymbolInfo", funcArguments)
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。")
		return nil
	}
	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		row := api.GetMACSymbolInfoData(stockCode)
		if row == nil {
			return stockCode + "：获取MAC盘口摘要失败或无数据"
		}
		return util.MarkdownTableWithTitle(stockCode+" 盘口摘要（通达信MAC）", []MACSymbolInfoData{*row})
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

// TdxBigTradeItem 逐笔成交中的大单条目
type TdxBigTradeItem struct {
	Time      string  `md:"时间"`
	Price     float64 `md:"成交价"`
	Vol       int64   `md:"成交量(股)"`
	Amount    float64 `md:"成交额(元)"`
	BuyOrSell string  `md:"方向"`
}

func tdxTickActionText(action int) string {
	switch action {
	case 0:
		return "主动买"
	case 1:
		return "主动卖"
	default:
		return "中性"
	}
}

func handleGetTdxTickData(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTdxTickData", funcArguments)
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。")
		return nil
	}
	tradeDate := strings.TrimSpace(gjson.Get(funcArguments, "tradeDate").String())
	topN := macIntArg(funcArguments, "topN", 15, 50)

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		section, err := TdxTickSection(api, stockCode, tradeDate, topN)
		if err != nil {
			return fmt.Sprintf("%s：%v", stockCode, err)
		}
		return section
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

// TdxTickSection 生成单只股票的逐笔成交统计与大单明细 Markdown 片段；
// tradeDate 为空取当日，否则取历史日期。
func TdxTickSection(api *TdxKLineApi, stockCode, tradeDate string, topN int) (string, error) {
	var ticks *[]TdxTransactionData
	if tradeDate == "" {
		ticks = api.GetAllTransactionDataAuto(stockCode, false)
	} else {
		ticks = api.GetHistoryTransactionDataAuto(stockCode, tradeDate, false)
	}
	if ticks == nil || len(*ticks) == 0 {
		return "", fmt.Errorf("获取逐笔成交失败或无数据")
	}
	list := *ticks
	var totalVol int64
	var totalAmount, buyAmount, sellAmount float64
	bigTrades := make([]TdxBigTradeItem, 0, len(list))
	for _, item := range list {
		amount := item.Price * float64(item.Vol)
		totalVol += item.Vol
		totalAmount += amount
		switch item.BuyOrSell {
		case 0:
			buyAmount += amount
		case 1:
			sellAmount += amount
		}
		bigTrades = append(bigTrades, TdxBigTradeItem{
			Time:      item.Time,
			Price:     item.Price,
			Vol:       item.Vol,
			Amount:    amount,
			BuyOrSell: tdxTickActionText(item.BuyOrSell),
		})
	}
	sortTradesByAmountDesc(bigTrades)
	if topN > 0 && len(bigTrades) > topN {
		bigTrades = bigTrades[:topN]
	}
	buyRatio := 0.0
	if totalAmount > 0 {
		buyRatio = buyAmount / totalAmount * 100
	}
	title := stockCode
	if tradeDate != "" {
		title = fmt.Sprintf("%s %s", stockCode, tradeDate)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## %s 逐笔成交统计（通达信）\n", title))
	sb.WriteString(fmt.Sprintf("\n- 成交笔数：%d\n- 总成交量：%d 股\n- 总成交额：%.2f 元\n- 主动买成交额：%.2f 元\n- 主动卖成交额：%.2f 元\n- 主动买占比：%.2f%%\n",
		len(list), totalVol, totalAmount, buyAmount, sellAmount, buyRatio))
	sb.WriteString(util.MarkdownTableWithTitle(fmt.Sprintf("成交额最大 %d 笔", len(bigTrades)), bigTrades))
	return sb.String(), nil
}

func sortTradesByAmountDesc(items []TdxBigTradeItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Amount > items[j-1].Amount; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func handleGetTdxMinuteTrend(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetTdxMinuteTrend", funcArguments)
	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。")
		return nil
	}
	tradeDate := strings.TrimSpace(gjson.Get(funcArguments, "tradeDate").String())
	points := macIntArg(funcArguments, "points", 30, 100)

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		return TdxMinuteTrendSection(api, stockCode, tradeDate, points)
	})
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

// TdxMinuteTrendSection 生成单只股票的分时走势 Markdown 片段（按 points 等间隔采样）；
// tradeDate 为空取当日，否则取历史日期。
func TdxMinuteTrendSection(api *TdxKLineApi, stockCode, tradeDate string, points int) string {
	var bundle *TdxMinuteTimeDataBundle
	if tradeDate == "" {
		bundle = api.GetMinuteTimeDataAuto(stockCode)
	} else {
		bundle = api.GetHistoryMinuteTimeDataAuto(stockCode, tradeDate)
	}
	if bundle == nil || len(bundle.Items) == 0 {
		return stockCode + "：获取分时数据失败或无数据"
	}
	changePct := 0.0
	if bundle.PreClose > 0 {
		changePct = (bundle.Close - bundle.PreClose) / bundle.PreClose * 100
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## %s %s 分时走势（通达信）\n", stockCode, bundle.Date))
	sb.WriteString(fmt.Sprintf("\n- 昨收：%.2f　今开：%.2f　最高：%.2f　最低：%.2f　最新：%.2f　涨跌幅：%.2f%%\n- 总成交量：%d　总成交额：%.2f 元\n",
		bundle.PreClose, bundle.Open, bundle.High, bundle.Low, bundle.Close, changePct, bundle.Vol, bundle.Amount))
	sb.WriteString(util.MarkdownTableWithTitle(fmt.Sprintf("分时采样（共%d个点）", points), sampleMinuteData(bundle.Items, points)))
	return sb.String()
}

// sampleMinuteData 等间隔采样分时点，始终保留最后一个点，避免输出过长
func sampleMinuteData(items []TdxMinuteTimeData, maxPoints int) []TdxMinuteTimeData {
	if maxPoints <= 0 || len(items) <= maxPoints {
		return items
	}
	step := float64(len(items)) / float64(maxPoints)
	sampled := make([]TdxMinuteTimeData, 0, maxPoints+1)
	for i := 0; i < maxPoints; i++ {
		sampled = append(sampled, items[int(float64(i)*step)])
	}
	if last := items[len(items)-1]; last.Time != sampled[len(sampled)-1].Time {
		sampled = append(sampled, last)
	}
	return sampled
}
