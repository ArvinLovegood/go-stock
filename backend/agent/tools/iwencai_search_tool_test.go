package tools

import (
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"strings"
	"testing"
)

func findDataTool(t *testing.T, name string) *DataToolWrapper {
	t.Helper()
	for _, tool := range GetAllDataTools() {
		w, ok := tool.(*DataToolWrapper)
		if !ok || w.name != name {
			continue
		}
		return w
	}
	t.Fatalf("%s tool not registered in agent tools", name)
	return nil
}

func TestIwencaiSearchToolsRegisteredWithoutApiKey(t *testing.T) {
	db.Init("../../../data/stock.db")
	searchTools := []string{"SearchNews", "SearchReport", "SearchInvestor", "SearchAnnouncement"}
	for _, name := range searchTools {
		if !data.IsToolKeyConfigured(name) {
			t.Fatalf("%s should be available without IwencaiApiKey", name)
		}
		findDataTool(t, name)
	}
}

func TestIwencaiSearchNewsToolCallable(t *testing.T) {
	db.Init("../../../data/stock.db")
	searchTool := findDataTool(t, "SearchNews")
	args, _ := json.Marshal(map[string]any{"query": "机器人龙头"})
	out, err := searchTool.handler(string(args))
	if err != nil {
		t.Fatalf("SearchNews handler error: %v", err)
	}
	t.Logf("SearchNews output prefix: %.200s", out)
	if strings.Contains(out, "搜索失败") || strings.Contains(out, "API密钥未配置") {
		t.Fatalf("unexpected failure output: %s", out)
	}
	if !strings.Contains(out, "同花顺问财新闻搜索结果") {
		t.Fatalf("expected iwencai news search markdown, got: %.120s", out)
	}
}

func TestIwencaiSearchReportToolCallable(t *testing.T) {
	db.Init("../../../data/stock.db")
	searchTool := findDataTool(t, "SearchReport")
	args, _ := json.Marshal(map[string]any{"query": "机器人龙头"})
	out, err := searchTool.handler(string(args))
	if err != nil {
		t.Fatalf("SearchReport handler error: %v", err)
	}
	if strings.Contains(out, "搜索失败") || strings.Contains(out, "API密钥未配置") {
		t.Fatalf("unexpected failure output: %s", out)
	}
	if !strings.Contains(out, "同花顺问财研报搜索结果") {
		t.Fatalf("expected iwencai report search markdown, got: %.120s", out)
	}
}

func TestIwencaiSearchAnnouncementToolCallable(t *testing.T) {
	db.Init("../../../data/stock.db")
	searchTool := findDataTool(t, "SearchAnnouncement")
	args, _ := json.Marshal(map[string]any{"query": "机器人龙头"})
	out, err := searchTool.handler(string(args))
	if err != nil {
		t.Fatalf("SearchAnnouncement handler error: %v", err)
	}
	if strings.Contains(out, "搜索失败") || strings.Contains(out, "API密钥未配置") {
		t.Fatalf("unexpected failure output: %s", out)
	}
	if !strings.Contains(out, "同花顺问财公告搜索结果") {
		t.Fatalf("expected iwencai announcement search markdown, got: %.120s", out)
	}
}

func TestIwencaiSearchInvestorToolCallable(t *testing.T) {
	db.Init("../../../data/stock.db")
	searchTool := findDataTool(t, "SearchInvestor")
	args, _ := json.Marshal(map[string]any{"query": "机器人龙头"})
	out, err := searchTool.handler(string(args))
	if err != nil {
		t.Fatalf("SearchInvestor handler error: %v", err)
	}
	if strings.Contains(out, "搜索失败") || strings.Contains(out, "API密钥未配置") {
		t.Fatalf("unexpected failure output: %s", out)
	}
	if !strings.Contains(out, "同花顺问财投资者关系活动搜索结果") {
		t.Fatalf("expected iwencai investor search markdown, got: %.120s", out)
	}
}

func TestIwencaiSearchToolsInNewsResearchGroup(t *testing.T) {
	db.Init("../../../data/stock.db")
	newsGroup := map[ToolGroup]bool{GroupNewsResearch: true}
	allTools := GetAllDataTools()
	filtered := FilterToolsByGroups(allTools, newsGroup)

	names := map[string]bool{}
	for _, tool := range filtered {
		info, err := tool.Info(nil)
		if err != nil {
			t.Fatalf("tool info error: %v", err)
		}
		names[info.Name] = true
	}

	for _, name := range []string{"SearchNews", "SearchReport", "SearchInvestor", "SearchAnnouncement"} {
		if !names[name] {
			t.Fatalf("%s not found in GroupNewsResearch filtered tools", name)
		}
	}
}
