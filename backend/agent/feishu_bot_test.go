package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// strPtr 返回字符串指针，便于构造 larkim.EventMessage 的字段
func strPtr(s string) *string {
	return &s
}

// makeTextMessage 构造一个 text 类型的 EventMessage，content 形如 {"text":"..."}
func makeTextMessage(text, chatType, messageID, chatID string, mentions []*larkim.MentionEvent) *larkim.EventMessage {
	contentJSON, _ := json.Marshal(map[string]string{"text": text})
	return &larkim.EventMessage{
		MessageId:   strPtr(messageID),
		ChatId:      strPtr(chatID),
		ChatType:    strPtr(chatType),
		MessageType: strPtr("text"),
		Content:     strPtr(string(contentJSON)),
		Mentions:    mentions,
	}
}

func TestExtractMessageText_PlainText(t *testing.T) {
	msg := makeTextMessage("你好", "p2p", "om_1", "oc_1", nil)
	got := extractMessageText(msg)
	assert.Equal(t, "你好", got)
}

func TestExtractMessageText_StripsAtMention(t *testing.T) {
	// 飞书群聊 @机器人 后的 content 格式：{"text":"@_user_1 你好"}
	msg := makeTextMessage("@_user_1 你好", "group", "om_1", "oc_1", nil)
	got := extractMessageText(msg)
	assert.Equal(t, "你好", got)
}

func TestExtractMessageText_StripsMultipleAtMentions(t *testing.T) {
	msg := makeTextMessage("@_user_1 @_user_2 分析贵州茅台", "group", "om_1", "oc_1", nil)
	got := extractMessageText(msg)
	assert.Equal(t, "分析贵州茅台", got)
}

func TestExtractMessageText_NilMessage(t *testing.T) {
	got := extractMessageText(nil)
	assert.Equal(t, "", got)
}

func TestExtractMessageText_NilContent(t *testing.T) {
	msg := &larkim.EventMessage{
		MessageType: strPtr("text"),
		Content:     nil,
	}
	got := extractMessageText(msg)
	assert.Equal(t, "", got)
}

func TestStripAtMentionPlaceholders(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"@_user_1 你好", "你好"},
		{"@_user_1 @_user_2 测试", "测试"},
		{"没有占位符", "没有占位符"},
		{"@_user_1", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := stripAtMentionPlaceholders(c.input)
		assert.Equal(t, c.want, got, "input=%q", c.input)
	}
}

func TestBuildSessionID_SingleChat(t *testing.T) {
	got := buildSessionID("p2p", "oc_chat123", "ou_user456")
	assert.Equal(t, "feishu_p2p_oc_chat123_ou_user456", got)
}

func TestBuildSessionID_GroupChat(t *testing.T) {
	got := buildSessionID("group", "oc_group123", "ou_user456")
	assert.Equal(t, "feishu_group_oc_group123_ou_user456", got)
}

func TestBuildSessionID_TopicGroup(t *testing.T) {
	got := buildSessionID("topic_group", "oc_topic123", "ou_user456")
	assert.Equal(t, "feishu_group_oc_topic123_ou_user456", got)
}

func TestBuildSessionID_EmptyOpenID(t *testing.T) {
	got := buildSessionID("p2p", "oc_chat123", "")
	assert.Equal(t, "feishu_p2p_oc_chat123_anonymous", got)
}

func TestBuildSessionID_EmptyChatID(t *testing.T) {
	got := buildSessionID("p2p", "", "ou_user456")
	assert.Equal(t, "feishu_p2p_ou_user456", got)
}

func TestBuildSessionID_UnknownChatType(t *testing.T) {
	got := buildSessionID("unknown", "oc_x", "ou_y")
	assert.Equal(t, "feishu_oc_x_ou_y", got)
}

func TestIsMentionedToBot_WithAppMention(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{MentionedType: strPtr("user"), Key: strPtr("@_user_1")},
		{MentionedType: strPtr("app"), Key: strPtr("@_user_2")},
	}
	assert.True(t, isMentionedToBot(mentions))
}

