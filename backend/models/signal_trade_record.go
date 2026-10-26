package models

import "time"

// SignalTradeRecord 信号触发的 OKX 自动交易流水。
//
// 来源：信号监控面板（SignalMonitorPanel）上报的买卖点信号经 Go 侧去重后，
// 对加密类代码（ok:/bn:/bt: 前缀或裸合约代码）自动下单。
// 环境跟随设置页「模拟盘」开关：模拟盘（模拟密钥 + x-simulated-trading:1）
// 或实盘（实盘密钥与真实资金），下单时的环境记入 Env 字段。
// 成功与失败都落一条，便于复盘信号策略的实际成交效果。
type SignalTradeRecord struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Code        string    `json:"code" gorm:"index;size:32"`   // 信号代码（如 bn:BTCUSDT）
	InstId      string    `json:"instId" gorm:"size:32"`       // OKX 合约标识（如 BTC-USDT-SWAP）
	InstType    string    `json:"instType" gorm:"size:8"`      // SWAP / SPOT
	Klt         string    `json:"klt" gorm:"size:8"`           // 信号 K 线周期（平仓流水可为空）
	Family      string    `json:"family" gorm:"size:16"`       // 记录族：buysell=开仓 / reverse=信号反向平仓 / tpsl=止盈止损平仓 / swap=满仓换仓平仓
	Kind        string    `json:"kind" gorm:"size:8"`          // buysell: buy/sell；reverse: close；tpsl: tp/sl
	BarTime     int64     `json:"barTime"`                     // 信号所在 K 线时间（Unix 秒）
	SignalPrice float64   `json:"signalPrice"`                 // 信号价格（可能为 0）
	SignalScore float64   `json:"signalScore"`                 // 买卖点共振路数
	Name        string    `json:"name" gorm:"size:64"`         // 标的名称
	Side        string    `json:"side" gorm:"size:8"`          // 下单方向：buy / sell
	OrdType     string    `json:"ordType" gorm:"size:8"`       // 市价 market
	Sz          string    `json:"sz" gorm:"size:32"`           // 下单数量（SWAP 张 / SPOT 基础币）
	RefPrice    float64   `json:"refPrice"`                    // 下单时参考价（最新行情）
	OrdId       string    `json:"ordId" gorm:"size:32"`        // OKX 订单号
	Env         string    `json:"env" gorm:"size:8"`           // 下单时环境：模拟盘 / 实盘
	Status      string    `json:"status" gorm:"size:16;index"` // success / failed / skipped（金额不足最小下单量等正常跳过）
	ErrMsg      string    `json:"errMsg" gorm:"size:300"`      // 失败原因
	AmountUSDT  float64   `json:"amountUsdt"`                  // 计划投入金额（USDT）
	At          int64     `json:"at" gorm:"index"`             // 下单时刻（墙钟毫秒）
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (SignalTradeRecord) TableName() string {
	return "signal_trade_record"
}

// SignalTradeQuery 模拟交易流水分页查询条件。
type SignalTradeQuery struct {
	Keyword   string `json:"keyword"` // 模糊匹配代码/合约标识/名称
	Status    string `json:"status"`  // 空=全部，success/failed
	Env       string `json:"env"`     // 空=全部，模拟盘/实盘（面板按页签环境过滤）
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// SignalTradePageData 模拟交易流水分页结果。
type SignalTradePageData struct {
	List       []SignalTradeRecord `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}
