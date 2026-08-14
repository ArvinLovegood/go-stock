package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeSummaryModel 最小化实现 model.ToolCallingChatModel，用于验证摘要超时逻辑。
// delay 控制 Generate 的响应耗时，resp/err 控制返回内容，用于构造各种超时场景。
type fakeSummaryModel struct {
	delay time.Duration
	resp  string
	err   error
}

func (f *fakeSummaryModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	return &schema.Message{Role: schema.Assistant, Content: f.resp}, nil
}

func (f *fakeSummaryModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeSummaryModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}

// 固定一个超过 4000 token 的长正文，确保触发 LLM 摘要分支。
var longBody = "指标：价格 12.34 元，涨幅 5.67%，成交额 8901 万。" + repeatRune("数据说明", 3000)

func repeatRune(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

// 父 ctx 已过期：旧实现 WithTimeout(ctx, 5s) 会立即失效导致降级，
// 新实现基于 Background 派生，仍应成功生成摘要。
func TestLLMSummarizeIgnoresExpiredParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel() // 父 ctx 已取消

	m := &fakeSummaryModel{delay: 200 * time.Millisecond, resp: "这是摘要：价格 12.34 元，涨幅 5.67%"}
	ctx := WithSummaryModel(parent, m)

	start := time.Now()
	out := llmSummarizeToolResult(ctx, longBody)
	elapsed := time.Since(start)

	if out == "" {
		t.Fatalf("父 ctx 已过期时摘要仍应成功，但返回了空（fallback），耗时 %v", elapsed)
	}
	if !strings.Contains(out, "12.34") {
		t.Fatalf("摘要应保留关键数字，实际: %q", out)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("快速模型不应等待太久，耗时 %v", elapsed)
	}
}

// 慢模型（8s）：旧 5s 超时会失败，新 15s 超时应成功。
func TestLLMSummarizeSlowModelWithinTimeout(t *testing.T) {
	m := &fakeSummaryModel{delay: 8 * time.Second, resp: "摘要内容（慢模型）"}
	ctx := WithSummaryModel(context.Background(), m)

	start := time.Now()
	out := llmSummarizeToolResult(ctx, longBody)
	elapsed := time.Since(start)

	if out == "" {
		t.Fatalf("8s 慢模型应在新 15s 超时内成功，但返回空，耗时 %v", elapsed)
	}
	if elapsed > 14*time.Second {
		t.Fatalf("成功耗时不应接近/超过 15s 上限，实际 %v", elapsed)
	}
}

// 超过新超时上限（30s > 15s）：应超时返回空并降级，且整体耗时受 15s 约束。
func TestLLMSummarizeTimeoutBeyondBound(t *testing.T) {
	m := &fakeSummaryModel{delay: 30 * time.Second, resp: "不会用到"}
	ctx := WithSummaryModel(context.Background(), m)

	start := time.Now()
	out := llmSummarizeToolResult(ctx, longBody)
	elapsed := time.Since(start)

	if out != "" {
		t.Fatalf("超过 15s 超时应返回空降级，实际非空")
	}
	if elapsed > 20*time.Second {
		t.Fatalf("超时上限应受 15s 约束，实际耗时 %v", elapsed)
	}
	if elapsed < 14*time.Second {
		t.Fatalf("30s 模型应在 ~15s 处超时返回，但过早返回（%v）", elapsed)
	}
}
