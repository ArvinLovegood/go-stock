package agent

import (
	"context"
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
