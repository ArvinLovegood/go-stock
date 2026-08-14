package agent

// recommend_backtest.go — AI 推荐效果回测（P3）。
//
// 把 AiRecommendStocks 的历史推荐与"推荐后 N 个交易日实际涨跌"对比，
// 计算个股收益率、基准（沪深300）收益率与超额收益，写入 ai_recommend_backtest，
// 供前端展示"AI 历史上推荐准不准"，并把"连续表现差/好"沉淀进学习经验，形成判断质量闭环。
//
// 设计原则：
//   - 复用 FetchKLineWithFallback 拉日 K，不新造行情源
//   - 低频：仅对"已过 N 交易日"且未回测的记录计算；避免每次对话触发
//   - 相对收益（vs 基准）而非绝对收益，缓解大盘时点偏差（见方案 7.1）
//   - 失败单条跳过，不阻断整体；无足够数据（推荐太近）自动跳过

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// RecommendBacktestApi 推荐回测 API（Wails 绑定）
type RecommendBacktestApi struct{}

// NewRecommendBacktestApi 构造推荐回测 API 实例
func NewRecommendBacktestApi() *RecommendBacktestApi {
	return &RecommendBacktestApi{}
}

// 沪深300 基准指数代码（有沪市镜像，走 MAC 主客户端/K线正常路径）
const backtestBenchmarkCode = "000300.SH"

// RunBacktest 对满足条件的推荐记录执行 N 交易日回测，返回本次回测的统计摘要。
// periodDays <=0 时默认 5。单次最多处理 100 条，避免耗时过长。
func (a *RecommendBacktestApi) RunBacktest(periodDays int) (string, error) {
	if periodDays <= 0 {
		periodDays = 5
	}
	if periodDays > 60 {
		periodDays = 60
	}

	// 已回测的 recommendID 排除集合
	var doneIDs []uint
	db.Dao.Model(&models.AiRecommendBacktest{}).Pluck("recommend_id", &doneIDs)
	doneSet := make(map[uint]bool, len(doneIDs))
	for _, id := range doneIDs {
		doneSet[id] = true
	}

	// 取全部推荐（按时间倒序），剔除已回测与推荐时间过近的
	var recs []models.AiRecommendStocks
	if err := db.Dao.Model(&models.AiRecommendStocks{}).Order("data_time desc").Find(&recs).Error; err != nil {
		return "", fmt.Errorf("查询推荐记录失败: %w", err)
	}

	now := time.Now()
	var total, win, skip int
	processed := 0
	for _, r := range recs {
		if processed >= 100 {
			break
		}
		if r.DataTime == nil {
			continue
		}
		if doneSet[r.ID] {
			continue
		}
		isWin, err := a.backtestOne(r, periodDays, now)
		if err != nil {
			skip++
			logger.SugaredLogger.Debugf("回测跳过 %s(%s): %v", r.StockName, r.StockCode, err)
			continue
		}
		processed++
		total++
		if isWin {
			win++
		}
	}

	if total == 0 {
		if skip > 0 {
			return fmt.Sprintf("本次无可回测记录（%d 条因数据不足/时间过近跳过）", skip), nil
		}
		return "暂无可回测的推荐记录（可能均已回测或暂无推荐）", nil
	}

	return fmt.Sprintf("回测完成：共 %d 条，其中 %d 条为正收益（胜率 %.1f%%），%d 条因数据不足跳过",
		total, win, float64(win)/float64(total)*100, skip), nil
}

