package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2025/8/4 16:38
// @Desc
//-----------------------------------------------------------------------------------

func GetQueryMarketNewsTool() tool.InvokableTool {
	return &QueryMarketNews{}
}

type QueryMarketNews struct {
}

func (q QueryMarketNews) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "QueryMarketNews",
		Desc: "国内外市场资讯/电报/会议/事件检索。数据源为本地快讯库（财联社电报/新浪财经/外媒/格隆汇），支持关键词搜索与分页；keyword 为空时返回最新快讯，同时附财联社财经日历（事件/会议）",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     "string",
				Desc:     "关键词，搜索标题与正文（可选，为空返回最新快讯）",
				Required: false,
			},
			"source": {
				Type:     "string",
				Desc:     "来源过滤：财联社电报/新浪财经/外媒/格隆汇（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20，最大50",
				Required: false,
			},
		}),
	}, nil
}

func (q QueryMarketNews) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	keyword := strings.TrimSpace(gjson.Get(argumentsInJSON, "keyword").String())
	source := strings.TrimSpace(gjson.Get(argumentsInJSON, "source").String())
	page := int(gjson.Get(argumentsInJSON, "page").Int())
	pageSize := int(gjson.Get(argumentsInJSON, "pageSize").Int())
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	md := strings.Builder{}

	// 无关键词时附带财联社财经日历（事件/会议安排），关键词搜索场景下日历无意义，省 token
	if keyword == "" {
		res := data.NewMarketNewsApi().ClsCalendar()
		for _, a := range res {
			bytes, err := json.Marshal(a)
			if err != nil {
				continue
			}
			date := gjson.Get(string(bytes), "calendar_day")
			md.WriteString("\n### 事件/会议日期：" + date.String())
			list := gjson.Get(string(bytes), "items")
			list.ForEach(func(key, value gjson.Result) bool {
				md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
				return true
			})
		}
	}

	news, total := data.NewMarketNewsApi().SearchNewsList(keyword, source, page, pageSize)
	md.WriteString(fmt.Sprintf("\n### 市场资讯（共 %d 条匹配，第 %d/%d 页）\n", total, page, (int(total)+pageSize-1)/pageSize))
	if total == 0 {
		md.WriteString("无匹配快讯，可尝试更换关键词或来源。\n")
		return md.String(), nil
	}
	for _, telegraph := range *news {
		content := stripHTMLForAI(telegraph.Content)
		if runes := []rune(content); len(runes) > 200 {
			content = string(runes[:200]) + "..."
		}
		md.WriteString(fmt.Sprintf("\n## %s | %s | %s\n", telegraph.Time, telegraph.Source, telegraph.Title))
		md.WriteString(content + "\n")
		if telegraph.Url != "" {
			md.WriteString("原文: " + telegraph.Url + "\n")
		}
	}
	return md.String(), nil
}
