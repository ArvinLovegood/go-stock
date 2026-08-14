package agent

import (
	"context"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"strings"
	"unicode"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	// chineseCharsPerToken 中文 token 估算系数。
	// 实测 GPT-4o (o200k_base) / Claude 3.x / DeepSeek / Qwen 对常见汉字约 1 字 = 1 token。
	// 取 1.3 而非 1.0 是为保留安全边际（避免低估导致上下文超限），
	// 相比原值 1.5 减少约 13% 的低估，提升 historyBudget 利用率。
	chineseCharsPerToken = 1.3
	englishCharsPerToken = 4.0
	// 以下预留用于 estimateMessagesTokens 之外的系统/工具/ReAct 开销，过大会过早压缩用户上下文。
	// PlanExecute 中「已完成步骤」另见 agent.go 的 compressExecutedStepResult。
	toolsTokenReserve  = 8000
	skillPromptReserve = 4000
	reactLoopReserve   = 8000
	safetyMargin       = 0.85
)

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	chineseCount := 0
	nonChineseCount := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		} else {
			nonChineseCount++
		}
	}
	tokens := float64(chineseCount)/chineseCharsPerToken + float64(nonChineseCount)/englishCharsPerToken
	return int(tokens) + 1
}

func estimateMessagesTokens(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		total += estimateTokens(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += estimateTokens(tc.Function.Name)
			total += estimateTokens(tc.Function.Arguments)
		}
		total += 4
	}
	return total
}

// defaultContextWindow 兜底上下文窗口，用于配置和内置表均未提供时的安全默认。
const defaultContextWindow = 64000

// resolveContextWindow 解析模型上下文窗口（输入+输出总容量），向后兼容旧配置。
//
// 优先级：
//  1. aiConfig.ContextWindow（由 FetchAiModelInfo 从模型 API 获取，或用户手动填写）
//  2. 内置模型表（GetBuiltinModelContextWindow）
//  3. aiConfig.MaxTokens（旧配置兜底：旧版本 MaxTokens 混装了上下文窗口值）
//  4. defaultContextWindow（64000，安全默认）
func resolveContextWindow(aiConfig data.AIConfig) int {
	if aiConfig.ContextWindow > 0 {
		return aiConfig.ContextWindow
	}
	if cw := GetBuiltinModelContextWindow(aiConfig.ModelName); cw > 0 {
		return cw
	}
	if aiConfig.MaxTokens > 0 {
		// 旧配置向后兼容：MaxTokens 可能来自旧的 FetchAiModelInfo（优先取
		// max_context_length）或内置表，作为上下文窗口兜底。
		return aiConfig.MaxTokens
	}
	return defaultContextWindow
}

// resolveOutputMaxTokens 解析模型输出上限（max_tokens API 参数），确保不超过上下文窗口。
//
// 优先级：
//  1. aiConfig.MaxTokens（若 > 0 且 < contextWindow，视为输出上限）
//  2. 内置模型表（GetBuiltinModelMaxOutput）
//  3. min(8000, contextWindow/2)（安全默认，留至少一半给输入）
func resolveOutputMaxTokens(aiConfig data.AIConfig, contextWindow int) int {
	if aiConfig.MaxTokens > 0 && aiConfig.MaxTokens < contextWindow {
		return aiConfig.MaxTokens
	}
	if mo := GetBuiltinModelMaxOutput(aiConfig.ModelName); mo > 0 && mo < contextWindow {
		return mo
	}
	defaultOut := 8000
	if half := contextWindow / 2; half < defaultOut {
		return half
	}
	return defaultOut
}

