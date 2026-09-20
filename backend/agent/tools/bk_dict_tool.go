package tools

import (
	"context"
	"encoding/json"
	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/coocood/freecache"
)

// @Author spark
// @Date 2025/9/27 14:09
// @Desc
// -----------------------------------------------------------------------------------
type ToolQueryBKDict struct{}

func (t ToolQueryBKDict) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "QueryBKDictInfo",
		Desc: "获取所有板块/行业名称或者代码(bkCode,bkName)",
		// 显式声明空参数 schema：部分模型网关（如火山 Ark）要求 parameters 必须是
		// type=object 的合法 JSON Schema，缺省会被拒绝(400)。
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t ToolQueryBKDict) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	resp := data.NewMarketNewsApi().EMDictCode("016", freecache.NewCache(100))
	bytes, err := json.Marshal(resp)
	return string(bytes), err
}

func GetQueryBKDictTool() tool.InvokableTool {
	return &ToolQueryBKDict{}
}
