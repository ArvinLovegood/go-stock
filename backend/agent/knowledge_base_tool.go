package agent

// knowledge_base_tool.go — 将自定义知识库检索暴露为 Agent 可调用的工具。
//
// 设计动机：
//   - 用户可在前端创建任意多个主题知识库（财报、行业研究、投资策略等）
//   - Agent 需要在回答过程中主动检索这些 KB，召回与问题相关的文档片段
//   - 暴露为两个工具：
//       1. ListKnowledgeBases：列出当前可用的所有知识库（让 Agent 知道有哪些 KB）
//       2. SearchKnowledgeBase：按 KB 名称 + 查询语句检索
//
// 调用时机（Agent 自主决策）：
//   - 用户明确要求"参考我的 XXX 知识库"或"从 XXX 文档中查找"
//   - 当前问题与某个 KB 主题相关，需补充背景知识
//   - 历史经验不足（SearchLongTermMemory 无结果）时作为补充知识源
//
// 不调用的场景：
//   - 实时行情/财务数据查询（应走数据工具）
//   - 用户未提及知识库且问题与已知 KB 主题无关

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"

	"go-stock/backend/logger"
)

// kbSearchTool 实现 tool.BaseTool 接口，封装 SearchKnowledgeBase 为 Agent 可调用工具。
type kbSearchTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewSearchKnowledgeBaseTool 构造知识库检索工具实例。
//
// 工具名：SearchKnowledgeBase
// 参数：
//   - kb_name (string, required): 知识库名称（用户在前端创建的 KB 名）
//   - query (string, required): 检索查询语句
//   - top_k (integer, optional): 返回结果数，默认 5，范围 [1, 20]
//
// 返回 markdown 格式的检索结果列表（含相似度、来源、内容片段）。
func NewSearchKnowledgeBaseTool() tool.BaseTool {
	return &kbSearchTool{
		name: "SearchKnowledgeBase",
		description: "在用户自定义的知识库中检索语义相关的文档片段。" +
			"用户已通过前端创建了若干主题知识库（如财报、行业研究、投资策略等）。" +
			"当用户明确要求参考某个知识库、或当前问题与某个 KB 主题相关时调用。" +
			"调用前可先用 ListKnowledgeBases 查看可用的知识库列表。" +
			"注意：本工具返回的是用户上传的文档，非实时数据；实时行情请使用数据查询工具。" +
			"\n\n调用示例：SearchKnowledgeBase(kb_name=\"投资策略\", query=\"白酒行业估值方法\", top_k=5)" +
			"\n返回格式：知识库 \"投资策略\" 检索结果（按相似度排序，共 N 条）：" +
			"\n1. [相似度 0.85] 来源: 贵州茅台_2024年报.txt (chunk 2/5)" +
			"\n   内容:" +
			"\n   <文档片段正文，每行缩进 3 空格>",
		params: map[string]*schema.ParameterInfo{
			"kb_name": {
				Type:     "string",
				Desc:     "知识库名称（用户在前端创建的 KB 名，如 '财报知识'、'行业研究'）。若不确定有哪些 KB，先调用 ListKnowledgeBases",
				Required: true,
			},
			"query": {
				Type:     "string",
				Desc:     "检索查询语句，通常是用户问题或问题关键词。例如：'贵州茅台财报分析'、'白酒行业趋势'",
				Required: true,
			},
			"top_k": {
				Type:     "integer",
				Desc:     "返回结果数，默认 5，范围 [1, 20]。结果按相似度降序排列",
				Required: false,
			},
		},
	}
}

