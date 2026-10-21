package agent

// web_search_tool.go — 将 DeepSeek 联网搜索/网页抓取暴露为 Agent 可调用的工具。
//
// 底层: api.deepseek.com/v1/tools/search 与 /v1/tools/fetch（data.DeepSeekSearchAPI）
// 认证: 设置页 deepSeekSearchApiKey（配置后无需重启，工具执行时实时读取）
// 计费: 按成功请求次数计费，故工具描述中明确「仅在必要时使用」，
//       并将默认 count 压到 5、need_content 默认 false，控制单次调用成本。
//
// 暴露为两个工具:
//   1. WebSearch: 全网实时搜索，返回标题/链接/摘要等结构化结果
//   2. WebFetch:  抓取指定网页正文并转 Markdown，用于阅读用户给出的链接
//      或搜索结果中某条的全文

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

// 联网搜索成本控制常量。
const (
	webSearchDefaultCount  = 5  // 默认返回条数（服务端默认 10，压缩以省成本）
	webSearchMaxCount      = 10 // 工具层上限，防止模型一次请求拉太多
	webSearchMaxQueryRunes = 128

	// webFetchMaxOutputChars Fetch 结果最大输出长度。
	// 网页正文动辄数万字符，全量回传会撑爆对话上下文，超出部分截断并提示。
	webFetchMaxOutputChars = 24000
)

// webSearchTool 实现 tool.BaseTool 接口，封装 DeepSeek 联网搜索。
type webSearchTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewWebSearchTool 构造联网搜索工具实例。
//
// 工具名：WebSearch
// 参数：
//   - query (string, required): 搜索词，支持 site:/-site:/"..."/lang:/after:/before: 等搜索语法
//   - count (integer, optional): 返回条数，默认 5，范围 [1, 10]
//   - need_content (boolean, optional): 是否附带网页正文，默认 false
//   - ori_question (string, optional): 用户原始问题，帮助理解搜索意图
//
// 返回 markdown 格式的搜索结果列表（标题、链接、站点、日期、摘要）。
func NewWebSearchTool() tool.BaseTool {
	return &webSearchTool{
		name: "WebSearch",
		description: "联网搜索实时网页信息（DeepSeek Search）。" +
			"适用场景：查询模型知识截止日期之后的最新事件/新闻/政策/数据、非股市领域的一般性问题、" +
			"需要给出来源链接佐证观点。" +
			"\n注意：" +
			"\n- 本工具按次计费，仅在本地数据工具（行情/新闻/公告/研报工具）无法覆盖时使用" +
			"\n- 股票实时行情、财务、新闻请优先使用专用数据工具，不要用本工具" +
			"\n- query 支持 DeepSeek 搜索语法：site: 限定站点、\"...\" 精确短语、lang:zh 限定语言、after:2025-01-01 限定时间范围" +
			"\n\n调用示例：WebSearch(query=\"美联储 2026年 降息 预期\", count=5)" +
			"\n返回格式：联网搜索结果（共 N 条）：每条含标题、链接、站点、日期、摘要。",
		params: map[string]*schema.ParameterInfo{
			"query": {
				Type: "string",
				Desc: "搜索词，1~128 个字符。支持搜索语法：site:github.com 限定站点、\"精确短语\"、lang:zh 限定语言、after:2025-01-01 限定日期之后",
				Required: true,
			},
			"count": {
				Type:     "integer",
				Desc:     "返回条数，默认 5，范围 [1, 10]",
				Required: false,
			},
			"need_content": {
				Type: "boolean",
				Desc: "是否附带网页正文全文，默认 false（仅返回摘要，更快更省）。仅当确需阅读全文且不便单独调用 WebFetch 时设为 true",
				Required: false,
			},
			"ori_question": {
				Type:     "string",
				Desc: "触发本次搜索的用户原始问题（可选）。当 query 是从用户问题提炼的关键词时传入，帮助理解搜索意图",
				Required: false,
			},
		},
	}
}

