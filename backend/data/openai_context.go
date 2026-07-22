package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"strings"
	"unicode"
)

type OpenAIContextStats struct {
	BeforeTokens    int
	AfterTokens     int
	BudgetTokens    int
	DroppedMessages int
	Truncated       bool
}

func logOpenAIContextStats(model string, stats OpenAIContextStats) {
	if stats.BeforeTokens <= stats.AfterTokens {
		return
	}
	logger.SugaredLogger.Infof(
		"AI context trimmed model=%q before_tokens=%d after_tokens=%d budget_tokens=%d dropped_messages=%d truncated=%v",
		model, stats.BeforeTokens, stats.AfterTokens, stats.BudgetTokens, stats.DroppedMessages, stats.Truncated,
	)
}

// PrepareOpenAIMessages applies one context policy to both plain and tool-based
// direct requests. It returns a copy, keeps system instructions and recent
// conversation, and treats an assistant tool call plus its tool results as one
// atomic segment so providers never receive orphan tool messages.
func PrepareOpenAIMessages(messages []map[string]interface{}, tools []Tool, maxInputTokens int) ([]map[string]interface{}, OpenAIContextStats) {
	cloned := cloneOpenAIMessages(messages)
	stats := OpenAIContextStats{BeforeTokens: estimateOpenAIValueTokens(cloned)}
	if maxInputTokens <= 0 {
		maxInputTokens = 120000
	}
	overhead := 1000 + estimateOpenAIValueTokens(tools)
	budget := maxInputTokens - overhead
	if budget < 1000 {
		budget = max(1000, maxInputTokens/2)
	}
	stats.BudgetTokens = budget
	if stats.BeforeTokens <= budget {
		stats.AfterTokens = stats.BeforeTokens
		return cloned, stats
	}

	contentCap := max(1000, budget/2)
	for _, message := range cloned {
		content, ok := message["content"].(string)
		messageCap := contentCap
		if roleOf(message) == "system" {
			messageCap = max(1000, budget/3)
		}
		if !ok || estimateOpenAITextTokens(content) <= messageCap {
			continue
		}
		message["content"] = truncateOpenAIText(content, messageCap)
		stats.Truncated = true
	}

	systems := make([]map[string]interface{}, 0, 1)
	nonSystem := make([]map[string]interface{}, 0, len(cloned))
	for _, message := range cloned {
		if roleOf(message) == "system" {
			systems = append(systems, message)
		} else {
			nonSystem = append(nonSystem, message)
		}
	}
	segments := openAIMessageSegments(nonSystem)
	if len(segments) > 0 {
		// Always reserve room for the newest user/assistant segment. A very large
		// system prompt must not crowd the actual request out of the payload.
		systemBudget := budget - max(256, budget/3)
		if estimateOpenAIValueTokens(systems) > systemBudget {
			if shrunk := shrinkOpenAISegmentToBudget(systems, systemBudget); shrunk != nil {
				systems = shrunk
				stats.Truncated = true
			}
		}
	}
	remaining := budget - estimateOpenAIValueTokens(systems)
	if remaining < 0 {
		for _, message := range systems {
			if content, ok := message["content"].(string); ok {
				message["content"] = truncateOpenAIText(content, max(500, budget/max(2, len(systems))))
				stats.Truncated = true
			}
		}
		remaining = max(0, budget-estimateOpenAIValueTokens(systems))
	}

	selected := make([][]map[string]interface{}, 0, len(segments))
	for i := len(segments) - 1; i >= 0; i-- {
		segment := segments[i]
		segmentTokens := estimateOpenAIValueTokens(segment)
		if segmentTokens > remaining && len(selected) == 0 {
			// The newest segment is mandatory. Shrink its content further while
			// retaining assistant/tool-call atomicity.
			if shrunk := shrinkOpenAISegmentToBudget(segment, remaining); shrunk != nil {
				segment = shrunk
				segmentTokens = estimateOpenAIValueTokens(segment)
				stats.Truncated = true
			}
		}
		if segmentTokens > remaining {
			continue
		}
		selected = append([][]map[string]interface{}{segment}, selected...)
		remaining -= segmentTokens
	}

	result := append([]map[string]interface{}{}, systems...)
	for _, segment := range selected {
		result = append(result, segment...)
	}
	stats.DroppedMessages = len(cloned) - len(result)
	stats.AfterTokens = estimateOpenAIValueTokens(result)
	return result, stats
}

