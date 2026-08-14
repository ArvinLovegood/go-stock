package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AgentRunState 描述一次 Agent 运行的生命周期。
// 运行状态独立于 React/PlanExecute/DeepAgents，避免每种执行器各自维护一套状态。
type AgentRunState string

const (
	AgentRunCreated  AgentRunState = "created"
	AgentRunRunning  AgentRunState = "running"
	AgentRunWaiting  AgentRunState = "waiting"
	AgentRunComplete AgentRunState = "complete"
	AgentRunFailed   AgentRunState = "failed"
	AgentRunCanceled AgentRunState = "canceled"
)

// AgentRunBudget 是单轮运行的硬预算。
// 预算先用于工具调用次数和生命周期状态，后续可继续扩展到模型请求数和成本。
type AgentRunBudget struct {
	MaxDuration  time.Duration
	MaxToolCalls int
}

func defaultAgentRunBudget() AgentRunBudget {
	return AgentRunBudget{
		MaxDuration:  10 * time.Minute,
		MaxToolCalls: 100,
	}
}

type agentRunContextKey struct{}

var agentRunSequence uint64

// AgentRun 贯穿单次用户请求，供不同执行模式、工具中间件和进度层共享。
type AgentRun struct {
	ID         string
	SessionID  string
	Question   string
	AIConfigID int
	Mode       Mode
	Phase      string
	StartedAt  time.Time
	UpdatedAt  time.Time
	Budget     AgentRunBudget

	mu         sync.Mutex
	state      AgentRunState
	toolCalls  int
	cancel     context.CancelFunc
	checkpoint *AgentRunCheckpointStore
	events     []AgentRunEvent
}

// NewAgentRunContext 为当前请求建立统一运行上下文。
func NewAgentRunContext(parent context.Context, question, sessionID string, budget AgentRunBudget) (context.Context, *AgentRun) {
	if parent == nil {
		parent = context.Background()
	}
	if budget.MaxDuration <= 0 || budget.MaxDuration > 30*time.Minute {
		budget.MaxDuration = defaultAgentRunBudget().MaxDuration
	}
	if budget.MaxToolCalls <= 0 {
		budget.MaxToolCalls = defaultAgentRunBudget().MaxToolCalls
	}

	ctx, cancel := context.WithTimeout(parent, budget.MaxDuration)
	seq := atomic.AddUint64(&agentRunSequence, 1)
	run := &AgentRun{
		ID:        fmt.Sprintf("run-%d-%d", time.Now().UnixNano(), seq),
		SessionID: sessionID,
		Question:  question,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
		Budget:    budget,
		state:     AgentRunCreated,
		cancel:    cancel,
	}
	return context.WithValue(ctx, agentRunContextKey{}, run), run
}

func AgentRunFromContext(ctx context.Context) *AgentRun {
	if ctx == nil {
		return nil
	}
	run, _ := ctx.Value(agentRunContextKey{}).(*AgentRun)
	return run
}

func (r *AgentRun) SetState(state AgentRunState) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.state = state
	r.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.persist()
}

func (r *AgentRun) SetMode(mode Mode) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.Mode = mode
	r.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.persist()
}

func (r *AgentRun) SetAIConfigID(id int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.AIConfigID = id
	r.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.persist()
}

func (r *AgentRun) SetCheckpointStore(store *AgentRunCheckpointStore) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.checkpoint = store
	r.mu.Unlock()
}

func (r *AgentRun) SetPhase(phase string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.Phase == phase {
		r.mu.Unlock()
		return
	}
	r.Phase = phase
	r.UpdatedAt = time.Now()
	r.events = appendRunEvent(r.events, AgentRunEvent{
		Type: "phase", Name: phase, At: r.UpdatedAt,
	})
	r.mu.Unlock()
	r.persist()
}

// RecordToolResult 保存工具调用的轻量摘要，结果正文最多保留 500 个字节。
func (r *AgentRun) RecordToolResult(name, status, args, result string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	now := time.Now()
	r.events = appendRunEvent(r.events, AgentRunEvent{
		Type: "tool", Name: name, Status: status, At: now,
		ArgPreview: truncateRunEvent(args, 160), ResultPreview: truncateRunEvent(result, 500),
	})
	r.UpdatedAt = now
	r.mu.Unlock()
	r.persist()
}

func appendRunEvent(events []AgentRunEvent, event AgentRunEvent) []AgentRunEvent {
	event.Sequence = len(events) + 1
	events = append(events, event)
	const maxRunEvents = 100
	if len(events) > maxRunEvents {
		events = events[len(events)-maxRunEvents:]
		for i := range events {
			events[i].Sequence = i + 1
		}
	}
	return events
}

func truncateRunEvent(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= maxLen {
		return value
	}
	runes := []rune(value)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	return string(runes) + "..."
}

func (r *AgentRun) persist() {
	if r == nil {
		return
	}
	r.mu.Lock()
	store := r.checkpoint
	r.mu.Unlock()
	if store != nil {
		_ = store.Save(r.Snapshot())
	}
}

func (r *AgentRun) Snapshot() AgentRunSnapshot {
	if r == nil {
		return AgentRunSnapshot{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return AgentRunSnapshot{
		ID: r.ID, SessionID: r.SessionID, Question: r.Question, AIConfigID: r.AIConfigID, Mode: r.Mode,
		Phase: r.Phase, State: r.state, StartedAt: r.StartedAt, UpdatedAt: r.UpdatedAt,
		Budget: r.Budget, ToolCalls: r.toolCalls,
		Events: append([]AgentRunEvent(nil), r.events...),
	}
}

func (r *AgentRun) State() AgentRunState {
	if r == nil {
		return AgentRunFailed
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *AgentRun) Finish(err error) {
	if r == nil {
		return
	}
	if err != nil {
		r.SetState(AgentRunFailed)
		if r.cancel != nil {
			r.cancel()
		}
		return
	}
	r.SetState(AgentRunComplete)
	if r.cancel != nil {
		r.cancel()
	}
}

// FinishFromContext 将调用方取消和运行超时反映到运行状态，避免已取消的任务
// 在统一收尾时被误记为 complete。
func (r *AgentRun) FinishFromContext(ctx context.Context) {
	if ctx != nil && ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.SetState(AgentRunCanceled)
			if r.cancel != nil {
				r.cancel()
			}
			return
		}
	}
	r.Finish(nil)
}

// ReserveTool 为一次工具调用预留预算。并行工具调用也通过同一把锁计数。
func (r *AgentRun) ReserveTool() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.state == AgentRunCanceled {
		r.mu.Unlock()
		return fmt.Errorf("agent run %s 已取消", r.ID)
	}
	if r.Budget.MaxToolCalls > 0 && r.toolCalls >= r.Budget.MaxToolCalls {
		r.mu.Unlock()
		return fmt.Errorf("本轮工具调用已达到上限 %d", r.Budget.MaxToolCalls)
	}
	r.toolCalls++
	r.state = AgentRunRunning
	r.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.persist()
	return nil
}

func (r *AgentRun) ToolCalls() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.toolCalls
}

func (r *AgentRun) Elapsed() time.Duration {
	if r == nil {
		return 0
	}
	return time.Since(r.StartedAt)
}

// RecordAgentRunTool 将工具调用摘要写入当前运行快照。
func RecordAgentRunTool(ctx context.Context, name, status, args, result string) {
	if run := AgentRunFromContext(ctx); run != nil {
		run.RecordToolResult(name, status, args, result)
	}
}
