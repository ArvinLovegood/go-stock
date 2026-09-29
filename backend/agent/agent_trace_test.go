package agent

import (
	"strings"
	"testing"
	"time"
)

// TestAggregatedToolStats 验证按工具聚合统计：次数、状态分布、平均/最大耗时、
// 按调用次数降序排列。
func TestAggregatedToolStats(t *testing.T) {
	trace := &AgentTurnTrace{StartedAt: time.Now()}
	trace.RecordToolCallWithElapsed("GetStockKLine", "ok", `{"a":1}`, 100*time.Millisecond)
	trace.RecordToolCallWithElapsed("GetStockKLine", "ok", `{"a":2}`, 300*time.Millisecond)
	trace.RecordToolCallWithElapsed("GetStockKLine", "error", `{"a":3}`, 50*time.Millisecond)
	trace.RecordToolCallWithElapsed("SearchStockByIndicators", "ok", "{}", 200*time.Millisecond)
	trace.RecordToolCall("GetStockKLine", "cached", `{"a":1}`) // 缓存命中无耗时

	got := trace.aggregatedToolStatsLocked()

	// GetStockKLine 调用 4 次（含 1 次缓存命中），应排在最前
	if !strings.HasPrefix(got, "GetStockKLine×4") {
		t.Errorf("聚合统计应按调用次数降序且以 GetStockKLine×4 开头, got: %s", got)
	}
	if !strings.Contains(got, "ok=2") || !strings.Contains(got, "err=1") || !strings.Contains(got, "cached=1") {
		t.Errorf("状态分布不正确: %s", got)
	}
	// 平均耗时 (100+300+50+0)/4 = 112.5ms
	if !strings.Contains(got, "avg=112ms") && !strings.Contains(got, "avg=113ms") {
		t.Errorf("平均耗时应约 112.5ms: %s", got)
	}
	if !strings.Contains(got, "max=300ms") {
		t.Errorf("最大耗时应为 300ms: %s", got)
	}
	if !strings.Contains(got, "SearchStockByIndicators×1") {
		t.Errorf("第二个工具聚合缺失: %s", got)
	}
}

// TestLogSummaryAggregationSwitch 调用次数 >10 时走聚合分支（不 panic 即通过），
// ≤10 时保留逐条明细。
func TestLogSummaryAggregationSwitch(t *testing.T) {
	small := &AgentTurnTrace{StartedAt: time.Now()}
	for i := 0; i < 3; i++ {
		small.RecordToolCallWithElapsed("GetStockKLine", "ok", "{}", time.Millisecond)
	}
	small.LogSummary("react") // 逐条分支

	big := &AgentTurnTrace{StartedAt: time.Now()}
	for i := 0; i < 15; i++ {
		big.RecordToolCallWithElapsed("GetStockKLine", "ok", "{}", time.Millisecond)
	}
	big.LogSummary("react") // 聚合分支

	var nilTrace *AgentTurnTrace
	nilTrace.LogSummary("react") // nil 安全
}
