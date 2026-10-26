package data

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// 信号 → OKX 自动交易。
//
// 数据流：信号监控面板上报信号 → SaveSignalRecords 去重落库 → 新信号进入
// DispatchOKXSignalTrades → 命中加密代码（ok:/bn:/bt: 前缀）且对应开关开启时，
// 按市价自动下单。模拟盘与实盘是两套独立开关（OKXSignalTradeEnable / OKXSignalTradeLive），
// 可同时启用或各自暂停；监控池也按环境分成两套（模拟盘池 / 实盘池），信号带 Envs 标注来源池，
// 只在来源池对应的环境下单（同一信号两池都有时两环境各下一笔、各落一条流水，Env 标注环境）。
// 模拟盘走模拟盘密钥 + x-simulated-trading:1，实盘走实盘密钥与真实资金（谨慎开启）；
// 模拟盘与实盘是两个独立账户，反向平仓按环境各查各的持仓。
// 成交结果（含失败原因）落 signal_trade_record 流水。
// 平仓动作同样落流水并用 family 区分：buysell=开仓、reverse=信号反向平仓、tpsl=止盈止损平仓、swap=满仓换仓平仓，
// 流水行 status 取值 success（已成交/已提交）/ failed（失败，含原因）/ skipped（正常跳过，如金额不足最小下单量）。
// 加密信号与股票永续信号（Name 带「美股/港股永续」标记）各自独立开关与每笔金额；
// 股票永续信号暂仅模拟盘语义。
//
// 只交易 buysell 族信号（TEMA 转折是趋势提示，与买卖点同向时会重复表达同一笔行情，
// 交易口径以买卖点为准，与信号收益统计的配对口径一致）。
//
// 防重复开仓四道闸（实测：实时池同一信号会以新 bar_time 逐轮重复派发，曾 14 分钟开出
// 12 笔 buy/sell 亚秒级翻单，手续费吃掉全部毛利；同向重复信号还会堆积仓位）：
//  1. 去重按分钟窗口：同一信号同一分钟内的重复上报视为同一信号（okxSignalTradeDone，
//     窗口范围匹配兼容流水库中秒/毫秒两种 bar_time 口径）；
//  2. 同向持仓不加仓：该环境已有同方向非零持仓时跳过（okxHasSameDirectionPosition，查询失败则 fail-closed 跳过）；
//  3. 平仓即冷却：TP/SL 或信号反向平仓后，被平方向在 ReentryCooldown 内不自动重进
//     （okxMarkTPSLCooldown，两类平仓共用同一冷却窗口，反向换向不受影响）；
//  4. 51001 合约熔断：合约在当前账户不存在/不可交易（如 SOXL-USDT-SWAP）时熔断 6 小时，
//     避免每轮扫描重复重试刷失败流水（okxMarkInstBlocked）。
//     冷却与同向跳过只记日志不落流水：持续信号每轮扫描都会重复派发，落流水会逐分钟刷屏。

// okxSignalTradeDispatching 防止信号风暴时并发堆积：同一时刻只允许一个调度协程在跑。
var okxSignalTradeDispatching atomic.Bool

// okxSignalTradeConf 信号自动交易配置（设置页 OKX 区块）。
// 加密信号与股票永续信号（美股/港股等合成股票合约）各自独立开关与金额。
type okxSignalTradeConf struct {
	Enable     bool // 加密信号·模拟盘开关
	AmountUSDT float64
	// Live 加密信号·实盘开关：与 Enable 相互独立，可同时启用（同一信号两个环境各下一笔）
	Live           bool
	LiveAmountUSDT float64 // 实盘每笔金额（USDT），独立于模拟盘金额
	// AmountMode 下单金额模式：fixed=固定金额（默认）；ratio=按下单环境可用余额比例开仓
	AmountMode string
	// AmountRatio 仓位比例（ratio 模式）：0.25=1/4 仓、0.3333=1/3 仓、0.5=半仓、1=全仓。
	// 口径为「名义价值」（开仓市值）= 可用余额 × 比例，不随杠杆放大；
	// 10x 杠杆下实际占用保证金 = 名义/10。
	AmountRatio float64
	// SignalMinScore 信号质量门槛：买卖点共振路数低于该值不自动下单；0=不限制
	SignalMinScore float64
	// ReverseMinScore 反手开仓·分数门槛：信号刚平掉反向仓后的反手开仓须满足的最低共振路数
	//（反手是换向鞭打的高发通道——实盘反手新腿胜率仅 18%、78% 换向双亏）；0=不限制反手。
	ReverseMinScore float64
	// ReentryCooldown TP/SL 平仓后同合约同方向的开仓冷却时长；0=不冷却
	ReentryCooldown time.Duration
	// MaxPositions 最大并发持仓数（防爆仓）：当前环境账户非零持仓 ≥ 该值时拒绝开新仓；0=不限制
	MaxPositions int
	// MaxMarginRatio 最大保证金率 %（防爆仓）：维持保证金合计/总权益 ≥ 阈值时拒绝开新仓；0=不限制
	MaxMarginRatio float64
	// ReservedSlots 高分预留槽位数：持仓占用 ≥ MaxPositions-ReservedSlots 时，
	// 共振路数低于 HighScore 的信号不再开新仓，最后几个槽位只留给高分信号——
	// 防止低分小仓把槽位占满后高分信号吃闭门羹（劣币驱逐良币）；0=关闭预留。
	ReservedSlots int
	// HighScore 预留槽位的信号分数门槛：接近满仓时只有 ≥ 该值的信号可占用预留槽位。
	HighScore float64
	// SwapGap 满仓换仓·分差门槛：新信号分数 − 最弱持仓入场分数 < 该值时不触发换仓
	//（边际优势须付得起平仓+开仓双份手续费）；≤0 = 关闭满仓换仓。
	SwapGap float64
	// SwapLossPct 满仓换仓·截亏线(%)：仅当最弱持仓浮动收益率（账户口径）≤ −该值/100 时
	// 才允许被换出腾槽——只截亏、不卖盈利仓追高；≤0 = 关闭满仓换仓。
	SwapLossPct float64
	// SwapMaxPerDay 满仓换仓·单日次数上限（按环境计数）；0 = 不限制。
	SwapMaxPerDay int
	// SignalLever 信号下单杠杆（仅 SWAP，cross）：空=不设置沿用账户当前杠杆；下单前按环境设置，失败拒绝下单
	SignalLever string
	// DirMode 开仓方向：both=多空都做（默认）；long=只做多（卖出信号只用于平多）；short=只做空（买入信号只用于平空）。
	// 回测：5m/15m 上「只做多」在下跌季度亏损更小，但最优解是配合 RegimeGate 按大势切换，故默认 both。
	DirMode string
	// RegimeGate 顺大势门控：开启后按「该合约自身」1H 趋势态过滤——多头趋势只开多、空头趋势只开空、过渡区不限制。
	// 回测（19 品种 / 1 年）：15m ≥6 路信号叠加该门控后年化由 -59% 收敛到 -28%，为全部组合中最优。
	RegimeGate bool
	// MaxPerDay 单合约单日最大开仓笔数（按开仓流水统计，东八区自然日）：0=不限制。
	// 5m 裸信号实测日均 4.5 笔/品种，手续费吞噬本金的主因，限笔数是最直接的止损手段。
	MaxPerDay int
	// MinTpPct 开单最低止盈收益率（%）：按当前止盈止损配置估算的本笔预期止盈收益率
	//（账户保证金口径，与止盈线同口径）低于该值时放弃开仓；仅 SWAP 生效，
	// 止盈未配置或无法估算时不拦截；0=不限制。
	MinTpPct float64
	// MaxTpPct 开单最大止盈收益率（%）：预期止盈收益率高于该值时放弃开仓——vol 模式下
	// 振幅过大意味着止损线同样过远，单笔最大回撤远超常规，可过滤极端行情/妖币合约；
	// 0=不限制。
	MaxTpPct float64
	// StockEnable 股票信号独立开关（默认关闭，OKX 模拟盘未上架股票永续；暂仅模拟盘语义）
	StockEnable     bool
	StockAmountUSDT float64         // 股票信号每笔金额（USDT）
	InstType        string          // SWAP / SPOT
	Klts            map[string]bool // 允许的 K 线周期；空 = 不限
}

