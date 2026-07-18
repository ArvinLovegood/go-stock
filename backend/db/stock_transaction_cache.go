package db

import (
	"go-stock/backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 缓存有效期：交易时段内 5 分钟，避免频繁请求 gotdx 接口
const TransactionCacheTTL = 5 * time.Minute

// GetStockTransactionCacheMeta 查询某只股票某日的缓存元数据。
func GetStockTransactionCacheMeta(stockCode, tradeDate string) (*models.StockTransactionCacheMeta, error) {
	var meta models.StockTransactionCacheMeta
	err := Dao.Where("stock_code = ? AND trade_date = ?", stockCode, tradeDate).First(&meta).Error
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// IsTransactionCacheExpired 判断缓存是否过期：距上次同步时间超过 TTL 即视为过期。
func IsTransactionCacheExpired(meta *models.StockTransactionCacheMeta) bool {
	if meta == nil {
		return true
	}
	return time.Since(meta.LastSyncTime) > TransactionCacheTTL
}

// GetStockTransactionCache 查询某只股票某日的全部分笔成交缓存（按时间、序号升序）。
func GetStockTransactionCache(stockCode, tradeDate string) ([]models.StockTransactionCache, error) {
	var list []models.StockTransactionCache
	err := Dao.Where("stock_code = ? AND trade_date = ?", stockCode, tradeDate).
		Order("trade_time ASC, seq ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// SaveStockTransactionCache 批量写入分笔成交缓存，并更新元数据。
// 先删除该股票当日旧缓存，再批量插入新数据，最后 upsert 元数据。
func SaveStockTransactionCache(stockCode, tradeDate string, items []models.StockTransactionCache) error {
	if len(items) == 0 {
		return nil
	}
	return Dao.Transaction(func(tx *gorm.DB) error {
		// 删除旧缓存
		if err := tx.Where("stock_code = ? AND trade_date = ?", stockCode, tradeDate).
			Delete(&models.StockTransactionCache{}).Error; err != nil {
			return err
		}
		// 批量插入（每批 1000 条）
		if err := tx.CreateInBatches(items, 1000).Error; err != nil {
			return err
		}
		// upsert 元数据：利用 SQLite 的 ON CONFLICT DO UPDATE（uniqueIndex 命中时更新）
		meta := models.StockTransactionCacheMeta{
			StockCode:    stockCode,
			TradeDate:    tradeDate,
			LastSyncTime: time.Now(),
			TotalCount:   len(items),
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"last_sync_time", "total_count", "updated_at",
			}),
		}).Create(&meta).Error
	})
}

// ClearStockTransactionCache 清除某只股票某日的缓存（含明细和元数据）。
func ClearStockTransactionCache(stockCode, tradeDate string) error {
	return Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("stock_code = ? AND trade_date = ?", stockCode, tradeDate).
			Delete(&models.StockTransactionCache{}).Error; err != nil {
			return err
		}
		return tx.Where("stock_code = ? AND trade_date = ?", stockCode, tradeDate).
			Delete(&models.StockTransactionCacheMeta{}).Error
	})
}

// ClearExpiredStockTransactionCache 清理过期缓存（保留最近 1 天，避免数据库无限增长）。
func ClearExpiredStockTransactionCache() error {
	threshold := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("trade_date < ?", threshold).
			Delete(&models.StockTransactionCache{}).Error; err != nil {
			return err
		}
		return tx.Where("trade_date < ?", threshold).
			Delete(&models.StockTransactionCacheMeta{}).Error
	})
}
