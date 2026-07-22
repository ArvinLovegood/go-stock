package agent

import (
	"context"
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestGPTAgentRequestUsesModernReasoningParameters(t *testing.T) {
	requestBody := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requestBody <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-5.6-sol","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	db.Init("file:agent-policy-test?mode=memory&cache=shared")
	chatModel, err := createChatModel(context.Background(), data.AIConfig{
		BaseUrl:         server.URL + "/v1",
		ApiKey:          "test-key",
		ModelName:       "gpt-5.6-sol",
		Thinking:        true,
		MaxOutputTokens: 16384,
		ReasoningEffort: "medium",
		TemperatureOpt:  nil,
		TimeOut:         30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}); err != nil {
		t.Fatal(err)
	}
	body := <-requestBody
	if body["reasoning_effort"] != "medium" {
		t.Fatalf("reasoning_effort = %#v", body["reasoning_effort"])
	}
	if body["max_completion_tokens"] != float64(16384) {
		t.Fatalf("max_completion_tokens = %#v", body["max_completion_tokens"])
	}
	if _, ok := body["max_tokens"]; ok {
		t.Fatalf("modern GPT request unexpectedly contains max_tokens: %#v", body["max_tokens"])
	}
	if _, ok := body["temperature"]; ok {
		t.Fatalf("modern GPT request unexpectedly contains temperature: %#v", body["temperature"])
	}
}

func TestGrokAgentRequestUsesModernReasoningParameters(t *testing.T) {
	requestBody := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requestBody <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"grok-4.5","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	db.Init("file:agent-grok-policy-test?mode=memory&cache=shared")
	chatModel, err := createChatModel(context.Background(), data.AIConfig{
		BaseUrl:         server.URL + "/v1",
		ApiKey:          "test-key",
		ModelName:       "grok-4.5",
		Thinking:        true,
		ReasoningEffort: "medium",
		TimeOut:         30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}); err != nil {
		t.Fatal(err)
	}
	body := <-requestBody
	if body["reasoning_effort"] != "medium" || body["max_completion_tokens"] != float64(8192) {
		t.Fatalf("unexpected Grok policy body: %#v", body)
	}
	if body["temperature"] != 0.2 {
		t.Fatalf("temperature = %#v, want 0.2", body["temperature"])
	}
	if _, ok := body["max_tokens"]; ok {
		t.Fatalf("modern Grok request unexpectedly contains max_tokens: %#v", body["max_tokens"])
	}
}