func okxSignalTradeConfig() okxSignalTradeConf {
	cfg := okxSignalTradeConf{InstType: "SWAP"}
	if db.Dao == nil {
		return cfg
	}
	s := GetSettingConfig()
	if s == nil || s.Settings == nil {
		return cfg
	}
	cfg.Enable = s.OKXSignalTradeEnable
	amount, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalTradeUSDT), 64)
	if err != nil || amount <= 0 {
		amount = 100
	}
	cfg.AmountUSDT = amount
	cfg.Live = s.OKXSignalTradeLive
	liveAmount, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalTradeLiveUSDT), 64)
	if err != nil || liveAmount <= 0 {
		liveAmount = 10 // 实盘默认金额保守取 10 USDT（模拟盘仍默认 100）；设置页显式配置后以配置为准
	}
	cfg.LiveAmountUSDT = liveAmount
	cfg.StockEnable = s.OKXSignalStockEnable
	stockAmount, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalStockUSDT), 64)
	if err != nil || stockAmount <= 0 {
		stockAmount = 100
	}
	cfg.StockAmountUSDT = stockAmount
	if strings.EqualFold(strings.TrimSpace(s.OKXSignalTradeAmountMode), "ratio") {
		cfg.AmountMode = "ratio"
		if r, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalTradeAmountRatio), 64); err == nil && r > 0 && r <= 1 {
			cfg.AmountRatio = r
		} else {
			cfg.AmountRatio = 0.25 // 比例配置缺失/非法时保守取 1/4 仓
		}
	} else {
		cfg.AmountMode = "fixed"
	}
	// 防爆仓风控：空/非法 → 保守默认（3 笔 / 30%）；显式 0 = 不限制
	cfg.MaxPositions = 3
	if v, err := strconv.Atoi(strings.TrimSpace(s.OKXRiskMaxPositions)); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.MaxPositions = v
	}
	cfg.MaxMarginRatio = 30
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXRiskMaxMarginRatio), 64); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.MaxMarginRatio = v
	}
	// 高分预留槽位：默认 2 槽 / 6 分；显式 0 = 关闭预留（任何分数都能用满全部槽位）
	cfg.ReservedSlots = 2
	if v, err := strconv.Atoi(strings.TrimSpace(s.OKXRiskReservedSlots)); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.ReservedSlots = v
	}
	cfg.HighScore = 6
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXRiskHighScore), 64); err == nil && v > 0 {
		cfg.HighScore = v
	}
	// 满仓换仓三参数：默认分差 3 / 截亏线 5% / 单日 2 次；显式 0 语义——分差与截亏线
	// 任一为 0 即整体关闭换仓（零分差换仓无优势、零截亏线会卖盈利仓，均不安全），
	// 日限额 0=不限制；负值一律归 0。
	cfg.SwapGap = 3
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXRiskSwapGap), 64); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.SwapGap = v
	}
	cfg.SwapLossPct = 5
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXRiskSwapLossPct), 64); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.SwapLossPct = v
	}
	cfg.SwapMaxPerDay = 2
	if v, err := strconv.Atoi(strings.TrimSpace(s.OKXRiskSwapMaxPerDay)); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.SwapMaxPerDay = v
	}
	if it := strings.ToUpper(strings.TrimSpace(s.OKXSignalTradeInstType)); it == "SPOT" || it == "SWAP" {
		cfg.InstType = it
	}
	// 信号质量门槛：合法值 >0 生效，留空/非法 = 不限制（宁可多看不可误杀，门槛由用户显式设置）
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalTradeMinScore), 64); err == nil && v > 0 {
		cfg.SignalMinScore = v
	}
	// 反手开仓分数门槛：留空/非法取默认 6（反手须比普通开仓更强的信号佐证，
	// 与高分预留门槛同档）；显式 0 = 不限制反手；负值一律归 0。
	cfg.ReverseMinScore = 6
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalReverseMinScore), 64); err == nil {
		if v < 0 {
			v = 0
		}
		cfg.ReverseMinScore = v
	}
	// TP/SL 平仓后同向开仓冷却（分钟）：留空/非法 = 默认 30 分钟；显式 0 = 关闭
	if v, err := strconv.Atoi(strings.TrimSpace(s.OKXSignalTradeCooldownMin)); err == nil {
		if v > 0 {
			cfg.ReentryCooldown = time.Duration(v) * time.Minute
		}
	} else {
		cfg.ReentryCooldown = 30 * time.Minute
	}
	// 信号杠杆：仅接受 1~125 的合法值，其余一律视为不设置
	if v := strings.TrimSpace(s.OKXSignalTradeLever); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 1 && n <= 125 {
			cfg.SignalLever = v
		}
	}
	// 开仓方向：默认双向；只认 long/short，其余（含留空）按 both 处理
	switch strings.ToLower(strings.TrimSpace(s.OKXSignalTradeDirMode)) {
	case "long":
		cfg.DirMode = "long"
	case "short":
		cfg.DirMode = "short"
	default:
		cfg.DirMode = "both"
	}
	cfg.RegimeGate = s.OKXSignalTradeRegime
	if v, err := strconv.Atoi(strings.TrimSpace(s.OKXSignalTradeMaxPerDay)); err == nil && v > 0 {
		cfg.MaxPerDay = v
	}
	// 开单最低止盈收益率：留空/非法/0 = 不限制
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalMinTpPct), 64); err == nil && v > 0 {
		cfg.MinTpPct = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(s.OKXSignalMaxTpPct), 64); err == nil && v > 0 {
		cfg.MaxTpPct = v
	}
	cfg.Klts = map[string]bool{}
	for _, k := range strings.Split(s.OKXSignalTradeKlts, ",") {
		if k = strings.TrimSpace(k); k != "" {
			cfg.Klts[k] = true
		}
	}
	return cfg
}

// okxTradeEnvLabel 交易环境标签：simulated=true 为「模拟盘」，false 为「实盘」，用于交易流水记录。
func okxTradeEnvLabel(simulated bool) string {
	if !simulated {
		return "实盘"
	}
	return "模拟盘"
}

// okxSignalEnvWants 判断信号是否要求在指定环境下单。Envs 为来源监控池标注
// （sim=模拟盘池 / live=实盘池，逗号分隔表示两池都命中）；
// 空 = 未标注：模拟盘照常处理，实盘一律不下（实盘宁可漏不可误，
// 防止未来新增信号源忘带标注时把真实资金单发出去）。
func okxSignalEnvWants(envs, env string) bool {
	if strings.TrimSpace(envs) == "" {
		return env != "live"
	}
	for _, t := range strings.Split(envs, ",") {
		if strings.TrimSpace(t) == env {
			return true
		}
	}
	return false
}

// okxSignalTradeDone 该信号在该环境是否已有成功下单流水。
// 双池上报同一信号会合并环境并二次派发，以此防止同一环境重复下单。
// bar_time 按分钟窗口范围匹配而非精确相等：OKX 实时池同一信号会逐轮重复派发（检测时刻逐 tick 变化），
// 且流水库中 bar_time 存在秒/毫秒两种口径（随 K 线数据源而定）。先归一为毫秒分钟，再同时查询
// 毫秒与秒级区间，兼容历史流水并避免秒级时间戳被误按毫秒分桶。
//
// 去重键**不含 kind**：同一根 K 线只允许产生一笔开仓。此前 kind 参与去重，导致 buy 与 sell
// 各自独立判定「未下过单」，同一分钟内反向信号能连续穿透，实测 ETH-USDT-SWAP 在 3 秒内开出
// buy→sell→buy→sell 四笔（流水 id 2~7），净仓位没变、手续费白付，且与「同向持仓不加仓」互相掩护。
// 平仓流水走 family=reverse，不参与本键，不受影响。
func okxSignalTradeDone(r models.SignalRecord, env string) bool {
	if db.Dao == nil {
		return false
	}
	msStart, msEnd, secStart, secEnd := okxSignalBarTimeWindows(r.BarTime)
	var cnt int64
	db.Dao.Model(&models.SignalTradeRecord{}).
		Where("code = ? AND klt = ? AND family = ? AND "+
			"((bar_time >= ? AND bar_time < ?) OR (bar_time >= ? AND bar_time < ?)) AND env = ? AND status = ?",
			r.Code, r.Klt, r.Family, msStart, msEnd, secStart, secEnd, env, "success").
		Limit(1).Count(&cnt)
	return cnt > 0
}

// okxSignalBarTimeWindows 返回同一分钟对应的毫秒与秒级区间。
// 信号来源的 bar_time 既有 Unix 秒也有 Unix 毫秒；统一按毫秒分桶后，同时查两种存储口径，
// 以兼容历史流水并确保新信号不会因时间单位不同而重复下单。
func okxSignalBarTimeWindows(barTime int64) (msStart, msEnd, secStart, secEnd int64) {
	if barTime > 0 && barTime < 100_000_000_000 {
		barTime *= 1000
	}
	msStart = barTime - barTime%60_000
	msEnd = msStart + 60_000
	secStart = msStart / 1000
	secEnd = msEnd / 1000
	return
}

// okxSignalCodeInstInput 判断信号代码是否为可交易的加密标的，并剥出合约输入。
// 只接受 ok:/bn:/bt: 前缀（OKX 直连代码与币安/Bitget 代码均可映射到 OKX 合约），
// 裸代码是 A 股/港股/美股体系，绝不参与，避免把股票代码误解析成合约。
func okxSignalCodeInstInput(code string) (string, bool) {
	c := strings.TrimSpace(code)
	lower := strings.ToLower(c)
	for _, p := range []string{"ok:", "bn:", "bt:"} {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(c[len(p):]), true
		}
	}
	return "", false
}

// okxSignalIsStockPerp 判断信号是否为股票永续类合成代码（美股/港股永续等）。
// Name 由行情层生成并带「美股永续」等标记；此类标的在 OKX 属独立产品线
// （模拟盘未上架，仅实盘 xStocks 类产品），与加密永续分开开关与金额管理。
func okxSignalIsStockPerp(name string) bool {
	return strings.Contains(name, "美股永续") ||
		strings.Contains(name, "港股永续") ||
		strings.Contains(name, "股票永续")
}

