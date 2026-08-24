package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
)

type StockAiAgent struct {
	instance     *Instance
	sessionID    string
	aiConfigId   int
	question     string
	thinkingMode bool
}

func NewStockAiAgentApi() *StockAiAgent {
	return &StockAiAgent{}
}

func (receiver StockAiAgent) newStockAiAgent(ctx *context.Context, aiConfigId int, thinkingMode bool, question string, agentMode string) (agent *StockAiAgent, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in newStockAiAgent: %v", r)
			agent = nil
			err = fmt.Errorf("Agent 初始化异常(panic): %v", r)
		}
	}()

	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		return nil, errors.New("设置配置加载失败，请检查配置文件")
	}

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !ok {
		return nil, fmt.Errorf("未找到 ID 为 %d 的 AI 配置，请检查 AI 配置", aiConfigId)
	}
	if aiConfig == nil {
		return nil, fmt.Errorf("ID 为 %d 的 AI 配置为空", aiConfigId)
	}

	aiConfig.Thinking = thinkingMode
	// 记忆模式不区分模型配置，所有 AI 配置共享同一份对话上下文。
	// sessionIDOverride（如飞书机器人按 chat+user 区分）仍可在 ChatWithContext 中覆盖。
	sessionID := "default"

	agentInstance, gErr := GetStockAiAgent(ctx, *aiConfig, question, agentMode)
	if gErr != nil {
		return nil, gErr
	}
	if agentInstance == nil {
		return nil, errors.New("创建 Agent 实例失败（未知原因）")
	}

	return &StockAiAgent{
		instance:     agentInstance,
		sessionID:    sessionID,
		aiConfigId:   aiConfigId,
		question:     question,
		thinkingMode: thinkingMode,
	}, nil
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *schema.Message {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false, "")
}

// archiveAnalysisReport 将 AI 分析结果按日期归档到程序所在目录的 memory 目录。
// 目录结构：<exe_dir>/memory/<YYYY-MM-DD>/<HHMMSS>_<问题摘要>.md
// 目录不存在时自动创建。归档失败仅记录日志，不影响主流程。
func archiveAnalysisReport(question, response string, mode Mode) {
	if strings.TrimSpace(response) == "" {
		return
	}

	rootDir := deepAgentRootDir()
	now := time.Now()
	dateDir := filepath.Join(rootDir, "memory", now.Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		logger.SugaredLogger.Errorf("归档分析报告: 创建目录失败: %v (path=%s)", err, dateDir)
		return
	}

	summary := sanitizeReportFilename(question, 30)
	fileName := fmt.Sprintf("%s_%s.md", now.Format("150405"), summary)
	reportPath := filepath.Join(dateDir, fileName)

	content := fmt.Sprintf("# 分析报告\n\n- **时间**: %s\n- **模式**: %s\n- **问题**: %s\n\n---\n\n## AI 回复\n\n%s\n",
		now.Format("2006-01-02 15:04:05"), mode, question, response)

	if err := os.WriteFile(reportPath, []byte(content), 0o644); err != nil {
		logger.SugaredLogger.Errorf("归档分析报告: 写入文件失败: %v (path=%s)", err, reportPath)
		return
	}
	logger.SugaredLogger.Infof("分析报告已归档: %s (rootDir=%s)", reportPath, rootDir)

	// 异步入库长期记忆向量库（切片+embedding+写入）。
	// 失败仅记日志，不影响归档主流程；reportPath 用于检索时追溯全文。
	AddMemory(question, response, mode, reportPath, CurrentUserKey(""))
}

// sanitizeReportFilename 将问题文本转换为安全的文件名片段：
// 移除换行和文件名非法字符，截断到指定长度。
func sanitizeReportFilename(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return '_'
		}
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	s = strings.TrimSpace(string(runes))
	if s == "" {
		s = "untitled"
	}
	return s
}

