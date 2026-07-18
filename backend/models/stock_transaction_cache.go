package models

import "time"

// StockTransactionCache 分笔成交明细缓存表（gotdx 拉取结果持久化，避免频繁请求接口）。
// 联合唯一索引：(stock_code, trade_date, trade_time, seq) 防止同股票同日重复写入。
type StockTransactionCache struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	StockCode string    `json:"stockCode" gorm:"uniqueIndex:idx_unique_tx;index;size:20"` // 股票代码
	TradeDate string    `json:"tradeDate" gorm:"uniqueIndex:idx_unique_tx;index;size:10"` // 交易日期 YYYY-MM-DD
	TradeTime string    `json:"tradeTime" gorm:"uniqueIndex:idx_unique_tx;size:10"`       // 成交时间 HH:MM:SS
	Seq       int       `json:"seq" gorm:"uniqueIndex:idx_unique_tx"`                     // 同一时间序号（防止 unique 冲突）
	Price     float64   `json:"price"`                                                    // 成交价
	Vol       int64     `json:"vol"`                                                      // 成交量(股)
	Num       int       `json:"num"`                                                      // 笔数
	BuyOrSell int       `json:"buyOrSell"`                                                // 买卖方向 0=买 1=卖 2=中性
	Action    string    `json:"action" gorm:"size:20"`                                    // 方向字符串 BUY/SELL/NEUTRAL
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

// TableName 表名
func (StockTransactionCache) TableName() string {
	return "stock_transaction_cache"
}

// StockTransactionCacheMeta 缓存元数据表（记录每只股票每个交易日的最后同步时间，用于 TTL 判断）。
type StockTransactionCacheMeta struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	StockCode    string    `json:"stockCode" gorm:"uniqueIndex:idx_unique_tx_meta;size:20"` // 股票代码
	TradeDate    string    `json:"tradeDate" gorm:"uniqueIndex:idx_unique_tx_meta;size:10"` // 交易日期 YYYY-MM-DD
	LastSyncTime time.Time `json:"lastSyncTime"`                                            // 最后一次拉取 gotdx 的时间
	TotalCount   int       `json:"totalCount"`                                              // 缓存总条数
	CreatedAt    time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// TableName 表名
func (StockTransactionCacheMeta) TableName() string {
	return "stock_transaction_cache_meta"
}