// DispatchOKXSignalTrades 信号落库后的自动交易入口。
// 异步执行且同时只跑一个调度协程；模拟盘/实盘/股票三类开关均未开启时直接返回，零开销。
func DispatchOKXSignalTrades(news []models.SignalRecord) {
	if len(news) == 0 || db.Dao == nil {
		return
	}
	cfg := okxSignalTradeConfig()
	if !cfg.Enable && !cfg.Live && !cfg.StockEnable {
		return
	}
	type okxSignalTask struct {
		rec   models.SignalRecord
		stock bool // true = 股票永续信号（用独立开关/金额，暂仅模拟盘）
	}
	pending := make([]okxSignalTask, 0, len(news))
	for _, r := range news {
		if r.Family != "" && r.Family != "buysell" {
			continue // 只交易买卖点信号
		}
		if _, ok := okxSignalCodeInstInput(r.Code); !ok {
			continue // 只交易加密标的
		}
		if okxSignalIsStockPerp(r.Name) {
			if !cfg.StockEnable {
				continue // 股票信号独立开关未开启
			}
			pending = append(pending, okxSignalTask{rec: r, stock: true})
			continue
		}
		if !cfg.Enable && !cfg.Live {
			continue // 加密信号模拟盘/实盘开关均未开启
		}
		pending = append(pending, okxSignalTask{rec: r})
	}
	if len(pending) == 0 {
		return
	}
	if !okxSignalTradeDispatching.CompareAndSwap(false, true) {
		logger.SugaredLogger.Warnf("OKX信号交易调度已在进行，丢弃 %d 条待处理信号", len(pending))
		return
	}
	go func() {
		defer okxSignalTradeDispatching.Store(false)
		cfg := okxSignalTradeConfig()
		for _, t := range pending {
			// 周期过滤（用最新配置，避免调度期间改配置不生效）
			if len(cfg.Klts) > 0 && !cfg.Klts[t.rec.Klt] {
				continue
			}
			// 信号质量门槛：低分路信号（如 3 路灵敏档）噪音大，低于门槛整笔跳过（各环境均不下单）
			if cfg.SignalMinScore > 0 && signalScoreOf(t.rec) < cfg.SignalMinScore {
				logger.SugaredLogger.Infof("OKX信号交易跳过: 共振路数 %.1f 低于门槛 %.1f（信号质量不足）: code=%s klt=%s barTime=%d",
					signalScoreOf(t.rec), cfg.SignalMinScore, t.rec.Code, t.rec.Klt, t.rec.BarTime)
				continue
			}
			if t.stock {
				// 股票永续信号：独立开关/金额，暂仅模拟盘
				okxSignalTradeOne(t.rec, cfg, cfg.StockAmountUSDT, true)
				continue
			}
			// 加密信号：按来源池环境（sim=模拟盘池 / live=实盘池）过滤，再由各环境开关把关；
			// 两池都命中且两开关都开启时，同一信号两个环境各下一笔
			if cfg.Enable && okxSignalEnvWants(t.rec.Envs, "sim") {
				okxSignalTradeOne(t.rec, cfg, okxResolveSignalAmount(true, cfg.AmountUSDT, cfg), true)
			}
			if cfg.Live && okxSignalEnvWants(t.rec.Envs, "live") {
				okxSignalTradeOne(t.rec, cfg, okxResolveSignalAmount(false, cfg.LiveAmountUSDT, cfg), false)
			}
		}
	}()
}

// okxResolveSignalAmount 解析本笔信号的下单金额（USDT，名义价值口径，不乘杠杆）：
// fixed 模式直接返回配置的固定金额；ratio 模式实时查询下单环境账户 USDT 可用余额 × 仓位比例，
// 向下取整到 0.0001。余额查询失败或可用余额为 0 时返回 0（由 okxSignalTradeOne 落失败流水）。
func okxResolveSignalAmount(simulated bool, fixedAmount float64, cfg okxSignalTradeConf) float64 {
	if cfg.AmountMode != "ratio" || cfg.AmountRatio <= 0 || cfg.AmountRatio > 1 {
		return fixedAmount
	}
	bal, err := GetOKXBalanceForEnv(simulated)
	if err != nil || bal == nil {
		logger.SugaredLogger.Warnf("OKX信号自动交易[%s]: 按仓位比例查询可用余额失败: %v", okxTradeEnvLabel(simulated), err)
		return 0
	}
	avail := 0.0
	for _, d := range bal.Details {
		if strings.EqualFold(d.Ccy, "USDT") {
			avail = okxFloatNum(d.AvailEq)
			if avail <= 0 {
				avail = okxFloatNum(d.AvailBal)
			}
			break
		}
	}
	if avail <= 0 {
		// 无 USDT 明细（如纯币本位账户）时退化用总权益折算，保守仍乘比例
		avail = okxFloatNum(bal.TotalEq)
	}
	if avail <= 0 {
		return 0
	}
	amount := math.Floor(avail*cfg.AmountRatio*10000) / 10000
	logger.SugaredLogger.Infof("OKX信号自动交易[%s]: 按仓位比例下单（名义价值口径，不乘杠杆） 可用USDT=%.4f 比例=%.4f → 本笔名义=%.4f",
		okxTradeEnvLabel(simulated), avail, cfg.AmountRatio, amount)
	return amount
}

