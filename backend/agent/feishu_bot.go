package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go-stock/backend/data"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// @Author spark
// @Date 2026/07/07
// @Desc 飞书应用机器人（长连接模式，接收用户消息并由 AI Agent 回复）
// 文档：https://open.feishu.cn/document/server-side-sdk/golang-sdk-guide/handle-events
//-----------------------------------------------------------------------------------

// FeishuBot 飞书应用机器人
// 通过 larkws 长连接接收 im.message.receive_v1 事件，调用 StockAiAgent.ChatWithContext
// 完成 AI 对话，再通过 lark.Client 的 Im.Message.Reply API 回复 interactive 卡片 2.0。
type FeishuBot struct {
	mu          sync.Mutex
	wsClient    *larkws.Client // 长连接客户端（接收事件）
	apiClient   *lark.Client   // API 客户端（发送消息）
	cancel      context.CancelFunc
	appID       string
	appSecret   string
	aiConfigId  int    // 使用的 AI 配置 ID
	sysPromptId int    // 系统提示词 ID（0 = 默认）
	thinking    bool   // 是否启用思考模式
	enableTools bool   // 是否启用工具调用（false 时走单轮 chat）
	memory      bool   // 是否启用多轮记忆（默认关闭：群聊场景历史对话意义有限且挤占上下文）
	agentMode   string // Agent 模式：react/plan_execute/deepagents（空=自动判断）
	running     bool
}

// NewFeishuBot 根据当前配置创建机器人实例；配置缺失返回 nil
func NewFeishuBot() *FeishuBot {
	cfg := data.GetSettingConfig()
	if cfg == nil {
		return nil
	}
	appID := strings.TrimSpace(cfg.FeishuAppID)
	appSecret := strings.TrimSpace(cfg.FeishuAppSecret)
	if appID == "" || appSecret == "" {
		logger.SugaredLogger.Warnf("feishu bot config missing: appID/appSecret empty")
		return nil
	}
	aiConfigId := cfg.FeishuBotAiConfigId
	if aiConfigId <= 0 && len(cfg.AiConfigs) > 0 {
		aiConfigId = int(cfg.AiConfigs[0].ID)
	}
	if aiConfigId <= 0 {
		logger.SugaredLogger.Warnf("feishu bot config missing: no ai config id")
		return nil
	}
	return &FeishuBot{
		appID:       appID,
		appSecret:   appSecret,
		aiConfigId:  aiConfigId,
		sysPromptId: cfg.FeishuBotSysPromptId,
		thinking:    cfg.FeishuBotThinking,
		enableTools: cfg.FeishuBotEnableTools,
		memory:      cfg.FeishuBotMemoryEnable,
		agentMode:   cfg.FeishuBotAgentMode,
	}
}

// Start 启动长连接（阻塞，需 go 调用）。ctx 退出时方法返回。
func (b *FeishuBot) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("feishu bot already running")
	}

	// 创建事件分发器（长连接模式下两个参数必须为空字符串）
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// 飞书要求事件回调 3 秒内返回，否则会触发重试。
			// 这里立即 return nil，AI 处理放到 goroutine 异步执行。
			go b.processEvent(ctx, event)
			return nil
		})

	// 创建 API 客户端（用于发送消息）
	b.apiClient = lark.NewClient(b.appID, b.appSecret,
		lark.WithLogLevel(larkcore.LogLevelInfo),
	)

	// 创建长连接客户端（用于接收事件）
	b.wsClient = larkws.NewClient(b.appID, b.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
		larkws.WithOnReady(func() {
			logger.SugaredLogger.Infof("feishu bot connected (app_id=%s)", b.appID)
		}),
		larkws.WithOnError(func(err error) {
			logger.SugaredLogger.Errorf("feishu bot error: %v", err)
		}),
		larkws.WithOnReconnecting(func() {
			logger.SugaredLogger.Infof("feishu bot reconnecting...")
		}),
		larkws.WithOnReconnected(func() {
			logger.SugaredLogger.Infof("feishu bot reconnected")
		}),
		larkws.WithOnDisconnected(func() {
			logger.SugaredLogger.Infof("feishu bot disconnected")
		}),
	)

	ctx, b.cancel = context.WithCancel(ctx)
	b.running = true
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.running = false
		b.wsClient = nil
		b.apiClient = nil
		b.mu.Unlock()
	}()

	logger.SugaredLogger.Infof("feishu bot starting (app_id=%s, ai_config_id=%d, tools=%v, thinking=%v, memory=%v)",
		b.appID, b.aiConfigId, b.enableTools, b.thinking, b.memory)

	// Start 阻塞；ctx 取消时调用 Close 关闭连接
	go func() {
		<-ctx.Done()
		logger.SugaredLogger.Infof("feishu bot context done, closing connection")
		if b.wsClient != nil {
			b.wsClient.Close()
		}
	}()

	return b.wsClient.Start(ctx)
}

// Stop 关闭连接
func (b *FeishuBot) Stop() {
	b.mu.Lock()
	cancel := b.cancel
	wsClient := b.wsClient
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if wsClient != nil {
		wsClient.Close()
	}
}

