package agent

import (
	"context"
	"testing"
	"time"
)

func TestAgentRunnerPersistsLifecycleAndPhases(t *testing.T) {
	ctx, runner := NewAgentRunner(context.Background(), "分析 600519", "session-1", AgentRunBudget{
		MaxDuration:  time.Minute,
		MaxToolCalls: 2,
	}, t.TempDir())
	runner.Start(PlanExecute)

	store := runner.checkpoints
	snapshot, err := store.Load(runner.Run().ID)
	if err != nil {
		t.Fatalf("load start snapshot: %v", err)
	}
	if snapshot.State != AgentRunRunning || snapshot.Mode != PlanExecute {
		t.Fatalf("unexpected start snapshot: %+v", snapshot)
	}

	SetAgentRunPhase(ctx, "tool_calling")
	if err := runner.Run().ReserveTool(); err != nil {
		t.Fatalf("reserve tool: %v", err)
	}
	runner.Finish()

	snapshot, err = store.Load(runner.Run().ID)
	if err != nil {
		t.Fatalf("load finish snapshot: %v", err)
	}
	if snapshot.State != AgentRunComplete || snapshot.Phase != "finished" || snapshot.ToolCalls != 1 {
		t.Fatalf("unexpected finish snapshot: %+v", snapshot)
	}
}