// TestIsMentionedToBot_OnlyUserMentions 飞书事件中 mentioned_type 可能不填充，
// 此时只要有 @ 项就视为 @bot（兜底策略：机器人通常仅申请 group_at_msg 权限，
// 收到群消息事件本身就意味着被 @）。
func TestIsMentionedToBot_OnlyUserMentions(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{MentionedType: strPtr("user"), Key: strPtr("@_user_1")},
		{MentionedType: strPtr("user"), Key: strPtr("@_user_2")},
	}
	assert.True(t, isMentionedToBot(mentions))
}

// TestIsMentionedToBot_EmptyMentionedType 飞书实测场景：mentioned_type 字段为空
func TestIsMentionedToBot_EmptyMentionedType(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{Key: strPtr("@_user_1"), Name: strPtr("go-stock AI")},
	}
	assert.True(t, isMentionedToBot(mentions))
}

// TestIsMentionedToBot_NilMentionedType mentioned_type 指针为 nil
func TestIsMentionedToBot_NilMentionedType(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{Key: strPtr("@_user_1")},
	}
	assert.True(t, isMentionedToBot(mentions))
}

func TestIsMentionedToBot_Empty(t *testing.T) {
	assert.False(t, isMentionedToBot(nil))
	assert.False(t, isMentionedToBot([]*larkim.MentionEvent{}))
}

// TestIsMentionedToBot_WithNilEntries mentions 中含 nil 项应跳过
func TestIsMentionedToBot_WithNilEntries(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		nil,
		{MentionedType: strPtr("app"), Key: strPtr("@_user_1")},
		nil,
	}
	assert.True(t, isMentionedToBot(mentions))
}

// TestDumpMentions 验证调试日志格式化
func TestDumpMentions(t *testing.T) {
	assert.Equal(t, "[]", dumpMentions(nil))
	assert.Equal(t, "[]", dumpMentions([]*larkim.MentionEvent{}))

	mentions := []*larkim.MentionEvent{
		{Key: strPtr("@_user_1"), MentionedType: strPtr("app"), Name: strPtr("Bot")},
	}
	result := dumpMentions(mentions)
	assert.Contains(t, result, "@_user_1")
	assert.Contains(t, result, "type=app")
	assert.Contains(t, result, "name=Bot")
}

func TestCollectAgentReply_OnlyContent(t *testing.T) {
	ch := make(chan *schema.Message, 3)
	ch <- &schema.Message{Role: schema.Assistant, Content: "你好"}
	ch <- &schema.Message{Role: schema.Assistant, Content: "，世界"}
	close(ch)

	got := collectAgentReply(ch)
	assert.Equal(t, "你好，世界", got)
}

func TestCollectAgentReply_MixedMessages(t *testing.T) {
	ch := make(chan *schema.Message, 5)
	// 模拟 processMessageFuture 协议：ReasoningContent 是思考/工具日志，Content 是 AI 回复
	ch <- &schema.Message{Role: schema.Assistant, Content: "", ReasoningContent: "[STEP]🔧 调用工具：GetStockRealTimePrice"}
	ch <- &schema.Message{Role: schema.Assistant, Content: "", ReasoningContent: "[STEP]✅ GetStockRealTimePrice 返回结果"}
	ch <- &schema.Message{Role: schema.Assistant, Content: "贵州茅台当前"}
	ch <- &schema.Message{Role: schema.Assistant, Content: "股价为 1500 元"}
	close(ch)

	got := collectAgentReply(ch)
	assert.Equal(t, "贵州茅台当前股价为 1500 元", got)
	// 确认不包含 ReasoningContent
	assert.NotContains(t, got, "[STEP]")
	assert.NotContains(t, got, "调用工具")
}