// IsRunning 状态查询
func (b *FeishuBot) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// processEvent 异步处理消息事件：提取文本 → 调用 AI → 回复卡片
func (b *FeishuBot) processEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in feishu bot processEvent: %v", r)
		}
	}()

	if event == nil || event.Event == nil || event.Event.Message == nil {
		return
	}

	message := event.Event.Message
	sender := event.Event.Sender

	// 仅处理文本类型消息；其他类型（图片/文件等）暂不支持
	if message.MessageType == nil || *message.MessageType != "text" {
		logger.SugaredLogger.Debugf("feishu bot skip non-text message: type=%v", message.MessageType)
		return
	}

	// 群聊消息仅在 @机器人 时回复；单聊（p2p）直接回复
	chatType := ""
	if message.ChatType != nil {
		chatType = *message.ChatType
	}
	if chatType == "group" || chatType == "topic_group" {
		if !isMentionedToBot(message.Mentions) {
			// 打印 mentions 详情便于排查（飞书事件中 mentioned_type 可能不填充）
			logger.SugaredLogger.Debugf("feishu bot skip group message without @bot: mentions=%s", dumpMentions(message.Mentions))
			return
		}
	}

	// 提取消息文本（去除 @机器人 标记）
	text := extractMessageText(message)
	if strings.TrimSpace(text) == "" {
		return
	}

	// 获取发送者 open_id 与 chat_id 用于构造 sessionID
	openID := ""
	if sender != nil && sender.SenderId != nil && sender.SenderId.OpenId != nil {
		openID = *sender.SenderId.OpenId
	}
	chatID := ""
	if message.ChatId != nil {
		chatID = *message.ChatId
	}
	messageID := ""
	if message.MessageId != nil {
		messageID = *message.MessageId
	}

	sessionID := buildSessionID(chatType, chatID, openID)
	logger.SugaredLogger.Infof("feishu bot received message: from=%s chat=%s type=%s session=%s text=%q",
		openID, chatID, chatType, sessionID, truncate(text, 200))

	// 先发占位卡片并拿到其 message_id，AI 运行期间通过 [STEP] 进度限频更新该卡片，
	// 最终把完整回复回填进同一张卡（参考 agent 前端的步骤流式展示）。
	// 占位卡发送失败时降级为「跑完后一次性回复」（与旧行为一致）。
	progressCardID := ""
	if cardID, err := b.sendCardReply(messageID, "⏳ AI 正在分析，请稍候...", "go-stock AI 助手"); err == nil {
		progressCardID = cardID
	} else {
		logger.SugaredLogger.Warnf("feishu bot progress card send failed, fallback to one-shot reply: %v", err)
	}

	// 进度回调：交给 askAgentOnce 消费 channel 时驱动，限频 PATCH 更新占位卡片
	progress := func(step string) {
		if progressCardID == "" || step == "" {
			return
		}
		if err := b.updateProgressCard(progressCardID, step); err != nil {
			logger.SugaredLogger.Debugf("feishu bot progress update failed: %v", err)
		}
	}

	// 调用 AI Agent
	reply := b.callAgent(ctx, text, sessionID, progress)
	if strings.TrimSpace(reply) == "" {
		logger.SugaredLogger.Warnf("feishu bot got empty reply for session=%s question=%q", sessionID, truncate(text, 200))
		reply = "AI 暂时无法生成回复，请稍后重试或简化您的问题。"
	}

	// 清理可能残留的历史数值脱敏占位符（模型有时会把上下文中的占位符原样回显）
	reply = stripRedactedPlaceholders(reply)

	// 有占位卡片：最终内容 PATCH 回填（图片场景改提示语+另发图片）；
	// 无占位卡片：走原有一次性回复逻辑。
	if progressCardID != "" {
		b.finalizeProgressCard(progressCardID, messageID, reply)
		return
	}
	if err := b.replyMessage(messageID, reply); err != nil {
		logger.SugaredLogger.Errorf("feishu bot reply failed: %v", err)
	}
}

// stripRedactedPlaceholders 清理回复中可能残留的历史数值脱敏占位符。
//
// 历史记忆中可能仍存有旧版脱敏占位符（[历史数值已省略，请重新调用工具查询] 或 [旧值]），
// 模型有时会把上下文中的占位符原样回显到最终回复中。在发送给用户前清理这些占位符，
// 确保回复中只包含实际数值（来自工具调用）。
func stripRedactedPlaceholders(content string) string {
	if !strings.Contains(content, "[旧值]") &&
		!strings.Contains(content, "[历史数值已省略") {
		return content
	}
	cleaned := strings.ReplaceAll(content, "[历史数值已省略，请重新调用工具查询]", "")
	cleaned = strings.ReplaceAll(cleaned, "[旧值]", "")
	// 清理可能留下的多余空格（如 "价格 [旧值] 元" → "价格  元" → "价格 元"）
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	return cleaned
}

// callAgent 调用 AI 生成回复。
//
// 根据 agentMode 配置选择调用路径：
//   - "direct"：直接走 NewDeepSeekOpenAi + AskAi（无工具、无记忆），最快最简单，
//     适合不需要实时数据、只需模型自身知识的场景（无 [STEP] 进度，onStep 不会被调用）
//   - 其他（react/plan_execute/deepagents/""）：走 Agent 管线（工具调用+多轮记忆），
//     最多重试 2 次（空回复时退避 1s/2s），仍为空时降级到 askAIFallback。
//     onStep 非空时，Agent 运行期间的 [STEP] 阶段/工具日志会实时回调（用于进度卡片展示）
//
// 参考 qq_bot.go processAndReply 的实现方式。
func (b *FeishuBot) callAgent(ctx context.Context, question, sessionID string, onStep func(step string)) string {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in feishu bot callAgent: %v", r)
		}
	}()

	// direct 模式：跳过 eino Agent 框架，直接用 NewDeepSeekOpenAi + AskAiWithTools
	if b.agentMode == "direct" {
		reply := b.askDirectWithTools(ctx, question, sessionID)
		if strings.TrimSpace(reply) != "" {
			logger.SugaredLogger.Infof("feishu bot direct AI reply: len=%d preview=%q", len(reply), truncate(reply, 200))
		} else {
			logger.SugaredLogger.Warnf("feishu bot direct AI returned empty, session=%s question=%q",
				sessionID, truncate(question, 200))
		}
		return reply
	}

	// Agent 模式：重试 + 兜底
	return callAgentWithRetry(
		func() string {
			reply := b.askAgentOnce(ctx, question, sessionID, onStep)
			if strings.TrimSpace(reply) != "" {
				logger.SugaredLogger.Infof("feishu bot AI reply: len=%d preview=%q", len(reply), truncate(reply, 200))
			} else {
				logger.SugaredLogger.Warnf("feishu bot AI returned empty, session=%s question=%q",
					sessionID, truncate(question, 200))
			}
			return reply
		},
		func() string {
			logger.SugaredLogger.Warnf("feishu bot agent empty after retries, falling back to direct OpenAI, session=%s",
				sessionID)
			reply := b.askAIFallback(ctx, question)
			if reply != "" {
				logger.SugaredLogger.Infof("feishu bot fallback reply: len=%d preview=%q", len(reply), truncate(reply, 200))
			}
			return reply
		},
		2,
		time.Sleep,
	)
}

