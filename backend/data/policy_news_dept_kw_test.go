package data

import (
	"fmt"
	"strings"
	"testing"
)

// TestGetPolicyNewsListByDeptKeyword 验证 GetPolicyNewsList 按部门名称关键词检索
// （依赖本地库已有数据；库为空时跳过断言仅验证不报错）
func TestGetPolicyNewsListByDeptKeyword(t *testing.T) {
	// 1. 关键词"能源"应命中"国家能源局"（库里已入库）或实时抓取回退
	md := NewPolicyNewsApi().GetPolicyNewsToMarkdown("能源", "", 10)
	fmt.Println("=== department=能源 ===")
	r := []rune(md)
	if len(r) > 400 {
		r = r[:400]
	}
	fmt.Println(string(r))
	if strings.Contains(md, "参数 url 不能为空") {
		t.Fatal("不应返回参数错误")
	}

	// 2. 关键词"数据"应命中"国家数据局"
	md = NewPolicyNewsApi().GetPolicyNewsToMarkdown("数据", "", 10)
	fmt.Println("\n=== department=数据（前 300 字）===")
	r = []rune(md)
	if len(r) > 300 {
		r = r[:300]
	}
	fmt.Println(string(r))

	// 3. 精确部门全名仍正常（走 LIKE 同样命中）
	md = NewPolicyNewsApi().GetPolicyNewsToMarkdown("国家能源局", "", 5)
	if !strings.Contains(md, "国家能源局 最新政策新闻") && !strings.Contains(md, "未抓取到") && !strings.Contains(md, "暂未抓取") {
		t.Fatalf("精确部门名查询异常: %s", md[:min(100, len(md))])
	}

	// 4. 无命中关键词：不 panic，返回空提示
	md = NewPolicyNewsApi().GetPolicyNewsToMarkdown("不存在的部门XYZ", "", 5)
	if !strings.Contains(md, "暂未抓取到政策新闻") && !strings.Contains(md, "未抓取") {
		// 库里 LIKE 无命中 + 部门列表无命中 -> 应返回空提示
		t.Logf("无命中关键词返回: %s", md)
	}
	fmt.Println("\n=== 无命中关键词 ===")
	fmt.Println(md)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