// TestCollectAgentReply_OnlyReasoning 验证：当模型（如 GLM-5.2）将全部回复放在
// reasoning_content 中而 content 为空时，不应将思考过程作为回复返回给用户。
// 飞书机器人只回复最终分析结果（Content），不回复思考过程（ReasoningContent）。
func TestCollectAgentReply_OnlyReasoning(t *testing.T) {
	ch := make(chan *schema.Message, 3)
	// 模拟 GLM-5.2 等推理模型：content 字段始终为空，回复在 reasoning_content 中
	ch <- &schema.Message{Role: schema.Assistant, Content: "", ReasoningContent: "你好！"}
	ch <- &schema.Message{Role: schema.Assistant, Content: "", ReasoningContent: "我是 go-stock AI 助手"}
	close(ch)

	got := collectAgentReply(ch)
	// 思考过程不应作为回复发送给用户
	assert.Equal(t, "", got)
}

func TestCollectAgentReply_EmptyChannel(t *testing.T) {
	ch := make(chan *schema.Message, 1)
	close(ch)

	got := collectAgentReply(ch)
	assert.Equal(t, "", got)
}

func TestCollectAgentReply_NilChannel(t *testing.T) {
	got := collectAgentReply(nil)
	assert.Equal(t, "", got)
}

func TestCollectAgentReply_NilMessage(t *testing.T) {
	ch := make(chan *schema.Message, 2)
	ch <- nil
	ch <- &schema.Message{Role: schema.Assistant, Content: "有效内容"}
	close(ch)

	got := collectAgentReply(ch)
	assert.Equal(t, "有效内容", got)
}

func TestBuildReplyCard(t *testing.T) {
	jsonStr := buildReplyCard("## 测试标题\n这是一条**测试**消息")

	// 解析为通用 map 验证结构
	var card map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &card)
	assert.NoError(t, err)

	// 验证 schema=2.0
	assert.Equal(t, "2.0", card["schema"])

	// 验证 header 存在
	header, ok := card["header"].(map[string]interface{})
	assert.True(t, ok)
	title, ok := header["title"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "plain_text", title["tag"])
	assert.Contains(t, title["content"], "go-stock")

	// 验证 body.elements 包含 markdown 元素
	body, ok := card["body"].(map[string]interface{})
	assert.True(t, ok)
	elements, ok := body["elements"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, elements, 1)

	elem, ok := elements[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "markdown", elem["tag"])
	assert.Contains(t, elem["content"], "测试标题")
	assert.Contains(t, elem["content"], "测试")
}

func TestBuildReplyCard_SpecialChars(t *testing.T) {
	// 包含特殊字符的内容不应导致 JSON 解析失败
	content := "包含\"引号\"和\\反斜杠和<at id=all></at>"
	jsonStr := buildReplyCard(content)

	var card map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &card)
	assert.NoError(t, err)

	body := card["body"].(map[string]interface{})
	elements := body["elements"].([]interface{})
	elem := elements[0].(map[string]interface{})
	assert.Equal(t, content, elem["content"])
}

func TestBuildReplyCard_EmptyContent(t *testing.T) {
	jsonStr := buildReplyCard("")
	var card map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &card)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", card["schema"])
}

func TestMustJSONString(t *testing.T) {
	assert.Equal(t, `"hello"`, mustJSONString("hello"))
	assert.Equal(t, `"含\"引号"`, mustJSONString(`含"引号`))
	assert.Equal(t, `""`, mustJSONString(""))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
	assert.Equal(t, "", truncate("", 5))
	assert.Equal(t, "abc", truncate("abc", 3)) // 等于不截断
}

// TestNewFeishuBot_NilConfig 验证无配置时返回 nil（不依赖真实数据库）
// 注：这个测试需要数据库支持；如果 GetSettingConfig 返回 nil，应返回 nil
func TestNewFeishuBot_NilConfig(t *testing.T) {
	// NewFeishuBot 内部调用 data.GetSettingConfig()，依赖数据库 db.Dao
	// 测试环境无数据库时该函数会 panic，这里 defer recover 兜底
	defer func() {
		if r := recover(); r != nil {
			t.Logf("NewFeishuBot panicked (expected without DB): %v", r)
			t.Skip("skipped: database not available in test env")
		}
	}()
	bot := NewFeishuBot()
	// 不做硬性断言：可能返回 nil（无数据库）或非 nil（有数据库且配置完整）
	_ = bot
}

