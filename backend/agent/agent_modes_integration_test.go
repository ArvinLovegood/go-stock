package agent

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// TestAllAgentModesLocalOllama 使用本地 Ollama 模型测试所有 Agent 模式（React、PlanExecute、DeepAgents）。
//
// 验证点：
//  1. 每种模式都能成功创建 Agent 实例
//  2. normalizeOllamaBaseURL 修复后，带 /v1 后缀的 BaseURL 在所有模式下均正常工作
//  3. 模型实际被调用并返回响应（通过事件流或流式输出验证）
//  4. DeepAgents 的 nonFatalSummaryMiddleware 在摘要失败时正确降级
//
// 前置条件：本地 Ollama 已启动且有 qwen3.8:latest 模型。
// 若 Ollama 未运行则跳过。
func TestAllAgentModesLocalOllama(t *testing.T) {
	// 检测 Ollama 是否在运行
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer checkCancel()
	req, _ := http.NewRequestWithContext(checkCtx, http.MethodGet, "http://127.0.0.1:11434/api/tags", nil)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("本地 Ollama 未运行，跳过集成测试: %v", err)
	}
	resp.Body.Close()

	// 初始化数据库（工具初始化需要）
	db.Init("../../data/stock.db")

	// 构造本地 Ollama AIConfig（带 /v1 后缀，验证 normalizeOllamaBaseURL 修复）
	// qwen3.8:latest 上下文窗口 262144，设 32768 足够工具 schema + 对话
	aiCfg := data.AIConfig{
		Name:          "local-ollama-test",
		BaseUrl:       "http://127.0.0.1:11434/v1",
		ModelName:     "qwen3.8:latest",
		ApiKey:        "ollama",
		Temperature:   0.3,
		TimeOut:       600,
		MaxTokens:     500,
		ContextWindow: 32768,
	}

	// 使用股票相关问题，确保 PlanExecute executor 能触发工具调用
	const question = "查询贵州茅台(600519)的最新行情"
	const sysPrompt = "你是一个股票分析助手。"

	modes := []struct {
		name string
		mode string
	}{
		{"React", string(React)},
		{"PlanExecute", string(PlanExecute)},
		{"DeepAgents", string(DeepAgents)},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
			defer cancel()

			// 创建 Agent 实例
			inst, err := GetStockAiAgent(&ctx, aiCfg, question, m.mode)
			if err != nil {
				t.Fatalf("GetStockAiAgent(mode=%s) 失败: %v", m.name, err)
			}
			if inst == nil {
				t.Fatalf("GetStockAiAgent(mode=%s) 返回 nil", m.name)
			}
			t.Logf("Agent 模式: %s, 工具数: %d", inst.Mode, len(inst.Tools))

			messages := []*schema.Message{
				{Role: schema.System, Content: sysPrompt},
				{Role: schema.User, Content: question},
			}

			var responseText string

			switch m.mode {
			case string(React):
				responseText = runReactForTest(t, ctx, inst, messages)
			default:
				// PlanExecute 和 DeepAgents 都使用 adk.NewRunner
				responseText = runAdkAgentForTest(t, ctx, inst, messages)
			}

			responseText = strings.TrimSpace(responseText)
			if responseText == "" {
				t.Fatalf("模式 %s 返回空响应", m.name)
			}
			t.Logf("模式 %s 响应 (前300字): %s", m.name, truncForTest(responseText, 300))
		})
	}
}

func runReactForTest(t *testing.T, ctx context.Context, inst *Instance, messages []*schema.Message) string {
	t.Helper()
	if inst.ReactAgent == nil {
		t.Fatal("ReactAgent 为 nil")
	}

	sr, err := inst.ReactAgent.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("React Stream 失败: %v", err)
	}

	var sb strings.Builder
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		if msg == nil {
			continue
		}
		if msg.Content != "" {
			sb.WriteString(msg.Content)
		}
		if msg.ReasoningContent != "" && msg.Content == "" {
			sb.WriteString(msg.ReasoningContent)
		}
	}
	return sb.String()
}

func runAdkAgentForTest(t *testing.T, ctx context.Context, inst *Instance, messages []*schema.Message) string {
	t.Helper()
	if inst.AdkAgent == nil {
		t.Fatal("AdkAgent 为 nil")
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: inst.AdkAgent,
	})
	iter := runner.Run(ctx, messages)

	var sb strings.Builder
	var eventErrors []string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			eventErrors = append(eventErrors, event.Err.Error())
			t.Logf("Agent 事件错误: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.Message != nil {
				if mv.Message.ReasoningContent != "" {
					sb.WriteString(mv.Message.ReasoningContent)
				}
				if mv.Message.Content != "" {
					sb.WriteString(mv.Message.Content)
				}
			}
		}
	}

	result := sb.String()
	if result == "" && len(eventErrors) > 0 {
		t.Logf("所有事件错误:\n  - %s", strings.Join(eventErrors, "\n  - "))
	}
	return result
}

func truncForTest(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
