package data

/**
 * MASS 反转策略回测器（TDX/gotdx 数据源，日K）。
 *
 * 目的：验证「进入 MASS 反转预警区域后买入」的历史表现——胜率、平均收益、
 * 盈亏比，并横向对比多种卖出策略，寻找最佳买卖组合。
 *
 * 回测口径（严格避免未来函数）：
 *  - 入场信号：日K 收盘后确认「MI 首次越过进入线」（凸起首日），以信号日收盘价买入
 *    （对应实盘尾盘确认、尾盘买入）；同一凸起只入场一次。
 *  - 阈值 point-in-time：信号日的进入线只用「截至该根」的 MI 历史样本算 P90
 *    （钳制 24~31、样本不足回退 27/26.5，与当前扫描器/主图竖带的当下口径一致）。
 *  - 趋势过滤：入场日的 5 路趋势证据分（massUptrendAt，与 MASS 预警扫描器同口径），
 *    逐笔记录并按趋势分桶统计，可直接对比出趋势过滤的增益。
 *  - 卖出：各策略以触发日收盘价卖出（收盘确认、当日收盘成交），无滑点与税费。
 *
 * 卖出策略集合（同一入场事件并行模拟，互不影响）：
 *  hold3/hold5/hold10/hold20 —— 固定持有 N 个交易日
 *  massExit   —— MI 回落跌破离开线（经典反转确认）卖出
 *  ma10Break  —— 收盘自上向下跌破 MA10 卖出
 *  ma20Break  —— 收盘自上向下跌破 MA20 卖出
 *  tp10sl8    —— 止盈 +10% / 止损 -8%（收盘触发，同根先判止损）
 *  combo      —— massExit 或 跌破MA20，先到先卖
 * 事件驱动策略持有满 40 根仍未触发则兜底强平；数据尽头未平仓按最后一根收盘强平。
 */

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// massBtMinBars 参与回测的最少根数：MI 有效样本（120）+ MA60/ADX 预热 + 一定历史
const massBtMinBars = 250

// massBtMaxHold 事件驱动卖出策略的兜底持有根数
const massBtMaxHold = 40

// massBtKlt 回测固定日K
const massBtKlt = "101"

// MassBacktestRequest 回测请求
type MassBacktestRequest struct {
	Markets      []string `json:"markets"`      // 市场范围：SH/SZ/BJ 任意组合；空=全部
	Boards       []string `json:"boards"`       // 板块细分：main/gem/star/bj 任意组合；空=全部
	LookbackBars int      `json:"lookbackBars"` // 日K回看根数（300~800，默认 500）
	SampleLimit  int      `json:"sampleLimit"`  // 只回测前 N 只（0=全部），用于快速抽样
}

// MassBacktestEvent 单次入场事件（同一凸起只入场一次），含各卖出策略的模拟结果
type MassBacktestEvent struct {
	StockCode  string             `json:"stockCode"`  // 带市场前缀（TDX 格式）
	Code       string             `json:"code"`       // 纯代码
	Name       string             `json:"name"`       // 股票名称
	Market     string             `json:"market"`     // SH/SZ/BJ
	SignalDate string             `json:"signalDate"` // 信号日=买入日（收盘确认、收盘买入）
	BuyPrice   float64            `json:"buyPrice"`   // 信号日收盘价
	TrendScore int                `json:"trendScore"` // 入场日 5 路趋势证据分（0~5）
	TrendInfo  string             `json:"trendInfo"`  // 五路证据明细
	MI         float64            `json:"mi"`         // 信号日 MI 值
	Enter      float64            `json:"enter"`      // 信号日进入线（point-in-time）
	Adaptive   bool               `json:"adaptive"`   // 阈值是否自适应（false=经典 27/26.5 回退）
	ZoneStart  string             `json:"zoneStart"`  // 凸起起始日（=信号日）
	Rets       map[string]float64 `json:"rets"`       // 卖出策略 key → 收益率 %
	Holds      map[string]int     `json:"holds"`      // 卖出策略 key → 持有交易日数
}