func (receiver StockAiAgent) ChatWithContext(ctx context.Context, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, optsOverride ...string) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in ChatWithContext: %v", r)
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: fmt.Sprintf("❌ 内部错误: %v", r),
				}
				close(ch)
			}
		}()

		var sessionIDOverride string
		var sysPromptOverride string
		var resumeContextOverride string
		if len(optsOverride) > 0 && optsOverride[0] != "" {
			sysPromptOverride = optsOverride[0]
		}
		if len(optsOverride) > 1 && optsOverride[1] != "" {
			sessionIDOverride = optsOverride[1]
		}
		if len(optsOverride) > 2 && optsOverride[2] != "" {
			resumeContextOverride = optsOverride[2]
		}

		stockAiAgent, agentErr := receiver.newStockAiAgent(&ctx, aiConfigId, thinkingMode, question, agentMode)
		if agentErr != nil || stockAiAgent == nil {
			// 直接透传错误原因，避免固定文案掩盖真实问题（如正则 panic、配置缺失、模型创建失败等）。
			// newStockAiAgent 已通过 defer recover 把 panic 转为 error，此处不会再次 panic。
			reason := "未知原因"
			if agentErr != nil {
				reason = agentErr.Error()
			}
			logger.SugaredLogger.Errorf("newStockAiAgent failed: %v", agentErr)
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: fmt.Sprintf("❌ Agent 初始化失败：%s", reason),
			}
			close(ch)
			return
		}

		if sessionIDOverride != "" {
			stockAiAgent.sessionID = sessionIDOverride
		}

		var memoryService *ChatMemoryService
		var historyMessages []*schema.Message
		if memoryMode && stockAiAgent.sessionID != "" {
			memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)
			var err error
			historyMessages, err = memoryService.GetHistoryMessages()
			if err != nil {
				logger.SugaredLogger.Errorf("failed to get history messages: %v", err)
				historyMessages = nil
			}
		}

		sysPrompt := ""
		if sysPromptOverride != "" {
			sysPrompt = sysPromptOverride
		} else if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = `你现在扮演一位拥有20年实战经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析等多种策略。你擅长结合宏观经济、行业周期和企业基本面进行全方位、精准的多维分析，尤其对A股、港股、美股市场有深刻理解，始终秉持"风险控制第一"的原则，善于用通俗易懂的方式传授投资智慧。`
		} else {
			sysPrompt = getCachedPromptTemplate(*sysPromptId) // 走 5 分钟 TTL 缓存，详见 sysprompt_cache.go
		}

		// 静态规则段（强制规则 + 合规边界）— 进程级缓存，详见 sysprompt_cache.go
		sysPrompt += staticRulesHead
		sysPrompt += staticRulesCompliance

		sysPrompt += buildAgentTimeContext()
		// 注入自进化层：SOUL.md（进化规则）+ MEMORY.md（长期记忆）+ 最近 LEARNINGS + 历史相关经验。
		// 对标 Hermes Agent 动态 Prompt：运行时按需组装记忆与规则到系统提示词，跨会话生效。
		// 历史相关经验优先走向量检索（按当前问题语义召回 Top-K），向量库未就绪时降级到文件名扫描。
		// 文件全部缺失时返回空字符串，不影响主流程。
		sysPrompt += buildSelfEvolutionPrompt(deepAgentRootDir(), question)
		// 注入项目级指令文件（.go-stock.md / AGENTS.md，递归向上查找）
		// 与用户偏好（<exe_dir>/memory/user_profile.md），均可能为空。
		sysPrompt += loadProjectInstructions("")
		sysPrompt += loadUserProfile()

		// 静态规则段（错误恢复 + 并行引导 + 检索规范）— 进程级缓存，详见 sysprompt_cache.go
		sysPrompt += staticRulesTail
		sysPrompt += staticRulesParallel
		sysPrompt += staticRulesRetrieval

		// 任务规划模板：仅在 PlanExecute 模式下注入，引导模型输出结构化任务清单
		if stockAiAgent.instance != nil && stockAiAgent.instance.Mode == PlanExecute {
			sysPrompt += staticRulesPlanExecute
		}

		// 思考模式引导：开启 thinking 时引导模型分步推理
		if thinkingMode {
			sysPrompt += staticRulesThinking
		}

		// 会话状态跟踪：从用户问题中提取股票代码，注入当前分析标的
		sysPrompt += buildSessionContext(question)
		if resumeContextOverride != "" {
			sysPrompt += resumeContextOverride
		}

		settingConfig := data.GetSettingConfig()
		aiConfig, _ := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return uint(aiConfigId) == item.ID
		})
		maxInputTokens := 0
		if aiConfig != nil {
			cw := resolveContextWindow(*aiConfig)
			out := resolveOutputMaxTokens(*aiConfig, cw)
			maxInputTokens = getMaxInputTokens(cw, out)
		}

		sysPromptTokens := estimateTokens(sysPrompt)
		questionTokens := estimateTokens(question)
		// 工具 schema 由 eino 注入到每次模型请求，需从历史预算中扣除，
		// 否则 DeepAgents/React 大量工具时会把历史塞满导致上下文超限。
		toolTokens := 0
		if stockAiAgent.instance != nil {
			toolTokens = estimateToolsTokens(stockAiAgent.instance.Tools)
		}
		historyBudget := getChatHistoryBudget(maxInputTokens, sysPromptTokens, questionTokens, toolTokens)
		if historyBudget < 0 {
			historyBudget = 0
		}
		logger.SugaredLogger.Infof("token 预算: maxInput=%d sysPrompt=%d question=%d tools=%d historyBudget=%d",
			maxInputTokens, sysPromptTokens, questionTokens, toolTokens, historyBudget)
		if len(historyMessages) > 0 && historyBudget > 0 {
			historyMessages = trimHistoryMessages(historyMessages, historyBudget)
		}

		var messages []*schema.Message
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: sysPrompt,
		})
		messages = append(messages, historyMessages...)
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: question,
		})

		if memoryService != nil {
			// 注意：用户消息不再在此处提前保存，改为在各 Agent 执行成功后与助手消息一起保存，
			// 避免 Agent 失败时产生孤立的 user 消息（无对应 assistant 回复）。
		}

		messages = validateAndFixMessages(messages)

		ctx, turnTrace := NewAgentTurnTrace(ctx, question)
		mode := React
		if stockAiAgent.instance != nil {
			mode = stockAiAgent.instance.Mode
		}
		budget := estimateAgentRunBudget(question, mode)
		logger.SugaredLogger.Infof("运行预算: mode=%s duration=%s maxToolCalls=%d", mode, budget.MaxDuration, budget.MaxToolCalls)
		ctx, runner := NewAgentRunner(ctx, question, stockAiAgent.sessionID, budget, deepAgentRootDir())
		runner.Start(mode)
		run := runner.Run()
		run.SetAIConfigID(aiConfigId)
		defer func() {
			runner.Finish()
			turnTrace.LogSummary(string(mode))
			logger.SugaredLogger.Infof("agent run completed: run_id=%s mode=%s state=%s tools=%d elapsed=%s",
				run.ID, mode, run.State(), run.ToolCalls(), run.Elapsed().Round(time.Millisecond))
		}()

		// 注入实际模型名与系统/用户提示词，供推荐工具（CreateAiRecommendStocks 等）在
		// InvokableRun 中提取，确保保存的推荐记录关联真实的模型与提示词，而非 AI 自填值。
		actualModelName := ""
		if aiConfig != nil {
			actualModelName = aiConfig.ModelName
		}
		ctx = tools.WithAgentMeta(ctx, tools.AgentMeta{
			ModelName:    actualModelName,
			SystemPrompt: sysPrompt,
			UserPrompt:   question,
		})
		// 注入前端进度反馈 channel：工具调用前后通过 ReasoningContent 发送预告与结果摘要
		ctx = WithProgressChannel(ctx, ch)
		// 注入摘要模型：trimToolResult 对超长工具结果调用 LLM 生成摘要
		if stockAiAgent.instance != nil && stockAiAgent.instance.ChatModel != nil {
			ctx = WithSummaryModel(ctx, stockAiAgent.instance.ChatModel)
		}

		runner.Execute(AgentExecutionInput{
			StockAgent:      stockAiAgent,
			Messages:        messages,
			Channel:         ch,
			MemoryService:   memoryService,
			HistoryMessages: historyMessages,
			SystemPrompt:    sysPrompt,
			Question:        question,
			AIConfigID:      aiConfigId,
			ThinkingMode:    thinkingMode,
		})
	}()

	return ch
}

