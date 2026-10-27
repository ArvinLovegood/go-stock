//go:build custom

package main

import (
	"go-stock/backend/data"
)

// 定制版本（wails build -tags custom）不包含 OKX/合约相关功能：
// 实现版见 app_okx.go（//go:build !custom），本文件只保留同名绑定桩，
// 保证 wails 绑定生成与前端 wailsjs 导出完整（前端在定制版中不展示合约相关界面，
// 也不会调用这些方法；桩实现不发起任何 OKX 网络请求，全部返回空数据）。

// OKXTradingOverview 合约行情总览：SWAP 合约规格 + 24h 行情合并下发。
type OKXTradingOverview struct {
	Instruments []data.OKXInstrument `json:"instruments"`
	Tickers     []data.OKXTicker     `json:"tickers"`
}

// GetOKXTradingOverview 拉取当前账户可交易品种（定制版桩：返回空表）。
func (a *App) GetOKXTradingOverview() OKXTradingOverview {
	return OKXTradingOverview{Instruments: []data.OKXInstrument{}, Tickers: []data.OKXTicker{}}
}

// GetOKXTradingOverviewEnv 按指定环境拉取可交易品种（定制版桩：返回空表）。
func (a *App) GetOKXTradingOverviewEnv(simulated bool) OKXTradingOverview {
	return OKXTradingOverview{Instruments: []data.OKXInstrument{}, Tickers: []data.OKXTicker{}}
}

// GetOkxInstruments 拉取 OKX 公共品种表（定制版桩：返回空表）。
func (a *App) GetOkxInstruments() []data.OKXInstrument {
	return []data.OKXInstrument{}
}

// SetOKXLeverageEnv 按环境设置合约杠杆（定制版桩：返回未支持错误）。
func (a *App) SetOKXLeverageEnv(simulated bool, instId, lever string) (*data.OKXSetLeverageResult, error) {
	return &data.OKXSetLeverageResult{}, errCustomBuildNoOKX()
}

// GetOKXCandleList 拉取 K 线（定制版桩：返回空表）。
func (a *App) GetOKXCandleList(instId, bar string, limit int) []data.OKXCandle {
	return []data.OKXCandle{}
}

// GetOKXInstTickers 按合约类型拉取 24h 行情（定制版桩：返回空表）。
func (a *App) GetOKXInstTickers(instType string) []data.OKXTicker {
	return []data.OKXTicker{}
}

// GetOKXInstTicker 单合约最新行情（定制版桩：返回空）。
func (a *App) GetOKXInstTicker(instId string) *data.OKXTicker {
	return &data.OKXTicker{}
}

// GetOKXRankings OKX USDT 永续行情榜单（定制版桩：返回空表）。
func (a *App) GetOKXRankings(kind string, limit int) []data.OKXRankItem {
	return []data.OKXRankItem{}
}

// GetOKXWallet 账户余额（定制版桩：返回空）。
func (a *App) GetOKXWallet() *data.OKXBalance {
	return &data.OKXBalance{Details: []data.OKXBalanceDetail{}}
}

// GetOKXWalletEnv 账户余额（定制版桩：返回空）。
func (a *App) GetOKXWalletEnv(simulated bool) *data.OKXBalance {
	return &data.OKXBalance{Details: []data.OKXBalanceDetail{}}
}

// GetOKXPositionList 当前持仓（定制版桩：返回空表）。
func (a *App) GetOKXPositionList(instId string) []data.OKXPosition {
	return []data.OKXPosition{}
}

// GetOKXPositionListEnv 当前持仓（定制版桩：返回空表）。
func (a *App) GetOKXPositionListEnv(simulated bool, instId string) []data.OKXPosition {
	return []data.OKXPosition{}
}

// GetOKXPositionTPSLEnv 当前持仓的止盈止损线视图（定制版桩：返回空表）。
func (a *App) GetOKXPositionTPSLEnv(simulated bool) []data.OKXPositionTPSLView {
	return []data.OKXPositionTPSLView{}
}

// GetOKXOpenOrders 当前委托（定制版桩：返回空表）。
func (a *App) GetOKXOpenOrders() []data.OKXOrder {
	return []data.OKXOrder{}
}

// GetOKXOpenOrdersEnv 当前委托（定制版桩：返回空表）。
func (a *App) GetOKXOpenOrdersEnv(simulated bool) []data.OKXOrder {
	return []data.OKXOrder{}
}

// GetOKXHistoryOrders 历史委托（定制版桩：返回空表）。
func (a *App) GetOKXHistoryOrders() []data.OKXOrder {
	return []data.OKXOrder{}
}

// GetOKXHistoryOrdersEnv 历史委托（定制版桩：返回空表）。
func (a *App) GetOKXHistoryOrdersEnv(simulated bool) []data.OKXOrder {
	return []data.OKXOrder{}
}

// GetOKXFillsHistory 近期成交明细（定制版桩：返回空表）。
func (a *App) GetOKXFillsHistory(instType, instId string, limit int) []data.OKXTradeFill {
	return []data.OKXTradeFill{}
}

// GetOKXFillsHistoryEnv 近期成交明细（定制版桩：返回空表）。
func (a *App) GetOKXFillsHistoryEnv(simulated bool, instType, instId string, limit int) []data.OKXTradeFill {
	return []data.OKXTradeFill{}
}

// SubmitOKXOrder 人工下单（定制版桩：返回不支持）。
func (a *App) SubmitOKXOrder(req data.OKXOrderRequest) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// SubmitOKXOrderEnv 人工下单（定制版桩：返回不支持）。
func (a *App) SubmitOKXOrderEnv(simulated bool, req data.OKXOrderRequest) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// RevokeOKXOrder 撤销委托（定制版桩：返回不支持）。
func (a *App) RevokeOKXOrder(instId, ordId string) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// RevokeOKXOrderEnv 撤销委托（定制版桩：返回不支持）。
func (a *App) RevokeOKXOrderEnv(simulated bool, instId, ordId string) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// CloseOKXPosition 市价平仓（定制版桩：返回不支持）。
func (a *App) CloseOKXPosition(instId, posSide, pos string) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// CloseOKXPositionEnv 市价平仓（定制版桩：返回不支持）。
func (a *App) CloseOKXPositionEnv(simulated bool, instId, posSide, pos string) *data.OKXPlaceOrderResult {
	return &data.OKXPlaceOrderResult{SCode: "1", SMsg: errCustomBuildNoOKX().Error()}
}

// GetOKXAccountConfigVM 账户配置（定制版桩：返回空）。
func (a *App) GetOKXAccountConfigVM() *data.OKXAccountConfig {
	return &data.OKXAccountConfig{}
}

// GetOKXAccountConfigVMEnv 账户配置（定制版桩：返回空）。
func (a *App) GetOKXAccountConfigVMEnv(simulated bool) *data.OKXAccountConfig {
	return &data.OKXAccountConfig{}
}
