package data

/**
 * 全市场买卖点扫描器（TDX/gotdx 数据源）。
 *
 * 用途：按市场范围（沪/深/京/全部）枚举全市场股票，逐只拉通达信 K 线，
 * 用 Go 版买卖点算法（signal_algo.go，与前端 calc.ts 同口径）打分，
 * 把最近 recentDays 个交易日内命中的买卖点信号流式推给前端。
 *
 * 设计要点：
 *  - TDX 标准行情客户端是单连接（内部互斥），扫描顺序执行，不做并发拉取；
 *  - 进度回调节流（默认 300ms）避免几千次事件刷爆前端；
 *  - 事件推送由上层注入回调（data 包不依赖 wails runtime）；
 *  - 全程单实例：重复 Start 返回错误；Stop 走 context 取消，扫完自动复位。
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

// 信号算法所需最少 K 线根数（与前端 signalMonitor.ts MIN_BARS 一致，低于此值门控会静默失效）
const marketScanMinBars = 300

// dailyLikeKlt 日K类周期（分时之外；与前端 constants.ts DAILY_LIKE_KLT 一致）
func dailyLikeKlt(klt string) bool {
	switch klt {
	case "101", "102", "103", "104", "106":
		return true
	}
	return false
}

// marketScanLimit 取数根数：与前端 scanLimitForKlt 同口径（最近一年折算，下限 MIN_BARS，上限图表 limit）
func marketScanLimit(klt string) int {
	yearBars := map[string]int{
		"1": 244 * 240, "5": 244 * 48, "15": 244 * 16, "30": 244 * 8,
		"60": 244 * 4, "101": 244, "102": 49, "103": 12, "104": 4, "106": 1,
	}
	chartLimit := map[string]int{
		"1": 1000, "5": 600, "15": 500, "30": 500, "60": 500,
		"101": 800, "102": 520, "103": 240, "104": 120, "106": 40,
	}
	limit, ok := chartLimit[klt]
	if !ok {
		limit = 800
	}
	year := chartLimit[klt]
	if y, ok := yearBars[klt]; ok {
		year = y
	}
	if year < marketScanMinBars {
		year = marketScanMinBars
	}
	if limit > year {
		limit = year
	}
	return limit
}

// MarketSignalScanRequest 全市场扫描请求
type MarketSignalScanRequest struct {
	Markets    []string `json:"markets"`    // 市场范围：SH/SZ/BJ 任意组合；空=全部
	Klt        string   `json:"klt"`        // 周期（101=日K，5/15/30/60=分时…）
	MinScore   int      `json:"minScore"`   // 灵敏度档：4=灵敏 6=标准 7=严格（与前端 BUY_SELL_SCORE_OPTIONS 一致）
	RecentDays int      `json:"recentDays"` // 只保留最近 N 个交易日内的信号
}

// MarketSignalHit 单条扫描命中
type MarketSignalHit struct {
	StockCode   string  `json:"stockCode"`   // 带市场前缀（sh600519，TDX 格式）
	Code        string  `json:"code"`        // 纯代码（600519，展示与东财 K 线用）
	Name        string  `json:"name"`        // 股票名称
	Market      string  `json:"market"`      // SH/SZ/BJ
	Klt         string  `json:"klt"`         // 周期
	Kind        string  `json:"kind"`        // buy / sell
	Time        int64   `json:"time"`        // 信号 K 线时间（unix 秒）
	TimeText    string  `json:"timeText"`    // 信号时间文本（yyyy-MM-dd HH:mm）
	Price       float64 `json:"price"`       // 信号价（信号 K 线收盘价，与前端 detectRecentBuySell 展示口径一致）
	SignalPrice float64 `json:"signalPrice"` // 算法价（买=low / 卖=high）
	Score       float64 `json:"score"`       // 共振路数（0-14）
	Reasons     string  `json:"reasons"`     // 命中信号（顿号分隔）
	LatestClose float64 `json:"latestClose"` // 最新收盘价
	LatestPct   float64 `json:"latestPct"`   // 最新一根涨跌幅 %
	Lag         int     `json:"lag"`         // 距最新一根的根数
	Bars        int     `json:"bars"`        // 参与计算的 K 线根数
}

// MarketSignalScanProgress 扫描进度快照
type MarketSignalScanProgress struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`      // 本轮计划扫描的股票数
	Scanned    int    `json:"scanned"`   // 已扫描数
	Hits       int    `json:"hits"`      // 命中信号数
	Current    string `json:"current"`   // 当前正在扫描的股票（名称+代码）
	StartedAt  int64  `json:"startedAt"` // 开始时间（毫秒）
	FinishedAt int64  `json:"finishedAt"`
	Error      string `json:"error"`
}

// MarketSignalScanCallbacks 扫描事件回调（由上层注入，桥接 EventsEmit）
type MarketSignalScanCallbacks struct {
	OnProgress func(MarketSignalScanProgress) // 进度（已节流）
	OnHit      func(MarketSignalHit)          // 单条命中
}

// marketSignalScanner 全市场扫描器单例
type marketSignalScanner struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	progress MarketSignalScanProgress
}

var marketSignalScannerInst = &marketSignalScanner{}

// StartMarketSignalScan 启动全市场扫描；已在运行时返回错误。
func StartMarketSignalScan(req MarketSignalScanRequest, cbs MarketSignalScanCallbacks) error {
	s := marketSignalScannerInst
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("扫描正在进行中，请先停止或等待完成")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.cancel = cancel
	s.progress = MarketSignalScanProgress{
		Running:   true,
		StartedAt: time.Now().UnixMilli(),
	}
	s.mu.Unlock()

	go s.run(ctx, req, cbs)
	return nil
}

// StopMarketSignalScan 停止扫描（异步生效，进度快照保持可查）
func StopMarketSignalScan() {
	s := marketSignalScannerInst
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

// GetMarketSignalScanStatus 查询扫描进度快照
func GetMarketSignalScanStatus() MarketSignalScanProgress {
	s := marketSignalScannerInst
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

// progressEmitThrottle 进度回调节流间隔
const progressEmitThrottle = 300 * time.Millisecond

func (s *marketSignalScanner) run(ctx context.Context, req MarketSignalScanRequest, cbs MarketSignalScanCallbacks) {
	defer func() {
		if e := recover(); e != nil {
			logger.SugaredLogger.Errorf("MarketSignalScan panic: %v", e)
			s.finish("", fmt.Sprintf("扫描异常: %v", e), cbs)
		}
	}()

	klt := req.Klt
	if klt == "" {
		klt = "101"
	}
	if req.MinScore != 4 && req.MinScore != 6 && req.MinScore != 7 {
		req.MinScore = 6
	}
	if req.RecentDays < 1 {
		req.RecentDays = 1
	}
	minGroups := 2 // 三档灵敏度 minGroups 均为 2（与前端 BUY_SELL_SCORE_OPTIONS 一致）

	api := NewTdxKLineApi()
	list := api.GetAllStockList()
	if list == nil || len(*list) == 0 {
		s.finish("通达信未返回股票列表（可能行情主站不可达）", "", cbs)
		return
	}

	// 按市场范围过滤（空=全部）
	want := map[string]bool{}
	for _, m := range req.Markets {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "SH" || m == "SZ" || m == "BJ" {
			want[m] = true
		}
	}
	var stocks []TdxStockBasic
	for _, it := range *list {
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

	limit := marketScanLimit(klt)
	intraday := !dailyLikeKlt(klt)
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

		// 进度节流回调
		now := time.Now()
		if cbs.OnProgress != nil && now.Sub(lastEmit) >= progressEmitThrottle {
			lastEmit = now
			cbs.OnProgress(GetMarketSignalScanStatus())
		}

		s.scanOne(ctx, st, klt, limit, intraday, req, minGroups, cbs)
	}
	s.finish("", "", cbs)
}

// finish 收尾：写终态并回调进度
func (s *marketSignalScanner) finish(errMsg, stopMsg string, cbs MarketSignalScanCallbacks) {
	s.mu.Lock()
	s.running = false
	s.cancel = nil
	s.progress.Running = false
	s.progress.FinishedAt = time.Now().UnixMilli()
	if errMsg != "" {
		s.progress.Error = errMsg
		logger.SugaredLogger.Errorf("MarketSignalScan 终止: %s", errMsg)
	} else if stopMsg != "" {
		s.progress.Error = stopMsg
	} else {
		logger.SugaredLogger.Infof("MarketSignalScan 完成: 共 %d 只, 命中 %d 条", s.progress.Scanned, s.progress.Hits)
	}
	p := s.progress
	s.mu.Unlock()
	if cbs.OnProgress != nil {
		cbs.OnProgress(p)
	}
}

// scanOne 扫描单只股票：拉 K 线 → 打分 → 时间窗口过滤 → 逐条回调命中
func (s *marketSignalScanner) scanOne(ctx context.Context, st TdxStockBasic, klt string, limit int,
	intraday bool, req MarketSignalScanRequest, minGroups int, cbs MarketSignalScanCallbacks) {

	select {
	case <-ctx.Done():
		return
	default:
	}

	// 复权口径与前端 adjustForCode 一致：场内 ETF 不复权，股票前复权
	adjust := "qfq"
	if IsOnExchangeFund(st.Code) {
		adjust = "none"
	}
	bars := NewTdxKLineApi().GetKLineData(st.StockCode, klt, limit, adjust)
	if bars == nil || len(*bars) < marketScanMinBars {
		return // 根数不足（新股/长期停牌）：门控会静默失效，不出信号
	}
	rows := *bars
	n := len(rows)

	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	vols := make([]float64, n)
	times := make([]int64, n)
	dayKeys := make([]string, n)
	cst := time.FixedZone("CST", 8*3600)
	for i, row := range rows {
		hi, _ := strconv.ParseFloat(row.High, 64)
		lo, _ := strconv.ParseFloat(row.Low, 64)
		cl, _ := strconv.ParseFloat(row.Close, 64)
		vo, _ := strconv.ParseFloat(row.Volume, 64)
		highs[i], lows[i], closes[i], vols[i] = hi, lo, cl, vo
		dayKeys[i] = row.Day
		if t, err := time.ParseInLocation("2006-01-02 15:04", row.Day, cst); err == nil {
			times[i] = t.Unix()
		} else if i > 0 {
			times[i] = times[i-1]
		}
	}

	opts := &BuySellPointsOptions{
		MinScore:   float64(req.MinScore),
		MinGroups:  minGroups,
		DayKeys:    dayKeys,
		Intraday:   intraday,
	}
	buys, sells := BuySellPointsValues(highs, lows, closes, vols, opts)

	// 时间窗口：最近 recentDays 个交易日（按 K 线出现过的自然日保序去重后取末尾，与前端口径一致）
	allowDays := map[string]bool{}
	{
		seen := map[string]bool{}
		distinct := []string{}
		for _, d := range dayKeys {
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			distinct = append(distinct, d)
		}
		start := len(distinct) - req.RecentDays
		if start < 0 {
			start = 0
		}
		for _, d := range distinct[start:] {
			allowDays[d] = true
		}
	}

	last := n - 1
	latestClose := closes[last]
	latestPct, _ := strconv.ParseFloat(rows[last].ChangePercent, 64)

	emitHit := func(kind string, pts []BuySellPoint) {
		for _, p := range pts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if p.I < 0 || p.I > last || !allowDays[dayKeys[p.I]] {
				continue
			}
			hit := MarketSignalHit{
				StockCode:   st.StockCode,
				Code:        st.Code,
				Name:        st.Name,
				Market:      st.Market,
				Klt:         klt,
				Kind:        kind,
				Time:        times[p.I],
				TimeText:    rows[p.I].Day,
				Price:       closes[p.I],
				SignalPrice: p.Price,
				Score:       p.Score,
				Reasons:     strings.Join(p.Reasons, "、"),
				LatestClose: latestClose,
				LatestPct:   latestPct,
				Lag:         last - p.I,
				Bars:        n,
			}
			s.mu.Lock()
			s.progress.Hits++
			s.mu.Unlock()
			if cbs.OnHit != nil {
				cbs.OnHit(hit)
			}
		}
	}
	emitHit("buy", buys)
	emitHit("sell", sells)
}