func runReact(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string) {
	reactAgent := stockAiAgent.instance.ReactAgent
	if reactAgent == nil {
		ch <- &schema.Message{
			Role:    schema.Assistant,
			Content: "❌ React Agent 实例无效",
		}
		close(ch)
		return
	}

	msgFutureOpt, msgFuture := react.WithMessageFuture()
	opts := agent.GetComposeOptions(msgFutureOpt)

	agentOption := []agent.AgentOption{
		agent.WithComposeOptions(opts...),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in processMessageFuture: %v", r)
			}
			wg.Done()
		}()
		processMessageFuture(msgFuture, ch)
	}()

	// 确保 processMessageFuture goroutine 完全结束后再关闭 ch，避免最终回答内容
	// 的 safeSend 与 close(ch) 产生竞态导致内容丢失（快速模式无最终结果的问题根因）。
	defer func() {
		wg.Wait()
		sendTurnStats(ctx, ch) // 在 close 前发送 token 统计
		close(ch)
	}()

	func() {
		sr, err := reactAgent.Stream(ctx, messages, agentOption...)
		if err != nil {
			logger.SugaredLogger.Errorf("stream error: %v", err)

			if isTokenLimitError(err) && len(historyMessages) > 0 {
				logger.SugaredLogger.Infof("token limit exceeded, retrying with reduced history")
				halfLen := len(historyMessages) / 2
				if halfLen == 0 {
					halfLen = 1
				}
				historyMessages = historyMessages[halfLen:]
				messages = []*schema.Message{}
				messages = append(messages, &schema.Message{
					Role:    schema.System,
					Content: sysPrompt,
				})
				messages = append(messages, historyMessages...)
				messages = append(messages, &schema.Message{
					Role:    schema.User,
					Content: question,
				})

				sr, err = reactAgent.Stream(ctx, messages, agentOption...)
				if err != nil {
					if isTokenLimitError(err) {
						logger.SugaredLogger.Infof("still over token limit after trimming, retrying without history")
						messages = []*schema.Message{}
						messages = append(messages, &schema.Message{
							Role:    schema.System,
							Content: sysPrompt,
						})
						messages = append(messages, &schema.Message{
							Role:    schema.User,
							Content: question,
						})
						sr, err = reactAgent.Stream(ctx, messages, agentOption...)
					}
					if err != nil {
						// 直接展示原始错误，避免固定文案掩盖真实原因（如 max_tokens 超限、限流、鉴权等）
						errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", err)
						ch <- &schema.Message{
							Role:    schema.Assistant,
							Content: errMsg,
						}
						return
					}
				}
			} else {
				errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", err)
				if strings.Contains(err.Error(), "exceeds max iterations") {
					errMsg += "\n\n**可能原因**：模型在执行过程中进行了过多轮工具调用仍无法收敛，可能陷入了循环。\n\n**解决方案**：\n1. 尝试更精确地描述你的问题，减少模糊性\n2. 切换到支持更长上下文或更强推理能力的模型\n3. 简化查询条件"
				} else if strings.Contains(err.Error(), "reasoning_content") || strings.Contains(err.Error(), "thinking is enabled") {
					errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
				}
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: errMsg,
				}
				return
			}
		}
		if sr == nil {
			logger.SugaredLogger.Errorf("stream result is nil")
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "❌ 流式响应无效",
			}
			return
		}
		defer func() {
			sr.Close()
		}()

		var fullResponse strings.Builder
		var reasoningFallback strings.Builder
		streamSuccess := false
		srTotalChunks := 0
		srContentChunks := 0
		srReasoningChunks := 0
		srSentContentMsgs := 0
		var srLastFinishReason string
		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					streamSuccess = true
					break
				}
				logger.SugaredLogger.Errorf("failed to recv: %v", err)
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: fmt.Sprintf("❌ 接收消息失败：%v", err),
				}
				break
			}
			if msg != nil {
				srTotalChunks++
				if msg.Content != "" {
					srContentChunks++
					// 仅累积 Assistant 角色的 Content 到 fullResponse。
					// React 模式流中会包含 Tool 角色的工具返回内容（可能很长），若一并写入
					// fullResponse 会导致保存到 chat_memory 的 assistant 消息混杂工具结果，
					// 下一轮读取历史时 AI 看到混乱内容而非纯净的对话回复。
					if msg.Role == schema.Assistant {
						fullResponse.WriteString(msg.Content)
						// 将 Assistant 角色的 Content 直接发送到 ch。
						// 注意：msgFuture 的流 fork 在某些场景下（推理模型+工具调用）可能丢失 Content，
						// 因此不从 processMessageFuture 发送 Content，改由此处直接发送，确保用户能收到回复。
						srSentContentMsgs++
						safeSend(ch, &schema.Message{
							Role:    schema.Assistant,
							Content: msg.Content,
						})
					}
				}
				if msg.ReasoningContent != "" {
					srReasoningChunks++
					reasoningFallback.WriteString(msg.ReasoningContent)
				}
				if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason != "" {
					srLastFinishReason = msg.ResponseMeta.FinishReason
				}
				// 累计 token 用量到 turnTrace
				if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.AccumulateUsage(msg.ResponseMeta.Usage)
					}
				}
			}
		}

		logger.SugaredLogger.Infof("runReact sr stats: total=%d content=%d reasoning=%d sent_content_msgs=%d fullResponse_len=%d reasoningFallback_len=%d stream_success=%v finish_reason=%s question=%q",
			srTotalChunks, srContentChunks, srReasoningChunks, srSentContentMsgs, fullResponse.Len(), reasoningFallback.Len(), streamSuccess, srLastFinishReason, truncate(question, 100))

		// 注意：不再将 reasoning_content 回退为最终回复。reasoning_content 是模型的思考过程，
		// 不是给用户的正式回复。如果模型只产生 reasoning_content 而没有 content，说明模型
		// 可能在思考阶段耗尽了 token 预算（finish_reason=length），此时不应将思考过程
		// 当作回复发送给用户或保存到多轮记忆中。
		if fullResponse.Len() == 0 && reasoningFallback.Len() > 0 {
			logger.SugaredLogger.Warnf("runReact: model produced only reasoning_content (len=%d) with no final content, "+
				"will not use thinking process as reply (finish_reason=%s)", reasoningFallback.Len(), srLastFinishReason)
		}

		// 保存条件：只要 fullResponse 有内容就保存，不依赖 streamSuccess。
		// 之前的实现 `fullResponse.Len() != 0 && streamSuccess` 会在流被中断（用户主动中断、
		// 网络抖动、超时等）时完全不保存对话，导致下一轮 AI 读取历史为空、回复"找不到之前的分析内容"。
		// 即使流被中断，已生成的部分回复对下一轮上下文仍有价值，应予以保留。
		// streamSuccess 仅用于决定是否将 reasoning_content 作为兜底回复（见上方分支）。
		if fullResponse.Len() != 0 {
			final := fullResponse.String()
			SendFinancialFactCheck(ctx, ch, final)
			archiveAnalysisReport(question, final, React)
			triggerPostTaskReflection(question, final, React, deepAgentRootDir())
			triggerPositiveReflection(question, final, React, deepAgentRootDir())
			if memoryService != nil {
				if err := memoryService.AddUserMessage(question); err != nil {
					logger.SugaredLogger.Errorf("failed to save user message: %v", err)
				}
				if err := memoryService.AddAssistantMessage(final); err != nil {
					logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
				}
			}
		}
	}()
}

