package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

func getYahooFinanceTools() []tool.BaseTool {
	return []tool.BaseTool{NewDataToolWrapper(
		"SearchYahooStockNews",
		"查询 Yahoo Finance 港美股个股近期新闻，返回标题、发布者、链接和发布时间。搜索结果不代表公告或实时行情；不得用于历史时点回测。",
		map[string]*schema.ParameterInfo{
			"stockCode": {Type: "string", Desc: "港美股代码，如 00700.HK、AAPL.US", Required: true},
			"limit":     {Type: "integer", Desc: "最多请求的新闻条数，1至20，默认10", Required: false},
		},
		func(args string) (string, error) {
			code := strings.TrimSpace(gjson.Get(args, "stockCode").String())
			limit := 10
			if gjson.Get(args, "limit").Exists() {
				limit = int(gjson.Get(args, "limit").Int())
			}
			items, err := data.NewYahooFinanceAPI().StockNews(code, limit)
			if err != nil {
				return "", err
			}
			if len(items) == 0 {
				return fmt.Sprintf("Yahoo Finance 未返回与 %s 明确关联的新闻。", code), nil
			}
			result, err := json.Marshal(items)
			if err != nil {
				return "", fmt.Errorf("Yahoo 新闻序列化失败: %w", err)
			}
			return "数据来源：Yahoo Finance；发布时间为 Unix 秒。新闻为第三方搜索结果，请核实原文。\n" + string(result), nil
		},
	)}
}
