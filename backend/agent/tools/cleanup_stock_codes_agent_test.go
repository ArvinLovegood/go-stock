package tools

import (
	"fmt"
	"testing"
)

// TestCleanupStockCodesToolGroup 验证 CleanupStockCodes 工具的分组映射。
// 注意：GetAllDataTools() 需要数据库初始化，这里只测试分组映射。
func TestCleanupStockCodesToolGroup(t *testing.T) {
	group, exists := toolGroupMap["CleanupStockCodes"]
	if !exists {
		t.Fatalf("✗ toolGroupMap 中不存在 CleanupStockCodes")
	}
	if group != GroupBase {
		t.Errorf("✗ CleanupStockCodes 分组错误：got=%v, want=%v", group, GroupBase)
	}
	fmt.Printf("✓ CleanupStockCodes 分组：%v\n", group)
}
