package agent

// template_backtest.go — 提示词模板回测统计（P4，基于 AI 推荐回测结果）。
//
// 在 ai_recommend_backtest 的基础上按"提示词模板 ID"聚合，评估每个模板在选股/分析
// 场景下的质量与稳定性：
//   - 胜率：绝对胜率（收益>0）+ 超额胜率（超额收益>0，剔除大盘时点偏差）
//   - 收益：平均收益率 / 中位数收益率 / 平均超额收益
//   - 波动率：各次推荐收益率的标准差（%）
//   - 稳定性：变异系数 CV=σ/|μ|（越小越稳定，均值≈0 时标记 -1 无效）
//   - 组合口径：按推荐日等权组合的累计收益净值曲线 + 最大回撤（取样本最多的回测周期）
//   - 综合评分：0-100，见 templateScore 注释
//
// 模板 ID 关联策略：
//   - 新推荐记录：AgentMeta.SysPromptId 在生成时快照（agent_api.go 注入）
//   - 存量记录：写入回测行时若 SysPromptId=0，按"系统提示词快照以模板内容为前缀"反查
//     （系统提示词在 agent 层会在模板内容后追加静态规则/时间上下文等，见 agent_api.go）
//   - 存量回测行：统计/过滤时同样用前缀反查兜底，无需回填写库

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// EquityPoint 等权组合净值曲线点（equity 从 1.0 起步）。
type EquityPoint struct {
	Date     string  `json:"date"`     // 推荐日（yyyy-MM-dd）
	Equity   float64 `json:"equity"`   // 当日组合净值
	DailyPct float64 `json:"dailyPct"` // 当日组合平均收益率（%）
}

// TemplateStat 按提示词模板聚合的回测统计。
type TemplateStat struct {
	TemplateID    int            `json:"templateId"`      // 模板 ID（0=内置/非模板提示词）
	TemplateName  string         `json:"templateName"`    // 模板名（ID=0 时为"内置/非模板提示词"）
	Total         int            `json:"total"`           // 已回测推荐数
	Win           int            `json:"win"`             // 绝对正收益数
	WinRate       float64        `json:"winRate"`         // 绝对胜率（%）
	ExcessWin     int            `json:"excessWin"`       // 超额收益>0 数
	ExcessWinRate float64        `json:"excessWinRate"`   // 超额胜率（%）
	AvgReturn     float64        `json:"avgReturn"`       // 平均收益率（%）
	MedianReturn  float64        `json:"medianReturn"`    // 收益率中位数（%）
	AvgExcess     float64        `json:"avgExcess"`       // 平均超额收益（%）
	Volatility    float64        `json:"volatility"`      // 收益率标准差/波动率（%）
	CV            float64        `json:"cv"`              // 变异系数 σ/|μ|，越小越稳定；均值≈0 时为 -1（无效）
	Sharpe        float64        `json:"sharpe"`          // 简版夏普：平均超额收益/波动率
	MaxDrawdown   float64        `json:"maxDrawdown"`     // 等权组合最大回撤（%，≤0）
	CumReturn     float64        `json:"cumReturn"`       // 等权组合累计收益（%）
	Score         float64        `json:"score"`           // 综合评分 0-100（见 templateScore）
	PeriodDays    int            `json:"periodDays"`      // 主回测周期（样本最多的周期，净值曲线按它构建）
	SampleCount   int            `json:"sampleCount"`     // 主周期样本数
	FirstTime     string         `json:"firstTime"`       // 最早推荐时间
	LastTime      string         `json:"lastTime"`        // 最近推荐时间
	Curve         []*EquityPoint `json:"curve,omitempty"` // 净值曲线（仅详情接口返回）
}

