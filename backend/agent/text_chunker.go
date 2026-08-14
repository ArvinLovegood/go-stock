package agent

// text_chunker.go — 基于 go-chunker 的文本切片封装。
//
// 用 LangChain 风格的递归分隔符策略替代自研切分逻辑：
//   - Markdown 标题 → 段落（\n\n）→ 单行（\n）→ 句号（。）→ 空格 → 字符
//   - 每片目标 ~500 字符（rune 计数），重叠 60 字符
//   - 过短片段（<20 字）会被合并而非单独输出
//
// 同时被 knowledge_base.go (sliceForKB) 和 long_term_memory.go (sliceForEmbedding) 复用。

import (
	"context"
	"strings"

	"github.com/ABDELRAHMAN-ELRAYES/go-chunker"
)

// chunkText 用 go-chunker 的 Markdown 策略将文本切分为适合 embedding 的片段。
//
// 参数：
//   - content: 已经过 normalizeText 标准化的文本
//
// 返回切分后的字符串数组（已 TrimSpace 且过滤 <20 字的过短片段）。
// 切分失败时返回 nil + 错误（极少数情况，如 ctx 取消）。
func chunkText(content string) ([]string, error) {
	c := strings.TrimSpace(content)
	if c == "" {
		return nil, nil
	}

	// 用 Markdown 策略：优先按标题/段落切，回退到句号/空格
	// LenFunc 用 rune 计数（中文友好，1 个汉字算 1）
	splitter := chunker.NewMarkdown(
		chunker.WithSize(chunkTargetRunes),
		chunker.WithOverlap(chunkOverlapRunes),
		chunker.WithMinSize(20),
		chunker.WithLenFunc(func(s string) int { return len([]rune(s)) }),
	)

	chunks, err := splitter.Split(context.Background(), c, chunker.Meta{})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		s := strings.TrimSpace(ch.Text)
		if len([]rune(s)) >= 20 {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