func runPlanExecuteWithFallback(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string, aiConfigId int, thinkingMode bool) {
	defer func() {
		sendTurnStats(ctx, ch)
		close(ch)
	}()

	planExecuteSuccess := tryPlanExecute(ctx, stockAiAgent, messages, ch, memoryService, historyMessages, sysPrompt, question)

	if !planExecuteSuccess {
		// context 已超时/取消时跳过降级：React 会复用同一失效 context 立即失败，
		// 提前终止并说明原因，避免"⚠️ 切换到工具分析模式"预告了不会发生的续跑。
		if ctx.Err() != nil {
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: fmt.Sprintf("❌ 规划模式失败且本轮运行已结束，跳过降级重试：%v", ctx.Err()),
			})
			return
		}
		logger.SugaredLogger.Warnf("PlanExecute 模式失败，降级到 React 模式")

		safeSend(ch, &schema.Message{
			Role:             schema.Assistant,
			Content:          "",
			ReasoningContent: "[FALLBACK]⚠️ 检测到编码问题，切换到工具分析模式...\n",
		})

		reactAgent := createFallbackReactAgent(ctx, stockAiAgent, thinkingMode)
		if reactAgent != nil {
			runReactWithAgent(ctx, reactAgent, messages, ch, memoryService, historyMessages, sysPrompt, question, false, stockAiAgent)
		} else {
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: "❌ 无法创建备用分析引擎，请稍后重试",
			})
		}
	}
}

// runDeepAgents 运行 DeepAgents 模式。
// DeepAgents 返回 adk.ResumableAgent，与 PlanExecute 走相同的 adk.NewRunner + iter.Next()
// 事件流机制，因此复用 processAdkMessage/processAdkMessageStream/handleAdkMessage 事件处理逻辑。
//
// 与 tryPlanExecute 的差异：
//   - 无 plan JSON 编码错误降级（DeepAgents 不产生 plan JSON）
//   - 阶段检测不同：write_todos→规划、task→委派、其他工具→执行
//   - 错误处理：记录日志并提示用户，不自动降级到 React（用户显式选择了 DeepAgents）
func runDeepAgents(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string) {
	defer func() {
		sendTurnStats(ctx, ch)
		close(ch)
	}()

	adkAgent := stockAiAgent.instance.AdkAgent
	if adkAgent == nil {
		safeSend(ch, &schema.Message{
			Role:    schema.Assistant,
			Content: "❌ DeepAgents Agent 初始化失败，请检查 AI 配置后重试",
		})
		return
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: adkAgent,
	})

	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: "[STEP]🧠 DeepAgents 模式启动，正在规划任务并调用工具分析...\n",
	})

	iter := runner.Run(ctx, messages)

	var fullResponse strings.Builder
	lastPhase := ""

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}

		if event.Err != nil {
			logger.SugaredLogger.Errorf("deepagents event error: %v", event.Err)

			// 直接展示原始错误，避免固定文案掩盖真实原因（如 max_tokens 超限、限流、鉴权等）。
			errMsg := fmt.Sprintf("❌ DeepAgents 执行失败：%v", event.Err)
			if strings.Contains(event.Err.Error(), "exceeds max iterations") || strings.Contains(event.Err.Error(), "exceeds max steps") {
				errMsg += "\n\n💡 已达到最大迭代次数限制，任务未完成。请尝试简化问题或切换到快速模式。"
			} else if strings.Contains(event.Err.Error(), "reasoning_content") || strings.Contains(event.Err.Error(), "thinking is enabled") {
				errMsg += "\n\n💡 可能是当前模型开启了 thinking/reasoning 模式，但该模式与工具调用不兼容。请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型。"
			}
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: errMsg,
			})
			break
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			phase := detectDeepAgentsPhase(mv.Role, mv.ToolName)
			if phase != "" && phase != lastPhase {
				lastPhase = phase
				SetAgentRunPhase(ctx, phase)
				var stepMsg string
				switch phase {
				case "planning":
					stepMsg = "[STEP]📋 正在拆解任务，制定 TODO 计划...\n"
				case "delegating":
					stepMsg = "[STEP]🔗 正在委派子任务到子 Agent（上下文隔离）...\n"
				case "executing":
					stepMsg = "[STEP]⚡ 正在执行工具调用...\n"
				}
				if stepMsg != "" {
					safeSend(ch, &schema.Message{
						Role:             schema.Assistant,
						Content:          "",
						ReasoningContent: stepMsg,
					})
				}
			}

			if mv.IsStreaming && mv.MessageStream != nil {
				processAdkMessageStream(ctx, mv.MessageStream, mv.Role, mv.ToolName, ch, &fullResponse)
			} else if mv.Message != nil {
				processAdkMessage(ctx, mv.Message, mv.Role, mv.ToolName, ch, &fullResponse)
			}
		}
	}

	if fullResponse.Len() != 0 {
		final := fullResponse.String()
		SendFinancialFactCheck(ctx, ch, final)
		archiveAnalysisReport(question, final, DeepAgents)
		triggerPostTaskReflection(question, final, DeepAgents, deepAgentRootDir())
		triggerPositiveReflection(question, final, DeepAgents, deepAgentRootDir())
		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
			if err := memoryService.AddAssistantMessage(final); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}
	}
}

// detectDeepAgentsPhase 根据 DeepAgents 的工具调用判断当前阶段。
// DeepAgents 内置工具：write_todos（规划）、task（子 Agent 委派）、其他自定义工具（执行）。
func detectDeepAgentsPhase(role schema.RoleType, toolName string) string {
	switch toolName {
	case "write_todos":
		return "planning"
	case "task":
		return "delegating"
	}
	if role == schema.Tool {
		return "executing"
	}
	if role == schema.Assistant {
		return "executing"
	}
	return ""
}