// MassBacktestStat 单策略 × 单趋势分桶的统计
type MassBacktestStat struct {
	Strategy      string  `json:"strategy"`      // 策略 key
	StrategyLabel string  `json:"strategyLabel"` // 策略中文名
	Bucket        string  `json:"bucket"`        // 趋势分桶
	Trades        int     `json:"trades"`        // 交易笔数
	Wins          int     `json:"wins"`          // 盈利笔数
	WinRate       float64 `json:"winRate"`       // 胜率 %
	AvgRet        float64 `json:"avgRet"`        // 平均收益率 %（期望值）
	MedRet        float64 `json:"medRet"`        // 收益率中位数 %
	AvgWin        float64 `json:"avgWin"`        // 平均盈利 %
	AvgLoss       float64 `json:"avgLoss"`       // 平均亏损 %（负数）
	ProfitFactor  float64 `json:"profitFactor"`  // 盈亏比 = 总盈利/总亏损
	MaxLoss       float64 `json:"maxLoss"`       // 最大单笔亏损 %
	AvgHold       float64 `json:"avgHold"`       // 平均持有交易日数
}

// MassBacktestResult 回测最终汇总
type MassBacktestResult struct {
	Stats      []MassBacktestStat `json:"stats"`      // 全部 策略×分桶 统计
	StockCount int                `json:"stockCount"` // 实际参与回测的股票数
	EventCount int                `json:"eventCount"` // 入场事件总数（=凸起次数）
	Stopped    bool               `json:"stopped"`    // 是否中途手动停止
	Error      string             `json:"error"`      // 终止原因（空=正常完成）
}

