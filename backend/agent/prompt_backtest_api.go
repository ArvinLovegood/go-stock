package agent

// prompt_backtest_api.go — 提示词模板主动回测 API（Wails 绑定层）。

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// PromptBacktestApi 提示词模板主动回测 API。
type PromptBacktestApi struct{}

func NewPromptBacktestApi() *PromptBacktestApi {
	return &PromptBacktestApi{}
}

// PromptBacktestCreateParams 创建任务参数。
type PromptBacktestCreateParams struct {
	Name             string `json:"name"`
	TemplateIds      string `json:"templateIds"` // 逗号分隔模板 ID
	AiConfigId       int    `json:"aiConfigId"`
	StartDate        string `json:"startDate"`
	EndDate          string `json:"endDate"`
	PeriodDays       int    `json:"periodDays"`
	TopN             int    `json:"topN"`
	RepeatRuns       int    `json:"repeatRuns"`
	SampleEveryNDays int    `json:"sampleEveryNDays"`
}

// PromptBacktestTemplateStat 按模板聚合的主动回测统计（含 Jaccard 输出稳定性）。
type PromptBacktestTemplateStat struct {
	TemplateID    int            `json:"templateId"`
	TemplateName  string         `json:"templateName"`
	Total         int            `json:"total"`   // 选股条数（已计算收益）
	Skipped       int            `json:"skipped"` // 数据不足跳过条数
	Win           int            `json:"win"`
	WinRate       float64        `json:"winRate"` // 绝对胜率（%）
	ExcessWin     int            `json:"excessWin"`
	ExcessWinRate float64        `json:"excessWinRate"` // 超额胜率（%）
	AvgReturn     float64        `json:"avgReturn"`
	MedianReturn  float64        `json:"medianReturn"`
	AvgExcess     float64        `json:"avgExcess"`
	Volatility    float64        `json:"volatility"`
	CV            float64        `json:"cv"`
	Sharpe        float64        `json:"sharpe"`
	MaxDrawdown   float64        `json:"maxDrawdown"`
	CumReturn     float64        `json:"cumReturn"`
	Score         float64        `json:"score"`
	Jaccard       float64        `json:"jaccard"`   // 同日多跑选股重合度（0~1，越高越稳定；-1=无重复跑）
	CallsDone     int            `json:"callsDone"` // 该模板已完成 AI 调用次数（含空选股）
	Curve         []*EquityPoint `json:"curve,omitempty"`
}

// PromptBacktestTaskDetail 任务详情（任务 + 模板统计 + 选股明细分页）。
type PromptBacktestTaskDetail struct {
	Task  *models.PromptBacktestTask    `json:"task"`
	Stats []*PromptBacktestTemplateStat `json:"stats"`
}

// CreatePromptBacktestTask 创建并启动回测任务（异步执行，进度经 promptBacktestProgress 事件推送）。
func (a *PromptBacktestApi) CreatePromptBacktestTask(ctx context.Context, params PromptBacktestCreateParams) (*models.PromptBacktestTask, error) {
	// 参数规范化
	ids := parseTemplateIds(params.TemplateIds)
	if len(ids) == 0 {
		return nil, fmt.Errorf("请至少选择 1 个提示词模板")
	}
	if len(ids) > promptBacktestMaxTemplates {
		return nil, fmt.Errorf("单任务最多对比 %d 个模板", promptBacktestMaxTemplates)
	}
	if params.PeriodDays <= 0 {
		params.PeriodDays = 5
	}
	if params.PeriodDays > 60 {
		params.PeriodDays = 60
	}
	if params.TopN <= 0 {
		params.TopN = 5
	}
	if params.TopN > 20 {
		params.TopN = 20
	}
	if params.RepeatRuns < 1 {
		params.RepeatRuns = 1
	}
	if params.RepeatRuns > 3 {
		params.RepeatRuns = 3
	}
	if params.SampleEveryNDays < 1 {
		params.SampleEveryNDays = 1
	}
	if params.StartDate == "" || params.EndDate == "" || params.StartDate > params.EndDate {
		return nil, fmt.Errorf("日期区间无效")
	}
	// 校验模板存在
	var cnt int64
	db.Dao.Model(&models.PromptTemplate{}).Where("id IN ?", ids).Count(&cnt)
	if int(cnt) != len(ids) {
		return nil, fmt.Errorf("部分提示词模板不存在或已删除")
	}
	if params.Name == "" {
		var names []string
		var tmpls []models.PromptTemplate
		db.Dao.Model(&models.PromptTemplate{}).Where("id IN ?", ids).Find(&tmpls)
		for _, t := range tmpls {
			names = append(names, t.Name)
		}
		params.Name = strings.Join(names, " vs ") + "（" + params.StartDate + "~" + params.EndDate + "）"
	}
	aiConfigId := FirstAiConfigId(params.AiConfigId)

	idStrs := make([]string, len(ids))
	for i, v := range ids {
		idStrs[i] = fmt.Sprintf("%d", v)
	}
	task := models.PromptBacktestTask{
		Name:             params.Name,
		TemplateIds:      strings.Join(idStrs, ","),
		AiConfigId:       aiConfigId,
		StartDate:        params.StartDate,
		EndDate:          params.EndDate,
		PeriodDays:       params.PeriodDays,
		TopN:             params.TopN,
		RepeatRuns:       params.RepeatRuns,
		SampleEveryNDays: params.SampleEveryNDays,
		Status:           "pending",
	}
	if err := db.Dao.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建任务失败：%w", err)
	}
	logger.SugaredLogger.Infof("提示词回测任务已创建：%d（%s）", task.ID, task.Name)
	StartPromptBacktestTask(ctx, task.ID)
	return &task, nil
}

