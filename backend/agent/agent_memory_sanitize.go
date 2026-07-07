package agent

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	stockCodePattern   = regexp.MustCompile(`(?:^|[^\d])((?:sh|sz|bj|hk)?\d{6})(?:[^\d]|$)`)
	priceLikePattern   = regexp.MustCompile(`(?:[-+]?\d[\d,]*(?:\.\d+)?)\s*(?:%|％|元|块|点|亿|万|倍|PE|PB|ROE|EPS|市值|股|手)`)
	plainNumberPattern = regexp.MustCompile(`(?:^|[^\d.])([-+]?\d[\d,]*(?:\.\d+)?)(?:%|％)?`)
)

const historyNumericPlaceholder = "[旧值]"

// sanitizeAssistantHistoryForContext 对注入 Agent 上下文的历史助手回复做数值脱敏，降低模型复用过时数据的风险。
// 仅影响加载到 LLM 的上下文，不修改数据库中保存的原始内容。
func sanitizeAssistantHistoryForContext(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	prefix := "【历史回复摘要：以下具体数值可能已过时，不可作为事实依据；涉及数字须重新调用工具。注意：你的回复中不得出现[旧值]等占位符，必须调用工具获取最新数值并直接输出】\n"
	if strings.HasPrefix(content, "【历史回复摘要") {
		prefix = ""
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, redactHistoryLine(line))
	}
	body := strings.Join(out, "\n")
	if prefix == "" {
		return body
	}
	return prefix + body
}

func redactHistoryLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	if stockCodePattern.MatchString(line) && !priceLikePattern.MatchString(line) {
		return line
	}
	if !containsHistoryNumeric(line) {
		return line
	}

	protected := map[string]string{}
	protectIdx := 0
	line = stockCodePattern.ReplaceAllStringFunc(line, func(match string) string {
		sub := stockCodePattern.FindStringSubmatch(match)
		code := match
		if len(sub) > 1 && sub[1] != "" {
			code = sub[1]
		}
		key := fmt.Sprintf("{{SC_%c}}", 'a'+protectIdx)
		protected[key] = code
		protectIdx++
		return strings.Replace(match, code, key, 1)
	})

	redacted := priceLikePattern.ReplaceAllString(line, historyNumericPlaceholder)
	redacted = plainNumberPattern.ReplaceAllStringFunc(redacted, func(s string) string {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" || strings.Contains(trimmed, "{{SC") {
			return s
		}
		for _, r := range trimmed {
			if r >= '0' && r <= '9' {
				return strings.Replace(s, trimmed, historyNumericPlaceholder, 1)
			}
		}
		return s
	})

	for key, code := range protected {
		redacted = strings.ReplaceAll(redacted, key, code)
	}
	if redacted == line && len(protected) == 0 {
		return line
	}
	return redacted
}

func containsHistoryNumeric(line string) bool {
	if priceLikePattern.MatchString(line) {
		return true
	}
	if strings.Contains(line, "%") || strings.Contains(line, "％") {
		return plainNumberPattern.MatchString(line)
	}
	if strings.ContainsAny(line, "元块") && plainNumberPattern.MatchString(line) {
		return true
	}
	return false
}