// backtestOne 对单条推荐执行回测并写库，返回是否为正向收益。数据不足返回 error（调用方跳过）。
func (a *RecommendBacktestApi) backtestOne(r models.AiRecommendStocks, periodDays int, now time.Time) (bool, error) {
	recDate := r.DataTime.Truncate(24 * time.Hour)

	// 拉取从推荐日到现在足够多的日 K（含推荐前后），覆盖基准与个股
	daysBetween := int(now.Sub(recDate).Hours() / 24)
	limit := daysBetween + periodDays + 10
	if limit < 60 {
		limit = 60
	}
	if limit > 300 {
		limit = 300
	}

	stockRes := data.FetchKLineWithFallback(r.StockCode, r.StockName, "101", limit, "")
	if stockRes == nil || stockRes.Data == nil || len(*stockRes.Data) == 0 {
		return false, fmt.Errorf("个股K线为空")
	}
	stockBars := *stockRes.Data

	baseIdx, endIdx := findBacktestRange(stockBars, recDate, periodDays)
	if baseIdx < 0 || endIdx < 0 {
		return false, fmt.Errorf("推荐日(%s)之后不足 %d 个交易日", recDate.Format("2006-01-02"), periodDays)
	}

	baseClose, err := parsePrice(stockBars[baseIdx].Close)
	if err != nil || baseClose <= 0 {
		return false, fmt.Errorf("基准日收盘价无效")
	}
	endClose, err := parsePrice(stockBars[endIdx].Close)
	if err != nil || endClose <= 0 {
		return false, fmt.Errorf("期末收盘价无效")
	}

	returnPct := (endClose - baseClose) / baseClose * 100

	// 基准：沪深300 同期收益率（按相同两个交易日对齐）
	benchPct := 0.0
	if res := data.FetchKLineWithFallback(backtestBenchmarkCode, "沪深300", "101", limit, ""); res != nil && res.Data != nil {
		if bi, ei := findBacktestRange(*res.Data, recDate, periodDays); bi >= 0 && ei >= 0 {
			if b, err1 := parsePrice((*res.Data)[bi].Close); err1 == nil && b > 0 {
				if e, err2 := parsePrice((*res.Data)[ei].Close); err2 == nil && e > 0 {
					benchPct = (e - b) / b * 100
				}
			}
		}
	}

	excessPct := returnPct - benchPct
	outcome := "lose"
	if returnPct >= 0 {
		outcome = "win"
	}

	// 推荐价优先取建议买入价下限，否则用基准日收盘
	recPrice := r.RecommendBuyPriceMin
	if recPrice <= 0 {
		recPrice = baseClose
	}

	bt := models.AiRecommendBacktest{
		RecommendID:    r.ID,
		StockCode:      r.StockCode,
		StockName:      r.StockName,
		Rating:         r.Rating,
		PeriodDays:     periodDays,
		RecommendTime:  *r.DataTime,
		RecommendPrice: recPrice,
		EndPrice:       endClose,
		ReturnPct:      round2(returnPct),
		BenchmarkPct:   round2(benchPct),
		ExcessPct:      round2(excessPct),
		Outcome:        outcome,
		ModelName:      r.ModelName,
		SystemPrompt:   r.SystemPrompt,
		UserPrompt:     r.UserPrompt,
	}
	if err := db.Dao.Create(&bt).Error; err != nil {
		return false, fmt.Errorf("写入回测结果失败: %w", err)
	}
	return outcome == "win", nil
}

// findBacktestRange 在日 K 列表中找到推荐日（或之后首个交易日）的基准下标与
// periodDays 个交易日之后的下标。返回 (-1,-1) 表示数据不足。
func findBacktestRange(bars []data.KLineData, recDate time.Time, periodDays int) (baseIdx, endIdx int) {
	baseIdx = -1
	endIdx = -1
	for i := range bars {
		d, err := parseKLineDay(bars[i].Day)
		if err != nil {
			continue
		}
		if !d.Before(recDate) {
			baseIdx = i
			break
		}
	}
	if baseIdx < 0 {
		return -1, -1
	}
	endIdx = baseIdx + periodDays
	if endIdx >= len(bars) {
		return -1, -1
	}
	return baseIdx, endIdx
}

// parseKLineDay 解析 K 线日期（兼容 "2006-01-02" 与 "20060102"）。
func parseKLineDay(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation("20060102", s, time.Local)
}

// parsePrice 解析价格字符串为 float64。
func parsePrice(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("空价格")
	}
	return strconv.ParseFloat(s, 64)
}

// round2 保留两位小数。
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// BacktestItem 回测列表条目（含格式化时间）。
type BacktestItem struct {
	models.AiRecommendBacktest
	RecommendTimeStr string `json:"recommendTimeStr"`
}

