package tools

import (
	"fmt"

	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

func getBloombergNewsTools() []tool.BaseTool {
	return []tool.BaseTool{NewDataToolWrapper(
		"GetBloombergMarketNews",
		"获取彭博 Bloomberg Markets RSS 最新市场新闻，含英文标题、摘要、发布时间和原文链接。仅为当前新闻，不支持历史时点查询，不能用于回测；原文可能需要订阅。",
		map[string]*schema.ParameterInfo{
			"limit": {Type: "integer", Desc: "返回条数，1至20，默认10", Required: false},
		},
		func(args string) (string, error) {
			limit := 0
			if gjson.Get(args, "limit").Exists() {
				limit = int(gjson.Get(args, "limit").Int())
				if limit == 0 {
					return "", fmt.Errorf("新闻条数必须在 1 至 20 之间")
				}
			}
			return data.NewBloombergNewsAPI().MarketNewsForAI(limit)
		},
	)}
}