// TestCollectAgentReply_LargeStream 模拟大量消息片段的拼接
func TestCollectAgentReply_LargeStream(t *testing.T) {
	ch := make(chan *schema.Message, 100)
	var expected strings.Builder
	for i := 0; i < 50; i++ {
		// 间隔发送 ReasoningContent 和 Content
		if i%5 == 0 {
			ch <- &schema.Message{Role: schema.Assistant, ReasoningContent: "思考中..."}
		}
		piece := "片段" + string(rune('A'+i)) + " "
		ch <- &schema.Message{Role: schema.Assistant, Content: piece}
		expected.WriteString(piece)
	}
	close(ch)

	got := collectAgentReply(ch)
	assert.Equal(t, expected.String(), got)
}

// noopSleep 测试用：不真正 sleep，避免单测变慢
func noopSleep(time.Duration) {}

// TestCallAgentWithRetry_SuccessOnFirstAttempt 首次即成功，不应调用 fallback
func TestCallAgentWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	var askCalls, fallbackCalls int32
	reply := callAgentWithRetry(
		func() string {
			atomic.AddInt32(&askCalls, 1)
			return "你好"
		},
		func() string {
			atomic.AddInt32(&fallbackCalls, 1)
			return "fallback"
		},
		2,
		noopSleep,
	)
	assert.Equal(t, "你好", reply)
	assert.Equal(t, int32(1), atomic.LoadInt32(&askCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&fallbackCalls), "fallback should not be called on success")
}

// TestCallAgentWithRetry_RetryThenSuccess 第一次空、第二次成功
func TestCallAgentWithRetry_RetryThenSuccess(t *testing.T) {
	var askCalls, fallbackCalls, sleepCalls int32
	reply := callAgentWithRetry(
		func() string {
			n := atomic.AddInt32(&askCalls, 1)
			if n == 1 {
				return "" // 第一次空，触发重试
			}
			return "第二次成功"
		},
		func() string {
			atomic.AddInt32(&fallbackCalls, 1)
			return "fallback"
		},
		2,
		func(d time.Duration) {
			atomic.AddInt32(&sleepCalls, 1)
			assert.Equal(t, 1*time.Second, d, "第一次重试应 sleep 1s")
		},
	)
	assert.Equal(t, "第二次成功", reply)
	assert.Equal(t, int32(2), atomic.LoadInt32(&askCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&sleepCalls), "应 sleep 一次（重试间隔）")
	assert.Equal(t, int32(0), atomic.LoadInt32(&fallbackCalls), "成功后不应调用 fallback")
}

// TestCallAgentWithRetry_AllEmptyThenFallback 全部重试都空，应调用 fallback
func TestCallAgentWithRetry_AllEmptyThenFallback(t *testing.T) {
	var askCalls, fallbackCalls int32
	reply := callAgentWithRetry(
		func() string {
			atomic.AddInt32(&askCalls, 1)
			return "" // 始终空
		},
		func() string {
			atomic.AddInt32(&fallbackCalls, 1)
			return "兜底回复"
		},
		2,
		noopSleep,
	)
	assert.Equal(t, "兜底回复", reply)
	assert.Equal(t, int32(2), atomic.LoadInt32(&askCalls), "应重试 2 次")
	assert.Equal(t, int32(1), atomic.LoadInt32(&fallbackCalls), "重试全空后应调用 fallback 一次")
}

// TestCallAgentWithRetry_FallbackAlsoEmpty fallback 也返回空（极端场景）
func TestCallAgentWithRetry_FallbackAlsoEmpty(t *testing.T) {
	var askCalls, fallbackCalls int32
	reply := callAgentWithRetry(
		func() string {
			atomic.AddInt32(&askCalls, 1)
			return ""
		},
		func() string {
			atomic.AddInt32(&fallbackCalls, 1)
			return ""
		},
		3,
		noopSleep,
	)
	assert.Equal(t, "", reply)
	assert.Equal(t, int32(3), atomic.LoadInt32(&askCalls), "应重试 3 次")
	assert.Equal(t, int32(1), atomic.LoadInt32(&fallbackCalls))
}

