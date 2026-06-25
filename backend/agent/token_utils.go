package agent

import (
	"go-stock/backend/logger"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/schema"
)

const (
	chineseCharsPerToken = 1.5
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

// getModelContextWindow 根据模型名估算模型的总上下文窗口（输入+输出）大小（token）。
// 注意：这与 AIConfig.MaxTokens 不同——MaxTokens 是单次回复的最大输出 token 上限，
// 而上下文窗口是模型一次请求能容纳的输入+输出总量，通常远大于输出上限。
func getModelContextWindow(modelName string) int {
	m := strings.ToLower(strings.TrimSpace(modelName))
	if m == "" {
		return 64000
	}
	// 按子串匹配常见模型的上下文窗口，保守取值。
	type entry struct {
		key    string
		window int
	}
	table := []entry{
		{"deepseek-reasoner", 64000},
		{"deepseek", 64000},
		{"gpt-4o", 128000},
		{"gpt-4.1", 128000},
		{"gpt-4-turbo", 128000},
		{"gpt-4", 128000},
		{"gpt-3.5", 16000},
		{"o1", 128000},
		{"o3", 128000},
		{"claude", 200000},
		{"gemini", 128000},
		{"moonshot", 128000},
		{"kimi", 128000},
		{"glm-4", 128000},
		{"glm", 128000},
		{"qwen-long", 1000000},
		{"qwen", 128000},
		{"doubao", 128000},
		{"ernie", 128000},
		{"yi", 32000},
		{"llama", 32000},
		{"mistral", 32000},
		{"gemma", 8000},
	}
	for _, e := range table {
		if strings.Contains(m, e.key) {
			return e.window
		}
	}
	// 未知模型：使用一个对现代 API 模型较安全的默认值，避免被过度压缩到极小预算。
	return 64000
}

// getMaxInputTokens 计算可用于输入（system + 历史 + 当前问题 + 工具结果）的最大 token 预算。
// 预算 = 上下文窗口 - 输出上限 - 各项固定预留，再乘以安全系数。
// 关键修复：输入预算应基于「模型上下文窗口」而非「输出上限 MaxTokens」推导，
// 否则当用户把 MaxTokens 设为 8192（DeepSeek 的输出上限）时，预算会被错误地压到下限 4000，
// 导致工具结果/历史被过度截断，模型在拿不到完整数据的情况下草草收尾或提前结束。
func getMaxInputTokens(maxOutputTokens int, modelName string) int {
	contextWindow := getModelContextWindow(modelName)
	if maxOutputTokens <= 0 {
		maxOutputTokens = 8192
	}

	availableTokens := int(float64(contextWindow) * safetyMargin)
	reserved := toolsTokenReserve + skillPromptReserve + reactLoopReserve + maxOutputTokens
	result := availableTokens - reserved
	if result < 4000 {
		result = 4000
	}
	return result
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

func trimToolResult(content string, maxTokens int) string {
	if content == "" {
		return content
	}
	estimatedTokens := estimateTokens(content)
	if estimatedTokens <= maxTokens {
		return content
	}
	maxChars := int(float64(maxTokens) * englishCharsPerToken * 0.8)
	if maxChars > len(content) {
		maxChars = len(content)
	}

	lines := strings.Split(content, "\n")
	var result []string
	charCount := 0
	for i := len(lines) - 1; i >= 0; i-- {
		lineLen := len(lines[i]) + 1
		if charCount+lineLen > maxChars {
			break
		}
		charCount += lineLen
		result = append([]string{lines[i]}, result...)
	}

	if len(result) == 0 && len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if len(lastLine) > maxChars {
			lastLine = lastLine[len(lastLine)-maxChars:]
		}
		result = []string{lastLine}
	}

	return strings.Join(result, "\n") + "\n\n...(内容过长，已截断显示)"
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
			msgTokens := estimateTokens(msg.Content)
			if msgTokens > 500 {
				currentBytes := len([]byte(msg.Content))
				ratio := float64(targetTokens) / float64(g.tokens)
				targetBytes := int(float64(currentBytes) * ratio)
				if targetBytes < 600 {
					targetBytes = 600
				}
				if targetBytes < currentBytes {
					compressed := smartContentCompress(msg.Content, targetBytes)
					cp := *msg
					cp.Content = compressed + "\n\n[以上数据已智能压缩，保留了关键指标和结论]"
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

func isTokenLimitError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "max_prompt_tokens") ||
		strings.Contains(errMsg, "exceeded") ||
		strings.Contains(errMsg, "token limit") ||
		strings.Contains(errMsg, "context_length") ||
		strings.Contains(errMsg, "maximum context") ||
		strings.Contains(errMsg, "too many tokens") ||
		strings.Contains(errMsg, "context window") ||
		strings.Contains(errMsg, "400") && strings.Contains(errMsg, "token")
}