// callAgentWithRetry 执行「重试 + 兜底」的编排逻辑，抽出来便于单元测试。
//   - askOnce 每次返回空字符串时触发重试，最多 maxRetries 次
//   - 重试之间用 sleep 做指数退避（1s, 2s, ...）；测试时可传 no-op sleep
//   - 全部重试后仍为空则调用 fallback
//
// 返回值为最终回复（保留原始空白，仅用 TrimSpace 判空）。
func callAgentWithRetry(askOnce, fallback func() string, maxRetries int, sleep func(time.Duration)) string {
	if askOnce == nil || fallback == nil {
		return ""
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	if maxRetries < 1 {
		maxRetries = 1
	}
	var reply string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		reply = askOnce()
		if strings.TrimSpace(reply) != "" {
			return reply
		}
		if attempt < maxRetries {
			sleep(time.Duration(attempt) * time.Second)
		}
	}
	return fallback()
}

// askAgentOnce 执行一次 Agent 调用并收集回复（单次尝试）。
//
// 不再设置外层超时：ChatWithContext 内部已用 estimateAgentRunBudget 建立运行预算
// （执行不限时、仅工具调用次数上限 100~200 次），外层 5 分钟超时会先到期掐断
// DeepAgents 长任务（表现为 context deadline exceeded），与主应用策略冲突。
// 停止机器人时父 ctx（飞书事件 ctx）仍可级联取消进行中的调用。
// onStep 非空时实时回调 [STEP] 阶段/工具日志（用于进度卡片展示）。
func (b *FeishuBot) askAgentOnce(ctx context.Context, question, sessionID string, onStep func(step string)) string {
	agentApi := NewStockAiAgentApi()
	var sysPromptId *int
	if b.sysPromptId > 0 {
		id := b.sysPromptId
		sysPromptId = &id
	}

	// agentMode: 优先使用配置的 Agent 模式（react/plan_execute/deepagents）；
	// 未配置时 enableTools=true 默认 react，enableTools=false 传 "" 由 classifyComplexity 自动判断。
	agentMode := b.agentMode
	if agentMode == "" && b.enableTools {
		agentMode = "react"
	}

	logger.SugaredLogger.Infof("feishu bot calling AI: config_id=%d tools=%v thinking=%v mode=%q session=%s question=%q",
		b.aiConfigId, b.enableTools, b.thinking, agentMode, sessionID, truncate(question, 200))

	ch := agentApi.ChatWithContext(
		ctx,
		question,
		b.aiConfigId,
		sysPromptId,
		b.memory,   // memoryMode：默认关闭（设置页 feishuBotMemoryEnable 开启）
		1,          // memoryCount：开启后仅加载最近一轮对话（ChatMemoryService 内部 ×2 = 用户+助手各 1 条）
		b.thinking, // thinkingMode
		agentMode,
		"",        // sysPromptOverride（使用 sysPromptId）
		sessionID, // sessionIDOverride
	)

	return collectAgentReplyWithProgress(ch, onStep)
}

// askDirectWithTools 直接模式：用 NewDeepSeekOpenAi + AskAiWithTools 完成
// 带工具调用的 chat completion（不走 eino Agent 框架，更轻量）。
//
// 与 askAIFallback 的区别：
//   - askAIFallback 用 AskAi（无工具），仅作为 Agent 失败时的兜底
//   - askDirectWithTools 用 AskAiWithTools（带工具），支持实时数据查询
//
// 当 enableTools=false 时传入空 tools 列表，AskAiWithTools 内部自动降级为 AskAi。
// 适合不需要 Agent 多轮规划、但需要工具调用获取实时数据的场景。
func (b *FeishuBot) askDirectWithTools(ctx context.Context, question, sessionID string) string {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in feishu bot askDirectWithTools: %v", r)
		}
	}()

	cfg := data.GetSettingConfig()
	if cfg == nil || !cfg.OpenAiEnable || len(cfg.AiConfigs) == 0 {
		logger.SugaredLogger.Warnf("feishu bot askDirectWithTools: AI not enabled or no ai configs")
		return ""
	}

	aiConfigId := b.aiConfigId
	if aiConfigId <= 0 {
		aiConfigId = int(cfg.AiConfigs[0].ID)
	}

	// 独立超时（5 分钟），工具调用可能多轮
	directCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	oai := data.NewDeepSeekOpenAi(directCtx, aiConfigId)
	if oai == nil {
		logger.SugaredLogger.Errorf("feishu bot askDirectWithTools: create OpenAi instance failed, aiConfigId=%d", aiConfigId)
		return ""
	}

	// 加载系统提示词（配置了 sysPromptId 则用配置的，否则用默认）
	sysPrompt := ""
	if b.sysPromptId > 0 {
		sysPrompt = data.NewPromptTemplateApi().GetPromptTemplateByID(b.sysPromptId)
	}
	if sysPrompt == "" {
		sysPrompt = "你是一个专业的股票分析助手。请通过工具调用获取实时数据，给出专业的分析。如果无法获取实时数据，请根据你的知识给出参考性分析，并明确说明数据可能不是最新的。"
	}

	// 构建消息（含当前时间，对齐 NewSummaryStockNewsStreamWithTools 的格式）
	now := time.Now().Format("2006-01-02 15:04:05")
	questionWithTime := fmt.Sprintf("当前时间: %s\n\n%s", now, question)
	msg := []map[string]interface{}{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "当前时间"},
		{"role": "assistant", "reasoning_content": "使用工具查询", "content": "当前本地时间是:" + now},
		{"role": "user", "content": questionWithTime},
	}

	// 获取工具：enableTools=true 时按问题相关性过滤；false 时传空（AskAiWithTools 内部降级为 AskAi）
	var tools []data.Tool
	if b.enableTools {
		tools = data.ToolsForQuestion(question)
	}
	logger.SugaredLogger.Infof("feishu bot direct mode: config_id=%d tools=%d thinking=%v session=%s question=%q",
		aiConfigId, len(tools), b.thinking, sessionID, truncate(question, 200))

	ch := make(chan map[string]any, 512)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in feishu bot askDirectWithTools goroutine: %v", r)
			}
		}()
		data.AskAiWithTools(oai, fmt.Errorf("feishu bot direct"), msg, ch, questionWithTime, tools, b.thinking)
	}()

	// 收集回复：code!=0 且 content 非空（跳过 reasoning_content 和错误）
	var contentBuilder strings.Builder
	for item := range ch {
		if item == nil {
			continue
		}
		code, _ := item["code"].(float64)
		if code == 0 {
			continue
		}
		content, _ := item["content"].(string)
		if content != "" {
			contentBuilder.WriteString(content)
		}
	}

	result := contentBuilder.String()
	if result == "" {
		logger.SugaredLogger.Warnf("feishu bot askDirectWithTools returned empty")
	}
	return result
}

