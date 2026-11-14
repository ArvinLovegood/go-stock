package data

/**
 * 买卖点共振算法（Go 版）——与前端 kline/calc.ts 的 buySellPointsValues 逐行同口径。
 *
 * 背景：图上箭头与监控流水由前端 TS 算法负责（单一实现原则）；本文件专为「全市场扫描器」
 * （market_signal_scanner.go）服务——几千只票批量打分不可能逐只走前端绑定调用，
 * 故在 Go 侧复刻一份完全相同的算法。口径与 TS 版保持逐行对齐，任何一侧调整必须两侧同步。
 *
 * 约定：无效值用 NaN 表示（对应 TS 版数组里的 null/undefined），判断统一走 isNaN。
 * 14 路基础信号分四组：0=震荡(osc) 1=动量(mom) 2=量价(vol) 3=趋势(trend)，等权计数、分组门控，
 * 详见 TS 版函数头注释（此处不重复粘贴长注释）。
 *
 * 4 路扩展信号（仅日K类周期启用，前端图表侧以指标信号展示，此处接入共振打分）：
 *   BIAS 乖离率（组0，前端 biasValues）、K线形态（组0，candlePatternsValues）、
 *   获利盘比例（组2，chip.ts profitRatioSeries）、多周期共振（组3，multiTimeframe.ts mtfScoreBundle）。
 * 扩展启用时满分 14→18，命中门限按 18/14 等比放大以保持灵敏度档位语义。
 * RS 相对强度需逐只拉基准指数数据，全市场扫描成本过高，不接入打分。
 */

import (
	"math"
	"sort"
	"time"
)

// ---- 基础指标（与 calc.ts 同名函数同口径） ----

func isNaN(v float64) bool { return math.IsNaN(v) }