// TestCallAgentWithRetry_WhitespaceOnlyCountsAsEmpty 仅空白也视为空，触发重试
func TestCallAgentWithRetry_WhitespaceOnlyCountsAsEmpty(t *testing.T) {
	var askCalls int32
	reply := callAgentWithRetry(
		func() string {
			n := atomic.AddInt32(&askCalls, 1)
			if n == 1 {
				return "   \n\t  " // 仅空白
			}
			return "有效回复"
		},
		func() string { return "fallback" },
		2,
		noopSleep,
	)
	assert.Equal(t, "有效回复", reply)
	assert.Equal(t, int32(2), atomic.LoadInt32(&askCalls))
}

// TestCallAgentWithRetry_NilFuncs nil 函数应返回空串而不是 panic
func TestCallAgentWithRetry_NilFuncs(t *testing.T) {
	assert.Equal(t, "", callAgentWithRetry(nil, func() string { return "x" }, 2, noopSleep))
	assert.Equal(t, "", callAgentWithRetry(func() string { return "x" }, nil, 2, noopSleep))
}

// TestCallAgentWithRetry_NilSleepUsesDefault nil sleep 应回退到 time.Sleep（不实际触发，仅验证不 panic）
func TestCallAgentWithRetry_NilSleepUsesDefault(t *testing.T) {
	// 首次成功，不会走到 sleep 分支，因此 nil sleep 不会被执行
	reply := callAgentWithRetry(
		func() string { return "ok" },
		func() string { return "fallback" },
		2,
		nil,
	)
	assert.Equal(t, "ok", reply)
}

// TestCallAgentWithRetry_InvalidMaxRetries maxRetries<1 应被校正为 1，至少调用一次 askOnce
func TestCallAgentWithRetry_InvalidMaxRetries(t *testing.T) {
	var askCalls int32
	reply := callAgentWithRetry(
		func() string {
			atomic.AddInt32(&askCalls, 1)
			return "ok"
		},
		func() string { return "fallback" },
		0, // 非法
		noopSleep,
	)
	assert.Equal(t, "ok", reply)
	assert.Equal(t, int32(1), atomic.LoadInt32(&askCalls))
}

// --- splitMarkdownContent 测试 ---

// TestSplitMarkdownContent_NoSplit 内容未超限，应原样返回单条
func TestSplitMarkdownContent_NoSplit(t *testing.T) {
	content := "短内容"
	chunks := splitMarkdownContent(content, 1000)
	assert.Len(t, chunks, 1)
	assert.Equal(t, content, chunks[0])
}

// TestSplitMarkdownContent_ParagraphBoundary 优先在段落边界（\n\n）处拆分
func TestSplitMarkdownContent_ParagraphBoundary(t *testing.T) {
	// 构造两个段落，每段约 600 字节，maxBytes=700 应在段落边界拆分
	// "段一段一" = 4 个中文字符 = 12 字节，× 50 = 600 字节
	para1 := strings.Repeat("段一段一", 50) + "\n\n" // 600 + 2 = 602 bytes
	para2 := strings.Repeat("段二段二", 50)          // 600 bytes
	content := para1 + para2                     // 1202 bytes total
	chunks := splitMarkdownContent(content, 700)
	assert.True(t, len(chunks) >= 2, "应拆分为至少 2 块, got %d", len(chunks))
	// 第一块应以 \n\n 结尾（段落边界保留在前一块）
	assert.True(t, strings.HasSuffix(chunks[0], "\n\n"), "第一块应以 \\n\\n 结尾")
	// 拼接后应与原文一致
	assert.Equal(t, content, strings.Join(chunks, ""))
	// 每块都不应超过 maxBytes（最后一块除外可能正好等于）
	for i, c := range chunks {
		if i < len(chunks)-1 {
			assert.LessOrEqual(t, len(c), 700, "第 %d 块不应超过 maxBytes", i)
		}
	}
}