// askAIFallback 直接调用 OpenAI 完成一次简单 chat completion（无工具、无记忆）。
//
// 作为 Agent 模式的兜底：Agent 重试 2 次仍空时降级调用此方法。
// 参考 qq_bot.go askAIFallback 的实现方式，用 NewDeepSeekOpenAi + AskAi 发起
// 无工具流式 chat completion（2 分钟独立超时）。可应对 eino msgFuture fork 丢失
// Content、推理模型 token 耗尽 finish_reason=length、工具调用异常等场景。
func (b *FeishuBot) askAIFallback(ctx context.Context, question string) string {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in feishu bot askAIFallback: %v", r)
		}
	}()

	cfg := data.GetSettingConfig()
	if cfg == nil || !cfg.OpenAiEnable || len(cfg.AiConfigs) == 0 {
		logger.SugaredLogger.Warnf("feishu bot askAIFallback: AI not enabled or no ai configs")
		return ""
	}

	aiConfigId := b.aiConfigId
	if aiConfigId <= 0 {
		aiConfigId = int(cfg.AiConfigs[0].ID)
	}

	// 兜底调用独立超时（2 分钟），避免 Agent 已耗时较长后再卡死
	fallbackCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	oai := data.NewDeepSeekOpenAi(fallbackCtx, aiConfigId)
	if oai == nil {
		logger.SugaredLogger.Errorf("feishu bot askAIFallback: create OpenAi instance failed, aiConfigId=%d", aiConfigId)
		return ""
	}

	questionWithTime := fmt.Sprintf("当前时间: %s\n\n%s", time.Now().Format("2006-01-02 15:04:05"), question)
	prompt := "你是一个专业的股票分析助手。请用简洁的中文回答以下问题。如果无法获取实时数据，请根据你的知识给出参考性分析，并明确说明数据可能不是最新的。"
	chatMsgs := []map[string]interface{}{
		{"role": "system", "content": prompt},
		{"role": "user", "content": questionWithTime},
	}

	ch := make(chan map[string]any, 512)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in feishu bot askAIFallback goroutine: %v", r)
			}
		}()
		data.AskAi(oai, fmt.Errorf("feishu bot fallback"), chatMsgs, ch, questionWithTime, false)
	}()

	var contentBuilder strings.Builder
	for item := range ch {
		if item == nil {
			continue
		}
		// code==0 表示错误（AskAi 在 HTTP/stream 错误时发送），跳过
		code, _ := item["code"].(float64)
		if code == 0 {
			continue
		}
		content, _ := item["content"].(string)
		if content != "" {
			contentBuilder.WriteString(content)
		}
	}

	result := contentBuilder.String()
	if result == "" {
		logger.SugaredLogger.Warnf("feishu bot askAIFallback returned empty")
	}
	return result
}

// maxFeishuContentBytes 单条飞书卡片消息中 markdown 内容的最大字节数。
//
// 飞书文档明确：卡片消息请求体最大不能超过 30KB，且「如果消息中包含样式标签，
// 会使实际消息体长度大于您输入的请求体长度」。即 **bold**、# 标题、| 表格 | 等
// markdown 样式在飞书内部会被展开为更长的结构，导致 21KB 的 markdown 实际可能
// 膨胀到 30KB+ 被拒绝。这里取 20KB 作为保守阈值，预留 JSON 转义+样式展开的空间。
//
// 文档：https://open.feishu.cn/document/server-docs/im-v1/message/reply
const maxFeishuContentBytes = 20000