// templateScore 综合评分（0-100）：
//
//	胜率分 40 分：超额胜率 * 40 / 100
//	收益分 30 分：30 * (tanh(平均超额收益/10) + 1) / 2 —— 超额 ±10% 对应 6.5~23.5 分，±30% 接近满/零分
//	稳定分 30 分：30 * (1 - min(CV, 1)) —— CV≥1 得 0 分，CV=0 得满分；CV 无效（-1）按 0.5 计 15 分
//
// 样本量小时评分仅供参考（前端对 total<10 的模板标注"样本不足"）。
func templateScore(excessWinRate, avgExcess, cv float64) float64 {
	retScore := 30 * (math.Tanh(avgExcess/10) + 1) / 2
	if cv < 0 {
		cv = 0.5 // 均值≈0 时 CV 无效，按中性 0.5 计
	}
	stabScore := 30 * (1 - math.Min(cv, 1))
	return round1(excessWinRate/100*40 + retScore + stabScore)
}

// ---- 提示词模板列表缓存（反查 ID 用，5 分钟 TTL；与 sysprompt_cache.go 的按 ID 缓存互补） ----

var promptTemplateListCache struct {
	sync.Mutex
	loadedAt time.Time
	items    []models.PromptTemplate
}

// loadPromptTemplatesCached 加载全部提示词模板（带 5 分钟 TTL 缓存，查询失败退回旧缓存）。
func loadPromptTemplatesCached() []models.PromptTemplate {
	promptTemplateListCache.Lock()
	defer promptTemplateListCache.Unlock()
	if promptTemplateListCache.items != nil && time.Since(promptTemplateListCache.loadedAt) < 5*time.Minute {
		return promptTemplateListCache.items
	}
	if db.Dao == nil {
		return promptTemplateListCache.items // DB 未初始化（如单测环境）
	}
	var items []models.PromptTemplate
	if err := db.Dao.Model(&models.PromptTemplate{}).Find(&items).Error; err != nil {
		return promptTemplateListCache.items // 查询失败时返回旧缓存（可能为 nil）
	}
	promptTemplateListCache.items = items
	promptTemplateListCache.loadedAt = time.Now()
	return items
}

// matchPromptTemplateID 依据系统提示词快照反查模板 ID（存量数据兜底）。
// 系统提示词在 agent 层 = 模板内容 + 静态规则 + 时间上下文 + …，故用"模板内容为前缀"匹配；
// 多个模板同时命中时取内容最长者（最精确）。未命中返回 0。
func matchPromptTemplateID(systemPrompt string) int {
	s := strings.TrimSpace(systemPrompt)
	if s == "" {
		return 0
	}
	bestID, bestLen := 0, 0
	for _, t := range loadPromptTemplatesCached() {
		c := strings.TrimSpace(t.Content)
		if c != "" && strings.HasPrefix(s, c) && len(c) > bestLen {
			bestID, bestLen = t.ID, len(c)
		}
	}
	return bestID
}

// promptTemplateNameMap 返回模板 ID → 名称映射（含缓存）。
func promptTemplateNameMap() map[int]string {
	m := map[int]string{}
	for _, t := range loadPromptTemplatesCached() {
		m[t.ID] = t.Name
	}
	return m
}

// ---- 统计计算 ----

