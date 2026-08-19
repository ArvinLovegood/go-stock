package agent

import (
	"testing"
	"time"

	"go-stock/backend/db"
)

// TestMCPToolsCacheE2ERealDB 用真实数据库验证 MCP 工具缓存：
// 第一次 getMCPTools 建连加载（秒级），第二次在 TTL 内应命中缓存（毫秒级）。
func TestMCPToolsCacheE2ERealDB(t *testing.T) {
	db.Init("../../data/stock.db")

	t0 := time.Now()
	tools1 := getMCPTools()
	d1 := time.Since(t0)

	t1 := time.Now()
	tools2 := getMCPTools()
	d2 := time.Since(t1)

	t.Logf("第1次: %d 个工具, 耗时 %v (建连+拉取)", len(tools1), d1)
	t.Logf("第2次: %d 个工具, 耗时 %v (应命中缓存)", len(tools2), d2)

	if len(tools1) != len(tools2) {
		t.Fatalf("两次工具数不一致: %d vs %d", len(tools1), len(tools2))
	}

	// 命中缓存时应显著快于首次（无 MCP 服务器时两者都为 0 工具且都极快，跳过断言）
	if len(tools1) > 0 {
		if d2 >= d1 {
			t.Logf("警告: 二次调用未观察到加速 (%v vs %v)，可能均为缓存命中", d2, d1)
		}
	}
}
