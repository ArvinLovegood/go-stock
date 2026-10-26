package data

import (
	"encoding/json"
	"errors"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"strings"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

// DefaultPromptPlazaApiBase 广场服务默认接口域名（定制版不允许修改）。
// 提示词/技能广场、分析报告分享等所有客户端↔服务端交互统一走此地址。
const DefaultPromptPlazaApiBase = "https://proxy.sparkmemory.top/api"

// PromptPlazaApiFallbacks 广场服务回退地址：默认地址网络层不可用（超时/连接失败）时按顺序回退。
var PromptPlazaApiFallbacks = []string{
	"https://go-stock.sparkmemory.top/api",
	"https://stock.sparkmemory.top/api",
}

// PromptPlazaApiBases 返回广场请求的候选地址列表：默认地址优先，其余按顺序兜底。
// primary 为配置值（定制版固定等于默认地址），为空时忽略；重复地址去重，避免重复请求。
func PromptPlazaApiBases(primary string) []string {
	bases := []string{DefaultPromptPlazaApiBase}
	if primary = strings.TrimSpace(primary); primary != "" {
		bases = append(bases, primary)
	}
	bases = append(bases, PromptPlazaApiFallbacks...)
	return lo.Uniq(bases)
}

type Settings struct {
	gorm.Model
	TushareToken     string `json:"tushareToken"`
	LocalPushEnable  bool   `json:"localPushEnable"`
	DingPushEnable   bool   `json:"dingPushEnable"`
	DingRobot        string `json:"dingRobot"`
	FeishuPushEnable bool   `json:"feishuPushEnable"`
	FeishuRobot      string `json:"feishuRobot"`
	FeishuSecret     string `json:"feishuSecret" gorm:"column:feishu_secret"`
	// 飞书应用机器人（接收消息+AI回复，长连接模式，与 FeishuPush 自定义机器人推送独立）
	FeishuBotEnable        bool   `json:"feishuBotEnable"`
	FeishuAppID            string `json:"feishuAppId" gorm:"column:feishu_app_id"`
	FeishuAppSecret        string `json:"feishuAppSecret" gorm:"column:feishu_app_secret"`
	FeishuBotAiConfigId    int    `json:"feishuBotAiConfigId" gorm:"column:feishu_bot_ai_config_id"`
	FeishuBotSysPromptId   int    `json:"feishuBotSysPromptId" gorm:"column:feishu_bot_sys_prompt_id"`
	FeishuBotEnableTools   bool   `json:"feishuBotEnableTools"`
	FeishuBotThinking      bool   `json:"feishuBotThinking"`
	FeishuBotMemoryEnable  bool   `json:"feishuBotMemoryEnable" gorm:"column:feishu_bot_memory_enable"`
	FeishuBotAgentMode     string `json:"feishuBotAgentMode" gorm:"column:feishu_bot_agent_mode"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`
	OpenAiEnable           bool   `json:"openAiEnable"`
	Prompt                 string `json:"prompt"`
	CheckUpdate            bool   `json:"checkUpdate"`
	UpdateChannel          string `json:"updateChannel"`
	QuestionTemplate       string `json:"questionTemplate"`
	CrawlTimeOut           int64  `json:"crawlTimeOut"`
	KDays                  int64  `json:"kDays"`
	EnableDanmu            bool   `json:"enableDanmu"`
	BrowserPath            string `json:"browserPath"`
	EnableNews             bool   `json:"enableNews"`
	DarkTheme              bool   `json:"darkTheme"`
	BrowserPoolSize        int    `json:"browserPoolSize"`
	EnableFund             bool   `json:"enableFund"`
	EnablePushNews         bool   `json:"enablePushNews"`
	EnableOnlyPushRedNews  bool   `json:"enableOnlyPushRedNews"`
	SponsorCode            string `json:"sponsorCode"`
	HttpProxy              string `json:"httpProxy"`
	HttpProxyEnabled       bool   `json:"httpProxyEnabled"`
	// HttpProxyMode 全局 HttpProxy 的作用范围：overseas=仅国外站点与数据走代理（默认，
	// 国内数据源保持直连）；global=所有请求走代理（旧行为）。空值按 overseas 处理。
	HttpProxyMode string `json:"httpProxyMode" gorm:"column:http_proxy_mode"`
	// BinanceProxy 币安 USDT-M 永续合约专用 HTTP 代理（如 http://127.0.0.1:7890）。
	// 与全局 HttpProxy 完全独立：仅作用于币安合约数据源（国内直连 fapi.binance.com 常被阻断），
	// 不影响 A股/港美股等国内数据源的直连行为。留空表示币安不使用代理。
	BinanceProxy string `json:"binanceProxy" gorm:"column:binance_proxy"`
	// BitgetProxy Bitget 美股永续合约专用 HTTP 代理（如 http://127.0.0.1:10809）。
	// 与全局 HttpProxy、币安 BinanceProxy 三者相互独立：仅作用于 Bitget 美股永续数据源
	// （国内直连 api.bitget.com 常被阻断），不影响 A股/港美股/币安的既有行为。留空表示不使用代理。
	BitgetProxy string `json:"bitgetProxy" gorm:"column:bitget_proxy"`
	// EnableContracts 控制「合约行情」独立页面与菜单项的显隐：展示币安 USDT-M 永续（加密）
	// 与 Bitget 美股永续（RWA）全部合约。默认开启，可在设置页关闭。
	EnableContracts bool `json:"enableContracts" gorm:"column:enable_contracts;default:true"`
	// OKX 交易 API 凭证（欧易）。签名认证需要 APIKey + SecretKey + Passphrase（创建密钥时设置的「口令」，
	// 非备注名）。密钥需在 OKX 后台绑定本机出口 IP 白名单，否则私有接口会返回 IP 不可用错误。
	OKXApiKey     string `json:"okxApiKey" gorm:"column:okx_api_key"`
	OKXSecretKey  string `json:"okxSecretKey" gorm:"column:okx_secret_key"`
	OKXPassphrase string `json:"okxPassphrase" gorm:"column:okx_passphrase"`
	// OKX 实盘 API 凭证。OKX 模拟盘与实盘的 API 密钥是两套（模拟盘创建的密钥仅模拟环境可用，
	// 实盘创建的密钥仅真实环境可用），因此分开保存：上面三个字段作为「模拟盘密钥」，
	// 下面三个为「实盘密钥」，请求时按 OKXSimulated 开关自动选用；实盘密钥同样需绑定 IP 白名单。
	OKXLiveApiKey     string `json:"okxLiveApiKey" gorm:"column:okx_live_api_key"`
	OKXLiveSecretKey  string `json:"okxLiveSecretKey" gorm:"column:okx_live_secret_key"`
	OKXLivePassphrase string `json:"okxLivePassphrase" gorm:"column:okx_live_passphrase"`
	// OKXEnableTrade 交易开关：关闭（默认）时 AI 仅能查询余额/持仓/订单，
	// 下单与撤单请求直接拒绝；开启后 AI 助手才可执行交易操作（环境跟随模拟盘开关）。
	OKXEnableTrade bool `json:"okxEnableTrade" gorm:"column:okx_enable_trade;default:false"`
	// OKXSimulated 模拟盘（全局环境开关）：开启（默认）后所有 OKX 交易请求携带
	// x-simulated-trading:1 头并使用模拟盘密钥，走 OKX 模拟交易环境；
	// 关闭即切换为实盘——手动交易、信号自动下单、止盈止损监控全部使用实盘密钥与真实资金。
	OKXSimulated bool `json:"okxSimulated" gorm:"column:okx_simulated;default:true"`
	// OKXSignalTradeEnable 信号自动下单·模拟盘开关：开启后信号监控面板上报的加密买卖点信号
	//（ok:/bn:/bt: 前缀，buysell 族）会自动在 OKX 模拟环境按市价下单，结果落 signal_trade_record 流水。
	// 与 OKXSignalTradeLive 相互独立，可同时开启——同一信号在两个环境各下一笔。
	OKXSignalTradeEnable bool `json:"okxSignalTradeEnable" gorm:"column:okx_signal_trade_enable;default:false"`
	// OKXSignalTradeUSDT 模拟盘每笔信号投入金额（USDT），按最新市价换算为张数/币数，默认 100。
	OKXSignalTradeUSDT string `json:"okxSignalTradeUSDT" gorm:"column:okx_signal_trade_usdt;size:16"`
	// OKXSignalTradeLive 信号自动下单·实盘开关：开启后加密买卖点信号会自动在 OKX 实盘环境
	// 按市价下单，使用实盘密钥与真实资金，请谨慎开启；与模拟盘开关相互独立、可同时启用。
	// 股票永续信号不受此开关影响（其开关 OKXSignalStockEnable 暂仅走模拟盘语义不变）。
	OKXSignalTradeLive bool `json:"okxSignalTradeLive" gorm:"column:okx_signal_trade_live;default:false"`
	// OKXSignalTradeLiveUSDT 实盘每笔信号投入金额（USDT），独立于模拟盘金额，默认 100。
	OKXSignalTradeLiveUSDT string `json:"okxSignalTradeLiveUSDT" gorm:"column:okx_signal_trade_live_usdt;size:16"`
	// OKXAutoPoolEnable 自动调池·模拟盘确认：趋势跟随引擎的全局总闸之一——开启后「OKX 模拟盘池」的
	// 趋势跟随（定时扫描自动入池、失趋势移除）才允许执行。
	// 与监控面板「趋势跟随」开关是双重约束，二者都开才生效；模拟盘不动真实资金。
	// 趋势跟随只调池、绝不主动平仓：失趋势但仍有持仓的品种暂留池中，平仓由买卖点信号驱动。
	OKXAutoPoolEnable bool `json:"okxAutoPoolEnable" gorm:"column:okx_auto_pool_enable;default:false"`
	// OKXAutoPoolLive 自动调池·实盘确认：开启后「OKX 实盘池」的趋势跟随才允许执行（影响实盘监控池，
	// 务必充分评估后开启）；与面板开关双重约束。趋势跟随不下单、不碰仓位——实盘真实资金风险仍全部
	// 由「实盘自动下单」及其风控参数承担；失趋势且有持仓的品种同样暂留池中待信号平仓。
	OKXAutoPoolLive bool `json:"okxAutoPoolLive" gorm:"column:okx_auto_pool_live;default:false"`
	// OKXSignalTradeAmountMode 信号下单金额模式：fixed=固定金额（默认，用上面两个金额字段）；
	// ratio=按下单环境账户可用余额（USDT）比例开仓，比例见 OKXSignalTradeAmountRatio。
	// 模拟盘/实盘共用同一模式；股票信号固定金额不受影响。
	OKXSignalTradeAmountMode string `json:"okxSignalTradeAmountMode" gorm:"column:okx_signal_trade_amount_mode;size:8"`
	// OKXSignalTradeAmountRatio 仓位比例（ratio 模式）：0.25=1/4 仓、0.3333=1/3 仓、0.5=半仓、1=全仓。
	// 口径为「名义价值」（开仓市值）：本笔名义 = 可用 USDT × 比例，不随杠杆放大；
	// 10x 杠杆下实际占用保证金 = 名义/10（如 1/3 仓 ≈ 动用 3.3% 余额作保证金）。
	OKXSignalTradeAmountRatio string `json:"okxSignalTradeAmountRatio" gorm:"column:okx_signal_trade_amount_ratio;size:16"`
	// OKXSignalTradeMinScore 信号质量门槛：买卖点共振路数（满分 14，等权计分）低于该值的信号
	// 不自动下单（模拟盘/实盘/股票信号统一生效）；留空或 0=不限制。如 4 表示只交易 ≥4 路共振信号
	//（对应信号面板「标准」灵敏度档，低分路信号噪音大，实测胜率边际薄）。
	OKXSignalTradeMinScore string `json:"okxSignalTradeMinScore" gorm:"column:okx_signal_trade_min_score;size:8"`
	// OKXSignalReverseMinScore 反手开仓·分数门槛：信号刚平掉反向仓（reverse 平仓成交）后的
	// 反手开仓须满足的最低共振路数——反手是换向鞭打的高发通道（实盘反手新腿胜率仅 18%、
	// 78% 换向双亏），分数不够时只平仓不反手。留空 = 默认 6（与高分预留同档）；显式 0 = 不限制。
	OKXSignalReverseMinScore string `json:"okxSignalReverseMinScore" gorm:"column:okx_signal_reverse_min_score;size:8"`
	// OKXSignalTradeCooldownMin TP/SL 平仓后同合约同方向的开仓冷却分钟数（防震荡市反复止损收割）；
	// 留空 = 默认 30 分钟，显式 0 = 关闭冷却。
	OKXSignalTradeCooldownMin string `json:"okxSignalTradeCooldownMin" gorm:"column:okx_signal_trade_cooldown_min;size:8"`
	// OKXSignalMinTpPct 开单最低止盈收益率（%）：信号开仓前按当前止盈止损配置估算本笔预期止盈收益率
	//（账户保证金口径，与止盈线同口径：fixed=止盈收益率；vol=平均振幅×止盈倍数×生效杠杆，杠杆未知按 10x 兜底估算），
	// 低于该值则放弃开单——低波动合约算出的止盈线过近，双边手续费+滑点即可吃掉全部毛利。
	// 仅 SWAP 生效；止盈止损未开启或未配置止盈时无法估算、不拦截。留空或 0=不限制。如 5 表示预期止盈不足 +5% 就不开单。
	OKXSignalMinTpPct string `json:"okxSignalMinTpPct" gorm:"column:okx_signal_min_tp_pct;size:16"`
	// OKXSignalMaxTpPct 开单最大止盈收益率（%）：与 OKXSignalMinTpPct 同口径、同开仓门槛，
	// 预期止盈收益率高于该值则放弃开单——波动率自适应模式下振幅过大意味着止损线同样过远，
	// 单笔最大回撤远超常规，可用于过滤极端行情/妖币合约。留空或 0=不限制。
	// 如 50 表示预期止盈超过 +50% 就不开单。
	OKXSignalMaxTpPct string `json:"okxSignalMaxTpPct" gorm:"column:okx_signal_max_tp_pct;size:16"`
	// OKXRiskMaxPositions 防爆仓·最大并发持仓数：当前环境账户非零持仓 ≥ 该值时拒绝开新仓；
	// 空或非法取默认 3；0=不限制。
	OKXRiskMaxPositions string `json:"okxRiskMaxPositions" gorm:"column:okx_risk_max_positions;size:8"`
	// OKXRiskMaxMarginRatio 防爆仓·最大保证金率（%）：维持保证金合计/总权益 ≥ 阈值时拒绝开新仓，
	// 直接对应强平距离（TPSL 按单笔收益率兜不住总仓位过重）；空或非法取默认 30；0=不限制。
	OKXRiskMaxMarginRatio string `json:"okxRiskMaxMarginRatio" gorm:"column:okx_risk_max_margin_ratio;size:16"`
	// OKXRiskReservedSlots 防爆仓·高分预留槽位数：当前持仓进入「接近满仓」区间
	//（占用 ≥ 最大持仓数 - 预留槽位）时，信号分数低于 OKXRiskHighScore 的信号不再开新仓，
	// 最后几个槽位只留给高分信号——防止低分小仓把槽位占满后高分信号吃闭门羹（劣币驱逐良币）。
	// 空或非法取默认 2；0=关闭预留（任何分数都能用满全部槽位）。
	OKXRiskReservedSlots string `json:"okxRiskReservedSlots" gorm:"column:okx_risk_reserved_slots;size:8"`
	// OKXRiskHighScore 高分预留槽位对应的信号分数门槛：接近满仓时只有共振路数 ≥ 该值的信号
	// 才允许占用预留槽位开新仓。空或非法取默认 6。
	OKXRiskHighScore string `json:"okxRiskHighScore" gorm:"column:okx_risk_high_score;size:8"`
	// OKXRiskSwapGap 满仓换仓·分差门槛：满仓时高分信号可平掉在场最弱浮亏仓腾槽换入，
	// 新信号分数 − 最弱持仓入场分数 < 该值时不触发（边际优势须付得起平仓+开仓双份手续费）；
	// 空或非法取默认 3；0=关闭满仓换仓。
	OKXRiskSwapGap string `json:"okxRiskSwapGap" gorm:"column:okx_risk_swap_gap;size:8"`
	// OKXRiskSwapLossPct 满仓换仓·截亏线(%)：仅当最弱持仓浮动收益率（账户口径）≤ −该值时
	// 才允许被换出腾槽——只截亏、不卖盈利仓追高；空或非法取默认 5；0=关闭满仓换仓。
	OKXRiskSwapLossPct string `json:"okxRiskSwapLossPct" gorm:"column:okx_risk_swap_loss_pct;size:8"`
	// OKXRiskSwapMaxPerDay 满仓换仓·单日次数上限（按环境计数）：超限后满仓只落 skipped
	// 不再拆仓；空或非法取默认 2；0=不限制。
	OKXRiskSwapMaxPerDay string `json:"okxRiskSwapMaxPerDay" gorm:"column:okx_risk_swap_max_per_day;size:8"`
	// OKXSignalTradeInstType 信号下单品种：SWAP=永续合约（默认）/ SPOT=币币现货。
	OKXSignalTradeInstType string `json:"okxSignalTradeInstType" gorm:"column:okx_signal_trade_inst_type;size:8"`
	// OKXSignalTradeLever 信号下单杠杆（仅 SWAP）：每笔下单前按环境设置合约杠杆（cross 全仓），
	// 取值 1~125；留空=不设置（沿用账户当前杠杆）。设置失败时该笔拒绝下单（宁可漏不误）。
	OKXSignalTradeLever string `json:"okxSignalTradeLever" gorm:"column:okx_signal_trade_lever;size:8"`
	// OKXSignalTradeKlts 允许交易的 K 线周期（逗号分隔，如 15,60,101）；留空=全部周期。
	// 周期含义与信号监控面板一致：1/5/15/30/60 分钟，101=日K，102=周K，103=月K。
	OKXSignalTradeKlts string `json:"okxSignalTradeKlts" gorm:"column:okx_signal_trade_klts;size:64"`
	// OKXSignalTradeDirMode 开仓方向：both=多空都做（默认）/ long=只做多（卖出信号仅平多）/ short=只做空。
	// 5m/15m 实测「只做多」在下跌季度回撤更小；配合 OKXSignalTradeRegime 可按大势自动切换方向。
	OKXSignalTradeDirMode string `json:"okxSignalTradeDirMode" gorm:"column:okx_signal_trade_dir_mode;size:8"`
	// OKXSignalTradeRegime 顺大势门控：开启后按 BTC 1H（EMA50/EMA200 + 收盘位置）判断趋势态，
	// 多头趋势只开多、空头趋势只开空、过渡区不限制。回测：15m ≥6 路信号年化 -59%→-28%，是全部组合最优。
	OKXSignalTradeRegime bool `json:"okxSignalTradeRegime" gorm:"column:okx_signal_trade_regime;default:false"`
	// OKXSignalTradeMaxPerDay 单合约单日最大开仓笔数（东八区自然日，按成功开仓流水统计）；
	// 留空或 0 = 不限制。5m 裸信号实测日均 4.5 笔/品种，手续费是主要亏损来源，限笔数最直接有效。
	OKXSignalTradeMaxPerDay string `json:"okxSignalTradeMaxPerDay" gorm:"column:okx_signal_trade_max_per_day;size:8"`
	// OKXSignalStockEnable 股票信号（美股/港股永续等合成股票合约代码）单独开关：
	// 与加密信号交易完全独立（独立开关+独立金额），默认关闭——OKX 模拟盘未上架股票永续，
	// 仅实盘 xStocks 类股票永续上线后才有实际成交。
	OKXSignalStockEnable bool `json:"okxSignalStockEnable" gorm:"column:okx_signal_stock_enable;default:false"`
	// OKXSignalStockUSDT 股票信号每笔交易金额（USDT），默认 100。
	OKXSignalStockUSDT string `json:"okxSignalStockUSDT" gorm:"column:okx_signal_stock_usdt;size:16"`
	// OKXTPSLEnable 每笔持仓止盈止损监控开关：开启后后台每 10 秒检查一次全部 OKX 持仓，
	// 任一持仓的未实现收益率达到阈值立即市价 reduceOnly 平仓。跟随当前凭证环境（模拟盘开关开启则只动模拟账户）。
	OKXTPSLEnable bool `json:"okxTpSlEnable" gorm:"column:okx_tpsl_enable;default:false"`
	// OKXTakeProfitPct 止盈收益率（%）：未实现收益率 ≥ 该值时市价平仓，如 5 表示 +5%；0/留空=不启用止盈。
	OKXTakeProfitPct string `json:"okxTakeProfitPct" gorm:"column:okx_take_profit_pct;size:16"`
	// OKXStopLossPct 止损收益率（%）：未实现收益率 ≤ -该值时市价平仓，如 3 表示 -3%；0/留空=不启用止损。
	OKXStopLossPct string `json:"okxStopLossPct" gorm:"column:okx_stop_loss_pct;size:16"`
	// OKXTPSLMode 阈值模式：fixed=固定收益率（默认，用上面两个百分比）；vol=波动率自适应
	//（按各合约近 N 根参考周期 K 线的平均振幅动态计算：止盈线=振幅×止盈倍数、止损线=振幅×止损倍数，
	// 收益口径为价格收益（未实现盈亏/开仓名义价值），与杠杆无关）。
	OKXTPSLMode string `json:"okxTpSlMode" gorm:"column:okx_tpsl_mode;size:8"`
	// OKXTPSLVolKlt 波动率参考 K 线周期（如 15m / 1H / 1D），默认 15m。
	OKXTPSLVolKlt string `json:"okxTpSlVolKlt" gorm:"column:okx_tpsl_vol_klt;size:8"`
	// OKXTPSLVolBars 波动率回看 K 线根数，默认 20。
	OKXTPSLVolBars string `json:"okxTpSlVolBars" gorm:"column:okx_tpsl_vol_bars;size:8"`
	// OKXTPSLTpMult 止盈倍数：止盈线 = 平均振幅 × 该倍数，默认 1.5。
	OKXTPSLTpMult string `json:"okxTpSlTpMult" gorm:"column:okx_tpsl_tp_mult;size:16"`
	// OKXTPSLSlMult 止损倍数：止损线 = 平均振幅 × 该倍数，默认 1。
	OKXTPSLSlMult string `json:"okxTpSlSlMult" gorm:"column:okx_tpsl_sl_mult;size:16"`
	EnableAgent   bool   `json:"enableAgent"`
	QgqpBId       string `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	IwencaiApiKey string `json:"iwencaiApiKey" gorm:"column:iwencai_api_key"`
	EmApiKey      string `json:"emApiKey" gorm:"column:em_api_key"`
	// DeepSeekSearchApiKey DeepSeek 联网搜索/网页抓取 API Key（api.deepseek.com），
	// 为 Agent 的 WebSearch/WebFetch 工具提供鉴权，按次计费。
	DeepSeekSearchApiKey string `json:"deepSeekSearchApiKey" gorm:"column:deepseek_search_api_key"`
	WindowWidth          int    `json:"windowWidth"`
	WindowHeight         int    `json:"windowHeight"`
	PromptPlazaApiBase   string `json:"promptPlazaApiBase" gorm:"column:prompt_plaza_api_base"`
	// LongTermMemoryAiConfigId 长期记忆向量检索绑定的 AIConfig ID。
	// 0=自动模式（优先 ModelType=embedding 的服务）；>0=用指定 AIConfig。
	// 用于让用户明确指定长期记忆用哪个向量服务，避免自动选错。
	LongTermMemoryAiConfigId int `json:"longTermMemoryAiConfigId" gorm:"column:long_term_memory_ai_config_id;default:0"`
	// ProfileLearnAiConfigId 用户画像学习（重新学习/纠正即学习）绑定的 AIConfig ID。
	// 0=自动模式（取第一个可用的对话模型）；>0=用指定 AIConfig。
	// 画像学习涉及归纳与增量调和，建议指定能力较强的模型。
	ProfileLearnAiConfigId int `json:"profileLearnAiConfigId" gorm:"column:profile_learn_ai_config_id;default:0"`
	// SignalScanIntervalSec 信号扫描节拍间隔（秒）：Go cron 唤醒前端信号引擎的周期，默认 60。
	// 不进 UpdateConfig 的字段表（设置页表单不含它），只能经 SetSignalScanInterval 单字段 API 修改，
	// 避免主设置页提交时被零值静默重置（同 long_term_memory_ai_config_id 的处理方式）。
	SignalScanIntervalSec int `json:"signalScanIntervalSec" gorm:"column:signal_scan_interval_sec;default:60"`
}

// GetSignalScanIntervalSec 读取信号扫描节拍间隔（秒）；未配置或非法（<=0 / 超界）时回退默认值。
// 下限 1 秒：面板扫描周期提供 1/5/10 秒档位，加密 7x24 行情下高频档位由此放行；
// 1 秒档仅作为唤醒节拍，实际节奏由前端 running 锁与按周期节流保证（一轮扫描跑不满周期时自动跳过多余 tick）。
func GetSignalScanIntervalSec() int {
	sec := 60
	var v int
	if err := db.Dao.Model(&Settings{}).Select("signal_scan_interval_sec").Limit(1).Scan(&v).Error; err == nil && v > 0 {
		sec = v
	}
	if sec < 1 {
		sec = 1
	}
	if sec > 600 {
		sec = 600
	}
	return sec
}

// SetSignalScanIntervalSec 持久化信号扫描节拍间隔（秒）。
func SetSignalScanIntervalSec(sec int) error {
	return db.Dao.Model(&Settings{}).Where("id=?", NewSettingsApi().Config.ID).UpdateColumn("signal_scan_interval_sec", sec).Error
}

func (receiver Settings) TableName() string {
	return "settings"
}

type AIConfig struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `json:"name"`
	BaseUrl   string `json:"baseUrl"`
	ApiKey    string `json:"apiKey" `
	ModelName string `json:"modelName"`
	// ModelType 模型类型："chat"=文本对话模型（默认），"embedding"=向量模型。
	// 同一提供商的对话与向量接口地址可能不同，故拆分为独立 AIConfig 条目。
	// type=embedding 时，ModelName 即向量模型名（如 text-embedding-3-small / BAAI/bge-m3）。
	ModelType string `json:"modelType" gorm:"column:model_type;default:'chat'"`
	MaxTokens int    `json:"maxTokens"`
	// ContextWindow 模型上下文窗口大小（输入+输出总容量）。
	// 由 FetchAiModelInfo 从模型 API 的 max_context_length/context_length 自动获取，
	// 或从内置模型表推导。用于摘要中间件和消息压缩的 token 预算计算。
	// 为 0 时运行时按内置表/MaxTokens/默认值兜底（向后兼容旧配置）。
	ContextWindow    int     `json:"contextWindow" gorm:"column:context_window"`
	Temperature      float64 `json:"temperature"`
	TimeOut          int     `json:"timeOut"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	SessionId        string  `json:"sessionId" gorm:"index;size:64"`
	Thinking         bool    `json:"thinking"`
	// SupportVision 模型是否支持视觉理解（图片输入，OpenAI 兼容 image_url 内容块）。
	// 开启后 AI 助手对话可发送图片（base64 data URL 或外部图片 URL），
	// 适用于 DeepSeek-Vision、GLM-4V、Qwen-VL 等多模态模型。
	SupportVision bool `json:"supportVision" gorm:"column:support_vision;default:false"`
	// ExtraHeaders 自定义 HTTP 请求头（JSON 格式字符串，如 {"x-team-id":"...","x-agent-id":"..."}）。
	// 支持模板变量：{{sessionId}}（会话ID）、{{uuid}}（每次请求生成新UUID）。
	// 用于对接需携带额外 Header 的代理/网关（如腾讯云 TencentDB-Agent-Memory / CodeBuddy Proxy）。
	ExtraHeaders string `json:"extraHeaders" gorm:"type:text"`
	// EmbeddingModel 长期记忆向量检索使用的 embedding 模型名（OpenAI 兼容 /v1/embeddings 接口）。
	// 留空时默认 "text-embedding-3-small"；中文供应商可填其支持的模型名（如 Qwen 的 text-embedding-v3）。
	// 仅 backend/agent/long_term_memory.go 使用，与对话模型 ModelName 独立。
	EmbeddingModel string `json:"embeddingModel" gorm:"column:embedding_model"`
}

func (AIConfig) TableName() string {
	return "ai_config"
}

type SettingConfig struct {
	*Settings
	AiConfigs []*AIConfig `json:"aiConfigs"`
}

func (c *SettingConfig) GetAIConfigThinking(aiConfigId int) bool {
	if aiConfigId <= 0 && len(c.AiConfigs) > 0 {
		return c.AiConfigs[0].Thinking
	}
	for _, cfg := range c.AiConfigs {
		if int(cfg.ID) == aiConfigId {
			return cfg.Thinking
		}
	}
	return false
}

type SettingsApi struct {
	Config *SettingConfig
}

func NewSettingsApi() *SettingsApi {
	return &SettingsApi{
		Config: GetSettingConfig(),
	}
}

func (s *SettingsApi) Export() string {
	d, _ := json.MarshalIndent(s.Config, "", "    ")
	return string(d)
}

func UpdateConfig(s *SettingConfig) string {
	if s.Settings == nil {
		return "保存失败: 配置数据为空"
	}
	count := int64(0)
	db.Dao.Model(&Settings{}).Count(&count)
	if count > 0 {
		result := db.Dao.Model(&Settings{}).Where("id=?", s.ID).Updates(map[string]any{
			"local_push_enable":             s.LocalPushEnable,
			"ding_push_enable":              s.DingPushEnable,
			"ding_robot":                    s.DingRobot,
			"feishu_push_enable":            s.FeishuPushEnable,
			"feishu_robot":                  s.FeishuRobot,
			"feishu_secret":                 s.FeishuSecret,
			"feishu_bot_enable":             s.FeishuBotEnable,
			"feishu_app_id":                 s.FeishuAppID,
			"feishu_app_secret":             s.FeishuAppSecret,
			"feishu_bot_ai_config_id":       s.FeishuBotAiConfigId,
			"feishu_bot_sys_prompt_id":      s.FeishuBotSysPromptId,
			"feishu_bot_enable_tools":       s.FeishuBotEnableTools,
			"feishu_bot_thinking":           s.FeishuBotThinking,
			"feishu_bot_memory_enable":      s.FeishuBotMemoryEnable,
			"feishu_bot_agent_mode":         s.FeishuBotAgentMode,
			"update_basic_info_on_start":    s.UpdateBasicInfoOnStart,
			"refresh_interval":              s.RefreshInterval,
			"open_ai_enable":                s.OpenAiEnable,
			"tushare_token":                 s.TushareToken,
			"prompt":                        s.Prompt,
			"check_update":                  s.CheckUpdate,
			"update_channel":                s.UpdateChannel,
			"question_template":             s.QuestionTemplate,
			"crawl_time_out":                s.CrawlTimeOut,
			"k_days":                        s.KDays,
			"enable_danmu":                  s.EnableDanmu,
			"browser_path":                  s.BrowserPath,
			"enable_news":                   s.EnableNews,
			"dark_theme":                    s.DarkTheme,
			"enable_fund":                   s.EnableFund,
			"enable_push_news":              s.EnablePushNews,
			"enable_only_push_red_news":     s.EnableOnlyPushRedNews,
			"sponsor_code":                  s.SponsorCode,
			"http_proxy":                    s.HttpProxy,
			"http_proxy_enabled":            s.HttpProxyEnabled,
			"http_proxy_mode":               s.HttpProxyMode,
			"binance_proxy":                 s.BinanceProxy,
			"bitget_proxy":                  s.BitgetProxy,
			"enable_contracts":              s.EnableContracts,
			"okx_api_key":                   s.OKXApiKey,
			"okx_secret_key":                s.OKXSecretKey,
			"okx_passphrase":                s.OKXPassphrase,
			"okx_live_api_key":              s.OKXLiveApiKey,
			"okx_live_secret_key":           s.OKXLiveSecretKey,
			"okx_live_passphrase":           s.OKXLivePassphrase,
			"okx_enable_trade":              s.OKXEnableTrade,
			"okx_simulated":                 s.OKXSimulated,
			"okx_signal_trade_enable":       s.OKXSignalTradeEnable,
			"okx_signal_trade_usdt":         s.OKXSignalTradeUSDT,
			"okx_signal_trade_live":         s.OKXSignalTradeLive,
			"okx_signal_trade_live_usdt":    s.OKXSignalTradeLiveUSDT,
			"okx_auto_pool_enable":          s.OKXAutoPoolEnable,
			"okx_auto_pool_live":            s.OKXAutoPoolLive,
			"okx_signal_trade_amount_mode":  s.OKXSignalTradeAmountMode,
			"okx_signal_trade_amount_ratio": s.OKXSignalTradeAmountRatio,
			"okx_risk_max_positions":        s.OKXRiskMaxPositions,
			"okx_risk_max_margin_ratio":     s.OKXRiskMaxMarginRatio,
			"okx_risk_reserved_slots":       s.OKXRiskReservedSlots,
			"okx_risk_high_score":           s.OKXRiskHighScore,
			"okx_risk_swap_gap":             s.OKXRiskSwapGap,
			"okx_risk_swap_loss_pct":        s.OKXRiskSwapLossPct,
			"okx_risk_swap_max_per_day":     s.OKXRiskSwapMaxPerDay,
			"okx_signal_trade_inst_type":    s.OKXSignalTradeInstType,
			"okx_signal_trade_lever":        s.OKXSignalTradeLever,
			"okx_signal_trade_klts":         s.OKXSignalTradeKlts,
			"okx_signal_trade_min_score":    s.OKXSignalTradeMinScore,
			"okx_signal_reverse_min_score":  s.OKXSignalReverseMinScore,
			"okx_signal_trade_cooldown_min": s.OKXSignalTradeCooldownMin,
			"okx_signal_min_tp_pct":         s.OKXSignalMinTpPct,
			"okx_signal_max_tp_pct":         s.OKXSignalMaxTpPct,
			"okx_signal_trade_dir_mode":     s.OKXSignalTradeDirMode,
			"okx_signal_trade_regime":       s.OKXSignalTradeRegime,
			"okx_signal_trade_max_per_day":  s.OKXSignalTradeMaxPerDay,
			"okx_signal_stock_enable":       s.OKXSignalStockEnable,
			"okx_signal_stock_usdt":         s.OKXSignalStockUSDT,
			"okx_tpsl_enable":               s.OKXTPSLEnable,
			"okx_take_profit_pct":           s.OKXTakeProfitPct,
			"okx_stop_loss_pct":             s.OKXStopLossPct,
			"okx_tpsl_mode":                 s.OKXTPSLMode,
			"okx_tpsl_vol_klt":              s.OKXTPSLVolKlt,
			"okx_tpsl_vol_bars":             s.OKXTPSLVolBars,
			"okx_tpsl_tp_mult":              s.OKXTPSLTpMult,
			"okx_tpsl_sl_mult":              s.OKXTPSLSlMult,
			"enable_agent":                  s.EnableAgent,
			"qgqp_b_id":                     s.QgqpBId,
			"iwencai_api_key":               s.IwencaiApiKey,
			"em_api_key":                    s.EmApiKey,
			"deepseek_search_api_key":       s.DeepSeekSearchApiKey,
			"window_width":                  s.WindowWidth,
			"window_height":                 s.WindowHeight,
			"prompt_plaza_api_base":         DefaultPromptPlazaApiBase, // 固定值，不信任提交内容
			// 注意：long_term_memory_ai_config_id / profile_learn_ai_config_id 不在此更新。
			// 主设置页（settings.vue）构造的表单不含这两个字段，提交时为零值，
			// 若在此写库会把知识库页/画像页设置的绑定服务静默重置为 0（自动模式）。
			// 它们只能通过专用单字段 API 修改：
			//   KnowledgeBaseApi.SetLongTermMemoryAiConfigId / UserProfileApi.SetProfileLearnAiConfigId
		})
		if result.Error != nil {
			logger.SugaredLogger.Errorf("更新配置失败: %v", result.Error)
			return "保存失败: " + result.Error.Error()
		}

		err := updateAiConfigs(s.AiConfigs)
		if err != nil {
			logger.SugaredLogger.Errorf("更新AI模型服务配置失败: %v", err)
			return "更新AI模型服务配置失败: " + err.Error()
		}
	} else {
		result := db.Dao.Model(&Settings{}).Create(s.Settings)
		if result.Error != nil {
			logger.SugaredLogger.Error("创建配置失败:", result.Error)
			return "创建配置失败: " + result.Error.Error()
		}
		err := updateAiConfigs(s.AiConfigs)
		if err != nil {
			logger.SugaredLogger.Errorf("更新AI模型服务配置失败: %v", err)
			return "更新AI模型服务配置失败: " + err.Error()
		}
	}

	ConfigureFromSettings(s)

	return "保存成功！"
}

func updateAiConfigs(aiConfigs []*AIConfig) error {
	// nil 表示调用方不希望更新 AI 配置（保留现有配置）；
	// 空 slice（len==0）才表示清空所有 AI 配置
	if aiConfigs == nil {
		return nil
	}
	if len(aiConfigs) == 0 {
		err := db.Dao.Exec("DELETE FROM ai_config").Error
		if err != nil {
			return err
		}
		return db.Dao.Exec("DELETE FROM sqlite_sequence WHERE name='ai_config'").Error
	}
	// 仅收集大于 0 的 ID，用于识别已存在的配置；
	// ID<=0 视为“新配置”，强制走插入逻辑，避免多个 ID 为 0 的配置互相覆盖。
	var ids []uint
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	})
	var existAiConfigs []*AIConfig
	err := db.Dao.Model(&AIConfig{}).Select("id").Where("id in (?) ", ids).Find(&existAiConfigs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	idMap := make(map[uint]bool)
	lo.ForEach(existAiConfigs, func(item *AIConfig, index int) {
		idMap[item.ID] = true
	})
	var addAiConfigs []*AIConfig
	var notDeleteIds []uint
	var e error
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if e != nil {
			return
		}
		// ID<=0 一律视为新配置，走插入逻辑；否则根据是否已存在决定更新或新增
		if item.ID <= 0 || !idMap[item.ID] {
			addAiConfigs = append(addAiConfigs, item)
		} else {
			notDeleteIds = append(notDeleteIds, item.ID)
			e = db.Dao.Model(&AIConfig{}).Where("id=?", item.ID).Updates(map[string]interface{}{
				"name":               item.Name,
				"base_url":           item.BaseUrl,
				"api_key":            item.ApiKey,
				"model_name":         item.ModelName,
				"max_tokens":         item.MaxTokens,
				"context_window":     item.ContextWindow,
				"temperature":        item.Temperature,
				"time_out":           item.TimeOut,
				"http_proxy":         item.HttpProxy,
				"http_proxy_enabled": item.HttpProxyEnabled,
				"session_id":         item.SessionId,
				"thinking":           item.Thinking,
				"support_vision":     item.SupportVision,
				"extra_headers":      item.ExtraHeaders,
				"embedding_model":    item.EmbeddingModel,
				"model_type":         item.ModelType,
			}).Error
			if e != nil {
				return
			}
		}
	})
	if e != nil {
		return e
	}
	//删除旧的配置
	if len(notDeleteIds) > 0 {
		err = db.Dao.Exec("DELETE FROM ai_config WHERE id NOT IN ?", notDeleteIds).Error
		if err != nil {
			return err
		}
	}
	//logger.SugaredLogger.Infof("更新aiConfigs +%d", len(addAiConfigs))
	//批量新增的配置
	err = db.Dao.CreateInBatches(addAiConfigs, len(addAiConfigs)).Error
	return err
}