// Info 返回工具元信息，供 LLM 选择工具时参考。
func (t *kbSearchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行知识库检索。
//
// 参数解析（JSON）：
//   - kb_name: 必填，非空字符串
//   - query: 必填，非空字符串
//   - top_k: 可选，默认 5，clamp 到 [1, 20]
//
// 返回：
//   - 命中：markdown 格式的检索结果列表
//   - 未命中：友好提示
//   - 错误：错误信息（KB 不存在、向量库未初始化等）
func (t *kbSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)

	kbName := strings.TrimSpace(gjson.Get(argumentsInJSON, "kb_name").String())
	if kbName == "" {
		return "请提供知识库名称（kb_name 参数）。若不确定有哪些知识库，可调用 ListKnowledgeBases 工具查看。", nil
	}

	query := strings.TrimSpace(gjson.Get(argumentsInJSON, "query").String())
	if query == "" {
		return fmt.Sprintf("请提供检索查询语句（query 参数），用于在知识库 %q 中检索", kbName), nil
	}

	topK := int(gjson.Get(argumentsInJSON, "top_k").Int())
	if topK <= 0 {
		topK = kbDefaultTopK
	}
	if topK > kbMaxTopK {
		topK = kbMaxTopK
	}
	if topK < 1 {
		topK = 1
	}

	results, err := SearchKnowledgeBase(ctx, kbName, query, topK)
	if err != nil {
		return fmt.Sprintf("知识库检索失败: %v", err), nil
	}
	if len(results) == 0 {
		// 区分"KB 为空"与"无匹配结果"两种情况，便于 Agent 决策
		info := GetKnowledgeBase(kbName)
		if info == nil {
			return fmt.Sprintf("知识库 %q 不存在。可调用 ListKnowledgeBases 查看可用的知识库列表。", kbName), nil
		}
		if info.DocumentCount == 0 {
			return fmt.Sprintf("知识库 %q 当前为空（无文档），请提示用户先上传文档。", kbName), nil
		}
		return fmt.Sprintf("在知识库 %q 中未找到与 %q 相关的文档。", kbName, query), nil
	}

	return formatKBSearchResults(results, topK), nil
}

// formatKBSearchResults 将检索结果格式化为 markdown，便于 LLM 解析。
//
// 输出格式示例：
//
//	知识库 "财报知识" 检索结果（按相似度排序，共 3 条）:
//
//	1. [相似度 0.85] 来源: 贵州茅台_2024年报.txt (chunk 2/5)
//	   内容:
//	   2024 年公司实现营业收入...
//
//	2. [相似度 0.78] 来源: 五粮液_财报摘要.md (chunk 1/3)
//	   内容:
//	   ...
func formatKBSearchResults(results []KnowledgeBaseSearchResult, topN int) string {
	if len(results) == 0 {
		return ""
	}
	if topN > 0 && len(results) > topN {
		results = results[:topN]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("知识库 %q 检索结果（按相似度排序，共 %d 条）:\n\n",
		results[0].KBName, len(results)))

	for i, r := range results {
		source := r.Source
		if source == "" {
			source = "未知来源"
		}
		chunkInfo := ""
		if r.Metadata != nil {
			if ci := r.Metadata["chunk_index"]; ci != "" {
				tc := r.Metadata["total_chunks"]
				if tc != "" {
					chunkInfo = fmt.Sprintf(" (chunk %s/%s)", ci, tc)
				} else {
					chunkInfo = fmt.Sprintf(" (chunk %s)", ci)
				}
			}
		}

		sb.WriteString(fmt.Sprintf("%d. [相似度 %.2f] 来源: %s%s\n", i+1, r.Similarity, source, chunkInfo))
		sb.WriteString("   内容:\n")
		// 内容缩进 3 空格，便于 LLM 区分结构与正文
		content := strings.TrimSpace(r.Content)
		for _, line := range strings.Split(content, "\n") {
			sb.WriteString("   " + line + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("请在回答时参考上述文档片段；若内容过时或不适用请主动说明，并提示用户可更新知识库。\n")
	return sb.String()
}

// kbListTool 实现 tool.BaseTool 接口，封装 ListKnowledgeBases 为 Agent 可调用工具。
type kbListTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewListKnowledgeBasesTool 构造知识库列表工具实例。
//
// 工具名：ListKnowledgeBases
// 参数：无
//
// 返回 markdown 格式的知识库列表（含名称、描述、文档数、创建时间）。
func NewListKnowledgeBasesTool() tool.BaseTool {
	return &kbListTool{
		name: "ListKnowledgeBases",
		description: "列出用户已创建的所有自定义知识库。" +
			"当不确定有哪些知识库可用、或需要确认某个 KB 是否存在时调用。" +
			"获取到知识库名称后，可用 SearchKnowledgeBase 工具按名称检索具体内容。",
		params: map[string]*schema.ParameterInfo{},
	}
}

// Info 返回工具元信息
func (t *kbListTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行知识库列表查询
func (t *kbListTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called", t.name)

	kbs := ListKnowledgeBases()
	if len(kbs) == 0 {
		return "当前没有任何自定义知识库。可提示用户在前端创建知识库并上传文档。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前可用的知识库（共 %d 个）:\n\n", len(kbs)))
	sb.WriteString("| 名称 | 描述 | 文档数 | 创建时间 |\n")
	sb.WriteString("|------|------|--------|----------|\n")
	for _, kb := range kbs {
		desc := kb.Description
		if desc == "" {
			desc = "-"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n",
			kb.Name, desc, kb.DocumentCount, kb.CreatedAt.Format("2006-01-02")))
	}

	sb.WriteString("\n可用 SearchKnowledgeBase 工具按名称检索具体内容（参数: kb_name + query）。\n")
	return sb.String(), nil
}

// kbSearchAllTool 实现 tool.BaseTool 接口，封装 SearchAllKnowledge 为 Agent 可调用工具。
type kbSearchAllTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewSearchAllKnowledgeTool 构造跨所有知识库 + 长期记忆的统一检索工具实例。
//
// 工具名：SearchAllKnowledge
// 参数：
//   - query (string, required): 检索查询语句
//   - top_k (integer, optional): 返回结果数，默认 5，范围 [1, 20]
//
// 与 SearchKnowledgeBase 的区别：无需指定 kb_name，一次调用即检索所有自定义 KB
// 与历史问答经验（qa_history），合并后按相似度全局重排。适用于需要跨多个知识源
// 综合获取背景信息、或不确定相关知识分布在哪个 KB 的场景。
func NewSearchAllKnowledgeTool() tool.BaseTool {
	return &kbSearchAllTool{
		name: "SearchAllKnowledge",
		description: "跨所有自定义知识库与历史问答经验统一检索，返回按相似度全局排序的相关片段。" +
			"一次调用同时检索所有 KB 与长期记忆（qa_history），无需预先指定知识库名称。" +
			"适用场景：需要跨多个知识源综合获取背景信息、不确定相关知识分布在哪个 KB、" +
			"或希望一次性获取最相关的片段而不必逐个检索各 KB。" +
			"若用户明确指定了某个知识库，优先用 SearchKnowledgeBase 精确检索该 KB。",
		params: map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "检索查询语句，通常是用户问题或问题关键词。例如：'贵州茅台财报分析'、'白酒行业趋势'",
				Required: true,
			},
			"top_k": {
				Type:     "integer",
				Desc:     "返回结果数，默认 5，范围 [1, 20]。结果按相似度降序排列",
				Required: false,
			},
		},
	}
}

