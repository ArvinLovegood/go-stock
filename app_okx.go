//go:build !custom

package main

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

// OKX 交易绑定：供「合约行情 → 模拟交易」页签的人工手动交易界面使用。
// 下单/撤单经由 data.PlaceOKXOrder/CancelOKXOrder，受设置页交易开关与模拟盘开关控制
// （当前配置为模拟盘环境，所有请求带 x-simulated-trading:1，不会触碰实盘资金）。

// OKXTradingOverview 合约行情总览：SWAP 合约规格 + 24h 行情合并下发。
type OKXTradingOverview struct {
	Instruments []data.OKXInstrument `json:"instruments"`
	Tickers     []data.OKXTicker     `json:"tickers"`
}

// GetOKXTradingOverview 拉取当前账户可交易品种（优先私有接口 /api/v5/account/instruments，
// 失败回退公共合约表）：USDT 永续 + USDT 币币现货，规格与行情合并下发（前端本地筛选/排序）。
func (a *App) GetOKXTradingOverview() OKXTradingOverview {
	return okxTradingOverviewFromInsts(data.GetOKXAccountInstruments(""))
}

// GetOKXTradingOverviewEnv 按指定环境拉取可交易品种（simulated=true 模拟盘，false 实盘）。
// 交易面板双环境并行时各取各的账户品种表：模拟盘只有主流币，实盘为全量市场。
func (a *App) GetOKXTradingOverviewEnv(simulated bool) OKXTradingOverview {
	return okxTradingOverviewFromInsts(data.GetOKXAccountInstrumentsForEnv(simulated, ""))
}

// GetOkxInstruments 拉取 OKX 公共品种表（SPOT+SWAP，免签公共接口 + 6h 缓存），前端按需过滤。
// 供 K 线分析页搜索联想使用：与交易总览不同，不请求私有账户接口、不拉 24h 行情，轻量快速；
// OKX 不可达时返回空表（前端独立降级，不影响币安/Bitget 联想）。
func (a *App) GetOkxInstruments() []data.OKXInstrument {
	insts := data.GetOKXInstruments("")
	if insts == nil {
		return []data.OKXInstrument{}
	}
	return insts
}

// SetOKXLeverageEnv 按环境设置合约杠杆（cross 全仓，simulated=true 模拟盘，false 实盘）。
// 仅 SWAP 永续有效；lever 取值 "1"~"125"，空值/现货由后端拒绝。
func (a *App) SetOKXLeverageEnv(simulated bool, instId, lever string) (*data.OKXSetLeverageResult, error) {
	return data.SetOKXLeverageEnv(simulated, instId, lever)
}

// okxTradingOverviewFromInsts 品种过滤 + 行情合并的公共逻辑；账户品种表为空（未配置凭证/请求失败/空表）时回退公共合约表。
func okxTradingOverviewFromInsts(insts []data.OKXInstrument) OKXTradingOverview {
	if len(insts) == 0 {
		insts = data.GetOKXInstruments("")
	}
	if insts == nil {
		insts = []data.OKXInstrument{}
	}
	// 只保留 USDT 计价/结算的可交易品种，与账户可用余额（USDT）匹配
	filtered := make([]data.OKXInstrument, 0, len(insts))
	for _, ins := range insts {
		if ins.State != "" && ins.State != "live" {
			continue
		}
		if strings.EqualFold(ins.SettleCcy, "USDT") || strings.EqualFold(ins.QuoteCcy, "USDT") {
			filtered = append(filtered, ins)
		}
	}
	tickers := append(data.GetOKXTickers("SWAP"), data.GetOKXTickers("SPOT")...)
	if tickers == nil {
		tickers = []data.OKXTicker{}
	}
	return OKXTradingOverview{Instruments: filtered, Tickers: tickers}
}

// GetOKXCandleList 拉取 K 线（bar: 1m/3m/5m/15m/30m/1H/2H/4H/6H/12H/1D/1W/1M）。
func (a *App) GetOKXCandleList(instId, bar string, limit int) []data.OKXCandle {
	candles := data.GetOKXCandles(instId, bar, limit)
	if candles == nil {
		return []data.OKXCandle{}
	}
	return candles
}

// GetOKXInstTickers 按合约类型拉取 24h 行情。
func (a *App) GetOKXInstTickers(instType string) []data.OKXTicker {
	tickers := data.GetOKXTickers(instType)
	if tickers == nil {
		return []data.OKXTicker{}
	}
	return tickers
}

