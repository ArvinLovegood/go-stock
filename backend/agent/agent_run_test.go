package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAgentRunReservesToolBudgetAcrossConcurrentCalls(t *testing.T) {
	ctx, run := NewAgentRunContext(context.Background(), "test", "session-1", AgentRunBudget{
		MaxDuration:  time.Minute,
		MaxToolCalls: 2,
	})
	if AgentRunFromContext(ctx) != run {
		t.Fatal("run should be available from context")
	}

	if err := run.ReserveTool(); err != nil {
		t.Fatalf("first reservation failed: %v", err)
	}
	if err := run.ReserveTool(); err != nil {
		t.Fatalf("second reservation failed: %v", err)
	}
	if err := run.ReserveTool(); err == nil {
		t.Fatal("third reservation should exceed budget")
	}
	if got := run.ToolCalls(); got != 2 {
		t.Fatalf("tool calls = %d, want 2", got)
	}

	run.Finish(nil)
	if got := run.State(); got != AgentRunComplete {
		t.Fatalf("state = %s, want %s", got, AgentRunComplete)
	}
}

func TestAgentRunDeadlineIsApplied(t *testing.T) {
	ctx, run := NewAgentRunContext(context.Background(), "test", "", AgentRunBudget{
		MaxDuration:  10 * time.Millisecond,
		MaxToolCalls: 1,
	})
	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("run context did not reach its deadline")
	}
	if run.State() != AgentRunCreated {
		t.Fatalf("state changed unexpectedly: %s", run.State())
	}
}

// 智能预算分档：短问题基础档，长问题/多标的/DeepAgents 逐级升档，组合复杂问题到顶。
func TestEstimateAgentRunBudget(t *testing.T) {
	longQuestion := strings.Repeat("请综合分析贵州茅台以及五粮液的估值并对比", 10) // >80 字 + 多标的 + 综合报告意图

	cases := []struct {
		name     string
		question string
		mode     Mode
		duration time.Duration
		tools    int
	}{
		// 短问句（报价类）+ React：基础档
		{"simple quote react", "600519 现在多少钱", React, 10 * time.Minute, 100},
		// 长问题（>80 字）+ PlanExecute：1 分 → 仍是基础档
		{"long question plan", strings.Repeat("详细说明一下市场情况 ", 10), PlanExecute, 10 * time.Minute, 100},
		// DeepAgents + 长问题：2 分 → 中档
		{"deepagents long", strings.Repeat("详细说明一下市场情况 ", 10), DeepAgents, 20 * time.Minute, 150},
		// 综合报告 + 多标的 + 长问题 + DeepAgents：≥4 分 → 顶档
		{"comprehensive deep", longQuestion, DeepAgents, 30 * time.Minute, 200},
	}
	for _, c := range cases {
		b := estimateAgentRunBudget(c.question, c.mode)
		if b.MaxDuration != c.duration || b.MaxToolCalls != c.tools {
			t.Errorf("%s: budget = %v/%d, want %v/%d", c.name, b.MaxDuration, b.MaxToolCalls, c.duration, c.tools)
		}
	}
}
