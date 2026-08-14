package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

type agentTurnTraceKey struct{}

// ToolCallRecord 单次工具调用记录。
type ToolCallRecord struct {
	Name       string
	Status     string
	At         time.Time
	ArgPreview string
	Elapsed    time.Duration // 工具调用耗时（含预告/摘要发送）
}

// AgentTurnTrace 单轮对话工具调用链路，用于可观测性（日志记录）。
//
// 注意：工具由 eino ToolsNode 默认并行执行，多个 tool_calls 会在不同 goroutine
// 中同时调用 RecordToolCall / AccumulateUsage，因此所有读写共享字段的路径都必须加锁。
type AgentTurnTrace struct {
	mu sync.Mutex

	Question  string
	StartedAt time.Time
	ToolCalls []ToolCallRecord
	// InputTokens 累计输入 token（来自模型响应的 ResponseMeta.Usage.PromptTokens）
	InputTokens int
	// OutputTokens 累计输出 token（来自模型响应的 ResponseMeta.Usage.CompletionTokens）
	OutputTokens int
}

func NewAgentTurnTrace(ctx context.Context, question string) (context.Context, *AgentTurnTrace) {
	trace := &AgentTurnTrace{
		Question:  question,
		StartedAt: time.Now(),
	}
	return context.WithValue(ctx, agentTurnTraceKey{}, trace), trace
}

// AccumulateUsage 累计单次模型响应的 token 用量。
// 多次模型调用（ReAct 多步、PlanExecute 规划/执行/重规划）会累加。
// usage 为 nil 时静默跳过。
func (t *AgentTurnTrace) AccumulateUsage(usage *schema.TokenUsage) {
	if t == nil || usage == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.InputTokens += usage.PromptTokens
	t.OutputTokens += usage.CompletionTokens
}

func AgentTurnTraceFromContext(ctx context.Context) *AgentTurnTrace {
	if ctx == nil {
		return nil
	}
	trace, _ := ctx.Value(agentTurnTraceKey{}).(*AgentTurnTrace)
	return trace
}

func (t *AgentTurnTrace) RecordToolCall(name, status, args string) {
	t.RecordToolCallWithElapsed(name, status, args, 0)
}

// RecordToolCallWithElapsed 记录工具调用，含耗时。
// 该方法在工具并行执行的 goroutine 中被并发调用，必须加锁。
func (t *AgentTurnTrace) RecordToolCallWithElapsed(name, status, args string, elapsed time.Duration) {
	if t == nil || name == "" {
		return
	}
	preview := strings.TrimSpace(args)
	if len(preview) > 120 {
		preview = preview[:120] + "..."
	}
	rec := ToolCallRecord{
		Name:       name,
		Status:     status,
		At:         time.Now(),
		ArgPreview: preview,
		Elapsed:    elapsed,
	}
	t.mu.Lock()
	t.ToolCalls = append(t.ToolCalls, rec)
	t.mu.Unlock()
}

// SuccessfulToolCallCount 返回本轮成功工具调用数，供最终事实校验使用。
func (t *AgentTurnTrace) SuccessfulToolCallCount() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, call := range t.ToolCalls {
		if call.Status == "ok" {
			count++
		}
	}
	return count
}

func (t *AgentTurnTrace) LogSummary(mode string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	duration := time.Since(t.StartedAt)
	toolCalls := len(t.ToolCalls)
	inTokens := t.InputTokens
	outTokens := t.OutputTokens
	toolNames := t.ToolNamesLocked()
	t.mu.Unlock()
	logger.SugaredLogger.Infof(
		"agent turn trace: mode=%s question=%q duration=%s tools=%d tokens(in=%d/out=%d/total=%d) [%s]",
		mode,
		truncateForLog(t.Question, 80),
		duration.Round(time.Millisecond),
		toolCalls,
		inTokens,
		outTokens,
		inTokens+outTokens,
		strings.Join(toolNames, ", "),
	)
}

// ToolNamesLocked 返回工具名列表，调用方必须已持有 t.mu。
func (t *AgentTurnTrace) ToolNamesLocked() []string {
	names := make([]string, 0, len(t.ToolCalls))
	for _, tc := range t.ToolCalls {
		if tc.Elapsed > 0 {
			names = append(names, fmt.Sprintf("%s(%s,%s)", tc.Name, tc.Status, tc.Elapsed.Round(time.Millisecond)))
		} else {
			names = append(names, fmt.Sprintf("%s(%s)", tc.Name, tc.Status))
		}
	}
	return names
}

// StatsMessage 生成面向前端的统计摘要消息（通过 ReasoningContent 发送）。
// 包含：工具调用次数、累计 token、总耗时。
func (t *AgentTurnTrace) StatsMessage() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	duration := time.Since(t.StartedAt).Round(time.Millisecond)
	return fmt.Sprintf("[STATS]📊 本轮统计：工具调用 %d 次｜输入 %d token｜输出 %d token｜耗时 %s\n",
		len(t.ToolCalls), t.InputTokens, t.OutputTokens, duration)
}

func truncateForLog(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