// computeTemplateStats 把回测行按（解析后的）模板 ID 聚合成 TemplateStat 列表（不含净值曲线）。
// 行的 SysPromptId=0 时用前缀反查兜底（覆盖存量数据）。withCurve 控制是否构建净值曲线。
func computeTemplateStats(rows []models.AiRecommendBacktest, withCurve bool) []*TemplateStat {
	nameMap := promptTemplateNameMap()

	type tmplAcc struct {
		rows        []models.AiRecommendBacktest
		periodCnt   map[int]int // 周期 → 样本数
		first, last time.Time
	}
	accs := map[int]*tmplAcc{}
	for _, r := range rows {
		// SysPromptId=0 时前缀反查兜底；仍为 0（内置默认/用户自输提示词）归入 0 组
		id := r.SysPromptId
		if id == 0 {
			id = matchPromptTemplateID(r.SystemPrompt)
		}
		acc := accs[id]
		if acc == nil {
			acc = &tmplAcc{periodCnt: map[int]int{}}
			accs[id] = acc
		}
		acc.rows = append(acc.rows, r)
		acc.periodCnt[r.PeriodDays]++
		if acc.first.IsZero() || r.RecommendTime.Before(acc.first) {
			acc.first = r.RecommendTime
		}
		if r.RecommendTime.After(acc.last) {
			acc.last = r.RecommendTime
		}
	}

	stats := make([]*TemplateStat, 0, len(accs))
	for id, acc := range accs {
		st := &TemplateStat{
			TemplateID: id,
			Total:      len(acc.rows),
			FirstTime:  acc.first.Format("2006-01-02"),
			LastTime:   acc.last.Format("2006-01-02"),
		}
		if id == 0 {
			st.TemplateName = "内置/非模板提示词"
		} else {
			st.TemplateName = nameMap[id]
			if st.TemplateName == "" {
				st.TemplateName = "已删除模板#" + strconv.Itoa(id)
			}
		}

		returns := make([]float64, 0, len(acc.rows))
		sumRet, sumExcess := 0.0, 0.0
		for _, r := range acc.rows {
			returns = append(returns, r.ReturnPct)
			sumRet += r.ReturnPct
			sumExcess += r.ExcessPct
			if r.ReturnPct > 0 {
				st.Win++
			}
			if r.ExcessPct > 0 {
				st.ExcessWin++
			}
		}
		n := float64(len(acc.rows))
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
			if len(returns) >= 2 && math.Abs(avgRet) > 0.005 { // 单样本/均值≈0 时 CV 无意义
				st.CV = round2(st.Volatility / math.Abs(avgRet))
			} else {
				st.CV = -1
			}
		} else if len(returns) < 2 {
			st.CV = -1
		}
		st.Score = templateScore(st.ExcessWinRate, avgExcess, st.CV)

		// 主周期（样本最多）用于净值曲线与组合指标
		st.PeriodDays, st.SampleCount = dominantPeriod(acc.periodCnt)
		curve := buildEquityCurve(acc.rows, st.PeriodDays)
		if len(curve) > 0 {
			last := curve[len(curve)-1]
			st.CumReturn = round2((last.Equity - 1) * 100)
			st.MaxDrawdown = round2(maxDrawdownPct(curve) * 100)
			if withCurve {
				st.Curve = curve
			}
		}
		stats = append(stats, st)
	}
	// 评分降序，同分按样本数降序
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Score != stats[j].Score {
			return stats[i].Score > stats[j].Score
		}
		return stats[i].Total > stats[j].Total
	})
	return stats
}

// dominantPeriod 返回样本数最多的（周期, 样本数）。
func dominantPeriod(cnt map[int]int) (int, int) {
	best, bestCnt := 0, 0
	for p, c := range cnt {
		if c > bestCnt || (c == bestCnt && p < best) {
			best, bestCnt = p, c
		}
	}
	return best, bestCnt
}

