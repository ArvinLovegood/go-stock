package data

import (
	"path/filepath"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestGetAIResponseResultSkipsAutomatedAnalysisRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ai_response_result.db")
	db.Init(dbPath)

	if err := db.Dao.Create(&models.AIResponseResult{
		StockCode: "sh600000",
		StockName: "trusted",
		ChatId:    "user-chat-1",
		ModelName: "model-a",
		Content:   "trusted analysis",
	}).Error; err != nil {
		t.Fatalf("create trusted row: %v", err)
	}
	if err := db.Dao.Create(&models.AIResponseResult{
		StockCode: "sh600000",
		StockName: "poisoned",
		ChatId:    "auto:cron-stock",
		ModelName: "model-a",
		Content:   "poisoned analysis",
	}).Error; err != nil {
		t.Fatalf("create auto row: %v", err)
	}
	if err := db.Dao.Create(&models.AIResponseResult{
		StockCode: "sh600001",
		StockName: "auto-only",
		ChatId:    "auto:cron-market",
		ModelName: "model-a",
		Content:   "poisoned only",
	}).Error; err != nil {
		t.Fatalf("create auto-only row: %v", err)
	}

	result := (&OpenAi{}).GetAIResponseResult("sh600000")
	if result == nil || result.Content != "trusted analysis" || result.ChatId != "user-chat-1" {
		t.Fatalf("expected trusted result, got %+v", result)
	}

	autoOnly := (&OpenAi{}).GetAIResponseResult("sh600001")
	if autoOnly == nil {
		t.Fatal("expected non-nil result for auto-only lookup")
	}
	if autoOnly.ID != 0 || autoOnly.Content != "" || autoOnly.ChatId != "" {
		t.Fatalf("expected automated row to be skipped, got %+v", autoOnly)
	}
}
