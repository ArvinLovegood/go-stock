package data

import (
	"context"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"testing"
)

func TestNewDeepSeekOpenAiConfig(t *testing.T) {
	db.Init("../../data/stock.db")
	InitAnalyzeSentiment()

	var tools []Tool
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票，返回自然语言选股条件要求的股票所有相关数据",
			Parameters: &FunctionParameters{
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

	ai := NewDeepSeekOpenAi(context.TODO(), 11)
	//res := ai.NewChatStream("长电科技", "sh600584", "长电科技分析和总结", nil)
	res := ai.NewSummaryStockNewsStreamWithTools("总结市场资讯，发掘潜力标的/行业/板块/概念，控制风险。调用工具函数验证", nil, tools, false, nil, nil)

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
	//SearchGuShiTongStockInfo("hk01810", 60)
	msgs := SearchGuShiTongStockInfo("sh600745", 60)
	for _, msg := range *msgs {
		log.SugaredLogger.Infof("%s", msg)
	}
	//SearchGuShiTongStockInfo("gb_goog", 60)

}

func TestGetZSInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	GetZSInfo("中证银行", "sz399986", 5)
	GetZSInfo("上海贝岭", "sh600171", 5)
}
