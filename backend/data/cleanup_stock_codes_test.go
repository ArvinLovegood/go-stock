package data

import (
	"os"
	"testing"

	"go-stock/backend/db"
)

func init() {
	if os.Getenv("TEST_DB_SKIP_INIT") == "1" {
		return
	}
	db.Init("D:/go-stock/data/stock.db")
}

// TestCleanupStockCodesDryRun 验证清理工具的扫描能力（dryRun=true，只读不写）。
// 作为回归测试：如果数据库中出现不规范的 stock_code，本测试会通过 t.Logf 提示，
// 可在 AI 对话中调用 CleanupStockCodes 工具（dryRun=false）执行实际清理。
func TestCleanupStockCodesDryRun(t *testing.T) {
	// dryRun=true：只扫描，不修改数据库
	followedFixed, followedDeleted, followedSkipped := cleanupFollowedStockTable(true)
	t.Logf("followed_stock 表：归一化=%d, 合并删除=%d, 跳过=%d",
		followedFixed, followedDeleted, followedSkipped)

	groupFixed, groupDeleted, groupSkipped := cleanupGroupStockInfoTable(true)
	t.Logf("group_stock_info 表：归一化=%d, 合并删除=%d, 跳过=%d",
		groupFixed, groupDeleted, groupSkipped)

	// 数据库应该全是规范格式，归一化数和合并删除数都应为 0
	if followedFixed > 0 || followedDeleted > 0 {
		t.Errorf("followed_stock 表存在不规范 stock_code，请调用 CleanupStockCodes 工具清理")
	}
	if groupFixed > 0 || groupDeleted > 0 {
		t.Errorf("group_stock_info 表存在不规范 stock_code，请调用 CleanupStockCodes 工具清理")
	}
}