// getMaxInputTokens 计算对话历史可用的输入 token 预算（React 模式 MessageRewriter）。
//
// contextWindow 为模型上下文窗口，outputMaxTokens 为输出上限。
// API 强制约束 input + max_tokens ≤ context_window，故先扣减输出预留，
// 再乘以安全系数并扣除工具/技能/ReAct 循环的固定预留。
func getMaxInputTokens(contextWindow, outputMaxTokens int) int {
	if contextWindow <= 0 {
		contextWindow = defaultContextWindow
	}
	availableInput := contextWindow - outputMaxTokens
	if availableInput < 4000 {
		availableInput = 4000
	}
	result := int(float64(availableInput)*safetyMargin) -
		(toolsTokenReserve + skillPromptReserve + reactLoopReserve)
	if result < 4000 {
		result = 4000
	}
	return result
}

// estimateToolsTokens 估算一批工具 schema 的输入 token 占用。
// 工具 schema 由 eino 注入到每次模型请求（DeepAgents 的主 Agent 与 general-purpose
// 子 Agent 都会携带整套工具），必须在上下文预算中扣除，否则系统会高估可用空间，
// 导致 DeepAgents/React 长工具链在真正溢出前得不到正确裁剪。
func estimateToolsTokens(ts []tool.BaseTool) int {
	if len(ts) == 0 {
		return 0
	}
	total := 0
	for _, t := range ts {
		if t == nil {
			continue
		}
		info, err := t.Info(nil)
		if err != nil || info == nil {
			continue
		}
		if data, err := sonic.Marshal(info); err == nil {
			total += estimateTokens(string(data))
		} else {
			// Marshal 失败时的保守估算：工具名 + 描述 + 参数 schema 大致开销
			total += estimateTokens(info.Name) + estimateTokens(info.Desc) + 50
		}
		total += 16 // 每个工具的 JSON 包装/分隔开销
	}
	return total
}

// estimateToolInfosTokens 估算已解析的工具 schema（[]*schema.ToolInfo）的 token 占用。
// 用于 summarization 中间件的自定义 TokenCounter：比 eino 默认估算（增量消息按
// ~4 字符/token）对中文更准确，避免低估导致摘要触发过晚、上下文溢出。
func estimateToolInfosTokens(infos []*schema.ToolInfo) int {
	if len(infos) == 0 {
		return 0
	}
	total := 0
	for _, info := range infos {
		if info == nil {
			continue
		}
		if data, err := sonic.Marshal(info); err == nil {
			total += estimateTokens(string(data))
		} else {
			// Marshal 失败时的保守估算：工具名 + 描述 + 参数 schema 大致开销
			total += estimateTokens(info.Name) + estimateTokens(info.Desc) + 50
		}
		total += 16 // 每个工具的 JSON 包装/分隔开销
	}
	return total
}

// getChatHistoryBudget 计算对话历史可用的输入 token 预算。
// 相比 getMaxInputTokens（固定预留 toolsTokenReserve），这里显式按工具 schema 的
// 真实占用扣除，避免 DeepAgents 大量工具时预留不足导致上下文超限。
// maxInputTokens 为 getMaxInputTokens 的结果，其中已含 toolsTokenReserve，
// 因此这里加回该预留、改为扣减 estimateToolsTokens 的真实估算值。
func getChatHistoryBudget(maxInputTokens, sysPromptTokens, questionTokens, toolTokens int) int {
	budget := maxInputTokens + toolsTokenReserve - sysPromptTokens - questionTokens - toolTokens
	if budget < 0 {
		budget = 0
	}
	return budget
}