// UpdateAiConfigsOnly 仅更新 AI 模型服务配置，不影响其他设置项
// 供独立的 AI 模型服务管理页面调用，避免覆盖 settings 表中的其他字段
func UpdateAiConfigsOnly(aiConfigs []*AIConfig) string {
	if err := updateAiConfigs(aiConfigs); err != nil {
		logger.SugaredLogger.Errorf("更新AI配置失败: %v", err)
		return "保存失败: " + err.Error()
	}
	// 刷新内存中的配置缓存
	ConfigureFromSettings(GetSettingConfig())
	return "保存成功！"
}

func GetSettingConfig() *SettingConfig {
	settingConfig := &SettingConfig{}
	settings := &Settings{}
	aiConfigs := make([]*AIConfig, 0)
	// 处理数据库查询可能返回的空结果
	settingsResult := db.Dao.Model(&Settings{}).First(settings)
	// 新用户无设置记录时，默认启用暗黑主题与 AI 诊股
	if errors.Is(settingsResult.Error, gorm.ErrRecordNotFound) {
		settings.DarkTheme = true
		settings.OpenAiEnable = true
		settings.EnableContracts = true
	}
	// AI 配置始终查询，不依赖 OpenAiEnable 开关：
	// AI 配置管理页面、飞书机器人、AI 助手等独立功能可能在 OpenAiEnable=false 时也需要读取已保存的配置
	result := db.Dao.Model(&AIConfig{}).Find(&aiConfigs)
	if result.Error != nil {
		logger.SugaredLogger.Error("查询AI配置失败:", result.Error)
	} else if len(aiConfigs) > 0 {
		lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
			if item.TimeOut <= 0 {
				item.TimeOut = 60 * 5
			}
		})
	}
	if settings.OpenAiEnable {
		if settings.CrawlTimeOut <= 0 {
			settings.CrawlTimeOut = 60
		}
		if settings.KDays < 30 {
			settings.KDays = 60
		}
	}
	if settings.BrowserPath == "" {
		settings.BrowserPath, _ = CheckBrowser()
	}
	if settings.BrowserPoolSize <= 0 {
		settings.BrowserPoolSize = 1
	}
	settings.EnableAgent = false
	// 广场服务地址固定（定制版不可修改），无视 DB 历史值
	settings.PromptPlazaApiBase = DefaultPromptPlazaApiBase

	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs

	return settingConfig
}
