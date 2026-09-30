package tools

import (
	"context"
	"sort"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// 分组裁剪只对 toolGroupMap / alwaysVisibleTools 中登记的工具生效：未登记的工具在
// FilterToolsByGroups 中一律保留（向后兼容），等于静默关闭了该工具的裁剪。
// 此测试强制每个内置工具名都被显式登记，避免新增工具后分组功能静默失效。
func TestAllStaticToolsAreGrouped(t *testing.T) {
	initToolGroupsTestDB(t)

	all := GetAllDataTools()
	all = append(all, GetQueryStockCodeInfoTool(), GetQueryStockNewsTool(), GetQueryBKDictTool())
	all = append(all, GetHolidayTools()...)

	seen := make(map[string]bool, len(all))
	var ungrouped []string
	for _, tl := range all {
		info, err := tl.Info(context.Background())
		if err != nil || info == nil || info.Name == "" {
			continue
		}
		if seen[info.Name] {
			continue
		}
		seen[info.Name] = true
		if !IsToolGrouped(info.Name) {
			ungrouped = append(ungrouped, info.Name)
		}
	}
	sort.Strings(ungrouped)
	if len(ungrouped) > 0 {
		t.Fatalf("以下工具未纳入 toolGroupMap，将对所有问题始终可见（新增工具请显式分组，或登记到 alwaysVisibleTools 并写明原因）: %v", ungrouped)
	}
	if len(seen) == 0 {
		t.Fatal("未采集到任何工具名，测试自身失效")
	}
}

// 分组裁剪必须真的把无关工具剔除，否则分组只是空转。
func TestFilterToolsByGroupsDropsIrrelevantTools(t *testing.T) {
	initToolGroupsTestDB(t)

	all := GetAllDataTools()
	// 只命中 market 组（大盘/指数）：资金流专用工具应被剔除。
	groups := ClassifyQuestion("今天大盘指数点位怎么样")
	if !groups[GroupMarket] {
		t.Fatalf("market 组未命中，测试前提失效: %v", groups)
	}
	if groups[GroupMoneyFlow] {
		t.Fatalf("money_flow 组不应命中，测试前提失效: %v", groups)
	}

	filtered := FilterToolsByGroups(all, groups)
	if len(filtered) >= len(all) {
		t.Fatalf("分组过滤未削减任何工具: filtered=%d total=%d", len(filtered), len(all))
	}
	for _, tl := range filtered {
		info, err := tl.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		if info.Name == "GetStockMoneyData" {
			t.Fatalf("GetStockMoneyData 属 money_flow 组，不应在仅命中 market 时注入")
		}
	}
}

// initToolGroupsTestDB 数据工具构造链路会读取设置（GetAllDataTools →
// NewStockDataApi → GetSettingConfig），需要 db.Dao 与 settings/ai_config 表。
// DSN 需自带 _pragma=，否则 db.sqliteDSN 会追加第二个 "?" 使参数错乱。
func initToolGroupsTestDB(t *testing.T) {
	t.Helper()
	db.Init("file:tool_groups_test?mode=memory&cache=shared&_pragma=busy_timeout(10000)")
	if err := db.Dao.AutoMigrate(&data.Settings{}, &data.AIConfig{}); err != nil {
		t.Fatalf("迁移 settings/ai_config 表失败: %v", err)
	}
}