func trimHistoryMessages(historyMessages []*schema.Message, maxTokens int) []*schema.Message {
	if len(historyMessages) == 0 {
		return historyMessages
	}

	currentTokens := estimateMessagesTokens(historyMessages)
	if currentTokens <= maxTokens {
		return historyMessages
	}

	halfLen := len(historyMessages) / 2
	if halfLen > 0 {
		trimmed := historyMessages[halfLen:]
		trimmedTokens := estimateMessagesTokens(trimmed)
		if trimmedTokens <= maxTokens {
			return trimmed
		}
	}

	result := []*schema.Message{}
	tokenSum := 0
	for i := len(historyMessages) - 1; i >= 0; i-- {
		msgTokens := estimateTokens(historyMessages[i].Content) + 4
		if tokenSum+msgTokens > maxTokens {
			break
		}
		tokenSum += msgTokens
		result = append([]*schema.Message{historyMessages[i]}, result...)
	}

	if len(result) == 0 && len(historyMessages) > 0 {
		lastMsg := historyMessages[len(historyMessages)-1]
		content := lastMsg.Content
		maxChars := maxTokens * 2
		if len(content) > maxChars {
			lastMsg = &schema.Message{
				Role:    lastMsg.Role,
				Content: content[len(content)-maxChars:] + "\n...(更早的内容已省略)",
			}
		}
		result = []*schema.Message{lastMsg}
	}

	return result
}

func trimToolResult(ctx context.Context, content string, maxTokens int) string {
	if content == "" {
		return content
	}
	metaLines, body := splitToolMetadataPrefix(content)
	metaPrefix := joinToolMetadataAndBody(metaLines, "")
	metaTokens := estimateTokens(metaPrefix)
	bodyBudgetTokens := maxTokens - metaTokens
	if bodyBudgetTokens < 200 {
		bodyBudgetTokens = 200
	}
	if estimateTokens(body) <= bodyBudgetTokens {
		return content
	}
	// 超长工具结果（>4000 token）优先尝试 LLM 摘要，保留关键指标和数字；
	// 失败或未配置摘要模型时降级到 smartContentCompress 规则压缩。
	if estimateTokens(body) > 4000 {
		if summarized := llmSummarizeToolResult(ctx, body); summarized != "" {
			return joinToolMetadataAndBody(metaLines, summarized)
		}
	}
	maxBodyBytes := int(float64(bodyBudgetTokens) * englishCharsPerToken * 0.8)
	if maxBodyBytes < 600 {
		maxBodyBytes = 600
	}
	compressedBody := smartContentCompress(body, maxBodyBytes)
	if !strings.Contains(compressedBody, "已截断") && !strings.Contains(compressedBody, "省略") {
		compressedBody += "\n\n...(内容过长，已截断显示)"
	}
	return joinToolMetadataAndBody(metaLines, compressedBody)
}

func compressMessages(messages []*schema.Message, maxTokens int) []*schema.Message {
	if maxTokens <= 0 || len(messages) == 0 {
		return messages
	}
	compressed := compressNonSystemMessages(messages, maxTokens)
	return validateAndFixMessages(compressed)
}

func validateAndFixMessageSequence(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	var fixed []*schema.Message
	for i, msg := range messages {
		if msg.Role == schema.Tool {
			hasParent := false
			for j := i - 1; j >= 0; j-- {
				if fixed[j].Role == schema.Assistant {
					for _, tc := range fixed[j].ToolCalls {
						if msg.ToolCallID == tc.ID {
							hasParent = true
							break
						}
					}
				}
				if hasParent {
					break
				}
			}
			if !hasParent {
				logger.SugaredLogger.Warnf("MessageRewriter: 移除孤立的Tool消息 (toolCallID=%s)", msg.ToolCallID)
				continue
			}
		}
		fixed = append(fixed, msg)
	}

	if len(fixed) == 0 {
		return messages
	}

	for i := 0; i < len(fixed); i++ {
		if fixed[i].Role == schema.System {
			if i+1 < len(fixed) && fixed[i+1].Role != schema.User {
				logger.SugaredLogger.Warnf("MessageRewriter: System后非User消息(role=%s), 插入占位User", fixed[i+1].Role)
				placeholder := &schema.Message{
					Role:    schema.User,
					Content: "继续",
				}
				fixed = append(fixed[:i+1], append([]*schema.Message{placeholder}, fixed[i+1:]...)...)
				break
			}
		}
	}

	return fixed
}

