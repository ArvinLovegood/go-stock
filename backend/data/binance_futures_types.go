package data

import "time"

// 本文件定义币安 USDT-M 永续合约（fapi.binance.com）相关的领域结构体。
// 设计约定：K线/分时/实时行情复用项目既有的 KLineData / MinuteData / StockInfo，
// 永续特有的资金费率、未平仓量、多空比等衍生指标走本文件的独立结构体，
// 避免给多数据源共用的 KLineData 增加币安专属字段。

// BinanceSymbolInfo 单个 USDT-M 永续合约的基础信息。
type BinanceSymbolInfo struct {
	Symbol         string `json:"symbol"`      // 币安合约标识，如 BTCUSDT
	BaseAsset      string `json:"baseAsset"`   // 基础币种，如 BTC / AAPL
	DisplayName    string `json:"displayName"` // 展示名，如「比特币/USDT 永续」
	PricePrecision int    `json:"pricePrecision"`
	OnboardDate    int64  `json:"onboardDate"` // 上线时间（毫秒）
	IsTradFi       bool   `json:"isTradFi"`    // 是否 TradFi 永续（contractType=TRADIFI_PERPETUAL，含美股/ETF/大宗商品等）
}

// BinanceExchangeInfo exchangeInfo 的解析结果与缓存时间。
type BinanceExchangeInfo struct {
	Symbols   []BinanceSymbolInfo
	SymbolMap map[string]BinanceSymbolInfo // key 为大写 symbol
	FetchedAt time.Time
}

// BinanceFundingRate 单期资金费率。
type BinanceFundingRate struct {
	Symbol      string `json:"symbol"`
	FundingTime int64  `json:"fundingTime"` // 毫秒
	FundingRate string `json:"fundingRate"` // 小数形式字符串，如 "0.00010000"
}

// BinanceOpenInterestHist 未平仓量历史点。
type BinanceOpenInterestHist struct {
	Symbol               string `json:"symbol"`
	SumOpenInterest      string `json:"sumOpenInterest"`      // 币数
	SumOpenInterestValue string `json:"sumOpenInterestValue"` // 名义价值（USDT）
	Timestamp            int64  `json:"timestamp"`
}

// BinanceLongShortRatio 多空比历史点。
// 不同端点返回的字段不同：globalLongShortAccountRatio 返回 longAccount/shortAccount，
// takerlongshortRatio 返回 buyVol/sellVol，未涉及的字段留空。
type BinanceLongShortRatio struct {
	Symbol         string `json:"symbol"`
	LongShortRatio string `json:"longShortRatio"` // 多空比
	LongAccount    string `json:"longAccount"`    // 多头账户占比
	ShortAccount   string `json:"shortAccount"`   // 空头账户占比
	BuySellRatio   string `json:"buySellRatio"`   // 主动买卖比
	BuyVol         string `json:"buyVol"`
	SellVol        string `json:"sellVol"`
	Timestamp      int64  `json:"timestamp"`
}

// BinanceDerivativesPoint 通用时序点（用于 OI 与多空比曲线）。
type BinanceDerivativesPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// BinanceDerivativesBundle 单个合约的衍生指标汇总（实时 + 历史序列）。
type BinanceDerivativesBundle struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`

	// 实时
	MarkPrice         float64 `json:"markPrice"`         // 标记价
	IndexPrice        float64 `json:"indexPrice"`        // 指数价
	LastPrice         float64 `json:"lastPrice"`         // 最新成交价
	Basis             float64 `json:"basis"`             // 基差 (mark-index)/index*100，单位 %
	LastFundingRate   float64 `json:"lastFundingRate"`   // 当期资金费率（小数）
	NextFundingTime   int64   `json:"nextFundingTime"`   // 下次结算时间（毫秒）
	AnnualizedRate    float64 `json:"annualizedRate"`    // 当期费率年化（%，按 8h 一期 *3*365）
	OpenInterest      float64 `json:"openInterest"`      // 未平仓量（币数）
	OpenInterestValue float64 `json:"openInterestValue"` // 未平仓名义价值（USDT）
	LongShortRatio    float64 `json:"longShortRatio"`    // 全局多空账户比
	LongAccount       float64 `json:"longAccount"`       // 多头账户占比
	ShortAccount      float64 `json:"shortAccount"`      // 空头账户占比
	TakerBuySellRatio float64 `json:"takerBuySellRatio"` // 主动买卖量比
	OpenInterestChange float64 `json:"openInterestChange"` // 区间 OI 变化率（%）

	// 历史序列
	FundingRateHistory  []BinanceFundingRate      `json:"fundingRateHistory"`
	OpenInterestHistory []BinanceDerivativesPoint `json:"openInterestHistory"`
	LongShortHistory    []BinanceDerivativesPoint `json:"longShortHistory"`
}