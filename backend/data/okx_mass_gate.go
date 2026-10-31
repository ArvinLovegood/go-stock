package data

import (
	"math"
	"sort"
	"sync"
	"time"
)

// 信号自动交易的 Mass Index 反转闸门。
//
// Mass Index(9,9,25) 的算法：range=high-low → EMA(9) → 再 EMA(9) → 两者比值 → EMA(9) → 25 期滚动求和。
// 经典用法：MI 抬升到 27 以上（区间扩张后的「凸起」）再回落到 26.5 以下，视为趋势反转临近；
// 前端 K 线在主图上用琥珀色竖带标出该窗口（见 frontend/src/components/kline/massZonePrimitive.js）。
//
// 该指标只描述「区间扩张是否已衰竭」，本身不指明方向，因此不适合当作方向过滤，
// 而是作为「开仓闸门」：凸起期间与回落确认后的少量根内不开新仓；平仓永远放行，
// 避免只做多/只做空模式下持仓被门控锁死。
//
// 阈值自适应：MI 是「单 EMA / 二次 EMA」的比值求和，本身无量纲——绝对波动率（价格高低、振幅宽窄）
// 不再影响它的数值，所以 27/26.5 对绝大多数品种直接可用。不同品种真正的差异在 MI 的分布：
// 区间扩张更持续、跳空更频繁的品种（大量山寨币）MI 长期偏高，固定 27 会被频繁触发而几乎持续拦截；
// 低波动品种/短周期（如 1 分钟线的横盘合约）MI 长期低于 27，固定 27 则一个窗口都抓不到。
// 因此进入线取「该合约自身 MI 历史的 90% 分位」，离开线固定低 0.5（保留迟滞防止窗口抖动），
// 并把进入线钳制在 [24, 31]：上限防极端品种把线抬到永不触发，下限 24 防止过度灵敏把噪声当信号；
// 样本不足时回退 27/26.5。
//
// 取不到行情（新上市合约预热不足、接口异常）时返回「正常」按不拦截处理，
// 与 okxTrendRegime 的处理一致：行情异常不应把全部信号静默吃掉。

const (
	// okxMassGateTTL 判定结果缓存时长：1H 指标不需要秒级刷新，5 分钟足够。
	okxMassGateTTL = 5 * time.Minute
	// okxMassEnter / okxMassExit 经典阈值，同时作为样本不足时的回退值（带 0.5 迟滞）。
	okxMassEnter = 27.0
	okxMassExit  = 26.5
	// okxMassCoolBars 反转确认（MI 回落到离开线以下）之后仍拦截的 1H 根数：
	// 给转折留出确认时间，避免在回落的同一根上按旧趋势开仓。
	okxMassCoolBars = 2
	// okxMassEmaPeriod / okxMassSumPeriod 与前端 massIndexValues(highs, lows) 的默认参数一致。
	okxMassEmaPeriod = 9
	okxMassSumPeriod = 25

	// okxMassQuantile 进入线取自身 MI 历史的分位点。
	okxMassQuantile = 0.90
	// okxMassHysteresis 离开线 = 进入线 − 该值（与经典 27/26.5 的 0.5 间隔一致）。
	okxMassHysteresis = 0.5
	// okxMassMinEnter / okxMassMaxEnter 进入线钳制区间：上限防止极端品种把线抬到
	// 永不触发的失效高度；下限 24 保证低波动品种也能出信号，又不至于把常态 MI
	// （25 附近）整段都判成窗口。
	okxMassMinEnter = 24.0
	okxMassMaxEnter = 31.0
	// okxMassMinSamples 有效 MI 样本数低于该值时回退固定阈值（分位数在小样本上不可靠）。
	okxMassMinSamples = 120
)

// Mass Index 闸门状态。
const (
	okxMassNormal   = 0 // 正常，放行
	okxMassBulge    = 1 // 反转窗口进行中（MI 已越过进入线，尚未跌破离开线）
	okxMassCooldown = 2 // 反转刚确认（MI 已跌破离开线），仍处冷却期
)

type okxMassGateEntry struct {
	state  int
	enter  float64
	exit   float64
	expiry time.Time
}

var (
	okxMassGateMu    sync.Mutex
	okxMassGateCache = map[string]okxMassGateEntry{}
)

// okxMassGateState 返回该合约自身 1H 的 Mass Index 闸门状态与该合约自适应阈值，结果按合约缓存 5 分钟。
func okxMassGateState(instId string) (state int, enter, exit float64) {
	if instId == "" {
		return okxMassNormal, okxMassEnter, okxMassExit
	}
	okxMassGateMu.Lock()
	if e, ok := okxMassGateCache[instId]; ok && time.Now().Before(e.expiry) {
		okxMassGateMu.Unlock()
		return e.state, e.enter, e.exit
	}
	okxMassGateMu.Unlock()

	state = okxMassNormal
	enter, exit = okxMassEnter, okxMassExit
	highs := make([]float64, 0, 600)
	lows := make([]float64, 0, 600)
	for _, c := range okxMassCandles(instId) {
		// 未收盘的那根不参与计算，否则同分钟反复扫描会出现状态抖动
		if !c.Confirm || c.High <= 0 || c.Low <= 0 {
			continue
		}
		highs = append(highs, c.High)
		lows = append(lows, c.Low)
	}
	if len(highs) > 0 {
		mass := okxMassIndex(highs, lows)
		// 阈值只取决于分布、与当前是否在窗口内无关，故先算阈值再判状态
		enter, exit = okxMassAdaptiveThresholds(mass)
		state = okxMassTailState(mass, enter, exit, okxMassCoolBars)
	}

	okxMassGateMu.Lock()
	okxMassGateCache[instId] = okxMassGateEntry{state: state, enter: enter, exit: exit, expiry: time.Now().Add(okxMassGateTTL)}
	okxMassGateMu.Unlock()
	return state, enter, exit
}