// replyMessage 调用 Im.Message.Reply 回复消息。
//
// 回复策略（智能自动）：
//  1. 内容超 maxFeishuContentBytes 或含表格/代码块时，优先尝试图片回复（渲染 markdown
//     为 PNG → 上传飞书 → 发送 image 消息），彻底绕过 30KB 卡片限制且渲染效果更好
//  2. 图片回复失败时降级为卡片回复（chromedp 不可用、上传失败等场景）
//  3. 短内容直接走卡片回复（单条）；超长内容走分片卡片回复
func (b *FeishuBot) replyMessage(messageID, content string) error {
	if b.apiClient == nil {
		return fmt.Errorf("api client is nil")
	}
	if messageID == "" {
		return fmt.Errorf("message id is empty")
	}

	// 智能自动：内容较大或含复杂格式时优先图片回复
	if shouldReplyAsImage(content) {
		if err := b.replyAsImage(messageID, content); err != nil {
			logger.SugaredLogger.Warnf("feishu bot image reply failed, falling back to card: %v", err)
			// 降级到卡片回复（继续走下方逻辑）
		} else {
			return nil
		}
	}

	// 内容较小时直接发送单条卡片
	if len(content) <= maxFeishuContentBytes {
		_, err := b.sendCardReply(messageID, content, "go-stock AI 助手")
		return err
	}

	// 内容过大时拆分为多条消息
	chunks := splitMarkdownContent(content, maxFeishuContentBytes)
	logger.SugaredLogger.Infof("feishu bot reply too large (content=%d bytes), splitting into %d messages",
		len(content), len(chunks))

	for i, chunk := range chunks {
		title := "go-stock AI 助手"
		if len(chunks) > 1 {
			title = fmt.Sprintf("go-stock AI 助手（%d/%d）", i+1, len(chunks))
		}
		if _, err := b.sendCardReply(messageID, chunk, title); err != nil {
			return fmt.Errorf("reply chunk %d/%d failed: %w", i+1, len(chunks), err)
		}
		// 拆分发送时避免触发飞书 5 QPS 限频
		if i < len(chunks)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return nil
}

// progressPatchMinInterval 进度卡片 PATCH 最小间隔（限频，避免触发飞书 QPS 限制）。
// 阶段消息本身低频（模式启动/计划/委派/工具调用），2s 足够跟手。
const progressPatchMinInterval = 2 * time.Second

// maxProgressSteps 进度卡片保留的最近步骤条数（展示最近轨迹，避免卡片无限增长）。
const maxProgressSteps = 6

// progressCardState 进度卡片的可变状态（最近步骤 + 限频时间戳），仅 bot 实例内使用。
type progressCardState struct {
	steps     []string
	lastPatch time.Time
}

// 全局进度状态按卡片 message_id 隔离（一个会话一张占位卡，天然不冲突）。
var progressStates sync.Map // map[string]*progressCardState

// updateProgressCard 用最新步骤限频更新占位卡片。
// 限频策略：距上次 PATCH 不足 progressPatchMinInterval 时跳过（步骤照记，下次 PATCH 一并展示）。
func (b *FeishuBot) updateProgressCard(cardMessageID, step string) error {
	stateAny, _ := progressStates.LoadOrStore(cardMessageID, &progressCardState{})
	state := stateAny.(*progressCardState)

	state.steps = append(state.steps, step)
	if len(state.steps) > maxProgressSteps {
		state.steps = state.steps[len(state.steps)-maxProgressSteps:]
	}

	now := time.Now()
	if now.Sub(state.lastPatch) < progressPatchMinInterval {
		return nil
	}
	state.lastPatch = now

	content := "⏳ AI 正在分析：\n" + strings.Join(state.steps, "\n")
	return b.patchCardContent(cardMessageID, content)
}

// finalizeProgressCard 把最终回复回填进占位卡片。
//   - 普通内容：PATCH 完整内容（≤20KB）
//   - 超长：PATCH 第一片，其余片新 Reply（复用分片逻辑）
//   - 图片场景（>500 字/表格/代码块）：PATCH 简短完成提示，另发渲染图片
func (b *FeishuBot) finalizeProgressCard(cardMessageID, sourceMessageID, reply string) {
	progressStates.Delete(cardMessageID)

	// 图片场景：占位卡改为完成提示，完整内容以图片另发
	if shouldReplyAsImage(reply) {
		if err := b.patchCardContent(cardMessageID, "✅ 分析完成，内容较长，请查看下方图片。"); err != nil {
			logger.SugaredLogger.Warnf("feishu bot finalize patch failed: %v", err)
		}
		if err := b.replyAsImage(sourceMessageID, reply); err != nil {
			logger.SugaredLogger.Warnf("feishu bot image reply failed, falling back to card: %v", err)
			_, _ = b.sendCardReply(sourceMessageID, reply, "go-stock AI 助手")
		}
		return
	}

	// 普通内容：第一片 PATCH 回占位卡，其余片 Reply
	if len(reply) <= maxFeishuContentBytes {
		if err := b.patchCardContent(cardMessageID, reply); err != nil {
			logger.SugaredLogger.Warnf("feishu bot finalize patch failed, sending new card: %v", err)
			_, _ = b.sendCardReply(sourceMessageID, reply, "go-stock AI 助手")
		}
		return
	}

	chunks := splitMarkdownContent(reply, maxFeishuContentBytes)
	logger.SugaredLogger.Infof("feishu bot finalize too large (content=%d bytes), splitting into %d messages",
		len(reply), len(chunks))
	if err := b.patchCardContent(cardMessageID, chunks[0]); err != nil {
		logger.SugaredLogger.Warnf("feishu bot finalize patch failed: %v", err)
	}
	for i := 1; i < len(chunks); i++ {
		title := fmt.Sprintf("go-stock AI 助手（%d/%d）", i+1, len(chunks))
		if _, err := b.sendCardReply(sourceMessageID, chunks[i], title); err != nil {
			logger.SugaredLogger.Errorf("feishu bot reply chunk %d/%d failed: %v", i+1, len(chunks), err)
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// patchCardContent 更新已发送卡片消息的内容（Im.Message.Patch，卡片整体替换）。
func (b *FeishuBot) patchCardContent(cardMessageID, content string) error {
	if b.apiClient == nil {
		return fmt.Errorf("api client is nil")
	}
	cardJSON := buildReplyCardWithTitle(content, "go-stock AI 助手")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := b.apiClient.Im.Message.Patch(ctx,
		larkim.NewPatchMessageReqBuilder().
			MessageId(cardMessageID).
			Body(larkim.NewPatchMessageReqBodyBuilder().
				Content(cardJSON).
				Build()).
			Build())
	if err != nil {
		return fmt.Errorf("patch api error: %w", err)
	}
	if resp == nil || resp.Code != 0 {
		code := -1
		msg := "unknown"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		return fmt.Errorf("patch api failed: code=%d msg=%s", code, msg)
	}
	return nil
}

// shouldReplyAsImage 判断内容是否适合以图片形式回复。
//
// 触发条件（满足任一）：
//   - 内容字符数超过 feishuImageReplyCharThreshold（500字）——长文本图片阅读体验更好
//   - 内容超过 maxFeishuContentBytes（避免分片，提升阅读体验）
//   - 含代码块（```）——飞书卡片代码块样式有限
//   - 含 markdown 表格分隔行（|---|、| --- |、|:---|、|:---:| 等变体）
//     ——飞书卡片表格渲染受限，图片效果更好
//
// feishuImageReplyCharThreshold 触发图片回复的字符数阈值（按 rune 计数）。
// 超过此阈值时将 markdown 渲染为 PNG 图片回复，避免长文本在飞书卡片中阅读体验差。
const feishuImageReplyCharThreshold = 500

func shouldReplyAsImage(content string) bool {
	// 按 rune 计数（中文字符数），超过阈值优先图片回复
	if utf8.RuneCountInString(content) > feishuImageReplyCharThreshold {
		return true
	}
	if len(content) > maxFeishuContentBytes {
		return true
	}
	if strings.Contains(content, "```") {
		return true
	}
	// markdown 表格分隔行的常见变体：|---|、| --- |、|:---|、|:---:|、| ---:|
	for _, sep := range []string{"|---|", "| --- |", "|:---|", "|:---:|", "| ---:|", "|:---: |"} {
		if strings.Contains(content, sep) {
			return true
		}
	}
	return false
}

// replyAsImage 将 markdown 渲染为 PNG 图片并作为图片消息回复。
// 完整链路：markdownToImage → uploadFeishuImage → sendImageReply。
// 任一步骤失败立即返回 error，由调用方决定是否降级。
func (b *FeishuBot) replyAsImage(messageID, markdownContent string) error {
	imageBytes, err := data.MarkdownToImageBytes(markdownContent)
	if err != nil {
		return fmt.Errorf("render markdown to image failed: %w", err)
	}
	logger.SugaredLogger.Infof("feishu bot rendered image: content_len=%d image_size=%d",
		len(markdownContent), len(imageBytes))

	// 上传图片独立超时（30s），避免图片较大时上传卡死
	uploadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	imageKey, err := b.uploadFeishuImage(uploadCtx, imageBytes)
	if err != nil {
		return fmt.Errorf("upload image failed: %w", err)
	}
	logger.SugaredLogger.Infof("feishu bot uploaded image: image_key=%s", imageKey)

	return b.sendImageReply(messageID, imageKey)
}

// uploadFeishuImage 上传 PNG 图片到飞书，返回 image_key。
// imageType 必须为 "message"（用于消息中的图片），图片大小不能超过 10MB。
func (b *FeishuBot) uploadFeishuImage(ctx context.Context, imageBytes []byte) (string, error) {
	if b.apiClient == nil {
		return "", fmt.Errorf("api client is nil")
	}
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("image bytes is empty")
	}

	resp, err := b.apiClient.Im.Image.Create(ctx,
		larkim.NewCreateImageReqBuilder().
			Body(larkim.NewCreateImageReqBodyBuilder().
				ImageType("message").
				Image(bytes.NewReader(imageBytes)).
				Build()).
			Build())
	if err != nil {
		return "", fmt.Errorf("upload image api error: %w", err)
	}
	if resp == nil || resp.Code != 0 || resp.Data == nil || resp.Data.ImageKey == nil {
		code := -1
		msg := "unknown"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		return "", fmt.Errorf("upload image failed: code=%d msg=%s", code, msg)
	}
	return *resp.Data.ImageKey, nil
}

// sendImageReply 以图片消息形式回复（msg_type=image，content 为 {"image_key":"..."}）。
func (b *FeishuBot) sendImageReply(messageID, imageKey string) error {
	if b.apiClient == nil {
		return fmt.Errorf("api client is nil")
	}
	if messageID == "" {
		return fmt.Errorf("message id is empty")
	}
	if imageKey == "" {
		return fmt.Errorf("image key is empty")
	}

	// content JSON：{"image_key":"img_v3_xxxx"}
	content := fmt.Sprintf(`{"image_key":%s}`, mustJSONString(imageKey))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.SugaredLogger.Infof("feishu bot sending image reply: message_id=%s image_key=%s content_len=%d",
		messageID, imageKey, len(content))

	resp, err := b.apiClient.Im.Message.Reply(ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("image").
				Content(content).
				Build()).
			Build())
	if err != nil {
		logger.SugaredLogger.Errorf("feishu bot image reply api error: %v (image_key=%s)",
			err, imageKey)
		return fmt.Errorf("image reply api error: %w", err)
	}
	if resp == nil || resp.Code != 0 {
		code := -1
		msg := "unknown"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		logger.SugaredLogger.Errorf("feishu bot image reply api failed: code=%d msg=%s (image_key=%s)",
			code, msg, imageKey)
		return fmt.Errorf("image reply api failed: code=%d msg=%s", code, msg)
	}
	logger.SugaredLogger.Infof("feishu bot image reply success: message_id=%s image_key=%s",
		messageID, imageKey)
	return nil
}

// sendCardReply 发送单条卡片回复（含超时和详细日志），成功时返回新消息的 message_id
// （用于后续 PATCH 更新该卡片内容；不需要时可忽略返回值）。
func (b *FeishuBot) sendCardReply(messageID, content, title string) (string, error) {
	cardJSON := buildReplyCardWithTitle(content, title)

	// 30 秒超时，防止 API 调用卡死（原代码用 context.Background() 无超时）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.SugaredLogger.Infof("feishu bot sending reply: message_id=%s content_len=%d card_len=%d",
		messageID, len(content), len(cardJSON))

	resp, err := b.apiClient.Im.Message.Reply(ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("interactive").
				Content(cardJSON).
				Build()).
			Build())
	if err != nil {
		logger.SugaredLogger.Errorf("feishu bot reply api error: %v (content_len=%d card_len=%d)",
			err, len(content), len(cardJSON))
		return "", fmt.Errorf("reply api error: %w", err)
	}
	if resp == nil || resp.Code != 0 {
		code := -1
		msg := "unknown"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		logger.SugaredLogger.Errorf("feishu bot reply api failed: code=%d msg=%s (content_len=%d card_len=%d)",
			code, msg, len(content), len(cardJSON))
		return "", fmt.Errorf("reply api failed: code=%d msg=%s", code, msg)
	}
	newMessageID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		newMessageID = *resp.Data.MessageId
	}
	logger.SugaredLogger.Infof("feishu bot reply success: message_id=%s new_message_id=%s content_len=%d",
		messageID, newMessageID, len(content))
	return newMessageID, nil
}