func nanSlice(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// smaValues 简单均线（前 period-1 根为 NaN）
func smaValues(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	for i := period - 1; i < len(values); i++ {
		s := 0.0
		for j := 0; j < period; j++ {
			s += values[i-j]
		}
		out[i] = s / float64(period)
	}
	return out
}

// emaFinite EMA：输入为 NaN 的根输出 NaN；首个有效 EMA = 前 period 根的 SMA（对应 emaFinite）
func emaFinite(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	k := 2.0 / float64(period+1)
	var ema float64 = math.NaN()
	for i, v := range values {
		if isNaN(v) {
			continue
		}
		if isNaN(ema) {
			if i < period-1 {
				continue
			}
			s := 0.0
			ok := true
			for j := i - period + 1; j <= i; j++ {
				if isNaN(values[j]) {
					ok = false
					break
				}
				s += values[j]
			}
			if !ok {
				continue
			}
			ema = s / float64(period)
			out[i] = ema
		} else {
			ema = v*k + ema*(1-k)
			out[i] = ema
		}
	}
	return out
}

// emaLeadingNull EMA：跳过 NaN 输入、累计满 period 个有效值才起步（对应 emaLeadingNull）
func emaLeadingNull(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	k := 2.0 / float64(period+1)
	var ema float64 = math.NaN()
	sum, cnt := 0.0, 0
	for i, v := range values {
		if isNaN(v) {
			continue
		}
		if isNaN(ema) {
			sum += v
			cnt++
			if cnt < period {
				continue
			}
			if cnt == period {
				ema = sum / float64(period)
				out[i] = ema
			}
		} else {
			ema = v*k + ema*(1-k)
			out[i] = ema
		}
	}
	return out
}

// macdBundle MACD(12,26,9)：返回 dif/dea
func macdBundle(closes []float64) (dif, dea []float64) {
	ema12 := emaFinite(closes, 12)
	ema26 := emaFinite(closes, 26)
	dif = nanSlice(len(closes))
	for i := range closes {
		if !isNaN(ema12[i]) && !isNaN(ema26[i]) {
			dif[i] = ema12[i] - ema26[i]
		}
	}
	dea = emaLeadingNull(dif, 9)
	return
}

// kdjBundle KDJ(9)：RSV 平滑，窗口不足的根保持 NaN
func kdjBundle(highs, lows, closes []float64, n int) (kk, dd []float64) {
	length := len(closes)
	rsv := nanSlice(length)
	for i := n - 1; i < length; i++ {
		hn, ln := math.Inf(-1), math.Inf(1)
		for j := 0; j < n; j++ {
			hn = math.Max(hn, highs[i-j])
			ln = math.Min(ln, lows[i-j])
		}
		c := closes[i]
		if hn == ln {
			rsv[i] = 50
		} else {
			rsv[i] = (c - ln) / (hn - ln) * 100
		}
	}
	kk, dd = nanSlice(length), nanSlice(length)
	pk, pd := 50.0, 50.0
	for i := 0; i < length; i++ {
		r := rsv[i]
		if isNaN(r) {
			continue
		}
		pk = (2*pk + r) / 3
		pd = (2*pd + pk) / 3
		kk[i] = pk
		dd[i] = pd
	}
	return
}

// rsiValues RSI(14)（简单求和口径）
func rsiValues(closes []float64, period int) []float64 {
	out := nanSlice(len(closes))
	for i := period; i < len(closes); i++ {
		gain, loss := 0.0, 0.0
		for j := 0; j < period; j++ {
			ch := closes[i-j] - closes[i-j-1]
			if ch >= 0 {
				gain += ch
			} else {
				loss -= ch
			}
		}
		ag := gain / float64(period)
		al := loss / float64(period)
		if al == 0 {
			out[i] = 100
		} else {
			out[i] = 100 - 100/(1+ag/al)
		}
	}
	return out
}

// atrValues ATR(14)（Wilder 递推）
func atrValues(highs, lows, closes []float64, period int) []float64 {
	length := len(closes)
	out := nanSlice(length)
	if length < 2 {
		return out
	}
	tr := make([]float64, length)
	tr[0] = highs[0] - lows[0]
	for i := 1; i < length; i++ {
		tr[i] = math.Max(highs[i]-lows[i], math.Max(math.Abs(highs[i]-closes[i-1]), math.Abs(lows[i]-closes[i-1])))
	}
	if length < period {
		return out
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < length; i++ {
		out[i] = (out[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return out
}

// vwapValues 滚动 20 根 VWAP（日K类周期用）
func vwapValues(highs, lows, closes, vols []float64, period int) []float64 {
	out := nanSlice(len(closes))
	for i := period - 1; i < len(closes); i++ {
		sumPV, sumV := 0.0, 0.0
		for j := 0; j < period; j++ {
			tp := (highs[i-j] + lows[i-j] + closes[i-j]) / 3
			sumPV += tp * vols[i-j]
			sumV += vols[i-j]
		}
		if sumV > 0 {
			out[i] = sumPV / sumV
		}
	}
	return out
}

// cumulativeVwapByDay 当日累计均价线（分时本义），前 warmup 根不输出（对应 calc.ts cumulativeVwapByDay）
func cumulativeVwapByDay(highs, lows, closes, vols []float64, dayKeys []string, warmup int) []float64 {
	out := nanSlice(len(closes))
	curDay := ""
	cumPV, cumV := 0.0, 0.0
	cnt := 0
	for i := 0; i < len(closes); i++ {
		if i >= len(dayKeys) || dayKeys[i] != curDay {
			curDay = dayKeys[i]
			cumPV, cumV, cnt = 0, 0, 0
		}
		tp := (highs[i] + lows[i] + closes[i]) / 3
		cumPV += tp * vols[i]
		cumV += vols[i]
		cnt++
		if cnt >= warmup && cumV > 0 {
			out[i] = cumPV / cumV
		}
	}
	return out
}

// mfiValues MFI(14)（量加权 RSI）
func mfiValues(highs, lows, closes, vols []float64, period int) []float64 {
	length := len(closes)
	out := nanSlice(length)
	if length < 2 {
		return out
	}
	tp := make([]float64, length)
	mf := make([]float64, length)
	for i := 0; i < length; i++ {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
		mf[i] = tp[i] * vols[i]
	}
	for i := period; i < length; i++ {
		posMF, negMF := 0.0, 0.0
		for j := 0; j < period; j++ {
			idx := i - j
			if tp[idx] > tp[idx-1] {
				posMF += mf[idx]
			} else if tp[idx] < tp[idx-1] {
				negMF += mf[idx]
			}
		}
		if negMF == 0 {
			out[i] = 100
		} else {
			out[i] = 100 - 100/(1+posMF/negMF)
		}
	}
	return out
}

// cmfValues CMF(20)（CLV 资金流向）
func cmfValues(highs, lows, closes, vols []float64, period int) []float64 {
	out := nanSlice(len(closes))
	for i := period - 1; i < len(closes); i++ {
		sumMFV, sumVol := 0.0, 0.0
		for j := 0; j < period; j++ {
			idx := i - j
			rng := highs[idx] - lows[idx]
			mfv := 0.0
			if rng > 0 {
				mfv = ((closes[idx] - lows[idx]) - (highs[idx] - closes[idx])) / rng * vols[idx]
			}
			sumMFV += mfv
			sumVol += vols[idx]
		}
		if sumVol > 0 {
			out[i] = sumMFV / sumVol
		}
	}
	return out
}

// cciValues CCI(20)
func cciValues(highs, lows, closes []float64, period int) []float64 {
	length := len(closes)
	out := nanSlice(length)
	tp := make([]float64, length)
	for i := 0; i < length; i++ {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
	}
	for i := period - 1; i < length; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += tp[i-j]
		}
		mean := sum / float64(period)
		meanDev := 0.0
		for j := 0; j < period; j++ {
			meanDev += math.Abs(tp[i-j] - mean)
		}
		meanDev /= float64(period)
		if meanDev > 0 {
			out[i] = (tp[i] - mean) / (0.015 * meanDev)
		}
	}
	return out
}

// donchianChannel 唐奇安通道（返回 upper/lower）
func donchianChannel(highs, lows []float64, period int) (upper, lower []float64) {
	length := len(highs)
	upper, lower = nanSlice(length), nanSlice(length)
	for i := period - 1; i < length; i++ {
		hi, lo := math.Inf(-1), math.Inf(1)
		for j := 0; j < period; j++ {
			hi = math.Max(hi, highs[i-j])
			lo = math.Min(lo, lows[i-j])
		}
		upper[i] = hi
		lower[i] = lo
	}
	return
}

// adxBundle ADX(14) + DI（Wilder 平滑）
func adxBundle(highs, lows, closes []float64, period int) (adx, diP, diM []float64) {
	length := len(closes)
	adx, diP, diM = nanSlice(length), nanSlice(length), nanSlice(length)
	if length < 2 {
		return
	}
	tr := make([]float64, length)
	plusDM := make([]float64, length)
	minusDM := make([]float64, length)
	tr[0] = highs[0] - lows[0]
	for i := 1; i < length; i++ {
		tr[i] = math.Max(highs[i]-lows[i], math.Max(math.Abs(highs[i]-closes[i-1]), math.Abs(lows[i]-closes[i-1])))
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
	}
	smoothTR, smoothPDM, smoothMDM := nanSlice(length), nanSlice(length), nanSlice(length)
	sTR, sPDM, sMDM := 0.0, 0.0, 0.0
	for i := 0; i < period && i < length; i++ {
		sTR += tr[i]
		sPDM += plusDM[i]
		sMDM += minusDM[i]
	}
	if length < period {
		return
	}
	smoothTR[period-1], smoothPDM[period-1], smoothMDM[period-1] = sTR, sPDM, sMDM
	for i := period; i < length; i++ {
		smoothTR[i] = smoothTR[i-1] - smoothTR[i-1]/float64(period) + tr[i]
		smoothPDM[i] = smoothPDM[i-1] - smoothPDM[i-1]/float64(period) + plusDM[i]
		smoothMDM[i] = smoothMDM[i-1] - smoothMDM[i-1]/float64(period) + minusDM[i]
	}
	dx := nanSlice(length)
	for i := 0; i < length; i++ {
		if !isNaN(smoothTR[i]) && smoothTR[i] > 0 {
			diP[i] = 100 * smoothPDM[i] / smoothTR[i]
			diM[i] = 100 * smoothMDM[i] / smoothTR[i]
			sum := diP[i] + diM[i]
			if sum > 0 {
				dx[i] = 100 * math.Abs(diP[i]-diM[i]) / sum
			} else {
				dx[i] = 0
			}
		}
	}
	if length >= period*2-1 {
		sumDx := 0.0
		for i := period - 1; i < period*2-1 && i < length; i++ {
			sumDx += dxOrZero(dx[i])
		}
		adx[period*2-2] = sumDx / float64(period)
		for i := period*2 - 1; i < length; i++ {
			adx[i] = (adx[i-1]*float64(period-1) + dxOrZero(dx[i])) / float64(period)
		}
	}
	return
}

func dxOrZero(v float64) float64 {
	if isNaN(v) {
		return 0
	}
	return v
}

// supertrendDirection Supertrend(10,3) 方向序列（0=ATR 未就绪）
func supertrendDirection(highs, lows, closes []float64, atrPeriod int, multiplier float64) []int {
	length := len(closes)
	direction := make([]int, length)
	atr := atrValues(highs, lows, closes, atrPeriod)
	var prevUpper, prevLower float64 = math.NaN(), math.NaN()
	prevDir := 0
	for i := 0; i < length; i++ {
		if isNaN(atr[i]) {
			continue
		}
		hl2 := (highs[i] + lows[i]) / 2
		rawUpper := hl2 + multiplier*atr[i]
		rawLower := hl2 - multiplier*atr[i]
		if !isNaN(prevUpper) && rawUpper >= prevUpper && closes[i-1] <= prevUpper {
			rawUpper = prevUpper
		}
		if !isNaN(prevLower) && rawLower <= prevLower && closes[i-1] >= prevLower {
			rawLower = prevLower
		}
		var dir int
		if prevDir == 0 {
			dir = 1
		} else if prevDir == 1 {
			dir = -1
			if closes[i] >= rawLower {
				dir = 1
			}
		} else {
			dir = -1
			if closes[i] > rawUpper {
				dir = 1
			}
		}
		direction[i] = dir
		prevUpper, prevLower, prevDir = rawUpper, rawLower, dir
	}
	return direction
}

// bbiValues BBI(3,6,12,24)
func bbiValues(closes []float64) []float64 {
	m1 := smaValues(closes, 3)
	m2 := smaValues(closes, 6)
	m3 := smaValues(closes, 12)
	m4 := smaValues(closes, 24)
	out := nanSlice(len(closes))
	for i := range closes {
		if !isNaN(m1[i]) && !isNaN(m2[i]) && !isNaN(m3[i]) && !isNaN(m4[i]) {
			out[i] = (m1[i] + m2[i] + m3[i] + m4[i]) / 4
		}
	}
	return out
}

// trixSlopeValues TRIX(15) 一阶差分
func trixSlopeValues(closes []float64, period int) []float64 {
	ema1 := emaFinite(closes, period)
	ema2 := emaFinite(ema1, period)
	ema3 := emaFinite(ema2, period)
	trix := nanSlice(len(closes))
	for i := 1; i < len(closes); i++ {
		if !isNaN(ema3[i]) && !isNaN(ema3[i-1]) && ema3[i-1] != 0 {
			trix[i] = (ema3[i] - ema3[i-1]) / ema3[i-1] * 10000
		}
	}
	slope := nanSlice(len(closes))
	for i := 1; i < len(closes); i++ {
		if !isNaN(trix[i]) && !isNaN(trix[i-1]) {
			slope[i] = trix[i] - trix[i-1]
		}
	}
	return slope
}

// chopValues CHOP(14) 状态指标
func chopValues(highs, lows, closes []float64, period int) []float64 {
	length := len(closes)
	chop := nanSlice(length)
	for i := period - 1; i < length; i++ {
		atrSum := 0.0
		ok := true
		for j := 0; j < period; j++ {
			idx := i - j
			var tr float64
			if idx-1 >= 0 {
				tr = math.Max(highs[idx]-lows[idx], math.Max(math.Abs(highs[idx]-closes[idx-1]), math.Abs(lows[idx]-closes[idx-1])))
			} else {
				tr = highs[idx] - lows[idx]
			}
			if math.IsInf(tr, 0) || isNaN(tr) {
				ok = false
				break
			}
			atrSum += tr
		}
		if !ok {
			continue
		}
		loIdx := i - period + 1
		hi, lo := math.Inf(-1), math.Inf(1)
		for j := loIdx; j <= i; j++ {
			hi = math.Max(hi, highs[j])
			lo = math.Min(lo, lows[j])
		}
		trueRange := hi - lo
		if trueRange <= 0 {
			continue
		}
		chop[i] = 100 * math.Log(atrSum/trueRange) / math.Log(float64(period))
	}
	return chop
}

// temaValues TEMA(21)
func temaValues(closes []float64, period int) []float64 {
	ema1 := emaFinite(closes, period)
	ema2 := emaFinite(ema1, period)
	ema3 := emaFinite(ema2, period)
	out := nanSlice(len(closes))
	for i := range closes {
		if !isNaN(ema1[i]) && !isNaN(ema2[i]) && !isNaN(ema3[i]) {
			out[i] = ema1[i] + (ema1[i] - ema2[i]) + ((ema1[i] - ema2[i]) - (ema2[i] - ema3[i]))
		}
	}
	return out
}

// linregSlope 最小二乘回归斜率（窗口 period 根）
func linregSlope(values []float64, period int) []float64 {
	n := period
	if n < 2 {
		n = 2
	}
	out := nanSlice(len(values))
	if len(values) < n {
		return out
	}
	sumX := float64(n*(n-1)) / 2
	sumXX := float64(n*(n-1)*(2*n-1)) / 6
	denom := float64(n)*sumXX - sumX*sumX
	for i := n - 1; i < len(values); i++ {
		sy, sxy := 0.0, 0.0
		ok := true
		for j := 0; j < n; j++ {
			v := values[i-n+1+j]
			if isNaN(v) {
				ok = false
				break
			}
			sy += v
			sxy += v * float64(j)
		}
		if !ok {
			continue
		}
		out[i] = (float64(n)*sxy - sumX*sy) / denom
	}
	return out
}

// temaVelocity TEMA 速度（回归斜率 / ATR 标准化）
func temaVelocity(highs, lows, closes []float64, period, slopePeriod int) []float64 {
	tema := temaValues(closes, period)
	atr := atrValues(highs, lows, closes, 14)
	rawVel := linregSlope(tema, slopePeriod)
	vel := nanSlice(len(closes))
	for i := range closes {
		if !isNaN(rawVel[i]) && !isNaN(atr[i]) && atr[i] > 0 {
			vel[i] = rawVel[i] / atr[i]
		}
	}
	return vel
}

// ---- 扩展评分维度（与前端 biasValues / candlePatternsValues / profitRatioSeries / mtfScoreBundle 同口径） ----

// biasValues BIAS 乖离率：(收盘 - N日SMA) / N日SMA × 100
func biasValues(closes []float64, period int) []float64 {
	ma := smaValues(closes, period)
	out := nanSlice(len(closes))
	for i, c := range closes {
		if !isNaN(ma[i]) && ma[i] != 0 {
			out[i] = (c - ma[i]) / ma[i] * 100
		}
	}
	return out
}

// rocValues ROC(period) 变动率
func rocValues(closes []float64, period int) []float64 {
	out := nanSlice(len(closes))
	for i := period; i < len(closes); i++ {
		if closes[i-period] != 0 {
			out[i] = (closes[i] - closes[i-period]) / closes[i-period] * 100
		}
	}
	return out
}

// candlePattern 单个K线形态命中
type candlePattern struct {
	idx  int
	name string
}

// candlePatternsValues K线形态识别（全部只用截至当根的数据，无未来函数）。
// 看涨：启明星/红三兵/看涨吞没/锤头/倒锤头；看跌：黄昏星/三只乌鸦/看跌吞没/上吊线/流星。
// 优先级：星形态 > 吞没 > 红三兵/三只乌鸦 > 锤头/流星，每根K线每侧最多一个；
// 锤头/上吊线/流星/倒锤头的趋势上下文：close[i] < close[i-5] 视为跌势，反之为涨势。
func candlePatternsValues(opens, highs, lows, closes []float64) (bullish, bearish []candlePattern) {
	bullish, bearish = []candlePattern{}, []candlePattern{}
	length := len(closes)
	for i := 2; i < length; i++ {
		o, h, l, c := opens[i], highs[i], lows[i], closes[i]
		if isNaN(o) || isNaN(h) || isNaN(l) || isNaN(c) {
			continue
		}
		rng := h - l
		if rng <= 0 {
			continue
		}
		body := math.Abs(c - o)
		upShadow := h - math.Max(o, c)
		loShadow := math.Min(o, c) - l

		// 三根组合形态（优先级最高，基于 i-2..i）
		o1, c1, o2, c2 := opens[i-2], closes[i-2], opens[i-1], closes[i-1]
		h1, l1, h2, l2 := highs[i-2], lows[i-2], highs[i-1], lows[i-1]
		if !isNaN(o1) && !isNaN(c1) && !isNaN(h1) && !isNaN(l1) && !isNaN(o2) && !isNaN(c2) && !isNaN(h2) && !isNaN(l2) {
			body1 := math.Abs(c1 - o1)
			body2 := math.Abs(c2 - o2)
			// 启明星：长阴 + 小实体 + 阳线收复阴线实体一半
			if c1 < o1 && c > o && body2 <= body1*0.6 && o2 <= c1 && c > (o1+c1)/2 && body1 > 0 {
				bullish = append(bullish, candlePattern{i, "启明星"})
				continue
			}
			// 黄昏星：长阳 + 小实体 + 阴线跌破阳线实体一半
			if c1 > o1 && c < o && body2 <= body1*0.6 && o2 >= c1 && c < (o1+c1)/2 && body1 > 0 {
				bearish = append(bearish, candlePattern{i, "黄昏星"})
				continue
			}
			// 红三兵：三连阳逐级抬升，开盘在前一根实体内，收盘贴近最高价
			if i >= 3 && !isNaN(closes[i-3]) && !isNaN(opens[i-3]) {
				if c1 > o1 && c2 > o2 && c > o && c > c2 && c2 > c1 && o2 > o1 && o2 < c1 && o > o2 && o < c2 &&
					(h-c) <= math.Max(body, rng*0.15)*1.5 {
					bullish = append(bullish, candlePattern{i, "红三兵"})
					continue
				}
				// 三只乌鸦：三连阴逐级下探
				if c1 < o1 && c2 < o2 && c < o && c < c2 && c2 < c1 && o2 < o1 && o2 > c1 && o < o2 && o > c2 {
					bearish = append(bearish, candlePattern{i, "三只乌鸦"})
					continue
				}
			}
		}

		// 吞没形态
		prevC, prevO := closes[i-1], opens[i-1]
		if !isNaN(prevC) && !isNaN(prevO) {
			prevBody := math.Abs(prevC - prevO)
			if prevC < prevO && c > o && c >= prevO && o <= prevC && body > prevBody && prevBody > 0 {
				bullish = append(bullish, candlePattern{i, "看涨吞没"})
				continue
			}
			if prevC > prevO && c < o && c <= prevO && o >= prevC && body > prevBody && prevBody > 0 {
				bearish = append(bearish, candlePattern{i, "看跌吞没"})
				continue
			}
		}

		// 趋势上下文（近 5 根收盘方向）
		ctx := ""
		if i >= 5 && !isNaN(closes[i-5]) {
			if c < closes[i-5] {
				ctx = "down"
			} else {
				ctx = "up"
			}
		}
		if body > 0 {
			// 长下影：跌势中=锤头（看涨），涨势中=上吊线（看跌）
			if loShadow >= body*2 && upShadow <= body {
				if ctx == "down" {
					bullish = append(bullish, candlePattern{i, "锤头"})
				} else if ctx == "up" {
					bearish = append(bearish, candlePattern{i, "上吊线"})
				}
				continue
			}
			// 长上影：涨势中=流星（看跌），跌势中=倒锤头（看涨）
			if upShadow >= body*2 && loShadow <= body {
				if ctx == "up" {
					bearish = append(bearish, candlePattern{i, "流星"})
				} else if ctx == "down" {
					bullish = append(bullish, candlePattern{i, "倒锤头"})
				}
				continue
			}
		}
	}
	return
}

// chipBarCostCenter / addChipVolumeKernel 复用 chip_distribution.go 中的同口径实现。

// profitRatioSeries 获利盘比例历史序列：沿K线逐根推进筹码分布（换手衰减 + 当根成交量按高低价
// 区间核分布），逐根记录「当前价以下的筹码占比」0~1。与前端 chip.ts profitRatioSeries 同口径。
// turnovers 为小数换手率（0~1，缺数据传 0=不衰减），amounts 为成交额（元）。
func profitRatioSeries(opens, highs, lows, closes, vols, amounts, turnovers []float64, bins int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	if n == 0 || bins <= 0 {
		return out
	}
	minP, maxP := math.Inf(1), 0.0
	for i := 0; i < n; i++ {
		if lows[i] > 0 && lows[i] < minP {
			minP = lows[i]
		}
		if highs[i] > 0 && highs[i] > maxP {
			maxP = highs[i]
		}
	}
	if minP <= 0 || maxP <= 0 || maxP < minP {
		return out
	}
	if maxP == minP {
		maxP = minP * 1.001
	}
	width := (maxP - minP) / float64(bins)
	if width <= 0 {
		return out
	}
	dist := make([]float64, bins)
	for k := 0; k < n; k++ {
		turn := turnovers[k]
		if turn < 0 {
			turn = 0
		}
		if turn > 0.98 {
			turn = 0.98
		}
		remain := 1.0 - turn
		for i := 0; i < bins; i++ {
			dist[i] *= remain
		}
		if vols[k] > 0 && lows[k] > 0 && highs[k] > 0 {
			addChipVolumeKernel(dist, bins, minP, width, lows[k], highs[k], vols[k],
				chipBarCostCenter(lows[k], highs[k], opens[k], closes[k], vols[k], amounts[k]))
		}
		sum := 0.0
		for i := 0; i < bins; i++ {
			sum += dist[i]
		}
		if sum <= 0 {
			continue
		}
		cur := closes[k]
		if isNaN(cur) || cur <= 0 {
			cur = highs[k]
		}
		profitVol, costAcc := 0.0, 0.0
		for i := 0; i < bins; i++ {
			center := minP + (float64(i)+0.5)*width
			costAcc += dist[i] * center
			if center <= cur {
				profitVol += dist[i]
			}
		}
		out[k] = math.Round(profitVol/sum*1e6) / 1e6
	}
	return out
}

// ymdOfDayKey 提取 'YYYY-MM-DD'（兼容 '2026-11-14' 与 '2026/11/14 15:00'）
func ymdOfDayKey(day string) string {
	if len(day) < 10 {
		return ""
	}
	s := day[:10]
	if s[4] != '-' && s[4] != '/' {
		return ""
	}
	if s[7] != '-' && s[7] != '/' {
		return ""
	}
	for _, pos := range [8]int{0, 1, 2, 3, 5, 6, 8, 9} {
		if s[pos] < '0' || s[pos] > '9' {
			return ""
		}
	}
	return s[0:4] + "-" + s[5:7] + "-" + s[8:10]
}

// weekKeyOfYmd 周键：所在周的周一日期（UTC 计算，与前端 weekKeyOf 一致）
func weekKeyOfYmd(ymd string) string {
	t, err := time.ParseInLocation("2006-01-02", ymd, time.UTC)
	if err != nil {
		return ""
	}
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1)).Format("2006-01-02")
}

// mtfHit 多周期因子命中：hits=命中数，available=可用因子数
type mtfHit struct {
	hits      int
	available int
}

// mtfFactorHits 在一组 OHLC 上评估 6 因子（收盘>MA20、MA5>MA20、DIF>DEA、RSI14>50、K>D、ROC10>0），
// 因子窗口不足（NaN）时该因子从分母剔除。
func mtfFactorHits(opens, highs, lows, closes []float64) []mtfHit {
	n := len(closes)
	ma5 := smaValues(closes, 5)
	ma20 := smaValues(closes, 20)
	dif, dea := macdBundle(closes)
	rsi14 := rsiValues(closes, 14)
	kk, dd := kdjBundle(highs, lows, closes, 9)
	roc10 := rocValues(closes, 10)
	out := make([]mtfHit, n)
	for i := 0; i < n; i++ {
		hits, avail := 0, 0
		add := func(ok bool) {
			avail++
			if ok {
				hits++
			}
		}
		if !isNaN(ma20[i]) {
			add(closes[i] > ma20[i])
		}
		if !isNaN(ma5[i]) && !isNaN(ma20[i]) {
			add(ma5[i] > ma20[i])
		}
		if !isNaN(dif[i]) && !isNaN(dea[i]) {
			add(dif[i] > dea[i])
		}
		if !isNaN(rsi14[i]) {
			add(rsi14[i] > 50)
		}
		if !isNaN(kk[i]) && !isNaN(dd[i]) {
			add(kk[i] > dd[i])
		}
		if !isNaN(roc10[i]) {
			add(roc10[i] > 0)
		}
		out[i] = mtfHit{hits: hits, available: avail}
	}
	return out
}

// mtfScoreSeries 多周期共振评分主入口（日K→周K聚合），返回与输入按下标对齐的评分序列（0~100，NaN=不可用）。
// 当前周期 50% + 上一级周期 50%（任一侧不可用时用另一侧）；防未来：第 N 周只使用键值严格小于
// 当前周的已收盘周线因子。与前端 multiTimeframe.ts mtfScoreBundle 同口径。
func mtfScoreSeries(opens, highs, lows, closes []float64, dayKeys []string) []float64 {
	n := len(closes)
	out := nanSlice(n)
	if n < 2 {
		return out
	}
	type mbar struct {
		idx int
		ymd string
	}
	bars := make([]mbar, 0, n)
	for i := 0; i < n && i < len(dayKeys); i++ {
		y := ymdOfDayKey(dayKeys[i])
		if y == "" {
			continue
		}
		if isNaN(opens[i]) || isNaN(highs[i]) || isNaN(lows[i]) || isNaN(closes[i]) {
			continue
		}
		bars = append(bars, mbar{idx: i, ymd: y})
	}
	if len(bars) < 2 {
		return out
	}
	sort.Slice(bars, func(a, b int) bool { return bars[a].ymd < bars[b].ymd })
	m := len(bars)
	bo := make([]float64, m)
	bh := make([]float64, m)
	bl := make([]float64, m)
	bc := make([]float64, m)
	for k, b := range bars {
		bo[k], bh[k], bl[k], bc[k] = opens[b.idx], highs[b.idx], lows[b.idx], closes[b.idx]
	}
	baseHits := mtfFactorHits(bo, bh, bl, bc)

	// 周线聚合：组内取首开盘/最高/最低/末收盘
	type wbar struct {
		key        string
		o, h, l, c float64
	}
	var wbs []wbar
	for _, b := range bars {
		k := weekKeyOfYmd(b.ymd)
		if k == "" {
			continue
		}
		if len(wbs) == 0 || wbs[len(wbs)-1].key != k {
			wbs = append(wbs, wbar{key: k, o: opens[b.idx], h: highs[b.idx], l: lows[b.idx], c: closes[b.idx]})
		} else {
			w := &wbs[len(wbs)-1]
			w.h = math.Max(w.h, highs[b.idx])
			w.l = math.Min(w.l, lows[b.idx])
			w.c = closes[b.idx]
		}
	}
	var higherHits []mtfHit
	if len(wbs) >= 2 {
		wo := make([]float64, len(wbs))
		wh := make([]float64, len(wbs))
		wl := make([]float64, len(wbs))
		wc := make([]float64, len(wbs))
		for i, w := range wbs {
			wo[i], wh[i], wl[i], wc[i] = w.o, w.h, w.l, w.c
		}
		higherHits = mtfFactorHits(wo, wh, wl, wc)
	}

	j := -1
	for k := 0; k < m; k++ {
		var base float64 = math.NaN()
		if bhit := baseHits[k]; bhit.available > 0 {
			base = float64(bhit.hits) / float64(bhit.available) * 100
		}
		hs := math.NaN()
		if wk := weekKeyOfYmd(bars[k].ymd); wk != "" && higherHits != nil {
			for j+1 < len(wbs) && wbs[j+1].key < wk {
				j++
			}
			ju := j
			if ju >= 0 && wbs[ju].key >= wk {
				ju-- // 防御性再校验：保证严格小于当前键
			}
			for ju >= 0 && higherHits[ju].available <= 0 {
				ju--
			}
			if ju >= 0 {
				hh := higherHits[ju]
				hs = float64(hh.hits) / float64(hh.available) * 100
			}
		}
		switch {
		case !isNaN(base) && !isNaN(hs):
			out[bars[k].idx] = base*0.5 + hs*0.5
		case !isNaN(base):
			out[bars[k].idx] = base
		case !isNaN(hs):
			out[bars[k].idx] = hs
		}
	}
	return out
}

// ---- 主算法：14+4 路共振打分（基础路与 buySellPointsValues 同口径，扩展路见文件头注释） ----

// BuySellPointsOptions 买卖点算法参数（默认值对应 TS 版）
type BuySellPointsOptions struct {
	MinScore   float64  // 命中最少信号路数（满分 14；启用扩展维度时按 18/14 等比放大）
	MinGroups  int      // 需覆盖的信号组数
	MinGap     int      // 同类聚类间距
	Window     int      // 买点共振累计窗口
	SellWindow int      // 卖点共振累计窗口
	DayKeys    []string // 交易日键（分钟周期传入以启用当日累计均价线；日K传入以启用多周期共振）
	Intraday   bool     // 是否分钟周期（分时不启用 MA20/CHOP/ATR 等门控，也不启用扩展维度）
	// ---- 扩展维度数据（仅日K类周期使用；缺失/等长不满足时对应维度静默跳过）----
	Opens         []float64 // 开盘价序列（K线形态/获利盘/多周期需要）
	Amounts       []float64 // 成交额序列（元，获利盘锚点需要）
	TurnoverRates []float64 // 换手率序列（小数 0~1，获利盘衰减需要；全 0 时该维度跳过）
}

// BuySellPoint 单个买卖点
type BuySellPoint struct {
	I       int      `json:"i"`       // 信号所在 K 线下标
	Price   float64  `json:"price"`   // 信号价（买=low / 卖=high）
	Score   float64  `json:"score"`   // 命中路数（0-14；启用扩展维度时 0-18）
	Reasons []string `json:"reasons"` // 命中信号名
}

// 信号命中：组别 + 名称 + 权重
type signalHit struct {
	group  int
	label  string
	weight float64
}

type windowHits struct {
	score   float64
	groups  int
	reasons []string
}

// collectWindowHits 累计窗口内的命中：按信号名去重，返回得分/组数/理由
func collectWindowHits(hitsByBar [][]signalHit, i, w int) windowHits {
	labels := map[string]signalHit{}
	order := []string{}
	for k := i - w + 1; k <= i; k++ {
		if k < 0 {
			continue
		}
		for _, h := range hitsByBar[k] {
			if _, ok := labels[h.label]; !ok {
				labels[h.label] = h
				order = append(order, h.label)
			}
		}
	}
	groupSet := map[int]bool{}
	score := 0.0
	for _, label := range order {
		h := labels[label]
		groupSet[h.group] = true
		score += h.weight
	}
	return windowHits{score: math.Round(score*100) / 100, groups: len(groupSet), reasons: order}
}

type signalCluster struct {
	buy     bool
	start   int
	end     int
	score   float64
	reasons []string
}

type candSignal struct {
	i      int
	score  float64
	groups int
	// windowHits 内嵌
	reasons []string
}

// buildClusters 时序聚类：minGap 根以内的同类候选归为一簇
func buildClusters(cands []candSignal, buy bool, minGap int, out *[]signalCluster) {
	var cur *signalCluster
	for _, c := range cands {
		if cur == nil || c.i-cur.end > minGap {
			if cur != nil {
				*out = append(*out, *cur)
			}
			cur = &signalCluster{buy: buy, start: c.i, end: c.i, score: c.score, reasons: c.reasons}
		} else {
			cur.end = c.i
		}
	}
	if cur != nil {
		*out = append(*out, *cur)
	}
}

// BuySellPointsValues 买卖点共振检测（纯因果，无未来函数）。
// 返回簇起点下标的买/卖信号列表，口径与前端 calc.ts buySellPointsValues 完全一致。
func BuySellPointsValues(highs, lows, closes, vols []float64, opts *BuySellPointsOptions) (buys, sells []BuySellPoint) {
	length := len(closes)
	buys, sells = []BuySellPoint{}, []BuySellPoint{}
	if length < 30 {
		return
	}
	o := BuySellPointsOptions{MinScore: 6, MinGroups: 2, MinGap: 5, Window: 4, SellWindow: 2}
	if opts != nil {
		if opts.MinScore > 0 {
			o.MinScore = opts.MinScore
		}
		if opts.MinGroups > 0 {
			o.MinGroups = opts.MinGroups
		}
		if opts.MinGap > 0 {
			o.MinGap = opts.MinGap
		}
		if opts.Window > 0 {
			o.Window = opts.Window
		}
		if opts.SellWindow > 0 {
			o.SellWindow = opts.SellWindow
		}
		o.DayKeys = opts.DayKeys
		o.Intraday = opts.Intraday
		o.Opens = opts.Opens
		o.Amounts = opts.Amounts
		o.TurnoverRates = opts.TurnoverRates
	}

	rsi := rsiValues(closes, 14)
	kk, dd := kdjBundle(highs, lows, closes, 9)
	dif, dea := macdBundle(closes)
	temaVel := temaVelocity(highs, lows, closes, 21, 3)
	trixSlope := trixSlopeValues(closes, 15)
	cci := cciValues(highs, lows, closes, 20)
	adx, diP, diM := adxBundle(highs, lows, closes, 14)
	var vwap []float64
	if o.Intraday && len(o.DayKeys) == length {
		vwap = cumulativeVwapByDay(highs, lows, closes, vols, o.DayKeys, 5)
	} else {
		vwap = vwapValues(highs, lows, closes, vols, 20)
	}
	volMa := smaValues(vols, 5)
	donUpper, donLower := donchianChannel(highs, lows, 20)
	mfi := mfiValues(highs, lows, closes, vols, 14)
	cmf := cmfValues(highs, lows, closes, vols, 20)
	stDir := supertrendDirection(highs, lows, closes, 10, 3)
	bbi := bbiValues(closes)

	// ---- 扩展评分维度（仅日K类周期；数据缺失的维度静默跳过，不阻塞其他维度） ----
	extOn := !o.Intraday && len(o.Opens) == length
	var bias6 []float64
	var bullPat, bearPat map[int]string
	var profitRatio []float64
	var mtfScores []float64
	if extOn {
		bias6 = biasValues(closes, 6)
		bullSlice, bearSlice := candlePatternsValues(o.Opens, highs, lows, closes)
		bullPat, bearPat = map[int]string{}, map[int]string{}
		for _, p := range bullSlice {
			bullPat[p.idx] = p.name
		}
		for _, p := range bearSlice {
			bearPat[p.idx] = p.name
		}
		if len(o.Amounts) == length && len(o.TurnoverRates) == length {
			hasTurn := false
			for _, t := range o.TurnoverRates {
				if t > 0 {
					hasTurn = true
					break
				}
			}
			if hasTurn {
				profitRatio = profitRatioSeries(o.Opens, highs, lows, closes, vols, o.Amounts, o.TurnoverRates, 50)
			}
		}
		if len(o.DayKeys) == length {
			mtfScores = mtfScoreSeries(o.Opens, highs, lows, closes, o.DayKeys)
		}
		// 18 路满分：命中门限按 18/14 等比放大，保持灵敏/标准/严格档位的原有筛选强度（4/6/7 → 5/8/9）
		if o.MinScore > 0 {
			o.MinScore = math.Round(o.MinScore * 18.0 / 14.0)
		}
	}

	crossUp := func(arr []float64, i int, lv float64) bool {
		return i > 0 && !isNaN(arr[i]) && !isNaN(arr[i-1]) && arr[i] > lv && arr[i-1] <= lv
	}
	crossDown := func(arr []float64, i int, lv float64) bool {
		return i > 0 && !isNaN(arr[i]) && !isNaN(arr[i-1]) && arr[i] < lv && arr[i-1] >= lv
	}
	slopeUp := func(i int) bool {
		return i > 0 && !isNaN(temaVel[i]) && !isNaN(temaVel[i-1]) && temaVel[i] > 0 && temaVel[i-1] <= 0
	}
	slopeDown := func(i int) bool {
		return i > 0 && !isNaN(temaVel[i]) && !isNaN(temaVel[i-1]) && temaVel[i] < 0 && temaVel[i-1] >= 0
	}
	trixUp := func(i int) bool {
		return i > 0 && !isNaN(trixSlope[i]) && !isNaN(trixSlope[i-1]) && trixSlope[i] > 0 && trixSlope[i-1] <= 0
	}
	trixDown := func(i int) bool {
		return i > 0 && !isNaN(trixSlope[i]) && !isNaN(trixSlope[i-1]) && trixSlope[i] < 0 && trixSlope[i-1] >= 0
	}
	adxUpBar := func(i int) bool {
		return i > 0 && !isNaN(adx[i]) && !isNaN(adx[i-1]) && adx[i] > adx[i-1]
	}
	adxDownBar := func(i int) bool {
		return i > 0 && !isNaN(adx[i]) && !isNaN(adx[i-1]) && adx[i] < adx[i-1]
	}
	// 通道含当根，与「上一根的通道值」比较（一次性事件后重新武装）
	donBreakUp := func(i int) bool {
		return i >= 2 && !isNaN(donUpper[i-1]) && !isNaN(donUpper[i-2]) && closes[i] > donUpper[i-1] && closes[i-1] <= donUpper[i-2]
	}
	donBreakDown := func(i int) bool {
		return i >= 2 && !isNaN(donLower[i-1]) && !isNaN(donLower[i-2]) && closes[i] < donLower[i-1] && closes[i-1] >= donLower[i-2]
	}
	stFlipUp := func(i int) bool {
		return i > 0 && stDir[i] == 1 && stDir[i-1] == -1
	}
	stFlipDown := func(i int) bool {
		return i > 0 && stDir[i] == -1 && stDir[i-1] == 1
	}
	bbiUp := func(i int) bool {
		return i > 0 && !isNaN(bbi[i]) && !isNaN(bbi[i-1]) && closes[i] > bbi[i] && closes[i-1] <= bbi[i-1]
	}
	bbiDown := func(i int) bool {
		return i > 0 && !isNaN(bbi[i]) && !isNaN(bbi[i-1]) && closes[i] < bbi[i] && closes[i-1] >= bbi[i-1]
	}

	// 逐根采集命中
	buyHits := make([][]signalHit, length)
	sellHits := make([][]signalHit, length)
	for i := 1; i < length; i++ {
		volOk := !isNaN(volMa[i]) && volMa[i] > 0 && vols[i] > volMa[i]*1.5

		b := []signalHit{}
		if crossUp(rsi, i, 30) {
			b = append(b, signalHit{0, "RSI上穿30", 1})
		}
		if !isNaN(kk[i]) && !isNaN(dd[i]) && !isNaN(kk[i-1]) && !isNaN(dd[i-1]) && kk[i] > dd[i] && kk[i-1] <= dd[i-1] && kk[i] < 35 {
			b = append(b, signalHit{0, "KDJ低位金叉", 1})
		}
		if crossUp(cci, i, -100) {
			b = append(b, signalHit{0, "CCI回穿-100", 1})
		}
		if !isNaN(dif[i]) && !isNaN(dea[i]) && !isNaN(dif[i-1]) && !isNaN(dea[i-1]) && dif[i] > dea[i] && dif[i-1] <= dea[i-1] {
			label := "MACD金叉"
			if dif[i] < 0 {
				label = "MACD零轴下金叉"
			}
			b = append(b, signalHit{1, label, 1})
		}
		if slopeUp(i) {
			b = append(b, signalHit{1, "TEMA斜率转正", 1})
		}
		if trixUp(i) {
			b = append(b, signalHit{1, "TRIX斜率转正", 1})
		}
		if adxUpBar(i) && diP[i] > diM[i] {
			b = append(b, signalHit{1, "ADX转强(DI+>DI-)", 1})
		}
		if i > 0 && !isNaN(vwap[i]) && !isNaN(vwap[i-1]) && closes[i] > vwap[i] && closes[i-1] <= vwap[i-1] {
			if o.Intraday {
				b = append(b, signalHit{2, "收复均价线", 1})
			} else {
				b = append(b, signalHit{2, "收复VWAP", 1})
			}
		}
		if volOk {
			b = append(b, signalHit{2, "放量", 1})
		}
		if crossUp(mfi, i, 20) {
			b = append(b, signalHit{2, "MFI上穿20", 1})
		}
		if crossUp(cmf, i, 0) {
			b = append(b, signalHit{2, "CMF转正", 1})
		}
		if donBreakUp(i) {
			b = append(b, signalHit{3, "突破20期高", 1})
		}
		if stFlipUp(i) {
			b = append(b, signalHit{3, "趋势翻多", 1})
		}
		if bbiUp(i) {
			b = append(b, signalHit{3, "上穿BBI", 1})
		}
		if extOn {
			if crossUp(bias6, i, -8) {
				b = append(b, signalHit{0, "BIAS超卖回升", 1})
			}
			if name, ok := bullPat[i]; ok {
				b = append(b, signalHit{0, name, 1})
			}
			if profitRatio != nil && crossUp(profitRatio, i, 0.10) {
				b = append(b, signalHit{2, "获利盘回升(超跌)", 1})
			}
			if mtfScores != nil && crossUp(mtfScores, i, 60) {
				b = append(b, signalHit{3, "多周期共振转多", 1})
			}
		}
		buyHits[i] = b

		s := []signalHit{}
		if crossDown(rsi, i, 70) {
			s = append(s, signalHit{0, "RSI下穿70", 1})
		}
		if !isNaN(kk[i]) && !isNaN(dd[i]) && !isNaN(kk[i-1]) && !isNaN(dd[i-1]) && kk[i] < dd[i] && kk[i-1] >= dd[i-1] && kk[i] > 65 {
			s = append(s, signalHit{0, "KDJ高位死叉", 1})
		}
		if crossDown(cci, i, 100) {
			s = append(s, signalHit{0, "CCI回穿+100", 1})
		}
		if !isNaN(dif[i]) && !isNaN(dea[i]) && !isNaN(dif[i-1]) && !isNaN(dea[i-1]) && dif[i] < dea[i] && dif[i-1] >= dea[i-1] {
			label := "MACD死叉"
			if dif[i] > 0 {
				label = "MACD零轴上死叉"
			}
			s = append(s, signalHit{1, label, 1})
		}
		if slopeDown(i) {
			s = append(s, signalHit{1, "TEMA斜率转负", 1})
		}
		if trixDown(i) {
			s = append(s, signalHit{1, "TRIX斜率转负", 1})
		}
		if adxDownBar(i) && diP[i] < diM[i] {
			s = append(s, signalHit{1, "ADX转弱(DI+<DI-)", 1})
		}
		if !isNaN(vwap[i]) && !isNaN(vwap[i-1]) && closes[i] < vwap[i] && closes[i-1] >= vwap[i-1] {
			if o.Intraday {
				s = append(s, signalHit{2, "跌破均价线", 1})
			} else {
				s = append(s, signalHit{2, "跌破VWAP", 1})
			}
		}
		if volOk {
			s = append(s, signalHit{2, "放量", 1})
		}
		if crossDown(mfi, i, 80) {
			s = append(s, signalHit{2, "MFI下穿80", 1})
		}
		if crossDown(cmf, i, 0) {
			s = append(s, signalHit{2, "CMF转负", 1})
		}
		if donBreakDown(i) {
			s = append(s, signalHit{3, "跌破20期低", 1})
		}
		if stFlipDown(i) {
			s = append(s, signalHit{3, "趋势翻空", 1})
		}
		if bbiDown(i) {
			s = append(s, signalHit{3, "下穿BBI", 1})
		}
		if extOn {
			if crossDown(bias6, i, 8) {
				s = append(s, signalHit{0, "BIAS超买回落", 1})
			}
			if name, ok := bearPat[i]; ok {
				s = append(s, signalHit{0, name, 1})
			}
			if profitRatio != nil && crossDown(profitRatio, i, 0.90) {
				s = append(s, signalHit{2, "获利盘回落(抛压)", 1})
			}
			if mtfScores != nil && crossDown(mtfScores, i, 40) {
				s = append(s, signalHit{3, "多周期共振转空", 1})
			}
		}
		sellHits[i] = s
	}

	// 门控（仅日K类周期启用；分时全部放行）
	var ma20, atr14, chop14, ma250, atrRatio []float64
	if !o.Intraday {
		ma20 = smaValues(closes, 20)
		ma250 = smaValues(closes, 250)
		atr14 = atrValues(highs, lows, closes, 14)
		chop14 = chopValues(highs, lows, closes, 14)
		// 滚动 250 根 ATR 均值（不含当根）
		atrRatio = nanSlice(length)
		sum, cnt := 0.0, 0
		for i := 0; i < length; i++ {
			if i > 0 && !isNaN(atr14[i-1]) {
				sum += atr14[i-1]
				cnt++
			}
			drop := i - 1 - 250
			if drop >= 0 && !isNaN(atr14[drop]) {
				sum -= atr14[drop]
				cnt--
			}
			if cnt >= 250 && sum > 0 && !isNaN(atr14[i]) {
				atrRatio[i] = atr14[i] / (sum / float64(cnt))
			}
		}
	}
	trendOk := func(i int, buy bool) bool {
		if ma20 == nil || isNaN(ma20[i]) {
			return true
		}
		if buy {
			return closes[i] > ma20[i]
		}
		return closes[i] < ma20[i]
	}
	const atrVolRatio = 1.3
	calmOk := func(i int) bool {
		if atrRatio == nil || isNaN(atrRatio[i]) {
			return true
		}
		return atrRatio[i] < atrVolRatio
	}
	regimeBuyOk := func(i int) bool {
		if chop14 == nil || isNaN(chop14[i]) {
			return true
		}
		return chop14[i] < 45
	}
	regimeSellOk := func(i int) bool {
		if chop14 == nil || isNaN(chop14[i]) {
			return true
		}
		return chop14[i] < 38.2 || chop14[i] > 61.8
	}
	const (
		buyMomo60Max = 0.15
		buyRsiMin    = 50.0
		buyDma250Min = 0.0
		buyDma250Max = -0.05
		sellRsiMax   = 50.0
	)
	buyQualityOk := func(i int) bool {
		if o.Intraday {
			return true
		}
		var m60 float64 = math.NaN()
		if i >= 60 && closes[i-60] > 0 {
			m60 = closes[i]/closes[i-60] - 1
		}
		var d250 float64 = math.NaN()
		if ma250 != nil && !isNaN(ma250[i]) && ma250[i] > 0 {
			d250 = closes[i]/ma250[i] - 1
		}
		momentumOk := calmOk(i) &&
			(isNaN(m60) || m60 <= buyMomo60Max) &&
			(isNaN(rsi[i]) || rsi[i] >= buyRsiMin) &&
			(isNaN(d250) || d250 >= buyDma250Min)
		contrarianOk := !isNaN(d250) && d250 <= buyDma250Max
		return momentumOk || contrarianOk
	}
	sellConfirmOk := func(i int) bool {
		if o.Intraday {
			return true
		}
		return i > 0 && closes[i] < closes[i-1] && (isNaN(rsi[i]) || rsi[i] <= sellRsiMax)
	}

	// 滚动窗口累计命中 → 共振候选
	w := o.Window
	if w < 1 {
		w = 1
	}
	wSell := o.SellWindow
	if wSell < 1 {
		wSell = 1
	}
	var candBuys, candSells []candSignal
	startIdx := w
	if wSell > startIdx {
		startIdx = wSell
	}
	for i := startIdx; i < length; i++ {
		bh := collectWindowHits(buyHits, i, w)
		if bh.score >= o.MinScore && bh.groups >= o.MinGroups && trendOk(i, true) && regimeBuyOk(i) && buyQualityOk(i) {
			candBuys = append(candBuys, candSignal{i: i, score: bh.score, groups: bh.groups, reasons: bh.reasons})
		}
		sh := collectWindowHits(sellHits, i, wSell)
		if sh.score >= o.MinScore && sh.groups >= o.MinGroups && trendOk(i, false) && calmOk(i) && regimeSellOk(i) && sellConfirmOk(i) {
			candSells = append(candSells, candSignal{i: i, score: sh.score, groups: sh.groups, reasons: sh.reasons})
		}
	}

	// 聚类（同类 minGap 内合并；买卖各自独立，不设跨方向约束）
	clusters := []signalCluster{}
	buildClusters(candBuys, true, o.MinGap, &clusters)
	buildClusters(candSells, false, o.MinGap, &clusters)

	for _, cl := range clusters {
		p := BuySellPoint{I: cl.start, Score: cl.score, Reasons: cl.reasons}
		if cl.buy {
			p.Price = lows[cl.start]
			buys = append(buys, p)
		} else {
			p.Price = highs[cl.start]
			sells = append(sells, p)
		}
	}
	return
}