// okxSignalTradeOne 处理单条信号在指定环境的一笔交易：换算数量 → 反向平仓 → 市价下单 → 落流水。
// simulated=true 走 OKX 模拟环境，false 走实盘（实盘密钥与真实资金）。
func okxSignalTradeOne(r models.SignalRecord, cfg okxSignalTradeConf, amountUSDT float64, simulated bool) {
	rec := models.SignalTradeRecord{
		Code:        r.Code,
		Klt:         r.Klt,
		Family:      r.Family,
		Kind:        r.Kind,
		BarTime:     r.BarTime,
		SignalPrice: signalPriceOf(r),
		SignalScore: signalScoreOf(r),
		Name:        r.Name,
		Side:        r.Kind,
		OrdType:     "market",
		Env:         okxTradeEnvLabel(simulated),
		AmountUSDT:  amountUSDT,
		At:          time.Now().UnixMilli(),
	}
	envLabel := okxTradeEnvLabel(simulated)
	// 同一信号同一环境只下一笔：双池上报同一信号合并环境后可能二次派发，已有成功流水直接跳过
	if okxSignalTradeDone(r, envLabel) {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: 该信号已在该环境下单 code=%s klt=%s barTime=%d", envLabel, r.Code, r.Klt, r.BarTime)
		return
	}
	fail := func(msg string) {
		rec.Status = "failed"
		rec.ErrMsg = msg
		// OKX 51001（合约在当前账户不存在/不可交易）属持久性错误：每轮扫描重复派发的信号
		// 都会再失败一次（实测 SOXL-USDT-SWAP 两天刷出 47 条「设置杠杆失败 51001」流水），
		// 熔断该合约在本环境的后续尝试，重试无意义。
		if strings.Contains(msg, "51001") && rec.InstId != "" {
			okxMarkInstBlocked(simulated, rec.InstId)
			logger.SugaredLogger.Warnf("OKX信号自动交易[%s]: %s 报 51001（合约在当前账户不存在/不可交易），熔断 %s；请核对该合约是否上架或移出监控池",
				envLabel, rec.InstId, okxInstBlockTTL)
		}
		okxSaveSignalTradeRecord(rec)
		logger.SugaredLogger.Warnf("OKX信号自动交易失败[%s]: code=%s kind=%s err=%s", envLabel, r.Code, r.Kind, msg)
	}
	// 按仓位比例模式下解析金额失败（余额查询异常/可用余额为 0）→ 直接落失败流水
	if amountUSDT <= 0 {
		fail(fmt.Sprintf("按仓位比例获取可用余额失败（余额查询异常或可用余额为 0），无法下单"))
		return
	}

	instInput, _ := okxSignalCodeInstInput(r.Code)
	instId, instType, ok := ResolveOKXInstId(instInput, cfg.InstType)
	if !ok || instId == "" {
		fail(fmt.Sprintf("无法识别合约标识: %s", instInput))
		return
	}
	rec.InstId = instId
	rec.InstType = instType

	// 合约熔断期内直接跳过（只记日志不落流水，避免每轮扫描刷失败流水）
	if d := okxInstBlockedRemaining(simulated, instId); d > 0 {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s 合约熔断中（剩余 %s，此前报 51001 不可交易）",
			envLabel, r.Code, r.Kind, d.Truncate(time.Minute))
		return
	}

	// 低于最小下单量冷却：该合约近期已判定金额不足最小下单量，冷却期内不再重复
	// 换算/落流水（防持续信号逐分钟刷 skipped 流水），到期自动重试（金额可能已变化）。
	if d := okxBelowMinSkipRemaining(simulated, instId); d > 0 {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s 金额低于最小下单量冷却中（剩余 %s，不重复落流水）",
			envLabel, r.Code, r.Kind, d.Truncate(time.Second))
		return
	}

	// TP/SL/反向平仓后的同向冷却：冷却期内不再同向开仓。只记日志不落流水——
	// 实时池的持续信号每轮扫描都会以新 bar_time 重复派发（分钟归一去重只挡同一分钟内的重报），
	// 落流水会逐分钟刷屏。
	if d := okxTPSLCooldownRemaining(simulated, instId, r.Kind); d > 0 {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s 止盈/止损/反向平仓后冷却中（剩余 %s），跳过同向开仓",
			envLabel, r.Code, r.Kind, d.Truncate(time.Second))
		return
	}

	// 同向持仓不加仓：该合约在本环境已有同方向非零持仓时直接跳过（同样只记日志不落流水，
	// 防止持续信号重复派发时逐分钟堆积仓位与流水）。反向信号不受影响，由下方反向平仓换向。
	sameDir, posErr := okxHasSameDirectionPosition(instId, r.Kind, simulated)
	if posErr != nil {
		logger.SugaredLogger.Warnf("OKX信号自动交易跳过[%s]: code=%s kind=%s 查询持仓失败，本笔放弃（宁可漏单也不重复开仓）: %v",
			envLabel, r.Code, r.Kind, posErr)
		return
	}
	if sameDir {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s 该合约已有同向持仓，不重复加仓", envLabel, r.Code, r.Kind)
		return
	}

	// 合约规格（最小/步进/每张面值）
	ins := okxLookupInstrument(instId)
	if ins == nil {
		GetOKXInstruments(instType)
		ins = okxLookupInstrument(instId)
	}
	if ins == nil {
		// 规格列表已加载但查无此合约 → 非 OKX 交易标的（如本应用合成的美股永续代码），
		// 属正常情况，直接跳过不落失败流水，避免刷屏。
		if _, byId := okxInstrumentsSnapshot(); byId != nil {
			logger.SugaredLogger.Infof("OKX信号交易跳过: %s 不是OKX已上线合约（可能为美股永续等合成代码）: code=%s", instId, r.Code)
			return
		}
		fail(fmt.Sprintf("OKX 合约规格获取失败（网络/代理异常），无法交易 %s", instId))
		return
	}

	// 参考价（最新行情）
	ticker := GetOKXTicker(instId)
	if ticker == nil || okxFloatNum(ticker.Last) <= 0 {
		fail(fmt.Sprintf("获取 %s 最新价失败", instId))
		return
	}
	price := okxFloatNum(ticker.Last)
	rec.RefPrice = price

	// 金额 → 数量：金额即名义价值（不乘杠杆，保证金≈名义/杠杆），
	// 向下取整到 lotSz 步进，不足 minSz 属正常跳过（skipped），其他错误才算失败
	notional := amountUSDT
	sz, err := okxCalcSizeByAmount(notional, price, *ins)
	if err != nil {
		if errors.Is(err, ErrBelowMinSz) {
			okxMarkBelowMinSkip(simulated, instId)
			rec.Status = "skipped"
			rec.ErrMsg = fmt.Sprintf("金额 %.6g USDT %v，跳过本笔（不视为失败）", amountUSDT, err)
			okxSaveSignalTradeRecord(rec)
			logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s %v（%g 分钟内不重复记录）",
				envLabel, r.Code, r.Kind, err, okxBelowMinCooldown.Minutes())
			return
		}
		fail(fmt.Sprintf("金额 %.6g USDT 换算数量失败: %v", amountUSDT, err))
		return
	}
	rec.Sz = sz

	// 反向信号自动平仓：买入信号先平空单、卖出信号先平多单（双向持仓下买卖信号互不抵消）。
	// 按环境查询各账户持仓（模拟盘与实盘持仓互不可见）；平仓动作落 reverse 流水。
	// 平仓成功后登记被平方向的再进场冷却（与 TP/SL 平仓同一窗口）：实测同一信号在实时池
	// 每轮扫描重复派发时，1~2 分钟内就会 buy→sell→buy 亚秒级来回翻单，手续费吃掉全部毛利，
	// 原方向冷却期内不再自动重进（反向换向须先过反手分数门槛，真实趋势反转的强信号仍可换向）。
	if closedDir := okxCloseOppositePosition(r, instId, instType, price, simulated); closedDir != "" {
		okxMarkTPSLCooldown(simulated, instId, closedDir, cfg.ReentryCooldown)
		// 反手分数门槛：刚平掉反向仓后立刻反手，实盘新腿胜率仅 18%、78% 换向双亏鞭打；
		// 共振路数不足时只平仓、不反手开新仓——截断鞭打通道，足够强的信号仍可正常换向。
		if cfg.ReverseMinScore > 0 && signalScoreOf(r) < cfg.ReverseMinScore {
			okxRecordSkippedThrottled(rec, envLabel, "reverse-gate",
				fmt.Sprintf("反向平仓已成交，反手分数 %.6g < 门槛 %.6g，只平不开", signalScoreOf(r), cfg.ReverseMinScore))
			return
		}
	}

	// 开仓门控（只拦开仓、不拦平仓）：方向模式 / 标的 1H 趋势态 / 单日开仓上限。
	// 放在反向平仓之后——被门控拦下的信号仍应把逆势持仓平掉，只是不再反手开新仓。
	if reason := okxSignalOpenBlock(cfg, r.Kind, instId, simulated); reason != "" {
		logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s %s", envLabel, r.Code, r.Kind, reason)
		return
	}

	// ===== 防爆仓风控（在反向平仓之后检查，一平一开的正常轮动不被误拦）=====
	// 满仓/预留槽位/保证金率属「正常跳过」而非失败：实时池的持续信号每轮扫描重复派发，
	// 实盘满仓 4 天曾刷出 1930 条 failed 流水；改为落 skipped 并按 环境+合约+原因 节流。
	if msg := okxRiskGateCheck(simulated, cfg, signalScoreOf(r)); msg != "" {
		// 满仓换仓：风控拒绝时先尝试平掉在场最弱浮亏仓腾槽（浮亏优先+分差守门+日限额，
		// 详见 okxTrySwapForSlot），高分信号不被劣质仓位卡死。未换成功 → 拒绝原因拼上
		// 换仓说明落 skipped；换仓成功 → 重过一次风控，仍拒（如保证金率超标，换仓解决不了）
		// 照常落 skipped（平仓成交到持仓可查有毫秒级延迟，偶发误拦由下轮信号重试兜底）。
		score := signalScoreOf(r)
		if swapped, note := okxTrySwapForSlot(simulated, cfg, r, score, instId); !swapped {
			okxRecordSkippedThrottled(rec, envLabel, "risk-gate", msg+note)
			return
		}
		if msg2 := okxRiskGateCheck(simulated, cfg, score); msg2 != "" {
			okxRecordSkippedThrottled(rec, envLabel, "risk-gate", msg2)
			return
		}
	}

	// 信号杠杆（仅 SWAP）：下单前按环境设置合约杠杆（cross），失败拒绝该笔（宁可漏不误，
	// 避免账户实际杠杆与预期不符放大风险）。杠杆先按合约最大可设杠杆收敛，
	// 避免低杠杆上限合约（如 OL-USDT）整笔交易因 59102 超限失败。
	lever := ""
	if instType == "SWAP" && cfg.SignalLever != "" {
		lever = okxClampLever(cfg.SignalLever, ins)
		if _, err := SetOKXLeverageEnv(simulated, instId, lever); err != nil {
			fail(fmt.Sprintf("设置杠杆 %sx 失败: %v", lever, err))
			return
		}
	}

	// 开单止盈收益率区间门槛：按当前止盈止损配置估算本笔预期止盈收益率（账户保证金口径，
	// 与止盈线同口径），低于下限不开单——低波动合约算出的止盈线过近，双边手续费+滑点
	// 即可吃掉全部毛利；高于上限也不开单——vol 模式下振幅过大意味着止损线同样过远，
	// 单笔最大回撤远超常规，可用于过滤极端行情/妖币合约。止盈止损未开启或未配置止盈
	//（估算值为 0）时无法比较，不拦截；与其他开仓门控一致只拦开仓不拦平仓（反向平仓
	// 已在上方完成）。持续信号逐轮重复派发，拦截只记日志不落流水，防逐分钟刷屏。
	if (cfg.MinTpPct > 0 || cfg.MaxTpPct > 0) && instType == "SWAP" {
		if sc := GetSettingConfig(); sc != nil && sc.Settings != nil && sc.OKXTPSLEnable {
			tpcfg := okxTPSLConfig()
			var amp float64
			if tpcfg.Mode == "vol" {
				amp = okxAvgAmplitudePct(instId, tpcfg.VolKlt, tpcfg.VolBars)
			}
			if reason := okxSignalTpOutOfRange(okxSignalTpYieldPct(tpcfg, amp, okxFloatNum(lever)), cfg.MinTpPct, cfg.MaxTpPct); reason != "" {
				logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s kind=%s 预期止盈收益率 %s，不开单",
					envLabel, r.Code, r.Kind, reason)
				return
			}
		}
	}

	// 市价单：SWAP 全仓 / SPOT 现货
	req := &OKXOrderRequest{
		InstId:  instId,
		Side:    r.Kind,
		Sz:      sz,
		OrdType: "market",
	}
	if instType == "SPOT" {
		req.TdMode = "cash"
	} else {
		req.TdMode = "cross"
	}

	// 交易所侧附带止盈止损（仅 SWAP）：与软件侧 10 秒轮询监控共用同一套 fixed/vol 配置，
	// 应用退出/断网期间由交易所侧价格触发兜底。附加被交易所拒绝时降级裸单重试
	//（市价单被拒未成交，重试无重复成交风险），不让 TPSL 参数问题阻断开仓。
	var attached *OKXAttachAlgoOrd
	if instType == "SWAP" {
		leverNum := okxFloatNum(lever)
		attached = okxAttachTPSLForOrder(simulated, instId, r.Kind, price, leverNum, okxTPSLConfig(), *ins)
		if attached != nil {
			req.AttachAlgoOrds = []OKXAttachAlgoOrd{*attached}
		}
	}

	res, err := PlaceOKXSignalOrder(req, simulated)
	if err != nil && len(req.AttachAlgoOrds) > 0 {
		logger.SugaredLogger.Warnf("OKX信号自动交易[%s] 交易所侧TPSL附加被拒，降级裸单重试: %s %s err=%v", envLabel, instId, r.Kind, err)
		req.AttachAlgoOrds = nil
		res, err = PlaceOKXSignalOrder(req, simulated)
	}
	if err != nil {
		fail(err.Error())
		return
	}
	rec.Status = "success"
	rec.OrdId = res.OrdId
	if attached != nil {
		rec.ErrMsg = fmt.Sprintf("TP=%.8g SL=%.8g", okxFloatNum(attached.TpTriggerPx), okxFloatNum(attached.SlTriggerPx))
	}
	okxSaveSignalTradeRecord(rec)
	if attached != nil {
		logger.SugaredLogger.Infof("OKX信号自动交易已提交[%s]: %s %s %s sz=%s 名义≈%.6g USDT ordId=%s 交易所侧TPSL TP=%s SL=%s",
			envLabel, instId, r.Kind, r.Klt, sz, notional, res.OrdId, attached.TpTriggerPx, attached.SlTriggerPx)
	} else {
		logger.SugaredLogger.Infof("OKX信号自动交易已提交[%s]: %s %s %s sz=%s 名义≈%.6g USDT ordId=%s",
			envLabel, instId, r.Kind, r.Klt, sz, notional, res.OrdId)
	}
}

// ===== 合约熔断（OKX 51001 持久性错误）=====
// 实测：SOXL-USDT-SWAP 在 OKX 账户不可交易（51001 Instrument ID doesn't exist），
// 监控池里的信号每轮扫描重复派发，每次都走到「设置杠杆/下单」再失败一次，
// 两天刷出 47 条失败流水。51001 属账户/合约维度的持久性错误，重试无意义：
// 熔断期内该合约在该环境直接跳过。仅内存态，重启清零（修好合约映射后重启即恢复）。

// okxInstBlockTTL 合约熔断时长。
const okxInstBlockTTL = 6 * time.Hour

// okxInstBlockKey 熔断表键：环境 + 合约（模拟盘与实盘账户独立，互不影响）。
type okxInstBlockKey struct {
	simulated bool
	instId    string
}

var (
	okxInstBlockMu    sync.Mutex
	okxInstBlockUntil = map[okxInstBlockKey]time.Time{}
)

