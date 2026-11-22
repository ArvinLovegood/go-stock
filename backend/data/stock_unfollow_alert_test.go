package data

import (
	"database/sql"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestUnFollowDisablesAlertsAndSetTradingPriceClears(t *testing.T) {
	origDao := db.Dao
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开内存 sql.DB 失败: %v", err)
	}
	memDB, err := gorm.Open(sqlite.Dialector{Conn: sqlDB}, &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := memDB.AutoMigrate(&FollowedStock{}, &models.AiRecommendStocks{}, &models.DailyOperationPlan{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	db.Dao = memDB
	t.Cleanup(func() { db.Dao = origDao })

	now := time.Now()
	if err := memDB.Create(&FollowedStock{
		StockCode:       "sh600519",
		Name:            "贵州茅台",
		Time:            now,
		EntryPrice:      1500,
		TakeProfitPrice: 1700,
		StopLossPrice:   1400,
		CostPrice:       1500,
	}).Error; err != nil {
		t.Fatalf("插入自选股失败: %v", err)
	}
	if err := memDB.Create(&models.AiRecommendStocks{
		StockCode:   "600519.SH",
		StockName:   "贵州茅台",
		DataTime:    &now,
		EnableAlert: true,
	}).Error; err != nil {
		t.Fatalf("插入 AI 推荐记录失败: %v", err)
	}
	if err := memDB.Create(&models.DailyOperationPlan{
		PlanDate:    now.Format("2006-01-02"),
		StockCode:   "600519.SH",
		StockName:   "贵州茅台",
		Status:      "pending",
		EnableAlert: true,
	}).Error; err != nil {
		t.Fatalf("插入操作计划失败: %v", err)
	}

	api := NewStockDataApi()
	// 1. 测试 SetTradingPrice 清零
	if res := api.SetTradingPrice(0, 0, 0, 0, "sh600519"); res != "设置成功" {
		t.Fatalf("SetTradingPrice 清零失败: %s", res)
	}
	var fs FollowedStock
	memDB.Where("stock_code = ?", "sh600519").First(&fs)
	if fs.EntryPrice != 0 || fs.TakeProfitPrice != 0 || fs.StopLossPrice != 0 || fs.CostPrice != 0 {
		t.Fatalf("交易价格未清零: %+v", fs)
	}

	// 2. 测试 UnFollow 联动关闭预警 & IsSoftDeletedFollowedStock
	if res := api.UnFollow("sh600519"); res != "取消关注成功" {
		t.Fatalf("UnFollow 失败: %s", res)
	}
	if !IsSoftDeletedFollowedStock("600519.SH") {
		t.Fatal("期望 IsSoftDeletedFollowedStock('600519.SH') 为 true")
	}
	if IsSoftDeletedFollowedStock("sz000001") {
		t.Fatal("未加过自选的股票不应被判为已删除自选")
	}

	var aiRec models.AiRecommendStocks
	memDB.First(&aiRec)
	if aiRec.EnableAlert {
		t.Fatal("UnFollow 后 AI 推荐股票 enable_alert 应被关闭")
	}
	var plan models.DailyOperationPlan
	memDB.First(&plan)
	if plan.EnableAlert {
		t.Fatal("UnFollow 后每日操作计划 enable_alert 应被关闭")
	}
}