// splitMarkdownContent 将 markdown 内容拆分为每段最多 maxBytes 字节的块。
//
// 拆分策略（优先级从高到低）：
//  1. 段落边界（\n\n）——保持段落在同一块内，可读性最好
//  2. 行边界（\n）——段落过长时退而求其次
//  3. UTF-8 字符边界——单行超长时按字节切，但不会截断多字节字符
//
// 不会在 maxBytes/2 之前拆分（避免拆得太碎），返回的切片均为有效的 UTF-8。
func splitMarkdownContent(content string, maxBytes int) []string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return []string{content}
	}

	var chunks []string
	remaining := content
	for len(remaining) > maxBytes {
		splitAt := findMarkdownSplitPoint(remaining, maxBytes)
		if splitAt <= 0 {
			// 未找到合适的换行边界，按 UTF-8 字符边界硬切
			splitAt = maxBytes
			for splitAt > 0 && !utf8.RuneStart(remaining[splitAt]) {
				splitAt--
			}
			if splitAt == 0 {
				splitAt = 1 // 至少切一个字节，避免死循环
			}
		}
		chunks = append(chunks, remaining[:splitAt])
		remaining = remaining[splitAt:]
	}
	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}
	return chunks
}

// findMarkdownSplitPoint 在 s[:maxBytes] 范围内寻找最佳拆分点（返回拆分位置，含边界字符）。
// 返回 0 表示未找到合适的换行边界。
func findMarkdownSplitPoint(s string, maxBytes int) int {
	if maxBytes > len(s) {
		maxBytes = len(s)
	}
	// 在 maxBytes 的后半段搜索（避免拆分太碎）
	searchStart := maxBytes / 2

	// 1. 优先段落边界 \n\n（返回 \n\n 之后的位置，把双换行留在前一块）
	for i := maxBytes; i > searchStart; i-- {
		if i >= 2 && s[i-1] == '\n' && s[i-2] == '\n' {
			return i
		}
	}

	// 2. 行边界 \n（返回 \n 之后的位置）
	for i := maxBytes; i > searchStart; i-- {
		if s[i-1] == '\n' {
			return i
		}
	}

	return 0
}