// GetPromptBacktestTaskList 任务列表（按创建时间倒序）。
func (a *PromptBacktestApi) GetPromptBacktestTaskList() ([]*models.PromptBacktestTask, error) {
	var list []*models.PromptBacktestTask
	if err := db.Dao.Model(&models.PromptBacktestTask{}).Order("created_at desc").Limit(100).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// DeletePromptBacktestTask 删除任务及其全部选股记录（运行中的任务不可删）。
func (a *PromptBacktestApi) DeletePromptBacktestTask(taskId uint) error {
	var task models.PromptBacktestTask
	if err := db.Dao.First(&task, taskId).Error; err != nil {
		return fmt.Errorf("任务不存在")
	}
	if task.Status == "running" && time.Since(task.UpdatedAt) < 30*time.Minute {
		return fmt.Errorf("任务正在执行中，无法删除")
	}
	if err := db.Dao.Where("task_id = ?", taskId).Delete(&models.PromptBacktestPick{}).Error; err != nil {
		return err
	}
	return db.Dao.Delete(&models.PromptBacktestTask{}, taskId).Error
}

// GetPromptBacktestTaskDetail 任务详情：任务 + 各模板统计（含净值曲线与 Jaccard 稳定性）。
func (a *PromptBacktestApi) GetPromptBacktestTaskDetail(taskId uint) (*PromptBacktestTaskDetail, error) {
	var task models.PromptBacktestTask
	if err := db.Dao.First(&task, taskId).Error; err != nil {
		return nil, fmt.Errorf("任务不存在")
	}
	var picks []models.PromptBacktestPick
	if err := db.Dao.Where("task_id = ?", taskId).Find(&picks).Error; err != nil {
		return nil, err
	}

	nameMap := promptTemplateNameMap()
	stats := computePromptBacktestStats(task, picks, nameMap)
	return &PromptBacktestTaskDetail{Task: &task, Stats: stats}, nil
}

// PromptBacktestPickPageData 选股明细分页结果。
type PromptBacktestPickPageData struct {
	List  []models.PromptBacktestPick `json:"list"`
	Total int64                       `json:"total"`
}

// GetPromptBacktestPicks 任务选股明细分页（可按模板过滤）。
func (a *PromptBacktestApi) GetPromptBacktestPicks(taskId uint, templateId int, page, pageSize int) (PromptBacktestPickPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.PromptBacktestPick{}).Where("task_id = ?", taskId)
	if templateId > 0 {
		q = q.Where("template_id = ?", templateId)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return PromptBacktestPickPageData{}, err
	}
	var list []models.PromptBacktestPick
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("trade_date desc, template_id asc, run_index asc").Find(&list).Error; err != nil {
		return PromptBacktestPickPageData{}, err
	}
	return PromptBacktestPickPageData{List: list, Total: total}, nil
}

