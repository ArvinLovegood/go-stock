package data

import (
	"fmt"
	"strings"
	"testing"
)

// TestSearchGovPolicyLibrary 验证国务院政策文件库检索（需联网）
func TestSearchGovPolicyLibrary(t *testing.T) {
	// 1. 最新文件（无关键词，按发布时间倒序）
	items := *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "", "", "pubtime", 1, 5)
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
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("人工智能", "title", "", "", "score", 1, 5)
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
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "", "gongwen", "pubtime", 1, 5)
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

	// 4. 部门过滤：部门全名（商务部，含多部门联合发文）
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "商务部", "", "pubtime", 1, 5)
	if len(items) == 0 {
		t.Fatal("部门过滤（商务部）结果为空")
	}
	for _, d := range items {
		if !strings.Contains(d.Puborg, "商务部") {
			t.Fatalf("部门过滤失败，出现非商务部条目: %s（%s）", d.Puborg, d.Title)
		}
	}
	fmt.Println("=== 部门：商务部 ===")
	for _, d := range items {
		fmt.Printf("%s | %s | %s | %s\n", d.Pubtime, d.Pcode, d.Puborg, d.Title)
	}

	// 5. 部门过滤：名称关键词（能源 -> 国家能源局，含联合发文；联合发文 Puborg 可能折叠为"XX部等"）
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("", "title", "能源", "", "pubtime", 1, 5)
	if len(items) == 0 {
		t.Fatal("部门关键词（能源）结果为空")
	}
	directHit := false
	for _, d := range items {
		if strings.Contains(d.Puborg, "国家能源局") {
			directHit = true
			break
		}
	}
	if !directHit {
		t.Logf("能源局文件均为联合发文（Puborg 折叠为 XX部等），列表: %v", items[0].Title)
	}
	fmt.Println("=== 部门关键词：能源 -> 国家能源局 ===")
	for _, d := range items {
		fmt.Printf("%s | %s | %s\n", d.Pubtime, d.Pcode, d.Title)
	}

	// 6. 关键词 + 部门组合
	items = *NewGovPolicyLibApi().SearchGovPolicyLibrary("消费", "title", "商务部", "", "score", 1, 5)
	fmt.Printf("=== 关键词 消费 + 商务部：共 %d 条 ===\n", len(items))
	for _, d := range items {
		fmt.Printf("%s | %s\n", d.Pubtime, d.Title)
	}

	// 7. Markdown 输出（含部门）
	md := NewGovPolicyLibApi().SearchGovPolicyLibraryToMarkdown("", "title", "能源", "", "pubtime", 1, 5)
	fmt.Println("=== Markdown 输出（部门=能源，前 400 字）===")
	r := []rune(md)
	if len(r) > 400 {
		r = r[:400]
	}
	fmt.Println(string(r))
	if len(md) < 100 {
		t.Fatal("Markdown 输出过短")
	}
	if !strings.Contains(md, "国家能源局") {
		t.Fatal("Markdown 标题应包含解析后的标准部门名")
	}
}
