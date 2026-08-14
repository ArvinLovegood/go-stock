package tools

import (
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"os"
	"strings"
	"testing"
)

func TestIwencaiToolRegisteredAndCallable(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test disabled; set GO_STOCK_RUN_INTEGRATION_TESTS=1 to enable")
	}
	db.Init("../../../data/stock.db")
	if !data.IsToolKeyConfigured("QueryIwencai") {
		t.Fatal("QueryIwencai should be enabled after iwencai_api_key is configured")
	}

	var queryTool *DataToolWrapper
	for _, tool := range GetAllDataTools() {
		w, ok := tool.(*DataToolWrapper)
		if !ok || w.name != "QueryIwencai" {
			continue
		}
		queryTool = w
		break
	}
	if queryTool == nil {
		t.Fatal("QueryIwencai tool not registered in agent tools")
	}

	args, _ := json.Marshal(map[string]any{
		"query": "立讯精密最新价",
		"page":  1,
		"limit": 3,
	})
	out, err := queryTool.handler(string(args))
	if err != nil {
		t.Fatalf("QueryIwencai handler error: %v", err)
	}
	t.Logf("QueryIwencai output prefix: %.200s", out)
	if strings.Contains(out, "查询失败") || strings.Contains(out, "API密钥未配置") {
		t.Fatalf("unexpected failure output: %s", out)
	}
	if !strings.Contains(out, "同花顺问财查询结果") {
		t.Fatalf("expected iwencai markdown result, got: %.120s", out)
	}
}
