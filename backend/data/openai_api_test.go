package data

import (
	"context"
	"go-stock/backend/db"
	"testing"
)

func TestNewDeepSeekOpenAiConfig(t *testing.T) {
	db.Init("../../data/stock.db")

	var tools []Tool
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票，返回自然语言选股条件要求的股票所有相关数据",
			Parameters: FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "选股自然语言,并且条件使用;分隔，或者条件使用,分隔。例如：创新药;PE<30;净利润增长率>50%;",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	ai := NewDeepSeekOpenAi(context.TODO())
	//res := ai.NewChatStream("长电科技", "sh600584", "长电科技分析和总结", nil)
	res := ai.NewSummaryStockNewsStreamWithTools("总结市场资讯，发掘潜力标的/行业/板块/概念，控制风险。调用工具函数验证", nil, tools)

	for {
		select {
		case msg := <-res:
			if len(msg) > 0 {
				t.Log(msg)
				if msg["content"] == "DONE" {
					return
				}
			}
		}
	}
}

func TestGetTopNewsList(t *testing.T) {
	news := GetTopNewsList(30)
	t.Log(news)
}

func TestSearchGuShiTongStockInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	SearchGuShiTongStockInfo("hk01810", 60)
	SearchGuShiTongStockInfo("sh600745", 60)
	SearchGuShiTongStockInfo("gb_goog", 60)

}
