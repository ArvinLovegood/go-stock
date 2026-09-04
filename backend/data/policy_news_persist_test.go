package data

import (
	"fmt"
	"os"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// 自建临时库，验证按 URL 去重持久化（不依赖主库 stock.db）
func TestPolicyNewsPersistence(t *testing.T) {
	tmpDB := fmt.Sprintf("%s/policy_news_test_%d.db", os.TempDir(), os.Getpid())
	db.Init(tmpDB)
	if db.Dao == nil {
		t.Skip("无法初始化临时库")
	}
	defer os.Remove(tmpDB)

	count := func() int64 {
		var c int64
		db.Dao.Model(&models.PolicyNews{}).Count(&c)
		return c
	}
	// 用唯一 URL 避免与其他测试数据冲突
	items := []PolicyNewsItem{
		{Title: "测试政策A", Url: "https://example.gov.cn/test-a.htm", Date: "2026-09-04", Source: "测试部"},
		{Title: "测试政策B", Url: "https://example.gov.cn/test-b.htm", Date: "2026-09-03", Source: "测试部"},
	}
	before := count()
	savePolicyNews(items)
	after := count()
	if after-before != 2 {
		t.Fatalf("落库数量不符: before=%d after=%d", before, after)
	}
	// 重复落库不应新增
	savePolicyNews(items)
	dup := count()
	if dup != after {
		t.Fatalf("重复落库未去重: after=%d dup=%d", after, dup)
	}
	// 读取验证
	stored := NewPolicyNewsApi().GetStoredPolicyNews("测试部", "", 1, 10)
	fmt.Printf("读取到 %d 条\n", len(*stored))
	if len(*stored) != 2 {
		t.Fatalf("读取数量不符: %d", len(*stored))
	}
	// 关键词过滤验证
	stored2 := NewPolicyNewsApi().GetStoredPolicyNews("", "测试政策A", 1, 10)
	if len(*stored2) != 1 {
		t.Fatalf("关键词过滤数量不符: %d", len(*stored2))
	}
}
