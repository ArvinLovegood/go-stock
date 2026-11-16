package data

import (
	"path/filepath"
	"testing"

	"go-stock/backend/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTdxUnusualSettingPersistence(t *testing.T) {
	dao, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "settings.db"),
	}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := db.Dao
	db.Dao = dao
	sqlDB, err := dao.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Dao = old
		_ = sqlDB.Close()
	})
	if err := dao.AutoMigrate(&Settings{}, &AIConfig{}); err != nil {
		t.Fatal(err)
	}
	settings := &Settings{}
	if err := dao.Create(settings).Error; err != nil {
		t.Fatal(err)
	}
	if settings.TdxUnusualNotify {
		t.Fatal("notification must default to disabled")
	}
	config := &SettingConfig{Settings: settings}
	for _, enabled := range []bool{true, false} {
		config.TdxUnusualNotify = enabled
		if result := UpdateConfig(config); result != "保存成功！" {
			t.Fatalf("save failed: %s", result)
		}
		var saved Settings
		if err := dao.First(&saved, settings.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.TdxUnusualNotify != enabled {
			t.Fatalf("saved %v, want %v", saved.TdxUnusualNotify, enabled)
		}
	}
}
