package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// isContextCanceledError 用于在 context 失效时跳过 React 降级重试，
// 必须能识别 eino 包装后的 GraphRunError/NodeRunError 文案。
func TestIsContextCanceledError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.DeadlineExceeded, true},
		{context.Canceled, true},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), true},
		{fmt.Errorf("[GraphRunError] context has been canceled: context deadline exceeded"), true},
		{fmt.Errorf("NodeRunError, gen message failed: context canceled"), true},
		{fmt.Errorf("max_tokens above maximum value"), false},
		{fmt.Errorf("unmarshal plan error: invalid char"), false},
	}
	for i, c := range cases {
		if got := isContextCanceledError(c.err); got != c.want {
			t.Errorf("case %d: isContextCanceledError(%v) = %v, want %v", i, c.err, got, c.want)
		}
	}
}

// TestProcessAdkMessageStreamAggregatesToolChunks 验证流式工具（如 execute）的
// 多个结果分片聚合后只发送一条 "✅ xxx 返回结果（总字数）" 步骤消息，
// 避免前端刷出大量流式执行中间结果。
func TestProcessAdkMessageStreamAggregatesToolChunks(t *testing.T) {
	ch := make(chan *schema.Message, 16)
	var full strings.Builder

	chunks := []string{"第一行输出\n", "ab\n", "第三行输出\n"}
	msgs := make([]*schema.Message, 0, len(chunks))
	for _, c := range chunks {
		msgs = append(msgs, schema.ToolMessage(c, "call-1", schema.WithToolName("execute")))
	}
	sr := schema.StreamReaderFromArray(msgs)

	processAdkMessageStream(context.Background(), sr, schema.Tool, "execute", ch, &full)
	close(ch)

	var stepMsgs []string
	for m := range ch {
		if m.ReasoningContent != "" {
			stepMsgs = append(stepMsgs, m.ReasoningContent)
		}
	}

	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if len(stepMsgs) != 1 {
		t.Fatalf("期望聚合后只有 1 条步骤消息，实际 %d 条: %v", len(stepMsgs), stepMsgs)
	}
	want := fmt.Sprintf("[STEP]✅ execute 返回结果（%d字）\n", total)
	if stepMsgs[0] != want {
		t.Errorf("步骤消息 = %q, want %q", stepMsgs[0], want)
	}
	if full.Len() != 0 {
		t.Errorf("工具结果不应写入 fullResponse，实际 len=%d", full.Len())
	}
}