// okxMarkInstBlocked 熔断某合约在指定环境的自动交易。
func okxMarkInstBlocked(simulated bool, instId string) {
	if strings.TrimSpace(instId) == "" {
		return
	}
	okxInstBlockMu.Lock()
	okxInstBlockUntil[okxInstBlockKey{simulated, instId}] = time.Now().Add(okxInstBlockTTL)
	okxInstBlockMu.Unlock()
}

// okxInstBlockedRemaining 查询熔断剩余时长；未熔断/已过期返回 0（过期顺手清理）。
func okxInstBlockedRemaining(simulated bool, instId string) time.Duration {
	key := okxInstBlockKey{simulated, instId}
	okxInstBlockMu.Lock()
	defer okxInstBlockMu.Unlock()
	until, ok := okxInstBlockUntil[key]
	if !ok {
		return 0
	}
	if d := time.Until(until); d > 0 {
		return d
	}
	delete(okxInstBlockUntil, key)
	return 0
}

// ===== TP/SL 平仓后同向开仓冷却 =====
// 背景：实盘实测震荡合约（如 MAGIC）止盈后 2 分钟即同向重进、止损后 7 分钟重进，
// 高换手的手续费吃掉全部毛利。TP/SL 平仓代表该方向本轮尝试已被否定，
// 冷却期内同合约同方向不再自动开仓（反向信号不受影响，趋势反转仍可换向）。
// 仅内存态，重启清零（冷却本就是短时效风控，无需持久化）。

// okxCooldownKey 冷却表键：环境 + 合约 + 开仓方向（buy=多头方向 / sell=空头方向）
type okxCooldownKey struct {
	simulated bool
	instId    string
	direction string
}

var (
	okxCooldownMu    sync.RWMutex
	okxCooldownUntil = map[okxCooldownKey]time.Time{}
)

// okxMarkTPSLCooldown TP/SL 平仓成功后登记同向冷却。
// direction 传被平仓头寸的开仓方向（多头头寸=buy、空头头寸=sell）。
func okxMarkTPSLCooldown(simulated bool, instId, direction string, d time.Duration) {
	if d <= 0 || instId == "" || (direction != "buy" && direction != "sell") {
		return
	}
	okxCooldownMu.Lock()
	defer okxCooldownMu.Unlock()
	okxCooldownUntil[okxCooldownKey{simulated, instId, direction}] = time.Now().Add(d)
}

// okxTPSLCooldownRemaining 返回剩余冷却时长；未命中或已过期返回 0。
// 过期键由下次 okxMarkTPSLCooldown 覆盖或重启清零，条目数以合约数×2为上界，无需主动清理。
func okxTPSLCooldownRemaining(simulated bool, instId, direction string) time.Duration {
	okxCooldownMu.RLock()
	defer okxCooldownMu.RUnlock()
	if d := time.Until(okxCooldownUntil[okxCooldownKey{simulated, instId, direction}]); d > 0 {
		return d
	}
	return 0
}

// ===== 低于最小下单量跳过冷却（防 skipped 流水刷屏）=====
// 背景：okxSignalTradeDone 只按 success 去重，实时池持续信号每轮带新 bar_time 重派，
// 低于最小下单量的合约每轮都重新换算→跳过→落一条 skipped 流水（实测一天 60+ 条，
// STABLE/MAGIC/AKE 等小面值合约各 20+ 条）。与 51001 熔断同类问题：
// 冷却期内不再重复换算/落库/记日志。ratio 模式金额随余额变化，冷却到期自然重试。

// okxBelowMinKey 跳过冷却表键：环境 + 合约。
type okxBelowMinKey struct {
	simulated bool
	instId    string
}

// okxBelowMinCooldown 跳过冷却时长：余额/金额变化后最多延迟半小时恢复尝试。
const okxBelowMinCooldown = 30 * time.Minute

var (
	okxBelowMinMu    sync.RWMutex
	okxBelowMinUntil = map[okxBelowMinKey]time.Time{}
)

// okxMarkBelowMinSkip 登记该合约在本环境低于最小下单量的跳过冷却。
func okxMarkBelowMinSkip(simulated bool, instId string) {
	if strings.TrimSpace(instId) == "" {
		return
	}
	okxBelowMinMu.Lock()
	defer okxBelowMinMu.Unlock()
	okxBelowMinUntil[okxBelowMinKey{simulated, instId}] = time.Now().Add(okxBelowMinCooldown)
}

// okxBelowMinSkipRemaining 查询跳过冷却剩余时长；未命中或已过期返回 0。
func okxBelowMinSkipRemaining(simulated bool, instId string) time.Duration {
	okxBelowMinMu.RLock()
	defer okxBelowMinMu.RUnlock()
	if d := time.Until(okxBelowMinUntil[okxBelowMinKey{simulated, instId}]); d > 0 {
		return d
	}
	return 0
}

// okxHasSameDirectionPosition 检查该合约在指定环境是否已有与信号同方向的非零持仓；
// 查询失败返回 error，由调用方跳过本笔（不能当成「没有持仓」放行）
// （双向持仓按 posSide 判，单向持仓按仓位符号判）。查询失败返回 false（不阻断下单，后续风控仍会把关）。
func okxHasSameDirectionPosition(instId, kind string, simulated bool) (bool, error) {
	pos, err := okxPositionsForEnv(instId, simulated)
	if err != nil {
		// 查询失败必须让调用方知道：此前直接返回 false（放行），一次网络抖动就会把
		// 「同向不加仓」这道闸静默打开，实测 DOGE 在 8 分钟内同向连开 3 笔（流水 id 8~10）。
		return false, err
	}
	for _, p := range pos {
		if p.InstId != instId {
			continue
		}
		v := okxFloatNum(p.Pos)
		if v == 0 {
			continue
		}
		if kind == "buy" && (p.PosSide == "long" || (p.PosSide == "net" && v > 0)) {
			return true, nil
		}
		if kind == "sell" && (p.PosSide == "short" || (p.PosSide == "net" && v < 0)) {
			return true, nil
		}
	}
	return false, nil
}

// okxCloseOppositePosition 反向信号自动平仓：买入信号平空单、卖出信号平多单。
// 双向持仓（long_short_mode）下 buy/sell 信号各自独立开仓、互不抵消，需显式 reduceOnly 平仓。
// SPOT 无持仓概念直接跳过；同向持仓不动。
// 平仓动作落 reverse 族流水（成功/失败均记录，一次性触发无重试不会刷屏）；
// 失败仅记流水与日志不阻断信号开仓（保持既有行为，实盘 net 模式下开仓单可自然对冲）。
// r 提供信号上下文（代码/周期/信号价/评分，用于流水追溯），refPrice 为下单时参考价。
// 返回被平仓头寸的方向（多头="buy"、空头="sell"）；未发生平仓返回空串，供调用方登记同向再进场冷却。
func okxCloseOppositePosition(r models.SignalRecord, instId, instType string, refPrice float64, simulated bool) string {
	if instType == "SPOT" {
		return ""
	}
	envLabel := okxTradeEnvLabel(simulated)
	pos, err := okxPositionsForEnv(instId, simulated)
	if err != nil {
		logger.SugaredLogger.Warnf("OKX反向平仓查持仓失败[%s]: instId=%s err=%v", envLabel, instId, err)
		return ""
	}
	for _, p := range pos {
		if p.InstId != instId {
			continue
		}
		posVal := okxFloatNum(p.Pos)
		if posVal == 0 {
			continue
		}
		var closeSide, closePosSide, closedDir string // closedDir=被平头寸方向（多头=buy/空头=sell）
		switch {
		case p.PosSide == "short" && r.Kind == "buy":
			// 平空：买回
			closeSide, closePosSide, closedDir = "buy", "short", "sell"
		case p.PosSide == "long" && r.Kind == "sell":
			// 平多：卖出
			closeSide, closePosSide, closedDir = "sell", "long", "buy"
		case p.PosSide == "net" && ((r.Kind == "buy" && posVal < 0) || (r.Kind == "sell" && posVal > 0)):
			// 单向持仓：pos<0 为空头 → 买入信号平仓；pos>0 为多头 → 卖出信号平仓
			closedDir = "sell"
			if posVal > 0 {
				closedDir = "buy"
			}
			closeSide, closePosSide = r.Kind, "net"
			posVal = -posVal
		default:
			continue // 同向持仓，不平
		}
		ro := true
		closeReq := &OKXOrderRequest{
			InstId:     instId,
			TdMode:     p.MgnMode,
			Side:       closeSide,
			PosSide:    closePosSide,
			OrdType:    "market",
			Sz:         strconv.FormatFloat(math.Abs(posVal), 'f', -1, 64),
			ReduceOnly: &ro,
		}
		if strings.TrimSpace(closeReq.TdMode) == "" {
			closeReq.TdMode = "cross"
		}
		res, err := placeOKXOrderWithEnv(closeReq, simulated)
		// 平仓流水：family=reverse 与开仓（buysell）区分，不参与开仓去重；名义价值按平仓数量×面值×参考价
		closeRec := models.SignalTradeRecord{
			Code:        r.Code,
			InstId:      instId,
			InstType:    instType,
			Klt:         r.Klt,
			Family:      "reverse",
			Kind:        "close",
			BarTime:     r.BarTime,
			SignalPrice: signalPriceOf(r),
			SignalScore: signalScoreOf(r),
			Name:        r.Name,
			Side:        closeSide,
			OrdType:     "market",
			Sz:          closeReq.Sz,
			RefPrice:    refPrice,
			Env:         envLabel,
			AmountUSDT:  okxNotionalBySz(instId, closeReq.Sz, refPrice),
			At:          time.Now().UnixMilli(),
		}
		if err != nil {
			closeRec.Status = "failed"
			closeRec.ErrMsg = err.Error()
			okxSaveSignalTradeRecord(closeRec)
			logger.SugaredLogger.Warnf("OKX反向平仓失败[%s]: instId=%s posSide=%s sz=%s err=%v", envLabel, instId, p.PosSide, closeReq.Sz, err)
			return "" // 平仓失败不登记冷却（头寸还在，下次信号继续尝试平仓）
		}
		closeRec.Status = "success"
		closeRec.OrdId = res.OrdId
		okxSaveSignalTradeRecord(closeRec)
		logger.SugaredLogger.Infof("OKX反向信号自动平仓[%s]: %s %s信号 平掉 posSide=%s pos=%s ordId=%s", envLabel, instId, r.Kind, p.PosSide, p.Pos, res.OrdId)
		// 撤销被平头寸方向的孤儿条件单（异步）：马上要反方向开新仓，旧条件单残留会误平新仓
		go okxCancelInstPendingAlgos(simulated, instId)
		return closedDir // 每次信号只平与方向相反的一边
	}
	return ""
}