func tryPlanExecute(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string) bool {
	adkAgent := stockAiAgent.instance.AdkAgent
	if adkAgent == nil {
		return false
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: adkAgent,
	})

	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: "[STEP]🧠 规划模式启动，正在分析问题并制定执行计划...\n",
	})

	iter := runner.Run(ctx, messages)

	var fullResponse strings.Builder
	stepCount := 0
	lastPhase := ""

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}

		if event.Err != nil {
			logger.SugaredLogger.Errorf("agent event error: %v", event.Err)

			// context 已超时或被取消：降级到 React 只会复用同一失效 context 立即失败
			// （表现为误导性的"❌ React Agent 调用失败：context deadline exceeded"），
			// 因此直接终止本轮并保留已生成的部分内容（break 落到末尾记忆保存逻辑）。
			if ctx.Err() != nil || isContextCanceledError(event.Err) {
				if ctx.Err() == context.Canceled {
					safeSend(ch, &schema.Message{
						Role:    schema.Assistant,
						Content: "⏹️ 已停止本轮分析。",
					})
				} else {
					errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", event.Err)
					errMsg += "\n\n💡 本轮运行预算已耗尽（默认单轮 10 分钟、最多 100 次工具调用）。已保留部分分析结果，可简化问题后重新提问，或切换到快速模式。"
					safeSend(ch, &schema.Message{
						Role:    schema.Assistant,
						Content: errMsg,
					})
				}
				break
			}

			if strings.Contains(event.Err.Error(), "unmarshal plan error") ||
				strings.Contains(event.Err.Error(), "invalid char") ||
				strings.Contains(event.Err.Error(), "UTF-8") {
				logger.SugaredLogger.Warnf("检测到编码错误，触发降级机制")
				return false
			}

			if strings.Contains(event.Err.Error(), "no tool call") {
				logger.SugaredLogger.Warnf("检测到模型未返回工具调用，降级到 React+工具 模式")
				safeSend(ch, &schema.Message{
					Role:             schema.Assistant,
					Content:          "",
					ReasoningContent: "[STEP]⚠️ 规划步骤工具调用失败，正在切换到工具分析模式继续...\n",
				})
				fallbackWithReactAgent(ctx, stockAiAgent, ch, messages, memoryService, historyMessages, sysPrompt, question, &fullResponse)
				return true
			}

			isMaxSteps := strings.Contains(event.Err.Error(), "exceeds max iterations") || strings.Contains(event.Err.Error(), "exceeds max steps")
			isNodeError := strings.Contains(event.Err.Error(), "NodeRunError")
			isCriticalTerminate := isMaxSteps || isNodeError

			if isCriticalTerminate {
				logger.SugaredLogger.Warnf("检测到模型终止任务(原因为: %s)，降级到 React+工具 模式", event.Err.Error())
				safeSend(ch, &schema.Message{
					Role:             schema.Assistant,
					Content:          "",
					ReasoningContent: "[STEP]⚠️ 模型中途终止任务，正在切换到工具分析模式继续...\n",
				})
				fallbackWithReactAgent(ctx, stockAiAgent, ch, messages, memoryService, historyMessages, sysPrompt, question, &fullResponse)
				return true
			}

			// 直接展示原始错误，避免固定文案掩盖真实原因（如 max_tokens 超限、限流、鉴权等）。
			errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", event.Err)
			if strings.Contains(event.Err.Error(), "reasoning_content") || strings.Contains(event.Err.Error(), "thinking is enabled") {
				errMsg += "\n\n💡 可能是当前模型开启了 thinking/reasoning 模式，但该模式与工具调用不兼容。请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
			}
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: errMsg,
			})
			// 注意：此处使用 break 而非 return true，以便落到函数末尾的记忆保存逻辑。
			// 若 return true 会跳过 fullResponse 已累积内容的保存，导致下一轮 AI 读取历史为空。
			// 与 runDeepAgents 的错误处理（break + 末尾保存）保持一致。
			break
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			phase := detectPhase(mv.Role, mv.ToolName)
			if phase != "" && phase != lastPhase {
				lastPhase = phase
				SetAgentRunPhase(ctx, phase)
				if phase == "planning" {
					safeSend(ch, &schema.Message{
						Role:             schema.Assistant,
						Content:          "",
						ReasoningContent: "[STEP]📋 正在制定执行计划...\n",
					})
				} else if phase == "executing" {
					stepCount++
					safeSend(ch, &schema.Message{
						Role:             schema.Assistant,
						Content:          "",
						ReasoningContent: fmt.Sprintf("[STEP]⚡ 执行步骤 %d...\n", stepCount),
					})
				} else if phase == "replanning" {
					safeSend(ch, &schema.Message{
						Role:             schema.Assistant,
						Content:          "",
						ReasoningContent: "[STEP]🔄 评估进度，调整计划...\n",
					})
				}
			}

			if mv.IsStreaming && mv.MessageStream != nil {
				processAdkMessageStream(ctx, mv.MessageStream, mv.Role, mv.ToolName, ch, &fullResponse)
			} else if mv.Message != nil {
				processAdkMessage(ctx, mv.Message, mv.Role, mv.ToolName, ch, &fullResponse)
			}
		}
	}

	if fullResponse.Len() != 0 {
		final := fullResponse.String()
		SendFinancialFactCheck(ctx, ch, final)
		archiveAnalysisReport(question, final, PlanExecute)
		triggerPostTaskReflection(question, final, PlanExecute, deepAgentRootDir())
		triggerPositiveReflection(question, final, PlanExecute, deepAgentRootDir())
		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
			if err := memoryService.AddAssistantMessage(final); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}
	}

	return true // 成功完成
}

func createFallbackReactAgent(ctx context.Context, stockAiAgent *StockAiAgent, thinkingMode bool) *react.Agent {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		logger.SugaredLogger.Errorf("createFallbackReactAgent: settingConfig is nil")
		return nil
	}

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(stockAiAgent.aiConfigId) == item.ID
	})
	if !ok || aiConfig == nil {
		logger.SugaredLogger.Errorf("createFallbackReactAgent: ai config not found for id: %d", stockAiAgent.aiConfigId)
		return nil
	}

	cfg := *aiConfig
	cfg.Thinking = thinkingMode

	toolableChatModel, err := createChatModel(ctx, cfg)
	if err != nil {
		logger.SugaredLogger.Errorf("createFallbackReactAgent: createChatModel failed: %v", err)
		return nil
	}

	question := stockAiAgent.question
	if question == "" {
		question = "继续分析"
	}
	allTools := getToolsByQuestion(question, false)
	instance, instErr := createReactAgent(ctx, toolableChatModel, allTools, cfg)
	if instErr != nil || instance == nil || instance.ReactAgent == nil {
		logger.SugaredLogger.Errorf("createFallbackReactAgent: createReactAgent failed: %v", instErr)
		return nil
	}
	return instance.ReactAgent
}

func buildFallbackMessages(messages []*schema.Message, partial *strings.Builder) []*schema.Message {
	fallbackMessages := make([]*schema.Message, 0, len(messages)+2)
	fallbackMessages = append(fallbackMessages, messages...)

	if partial != nil && partial.Len() > 0 {
		fallbackMessages = append(fallbackMessages, &schema.Message{
			Role:    schema.Assistant,
			Content: "（规划模式已完成的部分分析，数据可能不完整或未经验证）\n" + partial.String(),
		})
	}

	fallbackMessages = append(fallbackMessages, &schema.Message{
		Role: schema.User,
		Content: "规划模式未能完成。请通过工具重新查询所需数据后继续回答。" +
			"涉及股价、涨跌幅、财务指标等具体数字必须先调用工具获取，不得编造或使用训练数据。" +
			"若工具返回 status=empty 或 status=error，请明确告知用户未能获取数据。",
	})
	return validateAndFixMessages(fallbackMessages)
}

// isContextCanceledError 判断错误是否源于 context 超时或取消。
// eino 的 GraphRunError/NodeRunError 不保证实现 Unwrap，errors.Is 之外需辅以字符串匹配。
func isContextCanceledError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "context has been canceled")
}

