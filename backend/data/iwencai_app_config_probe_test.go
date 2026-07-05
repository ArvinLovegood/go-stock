package data

import (
	"go-stock/backend/db"
	"strings"
	"testing"
)

// TestIwencaiAppConfigProbe 验证应用内配置（DB settings）与同花顺工具可用性的关系。
func TestIwencaiAppConfigProbe(t *testing.T) {
	db.Init("../../data/stock.db")
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		t.Fatal("settings config is nil")
	}

	dbKey := strings.TrimSpace(cfg.Settings.IwencaiApiKey)
	t.Logf("DB iwencai_api_key configured: %v (len=%d)", dbKey != "", len(dbKey))
	t.Logf("IsToolKeyConfigured(QueryIwencai): %v", IsToolKeyConfigured("QueryIwencai"))
	t.Logf("IsToolKeyConfigured(SelectAStock): %v", IsToolKeyConfigured("SelectAStock"))

	md := NewIwencaiAPI().QueryToMarkdown("贵州茅台最新价", 1, 1)
	t.Logf("QueryToMarkdown via app config prefix: %.120s", md)
	if strings.Contains(md, "API密钥未配置") {
		t.Log("结论: 应用 settings 未填写问财密钥，Agent 会过滤同花顺工具且调用会失败")
	}
	if dbKey == "" && strings.Contains(md, "API密钥未配置") {
		t.Log("修复: 在 设置 -> 问财API密钥 填入有效密钥并保存")
	}
}
