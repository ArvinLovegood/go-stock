package data

// GPT-6.1 Sol（OpenAI 推理模型）兼容性测试。
//
// 背景：用户反馈 AI 助手选择 gpt-6.1-sol 模型后长时间没有任何输出。
// 该模型为 OpenAI 推理模型，与 DeepSeek/GLM 系列在流式协议细节上存在差异。
// 本文件通过本地 httptest 服务模拟 OpenAI 侧多种真实返回形态，
// 直接驱动生产解析代码 AskAi / AskAiWithTools（不经任何 mock 替身），
// 验证 go-stock 流式解析器对 gpt-6.1-sol 的兼容性。
//
// 覆盖场景：
//  1. 标准 SSE 流（基线）
//  2. 推理内容经 delta.reasoning 字段下发（OpenAI 风格，DeepSeek 用 reasoning_content）
//  3. 请求携带 thinking 参数被 OpenAI 400 拒绝（需自动降级重试）
//  4. 请求携带 max_tokens 被推理模型 400 拒绝（要求改用 max_completion_tokens）
//  5. 服务端忽略 stream=true 返回完整非流式 JSON
//  6. 流式 tool_calls 工具调用循环
//  7. SSE 注释行（: keep-alive）与 event: 行穿插
//  8. 工具模式 + thinking 降级组合
//  9. 流内嵌错误帧自动重试（真实根因：中转 HTTP 200 + SSE error 帧）
// 10. 工具模式流内错误帧自动重试
// 11. 已有输出后遇错误帧：推送可见错误而非静默卡死
//
// 运行方式：TEST_DB_SKIP_INIT=1 go test ./backend/data -run TestGPT61Sol -v

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ------------------------------------------------------------------------------------------------
// 测试基础设施
// ------------------------------------------------------------------------------------------------

const compatModel = "gpt-6.1-sol"

// compatStreamResult 汇总一次流式调用在前端视角下的全部可见输出
type compatStreamResult struct {
	Contents  string   // content 增量拼接
	Reasoning string   // reasoning_content 增量拼接
	Errors    []string // code=0 的错误内容
	MsgCount  int      // 收到的总消息数
}

func collectCompatStream(ch <-chan map[string]any) *compatStreamResult {
	res := &compatStreamResult{}
	for msg := range ch {
		res.MsgCount++
		code, _ := msg["code"].(int)
		if c, ok := msg["content"].(string); ok && c != "" {
			if code == 0 {
				res.Errors = append(res.Errors, c)
			} else {
				res.Contents += c
			}
		}
		if r, ok := msg["reasoning_content"].(string); ok {
			res.Reasoning += r
		}
	}
	return res
}

func newCompatOpenAi(serverURL string) *OpenAi {
	return &OpenAi{
		ctx:       context.Background(),
		BaseUrl:   serverURL,
		ApiKey:    "sk-compat-test",
		Model:     compatModel,
		TimeOut:   30,
		MaxTokens: 8192,
	}
}

func compatMessages() []map[string]interface{} {
	return []map[string]interface{}{
		{"role": "system", "content": "你是一个AI助手"},
		{"role": "user", "content": "你好"},
	}
}

// runAskAi / runAskAiWithTools 镜像生产调用方（openai_stream.go）的模式：
// 在独立 goroutine 中执行请求并以 defer close(ch) 结束，否则消费端 range 永远阻塞。
func runAskAi(o *OpenAi, ch chan map[string]any, question string, think bool) {
	go func() {
		defer close(ch)
		AskAi(o, nil, compatMessages(), ch, question, think)
	}()
}

func runAskAiWithTools(o *OpenAi, ch chan map[string]any, question string, tools []Tool, thinkingMode bool) {
	go func() {
		defer close(ch)
		AskAiWithTools(o, nil, compatMessages(), ch, question, tools, thinkingMode)
	}()
}

