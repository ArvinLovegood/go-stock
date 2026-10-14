package tools

import (
	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// OKX（欧易）交易的 Agent 工具。
// 与 OpenAI 直连模式的同名工具共用 data 层导出函数，保证两条链路输出一致。
// 参数 schema 与 backend/data/tools.go 中的定义保持同步。
// 下单/撤单受设置页「OKX 交易」开关控制；未配置凭证时由 IsToolKeyConfigured 过滤。

func GetOKXTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"GetOKXAccount",
		"查询用户 OKX（欧易）账户余额：总权益（USD）、各币种权益/可用/冻结与持仓模式。" +
			"用 env 参数选择账户环境：sim=模拟盘（默认），live=实盘（真实资金）。模拟盘与实盘是两套独立密钥与独立账户。" +
			"需先在设置页配置对应环境的 OKX API 凭证。当用户问「我的 OKX 余额/账户/资产/权益」时使用。",
		map[string]*schema.ParameterInfo{
			"env": {
				Type:     "string",
				Desc:     "账户环境：sim=模拟盘（默认），live=实盘（真实资金，只读查询不产生交易）。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.OKXAccountMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetOKXPositions",
		"查询用户 OKX（欧易）当前持仓：品种、方向（多/空）、数量（SWAP 以张计）、开仓均价、标记价、" +
			"未实现盈亏、收益率、杠杆、保证金模式与预估强平价。用 env 参数选择账户环境：sim=模拟盘（默认），live=实盘（真实资金）。" +
			"两环境账户独立、持仓互不可见。当用户问「我的 OKX 持仓/仓位/浮盈浮亏」时使用。",
		map[string]*schema.ParameterInfo{
			"instId": {
				Type:     "string",
				Desc:     "可选，品种标识（如 btc、BTC-USDT-SWAP、BTC-USDT），为空查全部持仓。",
				Required: false,
			},
			"env": {
				Type:     "string",
				Desc:     "账户环境：sim=模拟盘（默认），live=实盘（真实资金，只读查询不产生交易）。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.OKXPositionsMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetOKXOrders",
		"查询用户 OKX（欧易）订单：待成交委托或历史订单（含成交均价、成交量、状态）。" +
			"用 env 参数选择账户环境：sim=模拟盘（默认），live=实盘（真实资金），两环境订单互不可见。",
		map[string]*schema.ParameterInfo{
			"status": {
				Type:     "string",
				Desc:     "pending=待成交（默认），history=历史订单（含已成交/已撤销）。",
				Required: false,
			},
			"instType": {
				Type:     "string",
				Desc:     "可选，SPOT=币币现货，SWAP=永续合约（默认），不传查两类。",
				Required: false,
			},
			"instId": {
				Type:     "string",
				Desc:     "可选，品种标识（如 btc、BTC-USDT-SWAP），为空查全部。",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "返回条数，默认 20，最大 100。",
				Required: false,
			},
			"env": {
				Type:     "string",
				Desc:     "账户环境：sim=模拟盘（默认），live=实盘（真实资金，只读查询不产生交易）。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.OKXOrdersMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"PlaceOKXOrder",
		"在用户 OKX（欧易）账户下单，交易环境由 env 参数显式指定：sim=模拟盘（默认，不动真实资金），" +
			"live=实盘（真实资金！下单前必须与用户二次确认）。支持 SPOT 现货与 SWAP 永续、限价/市价、附带止损止盈。" +
			"注意：SWAP 数量单位是「张」（一张=ctVal 个币），SPOT 数量单位是基础币数量；" +
			"下单前应与用户确认品种/方向/数量/价格/环境，并先查余额或行情辅助决策。需在设置页开启「OKX 交易」开关。",
		map[string]*schema.ParameterInfo{
			"instId": {
				Type:     "string",
				Desc:     "品种标识，如 btc、BTC-USDT-SWAP（永续）、BTC-USDT（现货）。",
				Required: true,
			},
			"env": {
				Type:     "string",
				Desc:     "交易环境：sim=模拟盘（默认），live=实盘（真实资金，谨慎！下单前必须与用户确认）。",
				Required: false,
			},
			"instType": {
				Type:     "string",
				Desc:     "可选，SPOT 或 SWAP；不传时按标识自动判断（默认 SWAP）。",
				Required: false,
			},
			"side": {
				Type:     "string",
				Desc:     "buy=买入/开多，sell=卖出/开空。",
				Required: true,
			},
			"sz": {
				Type:     "string",
				Desc:     "下单数量：SWAP 为张数（如 1），SPOT 为基础币数量（如 0.01）。",
				Required: true,
			},
			"ordType": {
				Type:     "string",
				Desc:     "limit=限价（默认，需传 px），market=市价。",
				Required: false,
			},
			"px": {
				Type:     "string",
				Desc:     "限价单价格；市价单不传。",
				Required: false,
			},
			"tdMode": {
				Type:     "string",
				Desc:     "可选保证金模式：cash=现货（默认），cross=全仓（SWAP 默认），isolated=逐仓。",
				Required: false,
			},
			"posSide": {
				Type:     "string",
				Desc:     "可选持仓方向：net（默认，单向持仓），long/short（双向持仓模式必须传）。",
				Required: false,
			},
			"reduceOnly": {
				Type:     "string",
				Desc:     "可选，true=只减仓（平仓不反向开新仓）。",
				Required: false,
			},
			"slTriggerPx": {
				Type:     "string",
				Desc:     "可选，止损触发价（市价止损）。",
				Required: false,
			},
			"tpTriggerPx": {
				Type:     "string",
				Desc:     "可选，止盈触发价（市价止盈）。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.OKXPlaceOrderMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"CancelOKXOrder",
		"撤销用户 OKX（欧易）账户的待成交订单，用 env 参数选择环境：sim=模拟盘（默认），live=实盘（真实资金）。" +
			"两环境订单互不可见，撤单必须用下单时相同的环境。需提供品种与订单号（可先调用 GetOKXOrders status=pending 获取）。",
		map[string]*schema.ParameterInfo{
			"instId": {
				Type:     "string",
				Desc:     "品种标识，如 BTC-USDT-SWAP。",
				Required: true,
			},
			"ordId": {
				Type:     "string",
				Desc:     "订单号（GetOKXOrders 返回的订单号字段）。",
				Required: true,
			},
			"env": {
				Type:     "string",
				Desc:     "交易环境：sim=模拟盘（默认），live=实盘（真实资金）。须与下单时的环境一致。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.OKXCancelOrderMarkdown(args)
		},
	))

	return tools
}
