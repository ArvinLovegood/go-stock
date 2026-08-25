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

// 智能预算分档：执行时间一律不限（MaxDuration=0），仅工具调用次数按复杂度分档。
func TestEstimateAgentRunBudget(t *testing.T) {
	longQuestion := strings.Repeat("请综合分析贵州茅台以及五粮液的估值并对比", 10) // >80 字 + 多标的 + 综合报告意图

	cases := []struct {
		name     string
		question string
		mode     Mode
		tools    int
	}{
		// 短问句（报价类）+ React：基础档
		{"simple quote react", "600519 现在多少钱", React, 100},
		// 长问题（>80 字）+ PlanExecute：1 分 → 仍是基础档
		{"long question plan", strings.Repeat("详细说明一下市场情况 ", 10), PlanExecute, 100},
		// DeepAgents + 长问题：2 分 → 中档
		{"deepagents long", strings.Repeat("详细说明一下市场情况 ", 10), DeepAgents, 150},
		// 综合报告 + 多标的 + 长问题 + DeepAgents：≥4 分 → 顶档
		{"comprehensive deep", longQuestion, DeepAgents, 200},
	}
	for _, c := range cases {
		b := estimateAgentRunBudget(c.question, c.mode)
		if b.MaxDuration != 0 {
			t.Errorf("%s: duration = %v, want 0（不限时）", c.name, b.MaxDuration)
		}
		if b.MaxToolCalls != c.tools {
			t.Errorf("%s: maxToolCalls = %d, want %d", c.name, b.MaxToolCalls, c.tools)
		}
	}
}

// MaxDuration=0 表示不限时：ctx 不应携带 deadline，避免复杂任务被时间上限掐断。
func TestAgentRunZeroDurationMeansNoDeadline(t *testing.T) {
	ctx, run := NewAgentRunContext(context.Background(), "test", "s1", AgentRunBudget{MaxDuration: 0, MaxToolCalls: 5})
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("MaxDuration=0 时 ctx 不应设置 deadline")
	}
	select {
	case <-ctx.Done():
		t.Fatal("ctx 不应已结束")
	default:
	}
	run.Finish(nil)
}