func shrinkOpenAISegmentToBudget(segment []map[string]interface{}, budget int) []map[string]interface{} {
	if budget <= 0 {
		return nil
	}
	for attempts := 0; attempts < len(segment)*4+4; attempts++ {
		current := estimateOpenAIValueTokens(segment)
		if current <= budget {
			return segment
		}
		largestIndex, largestTokens := -1, 0
		for i, message := range segment {
			content, ok := message["content"].(string)
			if !ok {
				continue
			}
			if tokens := estimateOpenAITextTokens(content); tokens > largestTokens {
				largestIndex, largestTokens = i, tokens
			}
		}
		if largestIndex < 0 || largestTokens <= 16 {
			return nil
		}
		target := max(16, largestTokens-(current-budget)-16)
		segment[largestIndex]["content"] = truncateOpenAIText(segment[largestIndex]["content"].(string), target)
	}
	if estimateOpenAIValueTokens(segment) <= budget {
		return segment
	}
	return nil
}

func openAIMessageSegments(messages []map[string]interface{}) [][]map[string]interface{} {
	segments := make([][]map[string]interface{}, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		message := messages[i]
		role := roleOf(message)
		if role == "tool" {
			continue
		}
		segment := []map[string]interface{}{message}
		if role == "assistant" {
			if _, hasToolCalls := message["tool_calls"]; hasToolCalls {
				for i+1 < len(messages) && roleOf(messages[i+1]) == "tool" {
					i++
					segment = append(segment, messages[i])
				}
			}
		}
		segments = append(segments, segment)
	}
	return segments
}

func cloneOpenAIMessages(messages []map[string]interface{}) []map[string]interface{} {
	encoded, err := json.Marshal(messages)
	if err == nil {
		var cloned []map[string]interface{}
		if json.Unmarshal(encoded, &cloned) == nil {
			return cloned
		}
	}
	cloned := make([]map[string]interface{}, 0, len(messages))
	for _, message := range messages {
		copyMessage := make(map[string]interface{}, len(message))
		for key, value := range message {
			copyMessage[key] = value
		}
		cloned = append(cloned, copyMessage)
	}
	return cloned
}

func roleOf(message map[string]interface{}) string {
	return strings.ToLower(strings.TrimSpace(fmt.Sprint(message["role"])))
}

func estimateOpenAIValueTokens(value interface{}) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		return estimateOpenAITextTokens(fmt.Sprint(value))
	}
	return estimateOpenAITextTokens(string(encoded))
}

func estimateOpenAITextTokens(text string) int {
	if text == "" {
		return 0
	}
	han, other := 0, 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			han++
		} else {
			other++
		}
	}
	return int(float64(han)/1.5+float64(other)/4.0) + 1
}

func truncateOpenAIText(text string, maxTokens int) string {
	if estimateOpenAITextTokens(text) <= maxTokens {
		return text
	}
	marker := "\n\n...(内容过长，已按输入预算截断)...\n\n"
	runes := []rune(text)
	keep := min(len(runes), max(100, maxTokens*2))
	for keep > 100 {
		head := keep * 7 / 10
		candidate := string(runes[:head]) + marker + string(runes[len(runes)-(keep-head):])
		if estimateOpenAITextTokens(candidate) <= maxTokens {
			return candidate
		}
		keep = keep * 4 / 5
	}
	return string(runes[:min(len(runes), 100)]) + marker
}
