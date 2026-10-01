package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/tidwall/gjson"
	"go-stock/backend/data"
)

// @Author spark
// @Date 2025/8/5 11:31
// @Desc
//-----------------------------------------------------------------------------------

func GetStockKLineTool() tool.InvokableTool {
	return &QueryStockKLine{}
}

type QueryStockKLine struct {
}

func (q QueryStockKLine) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "QueryStockKLine",
		Desc: "获取股票K线数据。输入股票名称和K线周期，返回股票K线数据。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"days": {
				Type:     "string",
				Desc:     "日K数据条数。",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头;币安USDT-M永续合约：bn:开头，如bn:btcusdt;Bitget美股永续合约：bt:开头，如bt:aaplusdt）",
				Required: true,
			},
		}),
	}, nil
}

func (q QueryStockKLine) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	stockCode := GetStockCode(gjson.Get(argumentsInJSON, "stockCode").String())
	days := gjson.Get(argumentsInJSON, "days").String()
	toIntDay, err := convertor.ToInt(days)
	if err != nil {
		toIntDay = 90
	}
	if strutil.HasPrefixAny(stockCode, []string{"sz", "sh", "hk", "us", "gb_"}) || data.IsBinanceFuturesCode(stockCode) || data.IsBitgetFuturesCode(stockCode) {
		K := &[]data.KLineData{}
		if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
			K = data.NewStockDataApi().GetKLineData(stockCode, "240", toIntDay)
		}
		if strutil.HasPrefixAny(stockCode, []string{"hk", "us", "gb_"}) {
			K = data.NewStockDataApi().GetHK_KLineData(stockCode, "day", toIntDay)
		}
		var sourceLabel string
		if K == nil || len(*K) == 0 {
			fallbackResult := data.FetchKLineWithFallback(stockCode, "", "101", int(toIntDay), "")
			if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
				K = fallbackResult.Data
				sourceLabel = fallbackResult.Source
			}
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
		res := "\r\n ### " + stockCode + " " + convertor.ToString(toIntDay) + "日K线数据" + sourceInfo + "：\r\n" + markdownTable + "\r\n"
		return res, nil
	} else {
		return "无数据，可能股票代码错误。（A股：sh,sz开头;港股hk开头,美股：us开头;币安永续：bn:开头，如bn:btcusdt;美股永续：bt:开头，如bt:aaplusdt）", fmt.Errorf("不支持的股票代码:%s", stockCode)
	}
}
