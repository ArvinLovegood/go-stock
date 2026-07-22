package agent

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
)

type StockAiAgent struct {
	instance     *AgentInstance
	sessionID    string
	aiConfigId   int
	question     string
	thinkingMode bool
}

func NewStockAiAgentApi() *StockAiAgent {
	return &StockAiAgent{}
}

func (receiver StockAiAgent) newStockAiAgent(ctx *context.Context, aiConfigId int, thinkingMode bool, question string, agentMode string) *StockAiAgent {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in newStockAiAgent: %v", r)
		}
	}()

	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		logger.SugaredLogger.Errorf("settingConfig is nil")
		return nil
	}

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !ok {
		logger.SugaredLogger.Errorf("ai config not found for id: %d", aiConfigId)
		return nil
	}
	if aiConfig == nil {
		logger.SugaredLogger.Errorf("aiConfig is nil for id: %d", aiConfigId)
		return nil
	}

	aiConfig.Thinking = thinkingMode
	sessionID := aiConfig.SessionId
	if sessionID == "" {
		sessionID = fmt.Sprintf("ai-config-%d", aiConfig.ID)
	}

	agentInstance := GetStockAiAgent(ctx, *aiConfig, question, agentMode)
	if agentInstance == nil {
		logger.SugaredLogger.Errorf("failed to create agent for config id: %d", aiConfigId)
		return nil
	}

	return &StockAiAgent{
		instance:     agentInstance,
		sessionID:    sessionID,
		aiConfigId:   aiConfigId,
		question:     question,
		thinkingMode: thinkingMode,
	}
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *schema.Message {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false, "")
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
		if len(optsOverride) > 0 && optsOverride[0] != "" {
			sysPromptOverride = optsOverride[0]
		}
		if len(optsOverride) > 1 && optsOverride[1] != "" {
			sessionIDOverride = optsOverride[1]
		}

		stockAiAgent := receiver.newStockAiAgent(&ctx, aiConfigId, thinkingMode, question, agentMode)
		if stockAiAgent == nil {
			logger.SugaredLogger.Errorf("stockAiAgent is nil")
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "❌ AI 配置不存在或无效，请检查 AI 配置",
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
			sysPrompt = data.NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}

		sysPrompt += `

【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率等）——必须调用工具获取最新财报数据
3. 新闻资讯——必须调用工具获取最新新闻
4. 宏观经济数据——必须调用工具获取最新数据
任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息。如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"，绝不能编造数据。`

		sysPrompt += buildAgentTimeContext()

		settingConfig := data.GetSettingConfig()
		aiConfig, _ := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return uint(aiConfigId) == item.ID
		})
		maxInputTokens := 0
		if aiConfig != nil {
			maxInputTokens = aiConfig.EffectiveMaxInputTokens()
		}

		sysPromptTokens := estimateTokens(sysPrompt)
		questionTokens := estimateTokens(question)
		historyBudget := maxInputTokens - sysPromptTokens - questionTokens
		if historyBudget < 0 {
			historyBudget = 0
		}
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
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
		}

		messages = validateAndFixMessages(messages)

		ctx, turnTrace := NewAgentTurnTrace(ctx, question)
		defer func() {
			mode := "react"
			if stockAiAgent.instance != nil {
				mode = string(stockAiAgent.instance.Mode)
			}
			turnTrace.LogSummary(mode)
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

		switch stockAiAgent.instance.Mode {
		case AgentModePlanExecute:
			runPlanExecuteWithFallback(ctx, stockAiAgent, messages, ch, memoryService, historyMessages, sysPrompt, question, aiConfigId, thinkingMode)
		case AgentModeDeepAgents:
			runDeepAgents(ctx, stockAiAgent, messages, ch, memoryService, historyMessages, sysPrompt, question)
		default:
			runReact(ctx, stockAiAgent, messages, ch, memoryService, historyMessages, sysPrompt, question)
		}
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
						errMsg := "❌ Agent 调用失败（token 超限）：输入内容超过模型最大上下文长度限制。请尝试缩短对话历史或使用支持更长上下文的模型。"
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
		srTotalChunks := 0
		srContentChunks := 0
		srReasoningChunks := 0
		srSentContentMsgs := 0
		var srLastFinishReason string
		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
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
					fullResponse.WriteString(msg.Content)
					// 将 Assistant 角色的 Content 直接发送到 ch。
					// 注意：msgFuture 的流 fork 在某些场景下（推理模型+工具调用）可能丢失 Content，
					// 因此不从 processMessageFuture 发送 Content，改由此处直接发送，确保用户能收到回复。
					if msg.Role == schema.Assistant {
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
			}
		}

		logger.SugaredLogger.Infof("runReact sr stats: total=%d content=%d reasoning=%d sent_content_msgs=%d fullResponse_len=%d reasoningFallback_len=%d finish_reason=%s question=%q",
			srTotalChunks, srContentChunks, srReasoningChunks, srSentContentMsgs, fullResponse.Len(), reasoningFallback.Len(), srLastFinishReason, truncate(question, 100))

		// 注意：不再将 reasoning_content 回退为最终回复。reasoning_content 是模型的思考过程，
		// 不是给用户的正式回复。如果模型只产生 reasoning_content 而没有 content，说明模型
		// 可能在思考阶段耗尽了 token 预算（finish_reason=length），此时不应将思考过程
		// 当作回复发送给用户或保存到多轮记忆中。
		if fullResponse.Len() == 0 && reasoningFallback.Len() > 0 {
			logger.SugaredLogger.Warnf("runReact: model produced only reasoning_content (len=%d) with no final content, "+
				"will not use thinking process as reply (finish_reason=%s)", reasoningFallback.Len(), srLastFinishReason)
		}

		if fullResponse.Len() != 0 {
			final := fullResponse.String()
			if memoryService != nil {
				if err := memoryService.AddAssistantMessage(final); err != nil {
					logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
				}
			}
		}
	}()
}

