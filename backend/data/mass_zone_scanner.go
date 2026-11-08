package data

/**
 * 全市场 MASS 反转预警扫描器（TDX/gotdx 数据源，支持日K/分钟/周K周期）。
 *
 * 用途：按市场范围（沪/深/京/全部）枚举全市场股票，逐只按请求周期拉通达信 K 线，
 * 筛出「当前正处于 Mass Index 反转预警窗口（凸起已形成、尚未回落确认）」
 * 且「处于多头/上升趋势」的标的，流式推给前端。
 *
 * 口径说明：
 *  - MASS 预警：MI(9,9,25) 越过自适应进入线（自身历史 P90，钳制 24~31）后
 *    尚未跌破离开线（进入线-0.5）——与主图琥珀色「MASS 反转预警」竖带、
 *    后端 okxMassGateState 完全同口径（复用 okxMassIndex/okxMassAdaptiveThresholds）。
 *    注意：MI 只描述「区间扩张衰竭」，本身不指方向，故需趋势过滤。
 *  - 多头/上升趋势：复刻前端 okxUptrendScan 的 5 路趋势证据等权计分：
 *    多头排列（close>MA20>MA60）、MA20 上行、Supertrend(10,3) 翻多、
 *    ADX(14)>=25 且 +DI>-DI、Donchian(20) 通道上半区。
 *    支持自选指标（TrendChecks，空=全部 5 个），在自选范围内按 MinTrendScore
 *    计分过滤（阈值钳制到自选个数）；不选指标+≥3 即默认口径。回测仍固定 5 路。
 *
 * 设计要点与 market_signal_scanner.go 一致：TDX 单连接顺序扫描、进度 300ms 节流、
 * 事件回调由上层注入、单实例运行。
 */

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// massScanKltDefault 默认周期：日K（101）。支持 1/5/15/30/60 分钟与 102=周K。
const massScanKltDefault = "101"

// massKlt 校验请求周期，非法/空值回退日K。
func massKlt(klt string) string {
	switch klt {
	case "1", "5", "15", "30", "60", "101", "102":
		return klt
	}
	return massScanKltDefault
}

// massScanLimit 取数根数（各周期统一）：MI 预热（EMA9 两层+比值EMA9+25期求和约 50 根）
// + 自适应阈值样本（okxMassMinSamples=120 有效 MI）+ MA60/ADX 预热，300 根足够
const massScanLimit = 300

// massScanMinBars 参与计算的最少根数：保证 MI 有效样本与 MA60 就位
const massScanMinBars = 150

// MassZoneScanRequest 全市场 MASS 预警扫描请求
type MassZoneScanRequest struct {
	Markets       []string `json:"markets"`       // 市场范围：SH/SZ/BJ 任意组合；空=全部
	Boards        []string `json:"boards"`        // 板块细分：main/gem/star/bj 任意组合；空=全部
	Klt           string   `json:"klt"`           // K线周期：101=日K 1/5/15/30/60=分钟 102=周K；空=日K
	MinTrendScore int      `json:"minTrendScore"` // 多头/上升趋势最低满足数（1~5，默认 3；大于自选个数时钳制到自选个数）
	TrendChecks   []string `json:"trendChecks"`   // 自选趋势指标：ma_align/ma20_up/supertrend/adx/donchian；空=全部 5 个
	MinZoneBars   int      `json:"minZoneBars"`   // 进入预警根数下限（一根K线=一个周期，0=不限）
	MaxZoneBars   int      `json:"maxZoneBars"`   // 进入预警根数上限（一根K线=一个周期，0=不限）
}

// MassZoneHit 单条扫描命中
type MassZoneHit struct {
	StockCode  string  `json:"stockCode"`  // 带市场前缀（sh600519，TDX 格式）
	Code       string  `json:"code"`       // 纯代码（600519）
	Name       string  `json:"name"`       // 股票名称
	Market     string  `json:"market"`     // SH/SZ/BJ
	Klt        string  `json:"klt"`        // K线周期（101=日K 1/5/15/30/60=分钟 102=周K）
	MI         float64 `json:"mi"`         // 最新 Mass Index 值
	Enter      float64 `json:"enter"`      // 进入线（自适应阈值）
	Exit       float64 `json:"exit"`       // 离开线
	Adaptive   bool    `json:"adaptive"`   // 阈值是否自适应（false=经典 27/26.5 回退）
	ZoneStart  string  `json:"zoneStart"`  // 预警窗口起始日（yyyy-MM-dd）
	ZoneBars   int     `json:"zoneBars"`   // 预警窗口已持续根数
	TrendScore int     `json:"trendScore"` // 满足的自选趋势指标数
	TrendTotal int     `json:"trendTotal"` // 参与计分的自选指标数（5=全部）
	TrendInfo  string  `json:"trendInfo"`  // 满足的指标明细（顿号分隔）
	Close      float64 `json:"close"`      // 最新收盘价
	Pct        float64 `json:"pct"`        // 最新一根涨跌幅 %
	MA20       float64 `json:"ma20"`       // 最新 MA20
	MA60       float64 `json:"ma60"`       // 最新 MA60
	Day        string  `json:"day"`        // 最新 K 线日期
	Bars       int     `json:"bars"`       // 参与计算的 K 线根数
}