// okxMassCandles 拉取用于 MI 判定的 1H K 线：先取最近 300 根（OKX 上限），
// 已饱和时再按 after 游标往前翻一页，让分位数估计有更多样本；未饱和说明已到上市起点，直接用。
func okxMassCandles(instId string) []OKXCandle {
	recent := GetOKXCandles(instId, "1H", 300, 0)
	if len(recent) == 0 {
		return nil
	}
	if len(recent) < 300 {
		return recent
	}
	older := GetOKXCandles(instId, "1H", 300, recent[0].Ts)
	if len(older) == 0 {
		return recent
	}
	out := make([]OKXCandle, 0, len(older)+len(recent))
	out = append(out, older...)
	out = append(out, recent...)
	return out
}

// okxMassIndex 复刻前端的 massIndexValues(highs, lows, 9, 9, 25)。
// 返回值与输入等长，用 0 表示「该根无效」（预热不足或中间断档）——MI 恒为正数，0 不会与有效值混淆。
func okxMassIndex(highs, lows []float64) []float64 {
	n := len(highs)
	mass := make([]float64, n)
	if n == 0 {
		return mass
	}
	ranges := make([]float64, n)
	for i := 0; i < n; i++ {
		ranges[i] = highs[i] - lows[i]
	}
	single := okxMassEMA(ranges, okxMassEmaPeriod)
	dbl := okxMassEMA(single, okxMassEmaPeriod)
	ratio := make([]float64, n)
	for i := 0; i < n; i++ {
		if single[i] > 0 && dbl[i] > 0 {
			ratio[i] = single[i] / dbl[i]
		}
	}
	ratioEMA := okxMassEMA(ratio, okxMassEmaPeriod)
	for i := okxMassSumPeriod - 1; i < n; i++ {
		sum := 0.0
		ok := true
		for j := 0; j < okxMassSumPeriod; j++ {
			v := ratioEMA[i-j]
			if v <= 0 {
				ok = false
				break
			}
			sum += v
		}
		if ok {
			mass[i] = sum
		}
	}
	return mass
}

// okxMassEMA 与前端 emaFinite 同语义：前 period 个有效值用简单均值播种，之后按 k=2/(period+1) 递推。
// 输入/输出用 0 表示无效值（无效值跳过，但保留已播种的 EMA 状态）。
func okxMassEMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 {
		return out
	}
	k := 2.0 / float64(period+1)
	ema := 0.0
	seedSum := 0.0
	seedCnt := 0
	for i, v := range values {
		if v <= 0 {
			continue
		}
		if ema == 0 {
			seedSum += v
			seedCnt++
			if seedCnt < period {
				continue
			}
			ema = seedSum / float64(period)
			out[i] = ema
			continue
		}
		ema = v*k + ema*(1-k)
		out[i] = ema
	}
	return out
}

// okxMassAdaptiveThresholds 按 MI 历史分布给出该合约的进入/离开阈值：
// 进入线 = MI 的 90% 分位（钳制在 [24, 31]），离开线 = 进入线 − 0.5；
// 有效样本不足（okxMassMinSamples）时回退经典 27/26.5。
func okxMassAdaptiveThresholds(mass []float64) (enter, exit float64) {
	vals := make([]float64, 0, len(mass))
	for _, v := range mass {
		if v > 0 {
			vals = append(vals, v)
		}
	}
	if len(vals) < okxMassMinSamples {
		return okxMassEnter, okxMassExit
	}
	sort.Float64s(vals)
	enter = okxMassQuantileValue(vals, okxMassQuantile)
	if enter < okxMassMinEnter {
		enter = okxMassMinEnter
	}
	if enter > okxMassMaxEnter {
		enter = okxMassMaxEnter
	}
	exit = enter - okxMassHysteresis
	return enter, exit
}

// okxMassQuantileValue 最近秩法分位数（p ∈ (0,1]），sorted 必须已升序。
func okxMassQuantileValue(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// okxMassTailState 从 MI 序列尾部判定闸门状态：
//   - 末尾仍处于凸起（进入过 ≥ enter 且尚未 < exit）→ okxMassBulge；
//   - 末尾刚跌破 exit，且距确认根不足 coolBars 根 → okxMassCooldown；
//   - 其余（从未凸起 / 已冷却结束）→ okxMassNormal。
func okxMassTailState(mass []float64, enter, exit float64, coolBars int) int {
	inWindow := false
	lastExit := -1
	lastValid := -1
	for i, v := range mass {
		if v <= 0 {
			continue
		}
		lastValid = i
		if !inWindow {
			if v >= enter {
				inWindow = true
			}
			continue
		}
		if v < exit {
			inWindow = false
			lastExit = i
		}
	}
	switch {
	case lastValid < 0:
		return okxMassNormal
	case inWindow:
		return okxMassBulge
	case lastExit >= 0 && lastValid-lastExit < coolBars:
		return okxMassCooldown
	default:
		return okxMassNormal
	}
}