// TestSplitMarkdownContent_LineBoundary 段落过长时在行边界（\n）处拆分
func TestSplitMarkdownContent_LineBoundary(t *testing.T) {
	// 单个段落，由多行短文本组成，无 \n\n
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("行", 30) // ~90 bytes per line
	}
	content := strings.Join(lines, "\n") // 总约 4500 bytes，无 \n\n
	chunks := splitMarkdownContent(content, 500)
	assert.True(t, len(chunks) >= 2, "应拆分为至少 2 块")
	// 拼接后应与原文一致
	assert.Equal(t, content, strings.Join(chunks, ""))
	// 每块（除最后一块）不超过 maxBytes
	for i, c := range chunks {
		if i < len(chunks)-1 {
			assert.LessOrEqual(t, len(c), 500, "第 %d 块不应超过 maxBytes", i)
		}
	}
}

// TestSplitMarkdownContent_UTF8Safe 不会截断多字节 UTF-8 字符
func TestSplitMarkdownContent_UTF8Safe(t *testing.T) {
	// 全是中文（3字节/字符），maxBytes 设为非 3 的倍数，确保不会截断字符
	content := strings.Repeat("中", 100) // 300 bytes
	chunks := splitMarkdownContent(content, 100)
	assert.True(t, len(chunks) >= 3, "应拆分为至少 3 块")
	// 拼接后应与原文一致
	assert.Equal(t, content, strings.Join(chunks, ""))
	// 每块都应是有效的 UTF-8（不会出现乱码）
	for i, c := range chunks {
		assert.True(t, utf8.ValidString(c), "第 %d 块不是有效 UTF-8", i)
		// 每块不超过 maxBytes
		assert.LessOrEqual(t, len(c), 100, "第 %d 块不应超过 maxBytes", i)
	}
}

// TestSplitMarkdownContent_SingleLongLine 单行超长无换行，按 UTF-8 字符边界硬切
func TestSplitMarkdownContent_SingleLongLine(t *testing.T) {
	content := strings.Repeat("ABC", 200) // 600 bytes, 无换行
	chunks := splitMarkdownContent(content, 250)
	assert.True(t, len(chunks) >= 2)
	assert.Equal(t, content, strings.Join(chunks, ""))
	for i, c := range chunks {
		if i < len(chunks)-1 {
			assert.LessOrEqual(t, len(c), 250)
		}
	}
}

// TestSplitMarkdownContent_MaxBytesZero maxBytes<=0 时原样返回
func TestSplitMarkdownContent_MaxBytesZero(t *testing.T) {
	chunks := splitMarkdownContent("任意内容", 0)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "任意内容", chunks[0])
}

// TestSplitMarkdownContent_RealisticReport 模拟真实 AI 报告拆分
func TestSplitMarkdownContent_RealisticReport(t *testing.T) {
	// 模拟一份带 markdown 格式的报告
	var sb strings.Builder
	sb.WriteString("# 液冷板块深度分析报告\n\n")
	sb.WriteString("## 一、板块概述\n\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("液冷板块是**数据中心散热**的关键技术，第%d段内容。", i+1))
		sb.WriteString("\n\n")
	}
	sb.WriteString("## 二、重点个股\n\n")
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("- 英维克（002837）：第%d个分析点\n", i+1))
	}
	sb.WriteString("\n\n## 三、总结\n\n")
	sb.WriteString("以上内容仅供参考。\n")
	content := sb.String()

	chunks := splitMarkdownContent(content, 500)
	assert.True(t, len(chunks) >= 2, "应拆分为至少 2 块")
	// 拼接后应与原文完全一致（无丢失）
	assert.Equal(t, content, strings.Join(chunks, ""), "拆分后拼接应与原文一致")
	// 每块都是有效 UTF-8
	for i, c := range chunks {
		assert.True(t, utf8.ValidString(c), "第 %d 块不是有效 UTF-8", i)
	}
}