func fallbackWithReactAgent(ctx context.Context, stockAiAgent *StockAiAgent, ch chan *schema.Message, messages []*schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string, partial *strings.Builder) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in fallbackWithReactAgent: %v", r)
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: fmt.Sprintf("❌ 工具分析兜底失败: %v", r),
			})
		}
	}()

	// context 已超时/取消时跳过降级：React 会复用同一失效 context，
	// Stream 立即以 GraphRunError 失败，向用户展示误导性的"React Agent 调用失败"。
	if ctx.Err() != nil {
		safeSend(ch, &schema.Message{
			Role:    schema.Assistant,
			Content: fmt.Sprintf("❌ 本轮运行已结束，跳过降级重试：%v", ctx.Err()),
		})
		return
	}

	reactAgent := createFallbackReactAgent(ctx, stockAiAgent, stockAiAgent.thinkingMode)
	if reactAgent == nil {
		safeSend(ch, &schema.Message{
			Role:    schema.Assistant,
			Content: "❌ 工具分析兜底失败：无法创建 React Agent，请稍后重试",
		})
		return
	}

	fallbackMessages := buildFallbackMessages(messages, partial)
	runReactWithAgent(ctx, reactAgent, fallbackMessages, ch, memoryService, historyMessages, sysPrompt, question, false, stockAiAgent)
}

func runReactWithAgent(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string, closeChannel bool, stockAiAgent *StockAiAgent) {
	// 类似于原来的 runReact 函数，但使用指定的 agent
	if reactAgent == nil {
		safeSend(ch, &schema.Message{
			Role:    schema.Assistant,
			Content: "❌ React Agent 实例无效",
		})
		return
	}

	msgFutureOpt, msgFuture := react.WithMessageFuture()
	opts := agent.GetComposeOptions(msgFutureOpt)

	agentOption := []agent.AgentOption{
		agent.WithComposeOptions(opts...),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in processMessageFuture: %v", r)
			}
			wg.Done()
		}()
		processMessageFuture(msgFuture, ch)
	}()

	func() {
		if closeChannel {
			defer func() {
				sendTurnStats(ctx, ch)
				close(ch)
			}()
		}

		sr, err := reactAgent.Stream(ctx, messages, agentOption...)
		if err != nil {
			logger.SugaredLogger.Errorf("stream error: %v", err)

			// token 超限重试：与 runReact 保持一致，先裁剪历史半数重试，再无历史重试。
			// 降级路径下 messages 可能包含规划阶段的部分回复（buildFallbackMessages 注入），
			// 重试时丢弃这些附加内容，仅保留 system+history+question，优先保证能得到回复。
			if isTokenLimitError(err) && len(historyMessages) > 0 {
				halfLen := len(historyMessages) / 2
				if halfLen == 0 {
					halfLen = 1
				}
				historyMessages = historyMessages[halfLen:]
				messages = []*schema.Message{
					{Role: schema.System, Content: sysPrompt},
				}
				messages = append(messages, historyMessages...)
				messages = append(messages, &schema.Message{Role: schema.User, Content: question})
				messages = validateAndFixMessages(messages)
				logger.SugaredLogger.Infof("token limit in fallback react, retrying with reduced history (len=%d)", len(historyMessages))
				sr, err = reactAgent.Stream(ctx, messages, agentOption...)
			}
			if err != nil && isTokenLimitError(err) {
				messages = []*schema.Message{
					{Role: schema.System, Content: sysPrompt},
					{Role: schema.User, Content: question},
				}
				logger.SugaredLogger.Infof("still over token limit in fallback react, retrying without history")
				sr, err = reactAgent.Stream(ctx, messages, agentOption...)
			}
			if err != nil {
				// 直接展示原始错误，避免固定文案掩盖真实原因（如 max_tokens 超限、限流、鉴权等）
				errMsg := fmt.Sprintf("❌ React Agent 调用失败：%v", err)
				// context 已超时/取消：说明是运行预算耗尽而非 React 本身故障，避免误导排查方向
				if ctx.Err() != nil || isContextCanceledError(err) {
					errMsg += "\n\n💡 本轮运行预算已耗尽（默认单轮 10 分钟、最多 100 次工具调用），降级重试未能完成。请简化问题后重新提问，或切换到快速模式。"
				}
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: errMsg,
				})
				return
			}
		}
		if sr == nil {
			logger.SugaredLogger.Errorf("stream result is nil")
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: "❌ 流式响应无效",
			})
			return
		}
		defer func() {
			sr.Close()
		}()

		var fullResponse strings.Builder
		streamSuccess := false
		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					streamSuccess = true
					break
				}
				logger.SugaredLogger.Errorf("failed to recv: %v", err)
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: fmt.Sprintf("❌ 接收消息失败：%v", err),
				})
				break
			}
			// 仅处理 Assistant 角色的 Content：避免把 Tool 角色的工具返回内容写入 fullResponse
			// 或当作 AI 回复发给前端（详见 runReact 中相同修复的说明）。
			if msg != nil && msg.Content != "" && msg.Role == schema.Assistant {
				fullResponse.WriteString(msg.Content)
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: msg.Content,
				})
			}
		}

		logger.SugaredLogger.Infof("runReactWithAgent sr stats: fullResponse_len=%d stream_success=%v question=%q",
			fullResponse.Len(), streamSuccess, truncate(question, 100))

		// 保存条件不依赖 streamSuccess：流被中断时已生成的部分回复仍应写入 chat_memory，
		// 否则降级路径下也会出现"下一轮找不到之前分析内容"的问题。
		if fullResponse.Len() != 0 {
			final := fullResponse.String()
			SendFinancialFactCheck(ctx, ch, final)
			archiveAnalysisReport(question, final, React)
			triggerPostTaskReflection(question, final, React, deepAgentRootDir())
			triggerPositiveReflection(question, final, React, deepAgentRootDir())
			if memoryService != nil {
				if err := memoryService.AddUserMessage(question); err != nil {
					logger.SugaredLogger.Errorf("failed to save user message: %v", err)
				}
				if err := memoryService.AddAssistantMessage(final); err != nil {
					logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
				}
			}
		}
	}()

	wg.Wait()
}

func detectPhase(role schema.RoleType, toolName string) string {
	if toolName == "plan" {
		return "planning"
	}
	if toolName == "respond" {
		return "responding"
	}
	if role == schema.Tool {
		return "executing"
	}
	if role == schema.Assistant {
		return "executing"
	}
	return ""
}

