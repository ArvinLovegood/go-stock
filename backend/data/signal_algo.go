package data

/**
 * 买卖点共振算法（Go 版）——与前端 kline/calc.ts 的 buySellPointsValues 逐行同口径。
 *
 * 背景：图上箭头与监控流水由前端 TS 算法负责（单一实现原则）；本文件专为「全市场扫描器」
 * （market_signal_scanner.go）服务——几千只票批量打分不可能逐只走前端绑定调用，
 * 故在 Go 侧复刻一份完全相同的算法。口径与 TS 版保持逐行对齐，任何一侧调整必须两侧同步。
 *
 * 约定：无效值用 NaN 表示（对应 TS 版数组里的 null/undefined），判断统一走 isNaN。
 * 14 路信号分四组：0=震荡(osc) 1=动量(mom) 2=量价(vol) 3=趋势(trend)，等权计数、分组门控，
 * 详见 TS 版函数头注释（此处不重复粘贴长注释）。
 */

import "math"

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
				mfv = ((closes[idx]-lows[idx])-(highs[idx]-closes[idx]))/rng*vols[idx]
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
			meanDev += math.Abs(tp[i-j]-mean)
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
			out[i] = ema1[i] + (ema1[i]-ema2[i]) + ((ema1[i]-ema2[i]) - (ema2[i] - ema3[i]))
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

// ---- 主算法：14 路共振打分（与 buySellPointsValues 同口径） ----

// BuySellPointsOptions 买卖点算法参数（默认值对应 TS 版）
type BuySellPointsOptions struct {
	MinScore   float64  // 命中最少信号路数（满分 14）
	MinGroups  int      // 需覆盖的信号组数
	MinGap     int      // 同类聚类间距
	Window     int      // 买点共振累计窗口
	SellWindow int      // 卖点共振累计窗口
	DayKeys    []string // 交易日键（分钟周期传入以启用当日累计均价线）
	Intraday   bool     // 是否分钟周期（分时不启用 MA20/CHOP/ATR 等门控）
}

// BuySellPoint 单个买卖点
type BuySellPoint struct {
	I       int      `json:"i"`       // 信号所在 K 线下标
	Price   float64  `json:"price"`   // 信号价（买=low / 卖=high）
	Score   float64  `json:"score"`   // 命中路数（0-14）
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