// BacktestPageData 回测明细分页结果。
type BacktestPageData struct {
	List  []BacktestItem `json:"list"`
	Total int64          `json:"total"`
}

// ListBacktest 分页查询回测结果（按推荐时间倒序）。
func (a *RecommendBacktestApi) ListBacktest(page, pageSize int) (BacktestPageData, error) {
	return a.listBacktest(page, pageSize, "", "")
}

// ListBacktestByPrompt 按提示词过滤回测明细。promptType 为 "sys"/"usr"，
// 分别按 SystemPrompt/UserPrompt 精确匹配；prompt 为空或 promptType 非法时等同 ListBacktest。
func (a *RecommendBacktestApi) ListBacktestByPrompt(page, pageSize int, prompt, promptType string) (BacktestPageData, error) {
	return a.listBacktest(page, pageSize, prompt, promptType)
}

// listBacktest 分页查询回测结果（按推荐时间倒序），支持按提示词精确过滤。
func (a *RecommendBacktestApi) listBacktest(page, pageSize int, prompt, promptType string) (BacktestPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.AiRecommendBacktest{})
	prompt = strings.TrimSpace(prompt)
	// 依据提示词类型拼装过滤条件（仅当两者都合法时生效）
	if prompt != "" {
		switch promptType {
		case "sys":
			q = q.Where("system_prompt = ?", prompt)
		case "usr":
			q = q.Where("user_prompt = ?", prompt)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return BacktestPageData{}, err
	}
	var list []models.AiRecommendBacktest
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("recommend_time desc").Find(&list).Error; err != nil {
		return BacktestPageData{}, err
	}
	items := make([]BacktestItem, 0, len(list))
	for _, b := range list {
		items = append(items, BacktestItem{
			AiRecommendBacktest: b,
			RecommendTimeStr:    b.RecommendTime.Format("2006-01-02 15:04"),
		})
	}
	return BacktestPageData{List: items, Total: total}, nil
}

// BacktestStats 回测聚合统计。
type BacktestStats struct {
	Total   int     `json:"total"`   // 已回测总数
	Win     int     `json:"win"`     // 正收益数
	Lose    int     `json:"lose"`    // 负收益数
	WinRate float64 `json:"winRate"` // 胜率（%）
	// 按评级分组的胜率
	ByRating map[string]*RatingStat `json:"byRating"`
	// 按模型分组的胜率与收益率
	ByModel []*GroupStat `json:"byModel"`
	// 按系统/用户提示词分组的胜率与收益率
	BySystemPrompt []*GroupStat `json:"bySystemPrompt"`
	ByUserPrompt   []*GroupStat `json:"byUserPrompt"`
	// 达标率最高的模型与提示词
	BestModel        *GroupStat `json:"bestModel"`
	BestSystemPrompt *GroupStat `json:"bestSystemPrompt"`
	BestUserPrompt   *GroupStat `json:"bestUserPrompt"`
}

// RatingStat 单评级统计。
type RatingStat struct {
	Total   int     `json:"total"`
	Win     int     `json:"win"`
	WinRate float64 `json:"winRate"`
}

// GroupStat 分组统计（按模型 / 提示词）。
type GroupStat struct {
	Name      string  `json:"name"`    // 展示名（模型名或提示词截断标签）
	Content   string  `json:"content"` // 完整内容（提示词全文本，模型时为模型名）
	Total     int     `json:"total"`
	Win       int     `json:"win"`
	WinRate   float64 `json:"winRate"`
	AvgReturn float64 `json:"avgReturn"` // 平均个股收益率（%）
	AvgExcess float64 `json:"avgExcess"` // 平均超额收益（%）
}

// groupAcc 分组累加器。
type groupAcc struct {
	name      string
	content   string
	total     int
	win       int
	sumRet    float64
	sumExcess float64
}

// promptLabel 生成提示词的截断展示标签。
func promptLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(空)"
	}
	one := strings.Join(strings.Fields(s), " ")
	r := []rune(one)
	if len(r) > 30 {
		return string(r[:30]) + "…"
	}
	return one
}

