package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// TestDeepAgentVisionEndToEnd 完全复刻生产路径（createChatModel → createDeepAgent →
// adk.NewRunner.Run），验证带图用户消息经 DeepAgents 全链路后发出的 HTTP 请求体
// 仍包含标准 image_url 内容块。回归：DeepSeek vision 请求曾丢失图片（模型自称
// 文本 agent 去 fetch 图片 URL）。
func TestDeepAgentVisionEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	// 临时数据库（buildChatModelHTTPClient 读全局设置）。
	// 不用 t.TempDir()：db 句柄在测试进程内保持打开，Windows 下 TempDir 清理会因文件锁失败。
	db.Init(filepath.Join(os.TempDir(), fmt.Sprintf("go-stock-vision-e2e-%d.db", time.Now().UnixNano())))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		// 非流式简单回复，终止 agent 循环（无工具调用）
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test",
			"choices":[{"index":0,"message":{"role":"assistant","content":"这是一张测试图"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	cfg := data.AIConfig{
		BaseUrl:       server.URL,
		ModelName:     "test-vision",
		ApiKey:        "test-key",
		SupportVision: true,
		TimeOut:       30,
	}
	cm, err := createChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createChatModel: %v", err)
	}

	inst, err := createDeepAgent(context.Background(), cm, nil, cfg)
	if err != nil {
		t.Fatalf("createDeepAgent: %v", err)
	}

	// 复刻 ChatWithContext 修复后的消息构造（Content 与 MultiContent 互斥）
	u := "https://img.cdn1.vip/i/test.webp"
	parts := []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "图片里面是啥"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{URL: &u},
		}},
	}
	messages := []*schema.Message{
		{Role: schema.System, Content: "sys"},
		{Role: schema.User, UserInputMultiContent: parts},
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: inst.AdkAgent})
	iter := runner.Run(context.Background(), messages)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		event, ok := iter.Next()
		if !ok || event == nil {
			break
		}
		if event.Err != nil {
			t.Logf("agent event error: %v", event.Err)
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("no request captured")
	}
	// 检查所有发出的请求中至少有一个包含 image_url 内容块
	for _, b := range bodies {
		if strings.Contains(b, `"image_url"`) && strings.Contains(b, u) {
			// 进一步校验结构合法
			var req struct {
				Messages []struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal([]byte(b), &req); err == nil {
				t.Logf("OK: request contains image_url block (messages=%d)", len(req.Messages))
				return
			}
		}
	}
	t.Fatalf("no request contains image_url content block; %d requests captured, first body: %.2000s", len(bodies), bodies[0])
}