func runPlanExecuteWithFallback(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService, historyMessages []*schema.Message, sysPrompt string, question string, aiConfigId int, thinkingMode bool) {
	defer close(ch)

	planExecuteSuccess := tryPlanExecute(ctx, stockAiAgent, messages, ch, memoryService, historyMessages, sysPrompt, question)

	if !planExecuteSuccess {
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
	defer close(ch)

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

			errMsg := fmt.Sprintf("❌ DeepAgents 执行失败：%v", event.Err)
			if isTokenLimitError(event.Err) {
				errMsg = "❌ DeepAgents 执行失败（token 超限）：输入内容超过模型最大上下文长度限制。请尝试缩短对话历史或使用支持更长上下文的模型。"
			} else if strings.Contains(event.Err.Error(), "exceeds max iterations") || strings.Contains(event.Err.Error(), "exceeds max steps") {
				errMsg = "❌ DeepAgents 达到最大迭代次数限制，任务未完成。请尝试简化问题或切换到快速模式。"
			} else if strings.Contains(event.Err.Error(), "reasoning_content") || strings.Contains(event.Err.Error(), "thinking is enabled") {
				errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型。"
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
				processAdkMessageStream(mv.MessageStream, mv.Role, mv.ToolName, ch, &fullResponse)
			} else if mv.Message != nil {
				processAdkMessage(mv.Message, mv.Role, mv.ToolName, ch, &fullResponse)
			}
		}
	}

	if fullResponse.Len() != 0 {
		final := fullResponse.String()
		if memoryService != nil {
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

			errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", event.Err)
			if isTokenLimitError(event.Err) {
				errMsg = "❌ Agent 调用失败（token 超限）：输入内容超过模型最大上下文长度限制。请尝试缩短对话历史或使用支持更长上下文的模型。"
			} else if strings.Contains(event.Err.Error(), "reasoning_content") || strings.Contains(event.Err.Error(), "thinking is enabled") {
				errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
			} else if strings.Contains(event.Err.Error(), "unmarshal plan error") || strings.Contains(event.Err.Error(), "invalid char") {
				errMsg += "\n\n**可能原因**：计划解析时遇到中文字符编码问题，通常是模型返回的计划内容包含非UTF-8字符。\n\n**解决方案**：请尝试重新提问，或切换到不同的AI模型。"
			}
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: errMsg,
			})
			return true
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			phase := detectPhase(mv.Role, mv.ToolName)
			if phase != "" && phase != lastPhase {
				lastPhase = phase
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
				processAdkMessageStream(mv.MessageStream, mv.Role, mv.ToolName, ch, &fullResponse)
			} else if mv.Message != nil {
				processAdkMessage(mv.Message, mv.Role, mv.ToolName, ch, &fullResponse)
			}
		}
	}

	if fullResponse.Len() != 0 {
		final := fullResponse.String()
		if memoryService != nil {
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
	allTools := getToolsByQuestion(question)
	instance := createReactAgent(ctx, toolableChatModel, allTools, cfg)
	if instance == nil || instance.ReactAgent == nil {
		logger.SugaredLogger.Errorf("createFallbackReactAgent: createReactAgent failed")
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
			defer close(ch)
		}

		sr, err := reactAgent.Stream(ctx, messages, agentOption...)
		if err != nil {
			logger.SugaredLogger.Errorf("stream error: %v", err)
			errMsg := fmt.Sprintf("❌ React Agent 调用失败：%v", err)
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: errMsg,
			})
			return
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
		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				logger.SugaredLogger.Errorf("failed to recv: %v", err)
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: fmt.Sprintf("❌ 接收消息失败：%v", err),
				})
				break
			}
			if msg != nil && msg.Content != "" {
				fullResponse.WriteString(msg.Content)
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: msg.Content,
				})
			}
		}

		if fullResponse.Len() != 0 {
			final := fullResponse.String()
			if memoryService != nil {
				if err := memoryService.AddAssistantMessage(final); err != nil {
					logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
				}
			}
		}
	}()

	wg.Wait()
}