// GetOKXInstTicker 单合约最新行情。
func (a *App) GetOKXInstTicker(instId string) *data.OKXTicker {
	return data.GetOKXTicker(instId)
}

// GetOKXRankings OKX USDT 永续行情榜单（公共数据，模拟盘/实盘面板共用）。
// kind: gainers 涨幅榜 / losers 跌幅榜 / volume 成交额榜 / funding 资金费率榜 / longshort 多空持仓人数比榜；
// limit: 条数（1~50，默认 15）。funding/longshort 覆盖成交额头部合约（60/30）。
func (a *App) GetOKXRankings(kind string, limit int) []data.OKXRankItem {
	return data.GetOKXRankings(kind, limit)
}

// GetOKXWallet 账户余额（总权益 + 币种明细）。
func (a *App) GetOKXWallet() *data.OKXBalance {
	res, err := data.GetOKXBalance()
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXWallet error:%s", err.Error())
		return &data.OKXBalance{Details: []data.OKXBalanceDetail{}}
	}
	return res
}

// GetOKXWalletEnv 账户余额。与 GetOKXWallet 的区别是显式指定环境（simulated=true 模拟盘，
// false 实盘），不读取设置页「模拟盘」全局开关，供交易页签双环境并行查看各自账户。
func (a *App) GetOKXWalletEnv(simulated bool) *data.OKXBalance {
	res, err := data.GetOKXBalanceForEnv(simulated)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXWalletEnv error:%s", err.Error())
		return &data.OKXBalance{Details: []data.OKXBalanceDetail{}}
	}
	return res
}

// GetOKXPositionList 当前持仓（instId 传空 = 全部）。
func (a *App) GetOKXPositionList(instId string) []data.OKXPosition {
	positions, err := data.GetOKXPositions(instId)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXPositionList error:%s", err.Error())
		return []data.OKXPosition{}
	}
	return positions
}

// GetOKXPositionListEnv 当前持仓。与 GetOKXPositionList 的区别是显式指定环境，
// 不读取「模拟盘」全局开关（模拟盘与实盘账户独立，持仓互不可见）。
func (a *App) GetOKXPositionListEnv(simulated bool, instId string) []data.OKXPosition {
	positions, err := data.GetOKXPositionListForEnv(simulated, instId)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXPositionListEnv error:%s", err.Error())
		return []data.OKXPosition{}
	}
	return positions
}

// GetOKXPositionTPSLEnv 当前持仓的止盈止损线视图（与持仓监控同款口径；未开启止盈止损时返回空数组）。
func (a *App) GetOKXPositionTPSLEnv(simulated bool) []data.OKXPositionTPSLView {
	return data.GetOKXPositionTPSLForEnv(simulated)
}

// GetOKXOpenOrders 当前委托（未成交/部分成交）。
func (a *App) GetOKXOpenOrders() []data.OKXOrder {
	orders, err := data.GetOKXPendingOrders("SWAP", "", 50)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXOpenOrders error:%s", err.Error())
		return []data.OKXOrder{}
	}
	return orders
}

// GetOKXOpenOrdersEnv 当前委托。与 GetOKXOpenOrders 的区别是显式指定环境，不走全局开关。
func (a *App) GetOKXOpenOrdersEnv(simulated bool) []data.OKXOrder {
	orders, err := data.GetOKXOpenOrdersForEnv(simulated)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXOpenOrdersEnv error:%s", err.Error())
		return []data.OKXOrder{}
	}
	return orders
}

// GetOKXHistoryOrders 历史委托（最近 100 条，前端按 10 条/页滚动加载）。
func (a *App) GetOKXHistoryOrders() []data.OKXOrder {
	orders, err := data.GetOKXOrderHistory("SWAP", "", 100)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXHistoryOrders error:%s", err.Error())
		return []data.OKXOrder{}
	}
	return orders
}

// GetOKXHistoryOrdersEnv 历史委托。与 GetOKXHistoryOrders 的区别是显式指定环境，不走全局开关。
func (a *App) GetOKXHistoryOrdersEnv(simulated bool) []data.OKXOrder {
	orders, err := data.GetOKXHistoryOrdersForEnv(simulated)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXHistoryOrdersEnv error:%s", err.Error())
		return []data.OKXOrder{}
	}
	return orders
}

// GetOKXFillsHistory 近期成交明细（含每笔成交的已实现盈亏 fillPnl），
// 供交易面板按订单聚合出每笔盈亏/收益率与整体统计。
func (a *App) GetOKXFillsHistory(instType, instId string, limit int) []data.OKXTradeFill {
	fills, err := data.GetOKXFillsHistory(instType, instId, limit)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXFillsHistory error:%s", err.Error())
		return []data.OKXTradeFill{}
	}
	return fills
}

