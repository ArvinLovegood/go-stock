package models

import "time"

// SignalRecord 后台买卖点信号流水。
//
// 信号本身由前端 TypeScript 算法（kline/calc.ts）算出，Go 侧只负责落库与查询：
// 买卖点/TEMA 转折算法只保留一份 TS 实现，移植到 Go 会造成永久双份维护，
// 并与图上箭头逐根漂移。
//
// 唯一索引 (code,klt,family,kind,bar_time) 保证同一根 K 线的同向信号只落一条：
// 前端重复上报（重启后重新扫描、同一 tick 重跑）不会产生重复流水。
type SignalRecord struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Code      string    `json:"code" gorm:"uniqueIndex:idx_signal_unique;size:32"`   // 股票代码
	Klt       string    `json:"klt" gorm:"uniqueIndex:idx_signal_unique;size:8"`     // 周期（101=日K）
	Family    string    `json:"family" gorm:"uniqueIndex:idx_signal_unique;size:16"` // 信号族：buysell / tema
	Kind      string    `json:"kind" gorm:"uniqueIndex:idx_signal_unique;size:8"`    // 方向：buy / sell
	BarTime   int64     `json:"time" gorm:"uniqueIndex:idx_signal_unique"`           // 信号所在 K 线时间
	Name      string    `json:"name" gorm:"size:64"`                                 // 股票名称
	Price     *float64  `json:"price"`                                               // 信号价格；TEMA 转折无价位
	Score     *float64  `json:"score"`                                               // 买卖点共振路数(0-9)；TEMA 转折无
	Detail    string    `json:"detail" gorm:"size:500"`                              // 命中说明
	At        int64     `json:"at" gorm:"index"`                                     // 命中时刻（墙钟毫秒），用于排序与未读角标
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (SignalRecord) TableName() string {
	return "signal_record"
}

// SignalRecordQuery 信号流水分页查询条件。
// Keyword 同时模糊匹配股票代码与名称；时间筛的是信号所在 K 线时间（Unix 秒），0 表示该端不限。
type SignalRecordQuery struct {
	Keyword   string `json:"keyword"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// SignalRecordPageData 信号流水分页结果。
type SignalRecordPageData struct {
	List       []SignalRecord `json:"list"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

// SignalStatQuery 信号操作收益统计的查询条件：按信号所在 K 线时间（Unix 秒）圈定区间。
type SignalStatQuery struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
}

// SignalStatItem 信号操作收益统计：整体为一条，每只股票各一条。
// 口径是「买点开仓、卖点平仓」逐笔配对，同一代码的不同周期各自独立配对，不跨周期混算。
type SignalStatItem struct {
	Code    string  `json:"code"`    // 股票代码；整体统计时为空
	Name    string  `json:"name"`    // 股票名称
	Trades  int     `json:"trades"`  // 已平仓笔数
	Wins    int     `json:"wins"`    // 其中盈利笔数
	WinRate float64 `json:"winRate"` // 胜率（0-1）
	Return  float64 `json:"return"`  // 累计收益率（各笔等权累加，0.012 表示 +1.2%）
	Open    int     `json:"open"`    // 已买入未卖出的笔数（持仓中）
}

// SignalStatResult 信号操作收益统计结果：整体 + 按股票。
type SignalStatResult struct {
	Overall SignalStatItem   `json:"overall"`
	Stocks  []SignalStatItem `json:"stocks"`
}
