package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"

	"github.com/cloudwego/eino/schema"
)

// TestOllamaIntegrationLocal 在本地 Ollama (127.0.0.1:11434) 上进行端到端集成测试。
// 验证 normalizeOllamaBaseURL 修复后，带 /v1 后缀的 BaseURL 也能正确调用 Ollama。
//
// 前置条件：本地 Ollama 已启动且有 qwen3.8:latest 模型。
// 若 Ollama 未运行则跳过。
func TestOllamaIntegrationLocal(t *testing.T) {
	// 快速检测 Ollama 是否在运行
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer checkCancel()
	req, _ := http.NewRequestWithContext(checkCtx, http.MethodGet, "http://127.0.0.1:11434/api/tags", nil)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("本地 Ollama 未运行，跳过集成测试: %v", err)
	}
	resp.Body.Close()

	// 用 createChatModel 创建模型（模拟用户配置了带 /v1 的 BaseURL）
	testCases := []struct {
		name    string
		baseURL string
	}{
		{"correct_url", "http://127.0.0.1:11434"},
		{"with_v1_suffix", "http://127.0.0.1:11434/v1"},
		{"with_v1_chat_completions", "http://127.0.0.1:11434/v1/chat/completions"},
	}

	aiCfg := data.AIConfig{
		BaseUrl:       "", // 逐个覆盖
		ModelName:     "qwen3.8:latest",
		ApiKey:        "",
		Temperature:   0.1,
		TimeOut:       300,
		MaxTokens:     100,
		ContextWindow: 4096,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aiCfg.BaseUrl = tc.baseURL
			ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
			defer cancel2()

			chatModel, err := createChatModel(ctx2, aiCfg)
			if err != nil {
				t.Fatalf("createChatModel failed: %v", err)
			}

			msgs := []*schema.Message{
				schema.UserMessage("只回复两个字：你好"),
			}

			resp, err := chatModel.Generate(ctx2, msgs)
			if err != nil {
				t.Fatalf("Generate failed (baseURL=%s): %v", tc.baseURL, err)
			}

			content := resp.Content
			if content == "" && resp.ReasoningContent != "" {
				content = resp.ReasoningContent
			}
			content = strings.TrimSpace(content)
			if content == "" {
				t.Fatalf("Generate returned empty content (baseURL=%s)", tc.baseURL)
			}
			t.Logf("baseURL=%s -> response: %q", tc.baseURL, content)
		})
	}
}

// TestOllamaNumCtxRuntime 验证 createChatModel 对 Ollama 显式传递 num_ctx。
//
// 背景：Ollama 默认 num_ctx 仅 4096，Agent 工具 schema + 系统提示轻松超过 8k
// token，不传 num_ctx 会被静默截断头部（系统提示/工具定义丢失）。
//
// 用 ContextWindow=0 + MaxTokens=100（旧配置兜底路径）构造最不利场景：
// resolveContextWindow 会把 MaxTokens=100 误当上下文窗口，期望 num_ctx 命中
// minOllamaNumCtx=8192 下限。通过 Ollama /api/ps 的 context_length 字段验证
// 服务端实际加载的上下文长度（需 Ollama >= 0.5，旧版本无该字段则跳过断言）。
func TestOllamaNumCtxRuntime(t *testing.T) {
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer checkCancel()
	req, _ := http.NewRequestWithContext(checkCtx, http.MethodGet, "http://127.0.0.1:11434/api/tags", nil)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("本地 Ollama 未运行，跳过集成测试: %v", err)
	}
	resp.Body.Close()

	// ContextWindow=0（未配置）+ MaxTokens=100（旧配置）→ 触发解析兜底路径
	aiCfg := data.AIConfig{
		Name:      "local-ollama-numctx-test",
		BaseUrl:   "http://127.0.0.1:11434",
		ModelName: "qwen3.8:latest",
		ApiKey:    "ollama",
		TimeOut:   300,
		MaxTokens: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	chatModel, err := createChatModel(ctx, aiCfg)
	if err != nil {
		t.Fatalf("createChatModel failed: %v", err)
	}

	if _, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 查询 /api/ps 验证服务端实际生效的上下文长度
	psCtx, psCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer psCancel()
	psReq, _ := http.NewRequestWithContext(psCtx, http.MethodGet, "http://127.0.0.1:11434/api/ps", nil)
	psResp, err := client.Do(psReq)
	if err != nil {
		t.Skipf("查询 /api/ps 失败，跳过 context_length 断言: %v", err)
	}
	defer psResp.Body.Close()

	var ps struct {
		Models []struct {
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
		} `json:"models"`
	}
	if err := json.NewDecoder(psResp.Body).Decode(&ps); err != nil {
		t.Fatalf("解析 /api/ps 响应失败: %v", err)
	}

	for _, m := range ps.Models {
		if m.Name != aiCfg.ModelName {
			continue
		}
		if m.ContextLength == 0 {
			t.Skip("Ollama 版本较旧，/api/ps 不含 context_length 字段，跳过断言")
		}
		t.Logf("Ollama 实际加载 context_length=%d (期望 %d=下限)", m.ContextLength, minOllamaNumCtx)
		if m.ContextLength < minOllamaNumCtx {
			t.Fatalf("num_ctx 未生效或被误判为小值: context_length=%d, 期望 >= %d", m.ContextLength, minOllamaNumCtx)
		}
		return
	}
	t.Logf("未在 /api/ps 中找到模型 %s（可能已被卸载），跳过 context_length 断言", aiCfg.ModelName)
}
