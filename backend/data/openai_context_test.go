package data

import (
	"strings"
	"testing"
)

func TestPrepareOpenAIMessagesDropsOldHistoryAndKeepsCurrentQuestion(t *testing.T) {
	messages := []map[string]interface{}{
		{"role": "system", "content": "system rules"},
		{"role": "user", "content": "old question " + strings.Repeat("x", 18000)},
		{"role": "assistant", "content": "old answer " + strings.Repeat("y", 18000)},
		{"role": "user", "content": "current question"},
	}

	got, stats := PrepareOpenAIMessages(messages, nil, 5000)
	if stats.DroppedMessages == 0 && !stats.Truncated {
		t.Fatal("expected old history to be dropped or truncated")
	}
	if roleOf(got[0]) != "system" {
		t.Fatalf("first role = %q, want system", roleOf(got[0]))
	}
	last := got[len(got)-1]
	if last["content"] != "current question" {
		t.Fatalf("last content = %q", last["content"])
	}
	if stats.AfterTokens > stats.BudgetTokens {
		t.Fatalf("after tokens %d exceeds budget %d", stats.AfterTokens, stats.BudgetTokens)
	}
	if messages[1]["content"] == nil || !strings.Contains(messages[1]["content"].(string), "old question") {
		t.Fatal("input messages were mutated")
	}
}

func TestPrepareOpenAIMessagesKeepsToolCallPairWithoutOrphans(t *testing.T) {
	messages := []map[string]interface{}{
		{"role": "system", "content": "system"},
		{"role": "user", "content": "analyze"},
		{"role": "assistant", "content": "", "tool_calls": []map[string]any{{"id": "call_1", "type": "function"}}},
		{"role": "tool", "tool_call_id": "call_1", "content": strings.Repeat("market-data ", 6000)},
	}

	got, stats := PrepareOpenAIMessages(messages, nil, 5000)
	if !stats.Truncated {
		t.Fatal("expected oversized tool result to be truncated")
	}
	assistantIndex := -1
	for i, message := range got {
		if roleOf(message) == "assistant" {
			assistantIndex = i
		}
		if roleOf(message) == "tool" && assistantIndex < 0 {
			t.Fatal("tool message was kept without its assistant tool call")
		}
	}
	if assistantIndex < 0 || roleOf(got[len(got)-1]) != "tool" {
		t.Fatalf("tool call pair not preserved: %#v", got)
	}
	if stats.AfterTokens > stats.BudgetTokens {
		t.Fatalf("after tokens %d exceeds budget %d", stats.AfterTokens, stats.BudgetTokens)
	}
}

func TestPrepareOpenAIMessagesNeverDropsNewestQuestionBehindLargeSystemPrompts(t *testing.T) {
	messages := []map[string]interface{}{
		{"role": "system", "content": strings.Repeat("规则甲", 4000)},
		{"role": "system", "content": strings.Repeat("规则乙", 4000)},
		{"role": "user", "content": strings.Repeat("请分析", 4000) + " CURRENT_QUESTION_END"},
	}

	got, stats := PrepareOpenAIMessages(messages, nil, 1800)
	if len(got) < 2 {
		t.Fatalf("newest question was dropped: %#v", got)
	}
	last := got[len(got)-1]
	if roleOf(last) != "user" || !strings.Contains(last["content"].(string), "CURRENT_QUESTION_END") {
		t.Fatalf("newest question not retained at the end: %#v", last)
	}
	if stats.AfterTokens > stats.BudgetTokens {
		t.Fatalf("after tokens %d exceeds budget %d", stats.AfterTokens, stats.BudgetTokens)
	}
}