func compressNonSystemMessages(messages []*schema.Message, maxTokens int) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	currentTokens := estimateMessagesTokens(messages)
	if currentTokens <= maxTokens {
		return messages
	}

	var userMsg *schema.Message
	var afterUser []*schema.Message
	for i, msg := range messages {
		if msg.Role == schema.User {
			userMsg = msg
			afterUser = messages[i+1:]
			break
		}
	}

	if userMsg == nil {
		return dropOldestMessages(messages, maxTokens)
	}

	userTokens := estimateTokens(userMsg.Content) + 4
	afterUserBudget := maxTokens - userTokens
	if afterUserBudget < 0 {
		afterUserBudget = 0
	}

	var groups []toolGroup

	i := 0
	for i < len(afterUser) {
		msg := afterUser[i]
		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			groupTokens := estimateTokens(msg.Content) + 4
			for _, tc := range msg.ToolCalls {
				groupTokens += estimateTokens(tc.Function.Name) + estimateTokens(tc.Function.Arguments)
			}
			var groupMsgs []*schema.Message
			groupMsgs = append(groupMsgs, msg)

			j := i + 1
			for j < len(afterUser) {
				if afterUser[j].Role == schema.Tool {
					matched := false
					for _, tc := range msg.ToolCalls {
						if afterUser[j].ToolCallID == tc.ID {
							matched = true
							break
						}
					}
					if matched {
						groupMsgs = append(groupMsgs, afterUser[j])
						groupTokens += estimateTokens(afterUser[j].Content) + 4
						j++
						continue
					}
				}
				break
			}

			if j < len(afterUser) && afterUser[j].Role == schema.Assistant && afterUser[j].Content != "" && len(afterUser[j].ToolCalls) == 0 {
				groupMsgs = append(groupMsgs, afterUser[j])
				groupTokens += estimateTokens(afterUser[j].Content) + 4
				j++
			}

			groups = append(groups, toolGroup{messages: groupMsgs, tokens: groupTokens})
			i = j
		} else {
			msgTokens := estimateTokens(msg.Content) + 4
			groups = append(groups, toolGroup{messages: []*schema.Message{msg}, tokens: msgTokens})
			i++
		}
	}

	totalAfterUser := 0
	for _, g := range groups {
		totalAfterUser += g.tokens
	}

	if totalAfterUser <= afterUserBudget {
		result := make([]*schema.Message, 0, len(afterUser)+1)
		result = append(result, userMsg)
		result = append(result, afterUser...)
		return result
	}

	overBudget := totalAfterUser - afterUserBudget
	return progressiveCompressGroups(userMsg, groups, afterUserBudget, overBudget)
}