// MassBacktestProgress 回测进度快照
type MassBacktestProgress struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Scanned    int    `json:"scanned"`
	Events     int    `json:"events"` // 已产生入场事件数
	Current    string `json:"current"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Error      string `json:"error"`
}

// MassBacktestCallbacks 回测事件回调（由上层注入，桥接 EventsEmit）
type MassBacktestCallbacks struct {
	OnProgress func(MassBacktestProgress)
	OnEvent    func(MassBacktestEvent)
	OnResult   func(MassBacktestResult)
}

// massBtStrategies 卖出策略集合（顺序即统计表展示顺序）
var massBtStrategies = []struct {
	key   string
	label string
}{
	{"hold3", "固定持有3日"},
	{"hold5", "固定持有5日"},
	{"hold10", "固定持有10日"},
	{"hold20", "固定持有20日"},
	{"massExit", "MI跌破离开线"},
	{"ma10Break", "跌破MA10"},
	{"ma20Break", "跌破MA20"},
	{"tp10sl8", "止盈10%/止损8%"},
	{"combo", "MI离开线或破MA20"},
}

// massBtBuckets 趋势分桶（固定顺序；一笔交易可同时属于多个桶）
var massBtBuckets = []string{"全部", "趋势≥3", "趋势≥4", "趋势=5", "趋势≤2"}

// massBtBucketsOf 交易按趋势分归属的桶
func massBtBucketsOf(score int) []string {
	out := []string{"全部"}
	if score >= 3 {
		out = append(out, "趋势≥3")
	}
	if score >= 4 {
		out = append(out, "趋势≥4")
	}
	if score == 5 {
		out = append(out, "趋势=5")
	}
	if score <= 2 {
		out = append(out, "趋势≤2")
	}
	return out
}

// ---- 统计聚合（并发安全；当前为顺序扫描，锁仅为防御未来并行化） ----

type massBtAgg struct {
	rets  []float64
	holds []int
}

type massBtStats struct {
	mu sync.Mutex
	m  map[string]*massBtAgg // key: strategy|bucket
}

func newMassBtStats() *massBtStats {
	return &massBtStats{m: map[string]*massBtAgg{}}
}

func (a *massBtStats) add(strategy, bucket string, ret float64, hold int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	k := strategy + "|" + bucket
	agg := a.m[k]
	if agg == nil {
		agg = &massBtAgg{}
		a.m[k] = agg
	}
	agg.rets = append(agg.rets, ret)
	agg.holds = append(agg.holds, hold)
}

// snapshot 生成全部 策略×分桶 统计（顺序与 massBtStrategies/massBtBuckets 一致）
func (a *massBtStats) snapshot() []MassBacktestStat {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]MassBacktestStat, 0, len(massBtStrategies)*len(massBtBuckets))
	for _, stg := range massBtStrategies {
		for _, bucket := range massBtBuckets {
			agg := a.m[stg.key+"|"+bucket]
			if agg == nil || len(agg.rets) == 0 {
				continue
			}
			n := len(agg.rets)
			sum, wins, winSum, lossSum := 0.0, 0, 0.0, 0.0
			maxLoss := 0.0
			holdSum := 0
			for i, r := range agg.rets {
				sum += r
				holdSum += agg.holds[i]
				if r > 0 {
					wins++
					winSum += r
				} else if r < 0 {
					lossSum += r
					if r < maxLoss {
						maxLoss = r
					}
				}
			}
			sorted := append([]float64(nil), agg.rets...)
			sort.Float64s(sorted)
			med := sorted[n/2]
			if n%2 == 0 {
				med = (sorted[n/2-1] + sorted[n/2]) / 2
			}
			st := MassBacktestStat{
				Strategy:      stg.key,
				StrategyLabel: stg.label,
				Bucket:        bucket,
				Trades:        n,
				Wins:          wins,
				WinRate:       float64(wins) / float64(n) * 100,
				AvgRet:        sum / float64(n),
				MedRet:        med,
				AvgHold:       float64(holdSum) / float64(n),
				MaxLoss:       maxLoss,
			}
			if wins > 0 {
				st.AvgWin = winSum / float64(wins)
			}
			if wins < n {
				st.AvgLoss = lossSum / float64(n-wins)
			}
			if lossSum < 0 {
				st.ProfitFactor = winSum / (-lossSum)
			} else if winSum > 0 {
				st.ProfitFactor = 99 // 无亏损样本
			}
			out = append(out, st)
		}
	}
	return out
}

// ---- 回测单例（与 massZoneScanner 同款生命周期管理） ----

type massBtScanner struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	progress MassBacktestProgress
}

var massBtScannerInst = &massBtScanner{}

// StartMassBacktest 启动回测；已在运行时返回错误。
func StartMassBacktest(req MassBacktestRequest, cbs MassBacktestCallbacks) error {
	s := massBtScannerInst
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("回测正在进行中，请先停止或等待完成")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.cancel = cancel
	s.progress = MassBacktestProgress{
		Running:   true,
		StartedAt: time.Now().UnixMilli(),
	}
	s.mu.Unlock()

	go s.run(ctx, req, cbs)
	return nil
}

// StopMassBacktest 停止回测（异步生效；结束后 OnResult 会带 stopped=true）
func StopMassBacktest() {
	s := massBtScannerInst
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

// GetMassBacktestStatus 查询回测进度快照
func GetMassBacktestStatus() MassBacktestProgress {
	s := massBtScannerInst
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *massBtScanner) run(ctx context.Context, req MassBacktestRequest, cbs MassBacktestCallbacks) {
	result := MassBacktestResult{}
	defer func() {
		if e := recover(); e != nil {
			logger.SugaredLogger.Errorf("MassBacktest panic: %v", e)
			result.Error = fmt.Sprintf("回测异常: %v", e)
			s.finish(result, cbs)
		}
	}()

	if req.LookbackBars < 300 {
		req.LookbackBars = 500
	}
	if req.LookbackBars > 800 {
		req.LookbackBars = 800
	}

	api := NewTdxKLineApi()
	list := api.GetAllStockList()
	if list == nil || len(*list) == 0 {
		result.Error = "通达信未返回股票列表（可能行情主站不可达）"
		s.finish(result, cbs)
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
		// 剔除基金 ETF/LOF、债券等非个股标的，只回测 A 股个股
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
		result.Error = "所选市场范围内没有股票"
		s.finish(result, cbs)
		return
	}
	if req.SampleLimit > 0 && req.SampleLimit < len(stocks) {
		stocks = stocks[:req.SampleLimit]
	}

	s.mu.Lock()
	s.progress.Total = len(stocks)
	s.mu.Unlock()

	agg := newMassBtStats()
	lastEmit := time.Now()
	for idx, st := range stocks {
		select {
		case <-ctx.Done():
			result.Stopped = true
			result.Error = "已手动停止"
			s.finish(result, cbs)
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
			cbs.OnProgress(GetMassBacktestStatus())
		}

		s.btScanOne(ctx, st, req, agg, cbs, &result)
	}

	result.Stats = agg.snapshot()
	s.finish(result, cbs)
}

func (s *massBtScanner) finish(result MassBacktestResult, cbs MassBacktestCallbacks) {
	s.mu.Lock()
	s.running = false
	s.cancel = nil
	s.progress.Running = false
	s.progress.FinishedAt = time.Now().UnixMilli()
	s.progress.Error = result.Error
	if result.Error != "" {
		logger.SugaredLogger.Warnf("MassBacktest 终止: %s", result.Error)
	} else {
		logger.SugaredLogger.Infof("MassBacktest 完成: %d 只, %d 次入场事件", result.StockCount, result.EventCount)
	}
	p := s.progress
	s.mu.Unlock()
	if cbs.OnProgress != nil {
		cbs.OnProgress(p)
	}
	if cbs.OnResult != nil {
		cbs.OnResult(result)
	}
}

// btScanOne 回测单只股票：拉日K → 逐根凸起状态机（point-in-time 阈值）→
// 凸起首日生成入场事件并模拟全部卖出策略 → 聚合统计 + 流式回调
func (s *massBtScanner) btScanOne(ctx context.Context, st TdxStockBasic, req MassBacktestRequest, agg *massBtStats, cbs MassBacktestCallbacks, result *MassBacktestResult) {
	// 复权口径与扫描器一致：场内 ETF 不复权，股票前复权
	adjust := "qfq"
	if IsOnExchangeFund(st.Code) {
		adjust = "none"
	}
	bars := NewTdxKLineApi().GetKLineData(st.StockCode, massBtKlt, req.LookbackBars, adjust)
	if bars == nil || len(*bars) < massBtMinBars {
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

	mass := okxMassIndex(highs, lows)
	ma10 := smaValues(closes, 10)
	ma20 := smaValues(closes, 20)

	// 凸起状态机：越过进入线开窗（首日=入场信号），跌破离开线关窗。
	// 阈值 point-in-time：只在可能穿越/离场的根上精确计算（MI>=23.5 粗筛，
	// 因为进入线钳制下界 24、离开线>=23.5，低于该值无需排序）。
	inZone := false
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		v := mass[i]
		if v <= 0 {
			continue
		}
		if !inZone {
			if v < okxMassMinEnter {
				continue
			}
			enter, _, adaptive := massBtThresholdsUntil(mass, i)
			if v >= enter {
				inZone = true
				s.btRecordEvent(st, rows, i, v, enter, adaptive, highs, lows, closes, mass, ma10, ma20, agg, cbs, result)
			}
			continue
		}
		// 在凸起中：判离场（仅维护状态机，卖出由各策略独立模拟）
		if v < okxMassMinEnter-okxMassHysteresis {
			inZone = false
			continue
		}
		_, ex, _ := massBtThresholdsUntil(mass, i)
		if v < ex {
			inZone = false
		}
	}
}

// btRecordEvent 生成一条入场事件：入场日趋势评分 + 全策略模拟 + 统计聚合 + 回调
func (s *massBtScanner) btRecordEvent(st TdxStockBasic, rows []KLineData, sig int, mi, enter float64, adaptive bool,
	highs, lows, closes, mass, ma10, ma20 []float64, agg *massBtStats, cbs MassBacktestCallbacks, result *MassBacktestResult) {

	trend := massUptrendAt(highs, lows, closes, sig)
	buy := closes[sig]
	fills := massBtSimulate(sig, buy, mass, ma10, ma20, closes)

	rets := make(map[string]float64, len(fills))
	holds := make(map[string]int, len(fills))
	for _, f := range fills {
		rets[f.key] = f.ret
		holds[f.key] = f.hold
		for _, bucket := range massBtBucketsOf(trend.score) {
			agg.add(f.key, bucket, f.ret, f.hold)
		}
	}

	s.mu.Lock()
	s.progress.Events++
	s.mu.Unlock()

	if cbs.OnEvent != nil {
		cbs.OnEvent(MassBacktestEvent{
			StockCode:  st.StockCode,
			Code:       st.Code,
			Name:       st.Name,
			Market:     st.Market,
			SignalDate: rows[sig].Day,
			BuyPrice:   buy,
			TrendScore: trend.score,
			TrendInfo:  trend.info,
			MI:         mi,
			Enter:      enter,
			Adaptive:   adaptive,
			ZoneStart:  rows[sig].Day,
			Rets:       rets,
			Holds:      holds,
		})
	}
	result.EventCount++
}

// massBtFill 单策略模拟成交
type massBtFill struct {
	key    string
	sellIdx int
	ret    float64 // 收益率 %
	hold   int     // 持有交易日数
	reason string
}

// massBtThresholdsUntil point-in-time 自适应阈值：只用 mass[0..until] 的有效样本，
// 与 okxMassAdaptiveThresholds 同规则（P90 钳制 24~31、样本不足回退 27/26.5），
// 但样本止于 until——回测中信号日不可能使用未来数据。
func massBtThresholdsUntil(mass []float64, until int) (enter, exit float64, adaptive bool) {
	vals := make([]float64, 0, until+1)
	for k := 0; k <= until; k++ {
		if mass[k] > 0 {
			vals = append(vals, mass[k])
		}
	}
	if len(vals) < okxMassMinSamples {
		return okxMassEnter, okxMassExit, false
	}
	sort.Float64s(vals)
	enter = okxMassQuantileValue(vals, okxMassQuantile)
	if enter < okxMassMinEnter {
		enter = okxMassMinEnter
	}
	if enter > okxMassMaxEnter {
		enter = okxMassMaxEnter
	}
	return enter, enter - okxMassHysteresis, true
}

// massBtSimulate 对一次入场并行模拟全部卖出策略（买入=信号日收盘价，
// 卖出=触发日收盘价；持有满 massBtMaxHold 兜底强平，数据尽头按最后一根收盘平仓）。
func massBtSimulate(sig int, buy float64, mass, ma10, ma20, closes []float64) []massBtFill {
	n := len(closes)
	fills := make([]massBtFill, 0, len(massBtStrategies))
	for _, stg := range massBtStrategies {
		sellIdx, reason := -1, ""
		for j := sig + 1; j < n; j++ {
			hold := j - sig
			prev := closes[j-1]
			cur := closes[j]
			switch stg.key {
			case "hold3":
				if hold >= 3 {
					sellIdx, reason = j, "持有满3日"
				}
			case "hold5":
				if hold >= 5 {
					sellIdx, reason = j, "持有满5日"
				}
			case "hold10":
				if hold >= 10 {
					sellIdx, reason = j, "持有满10日"
				}
			case "hold20":
				if hold >= 20 {
					sellIdx, reason = j, "持有满20日"
				}
			case "tp10sl8":
				// 同根先判止损（保守口径）
				if cur <= buy*0.92 {
					sellIdx, reason = j, "止损-8%"
				} else if cur >= buy*1.10 {
					sellIdx, reason = j, "止盈+10%"
				}
			case "massExit", "combo":
				// MI 回落跌破离开线（经典反转确认）；低于 23.5 必低于离开线（离开线>=23.5）
				mv := mass[j]
				if mv > 0 {
					if mv < okxMassMinEnter-okxMassHysteresis {
						sellIdx, reason = j, "MI回落确认"
					} else if _, ex, _ := massBtThresholdsUntil(mass, j); mv < ex {
						sellIdx, reason = j, "MI回落确认"
					}
				}
			}
			// 均线跌破（自上向下穿越才触发，避免入场时已在均线下方时误判）
			if sellIdx < 0 && stg.key == "ma10Break" {
				if !isNaN(ma10[j]) && j-1 >= 0 && !isNaN(ma10[j-1]) && cur < ma10[j] && prev >= ma10[j-1] {
					sellIdx, reason = j, "跌破MA10"
				}
			}
			if sellIdx < 0 && (stg.key == "ma20Break" || stg.key == "combo") {
				if !isNaN(ma20[j]) && j-1 >= 0 && !isNaN(ma20[j-1]) && cur < ma20[j] && prev >= ma20[j-1] {
					sellIdx, reason = j, "跌破MA20"
				}
			}
			// 兜底强平
			if sellIdx < 0 && hold >= massBtMaxHold {
				sellIdx, reason = j, "到期强平"
			}
			if sellIdx >= 0 {
				break
			}
		}
		if sellIdx < 0 {
			// 数据尽头仍未触发（凸起一直未回落/均未穿越）：按最后一根收盘平仓
			sellIdx, reason = n-1, "数据尽头平仓"
		}
		ret := (closes[sellIdx] - buy) / buy * 100
		fills = append(fills, massBtFill{key: stg.key, sellIdx: sellIdx, ret: ret, hold: sellIdx - sig, reason: reason})
	}
	return fills
}