// finalizeGroups 将分组累加器转换为按总数倒序（同数按胜率降序）的统计列表。
func finalizeGroups(accs map[string]*groupAcc) []*GroupStat {
	groups := make([]*GroupStat, 0, len(accs))
	for _, g := range accs {
		gs := &GroupStat{Name: g.name, Content: g.content, Total: g.total, Win: g.win}
		if g.total > 0 {
			gs.WinRate = float64(g.win) / float64(g.total) * 100
			gs.AvgReturn = round2(g.sumRet / float64(g.total))
			gs.AvgExcess = round2(g.sumExcess / float64(g.total))
		}
		groups = append(groups, gs)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Total != groups[j].Total {
			return groups[i].Total > groups[j].Total
		}
		return groups[i].WinRate > groups[j].WinRate
	})
	return groups
}

// bestGroup 返回达标率（胜率）最高的分组。
func bestGroup(groups []*GroupStat) *GroupStat {
	var best *GroupStat
	for _, g := range groups {
		if g.Total <= 0 {
			continue
		}
		if best == nil || g.WinRate > best.WinRate {
			best = g
		}
	}
	return best
}

// BacktestStats 返回回测聚合统计。
func (a *RecommendBacktestApi) BacktestStats() (*BacktestStats, error) {
	var list []models.AiRecommendBacktest
	if err := db.Dao.Model(&models.AiRecommendBacktest{}).Find(&list).Error; err != nil {
		return nil, err
	}
	stats := &BacktestStats{ByRating: map[string]*RatingStat{}}
	modelAcc := map[string]*groupAcc{}
	sysAcc := map[string]*groupAcc{}
	usrAcc := map[string]*groupAcc{}

	for _, b := range list {
		stats.Total++
		win := b.Outcome == "win"
		if win {
			stats.Win++
		} else {
			stats.Lose++
		}
		rating := b.Rating
		if rating == "" {
			rating = "未标注"
		}
		rs := stats.ByRating[rating]
		if rs == nil {
			rs = &RatingStat{}
			stats.ByRating[rating] = rs
		}
		rs.Total++
		if win {
			rs.Win++
		}

		// 按模型分组
		m := strings.TrimSpace(b.ModelName)
		if m == "" {
			m = "未记录"
		}
		mg := modelAcc[m]
		if mg == nil {
			mg = &groupAcc{name: m, content: m}
			modelAcc[m] = mg
		}
		mg.total++
		if win {
			mg.win++
		}
		mg.sumRet += b.ReturnPct
		mg.sumExcess += b.ExcessPct

		// 按系统提示词分组
		sysLabel := promptLabel(b.SystemPrompt)
		sg := sysAcc[sysLabel]
		if sg == nil {
			sg = &groupAcc{name: sysLabel, content: b.SystemPrompt}
			sysAcc[sysLabel] = sg
		}
		sg.total++
		if win {
			sg.win++
		}
		sg.sumRet += b.ReturnPct
		sg.sumExcess += b.ExcessPct

		// 按用户提示词分组
		usrLabel := promptLabel(b.UserPrompt)
		ug := usrAcc[usrLabel]
		if ug == nil {
			ug = &groupAcc{name: usrLabel, content: b.UserPrompt}
			usrAcc[usrLabel] = ug
		}
		ug.total++
		if win {
			ug.win++
		}
		ug.sumRet += b.ReturnPct
		ug.sumExcess += b.ExcessPct
	}
	if stats.Total > 0 {
		stats.WinRate = float64(stats.Win) / float64(stats.Total) * 100
	}
	for _, rs := range stats.ByRating {
		if rs.Total > 0 {
			rs.WinRate = float64(rs.Win) / float64(rs.Total) * 100
		}
	}
	stats.ByModel = finalizeGroups(modelAcc)
	stats.BySystemPrompt = finalizeGroups(sysAcc)
	stats.ByUserPrompt = finalizeGroups(usrAcc)
	stats.BestModel = bestGroup(stats.ByModel)
	stats.BestSystemPrompt = bestGroup(stats.BySystemPrompt)
	stats.BestUserPrompt = bestGroup(stats.ByUserPrompt)
	return stats, nil
}