func processMessageFuture(msgFuture react.MessageFuture, ch chan *schema.Message) {
	if msgFuture == nil || ch == nil {
		logger.SugaredLogger.Errorf("msgFuture or ch is nil")
		return
	}

	iter := msgFuture.GetMessageStreams()
	if iter == nil {
		logger.SugaredLogger.Errorf("message stream iterator is nil")
		return
	}

	for {
		sr, ok, err := iter.Next()
		if err != nil {
			logger.SugaredLogger.Errorf("failed to get next message stream: %v", err)
			return
		}
		if !ok {
			break
		}
		if sr == nil {
			continue
		}

		var reasoningBuilder strings.Builder
		var contentBuilder strings.Builder
		toolCallsMap := make(map[int]*strings.Builder)
		toolCallNames := make(map[int]string)
		var toolResult *struct {
			name    string
			content string
		}

		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				logger.SugaredLogger.Errorf("failed to recv from message stream: %v", err)
				return
			}
			if msg == nil {
				continue
			}

			if msg.ReasoningContent != "" {
				reasoningBuilder.WriteString(msg.ReasoningContent)
				safeSend(ch, &schema.Message{
					Role:             schema.Assistant,
					Content:          "",
					ReasoningContent: msg.ReasoningContent,
				})
			}

			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					idx := 0
					if tc.Index != nil {
						idx = *tc.Index
					}
					if _, exists := toolCallsMap[idx]; !exists {
						toolCallsMap[idx] = &strings.Builder{}
					}
					if tc.Function.Name != "" {
						toolCallNames[idx] = tc.Function.Name
					}
					toolCallsMap[idx].WriteString(tc.Function.Arguments)
				}
			}

			if msg.Role == schema.Tool && msg.Content != "" {
				toolResult = &struct {
					name    string
					content string
				}{
					name:    msg.ToolName,
					content: msg.Content,
				}
			}

			if msg.Role == schema.Assistant && msg.Content != "" {
				// 仅用于 [FinalAnswer] 调试日志，不发送到 ch。
				// Content 由 runReact 的 sr 循环直接发送到 ch（msgFuture 的流 fork
				// 在推理模型+工具调用场景下可能丢失 Content，因此不依赖此路径）。
				contentBuilder.WriteString(msg.Content)
			}
		}

		if reasoningBuilder.Len() > 0 {
			fmt.Printf("\n[Reasoning]\n%s\n", reasoningBuilder.String())
		}

		if len(toolCallsMap) > 0 {
			for idx := 0; idx < len(toolCallsMap); idx++ {
				if builder, exists := toolCallsMap[idx]; exists {
					name := toolCallNames[idx]
					fmt.Printf("\n[ToolCall] %s(%s)\n", name, builder.String())
					safeSend(ch, &schema.Message{
						Role:             schema.Assistant,
						Content:          "",
						ReasoningContent: fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", name, builder.String()),
					})
				}
			}
		}

		if toolResult != nil {
			safeSend(ch, &schema.Message{
				Role:             schema.Assistant,
				Content:          "",
				ReasoningContent: fmt.Sprintf("[STEP]✅ %s 返回结果（%d字）\n", toolResult.name, len(toolResult.content)),
			})
			fmt.Printf("\n[ToolResult] %s:\n%s\n", toolResult.name, truncateString(toolResult.content, 300))
		}

		if contentBuilder.Len() > 0 && len(toolCallsMap) == 0 {
			fmt.Printf("\n[FinalAnswer]\n%s\n", contentBuilder.String())
		}

		logger.SugaredLogger.Infof("processMessageFuture stream stats: reasoning_len=%d content_len=%d tool_calls=%d tool_result=%v",
			reasoningBuilder.Len(), contentBuilder.Len(), len(toolCallsMap), toolResult != nil)
	}
}

func processAdkMessageStream(ctx context.Context, sr *schema.StreamReader[*schema.Message], role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		if msg == nil {
			continue
		}
		handleAdkMessage(ctx, msg, role, toolName, ch, fullResponse)
	}
}

func processAdkMessage(ctx context.Context, msg *schema.Message, role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
	handleAdkMessage(ctx, msg, role, toolName, ch, fullResponse)
}

func handleAdkMessage(ctx context.Context, msg *schema.Message, role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
	// 累计 token 用量到 turnTrace（DeepAgents/PlanExecute 路径）
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		if trace := AgentTurnTraceFromContext(ctx); trace != nil {
			trace.AccumulateUsage(msg.ResponseMeta.Usage)
		}
	}
	if msg.ReasoningContent != "" {
		safeSend(ch, &schema.Message{
			Role:             schema.Assistant,
			Content:          "",
			ReasoningContent: msg.ReasoningContent,
		})
	}

	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name != "" {
				// write_todos 工具调用：格式化为易读的任务清单，不显示原始 JSON
				if tc.Function.Name == "write_todos" {
					if formatted := formatWriteTodosArgs(tc.Function.Arguments); formatted != "" {
						safeSend(ch, &schema.Message{
							Role:             schema.Assistant,
							Content:          "",
							ReasoningContent: fmt.Sprintf("[STEP]📝 %s\n", formatted),
						})
					}
					continue
				}
				safeSend(ch, &schema.Message{
					Role:             schema.Assistant,
					Content:          "",
					ReasoningContent: fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", tc.Function.Name, tc.Function.Arguments),
				})
				// 技能激活特别提示：当 Agent 调用 skill 工具时，解析技能名并高亮提示
				if tc.Function.Name == "skill" {
					if skillName := extractSkillNameFromArgs(tc.Function.Arguments); skillName != "" {
						safeSend(ch, &schema.Message{
							Role:             schema.Assistant,
							Content:          "",
							ReasoningContent: fmt.Sprintf("[STEP]🎯 已激活技能：%s\n", skillName),
						})
					}
				}
			}
		}
	}

	if msg.Role == schema.Tool && msg.Content != "" {
		resultPreview := msg.Content
		if len(resultPreview) > 500 {
			resultPreview = resultPreview[:500] + "...(结果已截断)"
		}
		safeSend(ch, &schema.Message{
			Role:             schema.Assistant,
			Content:          "",
			ReasoningContent: fmt.Sprintf("[STEP]✅ %s 返回结果（%d字）\n", toolName, len(msg.Content)),
		})
		fmt.Printf("\n[ToolResult] %s:\n%s\n", toolName, truncateString(msg.Content, 300))
	}

	if msg.Content != "" && (role == schema.Assistant || msg.Role == schema.Assistant) {
		cleaned := stripPlanJSON(msg.Content)
		if cleaned != "" {
			fullResponse.WriteString(cleaned)
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: cleaned,
			})
		}
	}
}

// extractSkillNameFromArgs 从 skill 工具调用的 arguments JSON 中提取技能名。
// arguments 格式示例：{"skill":"技术分析助手"}
// 解析失败时返回空字符串。
func extractSkillNameFromArgs(args string) string {
	if args == "" {
		return ""
	}
	var parsed struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Skill)
}