// buildEquityCurve 按推荐日等权组合构建净值曲线：同一推荐日的多条推荐等权平均为
// 当日组合收益率，逐日复合。仅取 periodDays == 主周期 的行，避免混合周期失真。
func buildEquityCurve(rows []models.AiRecommendBacktest, periodDays int) []*EquityPoint {
	daySum := map[string][2]float64{} // 日期 → {收益和, 条数}（同日多次推荐等权）
	var days []string
	for _, r := range rows {
		if r.PeriodDays != periodDays {
			continue
		}
		d := r.RecommendTime.Format("2006-01-02")
		s, ok := daySum[d]
		if !ok {
			days = append(days, d)
		}
		daySum[d] = [2]float64{s[0] + r.ReturnPct, s[1] + 1}
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

// maxDrawdownPct 返回净值曲线最大回撤（小数形式，≤0）。
func maxDrawdownPct(curve []*EquityPoint) float64 {
	peak, maxDD := 0.0, 0.0
	for _, p := range curve {
		if p.Equity > peak {
			peak = p.Equity
		}
		if peak > 0 {
			dd := p.Equity/peak - 1
			if dd < maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// median 返回中位数（入参非空）。
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return (s[m-1] + s[m]) / 2
}

// stddev 返回样本标准差（ddof=1；单样本返回 0）。
func stddev(vals []float64, mean float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += (v - mean) * (v - mean)
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

// round4 保留四位小数（净值用）。
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// round1 保留一位小数（评分用）。
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// ---- 对外 API（Wails 绑定） ----

// TemplateBacktestStats 返回全部有回测数据的模板统计（不含净值曲线，按评分降序）。
func (a *RecommendBacktestApi) TemplateBacktestStats() ([]*TemplateStat, error) {
	var rows []models.AiRecommendBacktest
	if err := db.Dao.Model(&models.AiRecommendBacktest{}).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*TemplateStat{}, nil
	}
	return computeTemplateStats(rows, false), nil
}

// TemplateBacktestDetail 返回单个模板的回测统计（含净值曲线）；无数据时 Total=0。
func (a *RecommendBacktestApi) TemplateBacktestDetail(templateId int) (*TemplateStat, error) {
	if templateId < 0 {
		templateId = 0
	}
	rows, err := loadBacktestRowsByTemplate(templateId)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &TemplateStat{
			TemplateID:   templateId,
			TemplateName: promptTemplateNameMap()[templateId],
			CV:           -1,
		}, nil
	}
	stats := computeTemplateStats(rows, true)
	for _, st := range stats {
		if st.TemplateID == templateId {
			return st, nil
		}
	}
	// 模板行全部经前缀反查归入了 0 组（理论上不会走到这里）
	if len(stats) > 0 {
		return stats[0], nil
	}
	return &TemplateStat{TemplateID: templateId, CV: -1}, nil
}

// loadBacktestRowsByTemplate 加载某模板的全部回测行：优先按 sys_prompt_id 匹配；
// 兼容存量 sys_prompt_id=0 的行——若模板内容是其系统提示词快照的前缀则同样命中
// （substr 精确前缀比较，无 LIKE 通配符误匹配问题）。
func loadBacktestRowsByTemplate(templateId int) ([]models.AiRecommendBacktest, error) {
	q := db.Dao.Model(&models.AiRecommendBacktest{})
	if templateId > 0 {
		var tmpl models.PromptTemplate
		if err := db.Dao.Model(&models.PromptTemplate{}).Where("id = ?", templateId).First(&tmpl).Error; err == nil && strings.TrimSpace(tmpl.Content) != "" {
			c := strings.TrimSpace(tmpl.Content)
			q = q.Where("sys_prompt_id = ? OR (sys_prompt_id = 0 AND substr(system_prompt, 1, length(?)) = ?)", templateId, c, c)
		} else {
			q = q.Where("sys_prompt_id = ?", templateId)
		}
	} else {
		q = q.Where("sys_prompt_id = ?", 0)
	}
	var rows []models.AiRecommendBacktest
	if err := q.Order("recommend_time asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListBacktestByTemplate 按提示词模板 ID 分页查询回测明细（含存量前缀兜底匹配）。
func (a *RecommendBacktestApi) ListBacktestByTemplate(page, pageSize, templateId int) (BacktestPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.AiRecommendBacktest{})
	if templateId > 0 {
		var tmpl models.PromptTemplate
		if err := db.Dao.Model(&models.PromptTemplate{}).Where("id = ?", templateId).First(&tmpl).Error; err == nil && strings.TrimSpace(tmpl.Content) != "" {
			c := strings.TrimSpace(tmpl.Content)
			q = q.Where("sys_prompt_id = ? OR (sys_prompt_id = 0 AND substr(system_prompt, 1, length(?)) = ?)", templateId, c, c)
		} else {
			q = q.Where("sys_prompt_id = ?", templateId)
		}
	} else {
		q = q.Where("sys_prompt_id = ?", 0)
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

// ListBacktestBySkill 按技能 ID（目录名）分页查询回测明细。
// skillId 为空时等同 ListBacktest；精确匹配 skill_id 快照字段（逗号分隔多选时整串匹配）。
func (a *RecommendBacktestApi) ListBacktestBySkill(page, pageSize int, skillId string) (BacktestPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.AiRecommendBacktest{})
	skillId = strings.TrimSpace(skillId)
	if skillId != "" {
		q = q.Where("skill_id = ?", skillId)
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
