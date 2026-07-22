package data

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAIConfigAutoMigratePreservesLegacyRowsAndAddsProviderPolicyColumns(t *testing.T) {
	dsn := fmt.Sprintf("file:ai-config-migration-%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	legacySchema := `CREATE TABLE ai_config (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT, base_url TEXT, api_key TEXT, model_name TEXT,
        max_tokens INTEGER, temperature REAL, time_out INTEGER,
        http_proxy TEXT, http_proxy_enabled numeric, session_id TEXT, thinking numeric
    )`
	if err := database.Exec(legacySchema).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`INSERT INTO ai_config
        (name, base_url, api_key, model_name, max_tokens, temperature, time_out, thinking)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "legacy-gpt", "http://example.test/v1", "secret", "gpt-5.6-sol", 81920, 0.1, 6000, true).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&AIConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"provider", "max_input_tokens", "max_output_tokens", "temperature_opt", "reasoning_effort"} {
		if !database.Migrator().HasColumn(&AIConfig{}, column) {
			t.Fatalf("migration did not add column %q", column)
		}
	}
	var got AIConfig
	if err := database.First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Name != "legacy-gpt" || got.ApiKey != "secret" || got.MaxTokens != 81920 {
		t.Fatalf("legacy row was not preserved: %+v", got)
	}
	if got.EffectiveMaxInputTokens() != 96000 || got.EffectiveMaxOutputTokens() != 16384 {
		t.Fatalf("legacy row did not receive effective defaults: input=%d output=%d", got.EffectiveMaxInputTokens(), got.EffectiveMaxOutputTokens())
	}
}
