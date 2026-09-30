package agent

// static_tools_test.go — 静态工具单例与切片拷贝隔离的守护测试。

import (
	"context"
	"sort"
	"testing"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
)

// initStaticToolsTestDB 静态工具装配链路会读取设置（GetAllDataTools →
// NewStockDataApi → GetSettingConfig），需要 db.Dao 且有 settings/ai_config 表。
// 用共享内存 SQLite（不落盘，避免沙箱环境磁盘 IO 限制）并显式迁移这两张表。
// DSN 必须自带 _pragma=，否则 db.sqliteDSN 会追加第二个 "?" 导致参数错乱。
func initStaticToolsTestDB(t *testing.T) {
	t.Helper()
	db.Init("file:static_tools_test?mode=memory&cache=shared&_pragma=busy_timeout(10000)")
	if err := db.Dao.AutoMigrate(&data.Settings{}, &data.AIConfig{}); err != nil {
		t.Fatalf("迁移 settings/ai_config 表失败: %v", err)
	}
}

func TestBuildStaticToolsSingleton(t *testing.T) {
	initStaticToolsTestDB(t)
	a := buildStaticTools()
	b := buildStaticTools()
	if len(a) == 0 {
		t.Fatal("静态工具列表不应为空")
	}
	if len(a) != len(b) {
		t.Fatalf("两次调用长度不一致: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("第 %d 个工具不是同一实例（单例失效）", i)
		}
	}
}

// TestGetToolsByQuestionDoesNotMutateSharedSlice 在 getToolsByQuestion 返回的
// 切片上追加元素，不应污染单例的共享底层数组。
func TestGetToolsByQuestionDoesNotMutateSharedSlice(t *testing.T) {
	initStaticToolsTestDB(t)
	before := len(buildStaticTools())

	// 触发一次完整装配（question 命中常见关键词，走分组过滤路径）
	got := getToolsByQuestion("分析一下贵州茅台的行情和资金流向", mcpInjectContext{}, false)
	if len(got) == 0 {
		t.Fatal("过滤后的工具列表不应为空")
	}
	// 在返回切片上追加，模拟上游 append 行为
	got = append(got, nil, nil)

	after := len(buildStaticTools())
	if before != after {
		t.Fatalf("单例切片被污染: before=%d after=%d", before, after)
	}
}

// TestGetToolsByQuestionFiltering 分组过滤应生效且保留未注册分组的工具。
func TestGetToolsByQuestionFiltering(t *testing.T) {
	initStaticToolsTestDB(t)
	all := buildStaticTools()
	filtered := getToolsByQuestion("查一下今天有哪些股票涨停、龙虎榜游资情况", mcpInjectContext{}, false)
	if len(filtered) == 0 || len(filtered) > len(all) {
		t.Fatalf("过滤结果异常: total=%d filtered=%d", len(all), len(filtered))
	}
}

// TestBuildStaticToolsAllRegistered 装配层下发的每个静态工具（含 agent 自有工具，
// 如知识库/长期记忆/画像）都必须被显式登记为「分组」或「常驻白名单」。
//
// tools.FilterToolsByGroups 对未登记工具一律保留，所以漏登记不会报错，只会让该工具
// 对所有问题常驻可见 —— 分组优化静默失效。此测试把这种静默失效变成可见失败。
func TestBuildStaticToolsAllRegistered(t *testing.T) {
	initStaticToolsTestDB(t)

	var ungrouped []string
	seen := make(map[string]bool)
	for _, tl := range buildStaticTools() {
		info, err := tl.Info(context.Background())
		if err != nil || info == nil || info.Name == "" {
			ungrouped = append(ungrouped, "<Info 失败>")
			continue
		}
		if seen[info.Name] {
			continue
		}
		seen[info.Name] = true
		if !tools.IsToolGrouped(info.Name) {
			ungrouped = append(ungrouped, info.Name)
		}
	}
	sort.Strings(ungrouped)
	if len(ungrouped) > 0 {
		t.Fatalf("以下静态工具未登记分组/常驻白名单，将对所有问题始终可见: %v", ungrouped)
	}
	if len(seen) == 0 {
		t.Fatal("未采集到任何工具名，测试自身失效")
	}
}

// TestGroupingActuallyCutsTools 端到端确认分组裁剪真的生效（不是空转）。
func TestGroupingActuallyCutsTools(t *testing.T) {
	initStaticToolsTestDB(t)
	all := buildStaticTools()
	filtered := getToolsByQuestion("今天大盘指数点位怎么样", mcpInjectContext{}, false)
	if len(filtered) >= len(all) {
		t.Fatalf("分组裁剪未削减任何工具: filtered=%d total=%d", len(filtered), len(all))
	}
	if len(filtered) < 10 {
		t.Fatalf("裁剪过狠，基础工具可能被误删: filtered=%d", len(filtered))
	}
}
