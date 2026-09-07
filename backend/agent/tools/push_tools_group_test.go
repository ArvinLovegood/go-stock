package tools

import "testing"

// 推送工具应归入基础组：任意问题下都应保留（无需关键词触发）
func TestPushToolsInBaseGroup(t *testing.T) {
	pushTools := []string{
		"SendDingDingMessage",
		"SendToDingDing",
		"SendFeishuMessage",
		"SendToFeishu",
	}
	for _, name := range pushTools {
		group, exists := toolGroupMap[name]
		if !exists {
			t.Fatalf("✗ toolGroupMap 中不存在 %s", name)
		}
		if group != GroupBase {
			t.Errorf("✗ %s 分组为 %s，期望 %s", name, group, GroupBase)
		}
	}

	// 无关问题（不含任何组关键词）也应命中基础组
	groups := ClassifyQuestion("你好")
	if !groups[GroupBase] {
		t.Fatal("✗ 基础组未默认命中")
	}
}
