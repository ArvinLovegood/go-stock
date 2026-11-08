package agent

// reactTurnRouter 单元测试：验证快速模式（React）流式输出的思考/回答分离路由。
// 场景覆盖：工具轮评论→思考区、终轮回答→回答区、<think> 内嵌思考剥离（含跨分片）、
// 长回答乐观直通（流式体验）、无 finish_reason 的 EOF 兜底、超长评论的记录在案泄漏。
//
// 运行方式：go test ./backend/agent -run 'TestReactTurnRouter' -v

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// routerHarness 构造一个收集输出的 router。feed 模拟 runReact 主循环逐条喂入消息
// （不含 flushEOF，由测试按需显式调用）；result 汇总发送结果：
// thinking=思考区累计文本，content=回答区累计文本，fullResp=fullResponse 累积。
func routerHarness() (r *reactTurnRouter, feed func(msgs ...*schema.Message), result func() (thinking, content, fullResp string)) {
	var full strings.Builder
	var out []*schema.Message
	r = newReactTurnRouter(func(m *schema.Message) { out = append(out, m) }, &full)

	feed = func(msgs ...*schema.Message) {
		for _, m := range msgs {
			if m.Content != "" && m.Role == schema.Assistant {
				r.handleContentChunk(m.Content)
			}
			if len(m.ToolCalls) > 0 {
				r.onToolCalls()
			}
			if m.Role == schema.Tool {
				r.onToolResult()
			}
			if m.ResponseMeta != nil && m.ResponseMeta.FinishReason != "" {
				r.onFinishReason(m.ResponseMeta.FinishReason)
			}
		}
	}
	result = func() (string, string, string) {
		var tb, cb strings.Builder
		for _, m := range out {
			if m.ReasoningContent != "" {
				tb.WriteString(m.ReasoningContent)
			}
			if m.Content != "" {
				cb.WriteString(m.Content)
			}
		}
		return tb.String(), cb.String(), full.String()
	}
	return r, feed, result
}

func assistantContent(text string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, Content: text}
}

func toolCallMsg(name string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:       "call-1",
			Function: schema.FunctionCall{Name: name, Arguments: "{}"},
		}},
	}
}

func toolResultMsg(content string) *schema.Message {
	return &schema.Message{Role: schema.Tool, ToolName: "t", Content: content}
}

func finishMsg(reason string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: reason}}
}

// 工具轮评论进思考区，终轮回答进回答区，fullResponse 只累积正式回答。
func TestReactTurnRouter_ToolTurnCommentary(t *testing.T) {
	r, feed, result := routerHarness()
	feed(
		assistantContent("我先行：查询板块成分股"),
		toolCallMsg("GetBKConstituentStocks"),
		toolResultMsg("...工具返回..."),
		assistantContent("最终回答：建议关注。"),
		finishMsg("stop"),
	)
	r.flushEOF()
	thinking, content, fullResp := result()
	if thinking != "我先行：查询板块成分股" {
		t.Errorf("thinking = %q, want 工具轮评论", thinking)
	}
	if content != "最终回答：建议关注。" {
		t.Errorf("content = %q, want 终轮回答", content)
	}
	if fullResp != content {
		t.Errorf("fullResponse = %q, want 与回答区一致", fullResp)
	}
	if r.sentAnswer != 1 {
		t.Errorf("sentAnswer = %d, want 1", r.sentAnswer)
	}
}

// 多轮工具调用：每轮评论都进思考区，只有终轮进回答区。
func TestReactTurnRouter_MultiToolTurns(t *testing.T) {
	r, feed, result := routerHarness()
	feed(
		assistantContent("第一轮评论"),
		toolCallMsg("ToolA"),
		toolResultMsg("A 结果"),
		assistantContent("第二轮评论"),
		toolCallMsg("ToolB"),
		toolResultMsg("B 结果"),
		assistantContent("正式结论"),
		finishMsg("stop"),
	)
	r.flushEOF()
	thinking, content, _ := result()
	if want := "第一轮评论第二轮评论"; thinking != want {
		t.Errorf("thinking = %q, want %q", thinking, want)
	}
	if content != "正式结论" {
		t.Errorf("content = %q, want 正式结论", content)
	}
}

// Content 与 ToolCalls 同分片到达：评论应被定性为工具轮并进思考区。
func TestReactTurnRouter_ContentWithToolCallsSameChunk(t *testing.T) {
	r, feed, result := routerHarness()
	m := toolCallMsg("SelectStock")
	m.Content = "我先筛选"
	feed(m, toolResultMsg("res"), assistantContent("结论"), finishMsg("stop"))
	r.flushEOF()
	thinking, content, _ := result()
	if thinking != "我先筛选" {
		t.Errorf("thinking = %q, want 我先筛选", thinking)
	}
	if content != "结论" {
		t.Errorf("content = %q, want 结论", content)
	}
}

