package data

import (
	"fmt"
	"testing"
)

// TestSearchGovPolicyLibrary 验证国务院政策文件库检索（需联网）
func TestSearchGovPolicyLibrary(t *testing.T) {
	// 1. 最新文件（无关键词，按发布时间倒序）
	items := *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "", "pubtime", 1, 5)
	if len(items) == 0 {
		t.Fatal("政策文件库检索结果为空")
	}
	fmt.Println("=== 最新文件 ===")
	for i, d := range items {
		if i >= 5 {
			break
		}
		fmt.Printf("%s | %s | %s | %s | %s\n", d.Pubtime, d.CategoryName, d.Puborg, d.Title, d.Url)
	}
	if items[0].Title == "" || items[0].Url == "" {
		t.Fatal("条目字段为空")
	}

	// 2. 关键词检索（标题）
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("人工智能", "title", "", "score", 1, 5)
	if len(items) == 0 {
		t.Fatal("关键词检索结果为空")
	}
	fmt.Println("=== 关键词：人工智能 ===")
	for _, d := range items {
		fmt.Printf("%s | %s | %s\n", d.Pubtime, d.CategoryName, d.Title)
	}
	for _, d := range items {
		if len(d.Title) > 0 && (d.Title[0] == '<' || d.Title[len(d.Title)-1] == '>') {
			t.Fatalf("标题残留 HTML 标签: %s", d.Title)
		}
	}

	// 3. 类别过滤：仅国务院文件
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "gongwen", "pubtime", 1, 5)
	if len(items) == 0 {
		t.Fatal("类别过滤结果为空")
	}
	for _, d := range items {
		if d.Category != "gongwen" || d.CategoryName != "国务院文件" {
			t.Fatalf("类别过滤失败: %s", d.CategoryName)
		}
	}
	fmt.Println("=== 国务院文件（类别过滤）===")
	for _, d := range items {
		fmt.Printf("%s | %s | %s | %s\n", d.Pubtime, d.Pcode, d.Puborg, d.Title)
	}

	// 4. Markdown 输出
	md := NewGovPolicyLibApi().SearchGovPolicyLibraryToMarkdown("人工智能", "title", "", "score", 1, 5)
	fmt.Println("=== Markdown 输出（前 500 字）===")
	r := []rune(md)
	if len(r) > 500 {
		r = r[:500]
	}
	fmt.Println(string(r))
	if len(md) < 100 {
		t.Fatal("Markdown 输出过短")
	}
}
