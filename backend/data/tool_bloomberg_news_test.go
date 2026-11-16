package data

import (
	"strings"
	"testing"
)

func TestBloombergNewsLegacyToolRegistration(t *testing.T) {
	if toolHandlers["GetBloombergMarketNews"] == nil {
		t.Fatal("Bloomberg news handler not registered")
	}
	if dataToolGroupMap["GetBloombergMarketNews"] != dataToolGroupNewsResearch {
		t.Fatal("Bloomberg news tool has wrong group")
	}
	if !classifyDataToolGroups("彭博")[dataToolGroupNewsResearch] {
		t.Fatal("Bloomberg question must enable news tools")
	}
	if !IsToolKeyConfigured("GetBloombergMarketNews") {
		t.Fatal("Bloomberg news tool should not require an API key")
	}
	var messages []map[string]any
	content, reasoning := &strings.Builder{}, &strings.Builder{}
	ctx := &ToolContext{
		Messages: &messages, CurrentAIContent: content, ReasoningContentText: reasoning,
		Ch: make(chan map[string]any, 1), FuncName: "GetBloombergMarketNews",
	}
	if err := handleBloombergMarketNews(nil, `{"limit":0}`, ctx); err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 || !strings.Contains(messages[len(messages)-1]["content"].(string), "1 至 20") {
		t.Fatalf("invalid argument was not returned to the model: %v", messages)
	}
}