func runPlanExecute(ctx context.Context, stockAiAgent *StockAiAgent, messages []*schema.Message, ch chan *schema.Message, memoryService *ChatMemoryService) {
	defer close(ch)

	adkAgent := stockAiAgent.instance.AdkAgent
	if adkAgent == nil {
		ch <- &schema.Message{
			Role:    schema.Assistant,
			Content: "❌ PlanExecute Agent 实例无效",
		}
		return
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

			isMaxSteps := strings.Contains(event.Err.Error(), "exceeds max iterations") || strings.Contains(event.Err.Error(), "exceeds max steps")
			errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", event.Err)
			if isTokenLimitError(event.Err) {
				errMsg = "❌ Agent 调用失败（token 超限）：输入内容超过模型最大上下文长度限制。请尝试缩短对话历史或使用支持更长上下文的模型。"
			} else if isMaxSteps {
				if fullResponse.Len() > 0 {
					errMsg = "\n---\n⚠️ **分析步骤已达上限，以下为已生成的部分分析结果：**\n\n"
				} else {
					errMsg = "❌ Agent 调用失败：分析步骤超过最大限制。\n\n**解决方案**：\n1. 尝试更精确地描述你的问题，减少模糊性\n2. 切换到支持更长上下文或更强推理能力的模型\n3. 简化查询条件"
				}
			} else if strings.Contains(event.Err.Error(), "reasoning_content") || strings.Contains(event.Err.Error(), "thinking is enabled") {
				errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
			} else if strings.Contains(event.Err.Error(), "unmarshal plan error") || strings.Contains(event.Err.Error(), "invalid char") {
				errMsg += "\n\n**可能原因**：计划解析时遇到中文字符编码问题，通常是模型返回的计划内容包含非UTF-8字符。\n\n**解决方案**：请尝试重新提问，或切换到不同的AI模型。"
			}
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: errMsg,
			})
			if isMaxSteps && fullResponse.Len() > 0 {
				safeSend(ch, &schema.Message{
					Role:    schema.Assistant,
					Content: fullResponse.String(),
				})
			}
			continue
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			phase := detectPhase(mv.Role, mv.ToolName)
			if phase != "" && phase != lastPhase {
				lastPhase = phase
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
				processAdkMessageStream(mv.MessageStream, mv.Role, mv.ToolName, ch, &fullResponse)
			} else if mv.Message != nil {
				processAdkMessage(mv.Message, mv.Role, mv.ToolName, ch, &fullResponse)
			}
		}
	}

	if fullResponse.Len() != 0 && memoryService != nil {
		if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
			logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
		}
	}
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

func processAdkMessageStream(sr *schema.StreamReader[*schema.Message], role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		if msg == nil {
			continue
		}
		handleAdkMessage(msg, role, toolName, ch, fullResponse)
	}
}

func processAdkMessage(msg *schema.Message, role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
	handleAdkMessage(msg, role, toolName, ch, fullResponse)
}

func handleAdkMessage(msg *schema.Message, role schema.RoleType, toolName string, ch chan *schema.Message, fullResponse *strings.Builder) {
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
				safeSend(ch, &schema.Message{
					Role:             schema.Assistant,
					Content:          "",
					ReasoningContent: fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", tc.Function.Name, tc.Function.Arguments),
				})
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
