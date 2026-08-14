package util

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestPrintExtractedTitles 打印 D:\go-stock\memory\2026-08-14 下所有 .md 文件的标题提取结果。
// 模拟 ShareText 的完整标题解析优先级：
//  1. 从 AI 回复正文提取（--- 包裹 # 标题 → 首个 # 标题 → 首行有效文本，跳过对话开头语）
//  2. 提取失败 → 用用户提问（文件元数据 - **问题**: xxx）兜底
//  3. 仍为空 → "AI助手"
func TestPrintExtractedTitles(t *testing.T) {
	dir := `D:\go-stock\memory\2026-08-14`
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("目录不存在或无法访问: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	fmt.Println("========== 标题提取结果（模拟 ShareText 完整优先级）==========")
	fmt.Printf("%-3s %-40s | %-22s | %-22s | %s\n", "#", "文件", "正文提取", "用户提问", "最终标题(分享用)")
	fmt.Println(strings.Repeat("-", 130))
	for idx, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Printf("❌ 读取失败: %s -> %v\n", name, err)
			continue
		}
		content := string(data)
		replyBody := extractAiReplyBody(content)
		question := extractQuestionFromMeta(content)
		extracted := ExtractTitleFromContent(replyBody)
		// 模拟 ShareText 标题解析：提取失败 → 提问兜底（剥离开头 #）→ AI助手
		final := extracted
		if final == "" {
			final = question
			for strings.HasPrefix(final, "#") {
				final = strings.TrimSpace(strings.TrimPrefix(final, "#"))
			}
		}
		if final == "" {
			final = "AI助手"
		}
		mark := "  "
		if extracted == "" && question != "" {
			mark = "🆎" // 用了提问兜底
		}
		fmt.Printf("%s%-2d %-40s | %-22s | %-22s | %s\n",
			mark, idx+1, truncForPrint(name, 40),
			truncForPrint(extracted, 20), truncForPrint(question, 20), truncForPrint(final, 30))
	}
	fmt.Println("============================================================")
	fmt.Println("🆎 = 正文提取失败，用用户提问兜底")
}

// extractAiReplyBody 提取 ## AI 回复 之后的正文，模拟 getLastAssistantContent() 返回内容。
func extractAiReplyBody(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "## AI 回复" || trim == "## AI回复" || strings.HasPrefix(trim, "## AI 回复") {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

// extractQuestionFromMeta 从保存文件的元数据 "- **问题**: xxx" 提取用户提问。
func extractQuestionFromMeta(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- **问题**:") {
			q := strings.TrimPrefix(trim, "- **问题**:")
			return strings.TrimSpace(q)
		}
	}
	return ""
}

func truncForPrint(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// TestExtractTitleStripOrdinals 验证行首序号前缀（中文/阿拉伯数字/带括号/圈号）被剥离。
func TestExtractTitleStripOrdinals(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"## 一、隔夜美股分析", "隔夜美股分析"},
		{"## 二、板块筛选结果", "板块筛选结果"},
		{"## 📊 一、央行数据", "央行数据"},
		{"## 1、早盘选股指标", "早盘选股指标"},
		{"## 1. 适用场景", "适用场景"},
		{"## 1) 方案一", "方案一"},
		{"## （1）背景介绍", "背景介绍"},
		{"## (2) 背景", "背景"},
		{"## ① 行情综述", "行情综述"},
		{"## 1.1 多级标题", "多级标题"},     // 多级序号+空格，剥离
		{"## 100股买入策略", "100股买入策略"}, // 纯数字+非序号分隔，保留
		{"## 2026年展望", "2026年展望"},   // 纯数字+年，保留
		{"## 一、1、嵌套序号", "嵌套序号"},     // 嵌套，循环剥离
		{"## 第一章 概述", "概述"},         // 第X章
		{"## 第二节 分析", "分析"},         // 第X节
		{"## 第三部分 内容", "内容"},        // 第X部分
		{"## 第一步：宏观环境扫描", "宏观环境扫描"}, // 第X步 + 冒号
		{"## 第十二篇 结语", "结语"},        // 第X篇 + 两位中文数字
		{"## 第三季度报告", "第三季度报告"},     // 第三季度（非章节），保留
	}
	for _, c := range cases {
		got := ExtractTitleFromContent(c.in)
		if got != c.want {
			t.Errorf("ExtractTitleFromContent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
