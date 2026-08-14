package agent

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const toolMetadataPrefix = "[as_of="

// wrapToolResultMetadata 为工具输出添加来源与时间元数据，便于模型识别数据时效与可用性。
func wrapToolResultMetadata(toolName, content, status string) string {
	if strings.HasPrefix(strings.TrimSpace(content), toolMetadataPrefix) {
		return content
	}
	asOf := extractToolAsOf(content)
	source := extractToolSource(content)
	observedAt := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf("[as_of=%s] [observed_at=%s] [source=%s] [tool=%s] [status=%s]\n%s",
		asOf, observedAt, source, toolName, status, content)
}

var toolAsOfRe = regexp.MustCompile(`(?i)(?:更新时间|数据日期|交易日期|报告期|日期)\s*[:：|]\s*([0-9]{4}[-/]?[0-9]{1,2}[-/]?[0-9]{1,2}(?:\s+[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)?)`)

// extractToolAsOf 只从工具正文中提取明确的日期；提取不到时返回 unknown，
// 不能把“工具调用时间”冒充成行情数据时间。
func extractToolAsOf(content string) string {
	match := toolAsOfRe.FindStringSubmatch(content)
	if len(match) != 2 {
		return "unknown"
	}
	return strings.ReplaceAll(strings.TrimSpace(match[1]), "/", "-")
}

var toolSourceRe = regexp.MustCompile(`(?i)(?:数据)?来源\s*[:：|]\s*([^\n|，,]+)`)

func extractToolSource(content string) string {
	match := toolSourceRe.FindStringSubmatch(content)
	if len(match) != 2 {
		return "unknown"
	}
	source := strings.TrimSpace(match[1])
	if source == "" {
		return "unknown"
	}
	return source
}

func detectToolResultStatus(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "empty"
	}
	if strings.Contains(trimmed, "工具调用出错") || strings.Contains(trimmed, "工具调用异常") {
		return "error"
	}
	emptyMarkers := []string{
		"无符合条件的数据",
		"未获取到",
		"未找到",
		"查询失败",
		"请提供",
	}
	for _, marker := range emptyMarkers {
		if strings.Contains(trimmed, marker) {
			return "empty"
		}
	}
	return "ok"
}

func isToolMetadataLine(line string) bool {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, toolMetadataPrefix) {
		return true
	}
	if strings.HasPrefix(t, "**更新时间**") {
		return true
	}
	return false
}

// splitToolMetadataPrefix 分离工具输出开头的元数据行，供压缩/截断时强制保留。
func splitToolMetadataPrefix(content string) (meta []string, body string) {
	if content == "" {
		return nil, content
	}
	lines := strings.Split(content, "\n")
	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		if isToolMetadataLine(lines[i]) {
			meta = append(meta, lines[i])
			i++
			continue
		}
		break
	}
	if i >= len(lines) {
		return meta, ""
	}
	return meta, strings.Join(lines[i:], "\n")
}

// joinToolMetadataAndBody 将元数据前缀重新拼接到压缩后的正文前。
func joinToolMetadataAndBody(meta []string, body string) string {
	if len(meta) == 0 {
		return body
	}
	prefix := strings.Join(meta, "\n")
	if strings.TrimSpace(body) == "" {
		return prefix
	}
	return prefix + "\n" + body
}