// Info 返回工具元信息，供 LLM 选择工具时参考。
func (t *webSearchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行联网搜索。
func (t *webSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)

	query := strings.TrimSpace(gjson.Get(argumentsInJSON, "query").String())
	if query == "" {
		return "请提供搜索词（query 参数）。", nil
	}

	count := int(gjson.Get(argumentsInJSON, "count").Int())
	if count <= 0 {
		count = webSearchDefaultCount
	}
	if count > webSearchMaxCount {
		count = webSearchMaxCount
	}
	needContent := gjson.Get(argumentsInJSON, "need_content").Bool()
	oriQuestion := strings.TrimSpace(gjson.Get(argumentsInJSON, "ori_question").String())

	results, err := data.NewDeepSeekSearchAPI().Search(query, count, needContent, oriQuestion)
	if err != nil {
		// 工具失败不中断 Agent：返回错误文案，由模型决定降级（改用其他工具或告知用户）
		return fmt.Sprintf("联网搜索失败: %v", err), nil
	}
	if len(results) == 0 {
		return fmt.Sprintf("未搜索到与 %q 相关的网页。可尝试更换关键词或调整搜索语法。", query), nil
	}

	return formatWebSearchResults(results), nil
}

// formatWebSearchResults 将搜索结果格式化为 markdown，便于 LLM 引用来源。
func formatWebSearchResults(results []data.DeepSeekSearchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("联网搜索结果（共 %d 条，按相关性排序）:\n\n", len(results)))

	for i, r := range results {
		meta := make([]string, 0, 3)
		if r.SiteName != "" {
			meta = append(meta, r.SiteName)
		}
		if r.Date != "" {
			meta = append(meta, r.Date)
		}
		if r.RerankScore >= 1.5 {
			meta = append(meta, fmt.Sprintf("相关度%.1f", r.RerankScore))
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   链接: %s", r.URL))
		if len(meta) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(meta, ", ")))
		}
		sb.WriteString("\n")
		// summary 为面向模型的长摘要；无 summary 时退化到 snippet
		summary := strings.TrimSpace(r.Summary)
		if summary == "" {
			summary = strings.TrimSpace(r.Snippet)
		}
		if summary != "" {
			sb.WriteString("   摘要: " + summary + "\n")
		}
		if content := strings.TrimSpace(r.Content); content != "" {
			sb.WriteString("   正文片段:\n")
			for _, line := range strings.Split(content, "\n") {
				sb.WriteString("   " + line + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("回答时请注明信息来源链接；若结果与问题相关性不足，可调整关键词重新搜索或改用其他工具。\n")
	return sb.String()
}

// webFetchTool 实现 tool.BaseTool 接口，封装 DeepSeek 网页抓取。
type webFetchTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewWebFetchTool 构造网页抓取工具实例。
//
// 工具名：WebFetch
// 参数：
//   - url (string, required): 要抓取的网页地址（http/https）
//
// 返回网页标题与 Markdown 正文（超长截断）。
func NewWebFetchTool() tool.BaseTool {
	return &webFetchTool{
		name: "WebFetch",
		description: "抓取指定网页正文并转为 Markdown 返回（DeepSeek Fetch）。" +
			"适用场景：用户给出链接要求阅读/总结、或 WebSearch 结果中某条需要阅读全文。" +
			"\n注意：本工具按次计费；抓取结果受目标站点可用性影响，失败时不要对同一 URL 反复重试。" +
			"\n\n调用示例：WebFetch(url=\"https://example.com/article\")" +
			"\n返回格式：页面标题 + Markdown 正文（超长自动截断）。",
		params: map[string]*schema.ParameterInfo{
			"url": {
				Type:     "string",
				Desc:     "要抓取的网页地址，必须为含域名的 http/https URL，长度不超过 8192",
				Required: true,
			},
		},
	}
}

// Info 返回工具元信息
func (t *webFetchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行网页抓取。
func (t *webFetchTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)

	rawURL := strings.TrimSpace(gjson.Get(argumentsInJSON, "url").String())
	if rawURL == "" {
		return "请提供要抓取的网页地址（url 参数）。", nil
	}

	page, err := data.NewDeepSeekSearchAPI().Fetch(rawURL)
	if err != nil {
		return fmt.Sprintf("网页抓取失败: %v。可告知用户该链接暂时无法读取，或尝试通过 WebSearch 获取相关内容。", err), nil
	}

	content := page.Content
	truncated := false
	if len(content) > webFetchMaxOutputChars {
		content = content[:webFetchMaxOutputChars]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("网页标题: %s\n来源: %s\n\n", page.Title, rawURL))
	sb.WriteString(content)
	if truncated {
		sb.WriteString(fmt.Sprintf("\n\n(正文超过 %d 字符，已截断。如需后续内容可说明。)", webFetchMaxOutputChars))
	}
	return sb.String(), nil
}

// 兼容编译期检查：确保 webSearchTool / webFetchTool 实现 tool.BaseTool 接口
var (
	_ tool.BaseTool = (*webSearchTool)(nil)
	_ tool.BaseTool = (*webFetchTool)(nil)
)
