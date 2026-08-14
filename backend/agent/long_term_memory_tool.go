package agent

// long_term_memory_tool.go — 将长期记忆向量检索暴露为 Agent 可调用的工具。
//
// 设计动机：
//   - buildSelfEvolutionPrompt 在会话启动时一次性注入 Top-K 历史经验，但 Agent 在
//     多轮推理过程中可能需要按子问题主动检索更细粒度的历史片段
//   - 暴露为 Tool 后，Agent 可在 ReAct/PlanExecute/DeepAgents 任意阶段自主调用，
//     实现"边回答边检索"的动态记忆召回
//
// 调用时机（Agent 自主决策）：
//   - 用户明确询问"之前是否问过/分析过..."
//   - 当前问题与历史结论可能相关，需承接或对比
//   - 需要验证某个数据点是否在过往分析中出现过
//
// 不调用的场景：
//   - 实时行情/财务数据查询（应走数据工具，而非历史经验）
//   - 用户明确要求"重新分析"（应直接执行，不参考历史）

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"

	"go-stock/backend/logger"
)

// longTermMemoryTool 实现 tool.BaseTool 接口，封装 SearchRelevant 为 Agent 可调用工具。
type longTermMemoryTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
}

// NewSearchLongTermMemoryTool 构造长期记忆检索工具实例。
//
// 工具名：SearchLongTermMemory
// 参数：
//   - query (string, required): 检索查询语句，通常是用户问题或问题关键词
//   - top_k (integer, optional): 返回结果数，默认 5，范围 [1, 10]
//
// 返回 markdown 格式的历史经验列表（含相似度、日期、模式、问题、回复摘要、报告路径）。
func NewSearchLongTermMemoryTool() tool.BaseTool {
	return &longTermMemoryTool{
		name: "SearchLongTermMemory",
		description: "检索历史问答经验库，召回与当前问题语义相似的历史对话片段。" +
			"当需要参考过往分析、验证之前是否问过类似问题、或承接历史结论时调用。" +
			"注意：本工具返回的是历史经验，非实时数据；实时行情/财务数据请使用对应的数据查询工具。",
		params: map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "检索查询语句，通常是用户当前问题或问题关键词。例如：'贵州茅台财报分析'、'白酒板块走势'",
				Required: true,
			},
			"top_k": {
				Type:     "integer",
				Desc:     "返回结果数，默认 5，范围 [1, 10]。结果按相似度降序排列",
				Required: false,
			},
		},
	}
}

// Info 返回工具元信息，供 LLM 选择工具时参考。
func (t *longTermMemoryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行长期记忆检索。
//
// 参数解析（JSON）：
//   - query: 必填，非空字符串
//   - top_k: 可选，默认 5，clamp 到 [1, 10]
//
// 返回：
//   - 命中：markdown 格式的历史经验列表
//   - 未命中：友好提示"未找到相关历史经验"
//   - 错误：错误信息（向量库未初始化等）
func (t *longTermMemoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)

	query := strings.TrimSpace(gjson.Get(argumentsInJSON, "query").String())
	if query == "" {
		return "请提供检索查询语句（query 参数）", nil
	}

	topK := int(gjson.Get(argumentsInJSON, "top_k").Int())
	if topK <= 0 {
		topK = longTermMemoryDefaultTopK
	}
	if topK > 10 {
		topK = 10
	}
	if topK < 1 {
		topK = 1
	}

	// 向量库未初始化时 SearchRelevant 返回 nil，走友好降级提示
	recalls := SearchRelevant(ctx, query, topK, CurrentUserKey(""))
	if len(recalls) == 0 {
		// 区分"向量库未就绪"与"无匹配结果"两种情况，便于 Agent 决策
		initLongTermMemoryStore()
		if longTermMemoryColl == nil {
			return fmt.Sprintf("长期记忆向量库未就绪（原因: %v），暂无法检索历史经验。", longTermMemoryErr), nil
		}
		if longTermMemoryColl.Count() == 0 {
			return "历史经验库为空，暂无可检索的记录。", nil
		}
		return fmt.Sprintf("未找到与 %q 相关的历史经验。", query), nil
	}

	return FormatMemoryRecall(recalls, topK), nil
}

// 兼容编译期检查：确保 longTermMemoryTool 实现 tool.BaseTool 接口
var _ tool.BaseTool = (*longTermMemoryTool)(nil)
