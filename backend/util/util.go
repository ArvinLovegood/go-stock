package util

import (
	"regexp"
	"strings"

	uaFake "github.com/lib4u/fake-useragent"
)

// @Author spark
// @Date 2026/4/10 9:02
// @Desc
//-----------------------------------------------------------------------------------

func GetUserAgent() string {
	ua, _ := uaFake.New()
	if ua != nil {
		randomUA := ua.Filter().Platform("desktop").Get()
		return randomUA
	}
	// 如果库获取失败，返回备用 UA
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

}

// 标题最大长度，超出以 … 省略
const shareTitleMaxLen = 60

// Markdown 标题正则：# ~ ######
var markdownHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)

// 纯分隔线（---/***/___，至少 3 个相同字符）
var separatorRe = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)

// 行首格式符号（标题 / 引用 / 列表 / 有序列表）
var linePrefixRe = regexp.MustCompile(`^\s*(#{1,6}\s+|>\s*|[-*+]\s+|\d+\.\s+)`)

// emphasisCharsRe 匹配所有强调/代码修饰符（* _ ` ~）
var emphasisCharsRe = regexp.MustCompile(`[*_` + "`" + `~]`)

// codeFenceRe 匹配 Markdown 代码块边界（``` 或 ~~~，可带语言标识）
var codeFenceRe = regexp.MustCompile("^(`{3,}|~{3,})")

// ExtractTitleFromContent 从 Markdown 文本中提取标题：
//  1. 优先取首个 Markdown 标题（# ~ ######）的文本（跳过代码块内的行）
//  2. 否则取首行非空、非纯格式符号的文本（剥离 **、`、>、-、* 等前缀和行内强调符）
//  3. 截断到 shareTitleMaxLen 字符，避免标题过长
//  4. 都取不到时返回空字符串
func ExtractTitleFromContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")

	// 预计算每行是否处于代码块内
	inCodeBlock := make([]bool, len(lines))
	inside := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if codeFenceRe.MatchString(line) {
			inCodeBlock[i] = true // 围栏行本身也算代码块内（跳过）
			inside = !inside
			continue
		}
		inCodeBlock[i] = inside
	}

	// 1. 首个 Markdown 标题（跳过代码块内的）
	for i, raw := range lines {
		if inCodeBlock[i] {
			continue
		}
		line := strings.TrimSpace(raw)
		m := markdownHeadingRe.FindStringSubmatch(line)
		if len(m) >= 3 && m[2] != "" {
			t := strings.TrimSpace(emphasisCharsRe.ReplaceAllString(m[2], ""))
			if t != "" {
				return truncateTitle(t)
			}
		}
	}

	// 2. 首行有效文本（跳过代码块内的）
	for i, raw := range lines {
		if inCodeBlock[i] {
			continue
		}
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// 跳过纯分隔线
		if separatorRe.MatchString(line) {
			continue
		}
		// 剥离行首格式符号
		stripped := linePrefixRe.ReplaceAllString(line, "")
		// 移除所有行内强调/代码修饰符（* _ ` ~），保留可读文本
		stripped = emphasisCharsRe.ReplaceAllString(stripped, "")
		stripped = strings.TrimSpace(stripped)
		if stripped != "" {
			return truncateTitle(stripped)
		}
	}
	return ""
}

// truncateTitle 截断标题到 shareTitleMaxLen 字符，超出加 …
func truncateTitle(s string) string {
	runes := []rune(s)
	if len(runes) <= shareTitleMaxLen {
		return s
	}
	return string(runes[:shareTitleMaxLen]) + "…"
}