// okxNotionalBySz 按数量与参考价估算名义价值（USDT）：SWAP = 张数×合约面值×价，其余 = 数量×价。
// 规格查不到时面值按 1 计；仅用于流水展示，不参与交易决策。
func okxNotionalBySz(instId, sz string, refPrice float64) float64 {
	n := okxFloatNum(sz)
	if n <= 0 || refPrice <= 0 {
		return 0
	}
	ctVal := 1.0
	if strings.HasSuffix(strings.ToUpper(instId), "-SWAP") {
		if spec := okxLookupInstrument(instId); spec != nil {
			if v := okxFloatNum(spec.CtVal); v > 0 {
				ctVal = v
			}
		}
	}
	return n * ctVal * refPrice
}

// okxSignalTpYieldPct 估算本笔开仓的预期止盈收益率（账户保证金口径 %，与止盈线同口径）。
// fixed 模式 = 固定止盈收益率 TpPct；vol 模式 = 振幅×止盈倍数×生效杠杆
// （lever 非法/未知按 okxTPSLFallbackLever 兜底，与监控兜底口径一致）。
// 返回 0 表示未配置止盈或无法估算（振幅取不到），调用方应视为「无止盈可拦」而非「收益为 0」。
func okxSignalTpYieldPct(tpcfg okxTPSLConf, amp, lever float64) float64 {
	if tpcfg.Mode != "vol" {
		return tpcfg.TpPct
	}
	if amp <= 0 || tpcfg.TpMult <= 0 {
		return 0
	}
	return amp * tpcfg.TpMult * okxEffLever(lever)
}

// okxSignalTpOutOfRange 判断预期止盈收益率是否落在开单区间 [minTp, maxTp] 之外，
// 返回用于日志的原因描述（空串=放行）。tp<=0 表示无法估算/未配置止盈，放行；
// minTp/maxTp<=0 表示该侧不限制。
func okxSignalTpOutOfRange(tp, minTp, maxTp float64) string {
	if tp <= 0 {
		return ""
	}
	if minTp > 0 && tp < minTp {
		return fmt.Sprintf("%.2f%% 低于开单下限 %.2f%%", tp, minTp)
	}
	if maxTp > 0 && tp > maxTp {
		return fmt.Sprintf("%.2f%% 超过开单上限 %.2f%%", tp, maxTp)
	}
	return ""
}

// okxSignalOpenBlock 判断该信号是否允许开新仓，返回拒绝原因（空串=放行）。
// 三道门控只作用于「开仓」：平仓（反向信号平掉逆势持仓）永远放行，
// 否则只做多模式下遇到卖出信号会把持仓永久锁死。
func okxSignalOpenBlock(cfg okxSignalTradeConf, kind, instId string, simulated bool) string {
	if kind != "buy" && kind != "sell" {
		return ""
	}
	switch cfg.DirMode {
	case "long":
		if kind == "sell" {
			return "只做多模式：卖出信号仅用于平多，不开空"
		}
	case "short":
		if kind == "buy" {
			return "只做空模式：买入信号仅用于平空，不开多"
		}
	}
	if cfg.RegimeGate {
		if v := okxTrendRegime(instId); v != 0 {
			if kind == "buy" && v < 0 {
				return "标的 1H 为空头趋势，拦截逆势做多"
			}
			if kind == "sell" && v > 0 {
				return "标的 1H 为多头趋势，拦截逆势做空"
			}
		}
	}
	if cfg.MaxPerDay > 0 {
		if n := okxSignalEntriesToday(simulated, instId); n >= cfg.MaxPerDay {
			return fmt.Sprintf("该合约今日已开仓 %d 笔（上限 %d），不再新开", n, cfg.MaxPerDay)
		}
	}
	return ""
}

// okxRiskGateCheck 防爆仓风控闸门：开新仓前检查两项（模拟盘/实盘各自独立检查）：
//  1. 最大并发持仓数：当前环境账户非零持仓笔数 ≥ 上限时拒绝开新仓（防止连续信号累加成超额总仓位）。
//     在此之上叠加「高分预留槽位」（ReservedSlots>0 时生效）：持仓占用进入接近满仓区间
//     （占用 ≥ MaxPositions-ReservedSlots）后，共振路数 < HighScore 的信号同样不开新仓——
//     最后几个槽位只留给高分信号，防止低分小仓把槽位占满后高分信号吃闭门羹（劣币驱逐良币）。
//     预留槽位数 ≥ 最大持仓数时阈值归零，等价于全部槽位只留给高分信号；
//  2. 最大保证金率：账户维持保证金合计 / 总权益 ≥ 阈值时拒绝开新仓（保证金率是距离强平的直接指标，
//     TPSL 按单笔收益率兜底、兜不住总仓位过重导致的集体逼近强平）。
//
// score 为当前信号共振路数（用于预留槽位门槛比较），未配置预留槽位时不参与判断。
// 返回空串 = 放行；非空 = 拒绝原因（按正常跳过落 skipped 流水并节流）。配置为 0 表示不限制该项。
func okxRiskGateCheck(simulated bool, cfg okxSignalTradeConf, score float64) string {
	envLabel := okxTradeEnvLabel(simulated)

	// 持仓笔数检查
	if cfg.MaxPositions > 0 {
		poss, err := GetOKXPositionListForEnv(simulated, "")
		if err != nil {
			// 风控数据取不到时宁可拦下（实盘宁可漏不可误）；模拟盘同样处理保持行为一致
			return fmt.Sprintf("风控检查失败（持仓查询异常: %v），拒绝开新仓", err)
		}
		cnt := 0
		for _, p := range poss {
			if okxFloatNum(p.Pos) != 0 {
				cnt++
			}
		}
		if cnt >= cfg.MaxPositions {
			return fmt.Sprintf("已达最大并发持仓数 %d/%d，拒绝开新仓", cnt, cfg.MaxPositions)
		}
		// 高分预留槽位：接近满仓区间内低分信号不占用最后几个槽位
		if cfg.ReservedSlots > 0 {
			if thr := cfg.MaxPositions - cfg.ReservedSlots; cnt >= thr && score < cfg.HighScore {
				return fmt.Sprintf("持仓 %d/%d 接近上限（预留 %d 槽给 ≥%.0f 分信号），当前信号 %.0f 分暂不新开",
					cnt, cfg.MaxPositions, cfg.ReservedSlots, cfg.HighScore, score)
			}
		}
	}

	// 保证金率检查
	if cfg.MaxMarginRatio > 0 {
		bal, err := GetOKXBalanceForEnv(simulated)
		if err != nil || bal == nil {
			return fmt.Sprintf("风控检查失败（余额查询异常: %v），拒绝开新仓", err)
		}
		totalEq := okxFloatNum(bal.TotalEq)
		if totalEq <= 0 {
			return fmt.Sprintf("账户总权益为 %.4f USD，拒绝开新仓", totalEq)
		}
		mmrSum := 0.0
		for _, d := range bal.Details {
			mmrSum += okxFloatNum(d.Mmr)
		}
		ratio := mmrSum / totalEq * 100
		if ratio >= cfg.MaxMarginRatio {
			return fmt.Sprintf("当前保证金率 %.1f%% ≥ 阈值 %.1f%%（仓位过重，接近强平风险），拒绝开新仓", ratio, cfg.MaxMarginRatio)
		}
		logger.SugaredLogger.Debugf("OKX风控[%s]: 保证金率 %.1f%%/%.1f%%，持仓检查通过", envLabel, ratio, cfg.MaxMarginRatio)
	}
	return ""
}

// okxSkipThrottleTTL skipped 流水去重节流时长：持续信号会被实时池每轮扫描重复派发，
// 同一环境+合约的同类拒绝（满仓/预留槽位/保证金率）若每次都落流水会逐分钟刷屏；
// 节流窗口内只记 Debug 日志，到期自动恢复落流水（仓位与保证金状态可能已变化）。
const okxSkipThrottleTTL = 10 * time.Minute

var (
	okxSkipThrottleMu sync.Mutex
	okxSkipThrottle   = map[string]time.Time{}
)

// okxRecordSkippedThrottled 落一条 skipped 流水并按「环境|合约|原因」节流：
// 同一键在 TTL 内重复触发只记 Debug 日志不落流水（返回 false），首条落流水并开启节流（返回 true）。
// rec 需已填好 Code/InstId/Env 等上下文字段（开仓流水在合约识别之后调用即满足）。
func okxRecordSkippedThrottled(rec models.SignalTradeRecord, envLabel, tag, msg string) bool {
	key := fmt.Sprintf("%s|%s|%s", rec.Env, rec.InstId, tag)
	now := time.Now()
	okxSkipThrottleMu.Lock()
	if until, ok := okxSkipThrottle[key]; ok && now.Before(until) {
		okxSkipThrottleMu.Unlock()
		logger.SugaredLogger.Debugf("OKX信号自动交易跳过[%s]（节流中，不重复落流水）: code=%s instId=%s %s",
			envLabel, rec.Code, rec.InstId, msg)
		return false
	}
	okxSkipThrottle[key] = now.Add(okxSkipThrottleTTL)
	okxSkipThrottleMu.Unlock()
	rec.Status = "skipped"
	rec.ErrMsg = msg
	okxSaveSignalTradeRecord(rec)
	logger.SugaredLogger.Infof("OKX信号自动交易跳过[%s]: code=%s instId=%s %s（%g 分钟内不重复记录）",
		envLabel, rec.Code, rec.InstId, msg, okxSkipThrottleTTL.Minutes())
	return true
}

