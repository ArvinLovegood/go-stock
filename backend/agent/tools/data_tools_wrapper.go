package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	fakeUserAgent "github.com/lib4u/fake-useragent"
	"github.com/tidwall/gjson"
)

type DataToolWrapper struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
	handler     func(args string) (string, error)
}

func NewDataToolWrapper(name, description string, params map[string]*schema.ParameterInfo, handler func(args string) (string, error)) *DataToolWrapper {
	return &DataToolWrapper{
		name:        name,
		description: description,
		params:      params,
		handler:     handler,
	}
}

func (t *DataToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

func (t *DataToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)
	return t.handler(argumentsInJSON)
}

func thsResultToMarkdown(res map[string]any, title string) string {
	if convertor.ToString(res["code"]) != "100" {
		return "无符合条件的数据"
	}

	resData, ok := res["data"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}

	dataList, ok := result["dataList"].([]any)
	if !ok {
		return "无符合条件的数据"
	}
	columns, ok := result["columns"].([]any)
	if !ok {
		return "无符合条件的数据"
	}

	headers := map[string]string{}
	for _, v := range columns {
		d := v.(map[string]any)
		colTitle := convertor.ToString(d["title"])
		if dm := convertor.ToString(d["dateMsg"]); dm != "" {
			colTitle += "[" + dm + "]"
		}
		if u := convertor.ToString(d["unit"]); u != "" {
			colTitle += "(" + u + ")"
		}
		headers[d["key"].(string)] = colTitle
	}

	table := &[]map[string]any{}
	for _, v := range dataList {
		d := v.(map[string]any)
		row := map[string]any{}
		for key, colTitle := range headers {
			row[colTitle] = convertor.ToString(d[key])
		}
		*table = append(*table, row)
	}

	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	return "\r\n### " + title + "：\r\n" + markdownTable + "\r\n"
}

func GetAllDataTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"FilterStocks",
		"根据技术指标或者关注排名或者连涨/连跌跌天数筛选股票。支持多种K线形态和技术指标条件筛选，如MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌跌天数等。",
		map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     "string",
				Desc:     "股票名称或代码关键词搜索（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
			"macdGoldenFork": {
				Type:     "boolean",
				Desc:     "MACD金叉",
				Required: false,
			},
			"kdjGoldenFork": {
				Type:     "boolean",
				Desc:     "KDJ金叉",
				Required: false,
			},
			"breakThrough": {
				Type:     "boolean",
				Desc:     "放量突破",
				Required: false,
			},
			"lowFundsInflow": {
				Type:     "boolean",
				Desc:     "低位资金净流入",
				Required: false,
			},
			"highFundsOutflow": {
				Type:     "boolean",
				Desc:     "高位资金净流出",
				Required: false,
			},
			"breakUpMa5Days": {
				Type:     "boolean",
				Desc:     "向上突破5日均线",
				Required: false,
			},
			"longAvgArray": {
				Type:     "boolean",
				Desc:     "均线多头排列",
				Required: false,
			},
			"shortAvgArray": {
				Type:     "boolean",
				Desc:     "均线空头排列",
				Required: false,
			},
			"upperLargeVolume": {
				Type:     "boolean",
				Desc:     "连涨放量",
				Required: false,
			},
			"downNarrowVolume": {
				Type:     "boolean",
				Desc:     "下跌无量",
				Required: false,
			},
			"oneDayangLine": {
				Type:     "boolean",
				Desc:     "一根大阳线",
				Required: false,
			},
			"twoDayangLines": {
				Type:     "boolean",
				Desc:     "两根大阳线",
				Required: false,
			},
			"riseSun": {
				Type:     "boolean",
				Desc:     "旭日东升",
				Required: false,
			},
			"powerFulgun": {
				Type:     "boolean",
				Desc:     "强势多方炮",
				Required: false,
			},
			"restoreJustice": {
				Type:     "boolean",
				Desc:     "拨云见日",
				Required: false,
			},
			"down7Days": {
				Type:     "boolean",
				Desc:     "七仙女下凡(七连阴)",
				Required: false,
			},
			"upper8Days": {
				Type:     "boolean",
				Desc:     "八仙过海(八连阳)",
				Required: false,
			},
			"upper9Days": {
				Type:     "boolean",
				Desc:     "九阳神功(九连阳)",
				Required: false,
			},
			"upper4Days": {
				Type:     "boolean",
				Desc:     "四串阳",
				Required: false,
			},
			"heavenRule": {
				Type:     "boolean",
				Desc:     "天量法则",
				Required: false,
			},
			"upsideVolume": {
				Type:     "boolean",
				Desc:     "放量上攻",
				Required: false,
			},
			"bearishEngulfing": {
				Type:     "boolean",
				Desc:     "穿头破脚",
				Required: false,
			},
			"reversingHammer": {
				Type:     "boolean",
				Desc:     "倒转锤头",
				Required: false,
			},
			"shootingStar": {
				Type:     "boolean",
				Desc:     "射击之星",
				Required: false,
			},
			"eveningStar": {
				Type:     "boolean",
				Desc:     "黄昏之星",
				Required: false,
			},
			"firstDawn": {
				Type:     "boolean",
				Desc:     "曙光初现",
				Required: false,
			},
			"pregnant": {
				Type:     "boolean",
				Desc:     "身怀六甲",
				Required: false,
			},
			"blackCloudTops": {
				Type:     "boolean",
				Desc:     "乌云盖顶",
				Required: false,
			},
			"morningStar": {
				Type:     "boolean",
				Desc:     "早晨之星",
				Required: false,
			},
			"narrowFinish": {
				Type:     "boolean",
				Desc:     "窄幅整理",
				Required: false,
			},
			"uppDays": {
				Type:     "integer",
				Desc:     "人气排名连涨天数：3/5/7天及以上",
				Required: false,
			},
			"concernRank7Days": {
				Type:     "integer",
				Desc:     "7日关注排名：10/50/100名以内",
				Required: false,
			},
			"upNday": {
				Type:     "integer",
				Desc:     "连涨天数：3/5/8天及以上",
				Required: false,
			},
			"downNday": {
				Type:     "integer",
				Desc:     "连跌天数：3/5/8/10/14天及以上",
				Required: false,
			},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			indicators := models.TechnicalIndicators{
				MACDGOLDENFORK:     gjson.Get(args, "macdGoldenFork").Bool(),
				KDJGOLDENFORK:      gjson.Get(args, "kdjGoldenFork").Bool(),
				BREAKTHROUGH:       gjson.Get(args, "breakThrough").Bool(),
				LOWFUNDSINFLOW:     gjson.Get(args, "lowFundsInflow").Bool(),
				HIGHFUNDSOUTFLOW:   gjson.Get(args, "highFundsOutflow").Bool(),
				BREAKUPMA5DAYS:     gjson.Get(args, "breakUpMa5Days").Bool(),
				LONGAVGARRAY:       gjson.Get(args, "longAvgArray").Bool(),
				SHORTAVGARRAY:      gjson.Get(args, "shortAvgArray").Bool(),
				UPPERLARGEVOLUME:   gjson.Get(args, "upperLargeVolume").Bool(),
				DOWNNARROWVOLUME:   gjson.Get(args, "downNarrowVolume").Bool(),
				ONEDAYANGLINE:      gjson.Get(args, "oneDayangLine").Bool(),
				TWODAYANGLINES:     gjson.Get(args, "twoDayangLines").Bool(),
				RISESUN:            gjson.Get(args, "riseSun").Bool(),
				POWERFULGUN:        gjson.Get(args, "powerFulgun").Bool(),
				RESTOREJUSTICE:     gjson.Get(args, "restoreJustice").Bool(),
				DOWN7DAYS:          gjson.Get(args, "down7Days").Bool(),
				UPPER8DAYS:         gjson.Get(args, "upper8Days").Bool(),
				UPPER9DAYS:         gjson.Get(args, "upper9Days").Bool(),
				UPPER4DAYS:         gjson.Get(args, "upper4Days").Bool(),
				HEAVENRULE:         gjson.Get(args, "heavenRule").Bool(),
				UPSIDEVOLUME:       gjson.Get(args, "upsideVolume").Bool(),
				BEARISHENGULFING:   gjson.Get(args, "bearishEngulfing").Bool(),
				REVERSINGHAMMER:    gjson.Get(args, "reversingHammer").Bool(),
				SHOOTINGSTAR:       gjson.Get(args, "shootingStar").Bool(),
				EVENINGSTAR:        gjson.Get(args, "eveningStar").Bool(),
				FIRSTDAWN:          gjson.Get(args, "firstDawn").Bool(),
				PREGNANT:           gjson.Get(args, "pregnant").Bool(),
				BLACKCLOUDTOPS:     gjson.Get(args, "blackCloudTops").Bool(),
				MORNINGSTAR:        gjson.Get(args, "morningStar").Bool(),
				NARROWFINISH:       gjson.Get(args, "narrowFinish").Bool(),
				UPP_DAYS:           int(gjson.Get(args, "uppDays").Int()),
				CONCERN_RANK_7DAYS: int(gjson.Get(args, "concernRank7Days").Int()),
				UPNDAY:             int(gjson.Get(args, "upNday").Int()),
				DOWNNDAY:           int(gjson.Get(args, "downNday").Int()),
			}

			result := data.NewStockDataApi().GetAllStocks(page, pageSize, keyword, indicators)
			if result == nil || len(result.Result.Data) == 0 {
				return "未找到符合条件的股票", nil
			}

			type stockRow struct {
				SECUCODE         string  `md:"股票代码"`
				SECURITYNAMEABBR string  `md:"股票名称"`
				NEWPRICE         float64 `md:"最新价"`
				CHANGERATE       float64 `md:"涨跌幅(%)"`
				HIGHPRICE        float64 `md:"最高价"`
				LOWPRICE         float64 `md:"最低价"`
				VOLUME           string  `md:"成交量"`
				DEALAMOUNT       float64 `md:"成交额"`
				TURNOVERRATE     float64 `md:"换手率(%)"`
				VOLUMERATIO      float64 `md:"量比"`
				INDUSTRY         string  `md:"所属行业"`
			}

			var rows []stockRow
			for _, s := range result.Result.Data {
				newPrice, _ := convertor.ToFloat(s.NEWPRICE)
				changeRate, _ := convertor.ToFloat(s.CHANGERATE)
				highPrice, _ := convertor.ToFloat(s.HIGHPRICE)
				lowPrice, _ := convertor.ToFloat(s.LOWPRICE)
				dealAmount, _ := convertor.ToFloat(s.DEALAMOUNT)
				turnoverRate, _ := convertor.ToFloat(s.TURNOVERRATE)
				volumeRatio, _ := convertor.ToFloat(s.VOLUMERATIO)

				rows = append(rows, stockRow{
					SECUCODE:         s.SECUCODE,
					SECURITYNAMEABBR: s.SECURITYNAMEABBR,
					NEWPRICE:         newPrice,
					CHANGERATE:       changeRate,
					HIGHPRICE:        highPrice,
					LOWPRICE:         lowPrice,
					VOLUME:           convertor.ToString(s.VOLUME),
					DEALAMOUNT:       dealAmount,
					TURNOVERRATE:     turnoverRate,
					VOLUMERATIO:      volumeRatio,
					INDUSTRY:         s.INDUSTRY,
				})
			}

			summary := fmt.Sprintf("共找到 %d 只符合条件的股票，当前显示第 %d 页（每页 %d 条）", result.Result.Count, page, pageSize)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票筛选结果", rows), nil
		},
	))
	tools = append(tools, NewDataToolWrapper(
		"SearchStockByIndicators",
		"根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股。可以查询股票常用的指标，如均线，kdj,rsi,boll，macd等。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "选股条件描述，支持K线形态、技术指标、财务指标、均线、kdj、rsi、boll、macd等。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchStock(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关股票数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchBk",
		"根据自然语言查询板块/概念/指数整体数据。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "板块/概念/指数查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchBk(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关板块/概念数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchETF",
		"根据自然语言查询ETF数据。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "ETF查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchETF(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关ETF数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"InteractiveAnswer",
		"获取投资者与上市公司互动问答的数据",
		map[string]*schema.ParameterInfo{
			"page": {
				Type:     "string",
				Desc:     "分页号",
				Required: true,
			},
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
		},
		func(args string) (string, error) {
			page := gjson.Get(args, "page").String()
			pageSize := gjson.Get(args, "pageSize").String()
			keyWord := gjson.Get(args, "keyWord").String()
			pageNo, _ := convertor.ToInt(page)
			if pageNo == 0 {
				pageNo = 1
			}
			pageSizeNum, _ := convertor.ToInt(pageSize)
			if pageSizeNum == 0 {
				pageSizeNum = 50
			}
			datas := data.NewMarketNewsApi().InteractiveAnswer(int(pageNo), int(pageSizeNum), keyWord)
			content := util.MarkdownTableWithTitle("投资互动数据", datas.Results)
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockResearchReport",
		"获取市场分析师的股票研究报告。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			news := data.NewMarketNewsApi()
			res := news.StockResearchReport(stockCode, 30)
			var md strings.Builder
			for _, a := range res {
				d, ok := a.(map[string]any)
				if !ok {
					continue
				}
				infoCode, _ := d["infoCode"].(string)
				md.WriteString(news.GetIndustryReportInfo(infoCode))
			}
			out := strings.TrimSpace(md.String())
			if out == "" {
				return stockCode + "：未查询到相关研究报告。", nil
			}
			return "### " + stockCode + " 研究报告\r\n" + out, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"HotStrategyTable",
		"获取当前热门选股策略",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			result := data.NewSearchStockApi("").HotStrategyTable()
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"HotStockTable",
		"当前热门股票排名",
		map[string]*schema.ParameterInfo{
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
		},
		func(args string) (string, error) {
			pageSize := gjson.Get(args, "pageSize").String()
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}
			res := data.NewMarketNewsApi().XUEQIUHotStock(int(pageSizeNum), "10")
			md := util.MarkdownTableWithTitle("当前热门股票排名", res)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockMoneyData",
		"今日股票资金流入排名",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			res := data.NewStockDataApi().GetStockMoneyData()
			md := util.MarkdownTableWithTitle("今日个股资金流向Top50", res.Data.Diff)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMutualTop10Deal",
		"获取北向资金/南向资金十大成交股数据",
		map[string]*schema.ParameterInfo{
			"mutualType": {
				Type:     "string",
				Desc:     "通道类型：001=沪股通，002=港股通(沪)，003=深股通，004=港股通(深)",
				Required: true,
			},
			"tradeDate": {
				Type:     "string",
				Desc:     "交易日期，格式：YYYY-MM-DD",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			mutualType := gjson.Get(args, "mutualType").String()
			tradeDate := gjson.Get(args, "tradeDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page == 0 {
				page = 1
			}
			if pageSize == 0 {
				pageSize = 10
			}
			result := data.NewStockDataApi().GetMutualTop10Deal(mutualType, tradeDate, page, pageSize)
			title := mutualTypeName(mutualType) + " " + tradeDate
			md := util.MarkdownTableWithTitle(title, result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockFinancialInfo",
		"获取股票财务报表信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockFinancialInfo(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"财务报表信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockHolderNum",
		"获取股票股东人数信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockHolderNum(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"股东人数信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockHistoryMoneyData",
		"获取股票历史资金流向数据",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockHistoryMoneyData(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"历史资金流向数据", result)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockRZRQInfo",
		"获取股票融资融券信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockRZRQInfo(stockCode)
			md := util.MarkdownTableWithTitle(stockCode+" 融资融券信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetIndustryValuation",
		"获取行业/板块平均估值和中值",
		map[string]*schema.ParameterInfo{
			"bkName": {
				Type:     "string",
				Desc:     "行业/板块名称，如：半导体",
				Required: true,
			},
		},
		func(args string) (string, error) {
			bkName := gjson.Get(args, "bkName").String()
			result := data.NewStockDataApi().GetIndustryValuation(bkName)
			md := util.MarkdownTableWithTitle(bkName+"行业估值", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetNewsListData",
		"获取新闻资讯",
		map[string]*schema.ParameterInfo{
			"startTime": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23 00:00:00）",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			startTime := gjson.Get(args, "startTime").String()
			keyWord := gjson.Get(args, "keyWord").String()
			limit := int(gjson.Get(args, "limit").Int())
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if pageSize <= 0 {
				pageSize = limit
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			if page < 1 {
				page = 1
			}

			parseTime, err := time.Parse(time.DateTime, startTime)
			if err != nil {
				parseTime = time.Now().Add(-time.Hour * 24)
			}
			list, total := data.NewMarketNewsApi().GetNewsListData(keyWord, parseTime, page, pageSize)

			var md strings.Builder
			md.WriteString("### 最近新闻资讯（共 " + convertor.ToString(total) + " 条，本页 " + convertor.ToString(len(*list)) + " 条）\r\n")
			for _, d := range *list {
				md.WriteString(d.DataTime.Format(time.DateTime) + " " + d.Content + "\r\n")
			}
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GlobalStockIndexesReadable",
		"获取全球主要指数概览",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			result := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			if result == "" {
				return "暂无全球指数数据。", nil
			}
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"StockNotice",
		"获取上市公司公告列表",
		map[string]*schema.ParameterInfo{
			"stock_list": {
				Type:     "string",
				Desc:     "股票代码，多只用英文逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockList := gjson.Get(args, "stock_list").String()
			res := data.NewMarketNewsApi().StockNotice(stockList)
			if len(res) == 0 {
				return "未查询到相关上市公司公告。", nil
			}

			type row struct {
				Title      string `md:"公告标题"`
				NoticeDate string `md:"公告日期"`
				ColumnName string `md:"公告类型"`
			}
			var rows []row
			for _, a := range res {
				m, ok := a.(map[string]any)
				if !ok {
					continue
				}
				if m["columns"].([]any) != nil && len(m["columns"].([]any)) > 0 {
					columns := m["columns"].([]any)[0].(map[string]any)
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
						ColumnName: convertor.ToString(columns["column_name"]),
					})
				} else {
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
					})
				}
			}
			md := util.MarkdownTableWithTitle("上市公司公告", rows)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetSecuritiesCompanyOpinion",
		"获取券商/机构的市场分析观点",
		map[string]*schema.ParameterInfo{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间（如：2026-02-26）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			res := data.NewMarketNewsApi().GetSecuritiesCompanyOpinion(startDate, endDate)
			var md strings.Builder
			for _, d := range res.Data {
				md.WriteString(d.OpinionData + "\r\n")
			}
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetCurrentTime",
		"获取当前本地时间（含星期几）及全球市场开盘状态",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			now := time.Now()
			weekday := data.WeekdayCN(now.Weekday())
			marketStatus := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			return "当前本地时间是：" + now.Format("2006-01-02 15:04:05") + " " + weekday + "\n\n" + marketStatus, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMarketData",
		"获取市场行情数据，包括指数行情、上涨/下跌/涨停/跌停家数、涨跌分布和今日申购信息",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			return getMarketDataContent()
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"AiRecommendStocks",
		"获取近期AI分析/推荐股票明细列表",
		map[string]*schema.ParameterInfo{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间",
				Required: true,
			},
			"page": {
				Type:     "string",
				Desc:     "分页号",
				Required: true,
			},
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
		},
		func(args string) (string, error) {
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := gjson.Get(args, "page").String()
			pageSize := gjson.Get(args, "pageSize").String()
			keyWord := gjson.Get(args, "keyWord").String()

			pageNo := int64(1)
			if page != "" {
				if d, err := parseInt(page); err == nil {
					pageNo = int64(d)
				}
			}
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}

			pageData, svcErr := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&models.AiRecommendStocksQuery{
				StartDate: startDate,
				EndDate:   endDate,
				Page:      int(pageNo),
				PageSize:  int(pageSizeNum),
				StockCode: keyWord,
				StockName: keyWord,
				BkName:    keyWord,
			})
			if svcErr != nil {
				return "", svcErr
			}

			var dataExport []models.AiRecommendStocksMdExport
			for _, v := range pageData.List {
				dataExport = append(dataExport, v.ToMdExportStruct())
			}
			content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisHistory",
		"查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选）",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选）",
				Required: false,
			},
			"question": {
				Type:     "string",
				Desc:     "问题关键词搜索（可选）",
				Required: false,
			},
			"modelName": {
				Type:     "string",
				Desc:     "AI模型名称筛选（可选）",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			stockName := gjson.Get(args, "stockName").String()
			question := gjson.Get(args, "question").String()
			modelName := gjson.Get(args, "modelName").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 || pageSize > 50 {
				pageSize = 10
			}

			pageData, svcErr := data.NewAIResponseResultService().GetAIResponseResultList(models.AIResponseResultQuery{
				StockCode: stockCode,
				StockName: stockName,
				Question:  question,
				ModelName: modelName,
				StartDate: startDate,
				EndDate:   endDate,
				Page:      page,
				PageSize:  pageSize,
			})
			if svcErr != nil {
				return "", svcErr
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的历史分析报告", nil
			}

			type historyRow struct {
				ID         uint   `md:"ID"`
				StockCode  string `md:"股票代码"`
				StockName  string `md:"股票名称"`
				Question   string `md:"问题"`
				ModelName  string `md:"模型"`
				CreateTime string `md:"创建时间"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				questionText := item.Question
				if len(questionText) > 50 {
					questionText = questionText[:50] + "..."
				}
				rows = append(rows, historyRow{
					ID:         item.ID,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					Question:   questionText,
					ModelName:  item.ModelName,
					CreateTime: item.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}

			summary := fmt.Sprintf("共找到 %d 条历史分析报告，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("历史AI分析报告", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisDetail",
		"根据ID获取历史AI分析报告的详细内容",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "分析报告ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的分析报告ID", nil
			}

			var result models.AIResponseResult
			err := db.Dao.First(&result, id).Error
			if err != nil {
				return "未找到该分析报告", nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### AI分析报告详情\n\n"))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", result.StockCode))
			md.WriteString(fmt.Sprintf("| 股票名称 | %s |\n", result.StockName))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisContent",
		"根据股票代码获取最新的AI分析报告内容。直接返回该股票最近一次AI分析的完整内容。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519.SH、000001.SZ",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请提供股票代码", nil
			}

			var result models.AIResponseResult
			err := db.Dao.Where("stock_code = ?", stockCode).Order("created_at DESC").First(&result).Error
			if err != nil {
				return fmt.Sprintf("未找到 %s 的历史分析报告", stockCode), nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s (%s) AI分析报告\n\n", result.StockName, result.StockCode))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| 报告ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 分析时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SendDingDingMessage",
		"发送消息到钉钉机器人",
		map[string]*schema.ParameterInfo{
			"message": {
				Type:     "string",
				Desc:     "要发送的消息内容，支持 Markdown 格式",
				Required: true,
			},
		},
		func(args string) (string, error) {
			message := gjson.Get(args, "message").String()
			if message == "" {
				return "消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing("通知", message)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SendToDingDing",
		"将指定标题和内容以 Markdown 形式发送到钉钉机器人",
		map[string]*schema.ParameterInfo{
			"title": {
				Type:     "string",
				Desc:     "消息标题，会显示为「go-stock {title}」",
				Required: true,
			},
			"message": {
				Type:     "string",
				Desc:     "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
				Required: true,
			},
		},
		func(args string) (string, error) {
			title := gjson.Get(args, "title").String()
			message := gjson.Get(args, "message").String()
			if title == "" || message == "" {
				return "标题和消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing(title, message)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockKLine",
		"获取股票日K线数据。支持一次查询多只。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "string",
				Desc:     "日K数据条数",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := gjson.Get(args, "days").String()
			codes := parseStockCodesFromArgs(args, "stockCode")
			toIntDay := 90
			if days != "" {
				if d, err := parseInt(days); err == nil {
					toIntDay = d
				}
			}
			var allResults []map[string]any
			api := data.NewStockDataApi()
			for _, code := range codes {
				var klineData *[]data.KLineData
				if strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "sh") {
					klineData = api.GetKLineData(code, "240", int64(toIntDay))
				} else if strings.HasPrefix(code, "hk") || strings.HasPrefix(code, "us") || strings.HasPrefix(code, "gb_") {
					klineData = api.GetHK_KLineData(code, "day", int64(toIntDay))
				}
				if klineData == nil || len(*klineData) == 0 {
					fallbackResult := data.FetchKLineWithFallback(code, "", "101", toIntDay, "")
					if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
						klineData = fallbackResult.Data
					}
				}
				if klineData != nil {
					for _, k := range *klineData {
						allResults = append(allResults, map[string]any{
							"stockCode": code,
							"date":      k.Day,
							"open":      k.Open,
							"high":      k.High,
							"low":       k.Low,
							"close":     k.Close,
							"volume":    k.Volume,
						})
					}
				}
			}
			if len(allResults) == 0 {
				return "未获取到 K 线数据", nil
			}
			jsonData, _ := json.Marshal(allResults)
			markdownTable, _ := data.JSONToMarkdownTable(jsonData)
			return markdownTable, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEastMoneyKLine",
		"获取股票 K 线数据。支持日/周/月/季/年 K 线及 1/5/15/30/60 分钟线，可选前复权或后复权。股票代码格式：A股 000001.SZ、600000.SH，港股 00700.HK 等。支持一次查询多只。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"adjustFlag": {
				Type:     "string",
				Desc:     "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			adjustFlag := gjson.Get(args, "adjustFlag").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
				res := data.EastMoneyKLineSection(api, code, kLineType, adjustFlag, limit)
				results = append(results, res)
			}
			return strings.Join(results, "\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEastMoneyKLineWithMA",
		"获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
			"maPeriods": {
				Type:     "string",
				Desc:     "均线周期，逗号分隔，如 5,10,20,60。不传则默认 5,10,20,60,120",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			limit := int(gjson.Get(args, "limit").Int())
			maPeriodsStr := gjson.Get(args, "maPeriods").String()
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
				res := data.EastMoneyKLineWithMASection(api, code, kLineType, limit, maPeriodsStr)
				results = append(results, res)
			}
			return strings.Join(results, "\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"CreateAiRecommendStocks",
		"创建/保存AI推荐股票记录",
		map[string]*schema.ParameterInfo{
			"modelName": {
				Type:     "string",
				Desc:     "模型名称",
				Required: true,
			},
			"rating": {
				Type:     "string",
				Desc:     "评级(买入/增持/中性/减持/卖出)",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称",
				Required: true,
			},
			"bkName": {
				Type:     "string",
				Desc:     "板块/概念/行业名称",
				Required: true,
			},
			"stockPrice": {
				Type:     "string",
				Desc:     "推荐时股票价格",
				Required: true,
			},
			"recommendReason": {
				Type:     "string",
				Desc:     "推荐理由/驱动因素/逻辑",
				Required: true,
			},
			"recommendBuyPrice": {
				Type:     "string",
				Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendBuyPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低买入价",
				Required: true,
			},
			"recommendBuyPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高买入价",
				Required: true,
			},
			"recommendStopProfitPrice": {
				Type:     "string",
				Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendStopProfitPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低止盈价",
				Required: true,
			},
			"recommendStopProfitPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高止盈价",
				Required: true,
			},
			"recommendStopLossPrice": {
				Type:     "string",
				Desc:     "ai建议止损价",
				Required: true,
			},
			"riskRemarks": {
				Type:     "string",
				Desc:     "风险提示",
				Required: true,
			},
			"remarks": {
				Type:     "string",
				Desc:     "操作总结/备注",
				Required: true,
			},
		},
		func(args string) (string, error) {
			var recommend models.AiRecommendStocks
			if err := json.Unmarshal([]byte(args), &recommend); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().CreateAiRecommendStocks(&recommend); err != nil {
				return "保存股票推荐失败: " + err.Error(), nil
			}
			return "保存股票推荐成功", nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"BatchCreateAiRecommendStocks",
		"批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
		map[string]*schema.ParameterInfo{
			"stocks": {
				Type:     "array",
				Desc:     "股票推荐列表",
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type:     "object",
					Required: true,
					SubParams: map[string]*schema.ParameterInfo{
						"modelName": {
							Type:     "string",
							Desc:     "模型名称",
							Required: true,
						},
						"rating": {
							Type:     "string",
							Desc:     "评级(买入/增持/中性/减持/卖出)",
							Required: true,
						},
						"stockCode": {
							Type:     "string",
							Desc:     "股票代码，如：601138.SH",
							Required: true,
						},
						"stockName": {
							Type:     "string",
							Desc:     "股票名称",
							Required: true,
						},
						"bkName": {
							Type:     "string",
							Desc:     "板块/概念/行业名称",
							Required: true,
						},
						"stockPrice": {
							Type:     "string",
							Desc:     "推荐时股票价格",
							Required: true,
						},
						"recommendReason": {
							Type:     "string",
							Desc:     "推荐理由/驱动因素/逻辑",
							Required: true,
						},
						"recommendBuyPrice": {
							Type:     "string",
							Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendBuyPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低买入价",
							Required: true,
						},
						"recommendBuyPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高买入价",
							Required: true,
						},
						"recommendStopProfitPrice": {
							Type:     "string",
							Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendStopProfitPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低止盈价",
							Required: true,
						},
						"recommendStopProfitPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高止盈价",
							Required: true,
						},
						"recommendStopLossPrice": {
							Type:     "string",
							Desc:     "ai建议止损价",
							Required: true,
						},
						"riskRemarks": {
							Type:     "string",
							Desc:     "风险提示",
							Required: true,
						},
						"remarks": {
							Type:     "string",
							Desc:     "操作总结/备注",
							Required: true,
						},
					},
				},
			},
		},
		func(args string) (string, error) {
			stocks := gjson.Get(args, "stocks").String()
			var recommends []*models.AiRecommendStocks
			if err := json.Unmarshal([]byte(stocks), &recommends); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(recommends); err != nil {
				return "批量保存股票推荐失败: " + err.Error(), nil
			}
			return "批量保存股票推荐成功，共保存 " + convertor.ToString(len(recommends)) + " 条记录", nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SetTradingPrice",
		"设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 000001.SZ、600000.SH",
				Required: true,
			},
			"entryPrice": {
				Type:     "number",
				Desc:     "开仓价/买入价（目标买入价格），0 表示不设置",
				Required: true,
			},
			"takeProfitPrice": {
				Type:     "number",
				Desc:     "止盈价（预期卖出价格），0 表示不设置",
				Required: true,
			},
			"stopLossPrice": {
				Type:     "number",
				Desc:     "止损价（亏损止损价格），0 表示不设置",
				Required: true,
			},
			"costPrice": {
				Type:     "number",
				Desc:     "成本价（持仓成本价格），0 表示不设置",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			entryPrice := gjson.Get(args, "entryPrice").Float()
			takeProfitPrice := gjson.Get(args, "takeProfitPrice").Float()
			stopLossPrice := gjson.Get(args, "stopLossPrice").Float()
			costPrice := gjson.Get(args, "costPrice").Float()
			result := data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
			if result == "设置成功" {
				return fmt.Sprintf("✅ 价位线设置成功！\n\n📈 %s\n💰 开仓价：%.2f\n🎯 止盈价：%.2f\n🛑 止损价：%.2f\n💵 成本价：%.2f", stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice), nil
			}
			return "❌ 价位线设置失败：" + result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchFund",
		"搜索基金信息，支持按基金代码或名称模糊搜索",
		map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     "string",
				Desc:     "搜索关键词（基金代码或名称）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			if keyword == "" {
				return "请输入搜索关键词", nil
			}
			funds := data.NewFundApi().GetFundList(keyword)
			if len(funds) == 0 {
				return "未找到相关基金，请检查关键词是否正确", nil
			}
			type fundRow struct {
				Code        string   `md:"基金代码"`
				Name        string   `md:"基金名称"`
				Type        string   `md:"基金类型"`
				Manager     string   `md:"基金经理"`
				NetGrowth1  *float64 `md:"近1月涨幅"`
				NetGrowth3  *float64 `md:"近3月涨幅"`
				NetGrowth6  *float64 `md:"近6月涨幅"`
				NetGrowth12 *float64 `md:"近1年涨幅"`
			}
			var rows []fundRow
			for _, f := range funds {
				rows = append(rows, fundRow{
					Code:        f.Code,
					Name:        f.Name,
					Type:        f.Type,
					Manager:     f.Manager,
					NetGrowth1:  f.NetGrowth1,
					NetGrowth3:  f.NetGrowth3,
					NetGrowth6:  f.NetGrowth6,
					NetGrowth12: f.NetGrowth12,
				})
			}
			return util.MarkdownTableWithTitle("基金搜索结果", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFundInfo",
		"获取基金详细信息，包括净值、涨跌幅、评级等",
		map[string]*schema.ParameterInfo{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			fundCode := gjson.Get(args, "fundCode").String()
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			fund, err := data.NewFundApi().CrawlFundBasic(fundCode)
			if err != nil || fund.Code == "" {
				return "未找到该基金信息，请检查基金代码是否正确", nil
			}
			type fundDetailRow struct {
				Code           string   `md:"基金代码"`
				Name           string   `md:"基金名称"`
				FullName       string   `md:"基金全称"`
				Type           string   `md:"基金类型"`
				Establishment  string   `md:"成立日期"`
				Scale          string   `md:"最新规模(亿)"`
				Company        string   `md:"基金管理人"`
				Manager        string   `md:"基金经理"`
				Rating         string   `md:"基金评级"`
				TrackingTarget string   `md:"跟踪标的"`
				NetUnitValue   *float64 `md:"单位净值"`
				NetAccumulated *float64 `md:"累计净值"`
				NetGrowth1     *float64 `md:"近1月涨幅(%)"`
				NetGrowth3     *float64 `md:"近3月涨幅(%)"`
				NetGrowth6     *float64 `md:"近6月涨幅(%)"`
				NetGrowth12    *float64 `md:"近1年涨幅(%)"`
				NetGrowthYTD   *float64 `md:"今年来涨幅(%)"`
			}
			row := fundDetailRow{
				Code:           fund.Code,
				Name:           fund.Name,
				FullName:       fund.FullName,
				Type:           fund.Type,
				Establishment:  fund.Establishment,
				Scale:          fund.Scale,
				Company:        fund.Company,
				Manager:        fund.Manager,
				Rating:         fund.Rating,
				TrackingTarget: fund.TrackingTarget,
				NetUnitValue:   fund.NetUnitValue,
				NetAccumulated: fund.NetAccumulated,
				NetGrowth1:     fund.NetGrowth1,
				NetGrowth3:     fund.NetGrowth3,
				NetGrowth6:     fund.NetGrowth6,
				NetGrowth12:    fund.NetGrowth12,
				NetGrowthYTD:   fund.NetGrowthYTD,
			}
			return util.MarkdownTableWithTitle(fund.Name+" ("+fund.Code+") 基金详细信息", row), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFundKLine",
		"获取基金K线数据，支持多周期(日K/周K/月K/年K等)。场内基金(ETF/LOF)使用4层数据源fallback，场外基金从东方财富历史净值接口获取",
		map[string]*schema.ParameterInfo{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 510050(场内ETF)、000001(场外基金)",
				Required: true,
			},
			"klt": {
				Type:     "string",
				Desc:     "K线周期: 101=日K, 102=周K, 103=月K, 104=年K",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回数据条数，默认100",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fundCode := gjson.Get(args, "fundCode").String()
			klt := gjson.Get(args, "klt").String()
			limit := gjson.Get(args, "limit").Int()
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			if klt == "" {
				klt = "101"
			}
			if limit <= 0 {
				limit = 100
			}
			result := data.NewFundKLineApi().GetFundKLine(fundCode, klt, int(limit))
			if result == nil || result.Data == nil || len(*result.Data) == 0 {
				return "未获取到该基金的K线数据", nil
			}
			type klineRow struct {
				Day           string `md:"日期"`
				Open          string `md:"开盘价"`
				Close         string `md:"收盘价"`
				High          string `md:"最高价"`
				Low           string `md:"最低价"`
				Volume        string `md:"成交量"`
				ChangePercent string `md:"涨跌幅(%)"`
			}
			var rows []klineRow
			klineData := *result.Data
			startIdx := 0
			if len(klineData) > 20 {
				startIdx = len(klineData) - 20
			}
			for i := startIdx; i < len(klineData); i++ {
				item := klineData[i]
				rows = append(rows, klineRow{
					Day:           item.Day,
					Open:          item.Open,
					Close:         item.Close,
					High:          item.High,
					Low:           item.Low,
					Volume:        item.Volume,
					ChangePercent: item.ChangePercent,
				})
			}
			source := result.Source
			if source == "" {
				source = "未知"
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s K线数据(最近20条, 来源:%s, 总%d条)", fundCode, source, len(klineData)), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFundHistoryNetValue",
		"获取基金历史净值数据。场外基金从东方财富API获取，场内基金(ETF/LOF)从K线收盘价换算",
		map[string]*schema.ParameterInfo{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
			"pageIndex": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式 YYYY-MM-DD",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式 YYYY-MM-DD",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fundCode := gjson.Get(args, "fundCode").String()
			pageIndex := gjson.Get(args, "pageIndex").Int()
			pageSize := gjson.Get(args, "pageSize").Int()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			if pageIndex <= 0 {
				pageIndex = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			values, err := data.NewFundApi().GetFundHistoryNetValue(fundCode, int(pageIndex), int(pageSize), startDate, endDate)
			if err != nil {
				return fmt.Sprintf("获取基金历史净值失败: %v", err), nil
			}
			if len(values) == 0 {
				return "未获取到该基金的历史净值数据", nil
			}
			type netValueRow struct {
				Date        string  `md:"日期"`
				NetValue    float64 `md:"单位净值"`
				AccumValue  float64 `md:"累计净值"`
				DailyGrowth float64 `md:"日增长率(%)"`
			}
			var rows []netValueRow
			for _, v := range values {
				rows = append(rows, netValueRow{
					Date:        v.Date,
					NetValue:    v.NetValue,
					AccumValue:  v.AccumValue,
					DailyGrowth: v.DailyGrowth,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s 历史净值(第%d页, 每页%d条)", fundCode, pageIndex, pageSize), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFundTop10Holdings",
		"获取基金前十大重仓持股信息，包括股票代码、名称、持仓占比、实时股价和涨跌幅",
		map[string]*schema.ParameterInfo{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			fundCode := gjson.Get(args, "fundCode").String()
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			holdings, err := data.NewFundApi().GetFundTop10Holdings(fundCode)
			if err != nil {
				return fmt.Sprintf("获取基金十大持仓股失败: %v", err), nil
			}
			if len(holdings) == 0 {
				return "未获取到该基金的持仓数据", nil
			}
			type holdingRow struct {
				Rank       int      `md:"排名"`
				StockCode  string   `md:"股票代码"`
				StockName  string   `md:"股票名称"`
				Market     string   `md:"市场"`
				Ratio      float64  `md:"持仓占比(%)"`
				Price      *float64 `md:"最新价"`
				ChangeRate *float64 `md:"涨跌幅(%)"`
				Quarter    string   `md:"报告期"`
			}
			var rows []holdingRow
			for _, h := range holdings {
				rows = append(rows, holdingRow{
					Rank:       h.Rank,
					StockCode:  h.StockCode,
					StockName:  h.StockName,
					Market:     h.Market,
					Ratio:      h.Ratio,
					Price:      h.Price,
					ChangeRate: h.ChangeRate,
					Quarter:    h.Quarter,
				})
			}
			quarter := holdings[0].Quarter
			if quarter == "" {
				quarter = "最新"
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金 %s 十大重仓股(%s)", fundCode, quarter), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFollowedStocks",
		"获取用户关注/自选的股票列表",
		map[string]*schema.ParameterInfo{
			"groupId": {
				Type:     "integer",
				Desc:     "股票分组ID，不传则返回所有关注/自选的股票",
				Required: false,
			},
		},
		func(args string) (string, error) {
			groupId := int(gjson.Get(args, "groupId").Int())
			var rows []map[string]any
			if groupId > 0 {
				groupStocks := data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
				for _, gs := range groupStocks {
					stockInfo := data.NewStockDataApi().GetFollowedStockByStockCode(gs.StockCode)
					if stockInfo.StockCode != "" {
						rows = append(rows, map[string]any{
							"股票代码": stockInfo.StockCode,
							"股票名称": stockInfo.Name,
							"成本价格": stockInfo.CostPrice,
							"持仓数量": stockInfo.Volume,
						})
					}
				}
			} else {
				list := data.NewStockDataApi().GetFollowList(0)
				if list != nil {
					for _, s := range *list {
						rows = append(rows, map[string]any{
							"股票代码": s.StockCode,
							"股票名称": s.Name,
							"成本价格": s.CostPrice,
							"持仓数量": s.Volume,
						})
					}
				}
			}
			if len(rows) == 0 {
				return "暂无关注/自选的股票", nil
			}
			return util.MarkdownTableWithTitle("关注/自选的股票", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockInfo",
		"获取股票详细信息，包括实时行情、基本信息等",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码(（A股：sh,sz开头;港股hk开头,美股：us开头）)，支持多个股票代码，用逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if len(codes) == 0 {
				return "请输入股票代码", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
				if err != nil || stockData == nil || len(*stockData) == 0 {
					results = append(results, code+"：未找到股票信息")
					continue
				}
				for _, s := range *stockData {
					price, _ := convertor.ToFloat(s.Price)
					prePrice, _ := convertor.ToFloat(s.PrePrice)
					change := price - prePrice
					var pChange float64
					if prePrice > 0 {
						pChange = (price - prePrice) / prePrice * 100
					}
					content := fmt.Sprintf("### %s %s\n\n| 项目 | 值 |\n| --- | --- |\n| 股票代码 | %s |\n| 股票名称 | %s |\n| 当前价格 | %.2f |\n| 涨跌额 | %.2f |\n| 涨跌幅 | %.2f%% |\n| 成交量 | %s手 |\n| 成交额 | %s元 |\n| 今开 | %s |\n| 昨收 | %s |\n| 最高 | %s |\n| 最低 | %s |\n| 时间 | %s %s |",
						s.Name, s.Code,
						s.Code, s.Name, price, change, pChange,
						s.Volume, s.Amount,
						s.Open, s.PreClose, s.High, s.Low,
						s.Date, s.Time)
					results = append(results, content)
				}
			}
			return strings.Join(results, "\n\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetHotStockList",
		"获取雪球热门股票排行榜",
		map[string]*schema.ParameterInfo{
			"marketType": {
				Type:     "string",
				Desc:     "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)",
				Required: false,
			},
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			marketType := gjson.Get(args, "marketType").String()
			size := int(gjson.Get(args, "size").Int())
			if size <= 0 {
				size = 20
			}
			if marketType == "" {
				marketType = "10"
			}
			hotItems := data.NewMarketNewsApi().XUEQIUHotStock(size, marketType)
			if hotItems == nil || len(*hotItems) == 0 {
				return "暂无热门股票数据", nil
			}
			type hotStockRow struct {
				Rank    int     `md:"排名"`
				Code    string  `md:"股票代码"`
				Name    string  `md:"股票名称"`
				Price   float64 `md:"当前价"`
				Chg     float64 `md:"股价变化"`
				Percent float64 `md:"涨跌幅(%)"`
				Value   float64 `md:"热度"`
			}
			var rows []hotStockRow
			for i, item := range *hotItems {
				rows = append(rows, hotStockRow{
					Rank:    i + 1,
					Code:    item.Code,
					Name:    item.Name,
					Price:   item.Current,
					Chg:     item.Chg,
					Percent: item.Percent,
					Value:   item.Value,
				})
			}
			marketName := map[string]string{
				"10": "全球",
				"12": "沪深",
				"13": "港股",
				"11": "美股",
			}[marketType]
			return util.MarkdownTableWithTitle(marketName+"热门股票排行榜", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetHotEventList",
		"获取雪球热门话题/事件",
		map[string]*schema.ParameterInfo{
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			size := int(gjson.Get(args, "size").Int())
			if size <= 0 {
				size = 20
			}
			hotEvents := data.NewMarketNewsApi().HotEvent(size)
			if hotEvents == nil || len(*hotEvents) == 0 {
				return "暂无热门话题数据", nil
			}
			type hotEventRow struct {
				Rank    int    `md:"排名"`
				Tag     string `md:"话题标签"`
				Content string `md:"话题内容"`
				Hot     int    `md:"热度"`
			}
			var rows []hotEventRow
			for i, event := range *hotEvents {
				rows = append(rows, hotEventRow{
					Rank:    i + 1,
					Tag:     event.Tag,
					Content: event.Content,
					Hot:     event.Hot,
				})
			}
			return util.MarkdownTableWithTitle("雪球热门话题", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetIndustryMoneyRank",
		"获取行业资金流向排名（按行业分类）",
		map[string]*schema.ParameterInfo{
			"fenlei": {
				Type:     "string",
				Desc:     "行业分类：0=所有行业, 1=行业分类, 2=概念板块, 3=地域板块",
				Required: false,
			},
			"sort": {
				Type:     "string",
				Desc:     "排序字段：netamount=净流入, netbuy=主力净流入, change=涨跌幅",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fenlei := gjson.Get(args, "fenlei").String()
			sort := gjson.Get(args, "sort").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 20
			}
			if fenlei == "" {
				fenlei = "1"
			}
			if sort == "" {
				sort = "netamount"
			}
			rankData := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
			if len(rankData) == 0 {
				return "暂无行业资金流向数据", nil
			}
			type industryRankRow struct {
				Rank       int     `md:"排名"`
				Name       string  `md:"板块名称"`
				NetAmount  float64 `md:"净流入(万)"`
				NetBuy     float64 `md:"主力净流入(万)"`
				ChangeRate float64 `md:"涨跌幅(%)"`
			}
			var rows []industryRankRow
			for i, item := range rankData {
				if i >= limit {
					break
				}
				netAmount, _ := convertor.ToFloat(item["netamount"])
				netBuy, _ := convertor.ToFloat(item["netbuy"])
				changeRate, _ := convertor.ToFloat(item["change_rate"])
				rows = append(rows, industryRankRow{
					Rank:       i + 1,
					Name:       convertor.ToString(item["name"]),
					NetAmount:  netAmount / 10000,
					NetBuy:     netBuy / 10000,
					ChangeRate: changeRate,
				})
			}
			fenleiName := map[string]string{"0": "所有行业", "1": "行业分类", "2": "概念板块", "3": "地域板块"}[fenlei]
			return util.MarkdownTableWithTitle(fenleiName+"资金流向排名", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetLongTigerList",
		"获取龙虎榜数据（营业部排行榜）",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-03-28",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			longTigerData := data.NewMarketNewsApi().LongTiger(date)
			if longTigerData == nil || len(*longTigerData) == 0 {
				return "当日暂无龙虎榜数据", nil
			}
			type longTigerRow struct {
				Rank         int     `md:"排名"`
				Code         string  `md:"股票代码"`
				Name         string  `md:"股票名称"`
				ClosePrice   float64 `md:"收盘价"`
				ChangeRate   float64 `md:"涨跌幅(%)"`
				BizNetAmt    float64 `md:"营业部净买入(万)"`
				TurnoverRate float64 `md:"换手率(%)"`
			}
			var rows []longTigerRow
			for i, item := range *longTigerData {
				if i >= 50 {
					break
				}
				closePrice, _ := convertor.ToFloat(item.CLOSEPRICE)
				changeRate, _ := convertor.ToFloat(item.CHANGERATE)
				bizNetAmt, _ := convertor.ToFloat(item.BILLBOARDNETAMT)
				turnoverRate, _ := convertor.ToFloat(item.TURNOVERRATE)
				rows = append(rows, longTigerRow{
					Rank:         i + 1,
					Code:         item.SECURITYCODE,
					Name:         item.SECURITYNAMEABBR,
					ClosePrice:   closePrice,
					ChangeRate:   changeRate,
					BizNetAmt:    bizNetAmt / 10000,
					TurnoverRate: turnoverRate,
				})
			}
			return util.MarkdownTableWithTitle(date+" 龙虎榜数据", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEconomicData",
		"获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
		map[string]*schema.ParameterInfo{
			"dataType": {
				Type:     "string",
				Desc:     "数据类型：gdp=国内生产总值, cpi=居民消费价格指数, ppi=工业生产者出厂价格指数, pmi=采购经理指数",
				Required: true,
			},
		},
		func(args string) (string, error) {
			dataType := gjson.Get(args, "dataType").String()
			if dataType == "" {
				return "请指定数据类型", nil
			}
			api := data.NewMarketNewsApi()
			var result string
			switch dataType {
			case "gdp":
				gdp := api.GetGDP()
				if gdp == nil || len(gdp.GDPResult.Data) == 0 {
					return "暂无GDP数据", nil
				}
				type gdpRow struct {
					Time           string  `md:"时间"`
					Total          float64 `md:"国内生产总值(亿元)"`
					FirstIndustry  float64 `md:"第一产业(亿元)"`
					SecondIndustry float64 `md:"第二产业(亿元)"`
					ThirdIndustry  float64 `md:"第三产业(亿元)"`
				}
				var rows []gdpRow
				for _, d := range gdp.GDPResult.Data {
					rows = append(rows, gdpRow{
						Time:           d.TIME,
						Total:          d.DOMESTICLPRODUCTBASE / 100000000,
						FirstIndustry:  d.FIRSTPRODUCTBASE / 100000000,
						SecondIndustry: d.SECONDPRODUCTBASE / 100000000,
						ThirdIndustry:  d.THIRDPRODUCTBASE / 100000000,
					})
				}
				result = util.MarkdownTableWithTitle("国内生产总值(GDP)", rows)
			case "cpi":
				cpi := api.GetCPI()
				if cpi == nil || len(cpi.CPIResult.Data) == 0 {
					return "暂无CPI数据", nil
				}
				type cpiRow struct {
					Time         string  `md:"时间"`
					NationalBase float64 `md:"全国当月"`
					NationalSame float64 `md:"全国同比增长(%)"`
					CityBase     float64 `md:"城市当月"`
					RuralBase    float64 `md:"农村当月"`
				}
				var rows []cpiRow
				for _, d := range cpi.CPIResult.Data {
					rows = append(rows, cpiRow{
						Time:         d.TIME,
						NationalBase: d.NATIONALBASE,
						NationalSame: d.NATIONALSAME,
						CityBase:     d.CITYBASE,
						RuralBase:    d.RURALBASE,
					})
				}
				result = util.MarkdownTableWithTitle("居民消费价格指数(CPI)", rows)
			case "ppi":
				ppi := api.GetPPI()
				if ppi == nil || len(ppi.PPIResult.Data) == 0 {
					return "暂无PPI数据", nil
				}
				type ppiRow struct {
					Time  string  `md:"时间"`
					Base  float64 `md:"当月指数"`
					Same  float64 `md:"同比增长(%)"`
					Accum float64 `md:"累计指数"`
				}
				var rows []ppiRow
				for _, d := range ppi.PPIResult.Data {
					rows = append(rows, ppiRow{
						Time:  d.TIME,
						Base:  d.BASE,
						Same:  d.BASESAME,
						Accum: d.BASEACCUMULATE,
					})
				}
				result = util.MarkdownTableWithTitle("工业生产者出厂价格指数(PPI)", rows)
			case "pmi":
				pmi := api.GetPMI()
				if pmi == nil || len(pmi.PMIResult.Data) == 0 {
					return "暂无PMI数据", nil
				}
				type pmiRow struct {
					Time       string  `md:"时间"`
					MakeIndex  float64 `md:"制造业PMI"`
					MakeSame   float64 `md:"制造业同比增长(%)"`
					NMakeIndex float64 `md:"非制造业PMI"`
					NMakeSame  float64 `md:"非制造业同比增长(%)"`
				}
				var rows []pmiRow
				for _, d := range pmi.PMIResult.Data {
					rows = append(rows, pmiRow{
						Time:       d.TIME,
						MakeIndex:  d.MAKEINDEX,
						MakeSame:   d.MAKESAME,
						NMakeIndex: d.NMAKEINDEX,
						NMakeSame:  d.NMAKESAME,
					})
				}
				result = util.MarkdownTableWithTitle("采购经理指数(PMI)", rows)
			default:
				return "不支持的数据类型：" + dataType, nil
			}
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetInvestCalendar",
		"获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
		map[string]*schema.ParameterInfo{
			"yearMonth": {
				Type:     "string",
				Desc:     "年月，格式：2026-03",
				Required: false,
			},
		},
		func(args string) (string, error) {
			yearMonth := gjson.Get(args, "yearMonth").String()
			if yearMonth == "" {
				yearMonth = time.Now().Format("2006-01")
			}
			calendarData := data.NewMarketNewsApi().InvestCalendar(yearMonth)
			if len(calendarData) == 0 {
				return "当日暂无投资日历数据", nil
			}
			type calendarRow struct {
				Date      string `md:"日期"`
				Type      string `md:"事件类型"`
				StockCode string `md:"股票代码"`
				StockName string `md:"股票名称"`
				Title     string `md:"事件标题"`
			}
			var rows []calendarRow
			for _, item := range calendarData {
				if m, ok := item.(map[string]any); ok {
					date := convertor.ToString(m["date"])
					eventType := convertor.ToString(m["type"])
					title := convertor.ToString(m["title"])
					stockCode := ""
					stockName := ""
					if stock, ok := m["stock"].(map[string]any); ok {
						stockCode = convertor.ToString(stock["code"])
						stockName = convertor.ToString(stock["name"])
					}
					rows = append(rows, calendarRow{
						Date:      date,
						Type:      eventType,
						StockCode: stockCode,
						StockName: stockName,
						Title:     title,
					})
				}
			}
			return util.MarkdownTableWithTitle(yearMonth+" 投资日历", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockNotice",
		"获取个股公告信息",
		map[string]*schema.ParameterInfo{
			"stockCodes": {
				Type:     "string",
				Desc:     "股票代码列表，逗号分隔，如：600519,000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCodes := gjson.Get(args, "stockCodes").String()
			if stockCodes == "" {
				return "请输入股票代码", nil
			}
			noticeData := data.NewMarketNewsApi().StockNotice(stockCodes)
			if len(noticeData) == 0 {
				return "暂无公告数据", nil
			}
			type noticeRow struct {
				NoticeTime string `md:"公告时间"`
				Title      string `md:"公告标题"`
				Code       string `md:"股票代码"`
			}
			var rows []noticeRow
			for _, item := range noticeData {
				if m, ok := item.(map[string]any); ok {
					noticeTime := convertor.ToString(m["notice_time"])
					title := convertor.ToString(m["title"])
					code := convertor.ToString(m["secu_code"])
					rows = append(rows, noticeRow{
						NoticeTime: noticeTime,
						Title:      title,
						Code:       code,
					})
				}
			}
			return util.MarkdownTableWithTitle("个股公告", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockMinuteData",
		"获取股票分时数据（当日分钟级成交量和价格）",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			minuteData, updateTime := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
			if minuteData == nil || len(*minuteData) == 0 {
				return "未获取到分时数据", nil
			}
			type minuteRow struct {
				Time   string  `md:"时间"`
				Price  float64 `md:"价格"`
				Volume float64 `md:"成交量(手)"`
				Amount float64 `md:"成交额(元)"`
			}
			var rows []minuteRow
			for _, m := range *minuteData {
				price, _ := convertor.ToFloat(m.Price)
				volume, _ := convertor.ToFloat(m.Volume)
				amount, _ := convertor.ToFloat(m.Amount)
				rows = append(rows, minuteRow{
					Time:   m.Time,
					Price:  price,
					Volume: volume,
					Amount: amount,
				})
			}
			return fmt.Sprintf("**更新时间**: %s\n\n%s", updateTime, util.MarkdownTableWithTitle(stockCode+" 分时数据", rows)), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockConceptInfo",
		"获取股票的概念板块信息，包括概念名称、成分股数量等",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			conceptInfo := data.NewStockDataApi().GetStockConceptInfo(stockCode)
			if !conceptInfo.Success || len(conceptInfo.Result.Data) == 0 {
				return "未获取到概念板块信息", nil
			}
			type conceptRow struct {
				BoardName string  `md:"概念名称"`
				Desc      string  `md:"概念描述"`
				Yield     float64 `md:"涨跌幅(%)"`
			}
			var rows []conceptRow
			for _, c := range conceptInfo.Result.Data {
				rows = append(rows, conceptRow{
					BoardName: c.BOARDNAME,
					Desc:      c.SELECTEDBOARDREASON,
					Yield:     c.BOARDYIELD,
				})
			}
			return util.MarkdownTableWithTitle(stockCode+" 概念板块", rows), nil
		},
	))

	//tools = append(tools, NewDataToolWrapper(
	//	"WebSearch",
	//	"联网搜索工具，支持搜索最新财经新闻、股票资讯、市场动态等信息。使用此工具获取实时网络数据。",
	//	map[string]*schema.ParameterInfo{
	//		"query": {
	//			Type:     "string",
	//			Desc:     "搜索关键词/问题，如：茅台最新股价、A股今日行情、人工智能板块最新消息",
	//			Required: true,
	//		},
	//		"maxResults": {
	//			Type:     "integer",
	//			Desc:     "最大返回结果数，默认10条",
	//			Required: false,
	//		},
	//	},
	//	func(args string) (string, error) {
	//		defer data.GetBrowserManager().ResetBrowser()
	//		query := gjson.Get(args, "query").String()
	//		maxResults := int(gjson.Get(args, "maxResults").Int())
	//		if query == "" {
	//			return "请输入搜索关键词", nil
	//		}
	//		if maxResults <= 0 {
	//			maxResults = 10
	//		}
	//		searchApi := data.NewWebSearchApi(30)
	//		return searchApi.SearchToMarkdown(query, maxResults), nil
	//	},
	//))

	//tools = append(tools, NewDataToolWrapper(
	//	"SearchParams",
	//	"联网搜索股票技术指标参数、财务参数、API参数等的工具，用于查询指标的计算公式、参数设置和用法说明。",
	//	map[string]*schema.ParameterInfo{
	//		"paramName": {
	//			Type:     "string",
	//			Desc:     "参数/指标名称，如：MACD、RSI、布林带、PE、PB等技术指标或财务指标名称",
	//			Required: true,
	//		},
	//		"searchScope": {
	//			Type:     "string",
	//			Desc:     "搜索范围：technical=技术指标参数, financial=财务指标参数, api=API参数说明",
	//			Required: false,
	//		},
	//	},
	//	func(args string) (string, error) {
	//		defer data.GetBrowserManager().ResetBrowser()
	//		paramName := gjson.Get(args, "paramName").String()
	//		searchScope := gjson.Get(args, "searchScope").String()
	//		if paramName == "" {
	//			return "请输入参数名称", nil
	//		}
	//		var query string
	//		switch searchScope {
	//		case "technical":
	//			query = fmt.Sprintf("股票 %s 指标参数设置 计算公式 使用方法", paramName)
	//		case "financial":
	//			query = fmt.Sprintf("股票 %s 财务指标参数 计算公式 含义", paramName)
	//		case "api":
	//			query = fmt.Sprintf("%s API参数说明 接口文档", paramName)
	//		default:
	//			query = fmt.Sprintf("股票 %s 指标参数 计算公式 使用方法", paramName)
	//		}
	//		searchApi := data.NewWebSearchApi(30)
	//		return searchApi.SearchToMarkdown(query, 5), nil
	//	},
	//))

	tools = append(tools, NewDataToolWrapper(
		"GetStockChanges",
		"获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
		map[string]*schema.ParameterInfo{
			"changeTypes": {
				Type:     "string",
				Desc:     "异动类型，多个用逗号分隔：火箭发射=8201,快速反弹=8202,大笔买入=8193,封涨停板=4,打开跌停板=32,有大买盘=64,竞价上涨=8207,高开5日线=8209,向上缺口=8211,60日新高=8213,60日大幅上涨=8215,加速下跌=8204,高台跳水=8203,大笔卖出=8194,封跌停板=8,打开涨停板=16,有大卖盘=128,竞价下跌=8208,低开5日线=8210,向下缺口=8212,60日新低=8214,60日大幅下跌=8216。默认查询火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			changeTypesStr := gjson.Get(args, "changeTypes").String()
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if pageSize <= 0 {
				pageSize = 20
			}

			var changeTypes []int
			if changeTypesStr != "" {
				typeMap := map[string]int{
					"火箭发射": 8201, "快速反弹": 8202, "大笔买入": 8193, "封涨停板": 4,
					"打开跌停板": 32, "有大买盘": 64, "竞价上涨": 8207, "高开5日线": 8209,
					"向上缺口": 8211, "60日新高": 8213, "60日大幅上涨": 8215,
					"加速下跌": 8204, "高台跳水": 8203, "大笔卖出": 8194, "封跌停板": 8,
					"打开涨停板": 16, "有大卖盘": 128, "竞价下跌": 8208, "低开5日线": 8210,
					"向下缺口": 8212, "60日新低": 8214, "60日大幅下跌": 8216,
				}
				for _, t := range strings.Split(changeTypesStr, ",") {
					t = strings.TrimSpace(t)
					if code, ok := typeMap[t]; ok {
						changeTypes = append(changeTypes, code)
					} else if code, err := strconv.Atoi(t); err == nil {
						changeTypes = append(changeTypes, code)
					}
				}
			}

			return data.NewStockChangesApi().GetStockChangesReadable(changeTypes, 0, pageSize), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockChangeHistoryList",
		"查询股票异动历史记录。可以根据股票代码、股票名称、异动类型、日期范围、成交量、金额、涨跌幅、行业、概念等条件筛选历史异动数据。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选），支持模糊匹配",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选），支持模糊匹配",
				Required: false,
			},
			"changeType": {
				Type:     "integer",
				Desc:     "异动类型代码筛选（可选）。完整列表：8201=火箭发射,8202=快速反弹,8193=大笔买入,4=封涨停板,32=打开跌停板,64=有大买盘,8207=竞价上涨,8209=高开5日线,8211=向上缺口,8213=60日新高,8215=60日大幅上涨,8204=加速下跌,8203=高台跳水,8194=大笔卖出,8=封跌停板,16=打开涨停板,128=有大卖盘,8208=竞价下跌,8210=低开5日线,8212=向下缺口,8214=60日新低,8216=60日大幅下跌",
				Required: false,
			},
			"typeName": {
				Type:     "string",
				Desc:     "异动类型名称筛选（可选）。完整列表：火箭发射、快速反弹、大笔买入、封涨停板、打开跌停板、有大买盘、竞价上涨、高开5日线、向上缺口、60日新高、60日大幅上涨、加速下跌、高台跳水、大笔卖出、封跌停板、打开涨停板、有大卖盘、竞价下跌、低开5日线、向下缺口、60日新低、60日大幅下跌",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"startTime": {
				Type:     "string",
				Desc:     "开始时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"endTime": {
				Type:     "string",
				Desc:     "结束时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"minVolume": {
				Type:     "integer",
				Desc:     "最小成交量筛选（股），如50000表示大于500手（可选）",
				Required: false,
			},
			"minAmount": {
				Type:     "number",
				Desc:     "最小金额筛选（元），如10000000表示大于1000万（可选）",
				Required: false,
			},
			"minChangeRate": {
				Type:     "number",
				Desc:     "最小涨跌幅筛选（%），如5表示涨幅大于5%（可选）",
				Required: false,
			},
			"maxChangeRate": {
				Type:     "number",
				Desc:     "最大涨跌幅筛选（%），如-5表示跌幅小于-5%（可选）",
				Required: false,
			},
			"industry": {
				Type:     "string",
				Desc:     "行业关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"concept": {
				Type:     "string",
				Desc:     "概念关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: true,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			stockName := gjson.Get(args, "stockName").String()
			changeType := int(gjson.Get(args, "changeType").Int())
			typeName := gjson.Get(args, "typeName").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			startTime := gjson.Get(args, "startTime").String()
			endTime := gjson.Get(args, "endTime").String()
			minVolume := gjson.Get(args, "minVolume").Int()
			minAmount := gjson.Get(args, "minAmount").Float()
			minChangeRate := gjson.Get(args, "minChangeRate").Float()
			maxChangeRate := gjson.Get(args, "maxChangeRate").Float()
			industry := gjson.Get(args, "industry").String()
			concept := gjson.Get(args, "concept").String()

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			query := models.StockChangeHistoryQuery{
				StockCode:     stockCode,
				StockName:     stockName,
				ChangeType:    changeType,
				TypeName:      typeName,
				StartDate:     startDate,
				EndDate:       endDate,
				Page:          page,
				PageSize:      pageSize,
				StartTime:     startTime,
				EndTime:       endTime,
				MinVolume:     minVolume,
				MinAmount:     minAmount,
				MinChangeRate: minChangeRate,
				MaxChangeRate: maxChangeRate,
				Industry:      industry,
				Concept:       concept,
			}

			pageData, err := data.NewStockChangeHistoryService().GetHistoryList(query)
			if err != nil {
				return "", err
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的异动历史记录", nil
			}

			type historyRow struct {
				ChangeTime string  `md:"异动时间"`
				ChangeDate string  `md:"异动日期"`
				StockCode  string  `md:"股票代码"`
				StockName  string  `md:"股票名称"`
				TypeName   string  `md:"异动类型"`
				Volume     int64   `md:"成交量(股)"`
				Price      float64 `md:"价格"`
				ChangeRate float64 `md:"涨跌幅(%)"`
				Amount     float64 `md:"金额"`
				Industry   string  `md:"所属行业"`
				Concept    string  `md:"所属概念"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				rows = append(rows, historyRow{
					ChangeTime: item.ChangeTime,
					ChangeDate: item.ChangeDate,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					TypeName:   item.TypeName,
					Volume:     item.Volume,
					Price:      item.Price,
					ChangeRate: item.ChangeRate,
					Amount:     item.Amount,
					Industry:   item.Industry,
					Concept:    item.Concept,
				})
			}

			summary := fmt.Sprintf("共找到 %d 条异动历史记录，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票异动历史记录", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetDailyChangeStats",
		"获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。用于分析市场异动活跃度的变化趋势。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，如7表示近7日，30表示近30日，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return "暂无异动统计数据", nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				TotalCount int64  `md:"总异动数"`
				UpCount    int64  `md:"上涨异动"`
				DownCount  int64  `md:"下跌异动"`
				LimitUp    int64  `md:"封涨停"`
				LimitDown  int64  `md:"封跌停"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					TotalCount: d.TotalCount,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					LimitUp:    d.LimitUp,
					LimitDown:  d.LimitDown,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("近%d日每日异动统计", days), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetChangeRank",
		"获取异动次数排行榜，支持按股票、行业、概念三个维度排名，区分利好异动（封涨停板、火箭发射、快速反弹等）和利空异动（封跌停板、高台跳水、加速下跌等）。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，1=当日，3=近3日，5=近5日，10=近10日，默认1",
				Required: false,
			},
			"topN": {
				Type:     "integer",
				Desc:     "返回排名前N个，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 1
			}
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 20
			}
			result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
			if err != nil {
				return "", err
			}
			periodLabel := "当日"
			if days > 1 {
				periodLabel = fmt.Sprintf("近%d日", days)
			}
			var sb strings.Builder
			if len(result.TopStocks) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					StockName string `md:"股票名称"`
					StockCode string `md:"股票代码"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopStocks {
					rows = append(rows, row{Rank: i + 1, StockName: d.Name, StockCode: d.Code, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的股票", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopIndustries) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Industry  string `md:"行业"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopIndustries {
					rows = append(rows, row{Rank: i + 1, Industry: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的行业", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopConcepts) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Concept   string `md:"概念"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopConcepts {
					rows = append(rows, row{Rank: i + 1, Concept: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的概念", rows))
			}
			output := sb.String()
			if output == "" {
				return "暂无异动排行数据", nil
			}
			return output, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetDailyDimensionStats",
		"按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询，返回每天的利好异动数、利空异动数和总异动数。用于分析某个股票/行业/概念/异动类型在一段时间内的异动变化趋势。",
		map[string]*schema.ParameterInfo{
			"dimension": {
				Type:     "string",
				Desc:     "查询维度：stock=按股票，industry=按行业，concept=按概念，type=按异动类型",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
				Required: true,
			},
			"days": {
				Type:     "integer",
				Desc:     "查询天数，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			dimension := gjson.Get(args, "dimension").String()
			name := gjson.Get(args, "name").String()
			days := int(gjson.Get(args, "days").Int())
			if dimension == "" || name == "" {
				return "请提供dimension和name参数", nil
			}
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s[%s]的异动趋势数据", dimension, name), nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总异动数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			dimLabels := map[string]string{"stock": "股票", "industry": "行业", "concept": "概念", "type": "异动类型"}
			title := fmt.Sprintf("%s[%s]近%d日异动趋势", dimLabels[dimension], name, days)
			return util.MarkdownTableWithTitle(title, rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTypeStatsByDate",
		"查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。用于分析某天的市场异动结构。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：YYYY-MM-DD，如2025-04-13",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				return "请提供date参数", nil
			}
			result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s的异动类型分布数据", date), nil
			}
			type row struct {
				TypeName   string `md:"异动类型"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总次数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					TypeName:   d.TypeName,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("%s异动类型分布", date), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryIwencai",
		"同花顺问财行情数据查询。支持自然语言查询股票、ETF、指数等实时价格、涨跌幅、成交量、主力资金流向、大小单、技术指标等行情数据。当用户询问股票价格、ETF行情、指数行情、涨跌幅、成交量、资金流向、技术指标等行情数据查询问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺最新价格、主力资金流向、上证指数行情、连续涨停的股票等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchReport",
		"研报搜索。搜索主流投研机构发布的研究报告，获取专业分析逻辑、投资评级、目标价等重要投研决策信息。当用户询问研究报告、研报、投资评级、目标价、行业分析报告、公司深度分析等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "搜索关键词，如：人工智能行业研究报告、特斯拉投资评级、芯片行业深度分析等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索关键词", nil
			}
			result := data.NewIwencaiAPI().SearchReportToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryInsResearch",
		"机构研究与评级查询。查询研报评级、业绩预测、ESG评级、信用评级、主体评级、基金评级、券商金股等机构观点数据。支持自然语言问句输入。当用户询问研报评级、业绩预测、ESG评级、信用评级、主体评级、基金评级、券商金股等机构研究数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺研报评级、业绩预测、券商金股、ESG评级等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryZhishu",
		"指数数据查询。查询上证指数、沪深300、创业板指、恒生指数、纳斯达克指数等指数行情数据，支持涨跌幅、成交量、点位等指标查询。当用户询问指数数据、上证指数、沪深300、创业板指、恒生指数、纳斯达克指数、指数行情、指数涨跌幅、指数点位等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：上证指数涨跌幅、沪深300最新点位、创业板指成交量等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryEvent",
		"事件数据查询。查询个股业绩预告、增发配股、股权质押、限售解禁、机构调研、监管函、股东大会等事件数据。当用户询问业绩预告、增发配股、股权质押、限售解禁、机构调研、监管函等事件数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺业绩预告、最近的增发配股、机构调研记录等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchNews",
		"财经新闻搜索。搜索财经领域新闻资讯，覆盖官媒、主流财经媒体、垂直行业网站等，帮助了解最新财经事件、政策动态、行业革新、企业业务进展。当用户询问财经新闻、最新动态、政策变化、行业趋势等新闻资讯问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "搜索关键词，如：人工智能最新动态、央行货币政策、芯片行业新闻等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索关键词", nil
			}
			result := data.NewIwencaiAPI().SearchNewsToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchInvestor",
		"投资者关系活动搜索。搜索上市公司投资者关系活动记录，包括业绩说明会、路演活动、投资者调研、分析师会议等投关活动信息，获取公司管理层对业务发展、战略规划、行业前景等关键问题的回应。当用户询问投资者关系活动、业绩说明会、路演、投资者调研、分析师会议、投关活动等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "搜索关键词，如：贵州茅台投资者关系活动、宁德时代业绩说明会、芯片行业投资者调研等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索关键词", nil
			}
			result := data.NewIwencaiAPI().SearchInvestorToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectAStock",
		"A股智能选股。通过自然语言查询进行A股股票筛选，支持行情指标（股价、涨跌幅、成交量等）、技术形态（均线多头、突破新高、K线形态等）、财务指标（营收、利润、PE、PB等）、行业概念（科技、医药、消费等）等多条件组合筛选。当用户需要进行股票筛选、选股、条件选股时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言选股条件，如：今日涨跌幅超过5%的A股、均线多头的科技股、PE小于20且营收增长超过30%的股票等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入选股条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryMacro",
		"宏观数据查询。查询GDP、CPI、PPI、利率、汇率、社融、M2、PMI、工业增加值、消费、投资、进出口等宏观经济指标数据。当用户询问宏观经济数据、GDP、CPI、PPI、利率、汇率、社融、M2、PMI等宏观经济指标时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：2024年中国GDP、最近一期CPI、LPR利率、M2增速等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectSector",
		"板块智能筛选。通过自然语言查询筛选市场板块，支持行业估值（PE、PB、估值分位等）、资金流向（主力资金净流入、北向资金等）、涨跌幅、板块类型（行业板块、概念板块、地域板块等）、成交量等多条件组合筛选。当用户需要进行板块筛选、选板块、板块排行时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：今日涨幅最大的板块、主力资金净流入的板块、PE最低的行业板块等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryBasicInfo",
		"基本资料查询。查询全品类标的（股票、指数、基金、期货、期权、转债、债券、理财、保险等）的基础信息、发行主体、机构资料、费率、上市地点、上市日期等静态信息。当用户询问股票基本信息、基金资料、期货合约信息、债券资料、费率信息、上市日期等基本资料时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺上市日期、基金费率、期货合约详情、可转债基本信息等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryFinance",
		"财务数据查询。查询全市场个股营业收入、净利润、毛利率、净利率、ROE、ROA、负债率、现金流、市盈率、市净率、市销率等财务指标。当用户询问股票财务指标、营业收入、净利润、ROE、负债率、现金流、毛利率、净利率等财务数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺营业收入、ROE最高的股票、负债率最低的行业等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryIndustry",
		"行业数据查询。查询行业估值、行业财务指标、行业盈利数据、行业行情数据、板块排名等行业维度数据，支持自然语言问句输入。当用户询问行业数据、行业估值、行业排名、行业财务、行业盈利、行业行情、板块排名等行业相关问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：A股行业估值排名、银行业盈利数据、新能源板块行情等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryFutures",
		"期货期权数据查询。查询期货期权的行情数据（价格、涨跌幅、成交量等）、波动率数据（隐含波动率、历史波动率等）、产销数据（库存、产量、销量等）、会员持仓数据（持仓量、持仓变化等）、会员榜单数据（成交量排行、持仓量排行等）、行权数据（行权价、行权量等）。当用户询问期货行情、期权波动率、期货持仓、期货产销、会员持仓、行权等期货期权数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：沪铜期货最新行情、50ETF期权隐含波动率、螺纹钢期货会员持仓排名等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectETF",
		"ETF智能筛选。通过自然语言查询筛选ETF，支持行情指标（价格、涨跌幅、成交量、换手率等）、跟踪指数（沪深300、中证500、上证50、创业板指等）、基本面（估值、费率、跟踪误差等）、规模（资产规模、份额变化等）、风格类型（成长、价值、平衡等）多条件组合筛选。当用户需要筛选ETF、选ETF、查询ETF时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：沪深300ETF有哪些、规模最大的ETF、创业板ETF等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryManagement",
		"公司股东股本查询。查询股本结构（总股本、流通股本、限售股本等）、股权结构、股东户数、前十大股东/流通股东、主要持有人、实控人信息、股权质押情况、高管信息（董事会、监事会、高管团队等）。当用户询问股本结构、股东户数、前十大股东、实控人、股权质押、高管等股东股本数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺股本结构、前十大股东、实控人信息、股东户数变化等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryStockConnect",
		"沪深港通资金流查询。查询北向资金（沪股通、深股通）和南向资金（港股通）的净流入流出、个股资金流向、板块资金配置、北向持股变动、AH溢价指数等沪深港通资金流数据。当用户询问北向资金、南向资金、沪深港通、沪股通、深股通、港股通、外资流入、AH溢价等资金流问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：今日北向资金净流入、沪深港通个股资金流向、北向持股变动、AH溢价指数等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchAnnouncement",
		"公告搜索。搜索A股、港股、基金、ETF等金融标的公告，公告类型包括定期财务报告、分红派息、回购增持、资产重组等。当用户询问公司公告、分红公告、回购公告、重组公告、定期报告等公告信息时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "搜索关键词，如：贵州茅台分红公告、宁德时代回购公告、资产重组公告等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索关键词", nil
			}
			result := data.NewIwencaiAPI().SearchAnnouncementToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectFundManager",
		"智能选基金经理。根据历史业绩、管理规模、投资风格、风险控制等维度筛选公募基金经理，返回符合条件的相关基金经理数据。当用户询问基金经理筛选、基金经理排名、基金经理业绩等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：管理规模最大的基金经理、近三年业绩最好的基金经理、投资风格偏价值的基金经理等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectConvertibleBond",
		"智能选可转债。通过转股溢价率、正股表现、评级、剩余期限等多条件组合筛选可转债，返回符合条件的相关可转债数据。当用户询问可转债筛选、可转债溢价率、可转债评级等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：转股溢价率低于10%的可转债、AAA级可转债、剩余期限3年内的可转债等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectFundCompany",
		"智能选基金公司。根据管理规模、旗下产品业绩、投研实力、风险评级等维度筛选公募基金公司，返回符合条件的相关基金公司数据。当用户询问基金公司筛选、基金公司排名、基金公司规模等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：规模最大的基金公司、业绩最好的基金公司、头部基金公司等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectFund",
		"智能选基金。根据基金类型、业绩、基金经理、风险、持仓、资产配置等维度筛选公募基金，返回符合条件的相关基金数据。当用户询问基金筛选、选基金、基金排名等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：股票型基金有哪些、近一年收益率最高的基金、百亿规模基金等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectFuturesOption",
		"智能选期货期权。通过行情、波动率、产销、会员持仓、会员榜单、行权等多条件组合筛选期货期权，返回符合条件的相关期货期权数据。当用户询问期货筛选、期权筛选、期货期权组合等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：原油期货有哪些、黄金期货行情、多头持仓的期货等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectHKStock",
		"智能选港股。通过自然语言查询进行港股筛选，支持行情指标、财务指标、行业概念、陆港通等多条件组合筛选，返回符合条件的相关港股数据。当用户询问港股筛选、选港股、港股排行等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：港股科技股有哪些、港股银行股、北向资金增持的港股等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SelectUSStock",
		"智能选美股。通过自然语言查询进行美股筛选，支持行情指标、财务指标、行业概念、业绩预测、研报评级等多条件组合筛选，返回符合条件的相关美股数据。当用户询问美股筛选、选美股、美股排行等问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言筛选条件，如：美股科技股有哪些、评级买入的美股、美股市盈率低于20等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入筛选条件", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryFundFinance",
		"基金理财查询。对基金做业绩、持仓、风险、评级、获奖、基金经理、基金公司综合分析，支持自然语言问句输入，返回相关基金理财数据结果。当用户询问基金查询、基金业绩、基金持仓、基金风险、基金评级、基金获奖、基金经理、基金公司分析等基金理财相关问题时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：业绩最好的基金有哪些、基金持仓明细、基金风险评级等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"QueryBusinessData",
		"公司经营数据查询。查询主营业务构成、主要客户、供应商、参控股公司、股权投资、重大合同等经营相关数据，支持自然语言问句输入，返回相关经营数据结果。当用户询问主营业务构成、主要客户、供应商信息、参控股公司、股权投资、重大合同等经营数据时使用此工具。数据来源于同花顺问财。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询语句，如：同花顺主营业务构成、主要客户、供应商信息、参控股公司等",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "分页页码，默认1",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			page := int(gjson.Get(args, "page").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if query == "" {
				return "请输入查询语句", nil
			}
			if page <= 0 {
				page = 1
			}
			if limit <= 0 {
				limit = 10
			}
			result := data.NewIwencaiAPI().QueryToMarkdown(query, page, limit)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"StockEarningsReview",
		"个股业绩点评。获取上市公司业绩点评报告，包含营收分析、利润分析、财务指标解读等深度内容。支持沪深京港美市场股票。当用户询问个股业绩点评、财报分析、业绩报告、营收利润分析等问题时使用此工具。数据来源于东方财富AI。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "股票名称或代码，如：贵州茅台、600519、宁德时代等",
				Required: true,
			},
			"reportDate": {
				Type:     "string",
				Desc:     "报告期，格式YYYY-MM-DD，如：2024-12-31。不填则使用最新报告期",
				Required: false,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			reportDate := gjson.Get(args, "reportDate").String()
			if query == "" {
				return "请输入股票名称或代码", nil
			}
			result := data.NewEmAPI().EarningsReviewToMarkdown(query, reportDate)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"FinancialQA",
		"金融智能问答。基于东方财富权威金融数据库，覆盖数据查询、资讯搜索、宏观分析、选股选基、金融百科、市场分析、热点解读等全链条智能问答服务。支持标准模式和深度思考模式。当用户提出自然语言金融问题，如'帮我查一下'、'分析一下'、'选股'、'XX怎么样'、'XX是什么'、'最新政策'、'宏观数据'等问答类请求时使用此工具。数据来源于东方财富AI。",
		map[string]*schema.ParameterInfo{
			"question": {
				Type:     "string",
				Desc:     "用户自然语言问题，如：今天A股市场表现如何、贵州茅台最新估值、近三年ROE最高的消费股有哪些等",
				Required: true,
			},
			"deepThink": {
				Type:     "boolean",
				Desc:     "是否开启深度思考模式，当用户明确要求深度分析、详细分析、仔细想想时设为true",
				Required: false,
			},
		},
		func(args string) (string, error) {
			question := gjson.Get(args, "question").String()
			deepThink := gjson.Get(args, "deepThink").Bool()
			if question == "" {
				return "请输入您想问的问题", nil
			}
			result := data.NewEmAPI().FinancialQAToMarkdown(question, deepThink)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"IndustryResearch",
		"行业研究报告生成。根据行业关键词生成深度行业研究报告，包含行业概况、市场规模、竞争格局、发展趋势、投资建议等内容。当用户要求生成行业研究报告、行业深度分析、产业分析、行业趋势分析等时使用此工具。数据来源于东方财富AI。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "行业关键词，如：半导体、新能源汽车、AI芯片、消费电子等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入行业关键词", nil
			}
			result := data.NewEmAPI().IndustryResearchToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"TrackingReport",
		"个股/行业跟踪报告。根据用户输入的股票或行业关键词，生成跟踪报告，包含最新动态、核心观点、关键指标变化、重要事件梳理等内容。支持A股、港股、美股及行业板块。当用户要求生成跟踪报告、最新动态跟踪、个股跟踪、行业跟踪等时使用此工具。数据来源于东方财富AI。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "股票名称/代码或行业关键词，如：贵州茅台、600519、半导体行业等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入股票名称/代码或行业关键词", nil
			}
			result := data.NewEmAPI().TrackingReportToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"FinanceDataQuery",
		"金融数据查询。基于东方财富数据库，支持自然语言查询金融结构化数据，覆盖A股、港股、美股、基金、债券等多种资产，包含实时行情、公司信息、估值指标、财务报表等。单次查询最多支持5个实体。当用户需要查询具体的金融数据、指标数值、财务数据、行情数据等结构化数据时使用此工具。数据来源于东方财富妙想。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言查询，如：贵州茅台最近一年的营业收入和净利润、沪深300当前点位和成交额、东方财富和拼多多最近一年的营收等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入查询内容", nil
			}
			result := data.NewEmAPI().FinanceDataQueryToMarkdown(query)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"FinanceSearch",
		"金融资讯搜索。基于东方财富数据库，支持自然语言搜索全网最新公告、研报、财经新闻、交易所动态及官方政策等，覆盖全球市场标的。适用于热点捕捉、舆情监控、研报速览、公告精读及投资决策等场景。当用户需要搜索最新金融资讯、新闻、公告、研报等文本类信息时使用此工具。数据来源于东方财富妙想。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "自然语言搜索查询，如：格力电器最新研报与公告、商业航天板块近期新闻、美联储加息对A股影响等",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入搜索内容", nil
			}
			result := data.NewEmAPI().FinanceSearchToMarkdown(query)
			return result, nil
		},
	))

	f10Tools := []struct {
		name      string
		desc      string
		paramDesc string
		handler   func(string) string
	}{
		{"GetStockLatestFinance", "获取股票最新财务主要数据，包括每股收益(EPS)、每股净资产(BPS)、净资产收益率(ROE)、营业收入、净利润及同比/环比增速等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockLatestFinanceToMarkdown},
		{"GetStockQtrMainFinance", "获取股票季度主要财务指标，包括EPS、BPS、营业收入、净利润、同比增长率、ROE、毛利率等按季度列示。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockQtrMainFinanceToMarkdown},
		{"GetStockOrgPredict", "获取股票机构预测数据，包括各券商/机构对未来数年的EPS和PE预测明细。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockOrgPredictToMarkdown},
		{"GetStockPredictSummary", "获取股票机构预测汇总，按年度汇总多家机构的EPS预测均值、增长率和PE估值。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockPredictSummaryToMarkdown},
		{"GetStockValuationPercentile", "获取股票估值百分位数据，展示当前PE在历史30%/50%/70%分位的值，判断估值高低。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockValuationPercentileToMarkdown},
		{"GetStockMarginTrading", "获取股票融资融券数据，包括融资买入额、融资余额、融券卖出量、融券余额等按日列示。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockMarginTradingToMarkdown},
		{"GetStockBlockTrade", "获取股票大宗交易数据，包括成交价、溢价率、成交金额、买方/卖方营业部等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockBlockTradeToMarkdown},
		{"GetStockHolderTrend", "获取股票户均持股趋势数据，展示股东户数和户均持股数量随时间的变化趋势。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockHolderTrendToMarkdown},
		{"GetStockBillboard", "获取股票龙虎榜数据，包括上榜日期、上榜原因、买入/卖出总额等。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockBillboardToMarkdown},
		{"GetStockOperationDeptTrade", "获取股票营业部买卖明细，展示各营业部在龙虎榜上的买入/卖出金额和占比。数据来源于东方财富F10。", "股票代码，如 600519、000001.SZ", data.NewStockDataApi().GetStockOperationDeptTradeToMarkdown},
	}

	for _, t := range f10Tools {
		tool := t
		tools = append(tools, NewDataToolWrapper(
			tool.name,
			tool.desc,
			map[string]*schema.ParameterInfo{
				"stockCode": {
					Type:     "string",
					Desc:     tool.paramDesc,
					Required: true,
				},
			},
			func(args string) (string, error) {
				stockCode := gjson.Get(args, "stockCode").String()
				if stockCode == "" {
					return "请输入股票代码", nil
				}
				return tool.handler(stockCode), nil
			},
		))
	}

	tools = append(tools, NewDataToolWrapper(
		"ComparableCompanyAnalysis",
		"可比公司分析(东方财富妙想)。对指定公司进行可比公司分析，包括财务指标对比和估值对比，帮助判断公司相对估值水平。",
		map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "公司名称或股票代码，如：贵州茅台、东方财富",
				Required: true,
			},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				return "请输入公司名称或股票代码", nil
			}
			return data.NewEmAPI().ComparableCompanyAnalysisToMarkdown(query), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"HotspotDiscovery",
		"市场热点发现(东方财富妙想)。发现当前A股市场热点板块和题材，包括热点逻辑分析和相关个股。",
		map[string]*schema.ParameterInfo{
			"question": {
				Type:     "string",
				Desc:     "热点的自然语言描述，如：今日热点、新能源热点、AI概念热点",
				Required: true,
			},
		},
		func(args string) (string, error) {
			question := gjson.Get(args, "question").String()
			if question == "" {
				return "请输入热点描述", nil
			}
			return data.NewEmAPI().HotspotDiscoveryToMarkdown(question), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetUplimitLadder",
		"获取连板梯队数据，包括连板统计（各层级数量）和连板梯队详情（最高连板到首板各层级的股票列表，含代码、名称、封单比、成交额、市值、概念板块等）。适用于分析连板高度、市场情绪、龙头股识别等场景。当用户提到连板、梯队、连板高度、最高板等关键词时使用此工具。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 连板梯队\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			stocksStr, _ := dataMap["stocks"].(string)
			stockList := strings.Split(stocksStr, ",")
			ztCount := 0
			for _, s := range stockList {
				if strings.TrimSpace(s) != "" {
					ztCount++
				}
			}
			maxCount, _ := dataMap["max_count"].(float64)
			sb.WriteString(fmt.Sprintf("**涨停总数**: %d只 | **最高连板**: %d\n\n", ztCount, int(maxCount)))
			banInfo, _ := dataMap["ban_info"].(map[string]any)
			if len(banInfo) > 0 {
				sb.WriteString("## 连板统计\n\n")
				sb.WriteString("| 连板层级 | 数量 |\n|:---:|:---:|\n")
				for i := int(maxCount); i >= 1; i-- {
					if info, ok := banInfo[fmt.Sprintf("%d", i)].(map[string]any); ok {
						cnt, _ := info["count"].(float64)
						sb.WriteString(fmt.Sprintf("| %d连板 | %d |\n", i, int(cnt)))
					}
				}
				sb.WriteString("\n")
			}
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			if len(banInfo) > 0 && len(plateStocks) > 0 {
				sb.WriteString("## 连板梯队详情\n\n")
				for i := int(maxCount); i >= 1; i-- {
					if info, ok := banInfo[fmt.Sprintf("%d", i)].(map[string]any); ok {
						cnt, _ := info["count"].(float64)
						if int(cnt) == 0 {
							continue
						}
						sb.WriteString(fmt.Sprintf("### %d连板（%d只）\n\n", i, int(cnt)))
						sb.WriteString("| 代码 | 名称 | 类型 | 描述 | 时间 | 封单比 | 收盘封单 | 成交额 | 市值 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
						seen := make(map[string]bool)
						for _, pStocks := range plateStocks {
							for _, s := range pStocks.([]any) {
								sm, _ := s.(map[string]any)
								keepTimes, _ := sm["up_limit_keep_times"].(float64)
								if int(keepTimes) != i {
									continue
								}
								sCode, _ := sm["stock_code"].(string)
								if seen[sCode] {
									continue
								}
								seen[sCode] = true
								sName, _ := sm["stock_name"].(string)
								upType, _ := sm["up_limit_type"].(string)
								upDesc, _ := sm["up_limit_desc"].(string)
								upTime, _ := sm["up_limit_time"].(string)
								fdMax := floatOrDefault(sm["fd_max"])
								fdClose := floatOrDefault(sm["fd_close"])
								amount := floatOrDefault(sm["amount"])
								marketC := floatOrDefault(sm["market_c"])
								platesStr := getPlatesStr(stockInfo, sCode)
								sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %.2f%% | %.2f%% | %.2f亿 | %.2f亿 | %s |\n",
									sCode, sName, upType, upDesc, upTime, fdMax, fdClose, amount, marketC, platesStr))
							}
						}
						sb.WriteString("\n")
					}
				}
			}
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetWallstreetcnLives",
		"获取华尔街见闻实时快讯。支持全球7x24、A股、美股、港股、外汇、商品、黄金、原油、债券、加密货币等频道。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*schema.ParameterInfo{
			"channel": {
				Type:     "string",
				Desc:     "频道：global-channel=全球7x24, a-stock-channel=A股, us-stock-channel=美股, hk-stock-channel=港股, forex-channel=外汇, commodity-channel=商品, goldc-channel=黄金, oil-channel=原油, bond-channel=债券, crypto-channel=加密货币",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "条数，默认20，最大50",
				Required: false,
			},
		},
		func(args string) (string, error) {
			channel := gjson.Get(args, "channel").String()
			limit := int(gjson.Get(args, "limit").Int())
			if channel == "" {
				channel = "global-channel"
			}
			if limit <= 0 {
				limit = 20
			}
			return data.NewWallstreetcnApi().GetLivesReadable(channel, limit), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetWallstreetcnMarketReal",
		"获取华尔街见闻全球实时行情报价。包含美元指数、欧元/美元、美元/日元、离岸人民币、现货黄金、WTI原油等品种。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*schema.ParameterInfo{
			"prodCodes": {
				Type:     "string",
				Desc:     "品种代码(逗号分隔)，可选：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油。留空返回全部。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			prodCodesStr := gjson.Get(args, "prodCodes").String()
			var prodCodes []string
			if prodCodesStr != "" {
				prodCodes = strings.Split(prodCodesStr, ",")
			}
			return data.NewWallstreetcnApi().GetMarketRealReadable(prodCodes), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetWallstreetcnKline",
		"获取华尔街见闻K线数据。支持美元指数、外汇、黄金、原油等品种。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*schema.ParameterInfo{
			"prodCode": {
				Type:     "string",
				Desc:     "品种代码：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油",
				Required: true,
			},
			"periodType": {
				Type:     "integer",
				Desc:     "K线周期(秒)：60=1分钟, 300=5分钟, 900=15分钟, 1800=30分钟, 3600=1小时, 14400=4小时, 86400=日线",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "K线条数，默认50",
				Required: false,
			},
		},
		func(args string) (string, error) {
			prodCode := gjson.Get(args, "prodCode").String()
			periodType := int(gjson.Get(args, "periodType").Int())
			limit := int(gjson.Get(args, "limit").Int())
			if prodCode == "" {
				prodCode = "XAUUSD.OTC"
			}
			if periodType <= 0 {
				periodType = 300
			}
			if limit <= 0 {
				limit = 50
			}
			return data.NewWallstreetcnApi().GetKlineReadable(prodCode, periodType, limit), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetWallstreetcnCalendar",
		"获取华尔街见闻财经日历。包含全球重要经济数据公布时间、预期值、前值等。数据来源：华尔街见闻(wallstreetcn.com)。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "查看未来几天内的财经日历，默认3天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 3
			}
			return data.NewWallstreetcnApi().GetCalendarReadable(days), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetUplimitHotPlates",
		"获取涨停热门板块排名和接力主线数据，包括板块热度得分、涨停数、炸板数、接力主线板块等。适用于分析板块轮动、市场热点方向、主线题材等场景。当用户提到热门板块、板块热度、板块轮动、主线题材、接力板块等关键词时使用此工具。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 热门板块\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			plateStocksZb, _ := dataMap["plate_stocks_zb"].(map[string]any)
			plateArr, _ := dataMap["plate"].([]any)
			if len(plateArr) > 0 {
				sb.WriteString("## 热门板块TOP20\n\n")
				sb.WriteString("| 排名 | 板块 | 热度得分 | 涨停数 | 炸板数 |\n|:---:|:---:|:---:|:---:|:---:|\n")
				for idx, p := range plateArr {
					if arr, ok := p.([]any); ok && len(arr) >= 3 {
						name, _ := arr[0].(string)
						pCode, _ := arr[1].(string)
						score, _ := arr[2].(float64)
						ztN := 0
						if ps, ok := plateStocks[pCode].([]any); ok {
							ztN = len(ps)
						}
						zbN := 0
						if ps, ok := plateStocksZb[pCode].([]any); ok {
							zbN = len(ps)
						}
						sb.WriteString(fmt.Sprintf("| %d | %s | %d | %d | %d |\n", idx+1, name, int(score), ztN, zbN))
					}
				}
				sb.WriteString("\n")
			}
			plateInfo, _ := dataMap["plate_info"].(map[string]any)
			relay, _ := dataMap["relay"].(map[string]any)
			if area, ok := relay["area"].([]any); ok && len(area) > 0 {
				sb.WriteString("## 接力主线\n\n")
				sb.WriteString("| 板块 | 热度 | 涨停数 |\n|:---:|:---:|:---:|\n")
				for _, a := range area {
					am, _ := a.(map[string]any)
					pCode, _ := am["p_code"].(string)
					pScore, _ := am["p_score"].(float64)
					count, _ := am["count"].(float64)
					pName := pCode
					if pi, ok := plateInfo[pCode].(map[string]any); ok {
						pName, _ = pi["name"].(string)
					}
					sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", pName, int(pScore), int(count)))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetUplimitHotStocks",
		"获取涨停个股热度排行数据，包括股票代码、名称、热度得分、概念板块等。适用于分析个股受关注程度、市场人气股、热门标的等场景。当用户提到个股热度、热门个股、人气股、关注度等关键词时使用此工具。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "返回数量，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 30
			}
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 个股热度排行\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			stocksHot, _ := dataMap["stocks_hot"].(map[string]any)
			hotN, _ := dataMap["stocks_hot_n"].(float64)
			if len(stocksHot) == 0 {
				sb.WriteString("暂无个股热度数据\n")
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("热度≥%d为超级热门\n\n", int(hotN)))
			type hotItem struct {
				code  string
				score float64
			}
			var hotList []hotItem
			for code, score := range stocksHot {
				s, _ := score.(float64)
				hotList = append(hotList, hotItem{code, s})
			}
			sort.Slice(hotList, func(i, j int) bool {
				return hotList[i].score > hotList[j].score
			})
			plateStocks, _ := dataMap["plate_stocks"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			sb.WriteString("| 排名 | 代码 | 名称 | 热度 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|\n")
			for idx, item := range hotList {
				if idx >= limit {
					break
				}
				platesStr := getPlatesStr(stockInfo, item.code)
				sName := getStockNameFromPlateStocks(plateStocks, item.code)
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %d | %s |\n", idx+1, item.code, sName, int(item.score), platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetUplimitExplodedStocks",
		"获取炸板股数据，即曾经涨停但未能封住的股票列表，包括代码、名称、炸板时间、概念板块等。适用于分析封板失败、市场分歧、抛压较重等场景。当用户提到炸板、封板失败、开板、破板等关键词时使用此工具。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 炸板股\n\n", date))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
			plateStocksZb, _ := dataMap["plate_stocks_zb"].(map[string]any)
			stockInfo, _ := dataMap["stock_info"].(map[string]any)
			var zbTotal []map[string]any
			for _, stocks := range plateStocksZb {
				if arr, ok := stocks.([]any); ok {
					for _, s := range arr {
						if sm, ok := s.(map[string]any); ok {
							zbTotal = append(zbTotal, sm)
						}
					}
				}
			}
			if len(zbTotal) == 0 {
				sb.WriteString("今日无炸板股\n")
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("共%d只炸板股\n\n", len(zbTotal)))
			sb.WriteString("| 代码 | 名称 | 时间 | 概念板块 |\n|:---:|:---:|:---:|:---:|\n")
			zbSeen := make(map[string]bool)
			for _, sm := range zbTotal {
				sCode, _ := sm["stock_code"].(string)
				if zbSeen[sCode] {
					continue
				}
				zbSeen[sCode] = true
				sName, _ := sm["stock_name"].(string)
				upTime, _ := sm["up_limit_time"].(string)
				platesStr := getPlatesStr(stockInfo, sCode)
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", sCode, sName, upTime, platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetUplimitPlateStocks",
		"获取指定板块的涨停股详情，包括板块内所有涨停股票的代码、名称、连板数、封单比、成交额、市值、概念板块等。适用于深入分析某个板块的涨停个股情况。当用户想查看某个板块的涨停股明细时使用此工具，必须提供板块名称参数。",
		map[string]*schema.ParameterInfo{
			"plate_name": {
				Type:     "string",
				Desc:     "板块名称，如：人工智能、机器人、芯片等",
				Required: true,
			},
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-04-17，默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			plateName := gjson.Get(args, "plate_name").String()
			if plateName == "" {
				return "请提供板块名称参数 plate_name", nil
			}
			date := gjson.Get(args, "date").String()
			dataMap, err := fetchUplimitData(date)
			if err != nil {
				return err.Error(), nil
			}
			loc, _ := time.LoadLocation("Asia/Shanghai")
			if date == "" {
				date = time.Now().In(loc).Format("2006-01-02")
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 板块【%s】涨停股详情\n\n", date, plateName))
			if today, _ := dataMap["today"].(bool); today {
				sb.WriteString("> 数据为实时数据\n\n")
			}
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
					if arr, ok := p.([]any); ok && len(arr) >= 3 {
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
				sb.WriteString(fmt.Sprintf("未找到板块【%s】，请检查板块名称是否正确\n", plateName))
				return sb.String(), nil
			}
			stocks, ok := plateStocks[targetCode].([]any)
			if !ok || len(stocks) == 0 {
				sb.WriteString(fmt.Sprintf("板块【%s】暂无涨停股\n", plateName))
				return sb.String(), nil
			}
			sb.WriteString(fmt.Sprintf("共%d只涨停股\n\n", len(stocks)))
			sb.WriteString("| 代码 | 名称 | 连板 | 类型 | 描述 | 时间 | 封单比 | 收盘封单 | 成交额 | 市值 | 概念板块 |\n|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
			for _, s := range stocks {
				sm, _ := s.(map[string]any)
				sCode, _ := sm["stock_code"].(string)
				sName, _ := sm["stock_name"].(string)
				keepTimes, _ := sm["up_limit_keep_times"].(float64)
				upType, _ := sm["up_limit_type"].(string)
				upDesc, _ := sm["up_limit_desc"].(string)
				upTime, _ := sm["up_limit_time"].(string)
				fdMax := floatOrDefault(sm["fd_max"])
				fdClose := floatOrDefault(sm["fd_close"])
				amount := floatOrDefault(sm["amount"])
				marketC := floatOrDefault(sm["market_c"])
				platesStr := getPlatesStr(stockInfo, sCode)
				sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %.2f%% | %.2f%% | %.2f亿 | %.2f亿 | %s |\n",
					sCode, sName, int(keepTimes), upType, upDesc, upTime, fdMax, fdClose, amount, marketC, platesStr))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTdxCompanyInfo",
		"通过通达信协议获取股票F10公司资料，包括公司简介、股本结构、财务摘要、除权除息等完整信息。当东方财富F10接口不可用或需要补充数据时可使用此工具。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			bundle := api.GetF10Data(stockCode)
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s F10公司资料（通达信）\n\n", stockCode))
			for _, s := range bundle.Sections {
				sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", s.Name, s.Content))
			}
			if bundle.Finance != nil {
				f := bundle.Finance
				sb.WriteString("## 财务摘要\n\n")
				sb.WriteString("| 指标 | 值 |\n|---|---|\n")
				sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
				if f.IPODate != "" {
					sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
				}
				sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
				sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
				sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
				sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
				sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
				sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
				sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
				sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
				sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
				sb.WriteString("\n")
			}
			if len(bundle.XDXR) > 0 {
				sb.WriteString("## 除权除息\n\n")
				sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 |\n|---|---|---|---|---|---|\n")
				for _, x := range bundle.XDXR {
					fh := "-"
					if x.Fenhong != nil {
						fh = fmt.Sprintf("%.4f", *x.Fenhong)
					}
					szg := "-"
					if x.Songzhuangu != nil {
						szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
					}
					pgj := "-"
					if x.Peigujia != nil {
						pgj = fmt.Sprintf("%.2f", *x.Peigujia)
					}
					pg := "-"
					if x.Peigu != nil {
						pg = fmt.Sprintf("%.4f", *x.Peigu)
					}
					sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", x.Date, x.Name, fh, szg, pgj, pg))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTdxFinanceInfo",
		"通过通达信协议获取股票财务信息，包括每股收益、总资产、净资产、营业收入、净利润、股东人数等核心财务指标。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			f := api.GetFinanceInfo(stockCode)
			if f == nil {
				return fmt.Sprintf("%s：获取财务信息失败", stockCode), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 财务信息（通达信）\n\n", stockCode))
			sb.WriteString("| 指标 | 值 |\n|---|---|\n")
			sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
			if f.IPODate != "" {
				sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
			}
			if f.UpdatedDate != "" {
				sb.WriteString(fmt.Sprintf("| 更新日期 | %s |\n", f.UpdatedDate))
			}
			sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
			sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
			sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
			sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
			sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
			sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
			sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
			sb.WriteString(fmt.Sprintf("| 营业成本(万元) | %.2f |\n", f.OperatingCost))
			sb.WriteString(fmt.Sprintf("| 营业利润(万元) | %.2f |\n", f.OperatingProfit))
			sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
			sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
			sb.WriteString(fmt.Sprintf("| 资本公积金(万元) | %.2f |\n", f.CapitalReserve))
			sb.WriteString(fmt.Sprintf("| 未分配利润(万元) | %.2f |\n", f.UndistributedProfit))
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTdxXDXRInfo",
		"通过通达信协议获取股票除权除息信息，包括分红、配股、送转股等历史记录及股本变动情况。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			items := api.GetXDXRInfo(stockCode)
			if items == nil || len(*items) == 0 {
				return fmt.Sprintf("%s：暂无除权除息数据", stockCode), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s 除权除息信息（通达信）\n\n", stockCode))
			sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 |\n|---|---|---|---|---|---|\n")
			for _, x := range *items {
				fh := "-"
				if x.Fenhong != nil {
					fh = fmt.Sprintf("%.4f", *x.Fenhong)
				}
				szg := "-"
				if x.Songzhuangu != nil {
					szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
				}
				pgj := "-"
				if x.Peigujia != nil {
					pgj = fmt.Sprintf("%.2f", *x.Peigujia)
				}
				pg := "-"
				if x.Peigu != nil {
					pg = fmt.Sprintf("%.4f", *x.Peigu)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", x.Date, x.Name, fh, szg, pgj, pg))
			}
			sb.WriteString("\n")
			return sb.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTdxCompanyCategory",
		"通过通达信协议获取股票F10分类信息。不传category参数时返回所有可用分类名称列表；传入category参数时返回该分类的详细内容。可用分类包括：最新提示、公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码,如：600519.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
				Required: true,
			},
			"category": {
				Type:     "string",
				Desc:     "F10分类名称，如：公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级、最新提示。不传或为空时返回所有可用分类列表。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			category := gjson.Get(args, "category").String()
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			api := data.NewTdxKLineApi()
			if category == "" {
				cats := api.GetF10CategoryList(stockCode)
				if cats == nil || len(*cats) == 0 {
					return fmt.Sprintf("%s：获取分类列表失败", stockCode), nil
				}
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("# %s F10可用分类列表（通达信）\n\n", stockCode))
				sb.WriteString("| 序号 | 分类名称 |\n|---|---|\n")
				for i, c := range *cats {
					sb.WriteString(fmt.Sprintf("| %d | %s |\n", i+1, c.Name))
				}
				sb.WriteString("\n> 提示：传入 category 参数可获取对应分类的详细内容。\n")
				return sb.String(), nil
			}
			section := api.GetF10CategoryContent(stockCode, category)
			if section == nil || section.Content == "" {
				return fmt.Sprintf("%s：分类 '%s' 获取失败或内容为空", stockCode, category), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s - %s（通达信）\n\n", stockCode, section.Name))
			sb.WriteString(section.Content + "\n")
			return sb.String(), nil
		},
	))

	// 根据 API Key 配置过滤工具，未配置对应 Key 的工具不注册
	filtered := make([]tool.BaseTool, 0, len(tools))
	for _, t := range tools {
		if wrapper, ok := t.(*DataToolWrapper); ok {
			if !data.IsToolKeyConfigured(wrapper.name) {
				continue
			}
		}
		filtered = append(filtered, t)
	}
	return filtered
}

func fetchUplimitData(date string) (map[string]any, error) {
	if date == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		date = time.Now().In(loc).Format("2006-01-02")
	}
	result := data.NewMarketNewsApi().GetUplimitHot(date, 20)
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

func floatOrDefault(val any) float64 {
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func getPlatesStr(stockInfo map[string]any, code string) string {
	if si, ok := stockInfo[code].(map[string]any); ok {
		if pa, ok := si["plates"].([]any); ok {
			var ps []string
			for _, p := range pa {
				ps = append(ps, fmt.Sprintf("%v", p))
			}
			return strings.Join(ps, ",")
		}
	}
	return ""
}

func getStockNameFromPlateStocks(plateStocks map[string]any, code string) string {
	for _, pStocks := range plateStocks {
		if arr, ok := pStocks.([]any); ok {
			for _, s := range arr {
				if sm, ok := s.(map[string]any); ok {
					if sm["stock_code"] == code {
						name, _ := sm["stock_name"].(string)
						return name
					}
				}
			}
		}
	}
	return ""
}

func marketSentiment(upCount, downCount int) string {
	ratio := float64(upCount) / float64(upCount+downCount)
	if ratio > 0.7 {
		return "极度乐观"
	} else if ratio > 0.6 {
		return "乐观"
	} else if ratio > 0.4 {
		return "中性"
	} else if ratio > 0.3 {
		return "悲观"
	} else {
		return "极度悲观"
	}
}

func parseStockCodesFromArgs(args string, mainField string) []string {
	var codes []string
	mainCode := gjson.Get(args, mainField).String()
	if mainCode != "" {
		for _, c := range strings.Split(mainCode, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				codes = append(codes, c)
			}
		}
	}
	codesArr := gjson.Get(args, "stockCodes").Array()
	for _, c := range codesArr {
		code := strings.TrimSpace(c.String())
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}

type APIResponse struct {
	Code int     `json:"code"`
	Msg  string  `json:"msg"`
	Data APIData `json:"data"`
}

type APIData struct {
	IndexQuote    []APIIndexQuote `json:"index_quote"`
	UpDownDis     APIUpDownDis    `json:"up_down_dis"`
	PurchaseToday []APIPurchase   `json:"purchase_today"`
}

type APIIndexQuote struct {
	SecuCode string  `json:"secu_code" md:"指数代码"`
	SecuName string  `json:"secu_name" md:"指数名称"`
	LastPx   float64 `json:"last_px" md:"最新价格"`
	Change   float64 `json:"change" md:"涨跌"`
	ChangePx float64 `json:"change_px" md:"涨跌点数"`
	UpNum    int     `json:"up_num" md:"上涨家数"`
	DownNum  int     `json:"down_num" md:"下跌家数"`
	FlatNum  int     `json:"flat_num" md:"平盘家数"`
}

type APIUpDownDis struct {
	UpNum       int     `json:"up_num" md:"涨停家数"`
	DownNum     int     `json:"down_num" md:"跌停家数"`
	AverageRise float64 `json:"average_rise" md:"平均涨幅"`
	RiseNum     int     `json:"rise_num" md:"上涨家数总计"`
	FallNum     int     `json:"fall_num" md:"下跌家数总计"`
	Down10      int     `json:"down_10" md:"跌幅8%~10%家数"`
	Down8       int     `json:"down_8" md:"跌幅6%~8%家数"`
	Down6       int     `json:"down_6" md:"跌幅4%~6%家数"`
	Down4       int     `json:"down_4" md:"跌幅2%~4%家数"`
	Down2       int     `json:"down_2" md:"跌幅0%~2%家数"`
	FlatNum     int     `json:"flat_num" md:"平盘家数"`
	Up2         int     `json:"up_2" md:"涨幅0%~2%家数"`
	Up4         int     `json:"up_4" md:"涨幅2%~4%家数"`
	Up6         int     `json:"up_6" md:"涨幅4%~6%家数"`
	Up8         int     `json:"up_8" md:"涨幅6%~8%家数"`
	Up10        int     `json:"up_10" md:"涨幅8%~10%家数"`
	SuspendNum  int     `json:"suspend_num" md:"停牌家数"`
	Status      bool    `json:"status" md:"状态"`
}

type APIPurchase struct {
	SecuCode     string   `json:"secu_code"`
	SecuName     string   `json:"secu_name"`
	SecuCodeFull string   `json:"SecuCode"`
	IPOPrice     float64  `json:"ipo_price"`
	IPOPE        float64  `json:"ipo_pe"`
	AllotMax     int      `json:"allot_max"`
	LotRate      *float64 `json:"lot_rate"`
}

func getMarketDataContent() (string, error) {
	client := data.SharedHTTPClient
	apiURL := "https://x-quote.cls.cn/quote/index/home?app=CailianpressWeb&os=web&sv=8.4.6"

	uaGen, err := fakeUserAgent.New()
	if err != nil {
		uaGen, _ = fakeUserAgent.New()
	}
	ua := uaGen.GetRandom()

	var apiResp APIResponse
	resp, err := client.R().
		SetHeader("User-Agent", ua).
		SetResult(&apiResp).
		Get(apiURL)

	if err != nil {
		return "", fmt.Errorf("调用API失败: %v", err)
	}

	if resp.StatusCode() != 200 || apiResp.Code != 200 {
		return "", fmt.Errorf("API返回错误: 状态码=%d, 错误信息=%s", resp.StatusCode(), apiResp.Msg)
	}

	content := strings.Builder{}
	content.WriteString("# 市场行情数据\r\n\r\n")

	content.WriteString("## 指数行情\r\n\r\n")
	content.WriteString("| 指数代码 | 指数名称 | 最新价格 | 涨跌(%) | 涨跌点数 | 上涨家数 | 下跌家数 | 平盘家数 |\r\n")
	content.WriteString("|----------|----------|----------|---------|----------|----------|----------|----------|\r\n")
	for _, index := range apiResp.Data.IndexQuote {
		content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %d | %d | %d |\r\n",
			index.SecuCode, index.SecuName, index.LastPx, index.Change*100, index.ChangePx,
			index.UpNum, index.DownNum, index.FlatNum))
	}

	content.WriteString("\r\n## 涨跌分布\r\n\r\n")
	content.WriteString("| 涨停家数 | 跌停家数 | 平均涨幅(%) | 上涨家数总计 | 下跌家数总计 |\r\n")
	content.WriteString("|----------|----------|-------------|--------------|--------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %.2f | %d | %d |\r\n",
		apiResp.Data.UpDownDis.UpNum, apiResp.Data.UpDownDis.DownNum, apiResp.Data.UpDownDis.AverageRise*100,
		apiResp.Data.UpDownDis.RiseNum, apiResp.Data.UpDownDis.FallNum))

	content.WriteString("\r\n### 跌幅分布\r\n\r\n")
	content.WriteString("| 跌幅8%~10% | 跌幅6%~8% | 跌幅4%~6% | 跌幅2%~4% | 跌幅0%~2% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|-----------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Down10, apiResp.Data.UpDownDis.Down8, apiResp.Data.UpDownDis.Down6,
		apiResp.Data.UpDownDis.Down4, apiResp.Data.UpDownDis.Down2))

	content.WriteString("\r\n### 涨幅分布\r\n\r\n")
	content.WriteString("| 涨幅0%~2% | 涨幅2%~4% | 涨幅4%~6% | 涨幅6%~8% | 涨幅8%~10% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Up2, apiResp.Data.UpDownDis.Up4, apiResp.Data.UpDownDis.Up6,
		apiResp.Data.UpDownDis.Up8, apiResp.Data.UpDownDis.Up10))

	content.WriteString("\r\n### 其他统计\r\n\r\n")
	content.WriteString(fmt.Sprintf("- 平盘家数: %d\r\n", apiResp.Data.UpDownDis.FlatNum))
	content.WriteString(fmt.Sprintf("- 停牌家数: %d\r\n", apiResp.Data.UpDownDis.SuspendNum))

	content.WriteString("\r\n## 今日申购\r\n\r\n")
	if len(apiResp.Data.PurchaseToday) > 0 {
		content.WriteString("| 股票代码 | 股票名称 | 申购价格 | 申购PE | 最大申购数量 | 中签率(%) |\r\n")
		content.WriteString("|----------|----------|----------|--------|--------------|-----------|\r\n")
		for _, purchase := range apiResp.Data.PurchaseToday {
			lotRate := "-"
			if purchase.LotRate != nil {
				lotRate = fmt.Sprintf("%.2f", *purchase.LotRate)
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %d | %s |\r\n",
				purchase.SecuCode, purchase.SecuName, purchase.IPOPrice, purchase.IPOPE,
				purchase.AllotMax, lotRate))
		}
	} else {
		content.WriteString("今日无新股申购\r\n")
	}

	logger.SugaredLogger.Debug("%s", content.String())
	return content.String(), nil
}

// mutualTypeName 将 MUTUAL_TYPE 代码翻译为中文名称
// 001=沪股通十大成交股, 002=港股通(沪)十大成交股, 003=深股通十大成交股, 004=港股通(深)十大成交股
func mutualTypeName(code string) string {
	switch code {
	case "001":
		return "沪股通十大成交股"
	case "002":
		return "港股通(沪)十大成交股"
	case "003":
		return "深股通十大成交股"
	case "004":
		return "港股通(深)十大成交股"
	default:
		return code
	}
}