func progressiveCompressGroups(userMsg *schema.Message, groups []toolGroup, budget int, overBudget int) []*schema.Message {
	n := len(groups)
	if n == 0 {
		result := []*schema.Message{userMsg}
		return result
	}

	groupTokens := make([]int, n)
	for i, g := range groups {
		groupTokens[i] = g.tokens
	}

	totalTokens := 0
	for _, t := range groupTokens {
		totalTokens += t
	}

	compressed := make([]bool, n)
	compressedGroupTokens := make([]int, n)
	copy(compressedGroupTokens, groupTokens)

	const (
		recentWindow = 2
		minToolBytes = 600
	)

	round := 0
	for totalTokens > budget {
		round++
		improved := false

		for i := 0; i < n; i++ {
			if totalTokens <= budget {
				break
			}

			distanceFromEnd := n - 1 - i
			isRecent := distanceFromEnd < recentWindow

			var maxBytes int
			var threshold int
			if isRecent {
				maxBytes = 8000
				threshold = 2000
			} else if distanceFromEnd < recentWindow+2 {
				maxBytes = 4000
				threshold = 1000
			} else {
				maxBytes = 2000
				threshold = 300
			}

			if compressed[i] && compressedGroupTokens[i] <= threshold {
				continue
			}

			oldTokens := compressedGroupTokens[i]
			newTokens := oldTokens
			for _, msg := range groups[i].messages {
				if msg.Role == schema.Tool && estimateTokens(msg.Content) > threshold {
					currentBytes := len([]byte(msg.Content))
					targetBytes := maxBytes
					if !isRecent && round > 1 {
						targetBytes = minToolBytes
					}

					if currentBytes > targetBytes {
						compressedContent := smartContentCompress(msg.Content, targetBytes)
						newEstimate := oldTokens - estimateTokens(msg.Content) + estimateTokens(compressedContent)
						if newEstimate < oldTokens {
							newTokens = newEstimate
						}
					}
				}
			}

			if newTokens >= oldTokens {
				compressed[i] = true
				continue
			}

			saved := oldTokens - newTokens
			totalTokens -= saved
			compressedGroupTokens[i] = newTokens
			compressed[i] = true
			improved = true
		}

		if !improved {
			startIdx := 0
			for startIdx < n {
				partial := 0
				for k := startIdx; k < n; k++ {
					partial += compressedGroupTokens[k]
				}
				if partial <= budget {
					break
				}
				startIdx++
			}
			if startIdx >= n {
				startIdx = n - 1
			}

			result := []*schema.Message{userMsg}
			for k := startIdx; k < n; k++ {
				result = append(result, groups[k].messages...)
			}
			return result
		}
	}

	result := []*schema.Message{userMsg}
	for i, g := range groups {
		if compressedGroupTokens[i] < groupTokens[i] {
			rebuilt := rebuildCompressedGroup(g, compressedGroupTokens[i])
			result = append(result, rebuilt...)
		} else {
			result = append(result, g.messages...)
		}
	}
	return result
}

func rebuildCompressedGroup(g toolGroup, targetTokens int) []*schema.Message {
	result := make([]*schema.Message, len(g.messages))
	for i, msg := range g.messages {
		if msg.Role == schema.Tool {
			metaLines, body := splitToolMetadataPrefix(msg.Content)
			msgTokens := estimateTokens(body)
			if msgTokens > 500 {
				currentBytes := len([]byte(body))
				ratio := float64(targetTokens) / float64(g.tokens)
				targetBytes := int(float64(currentBytes) * ratio)
				if targetBytes < 600 {
					targetBytes = 600
				}
				if targetBytes < currentBytes {
					compressed := smartContentCompress(body, targetBytes)
					cp := *msg
					cp.Content = joinToolMetadataAndBody(metaLines, compressed+"\n\n[以上数据已智能压缩，保留了关键指标和结论]")
					result[i] = &cp
					continue
				}
			}
		}
		result[i] = msg
	}
	return result
}

type toolGroup struct {
	messages []*schema.Message
	tokens   int
}