// ===== 满仓换仓（截弱留强腾槽）=====
// 背景：实盘最大持仓数常年被占满，高分信号因「已达最大并发持仓数」被持续跳过——
// 「高分预留槽位」只能拦低分信号进占，救不了已被占死的槽位。换仓机制在满仓时让
// 高分信号平掉在场「最弱浮亏仓」腾槽换入：浮亏优先（只截亏，不卖盈利仓追高）、
// 分差守门（新信号分数须显著高于弱者入场分，边际优势才付得起平仓+开仓双份手续费）、
// 日限额（防震荡日反复换仓磨损）。平仓动作落 swap 族流水（不参与开仓去重）。

// okxSwapFreshTTL 换入保护期：换仓新开的合约在保护期内不作为被换出候选——
// 刚换进来的仓立刻又被更强的信号换走会连环 churn。OKX 持仓接口无建仓时间，
// 无法按仓位年龄判断，故用内存登记（重启清零）。
const okxSwapFreshTTL = time.Hour

// okxSwapFreshKey 换入保护表键：环境 + 合约。
type okxSwapFreshKey struct {
	simulated bool
	instId    string
}

var (
	okxSwapFreshMu    sync.RWMutex
	okxSwapFreshUntil = map[okxSwapFreshKey]time.Time{}
)

// okxMarkSwapFresh 登记换入合约的保护期（换仓平仓成交、新仓即将开立时调用）。
func okxMarkSwapFresh(simulated bool, instId string) {
	if strings.TrimSpace(instId) == "" {
		return
	}
	okxSwapFreshMu.Lock()
	defer okxSwapFreshMu.Unlock()
	okxSwapFreshUntil[okxSwapFreshKey{simulated, instId}] = time.Now().Add(okxSwapFreshTTL)
}

// okxSwapFreshRemaining 查询换入保护剩余时长；未命中或已过期返回 0。
func okxSwapFreshRemaining(simulated bool, instId string) time.Duration {
	okxSwapFreshMu.RLock()
	defer okxSwapFreshMu.RUnlock()
	if d := time.Until(okxSwapFreshUntil[okxSwapFreshKey{simulated, instId}]); d > 0 {
		return d
	}
	return 0
}

// okxEntryScoreOf 查该合约在本环境最近一笔成功开仓的信号分数（入场分背书）：
// 换仓分差守门比较「新信号分数 − 弱者入场分」。查无成功开仓记录的手动仓按 0 分
// 处理（无信号背书，视为最弱候选）。
func okxEntryScoreOf(simulated bool, instId string) float64 {
	if db.Dao == nil || strings.TrimSpace(instId) == "" {
		return 0
	}
	var rec models.SignalTradeRecord
	err := db.Dao.Where("inst_id = ? AND env = ? AND family = ? AND status = ?",
		instId, okxTradeEnvLabel(simulated), "buysell", "success").
		Order("at DESC").Limit(1).First(&rec).Error
	if err != nil {
		return 0
	}
	return rec.SignalScore
}

// okxSwapCountToday 本环境今日已发生的满仓换仓次数（swap 族成功流水计数，东八区自然日）。
// 查询失败返回 -1（守门放行 fail-open：换仓仍有分差/截亏两道守门兜底，不宜因统计
// 查询异常把整个机制卡死）。
func okxSwapCountToday(simulated bool) int {
	if db.Dao == nil {
		return -1
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()
	var cnt int64
	err := db.Dao.Model(&models.SignalTradeRecord{}).
		Where("env = ? AND family = ? AND status = ? AND at >= ?",
			okxTradeEnvLabel(simulated), "swap", "success", dayStart).
		Count(&cnt).Error
	if err != nil {
		return -1
	}
	return int(cnt)
}

// okxSwapCandidate 被换出候选仓快照：浮亏率（UplRatio，账户口径小数）与入场分。
type okxSwapCandidate struct {
	pos        OKXPosition
	uplRatio   float64
	entryScore float64
}

// okxTrySwapForSlot 满仓换仓主流程：满仓且新信号为高分时，平掉在场最弱浮亏仓腾槽。
// 四道守门缺一不可：
//  1. 高分门槛：score ≥ HighScore——低分信号不配拆仓，留着仓位等更好的信号；
//  2. 日限额：本环境今日换仓次数 < SwapMaxPerDay——防震荡日反复换仓磨损手续费；
//  3. 只截亏：候选浮亏率 ≤ −SwapLossPct%——不卖盈利/浅亏仓追高，浮亏最深的优先被换；
//  4. 分差守门：score − 候选入场分 ≥ SwapGap——边际优势须付得起平仓+开仓双份手续费。
//
// 候选按（浮亏率升序、入场分升序）排序后遍历，首个全过守门者被平仓；浮亏最深的
// 都未达截亏线则更浅的无望，直接放弃；分差不足继续看下一个候选。
// 换出成功后：被平合约登记同向冷却+撤孤儿条件单（okxSwapClosePosition 内完成），
// 新合约登记换入保护期。返回（是否换出成功, 说明）：未换成功时说明为
// 「（换仓未触发：…）」格式，由调用方拼到拒绝原因后落 skipped 流水；
// 机制关闭/未满仓/查询失败等场景返回空说明（不附加噪音）。
func okxTrySwapForSlot(simulated bool, cfg okxSignalTradeConf, r models.SignalRecord, score float64, newInstId string) (bool, string) {
	// 总开关：截亏线或分差任一 ≤0 即关闭（避免零分差无优势 churn、零截亏线卖盈利仓）
	if cfg.SwapLossPct <= 0 || cfg.SwapGap <= 0 {
		return false, ""
	}
	if cfg.MaxPositions <= 0 {
		return false, "" // 未启用最大持仓数限制时不存在「满仓」，风控不会因持仓数拒绝
	}
	poss, err := GetOKXPositionListForEnv(simulated, "")
	if err != nil {
		return false, "" // 持仓查不到时风控本身已落拒绝说明，换仓无从谈起
	}
	envLabel := okxTradeEnvLabel(simulated)
	cnt := 0
	var cands []okxSwapCandidate
	for _, p := range poss {
		if okxFloatNum(p.Pos) == 0 {
			continue
		}
		cnt++
		// 新信号自己的合约不作候选（同向持仓早已跳过、反向属换向不占新槽）；
		// 换入保护期内的仓不作候选（防连环 churn）。
		if p.InstId == newInstId || okxSwapFreshRemaining(simulated, p.InstId) > 0 {
			continue
		}
		cands = append(cands, okxSwapCandidate{pos: p, uplRatio: okxFloatNum(p.UplRatio)})
	}
	if cnt < cfg.MaxPositions {
		return false, "" // 未满仓不换仓：预留槽位区间被拒的信号本就该等槽位，而非拆别人的仓
	}
	// 守门 1：高分门槛
	if score < cfg.HighScore {
		return false, fmt.Sprintf("（换仓未触发：信号 %.0f 分低于高分门槛 %.0f 分，不值得拆仓）", score, cfg.HighScore)
	}
	// 守门 2：日限额
	if cfg.SwapMaxPerDay > 0 {
		if n := okxSwapCountToday(simulated); n >= cfg.SwapMaxPerDay {
			return false, fmt.Sprintf("（换仓未触发：今日已换仓 %d 次达单日上限 %d 次）", n, cfg.SwapMaxPerDay)
		}
	}
	if len(cands) == 0 {
		return false, "（换仓未触发：满仓但无合格候选仓（新合约自身或均在换入保护期内））"
	}
	// 补齐入场分并按（浮亏深度、入场分）排序：最弱的最先被评估
	for i := range cands {
		cands[i].entryScore = okxEntryScoreOf(simulated, cands[i].pos.InstId)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].uplRatio != cands[j].uplRatio {
			return cands[i].uplRatio < cands[j].uplRatio
		}
		return cands[i].entryScore < cands[j].entryScore
	})
	lossLine := -cfg.SwapLossPct / 100
	for _, c := range cands {
		if c.uplRatio > lossLine {
			// 已按浮亏深度排序：本轮候选未达截亏线，其后更浅的更无望——只截亏，不卖盈利/浅亏仓
			return false, fmt.Sprintf("（换仓未触发：候选仓 %s 浮动收益率 %.1f%%，未达截亏线 −%.0f%%，不卖盈利/浅亏仓）",
				c.pos.InstId, c.uplRatio*100, cfg.SwapLossPct)
		}
		if score-c.entryScore < cfg.SwapGap {
			continue // 分差不足，看下一个候选（浮亏更浅但入场分可能更低）
		}
		reason := fmt.Sprintf("换仓：平掉 %s（入场 %.0f 分，浮动收益率 %.1f%%）腾槽给 %.0f 分信号 %s",
			c.pos.InstId, c.entryScore, c.uplRatio*100, score, newInstId)
		if !okxSwapClosePosition(simulated, cfg, r, c, reason) {
			return false, "（换仓失败：平仓单被拒，详见 swap 流水，下轮信号自动重试）"
		}
		// 新合约开启换入保护期：刚换入的仓 1 小时内不作为被换出候选
		okxMarkSwapFresh(simulated, newInstId)
		logger.SugaredLogger.Infof("OKX满仓换仓[%s]: %s → %s；%s", envLabel, c.pos.InstId, newInstId, reason)
		return true, reason
	}
	return false, fmt.Sprintf("（换仓未触发：候选仓入场分与 %.0f 分新信号的分差均不足 %.0f 分）", score, cfg.SwapGap)
}