// extractMessageText 从飞书事件消息中提取纯文本内容
// text 类型消息 content 格式：{"text":"@_user_1 你好"}，mentions 中 key=@_user_1 对应被 @ 的用户
// 我们去除所有 @_user_N 占位符，保留实际文本
func extractMessageText(message *larkim.EventMessage) string {
	if message == nil || message.Content == nil {
		return ""
	}

	// content 是 JSON 字符串，如 {"text":"hello"} 或 {"text":"@_user_1 hello"}
	var contentObj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*message.Content), &contentObj); err != nil {
		// 解析失败时直接返回原始 content
		return *message.Content
	}

	text := contentObj.Text
	// 去除 @机器人 占位符（@_user_1 / @_user_2 ...）
	text = stripAtMentionPlaceholders(text)

	// 去除多余空白
	text = strings.TrimSpace(text)
	return text
}

// stripAtMentionPlaceholders 移除飞书消息中的 @_user_N 占位符
func stripAtMentionPlaceholders(text string) string {
	// 匹配 @_user_1、@_user_2 等占位符
	// 同时清理占位符后多余的空格
	for i := 1; i <= 20; i++ {
		placeholder := fmt.Sprintf("@_user_%d", i)
		text = strings.ReplaceAll(text, placeholder+" ", "")
		text = strings.ReplaceAll(text, placeholder, "")
	}
	return text
}

// isMentionedToBot 判断消息是否 @ 了机器人。
//
// 检测策略（按可靠性从高到低）：
//  1. mentions 列表中存在 mentioned_type="app" 的项 —— 飞书官方标记，最可靠
//  2. mentions 列表非空（存在任意 @ 项）—— 兜底策略：
//     飞书事件中 mentioned_type 字段可能不填充（实测部分场景为空）。
//     由于机器人通常仅申请 im:message.group_at_msg:readonly 权限（只接收 @机器人
//     的群消息），收到群消息事件本身就意味着被 @。故存在任意 mention 时视为 @bot。
func isMentionedToBot(mentions []*larkim.MentionEvent) bool {
	if len(mentions) == 0 {
		return false
	}
	for _, m := range mentions {
		if m == nil {
			continue
		}
		// 策略 1：mentioned_type == "app"（官方标记）
		if m.MentionedType != nil && *m.MentionedType == "app" {
			return true
		}
	}
	// 策略 2：mentions 非空但无 mentioned_type="app" 标记
	// 飞书事件中 mentioned_type 可能不填充，此时只要有 @ 项就视为 @bot
	logger.SugaredLogger.Warnf("feishu bot: mentions present but no mentioned_type=app found, treating as @bot (len=%d)", len(mentions))
	return true
}

