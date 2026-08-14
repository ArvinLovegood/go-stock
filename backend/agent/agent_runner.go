package agent

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// AgentExecutionInput 描述一次 Agent 执行所需的公共输入。
// 不同执行模式只负责自己的事件处理，运行生命周期由 AgentRunner 统一管理。
type AgentExecutionInput struct {
	StockAgent      *StockAiAgent
	Messages        []*schema.Message
	Channel         chan *schema.Message
	MemoryService   *ChatMemoryService
	HistoryMessages []*schema.Message
	SystemPrompt    string
	Question        string
	AIConfigID      int
	ThinkingMode    bool
}

// AgentRunner 统一管理 React、PlanExecute、DeepAgents 的单轮生命周期。
// 执行器本身仍可独立演进，但状态、快照和收尾逻辑不再散落在 API 层。
type AgentRunner struct {
	ctx         context.Context
	run         *AgentRun
	checkpoints *AgentRunCheckpointStore
}

// NewAgentRunner 创建带超时、预算和快照能力的运行上下文。
func NewAgentRunner(parent context.Context, question, sessionID string, budget AgentRunBudget, rootDir string) (context.Context, *AgentRunner) {
	ctx, run := NewAgentRunContext(parent, question, sessionID, budget)
	checkpoints := NewAgentRunCheckpointStore(rootDir)
	run.SetCheckpointStore(checkpoints)
	return ctx, &AgentRunner{ctx: ctx, run: run, checkpoints: checkpoints}
}

func (r *AgentRunner) Context() context.Context {
	if r == nil {
		return context.Background()
	}
	return r.ctx
}

func (r *AgentRunner) Run() *AgentRun {
	if r == nil {
		return nil
	}
	return r.run
}

func (r *AgentRunner) Start(mode Mode) {
	if r == nil || r.run == nil {
		return
	}
	r.run.SetMode(mode)
	r.run.SetPhase("starting")
	r.run.SetState(AgentRunRunning)
}

func (r *AgentRunner) SetPhase(phase string) {
	if r == nil || r.run == nil {
		return
	}
	r.run.SetPhase(phase)
}

// Execute 只负责选择执行器，不负责改变执行器内部的事件处理逻辑。
func (r *AgentRunner) Execute(input AgentExecutionInput) {
	if r == nil || r.run == nil || input.StockAgent == nil || input.StockAgent.instance == nil {
		return
	}
	r.SetPhase("executing")
	switch input.StockAgent.instance.Mode {
	case PlanExecute:
		r.SetPhase("planning")
		runPlanExecuteWithFallback(r.ctx, input.StockAgent, input.Messages, input.Channel, input.MemoryService, input.HistoryMessages, input.SystemPrompt, input.Question, input.AIConfigID, input.ThinkingMode)
	case DeepAgents:
		r.SetPhase("planning")
		runDeepAgents(r.ctx, input.StockAgent, input.Messages, input.Channel, input.MemoryService, input.HistoryMessages, input.SystemPrompt, input.Question)
	default:
		r.SetPhase("executing")
		runReact(r.ctx, input.StockAgent, input.Messages, input.Channel, input.MemoryService, input.HistoryMessages, input.SystemPrompt, input.Question)
	}
}

func (r *AgentRunner) Finish() {
	if r == nil || r.run == nil {
		return
	}
	r.run.FinishFromContext(r.ctx)
	r.run.SetPhase("finished")
}

// SetAgentRunPhase 供执行器内部的阶段检测使用，避免执行器直接依赖 Runner 实例。
func SetAgentRunPhase(ctx context.Context, phase string) {
	if run := AgentRunFromContext(ctx); run != nil {
		run.SetPhase(phase)
	}
}
