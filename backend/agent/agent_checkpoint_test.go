package agent

import (
	"context"
	"testing"
	"time"
)

func TestAgentRunCheckpointStoreRoundTrip(t *testing.T) {
	ctx, run := NewAgentRunContext(context.Background(), "分析 600519", "s1", AgentRunBudget{
		MaxDuration:  time.Minute,
		MaxToolCalls: 3,
	})
	_ = ctx
	run.SetMode(PlanExecute)
	run.SetAIConfigID(7)
	run.SetPhase("planning")
	RecordAgentRunTool(ctx, "GetStockQuote", "ok", `{"stockCode":"600519"}`, "价格=100")
	if err := run.ReserveTool(); err != nil {
		t.Fatalf("reserve tool: %v", err)
	}

	store := NewAgentRunCheckpointStore(t.TempDir())
	if err := store.Save(run.Snapshot()); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	got, err := store.Load(run.ID)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if got.ID != run.ID || got.Mode != PlanExecute || got.AIConfigID != 7 || got.ToolCalls != 1 || got.Phase != "planning" || len(got.Events) < 2 {
		t.Fatalf("unexpected checkpoint: %+v", got)
	}
	if err := store.Delete(run.ID); err != nil {
		t.Fatalf("delete checkpoint: %v", err)
	}
}

func TestAgentRunCheckpointListIncomplete(t *testing.T) {
	store := NewAgentRunCheckpointStore(t.TempDir())
	ctx, run := NewAgentRunContext(context.Background(), "未完成任务", "s1", AgentRunBudget{MaxDuration: time.Minute, MaxToolCalls: 3})
	run.SetCheckpointStore(store)
	run.SetMode(React)
	run.SetState(AgentRunRunning)
	if err := store.Save(run.Snapshot()); err != nil {
		t.Fatalf("save incomplete checkpoint: %v", err)
	}

	items, err := store.ListIncomplete()
	if err != nil {
		t.Fatalf("list incomplete checkpoints: %v", err)
	}
	if len(items) != 1 || items[0].ID != run.ID {
		t.Fatalf("unexpected incomplete checkpoints: %+v", items)
	}
	_ = ctx
	run.Finish(nil)
	items, err = store.ListIncomplete()
	if err != nil {
		t.Fatalf("list finished checkpoints: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("finished run should not be listed: %+v", items)
	}
}
