package agent

// Eino Agent 路径对 OpenAI 推理系模型（gpt-6.1-sol 等）的兼容性测试。
//
// 背景：自研 SSE 解析器路径（backend/data/openai_tools.go）已修复 gpt-6.1-sol 兼容问题；
// 本文件验证 Eino Agent 模式（agent.go / agent_api.go 经 createChatModel 的路径）：
//  1. 推理系模型识别（isOpenAIReasoningStyleModel）
//  2. 请求参数：推理系模型必须用 max_completion_tokens 而非 max_tokens（OpenAI 400 拒收 max_tokens）
//  3. 非推理模型保持 max_tokens 不变
//  4. 中转常见形态「HTTP 200 + SSE 流内嵌错误帧」在 eino 底层必须返回显式错误而非静默无输出
//
// 运行方式：go test ./backend/agent -run 'TestIsOpenAIReasoningStyleModel|TestCreateChatModel_Reasoning|TestEinoOpenAI_InlineError' -v

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func TestIsOpenAIReasoningStyleModel(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		// 推理系：o 系列
		{"o1", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"O3-Mini", true},
		// 推理系：gpt-5 及以上（含小版本与中转变体名）
		{"gpt-5", true},
		{"gpt-5-mini", true},
		{"gpt-6.1-sol", true},
		{"gpt-6.2", true},
		// 非推理系：仍支持 max_tokens
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4.1", false},
		{"gpt-3.5-turbo", false},
		// 非 OpenAI 系不应误判
		{"deepseek-chat", false},
		{"deepseek-reasoner", false},
		{"claude-3-5-sonnet", false},
		{"qwen-max", false},
		{"ollama3", false},       // 避免 o 系列前缀误匹配
		{"openvoice-v1", false},  // 同上
		{"", false},
	}
	for _, c := range cases {
		if got := isOpenAIReasoningStyleModel(c.model); got != c.want {
			t.Errorf("isOpenAIReasoningStyleModel(%q) = %v, want %v", c.model, got, c.want)
		}
	}
}

// newFactoryTestDB 初始化内存库（buildChatModelHTTPClient 内部 GetSettingConfig 依赖 db.Dao）
func newFactoryTestDB(t *testing.T) {
	t.Helper()
	if db.Dao != nil {
		return
	}
	db.Init("file:agent_chat_model_factory_test?mode=memory&cache=shared&_pragma=busy_timeout(10000)")
}

// captureBodyServer 返回一个记录请求体的 mock OpenAI 服务，并以 SSE 正常回包
func captureBodyServer(t *testing.T) (*httptest.Server, func() map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		captured = body
		mu.Unlock()
		// 按请求的 stream 字段回对应格式：Generate 走非流式 JSON，Stream 走 SSE
		if stream, _ := body["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return captured
	}
}

// TestCreateChatModel_ReasoningModelUsesMaxCompletionTokens 验证 gpt-6.1-sol 走 max_completion_tokens
func TestCreateChatModel_ReasoningModelUsesMaxCompletionTokens(t *testing.T) {
	newFactoryTestDB(t)
	srv, getBody := captureBodyServer(t)

	cm, err := createChatModel(context.Background(), data.AIConfig{
		BaseUrl:       srv.URL + "/v1",
		ApiKey:        "sk-test",
		ModelName:     "gpt-6.1-sol",
		MaxTokens:     8192,
		ContextWindow: 1050000,
		TimeOut:       30,
	})
	if err != nil {
		t.Fatalf("createChatModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("你好")}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	body := getBody()
	if body == nil {
		t.Fatal("未捕获到请求体")
	}
	if _, ok := body["max_tokens"]; ok {
		t.Errorf("推理系模型不应下发 max_tokens 参数: %v", body)
	}
	mct, ok := body["max_completion_tokens"].(float64)
	if !ok {
		t.Fatalf("推理系模型应下发 max_completion_tokens: %v", body)
	}
	if int(mct) != 8192 {
		t.Errorf("max_completion_tokens = %v, want 8192", mct)
	}
}

// TestCreateChatModel_NonReasoningModelKeepsMaxTokens 验证 gpt-4o 等非推理模型保持 max_tokens
func TestCreateChatModel_NonReasoningModelKeepsMaxTokens(t *testing.T) {
	newFactoryTestDB(t)
	srv, getBody := captureBodyServer(t)

	cm, err := createChatModel(context.Background(), data.AIConfig{
		BaseUrl:       srv.URL + "/v1",
		ApiKey:        "sk-test",
		ModelName:     "gpt-4o",
		MaxTokens:     4096,
		ContextWindow: 128000,
		TimeOut:       30,
	})
	if err != nil {
		t.Fatalf("createChatModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("你好")}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	body := getBody()
	if body == nil {
		t.Fatal("未捕获到请求体")
	}
	if mt, ok := body["max_tokens"].(float64); !ok || int(mt) != 4096 {
		t.Errorf("非推理模型应保留 max_tokens=4096: %v", body)
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Errorf("非推理模型不应下发 max_completion_tokens: %v", body)
	}
}

// TestEinoOpenAI_InlineErrorFrameReturnsError 复现真实根因形态：
// 中转返回 HTTP 200 + SSE 流内嵌错误帧（data: {"error":{...}}）。
// 验证 eino 底层（go-openai fork 的 streamReader）将其转为显式错误返回，
// 而非像旧自研解析器那样静默吞掉导致前端长时间无输出。
func TestEinoOpenAI_InlineErrorFrameReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"error":{"message":"Our servers are currently overloaded. Please try again later.","type":"upstream_error"}}` + "\n\n"))
	}))
	defer srv.Close()

	cm, err := einoopenai.NewChatModel(context.Background(), &einoopenai.ChatModelConfig{
		BaseURL: srv.URL,
		Model:   "gpt-6.1-sol",
		APIKey:  "sk-test",
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("你好")})
	if err != nil {
		// Stream 阶段即报错亦可接受（同样是显式错误）
		t.Logf("Stream 阶段返回错误（可接受）: %v", err)
		return
	}
	defer sr.Close()

	_, err = sr.Recv()
	if err == nil {
		t.Fatal("流内嵌错误帧被静默吞掉：Recv 未返回错误（将导致 Agent 无输出直到超时）")
	}
	if !strings.Contains(err.Error(), "overloaded") {
		t.Errorf("错误信息应包含上游原始消息, got: %v", err)
	}
	t.Logf("内嵌错误帧已转为显式错误: %v", err)
}