// sseData 写一条标准 OpenAI data 行
func sseData(payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	return "data: " + string(raw) + "\n\n"
}

func chatChunk(id string, delta map[string]any, finishReason string) map[string]any {
	choice := map[string]any{
		"index":         0,
		"delta":         delta,
		"finish_reason": finishReason,
	}
	return map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   compatModel,
		"choices": []any{choice},
	}
}

// writeSSE 将若干 SSE 帧以 text/event-stream 写回
func writeSSE(w http.ResponseWriter, frames ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	for _, f := range frames {
		_, _ = w.Write([]byte(f))
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 1：标准 SSE 流（基线）
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_StandardStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w,
			sseData(chatChunk("chatcmpl-1", map[string]any{"role": "assistant", "content": ""}, "")),
			sseData(chatChunk("chatcmpl-1", map[string]any{"content": "你好"}, "")),
			sseData(chatChunk("chatcmpl-1", map[string]any{"content": "，世界！"}, "")),
			sseData(chatChunk("chatcmpl-1", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if res.Contents != "你好，世界！" {
		t.Errorf("标准流内容不符, got: %q", res.Contents)
	}
	if len(res.Errors) != 0 {
		t.Errorf("标准流不应有错误: %v", res.Errors)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 2：推理内容经 delta.reasoning 下发（OpenAI 风格）
// gpt-6.1-sol 是深度推理模型，回答前会有较长推理阶段。
// 若推理增量被解析器丢弃，用户在推理阶段看不到任何输出 → 「长时间没输出」的主诉。
// 期望：reasoning 增量必须映射为 reasoning_content 展示，且最终 content 完整。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_ReasoningViaDeltaReasoning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w,
			sseData(chatChunk("chatcmpl-2", map[string]any{"role": "assistant", "content": ""}, "")),
			// 长推理阶段：多个 reasoning 增量，无 content
			sseData(chatChunk("chatcmpl-2", map[string]any{"reasoning": "先分析用户意图…"}, "")),
			sseData(chatChunk("chatcmpl-2", map[string]any{"reasoning": "再检索相关知识…"}, "")),
			sseData(chatChunk("chatcmpl-2", map[string]any{"reasoning": "组织回答结构…"}, "")),
			// 正式输出
			sseData(chatChunk("chatcmpl-2", map[string]any{"content": "答案"}, "")),
			sseData(chatChunk("chatcmpl-2", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if res.Reasoning == "" {
		t.Errorf("delta.reasoning 推理增量被丢弃：推理阶段前端将完全无输出（长时间无输出的根因之一）")
	}
	if res.Contents != "答案" {
		t.Errorf("content 不符, got: %q", res.Contents)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 3：thinking 参数不被 OpenAI 兼容端点支持
// go-stock 思考模式会在请求体携带 {"thinking":{"type":"enabled"}}（DeepSeek/GLM 私有扩展），
// OpenAI 返回 400 Unrecognized request argument supplied: thinking。
// 期望：自动去除 thinking 参数重试并成功，前端不应看到错误。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_ThinkingParamUnsupported(t *testing.T) {
	var sawThinking atomic.Bool
	var sawRetry atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["thinking"]; ok {
			sawThinking.Store(true)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unrecognized request argument supplied: thinking","type":"invalid_request_error","param":"thinking","code":"unknown_parameter"}}`))
			return
		}
		sawRetry.Store(true)
		writeSSE(w,
			sseData(chatChunk("chatcmpl-3", map[string]any{"content": "重试后成功"}, "")),
			sseData(chatChunk("chatcmpl-3", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", true)
	res := collectCompatStream(ch)

	if !sawThinking.Load() {
		t.Log("警告：请求未携带 thinking 参数，测试前提不成立")
	}
	if !sawRetry.Load() {
		t.Errorf("thinking 参数被拒后未成功重试")
	}
	if len(res.Errors) != 0 {
		t.Errorf("thinking 降级重试后前端不应看到错误: %v", res.Errors)
	}
	if !strings.Contains(res.Contents, "重试后成功") {
		t.Errorf("重试内容缺失, got: %q", res.Contents)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 4：max_tokens 参数被推理模型拒绝
// OpenAI 推理模型要求使用 max_completion_tokens，携带 max_tokens 返回 400：
// "Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead."
// go-stock 全部用户配置默认填写了 MaxTokens，此场景命中概率极高。
// 期望：自动改用 max_completion_tokens 重试并成功。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_MaxTokensUnsupported(t *testing.T) {
	var sawMaxCompletionTokens atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["max_tokens"]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.","type":"invalid_request_error","param":"max_tokens","code":"unsupported_parameter"}}`))
			return
		}
		if _, ok := body["max_completion_tokens"]; ok {
			sawMaxCompletionTokens.Store(true)
		}
		writeSSE(w,
			sseData(chatChunk("chatcmpl-4", map[string]any{"content": "max_completion_tokens 重试成功"}, "")),
			sseData(chatChunk("chatcmpl-4", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if !sawMaxCompletionTokens.Load() {
		t.Errorf("max_tokens 被拒后未自动改用 max_completion_tokens 重试（前端将直接看到 400 报错）")
	}
	if len(res.Errors) != 0 {
		t.Errorf("前端不应看到错误: %v", res.Errors)
	}
	if !strings.Contains(res.Contents, "重试成功") {
		t.Errorf("重试内容缺失, got: %q", res.Contents)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 5：服务端忽略 stream=true，返回完整非流式 JSON
// 部分中转/网关对推理模型不支持流式，直接返回完整 chat.completion。
// 期望：解析器兜底提取 message.content 输出，而不是静默无输出直到看门狗超时。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_NonStreamResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-5",
			"object": "chat.completion",
			"model": "gpt-6.1-sol",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "非流式兜底输出"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if !strings.Contains(res.Contents, "非流式兜底输出") {
		t.Errorf("非流式响应内容未被提取（前端将持续无输出直到看门狗超时）, got: %q, errors: %v", res.Contents, res.Errors)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 6：流式 tool_calls 工具调用循环
// OpenAI 流式工具调用分片下发（arguments 分多帧），finish_reason=tool_calls。
// 期望：工具被执行，二轮请求返回最终内容。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_ToolCallsLoop(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			writeSSE(w,
				sseData(chatChunk("chatcmpl-6", map[string]any{
					"role": "assistant",
					"tool_calls": []any{map[string]any{
						"index": 0,
						"id":    "call_compat_1",
						"type":  "function",
						"function": map[string]any{
							"name":      "CompatMockTool",
							"arguments": "{\"query\":",
						},
					}},
				}, "")),
				sseData(chatChunk("chatcmpl-6", map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0,
						"function": map[string]any{
							"arguments": "\"测试\"}",
						},
					}},
				}, "")),
				sseData(chatChunk("chatcmpl-6", map[string]any{}, "tool_calls")),
				"data: [DONE]\n\n",
			)
			return
		}
		writeSSE(w,
			sseData(chatChunk("chatcmpl-6b", map[string]any{"content": "工具调用后的最终回答"}, "")),
			sseData(chatChunk("chatcmpl-6b", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	registerToolHandler("CompatMockTool", func(o *OpenAi, args string, ctx *ToolContext) error {
		if args != `{"query":"测试"}` {
			t.Errorf("工具参数分片拼接错误: %q", args)
		}
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, args, "mock 工具结果")
		return nil
	})
	defer delete(toolHandlers, "CompatMockTool")

	o := newCompatOpenAi(srv.URL)
	tools := []Tool{{
		Type: "function",
		Function: ToolFunction{
			Name:        "CompatMockTool",
			Description: "测试工具",
			Parameters:  &FunctionParameters{Type: "object", Properties: map[string]any{}},
		},
	}}
	ch := make(chan map[string]any, 512)
	runAskAiWithTools(o, ch, "测试", tools, false)
	res := collectCompatStream(ch)

	if calls.Load() < 2 {
		t.Errorf("工具调用后未发起二轮请求, calls=%d", calls.Load())
	}
	if !strings.Contains(res.Contents, "工具调用后的最终回答") {
		t.Errorf("工具循环最终内容缺失, got: %q, errors: %v", res.Contents, res.Errors)
	}
	if len(res.Errors) != 0 {
		t.Errorf("工具循环不应有错误: %v", res.Errors)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 7：SSE 注释行（: keep-alive）与 event: 行穿插
// 推理模型思考期间网关常发送 SSE 注释保活。期望：被忽略，正常解析。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_KeepAliveComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		frames := []string{
			": keep-alive\n\n",
			"event: ping\n\n",
			sseData(chatChunk("chatcmpl-7", map[string]any{"content": "保活后正常"}, "")),
			": keep-alive\n\n",
			sseData(chatChunk("chatcmpl-7", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		}
		for _, f := range frames {
			_, _ = w.Write([]byte(f))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if res.Contents != "保活后正常" {
		t.Errorf("保活注释行干扰了解析, got: %q, errors: %v", res.Contents, res.Errors)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 8：带工具模式下 reasoning 字段 + thinking 降级组合
// AI 助手默认开启工具 + 思考模式，是用户实际使用路径。
// 期望：thinking 被拒后自动降级重试，推理经 reasoning 字段流式展示，工具循环正常。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_WithToolsThinkingCombo(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["thinking"]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unrecognized request argument supplied: thinking","type":"invalid_request_error","param":"thinking","code":"unknown_parameter"}}`))
			return
		}
		n := calls.Add(1)
		if n == 1 {
			writeSSE(w,
				sseData(chatChunk("chatcmpl-8", map[string]any{"reasoning": "需要先调用工具查询"}, "")),
				sseData(chatChunk("chatcmpl-8", map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0,
						"id":    "call_compat_2",
						"type":  "function",
						"function": map[string]any{
							"name":      "CompatMockTool2",
							"arguments": "{}",
						},
					}},
				}, "tool_calls")),
				"data: [DONE]\n\n",
			)
			return
		}
		writeSSE(w,
			sseData(chatChunk("chatcmpl-8b", map[string]any{"content": "组合场景最终回答"}, "")),
			sseData(chatChunk("chatcmpl-8b", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	registerToolHandler("CompatMockTool2", func(o *OpenAi, args string, ctx *ToolContext) error {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, args, "组合场景工具结果")
		return nil
	})
	defer delete(toolHandlers, "CompatMockTool2")

	o := newCompatOpenAi(srv.URL)
	tools := []Tool{{
		Type: "function",
		Function: ToolFunction{
			Name:        "CompatMockTool2",
			Description: "测试工具2",
			Parameters:  &FunctionParameters{Type: "object", Properties: map[string]any{}},
		},
	}}
	ch := make(chan map[string]any, 512)
	runAskAiWithTools(o, ch, "测试", tools, true)
	res := collectCompatStream(ch)

	if calls.Load() < 2 {
		t.Errorf("thinking 降级+工具循环未完成, calls=%d", calls.Load())
	}
	if !strings.Contains(res.Contents, "组合场景最终回答") {
		t.Errorf("组合场景最终内容缺失, got: %q, errors: %v", res.Contents, res.Errors)
	}
	if len(res.Errors) != 0 {
		t.Errorf("组合场景不应有错误: %v", res.Errors)
	}
}

var _ = fmt.Sprintf // keep fmt import if unused later

// ------------------------------------------------------------------------------------------------
// 场景 9：流内嵌错误帧自动重试（真实根因复现）
// 实测 aixlau.me 中转在 gpt-6.1-sol 上游过载时返回 HTTP 200 的 SSE 流，
// 下发无 choices 字段的错误帧：data: {"error":{"message":"Our servers are currently overloaded...","type":"upstream_error"}}
// 旧解析器对该帧静默跳过 → 前端零输出直到看门狗超时（用户配置 timeout=6000s，约 102 分钟）。
// 期望：未输出过内容时自动重试一次并成功，前端无感知。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_InlineErrorFrameRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			writeSSE(w,
				`data: {"error":{"message":"Our servers are currently overloaded. Please try again later.","type":"upstream_error"}}`+"\n\n",
				"data: [DONE]\n\n",
			)
			return
		}
		writeSSE(w,
			sseData(chatChunk("chatcmpl-9", map[string]any{"content": "重试后恢复正常"}, "")),
			sseData(chatChunk("chatcmpl-9", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if calls.Load() != 2 {
		t.Errorf("流内错误帧未触发自动重试, calls=%d", calls.Load())
	}
	if len(res.Errors) != 0 {
		t.Errorf("自动重试成功后前端不应看到错误: %v", res.Errors)
	}
	if !strings.Contains(res.Contents, "重试后恢复正常") {
		t.Errorf("重试内容缺失, got: %q", res.Contents)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 10：工具模式流内错误帧自动重试（AI 助手实际使用路径）
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_WithToolsInlineErrorRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			writeSSE(w,
				`data: {"error":{"message":"Upstream error, please retry.","type":"upstream_error"}}`+"\n\n",
				"data: [DONE]\n\n",
			)
			return
		}
		writeSSE(w,
			sseData(chatChunk("chatcmpl-10", map[string]any{"content": "工具模式重试成功"}, "")),
			sseData(chatChunk("chatcmpl-10", map[string]any{}, "stop")),
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	o := newCompatOpenAi(srv.URL)
	tools := []Tool{{
		Type: "function",
		Function: ToolFunction{
			Name:        "CompatUnreachedTool",
			Description: "不应被调用的工具",
			Parameters:  &FunctionParameters{Type: "object", Properties: map[string]any{}},
		},
	}}
	ch := make(chan map[string]any, 512)
	runAskAiWithTools(o, ch, "测试", tools, false)
	res := collectCompatStream(ch)

	if calls.Load() != 2 {
		t.Errorf("工具模式流内错误帧未触发自动重试, calls=%d", calls.Load())
	}
	if len(res.Errors) != 0 {
		t.Errorf("工具模式重试成功后不应有错误: %v", res.Errors)
	}
	if !strings.Contains(res.Contents, "工具模式重试成功") {
		t.Errorf("工具模式重试内容缺失, got: %q", res.Contents)
	}
}

// ------------------------------------------------------------------------------------------------
// 场景 11：已有内容输出后流中断为错误帧
// 已输出内容时不可自动重试（会造成前端内容重复），但必须推送可见错误，
// 而不是让前端静默等待到看门狗超时。
// ------------------------------------------------------------------------------------------------
func TestGPT61Sol_InlineErrorAfterContent(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeSSE(w,
			sseData(chatChunk("chatcmpl-11", map[string]any{"content": "前半段内容"}, "")),
			`data: {"error":{"message":"Connection reset by upstream.","type":"upstream_error"}}`+"\n\n",
			"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	ch := make(chan map[string]any, 512)
	runAskAi(newCompatOpenAi(srv.URL), ch, "测试", false)
	res := collectCompatStream(ch)

	if calls.Load() != 1 {
		t.Errorf("已有输出后不应重试（会导致内容重复）, calls=%d", calls.Load())
	}
	if !strings.Contains(res.Contents, "前半段内容") {
		t.Errorf("已输出内容不应丢失, got: %q", res.Contents)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "Connection reset") {
		t.Errorf("已有输出后遇错误帧必须推送可见错误, got: %v", res.Errors)
	}
}