// dumpMentions 格式化 mentions 列表用于调试日志
func dumpMentions(mentions []*larkim.MentionEvent) string {
	if len(mentions) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, m := range mentions {
		if i > 0 {
			sb.WriteString(", ")
		}
		if m == nil {
			sb.WriteString("nil")
			continue
		}
		key := ""
		if m.Key != nil {
			key = *m.Key
		}
		mt := ""
		if m.MentionedType != nil {
			mt = *m.MentionedType
		}
		name := ""
		if m.Name != nil {
			name = *m.Name
		}
		openID := ""
		if m.Id != nil && m.Id.OpenId != nil {
			openID = *m.Id.OpenId
		}
		sb.WriteString(fmt.Sprintf("{key=%s type=%s name=%s open_id=%s}", key, mt, name, openID))
	}
	sb.WriteString("]")
	return sb.String()
}

// buildSessionID 按单聊/群聊 + openId 构造隔离的会话 ID
// 单聊（p2p）: feishu_p2p_<chatId>_<openId>  （chatId 通常与 openId 1:1，但加上更稳）
// 群聊（group/topic_group）: feishu_group_<chatId>_<openId>  （同一群不同用户隔离）
func buildSessionID(chatType, chatId, openId string) string {
	// 兜底：openId 为空时用 "anonymous"
	if openId == "" {
		openId = "anonymous"
	}
	prefix := "feishu"
	switch chatType {
	case "p2p":
		prefix = "feishu_p2p"
	case "group", "topic_group":
		prefix = "feishu_group"
	}
	if chatId == "" {
		return fmt.Sprintf("%s_%s", prefix, openId)
	}
	return fmt.Sprintf("%s_%s_%s", prefix, chatId, openId)
}

// collectAgentReply 消费 ChatWithContext 返回的 chan，拼装最终回复文本
// 协议对齐 processMessageFuture（agent_api.go L934-1056）：
//   - msg.Content != "" → AI 回复片段（最终答案的增量）
//   - msg.ReasoningContent != "" → 思考过程 / 工具调用日志（不回复给用户）
//   - 通道关闭 = 本轮结束
//
// 注意：只收集 Content 字段作为最终回复。ReasoningContent（思考过程、[STEP] 工具日志）
// 不会回复给用户——飞书机器人只回复最终分析结果，不回复思考过程或工具调用信息。
func collectAgentReply(ch chan *schema.Message) string {
	return collectAgentReplyWithProgress(ch, nil)
}

// collectAgentReplyWithProgress 在 collectAgentReply 基础上，把 ReasoningContent 中
// 的 [STEP] 行（阶段切换/工具调用日志，见 agent_api.go 各模式的 safeSend）实时回调给
// onStep（去掉 [STEP] 前缀、逐行回调），用于驱动进度卡片展示。
// onStep 为 nil 时行为与 collectAgentReply 完全一致。
func collectAgentReplyWithProgress(ch chan *schema.Message, onStep func(step string)) string {
	if ch == nil {
		logger.SugaredLogger.Warnf("collectAgentReply: channel is nil")
		return ""
	}
	var contentBuilder strings.Builder
	totalMsgs := 0
	contentMsgs := 0
	reasoningMsgs := 0
	for msg := range ch {
		if msg == nil {
			continue
		}
		totalMsgs++
		if msg.Content != "" {
			contentMsgs++
			contentBuilder.WriteString(msg.Content)
		}
		if msg.ReasoningContent != "" {
			reasoningMsgs++
			if onStep != nil {
				for _, line := range strings.Split(msg.ReasoningContent, "\n") {
					line = strings.TrimSpace(line)
					// 只回调 [STEP] 行（阶段/工具日志）；普通思考流是碎片文本，
					// 高频且杂乱，不适合刷进进度卡片。
					if !strings.HasPrefix(line, "[STEP]") {
						continue
					}
					onStep(strings.TrimSpace(strings.TrimPrefix(line, "[STEP]")))
				}
			}
		}
	}
	reply := contentBuilder.String()
	logger.SugaredLogger.Infof("collectAgentReply: total_msgs=%d content_msgs=%d reasoning_msgs=%d reply_len=%d",
		totalMsgs, contentMsgs, reasoningMsgs, len(reply))
	if contentMsgs == 0 && reasoningMsgs > 0 {
		logger.SugaredLogger.Warnf("collectAgentReply: AI produced only reasoning/tool logs (no final content), " +
			"will not send thinking process to user")
	}
	if totalMsgs == 0 {
		logger.SugaredLogger.Warnf("collectAgentReply: channel closed with no messages at all")
	}
	return reply
}

// buildReplyCard 构造 interactive 卡片 JSON 2.0（默认标题 "go-stock AI 助手"）
// 文档：https://open.feishu.cn/document/feishu-cards/card-json-v2-components/content-components/rich-text
func buildReplyCard(content string) string {
	return buildReplyCardWithTitle(content, "go-stock AI 助手")
}

// buildReplyCardWithTitle 构造带自定义标题的 interactive 卡片 JSON 2.0
// 用于内容拆分时在标题中显示「（1/2）」「（2/2）」等续篇指示
func buildReplyCardWithTitle(content, title string) string {
	card := data.FeishuCard{
		Schema: "2.0",
		Header: &data.FeishuHeader{
			Title: data.FeishuHeaderText{
				Tag:     "plain_text",
				Content: title,
			},
		},
		Body: data.FeishuCardBody{
			Elements: []data.FeishuElement{
				{
					Tag:     "markdown",
					Content: content,
				},
			},
		},
	}
	bs, err := json.Marshal(card)
	if err != nil {
		// 序列化失败时退化为纯文本卡片
		fallback := fmt.Sprintf(`{"schema":"2.0","header":{"title":{"tag":"plain_text","content":%s}},"body":{"elements":[{"tag":"markdown","content":%s}]}}`,
			mustJSONString(title), mustJSONString(content))
		return fallback
	}
	return string(bs)
}

// mustJSONString 将字符串编码为 JSON 字符串字面量（含引号）；用于错误兜底
func mustJSONString(s string) string {
	bs, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(bs)
}

// truncate 截断字符串到指定长度（用于日志）
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
