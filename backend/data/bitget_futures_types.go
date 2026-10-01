package data

import (
	"encoding/json"
	"time"
)

// 本文件定义 Bitget 美股永续合约（RWA，api.bitget.com/api/v2/mix/market）相关的领域结构体。
// 设计约定与币安一致：K线/分时/实时行情复用项目既有的 KLineData / MinuteData / StockInfo，
// 永续特有的资金费率、未平仓量等衍生指标走本文件的独立结构体，
// 不给多数据源共用的 KLineData 增加 Bitget 专属字段。

// BitgetSymbolInfo 单个 Bitget 美股永续合约基础信息。
type BitgetSymbolInfo struct {
	Symbol       string `json:"symbol"`       // Bitget 合约标识，如 AAPLUSDT
	BaseCoin     string `json:"baseCoin"`     // 基础标的（股票代码），如 AAPL
	DisplayName  string `json:"displayName"`  // 展示名，如「苹果/USDT 美股永续」
	PricePlace   string `json:"pricePlace"`   // 价格小数位（接口原样字符串）
	VolumePlace  string `json:"volumePlace"`  // 数量小数位
	FundInterval string `json:"fundInterval"` // 资金费率结算间隔（小时）
	MaxLever     string `json:"maxLever"`     // 最大杠杆
}

// BitgetFuturesInfo /contracts 的解析结果与缓存时间（已过滤为美股 RWA 白名单）。
type BitgetFuturesInfo struct {
	Symbols   []BitgetSymbolInfo
	SymbolMap map[string]BitgetSymbolInfo // key 为大写 symbol
	FetchedAt time.Time
}

// BitgetTicker /ticker 与 /tickers 的 24h 行情。
// 注意：接口不返回 priceChangePercent，只返回 ratio 形式的 change24h（小数），
// 涨跌幅 = change24h * 100；涨跌额 = lastPr - open24h。
type BitgetTicker struct {
	Symbol         string `json:"symbol"`
	LastPr         string `json:"lastPr"`
	BidPr          string `json:"bidPr"`
	BidSz          string `json:"bidSz"`
	AskPr          string `json:"askPr"`
	AskSz          string `json:"askSz"`
	High24h        string `json:"high24h"`
	Low24h         string `json:"low24h"`
	Open24h        string `json:"open24h"`
	Change24h      string `json:"change24h"` // 24h 涨跌，比率形式（小数）
	BaseVolume     string `json:"baseVolume"`
	QuoteVolume    string `json:"quoteVolume"`
	FundingRate    string `json:"fundingRate"`
	MarkPrice      string `json:"markPrice"`
	IndexPrice     string `json:"indexPrice"`
	HoldingAmount  string `json:"holdingAmount"` // 当前持仓量（OI）
	Ts             string `json:"ts"`
}

// BitgetFundingRate 单期资金费率。
// 注意：/history-fund-rate 的 fundingTime 是字符串（如 "1790812800000"），
// 而其它端点多为数字，故自定义反序列化以同时兼容两种形态。
type BitgetFundingRate struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"` // 小数形式字符串
	FundingTime int64  `json:"fundingTime"` // 毫秒
}

// UnmarshalJSON 兼容 fundingTime 的字符串与数字两种返回形态。
func (f *BitgetFundingRate) UnmarshalJSON(b []byte) error {
	raw := struct {
		Symbol      string          `json:"symbol"`
		FundingRate string          `json:"fundingRate"`
		FundingTime json.RawMessage `json:"fundingTime"`
	}{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	f.Symbol = raw.Symbol
	f.FundingRate = raw.FundingRate
	f.FundingTime = bitgetRawInt(raw.FundingTime)
	return nil
}

// BitgetDerivativesBundle 单个合约的衍生指标汇总。
// 与币安不同：Bitget 美股永续**无多空持仓比**、**无 OI 历史端点**，故只有当前值，无历史序列。
type BitgetDerivativesBundle struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`

	LastPrice       float64 `json:"lastPrice"`       // 最新成交价
	MarkPrice       float64 `json:"markPrice"`       // 标记价
	IndexPrice      float64 `json:"indexPrice"`      // 指数价
	Basis           float64 `json:"basis"`           // 基差 (mark-index)/index*100，单位 %
	FundingRate     float64 `json:"fundingRate"`     // 当期资金费率（小数）
	RatePeriod      int     `json:"ratePeriod"`      // 结算周期（小时）
	NextFundingTime int64   `json:"nextFundingTime"` // 下次结算时间（毫秒）
	AnnualizedRate  float64 `json:"annualizedRate"`  // 当期费率年化（%），按 ratePeriod 折算
	OpenInterest    float64 `json:"openInterest"`    // 当前未平仓量（币数/张）
	OpenInterestUSD float64 `json:"openInterestUsd"` // 当前未平仓名义价值（USDT）

	FundingRateHistory []BitgetFundingRate `json:"fundingRateHistory"`
}