// formatWriteTodosArgs 将 write_todos 工具调用的 arguments JSON 格式化为易读的多行任务清单。
// arguments 格式示例：{"todos":[{"activeForm":"...","content":"...","status":"completed"},...]}
// status 取值：completed / in_progress / pending
// 解析失败时返回空字符串（调用方应跳过发送）。
func formatWriteTodosArgs(args string) string {
	if args == "" {
		return ""
	}
	var parsed struct {
		Todos []struct {
			ActiveForm string `json:"activeForm"`
			Content    string `json:"content"`
			Status     string `json:"status"`
		} `json:"todos"`
	}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return ""
	}
	if len(parsed.Todos) == 0 {
		return ""
	}

	completed, inProgress, pending := 0, 0, 0
	for _, t := range parsed.Todos {
		switch t.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgress++
		default:
			pending++
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("任务清单更新（%d项：%d完成/%d进行中/%d待执行）\n",
		len(parsed.Todos), completed, inProgress, pending))
	for _, t := range parsed.Todos {
		desc := t.ActiveForm
		if desc == "" {
			desc = t.Content
		}
		switch t.Status {
		case "completed":
			b.WriteString(fmt.Sprintf("  ✅ %s\n", desc))
		case "in_progress":
			b.WriteString(fmt.Sprintf("  🔄 %s（进行中）\n", desc))
		default:
			b.WriteString(fmt.Sprintf("  ⏳ %s\n", desc))
		}
	}
	return b.String()
}

func stripPlanJSON(content string) string {
	if !strings.Contains(content, `"steps"`) {
		return content
	}
	var b strings.Builder
	b.Grow(len(content))
	inCodeBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}
		if inCodeBlock && strings.Contains(trimmed, `"steps"`) && strings.Contains(trimmed, "[") {
			continue
		}
		if !inCodeBlock && (strings.HasPrefix(trimmed, `{"steps":`) || strings.HasPrefix(trimmed, `{"steps" :`)) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	result := strings.TrimRight(b.String(), "\n ")
	if result == "" {
		return ""
	}
	lines := strings.Split(result, "\n")
	cleaned := make([]string, 0, len(lines))
	skipEmpty := true
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			if !skipEmpty {
				cleaned = append(cleaned, l)
			}
			skipEmpty = true
			continue
		}
		skipEmpty = false
		cleaned = append(cleaned, l)
	}
	return strings.Join(cleaned, "\n")
}

func formatMarkdown(content string) string {
	if content == "" {
		return content
	}

	inCodeBlock := false
	lines := strings.Split(content, "\n")
	var result []string

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if !inCodeBlock {
				result = append(result, trimmed)
				continue
			}
		}

		if inCodeBlock {
			result = append(result, line)
			continue
		}

		if trimmed != line && trimmed != "" {
			line = trimmed
		}

		if i > 0 && isBlockElement(trimmed) {
			prev := ""
			if len(result) > 0 {
				prev = result[len(result)-1]
			}
			if prev != "" && !isBlockElement(strings.TrimLeft(prev, " \t")) {
				result = append(result, "")
			}
		}

		line = splitInlineHeading(line)

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

var headingRe = regexp.MustCompile(`(#{1,6}\s+\S)`)

func splitInlineHeading(line string) string {
	idx := headingRe.FindStringIndex(line)
	if idx == nil {
		return line
	}
	if idx[0] == 0 {
		return line
	}
	prefix := line[:idx[0]]
	if strings.TrimSpace(prefix) == "" {
		return line
	}
	return prefix + "\n\n" + line[idx[0]:]
}

func isBlockElement(line string) bool {
	if len(line) == 0 {
		return false
	}
	if line[0] == '#' {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if strings.HasPrefix(line, "```") {
		return true
	}
	if strings.HasPrefix(line, "> ") {
		return true
	}
	if len(line) >= 2 && (line[0] >= '1' && line[0] <= '9') && line[1] == '.' {
		return true
	}
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "***") || strings.HasPrefix(line, "___") {
		return true
	}
	if strings.HasPrefix(line, "|") {
		return true
	}
	return false
}

func safeSend(ch chan *schema.Message, msg *schema.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic when sending to channel: %v", r)
		}
	}()
	select {
	case ch <- msg:
	default:
		logger.SugaredLogger.Warnf("channel full, message dropped")
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// validateAndFixMessages 验证并修复消息序列，确保兼容各类模型API的消息格式要求。
// 处理：1)移除空消息 2)去除连续重复User消息 3)修复孤立的Tool消息 4)确保消息序列合法
func validateAndFixMessages(messages []*schema.Message) []*schema.Message {
	if len(messages) <= 1 {
		return messages
	}

	// 1. 移除空消息
	var cleaned []*schema.Message
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Content == "" && len(msg.ToolCalls) == 0 && msg.ToolCallID == "" && msg.ReasoningContent == "" {
			continue
		}
		cleaned = append(cleaned, msg)
	}
	if len(cleaned) <= 1 {
		return cleaned
	}

	// 2. 合并连续User消息（保留最后一条），兼容要求严格 user/assistant 交替的模型
	var deduped []*schema.Message
	for _, msg := range cleaned {
		if msg.Role == schema.User && len(deduped) > 0 && deduped[len(deduped)-1].Role == schema.User {
			deduped[len(deduped)-1] = msg
			continue
		}
		deduped = append(deduped, msg)
	}

	// 3. 移除开头孤立的Tool消息（没有对应Assistant ToolCall）
	var result []*schema.Message
	hasAssistantWithTools := false
	for _, msg := range deduped {
		if msg.Role == schema.Tool && !hasAssistantWithTools {
			logger.SugaredLogger.Warnf("validateAndFixMessages: 跳过开头孤立的Tool消息 (toolCallID=%s)", msg.ToolCallID)
			continue
		}
		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			hasAssistantWithTools = true
		}
		result = append(result, msg)
	}

	if len(result) == 0 {
		return messages
	}
	return result
}

func fallbackWithOpenAI(ctx context.Context, ch chan *schema.Message, messages []*schema.Message, aiConfigId int, fullResponse *strings.Builder) {
	// 已废弃：无工具兜底会导致编造数据。保留函数签名避免外部引用编译失败，内部转日志。
	logger.SugaredLogger.Warnf("fallbackWithOpenAI is deprecated and should not be called (aiConfigId=%d)", aiConfigId)
	safeSend(ch, &schema.Message{
		Role:    schema.Assistant,
		Content: "❌ 分析引擎异常，请重试。系统已禁用无工具兜底以避免返回未验证数据。",
	})
	_ = messages
	_ = fullResponse
}
