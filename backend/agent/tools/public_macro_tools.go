package tools

import (
	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

func getPublicMacroTools() []tool.BaseTool {
	return []tool.BaseTool{
		NewDataToolWrapper(
			"GetWorldBankIndicator",
			"查询世界银行公开年度宏观指标，不需要密钥。支持各国 GDP 实际增长率、CPI通胀率、失业率；数据可能滞后或修订，不是实时或历史时点数据库。",
			map[string]*schema.ParameterInfo{
				"country":   {Type: "string", Desc: "国家三字母代码，如 CHN、USA、JPN", Required: true},
				"indicator": {Type: "string", Desc: "gdp_growth=GDP实际增长率，inflation=消费者物价通胀率，unemployment=失业率", Required: true},
				"limit":     {Type: "integer", Desc: "最近记录数量，默认5，最大30"},
			},
			func(args string) (string, error) {
				return data.NewPublicMacroAPI().WorldBankIndicator(gjson.Get(args, "country").String(), gjson.Get(args, "indicator").String(), int(gjson.Get(args, "limit").Int()))
			},
		),
		NewDataToolWrapper(
			"GetTreasuryExchangeRates",
			"查询美国财政部官方季度报告汇率，不需要密钥。返回每1美元对应的外币数量；不是实时市场汇率。",
			map[string]*schema.ParameterInfo{
				"currency": {Type: "string", Desc: "币种代码：CNY、HKD、JPY、EUR、GBP", Required: true},
				"limit":    {Type: "integer", Desc: "最近记录数量，默认5，最大30"},
			},
			func(args string) (string, error) {
				return data.NewPublicMacroAPI().TreasuryExchangeRates(gjson.Get(args, "currency").String(), int(gjson.Get(args, "limit").Int()))
			},
		),
	}
}