// <think> 内嵌思考剥离：思考块跨分片时仍能正确配对。
func TestReactTurnRouter_ThinkTagCrossChunk(t *testing.T) {
	r, feed, result := routerHarness()
	feed(
		assistantContent("<think>先想想"),
		assistantContent("跨分片思考</think>"),
		assistantContent("正式答案"),
		finishMsg("stop"),
	)
	r.flushEOF()
	thinking, content, _ := result()
	if thinking != "先想想跨分片思考" {
		t.Errorf("thinking = %q, want 剥离出的思考内文", thinking)
	}
	if content != "正式答案" {
		t.Errorf("content = %q, want 正式答案", content)
	}
}

// <think>…</think> 与答案同分片。
func TestReactTurnRouter_ThinkTagSingleChunk(t *testing.T) {
	r, feed, result := routerHarness()
	feed(
		assistantContent("<think>\n推理过程\n</think>\n\n## 结论\n买入"),
		finishMsg("stop"),
	)
	r.flushEOF()
	thinking, content, _ := result()
	if thinking != "\n推理过程\n" {
		t.Errorf("thinking = %q", thinking)
	}
	if content != "\n\n## 结论\n买入" {
		t.Errorf("content = %q", content)
	}
}

// <think> 未闭合到流结束：按思考处理，不进回答区。
func TestReactTurnRouter_ThinkTagUnclosedAtEOF(t *testing.T) {
	r, feed, result := routerHarness()
	feed(assistantContent("<think>一直想到结束"))
	r.flushEOF()
	thinking, content, _ := result()
	if thinking != "一直想到结束" {
		t.Errorf("thinking = %q", thinking)
	}
	if content != "" {
		t.Errorf("content = %q, want 空", content)
	}
}

// 长回答超过阈值即刷出并直通：不足阈值时不发送（等定性），超阈值后整体刷出。
func TestReactTurnRouter_OptimisticFlush(t *testing.T) {
	r, feed, result := routerHarness()
	feed(assistantContent(strings.Repeat("涨", 100)))
	if r.sentAnswer != 0 {
		t.Fatalf("不足阈值的分片应继续缓冲, sentAnswer=%d", r.sentAnswer)
	}
	feed(assistantContent(strings.Repeat("牛", 250)), finishMsg("stop"), assistantContent("续"))
	r.flushEOF()
	thinking, content, fullResp := result()
	if thinking != "" {
		t.Errorf("thinking = %q, want 空", thinking)
	}
	want := strings.Repeat("涨", 100) + strings.Repeat("牛", 250) + "续"
	if content != want {
		t.Errorf("content 长度 = %d, want %d", len([]rune(content)), len([]rune(want)))
	}
	if fullResp != content {
		t.Errorf("fullResponse 与回答区不一致")
	}
}

// 短回答：缓冲到 finish_reason=stop 才刷出。
func TestReactTurnRouter_ShortAnswerFlushedAtStop(t *testing.T) {
	r, feed, result := routerHarness()
	feed(assistantContent("好"))
	if r.sentAnswer != 0 {
		t.Fatalf("stop 前不应发送, sentAnswer=%d", r.sentAnswer)
	}
	feed(finishMsg("stop"))
	r.flushEOF()
	_, content, _ := result()
	if content != "好" {
		t.Errorf("content = %q, want 好", content)
	}
}

// 部分中转不回传 finish_reason：EOF 时残留缓冲按终轮回答兜底刷出。
func TestReactTurnRouter_EOFWithoutFinishReason(t *testing.T) {
	r, feed, result := routerHarness()
	feed(assistantContent("无 finish_reason 的终答"))
	// 不发 finishMsg，直接 EOF
	r.flushEOF()
	thinking, content, _ := result()
	_ = thinking
	if content != "无 finish_reason 的终答" {
		t.Errorf("content = %q, want EOF 兜底刷出终答", content)
	}
}

// 超长工具轮评论：前缀超过阈值的部分先进入回答区（记录在案的乐观直通泄漏），
// 工具调用信号之后到达的评论仍进思考区。
func TestReactTurnRouter_LongCommentaryLeak(t *testing.T) {
	r, feed, result := routerHarness()
	commentary := strings.Repeat("析", 320)
	feed(assistantContent(commentary)) // ≥阈值：乐观刷为回答
	feed(toolCallMsg("ToolA"))
	feed(assistantContent("工具后补充评论")) // 工具调用信号之后的评论 → 思考区
	feed(toolResultMsg("res"))
	r.flushEOF()
	thinking, content, _ := result()
	if content != commentary {
		t.Errorf("content = %q len=%d, want 泄漏的前缀 len=%d", truncateString(content, 30), len([]rune(content)), 320)
	}
	if thinking != "工具后补充评论" {
		t.Errorf("thinking = %q, want 工具后补充评论", thinking)
	}
}

// 工具结果之后截断（无 finish_reason、无终轮内容）：残留空缓冲，不产生回答。
func TestReactTurnRouter_TruncatedAfterToolResult(t *testing.T) {
	r, feed, result := routerHarness()
	feed(assistantContent("评论"), toolCallMsg("ToolA"), toolResultMsg("res"))
	r.flushEOF()
	thinking, content, fullResp := result()
	if thinking != "评论" {
		t.Errorf("thinking = %q, want 评论", thinking)
	}
	if content != "" || fullResp != "" {
		t.Errorf("content=%q fullResp=%q, want 均为空", content, fullResp)
	}
}
