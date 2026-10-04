package tools

import (
	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// 币安 USDT-M 永续合约的 Agent 工具。
// 与 OpenAI 直连模式的同名工具共用 data 层导出纯函数，保证两条链路输出一致。
// 参数 schema 与 backend/data/tools.go 中的定义保持同步。

func GetBinanceFuturesTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"GetBinanceFuturesMarket",
		"获取币安 USDT-M 永续合约行情榜单（24/7 交易的加密资产）。返回合约、最新价、24h涨跌幅、24h成交额、标记价、当期资金费率与下次结算时间。"+
			"symbol 参数支持中文别名容错（如「比特币」「BTC」「btc/usdt」均可）。",
		map[string]*schema.ParameterInfo{
			"sort": {
				Type:     "string",
				Desc:     "排序方式：percent=按24h涨跌幅（默认），amount=按24h成交额，funding=按当期资金费率绝对值。",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "返回条数，默认 20，最大 100。",
				Required: false,
			},
			"symbols": {
				Type:     "string",
				Desc:     "可选，指定合约并忽略排序，如 btc,eth,sol 或 bn:btcusdt，英文逗号分隔。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.BinanceFuturesMarketMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetBinanceFuturesKLine",
		"获取币安 USDT-M 永续合约 K 线数据。注意：币安 K 线以 UTC 为界（日K 为 UTC 00:00），与 A股/港股交易日不同。symbol 支持中文别名。",
		map[string]*schema.ParameterInfo{
			"symbol": {
				Type:     "string",
				Desc:     "合约标识，如 btc、比特币、btc/usdt、BTCUSDT、bn:btcusdt。",
				Required: true,
			},
			"interval": {
				Type:     "string",
				Desc:     "K 线周期：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104、halfYear/半年/105、year/年/106 均归为月K；分钟线：1/5/15/30/60/120/240。",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "K 线根数，默认 90，最大 1500。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.BinanceFuturesKLineMarkdown(args)
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetBinanceFuturesDerivatives",
		"获取币安 USDT-M 永续合约的衍生指标：资金费率（当期/近期均值/年化）、未平仓量 OI（当期与区间变化率）、多空比（全局账户比/大户持仓比/主动买卖比）。"+
			"用于判断杠杆情绪与多空拥挤度。symbol 支持中文别名。",
		map[string]*schema.ParameterInfo{
			"symbol": {
				Type:     "string",
				Desc:     "合约标识，如 btc、比特币、btc/usdt、BTCUSDT、bn:btcusdt。",
				Required: true,
			},
			"period": {
				Type:     "string",
				Desc:     "历史采样周期：5m/15m/30m/1h/2h/4h/6h/12h/1d，默认 1h。",
				Required: false,
			},
			"limit": {
				Type:     "number",
				Desc:     "历史序列条数，默认 48，最大 120。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return data.BinanceFuturesDerivativesMarkdown(args)
		},
	))

	return tools
}