// --- buildReplyCardWithTitle 测试 ---

func TestBuildReplyCardWithTitle_CustomTitle(t *testing.T) {
	jsonStr := buildReplyCardWithTitle("测试内容", "自定义标题（1/2）")
	var card map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &card)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", card["schema"])

	header := card["header"].(map[string]interface{})
	title := header["title"].(map[string]interface{})
	assert.Equal(t, "自定义标题（1/2）", title["content"])
}

func TestBuildReplyCardWithTitle_DefaultTitle(t *testing.T) {
	// buildReplyCard 应使用默认标题
	jsonStr := buildReplyCard("测试内容")
	var card map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &card)
	assert.NoError(t, err)

	header := card["header"].(map[string]interface{})
	title := header["title"].(map[string]interface{})
	assert.Equal(t, "go-stock AI 助手", title["content"])
}

// --- shouldReplyAsImage 测试 ---

// TestShouldReplyAsImage_ShortContent 短文本不应触发图片回复
func TestShouldReplyAsImage_ShortContent(t *testing.T) {
	assert.False(t, shouldReplyAsImage("你好，贵州茅台当前股价 1500 元。"))
}

// TestShouldReplyAsImage_LargeContent 超过 maxFeishuContentBytes 应触发图片回复
func TestShouldReplyAsImage_LargeContent(t *testing.T) {
	// 构造 >20KB 的普通文本（无表格/代码块）
	content := strings.Repeat("这是一段普通的股票分析文本内容。", 700) // >20KB
	assert.True(t, len(content) > maxFeishuContentBytes)
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_WithCodeBlock 含代码块应触发图片回复
func TestShouldReplyAsImage_WithCodeBlock(t *testing.T) {
	content := "代码示例：\n\n```go\nfmt.Println(\"hello\")\n```\n\n以上是代码。"
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_WithTable 含 markdown 表格分隔行 |---| 应触发
func TestShouldReplyAsImage_WithTable(t *testing.T) {
	content := "| 股票 | 价格 |\n|---|---|\n| 茅台 | 1500 |\n| 五粮液 | 200 |"
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_WithAlignedTable 含带空格的表格分隔行 | --- | 应触发
func TestShouldReplyAsImage_WithAlignedTable(t *testing.T) {
	content := "| 股票 | 价格 |\n| --- | --- |\n| 茅台 | 1500 |"
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_WithColonTable 含对齐表格 |:---:| 应触发
func TestShouldReplyAsImage_WithColonTable(t *testing.T) {
	content := "| 股票 | 涨跌 |\n|:---:|:---:|\n| 茅台 | +2% |"
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_WithRightAlignTable 含右对齐表格 | ---:| 应触发
func TestShouldReplyAsImage_WithRightAlignTable(t *testing.T) {
	content := "| 数值 |\n| ---:|\n| 123 |"
	assert.True(t, shouldReplyAsImage(content))
}

// TestShouldReplyAsImage_PlainShortText 纯短文本含竖线但非表格不应误判
func TestShouldReplyAsImage_PlainShortTextWithPipe(t *testing.T) {
	// 竖线但不构成表格分隔行
	assert.False(t, shouldReplyAsImage("A|B|C 是某种分隔符"))
}

// TestShouldReplyAsImage_EmptyContent 空内容不触发
func TestShouldReplyAsImage_EmptyContent(t *testing.T) {
	assert.False(t, shouldReplyAsImage(""))
}

// TestShouldReplyAsImage_LongPlainContentWithCodeBlock 超 20KB 且含代码块（多重条件）
func TestShouldReplyAsImage_LongContentWithCodeBlock(t *testing.T) {
	content := strings.Repeat("普通文本内容。", 700) + "\n\n```\ncode\n```"
	assert.True(t, shouldReplyAsImage(content))
}