// Info 返回工具元信息，供 LLM 选择工具时参考。
func (t *kbSearchAllTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行跨知识库统一检索。
//
// 参数解析（JSON）：
//   - query: 必填，非空字符串
//   - top_k: 可选，默认 5，clamp 到 [1, 20]
//
// 返回：
//   - 命中：markdown 格式的统一检索结果（含来源标签、相似度、内容片段）
//   - 未命中：友好提示
//   - 错误：错误信息
func (t *kbSearchAllTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)

	query := strings.TrimSpace(gjson.Get(argumentsInJSON, "query").String())
	if query == "" {
		return "请提供检索查询语句（query 参数）。", nil
	}

	topK := int(gjson.Get(argumentsInJSON, "top_k").Int())
	if topK <= 0 {
		topK = unifiedDefaultTopK
	}
	if topK > unifiedMaxTopK {
		topK = unifiedMaxTopK
	}
	if topK < 1 {
		topK = 1
	}

	hits, err := SearchAllKnowledge(ctx, query, topK)
	if err != nil {
		return fmt.Sprintf("统一检索失败: %v", err), nil
	}
	if len(hits) == 0 {
		return "未在任何知识库与历史经验中检索到与 " + query + " 相关的内容。" +
			"可能原因：知识库为空、向量库未就绪、或确实无匹配内容。", nil
	}

	return FormatUnifiedHits(hits), nil
}

// 兼容编译期检查：确保 kbSearchTool / kbListTool / kbSearchAllTool 实现 tool.BaseTool 接口
var (
	_ tool.BaseTool = (*kbSearchTool)(nil)
	_ tool.BaseTool = (*kbListTool)(nil)
	_ tool.BaseTool = (*kbSearchAllTool)(nil)
)
