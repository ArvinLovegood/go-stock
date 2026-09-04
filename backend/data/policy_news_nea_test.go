package data

import (
	"fmt"
	"testing"
)

// TestFetchNeaPolicyNews 验证能源局 datasource JSON 抓取器（需联网）
func TestFetchNeaPolicyNews(t *testing.T) {
	items := fetchNeaPolicyNews(10)
	if len(items) == 0 {
		t.Fatal("能源局抓取结果为空")
	}
	for i, item := range items {
		if i >= 5 {
			break
		}
		fmt.Printf("%s | %s | %s\n", item.Date, item.Title, item.Url)
	}
	if items[0].Source != "国家能源局" {
		t.Fatalf("来源错误: %s", items[0].Source)
	}
	// 标题不应残留 HTML 标签
	for _, item := range items {
		if len(item.Title) > 0 && (item.Title[0] == '<' || containsAngleBracket(item.Title)) {
			t.Fatalf("标题残留 HTML 标签: %s", item.Title)
		}
	}
}

func containsAngleBracket(s string) bool {
	for _, c := range s {
		if c == '<' || c == '>' {
			return true
		}
	}
	return false
}

// TestLoadKeyDepartments 验证重点部门默认列表含数据局/能源局（未自定义时回退默认）
func TestLoadKeyDepartments(t *testing.T) {
	depts := loadKeyDepartments()
	found := map[string]bool{}
	for _, d := range depts {
		found[d] = true
	}
	if !found["国家数据局"] {
		t.Fatal("默认重点部门缺少 国家数据局")
	}
	if !found["国家能源局"] {
		t.Fatal("默认重点部门缺少 国家能源局")
	}
	// 返回副本：修改不应污染默认列表
	depts[0] = "modified"
	if loadKeyDepartments()[0] == "modified" {
		t.Fatal("loadKeyDepartments 返回了默认列表本体（未做副本）")
	}
}
