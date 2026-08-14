package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// summaryModelCtxKey 用于在 context 中传递摘要用的 chatModel。
type summaryModelCtxKey struct{}

// WithSummaryModel 将主对话 chatModel 注入 context，供 trimToolResult 在
// 超长工具结果时调用 LLM 生成摘要。为 nil 时不注入。
func WithSummaryModel(ctx context.Context, m model.ToolCallingChatModel) context.Context {
	if m == nil {
		return ctx
	}
	return context.WithValue(ctx, summaryModelCtxKey{}, m)
}

// SummaryModelFromCtx 从 context 提取摘要模型，不存在返回 nil。
func SummaryModelFromCtx(ctx context.Context) model.ToolCallingChatModel {
	if ctx == nil {
		return nil
	}
	m, _ := ctx.Value(summaryModelCtxKey{}).(model.ToolCallingChatModel)
	return m
}

// llmSummaryMaxChars 摘要最大字符数（约 500 字 + 安全边际）。
const llmSummaryMaxChars = 800

// llmSummaryTimeout 摘要 LLM 调用超时。
// 故意基于 context.Background() 派生而非父 ctx：摘要属于独立的旁路任务，
// 若父 ctx 已接近/超过其自身 deadline，WithTimeout 会立即失效导致频繁降级。
const llmSummaryTimeout = 30 * time.Second

// llmSummarizeToolResult 调用 chatModel 生成工具结果摘要。
//
// 触发条件：工具结果正文 > 4000 token（由 trimToolResult 判断）。
// 摘要要求：保留具体数字与关键指标，500 字以内。
// 失败处理：返回空字符串，调用方降级到 smartContentCompress 规则压缩。
//
// 入参 content 为已分离元数据前缀的正文。
func llmSummarizeToolResult(ctx context.Context, content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	m := SummaryModelFromCtx(ctx)
	if m == nil {
		return ""
	}
	// 控制输入长度，避免摘要请求本身超出上下文（截到 12000 字符约 4000-6000 token）
	input := content
	if len([]rune(input)) > 12000 {
		input = string([]rune(input)[:6000]) + "\n...(中间部分省略)...\n" + string([]rune(input)[len([]rune(input))-6000:])
	}

	prompt := fmt.Sprintf(`请用 500 字以内总结以下工具返回数据的关键指标和结论，必须保留所有具体数字（金额、比例、日期、股票代码等）。
不要添加任何分析或建议，只客观总结数据要点。

工具返回数据：
%s`, input)

	msgs := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	// 基于独立上下文设置超时，避免受父 ctx deadline 影响，保证摘要有完整调用窗口
	sumCtx, cancel := context.WithTimeout(context.Background(), llmSummaryTimeout)
	defer cancel()

	resp, err := m.Generate(sumCtx, msgs)
	if err != nil {
		logger.SugaredLogger.Warnf("llmSummarizeToolResult generate failed: %v (fallback to rule-based compress)", err)
		return ""
	}
	if resp == nil {
		return ""
	}
	// 兼容思考模型：Content 为空时取 ReasoningContent
	summary := resp.Content
	if summary == "" && resp.ReasoningContent != "" {
		summary = resp.ReasoningContent
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	// 控制摘要长度，避免超出预算
	if len([]rune(summary)) > llmSummaryMaxChars {
		summary = string([]rune(summary)[:llmSummaryMaxChars]) + "..."
	}
	summary += "\n\n[以上为 LLM 摘要，保留关键数字与结论]"
	return summary
}
