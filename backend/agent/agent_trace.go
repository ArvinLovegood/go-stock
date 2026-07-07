package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"
)

type agentTurnTraceKey struct{}

// ToolCallRecord 单次工具调用记录。
type ToolCallRecord struct {
	Name       string
	Status     string
	At         time.Time
	ArgPreview string
}

// AgentTurnTrace 单轮对话工具调用链路，用于可观测性（日志记录）。
type AgentTurnTrace struct {
	Question  string
	StartedAt time.Time
	ToolCalls []ToolCallRecord
}

func NewAgentTurnTrace(ctx context.Context, question string) (context.Context, *AgentTurnTrace) {
	trace := &AgentTurnTrace{
		Question:  question,
		StartedAt: time.Now(),
	}
	return context.WithValue(ctx, agentTurnTraceKey{}, trace), trace
}

func AgentTurnTraceFromContext(ctx context.Context) *AgentTurnTrace {
	if ctx == nil {
		return nil
	}
	trace, _ := ctx.Value(agentTurnTraceKey{}).(*AgentTurnTrace)
	return trace
}

func (t *AgentTurnTrace) RecordToolCall(name, status, args string) {
	if t == nil || name == "" {
		return
	}
	preview := strings.TrimSpace(args)
	if len(preview) > 120 {
		preview = preview[:120] + "..."
	}
	t.ToolCalls = append(t.ToolCalls, ToolCallRecord{
		Name:       name,
		Status:     status,
		At:         time.Now(),
		ArgPreview: preview,
	})
}

func (t *AgentTurnTrace) ToolNames() []string {
	if t == nil {
		return nil
	}
	names := make([]string, 0, len(t.ToolCalls))
	for _, tc := range t.ToolCalls {
		names = append(names, fmt.Sprintf("%s(%s)", tc.Name, tc.Status))
	}
	return names
}

func (t *AgentTurnTrace) LogSummary(mode string) {
	if t == nil {
		return
	}
	duration := time.Since(t.StartedAt)
	logger.SugaredLogger.Infof(
		"agent turn trace: mode=%s question=%q duration=%s tools=%d [%s]",
		mode,
		truncateForLog(t.Question, 80),
		duration.Round(time.Millisecond),
		len(t.ToolCalls),
		strings.Join(t.ToolNames(), ", "),
	)
}

func truncateForLog(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