// computePromptBacktestStats 把任务选股按模板聚合统计（含净值曲线与 Jaccard）。
func computePromptBacktestStats(task models.PromptBacktestTask, picks []models.PromptBacktestPick, nameMap map[int]string) []*PromptBacktestTemplateStat {
	byTmpl := map[int][]models.PromptBacktestPick{}
	for _, p := range picks {
		byTmpl[p.TemplateId] = append(byTmpl[p.TemplateId], p)
	}
	stats := make([]*PromptBacktestTemplateStat, 0, len(byTmpl))
	for tid, ps := range byTmpl {
		st := &PromptBacktestTemplateStat{
			TemplateID:   tid,
			TemplateName: nameMap[tid],
			Total:        len(ps),
		}
		if st.TemplateName == "" {
			st.TemplateName = fmt.Sprintf("模板#%d", tid)
		}
		var returns []float64
		sumRet, sumExcess := 0.0, 0.0
		for _, p := range ps {
			returns = append(returns, p.ReturnPct)
			sumRet += p.ReturnPct
			sumExcess += p.ExcessPct
			if p.ReturnPct > 0 {
				st.Win++
			}
			if p.ExcessPct > 0 {
				st.ExcessWin++
			}
		}
		n := float64(len(ps))
		if n > 0 {
			avgRet := sumRet / n
			avgExcess := sumExcess / n
			st.WinRate = round2(float64(st.Win) / n * 100)
			st.ExcessWinRate = round2(float64(st.ExcessWin) / n * 100)
			st.AvgReturn = round2(avgRet)
			st.AvgExcess = round2(avgExcess)
			st.MedianReturn = round2(median(returns))
			st.Volatility = round2(stddev(returns, avgRet))
			if st.Volatility > 0 {
				st.Sharpe = round2(avgExcess / st.Volatility)
				if len(returns) >= 2 && math.Abs(avgRet) > 0.005 {
					st.CV = round2(st.Volatility / math.Abs(avgRet))
				} else {
					st.CV = -1
				}
			} else if len(returns) < 2 {
				st.CV = -1
			}
			st.Score = templateScore(st.ExcessWinRate, avgExcess, st.CV)
		}
		// 净值曲线：任务周期固定，全部选股按日等权复合
		curve := buildPickEquityCurve(ps)
		if len(curve) > 0 {
			last := curve[len(curve)-1]
			st.CumReturn = round2((last.Equity - 1) * 100)
			st.MaxDrawdown = round2(maxDrawdownPct(curve) * 100)
			st.Curve = curve
		}
		// Jaccard 输出稳定性：同日不同 run 的选股集合重合度
		if task.RepeatRuns > 1 {
			st.Jaccard = computePickJaccard(ps)
		} else {
			st.Jaccard = -1
		}
		// 调用次数（含 0 选股的调用）：按 (date, run) 去重
		seen := map[string]bool{}
		for _, p := range ps {
			seen[p.TradeDate+"|"+fmt.Sprint(p.RunIndex)] = true
		}
		st.CallsDone = len(seen)
		stats = append(stats, st)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Score != stats[j].Score {
			return stats[i].Score > stats[j].Score
		}
		return stats[i].Total > stats[j].Total
	})
	return stats
}

// buildPickEquityCurve 按选股日等权组合构建净值曲线（主动回测周期固定，直接用全部选股）。
func buildPickEquityCurve(picks []models.PromptBacktestPick) []*EquityPoint {
	daySum := map[string][2]float64{}
	var days []string
	for _, p := range picks {
		if _, ok := daySum[p.TradeDate]; !ok {
			days = append(days, p.TradeDate)
		}
		s := daySum[p.TradeDate]
		daySum[p.TradeDate] = [2]float64{s[0] + p.ReturnPct, s[1] + 1}
	}
	if len(days) == 0 {
		return nil
	}
	sort.Strings(days)
	curve := make([]*EquityPoint, 0, len(days))
	equity := 1.0
	for _, d := range days {
		s := daySum[d]
		dailyPct := s[0] / s[1]
		equity *= 1 + dailyPct/100
		curve = append(curve, &EquityPoint{Date: d, Equity: round4(equity), DailyPct: round2(dailyPct)})
	}
	return curve
}

// computePickJaccard 同一模板同一交易日的多次调用选股集合的 Jaccard 重合度均值
// （每两两一组，|交集|/|并集|，越高说明模板输出越稳定）。
func computePickJaccard(picks []models.PromptBacktestPick) float64 {
	type key struct {
		date string
		run  int
	}
	sets := map[key]map[string]bool{}
	for _, p := range picks {
		k := key{p.TradeDate, p.RunIndex}
		if sets[k] == nil {
			sets[k] = map[string]bool{}
		}
		sets[k][p.StockCode] = true
	}
	// 按日期分组两两比较
	byDate := map[string][]int{}
	for k := range sets {
		byDate[k.date] = append(byDate[k.date], k.run)
	}
	total, cnt := 0.0, 0.0
	for date, runs := range byDate {
		sort.Ints(runs)
		for i := 0; i < len(runs); i++ {
			for j := i + 1; j < len(runs); j++ {
				a := sets[key{date: date, run: runs[i]}]
				b := sets[key{date: date, run: runs[j]}]
				inter := 0
				for code := range a {
					if b[code] {
						inter++
					}
				}
				union := len(a) + len(b) - inter
				if union > 0 {
					total += float64(inter) / float64(union)
					cnt++
				}
			}
		}
	}
	if cnt == 0 {
		return -1
	}
	return round2(total / cnt)
}
