package data

import (
	"go-stock/backend/db"
	"strings"
	"testing"
)

func TestIwencaiQueryMarkdownIntegration(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewIwencaiAPI()
	queries := []struct {
		query    string
		contains []string
		excludes []string
	}{
		{
			query:    "贵州茅台最新价",
			contains: []string{"收盘价", "#### 贵州茅台", "开盘价_前复权"},
		},
		{
			query:    "2025年GDP增速",
			contains: []string{"#### GDP同比增长率", "条记录", "2025-12-31"},
			excludes: []string{"macro_id", "只标的"},
		},
		{
			query:    "今日涨停股票",
			contains: []string{"几天几板", "涨停原因", "是"},
			excludes: []string{"[20260703]"},
		},
		{
			query:    "立讯精密机构调研",
			contains: []string{"**内容**", "机构家数", "2026-04-20"},
		},
	}
	for _, tc := range queries {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			md := api.QueryToMarkdown(tc.query, 1, 3)
			if strings.Contains(md, "查询失败") {
				t.Skipf("query failed: %s", md)
			}
			t.Logf("\n%s", md)
			for _, s := range tc.contains {
				if !strings.Contains(md, s) {
					t.Fatalf("expected %q in output", s)
				}
			}
			for _, s := range tc.excludes {
				if strings.Contains(md, s) {
					t.Fatalf("unexpected %q in output", s)
				}
			}
		})
	}
}