// MassZoneScanProgress 扫描进度快照
type MassZoneScanProgress struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Scanned    int    `json:"scanned"`
	Hits       int    `json:"hits"`
	Current    string `json:"current"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Error      string `json:"error"`
}

// MassZoneScanCallbacks 扫描事件回调（由上层注入，桥接 EventsEmit）
type MassZoneScanCallbacks struct {
	OnProgress func(MassZoneScanProgress)
	OnHit      func(MassZoneHit)
}

type massZoneScanner struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	progress MassZoneScanProgress
}

var massZoneScannerInst = &massZoneScanner{}

// StartMassZoneScan 启动全市场 MASS 预警扫描；已在运行时返回错误。
func StartMassZoneScan(req MassZoneScanRequest, cbs MassZoneScanCallbacks) error {
	s := massZoneScannerInst
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("扫描正在进行中，请先停止或等待完成")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.cancel = cancel
	s.progress = MassZoneScanProgress{
		Running:   true,
		StartedAt: time.Now().UnixMilli(),
	}
	s.mu.Unlock()

	go s.run(ctx, req, cbs)
	return nil
}

// StopMassZoneScan 停止扫描（异步生效，进度快照保持可查）
func StopMassZoneScan() {
	s := massZoneScannerInst
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

// GetMassZoneScanStatus 查询扫描进度快照
func GetMassZoneScanStatus() MassZoneScanProgress {
	s := massZoneScannerInst
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *massZoneScanner) run(ctx context.Context, req MassZoneScanRequest, cbs MassZoneScanCallbacks) {
	defer func() {
		if e := recover(); e != nil {
			logger.SugaredLogger.Errorf("MassZoneScan panic: %v", e)
			s.finish("", fmt.Sprintf("扫描异常: %v", e), cbs)
		}
	}()

	if req.MinTrendScore < 1 || req.MinTrendScore > 5 {
		req.MinTrendScore = 3
	}

	api := NewTdxKLineApi()
	list := api.GetAllStockList()
	if list == nil || len(*list) == 0 {
		s.finish("通达信未返回股票列表（可能行情主站不可达）", "", cbs)
		return
	}

	want := map[string]bool{}
	for _, m := range req.Markets {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "SH" || m == "SZ" || m == "BJ" {
			want[m] = true
		}
	}
	// 板块细分过滤（主板/创业板/科创板/北交所，空=全部）
	boards := map[string]bool{}
	for _, b := range req.Boards {
		b = strings.ToLower(strings.TrimSpace(b))
		if b == "main" || b == "gem" || b == "star" || b == "bj" {
			boards[b] = true
		}
	}
	var stocks []TdxStockBasic
	for _, it := range *list {
		// 剔除基金 ETF/LOF、债券等非个股标的，只扫描 A 股个股
		if !isAShareStock(it) {
			continue
		}
		if len(boards) > 0 && !boards[boardOf(it)] {
			continue
		}
		if len(want) == 0 || want[it.Market] {
			stocks = append(stocks, it)
		}
	}
	if len(stocks) == 0 {
		s.finish("所选市场范围内没有股票", "", cbs)
		return
	}

	s.mu.Lock()
	s.progress.Total = len(stocks)
	s.mu.Unlock()

	lastEmit := time.Now()
	for idx, st := range stocks {
		select {
		case <-ctx.Done():
			s.finish("", "已手动停止", cbs)
			return
		default:
		}

		s.mu.Lock()
		s.progress.Scanned = idx + 1
		s.progress.Current = fmt.Sprintf("%s %s", st.Name, st.Code)
		s.mu.Unlock()

		now := time.Now()
		if cbs.OnProgress != nil && now.Sub(lastEmit) >= progressEmitThrottle {
			lastEmit = now
			cbs.OnProgress(GetMassZoneScanStatus())
		}

		s.scanOne(ctx, st, req, cbs)
	}
	s.finish("", "", cbs)
}

func (s *massZoneScanner) finish(errMsg, stopMsg string, cbs MassZoneScanCallbacks) {
	s.mu.Lock()
	s.running = false
	s.cancel = nil
	s.progress.Running = false
	s.progress.FinishedAt = time.Now().UnixMilli()
	if errMsg != "" {
		s.progress.Error = errMsg
		logger.SugaredLogger.Errorf("MassZoneScan 终止: %s", errMsg)
	} else if stopMsg != "" {
		s.progress.Error = stopMsg
	} else {
		logger.SugaredLogger.Infof("MassZoneScan 完成: 共 %d 只, 命中 %d 条", s.progress.Scanned, s.progress.Hits)
	}
	p := s.progress
	s.mu.Unlock()
	if cbs.OnProgress != nil {
		cbs.OnProgress(p)
	}
}

// scanOne 扫描单只股票：按请求周期拉K线 → MASS 预警窗口判定 → 上升趋势过滤 → 回调命中
func (s *massZoneScanner) scanOne(ctx context.Context, st TdxStockBasic, req MassZoneScanRequest, cbs MassZoneScanCallbacks) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	// 复权口径与买卖点扫描器一致：场内 ETF 不复权，股票前复权
	adjust := "qfq"
	if IsOnExchangeFund(st.Code) {
		adjust = "none"
	}
	klt := massKlt(req.Klt)
	bars := NewTdxKLineApi().GetKLineData(st.StockCode, klt, massScanLimit, adjust)
	if bars == nil || len(*bars) < massScanMinBars {
		return
	}
	rows := *bars
	n := len(rows)

	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i, row := range rows {
		hi, _ := strconv.ParseFloat(row.High, 64)
		lo, _ := strconv.ParseFloat(row.Low, 64)
		cl, _ := strconv.ParseFloat(row.Close, 64)
		highs[i], lows[i], closes[i] = hi, lo, cl
	}

	// ---- MASS 反转预警：尾部处于凸起（越过进入线、尚未跌破离开线）----
	mass := okxMassIndex(highs, lows)
	enter, exit := okxMassAdaptiveThresholds(mass)
	// 自适应阈值要求有效 MI 样本 >= okxMassMinSamples，否则回退经典 27/26.5
	adaptive := massValidCount(mass) >= okxMassMinSamples
	zoneStartIdx := massBulgeStart(mass, enter, exit)
	if zoneStartIdx < 0 {
		return // 不在预警窗口内（未凸起或已回落确认）
	}

	// ---- 进入预警根数过滤（一根K线=一个周期，从越过进入线那根起算；日K下即天数）----
	zoneBars := len(rows) - zoneStartIdx
	if req.MinZoneBars > 0 && zoneBars < req.MinZoneBars {
		return
	}
	if req.MaxZoneBars > 0 && zoneBars > req.MaxZoneBars {
		return
	}

	// ---- 多头/上升趋势：自选指标范围内计分（空=全部 5 路） ----
	trend := massUptrendScore(highs, lows, closes)
	score, info, total := massTrendRescore(trend.checks, massTrendSel(req.TrendChecks))
	required := req.MinTrendScore
	if required > total {
		required = total
	}
	if score < required {
		return
	}

	last := n - 1
	pct, _ := strconv.ParseFloat(rows[last].ChangePercent, 64)
	hit := MassZoneHit{
		StockCode:  st.StockCode,
		Code:       st.Code,
		Name:       st.Name,
		Market:     st.Market,
		Klt:        klt,
		MI:         mass[last],
		Enter:      enter,
		Exit:       exit,
		Adaptive:   adaptive,
		ZoneStart:  rows[zoneStartIdx].Day,
		ZoneBars:   zoneBars,
		TrendScore: score,
		TrendTotal: total,
		TrendInfo:  info,
		Close:      closes[last],
		Pct:        pct,
		MA20:       trend.ma20,
		MA60:       trend.ma60,
		Day:        rows[last].Day,
		Bars:       n,
	}
	s.mu.Lock()
	s.progress.Hits++
	s.mu.Unlock()
	if cbs.OnHit != nil {
		cbs.OnHit(hit)
	}
}

// massValidCount 有效 MI 样本数（okxMassIndex 用 0 表示无效）
func massValidCount(mass []float64) int {
	cnt := 0
	for _, v := range mass {
		if v > 0 {
			cnt++
		}
	}
	return cnt
}

// massBulgeStart 返回当前预警窗口的起始根下标；尾部不在凸起中返回 -1。
// 状态机与前端 computeMassIndexZones / okxMassTailState 一致：
// 越过进入线开窗，跌破离开线关窗，扫描结束时仍开着即「反转预警」。
func massBulgeStart(mass []float64, enter, exit float64) int {
	inWindow := false
	startIdx := -1
	for i, v := range mass {
		if v <= 0 {
			continue
		}
		if !inWindow {
			if v >= enter {
				inWindow = true
				startIdx = i
			}
			continue
		}
		if v < exit {
			inWindow = false
			startIdx = -1
		}
	}
	if inWindow {
		return startIdx
	}
	return -1
}

// massTrendCheck 一路趋势证据：key 供扫描请求自选过滤，name 供明细展示
type massTrendCheck struct {
	key  string
	name string
	ok   bool
}

// massUptrend 上升趋势判定结果
type massUptrend struct {
	score  int
	info   string
	ma20   float64
	ma60   float64
	checks []massTrendCheck // 5 路证据明细（供扫描器按自选指标重新计分；回测仍用全量 score）
}

// massUptrendScore 复刻前端 calc.ts okxUptrendScan 的 5 路趋势证据（最后一根收盘时点状态）。
func massUptrendScore(highs, lows, closes []float64) massUptrend {
	return massUptrendAt(highs, lows, closes, len(closes)-1)
}

// massUptrendAt 任意根 i 的 5 路趋势证据（i 为收盘时点下标），供回测在历史信号日复用：
//  1. 多头排列：close > MA20 > MA60
//  2. 均线向上：MA20 较 5 根前上行
//  3. Supertrend(10,3) 方向为多
//  4. ADX(14) >= 25 且 +DI > -DI
//  5. 通道上半区：close >= Donchian(20) 中轨
// 所用指标均为因果指标（只用历史数据），取截至 i 的值即无未来函数。
func massUptrendAt(highs, lows, closes []float64, i int) massUptrend {
	n := len(closes)
	if i < 0 || i >= n {
		return massUptrend{}
	}
	ma20 := smaValues(closes, 20)
	ma60 := smaValues(closes, 60)
	adx, diP, diM := adxBundle(highs, lows, closes, 14)
	dir := supertrendDirection(highs, lows, closes, 10, 3)
	donU, donL := donchianChannel(highs, lows, 20)

	maAlign := !isNaN(ma20[i]) && !isNaN(ma60[i]) && closes[i] > ma20[i] && ma20[i] > ma60[i]
	maRising := !isNaN(ma20[i]) && i-5 >= 0 && !isNaN(ma20[i-5]) && ma20[i] > ma20[i-5]
	stLong := dir[i] == 1
	adxTrend := !isNaN(adx[i]) && adx[i] >= 25 && !isNaN(diP[i]) && !isNaN(diM[i]) && diP[i] > diM[i]
	upperHalf := !isNaN(donU[i]) && !isNaN(donL[i]) && closes[i] >= (donU[i]+donL[i])/2

	checks := []massTrendCheck{
		{"ma_align", "多头排列", maAlign},
		{"ma20_up", "MA20上行", maRising},
		{"supertrend", "Supertrend翻多", stLong},
		{"adx", "ADX强多头", adxTrend},
		{"donchian", "通道上半区", upperHalf},
	}
	score, parts := 0, []string{}
	for _, c := range checks {
		if c.ok {
			score++
			parts = append(parts, c.name)
		}
	}
	return massUptrend{
		score:  score,
		info:   strings.Join(parts, "、"),
		ma20:   ma20[i],
		ma60:   ma60[i],
		checks: checks,
	}
}

// massTrendSel 归一化自选趋势指标：非法 key 剔除；空/全非法 = 全部 5 路
func massTrendSel(req []string) map[string]bool {
	all := []string{"ma_align", "ma20_up", "supertrend", "adx", "donchian"}
	m := map[string]bool{}
	for _, k := range req {
		switch k {
		case "ma_align", "ma20_up", "supertrend", "adx", "donchian":
			m[k] = true
		}
	}
	if len(m) == 0 {
		for _, k := range all {
			m[k] = true
		}
	}
	return m
}

// massTrendRescore 在自选指标范围内重新计分：返回（满足数、满足明细、参与数）
func massTrendRescore(checks []massTrendCheck, sel map[string]bool) (int, string, int) {
	score, total := 0, 0
	parts := []string{}
	for _, c := range checks {
		if !sel[c.key] {
			continue
		}
		total++
		if c.ok {
			score++
			parts = append(parts, c.name)
		}
	}
	return score, strings.Join(parts, "、"), total
}