// GetOKXFillsHistoryEnv 近期成交明细。与 GetOKXFillsHistory 的区别是显式指定环境，不走全局开关。
func (a *App) GetOKXFillsHistoryEnv(simulated bool, instType, instId string, limit int) []data.OKXTradeFill {
	fills, err := data.GetOKXFillsHistoryForEnv(simulated, instType, instId, limit)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXFillsHistoryEnv error:%s", err.Error())
		return []data.OKXTradeFill{}
	}
	return fills
}

// SubmitOKXOrder 人工下单（限价/市价，可附带止盈止损）。
// 受「OKX 交易」开关拦截；当前模拟盘开关开启时全部请求走模拟环境。
func (a *App) SubmitOKXOrder(req data.OKXOrderRequest) *data.OKXPlaceOrderResult {
	res, err := data.PlaceOKXOrder(&req)
	if err != nil {
		logger.SugaredLogger.Errorf("SubmitOKXOrder error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// SubmitOKXOrderEnv 人工下单。与 SubmitOKXOrder 的区别是显式指定环境（simulated=true 模拟盘，
// false 实盘真实资金），不读取「模拟盘」全局开关；「OKX 交易」总开关仍照常拦截。
func (a *App) SubmitOKXOrderEnv(simulated bool, req data.OKXOrderRequest) *data.OKXPlaceOrderResult {
	res, err := data.SubmitOKXOrderForEnv(simulated, &req)
	if err != nil {
		logger.SugaredLogger.Errorf("SubmitOKXOrderEnv error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// RevokeOKXOrder 撤销委托。
func (a *App) RevokeOKXOrder(instId, ordId string) *data.OKXPlaceOrderResult {
	res, err := data.CancelOKXOrder(instId, ordId)
	if err != nil {
		logger.SugaredLogger.Errorf("RevokeOKXOrder error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// RevokeOKXOrderEnv 撤销委托。与 RevokeOKXOrder 的区别是显式指定环境，不走全局开关。
func (a *App) RevokeOKXOrderEnv(simulated bool, instId, ordId string) *data.OKXPlaceOrderResult {
	res, err := data.RevokeOKXOrderForEnv(simulated, instId, ordId)
	if err != nil {
		logger.SugaredLogger.Errorf("RevokeOKXOrderEnv error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// CloseOKXPosition 市价平仓：net 模式按 pos 正负判方向；双向持仓按 posSide 判方向。
// 返回下单结果；reduceOnly 保证只减仓不反向开仓。
func (a *App) CloseOKXPosition(instId, posSide, pos string) *data.OKXPlaceOrderResult {
	res, err := data.CloseOKXPositionMarket(instId, posSide, pos)
	if err != nil {
		logger.SugaredLogger.Errorf("CloseOKXPosition error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// CloseOKXPositionEnv 市价平仓。与 CloseOKXPosition 的区别是显式指定环境，不走全局开关。
func (a *App) CloseOKXPositionEnv(simulated bool, instId, posSide, pos string) *data.OKXPlaceOrderResult {
	res, err := data.CloseOKXPositionForEnv(simulated, instId, posSide, pos)
	if err != nil {
		logger.SugaredLogger.Errorf("CloseOKXPositionEnv error:%s", err.Error())
		return &data.OKXPlaceOrderResult{SCode: "1", SMsg: err.Error()}
	}
	return res
}

// boolPtr 保留：其他下单路径仍使用。
func boolPtr(v bool) *bool { return &v }

// GetOKXAccountConfigVM 账户配置（持仓模式：net_mode / long_short_mode，前端下单时决定是否传 posSide）。
func (a *App) GetOKXAccountConfigVM() *data.OKXAccountConfig {
	res, err := data.GetOKXAccountConfig()
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXAccountConfigVM error:%s", err.Error())
		return nil
	}
	return res
}

// GetOKXAccountConfigVMEnv 账户配置。与 GetOKXAccountConfigVM 的区别是显式指定环境，不走全局开关。
func (a *App) GetOKXAccountConfigVMEnv(simulated bool) *data.OKXAccountConfig {
	res, err := data.GetOKXAccountConfigForEnv(simulated)
	if err != nil {
		logger.SugaredLogger.Errorf("GetOKXAccountConfigVMEnv error:%s", err.Error())
		return nil
	}
	return res
}
