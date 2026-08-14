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

// conversationalOpenerRe 匹配 AI 回复常见的对话开头语前缀，标题兜底时跳过这类行，
// 避免把"我来…"/"明白…"/"收到…"/"先读取…"等过程性陈述当作标题。
var conversationalOpenerRe = regexp.MustCompile(`^(我来|我先|我已|我现在|我会|我将|我们|让我|您好|您想|您需要|明白|好的|收到|了解|继续|是的|这套|这次|先读取|先查找|先看|先做|先检查|先执行|已掌握|已完成|已更新|已固化|资料)`)

// leadingChineseOrdinalRe 匹配行首的中文序号前缀（可前置 emoji/符号/空白），
// 如"一、""二、""📊 一、"，提取标题时剥离，避免标题带序号。
var leadingChineseOrdinalRe = regexp.MustCompile(`^[\p{So}\p{Sk}\p{Sm}\p{Sc}\s]*[一二三四五六七八九十百千零]+、`)

// leadingArabicOrdinalRe 匹配行首的阿拉伯数字序号前缀（可前置 emoji/符号/空白），
// 如"1、""1. ""1) ""（1）""①""1.1 "等，提取标题时剥离。纯数字开头（如"100股""2026年"）不受影响。
// 注意：点号分隔符须后接空白（\d+\.\s+），避免误伤小数；多级序号"1.1 "单独匹配。
var leadingArabicOrdinalRe = regexp.MustCompile(`^[\p{So}\p{Sk}\p{Sm}\p{Sc}\s]*(?:\d+(?:\.\d+)*[、）)]\s*|\d+(?:\.\d+)+\s+|\d+\.\s+|[\(（]\d+[\)）]\s*|[①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳]\s*)`)

// leadingChapterRe 匹配行首的"第X章/节/部分/篇/回/课/讲/卷/单元/步"等章节序号前缀
// （可前置 emoji/符号/空白，可后接冒号），提取标题时剥离。不含"季"，避免误伤"第三季度"等。
var leadingChapterRe = regexp.MustCompile(`^[\p{So}\p{Sk}\p{Sm}\p{Sc}\s]*第[一二三四五六七八九十百千零]+(?:部分|单元|章节|[章节篇回课讲卷组类条款项部步])\s*[:：]?\s*`)

// ExtractTitleFromContent 从 Markdown 文本中提取标题：
//  0. 优先取被分隔线（---/***/___）包裹的块中首个 Markdown 标题
//  1. 否则取首个 Markdown 标题（# ~ ######）的文本（跳过代码块内的行）
//  2. 否则取首行有效文本（仅首个非空行；跳过对话开头语/表格行/含句号/以冒号结尾的标签行，不符合则返回空）
//  3. 剥离行首序号前缀（一、二、… / 1、1. 1) （1）① / 第X章 第X步 等），截断到 shareTitleMaxLen 字符
//  4. 都取不到时返回空字符串（由调用方用提问等兜底）
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

	// 0. 优先从被分隔线（--- 等）包裹的块中提取首个 Markdown 标题
	if t := extractTitleFromSeparatedBlock(lines, inCodeBlock); t != "" {
		return t
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
				return finalizeTitle(t)
			}
		}
	}

	// 2. 首行有效文本：取首个非空、非分隔线、非代码块的行；
	//    仅当它是标题样行（非对话开头语、非表格行、不含句号、不以冒号结尾）时采用，
	//    否则返回空串，由调用方用用户提问等兜底——避免把过程陈述/表格头/标签行误当作标题。
	for i, raw := range lines {
		if inCodeBlock[i] {
			continue
		}
		line := strings.TrimSpace(raw)
		if line == "" || separatorRe.MatchString(line) {
			continue
		}
		// 跳过对话开头语（我来…/明白…/收到…/先读取… 等）→ 直接放弃，交由提问兜底
		if conversationalOpenerRe.MatchString(line) {
			return ""
		}
		// 跳过表格行、含句号的整句、以冒号结尾的标签行
		if strings.Contains(line, "|") || strings.Contains(line, "。") ||
			strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":") {
			return ""
		}
		// 剥离行首格式符号
		stripped := linePrefixRe.ReplaceAllString(line, "")
		// 移除所有行内强调/代码修饰符（* _ ` ~），保留可读文本
		stripped = emphasisCharsRe.ReplaceAllString(stripped, "")
		stripped = strings.TrimSpace(stripped)
		if stripped != "" {
			return finalizeTitle(stripped)
		}
		return ""
	}
	return ""
}

// extractTitleFromSeparatedBlock 优先从被分隔线（---/***/___）包裹的块中提取首个 Markdown 标题。
// 将任意两条相邻分隔线之间的内容视为一个包裹块，按出现顺序检查每个块，
// 返回首个含 Markdown 标题的块中的标题；找不到返回空字符串。
// 代码块内的分隔线与标题均被忽略。
func extractTitleFromSeparatedBlock(lines []string, inCodeBlock []bool) string {
	var sepIndices []int
	for i, raw := range lines {
		if inCodeBlock[i] {
			continue
		}
		if separatorRe.MatchString(strings.TrimSpace(raw)) {
			sepIndices = append(sepIndices, i)
		}
	}
	for k := 0; k+1 < len(sepIndices); k++ {
		start := sepIndices[k] + 1
		end := sepIndices[k+1]
		for i := start; i < end; i++ {
			if inCodeBlock[i] {
				continue
			}
			m := markdownHeadingRe.FindStringSubmatch(strings.TrimSpace(lines[i]))
			if len(m) >= 3 && m[2] != "" {
				t := strings.TrimSpace(emphasisCharsRe.ReplaceAllString(m[2], ""))
				if t != "" {
					return finalizeTitle(t)
				}
			}
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

// finalizeTitle 剥离行首序号前缀（一、二、… / 1、1. 1) （1）① / 第X章 第X步 等，循环剥离嵌套序号）后截断到 shareTitleMaxLen 字符。
func finalizeTitle(s string) string {
	prev := ""
	for s != prev {
		prev = s
		s = leadingArabicOrdinalRe.ReplaceAllString(s, "")
		s = leadingChineseOrdinalRe.ReplaceAllString(s, "")
		s = leadingChapterRe.ReplaceAllString(s, "")
	}
	s = strings.TrimSpace(s)
	return truncateTitle(s)
}