func dropOldestMessages(messages []*schema.Message, maxTokens int) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	totalTokens := estimateMessagesTokens(messages)
	if totalTokens <= maxTokens {
		return messages
	}

	toolCallIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == schema.Assistant {
			for _, tc := range msg.ToolCalls {
				toolCallIDs[tc.ID] = true
			}
		}
	}

	kept := make([]*schema.Message, 0, len(messages))
	tokenSum := 0
	started := false

	for i, msg := range messages {
		if !started {
			if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
				assistantTokens := estimateTokens(msg.Content)
				for _, tc := range msg.ToolCalls {
					assistantTokens += estimateTokens(tc.Function.Name) + estimateTokens(tc.Function.Arguments)
				}
				assistantTokens += 4

				var relatedToolMsgs []*schema.Message
				relatedTokens := 0
				for j := i + 1; j < len(messages); j++ {
					if messages[j].Role == schema.Tool {
						for _, tc := range msg.ToolCalls {
							if messages[j].ToolCallID == tc.ID {
								relatedToolMsgs = append(relatedToolMsgs, messages[j])
								relatedTokens += estimateTokens(messages[j].Content) + 4
								break
							}
						}
					}
				}

				groupTokens := assistantTokens + relatedTokens
				if tokenSum+groupTokens <= maxTokens {
					started = true
					kept = append(kept, msg)
					tokenSum += groupTokens
					kept = append(kept, relatedToolMsgs...)
				}
			} else if msg.Role == schema.Tool {
				if toolCallIDs[msg.ToolCallID] {
					continue
				}
			} else {
				msgTokens := estimateTokens(msg.Content) + 4
				if tokenSum+msgTokens <= maxTokens {
					started = true
					kept = append(kept, msg)
					tokenSum += msgTokens
				}
			}
			continue
		}

		msgTokens := estimateTokens(msg.Content) + 4
		for _, tc := range msg.ToolCalls {
			msgTokens += estimateTokens(tc.Function.Name) + estimateTokens(tc.Function.Arguments)
		}

		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			var relatedToolMsgs []*schema.Message
			for j := i + 1; j < len(messages); j++ {
				if messages[j].Role == schema.Tool {
					for _, tc := range msg.ToolCalls {
						if messages[j].ToolCallID == tc.ID {
							relatedToolMsgs = append(relatedToolMsgs, messages[j])
							msgTokens += estimateTokens(messages[j].Content) + 4
							break
						}
					}
				}
			}

			if tokenSum+msgTokens > maxTokens {
				break
			}
			kept = append(kept, msg)
			tokenSum += msgTokens
			kept = append(kept, relatedToolMsgs...)
		} else if msg.Role == schema.Tool {
			if !toolCallIDs[msg.ToolCallID] {
				if tokenSum+msgTokens <= maxTokens {
					kept = append(kept, msg)
					tokenSum += msgTokens
				}
			} else {
				kept = append(kept, msg)
				tokenSum += msgTokens
			}
		} else {
			if tokenSum+msgTokens > maxTokens {
				break
			}
			kept = append(kept, msg)
			tokenSum += msgTokens
		}
	}

	if len(kept) == 0 && len(messages) > 0 {
		last := messages[len(messages)-1]
		kept = append(kept, last)
	}

	return kept
}

// isTokenLimitError 判断错误是否为「上下文长度超限」类错误，用于触发裁剪历史重试。
//
// 必须精确匹配上下文超限特征，避免把以下错误误判为上下文超限（否则会触发无意义的
// 裁剪历史重试，并用误导性文案掩盖真实原因）：
//   - max_tokens 参数值过大（400 InvalidParameter，含 "max_tokens"/"token"/"400"，
//     如智谱 GLM: "expected a value <= 128000, but got 131072"）
//   - 限流 / 配额超限（rate limit exceeded / quota exceeded）
//   - 超时（deadline exceeded）
func isTokenLimitError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	// 1. 明确的上下文长度超限特征
	if strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "context length") ||
		strings.Contains(lower, "maximum context") ||
		strings.Contains(lower, "max_prompt_tokens") ||
		strings.Contains(lower, "too many tokens") ||
		strings.Contains(lower, "context window") ||
		strings.Contains(lower, "token limit") ||
		strings.Contains(lower, "reduce the length of the messages") ||
		strings.Contains(lower, "maximum number of tokens") {
		return true
	}
	// 2. "exceeded" 必须同时伴随 context/length/token 语义，
	//    排除 rate limit / quota / deadline exceeded 等非上下文超限错误。
	if strings.Contains(lower, "exceeded") &&
		(strings.Contains(lower, "context") || strings.Contains(lower, "length") || strings.Contains(lower, "token")) {
		// 但排除 max_tokens 参数值过大错误（如 "max_tokens ... above maximum value"），
		// 这类错误虽然含 token/exceeded 语义，但根因是参数值超出 API 限制而非对话过长。
		if strings.Contains(lower, "max_tokens") &&
			(strings.Contains(lower, "maximum value") || strings.Contains(lower, "above maximum")) {
			return false
		}
		return true
	}
	return false
}