// okxSwapClosePosition 平掉被换出的最弱仓（ReduceOnly 市价单，仿反向平仓执行口径），
// 落 swap 族流水：Kind="close"，Code/BarTime/SignalScore 为新信号上下文、InstId 为被平
// 合约、成功时 ErrMsg 记换仓原因（仿 TPSL 流水做法），面板可据此追溯「哪笔新信号换掉了
// 哪个仓」。成功后登记被平方向冷却（防该合约同向信号立刻重进）并异步撤孤儿条件单。
func okxSwapClosePosition(simulated bool, cfg okxSignalTradeConf, r models.SignalRecord, c okxSwapCandidate, reason string) bool {
	envLabel := okxTradeEnvLabel(simulated)
	p := c.pos
	posVal := okxFloatNum(p.Pos)
	if posVal == 0 {
		return false
	}
	// 平仓方向判定：short 空头买回、long 多头卖出；net 单向模式按仓位符号
	//（pos<0 空头 → 买回，pos>0 多头 → 卖出）
	var closeSide, closePosSide, closedDir string // closedDir=被平头寸方向（多头=buy/空头=sell）
	switch {
	case p.PosSide == "short":
		closeSide, closePosSide, closedDir = "buy", "short", "sell"
	case p.PosSide == "long":
		closeSide, closePosSide, closedDir = "sell", "long", "buy"
	default:
		closedDir = "sell"
		closeSide, closePosSide = "buy", "net"
		if posVal > 0 {
			closedDir, closeSide = "buy", "sell"
		}
	}
	ro := true
	closeReq := &OKXOrderRequest{
		InstId:     p.InstId,
		TdMode:     p.MgnMode,
		Side:       closeSide,
		PosSide:    closePosSide,
		OrdType:    "market",
		Sz:         strconv.FormatFloat(math.Abs(posVal), 'f', -1, 64),
		ReduceOnly: &ro,
	}
	if strings.TrimSpace(closeReq.TdMode) == "" {
		closeReq.TdMode = "cross"
	}
	// 参考价：优先标记价（平仓时点估值），兜底开仓均价
	refPx := okxFloatNum(p.MarkPx)
	if refPx <= 0 {
		refPx = okxFloatNum(p.AvgPx)
	}
	instType := "SPOT"
	if strings.HasSuffix(strings.ToUpper(p.InstId), "-SWAP") {
		instType = "SWAP"
	}
	res, err := placeOKXOrderWithEnv(closeReq, simulated)
	rec := models.SignalTradeRecord{
		Code:        r.Code,
		InstId:      p.InstId,
		InstType:    instType,
		Klt:         r.Klt,
		Family:      "swap",
		Kind:        "close",
		BarTime:     r.BarTime,
		SignalPrice: signalPriceOf(r),
		SignalScore: signalScoreOf(r),
		Name:        r.Name,
		Side:        closeSide,
		OrdType:     "market",
		Sz:          closeReq.Sz,
		RefPrice:    refPx,
		Env:         envLabel,
		AmountUSDT:  okxNotionalBySz(p.InstId, closeReq.Sz, refPx),
		At:          time.Now().UnixMilli(),
	}
	if err != nil {
		rec.Status = "failed"
		rec.ErrMsg = fmt.Sprintf("%s；平仓失败: %v", reason, err)
		okxSaveSignalTradeRecord(rec)
		logger.SugaredLogger.Warnf("OKX满仓换仓平仓失败[%s]: instId=%s posSide=%s sz=%s err=%v",
			envLabel, p.InstId, p.PosSide, closeReq.Sz, err)
		return false
	}
	rec.Status = "success"
	rec.OrdId = res.OrdId
	rec.ErrMsg = reason
	okxSaveSignalTradeRecord(rec)
	// 平仓成功：被平方向登记冷却（防同向信号立刻重进原合约）+ 撤孤儿条件单
	okxMarkTPSLCooldown(simulated, p.InstId, closedDir, cfg.ReentryCooldown)
	go okxCancelInstPendingAlgos(simulated, p.InstId)
	logger.SugaredLogger.Infof("OKX满仓换仓平仓[%s]: %s posSide=%s pos=%s ordId=%s；%s",
		envLabel, p.InstId, p.PosSide, p.Pos, res.OrdId, reason)
	return true
}

// ErrBelowMinSz 名义金额不足以买入最小下单量（lotSz 步进对齐后仍不足 minSz）。
// 该类信号属正常情况（金额小/合约面值大），落「skipped」流水而非 failed，便于面板区分统计。
var ErrBelowMinSz = errors.New("低于最小下单量")

// okxCalcSizeByAmount 按名义金额与最新价换算下单数量（SWAP 为张，SPOT 为基础币），
// 向下取整到 lotSz 步进并校验 minSz；不足最小下单量返回 ErrBelowMinSz，
// 错误信息带每张面值与最低所需名义金额，避免「最小 1 张 × 现价 0.03」式的误读。
func okxCalcSizeByAmount(notionalUSDT, price float64, ins OKXInstrument) (string, error) {
	if price <= 0 || notionalUSDT <= 0 {
		return "", errors.New("金额或价格无效")
	}
	unit := price // SPOT：1 个基础币的价值
	ctVal := 1.0
	if ins.InstType == "SWAP" {
		ctVal = okxFloatNum(ins.CtVal)
		if ctVal <= 0 {
			ctVal = 1
		}
		unit = ctVal * price // 一张合约的价值
	}
	sz := math.Floor(notionalUSDT/unit/okxLotStep(ins)) * okxLotStep(ins)
	if sz < okxFloatNum(ins.MinSz) {
		minAmt := okxFloatNum(ins.MinSz) * unit
		if ins.InstType == "SWAP" {
			return "", fmt.Errorf("%w %s 张（每张=面值 %.10g × 现价 %.6g，最低需名义 %.6g USDT）",
				ErrBelowMinSz, ins.MinSz, ctVal, price, minAmt)
		}
		return "", fmt.Errorf("%w %s（按现价 %.6g，最低需 %.6g USDT）", ErrBelowMinSz, ins.MinSz, price, minAmt)
	}
	// 按 lotSz 小数位格式化，避免浮点乘法产生 0.30000000000000004 这类脏精度
	return strconv.FormatFloat(sz, 'f', okxLotDecimals(ins), 64), nil
}

// okxLotDecimals 从 lotSz 字符串推断下单数量小数位（如 "0.1"→1、"1"→0、"0.0100"→2）。
func okxLotDecimals(ins OKXInstrument) int {
	s := strings.TrimSpace(ins.LotSz)
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return 0
	}
	dec := len(s) - dot - 1
	for dec > 0 && s[dot+dec] == '0' { // 去掉小数尾零
		dec--
	}
	return dec
}

// okxClampLever 将配置杠杆收敛到合约最大可设杠杆（ins.Lever，如 "5"）：
// 全局配置 20x 而合约上限 5x 时按 5x 下单，避免 OKX 59102 杠杆超限导致整笔交易失败。
// 上限未知（为空/非法）时按原配置返回。
func okxClampLever(cfgLever string, ins *OKXInstrument) string {
	if ins == nil {
		return cfgLever
	}
	maxL := okxFloatNum(ins.Lever)
	if maxL > 0 && okxFloatNum(cfgLever) > maxL {
		return strconv.FormatFloat(maxL, 'f', -1, 64)
	}
	return cfgLever
}

func okxLotStep(ins OKXInstrument) float64 {
	if lot := okxFloatNum(ins.LotSz); lot > 0 {
		return lot
	}
	return 1
}

func signalPriceOf(r models.SignalRecord) float64 {
	if r.Price == nil {
		return 0
	}
	return *r.Price
}

func signalScoreOf(r models.SignalRecord) float64 {
	if r.Score == nil {
		return 0
	}
	return *r.Score
}

func okxSaveSignalTradeRecord(rec models.SignalTradeRecord) {
	if db.Dao == nil {
		return
	}
	if err := db.Dao.Create(&rec).Error; err != nil {
		logger.SugaredLogger.Errorf("保存OKX信号交易流水失败: %v", err)
	}
}

// OKXSignalTradeService 模拟交易流水查询。
type OKXSignalTradeService struct{}

func NewOKXSignalTradeService() *OKXSignalTradeService {
	return &OKXSignalTradeService{}
}

// GetSignalTradePage 分页查询模拟交易流水。
func (s *OKXSignalTradeService) GetSignalTradePage(query models.SignalTradeQuery) (*models.SignalTradePageData, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 200 {
		query.PageSize = 20
	}
	dbQuery := db.Dao.Model(&models.SignalTradeRecord{})
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		dbQuery = dbQuery.Where("code LIKE ? OR inst_id LIKE ? OR name LIKE ?", like, like, like)
	}
	if st := strings.TrimSpace(query.Status); st == "success" || st == "failed" || st == "skipped" {
		dbQuery = dbQuery.Where("status = ?", st)
	}
	// 环境过滤：面板模拟盘池/实盘池页签各看各的流水（Env 落库值即「模拟盘」/「实盘」）
	if env := strings.TrimSpace(query.Env); env == "模拟盘" || env == "实盘" {
		dbQuery = dbQuery.Where("env = ?", env)
	}
	if query.StartTime > 0 {
		dbQuery = dbQuery.Where("bar_time >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		dbQuery = dbQuery.Where("bar_time <= ?", query.EndTime)
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, err
	}
	list := make([]models.SignalTradeRecord, 0)
	if err := dbQuery.Order("at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&list).Error; err != nil {
		return nil, err
	}
	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}
	return &models.SignalTradePageData{
		List: list, Total: total, Page: query.Page, PageSize: query.PageSize, TotalPages: totalPages,
	}, nil
}

// ClearSignalTradeRecords 清空模拟交易流水；env 为「模拟盘」/「实盘」时只清该环境，空=全部。
func (s *OKXSignalTradeService) ClearSignalTradeRecords(env string) error {
	if db.Dao == nil {
		return gorm.ErrInvalidDB
	}
	tx := db.Dao.Session(&gorm.Session{AllowGlobalUpdate: true})
	if e := strings.TrimSpace(env); e == "模拟盘" || e == "实盘" {
		tx = tx.Where("env = ?", e)
	}
	return tx.Delete(&models.SignalTradeRecord{}).Error
}
