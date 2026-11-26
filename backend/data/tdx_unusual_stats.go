package data

// tdx_unusual_stats.go — 通达信主力异动事件统计分析。
//
// 数据来自 tdx_unusual_events 表（盘中「通达信异动通知」轮询落库）。
// 统计口径：
//   - 利好异动：名称含 托盘/买入/拉升/反弹/涨停/买盘/回升/高开/向上缺口/新高 等关键词；
//   - 利空异动：名称含 压盘/卖出/下跌/跳水/跌停/卖盘/回落/低开/向下缺口/新低 等关键词；
//   - 其余为中性异动。
//
// 该服务同时供两处复用：App 绑定（实时看板页面轮询）与 AI 工具（agent 多维度分析）。

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

var tdxUnusualBullKeywords = []string{
	"托盘", "买入", "买盘", "拉升", "火箭", "反弹", "封涨停", "涨停", "打开跌停",
	"回升", "高开", "向上缺口", "新高", "上涨", "抢筹", "净流入",
}

var tdxUnusualBearKeywords = []string{
	"压盘", "卖出", "卖盘", "下跌", "跳水", "封跌停", "跌停", "打开涨停",
	"回落", "低开", "向下缺口", "新低", "砸盘", "净流出", "撤单",
}

// tdxTdxShanghaiTimezone 上海时区（东八区），与主程序口径一致。
// TdxShanghaiTimezone 上海时区（东八区），与主程序口径一致；供 agent 工具包复用。
var TdxShanghaiTimezone = time.FixedZone("CST", 8*60*60)

// TdxUnusualTrend 判断异动倾向：1=利好，-1=利空，0=中性。
func TdxUnusualTrend(typeName string) int {
	name := strings.TrimSpace(typeName)
	if name == "" {
		return 0
	}
	for _, kw := range tdxUnusualBullKeywords {
		if strings.Contains(name, kw) {
			return 1
		}
	}
	for _, kw := range tdxUnusualBearKeywords {
		if strings.Contains(name, kw) {
			return -1
		}
	}
	return 0
}

// TdxUnusualDashboard 实时看板/统计分析汇总结果。
type TdxUnusualDashboard struct {
	Today      map[string]any   `json:"today"`      // 今日汇总：total/bull/bear/neutral/activeStocks
	Trend      []map[string]any `json:"trend"`      // 近 N 日逐日趋势
	HotStocks  []map[string]any `json:"hotStocks"`  // 近 N 日异动次数 Top N（含连续异动天数）
	TopTypes   []map[string]any `json:"topTypes"`   // 近 N 日异动类型分布 Top N
	TimeHeat   []map[string]any `json:"timeHeat"`   // 今日分时热度（按 30 分钟桶）
	NewActive  []map[string]any `json:"newActive"`  // 今日首次异动的股票
	MarketDist []map[string]any `json:"marketDist"` // 近 N 日市场分布（沪深北）
	UpdatedAt  string           `json:"updatedAt"`
}

// GetTdxUnusualDashboard 汇总近 N 日（含今日）异动统计。
func GetTdxUnusualDashboard(days, hotTopN int) *TdxUnusualDashboard {
	if days <= 0 {
		days = 10
	}
	if hotTopN <= 0 {
		hotTopN = 30
	}
	today := time.Now().In(TdxShanghaiTimezone).Format("2006-01-02")
	startDay := time.Now().In(TdxShanghaiTimezone).AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	d := &TdxUnusualDashboard{UpdatedAt: time.Now().In(TdxShanghaiTimezone).Format("2006-01-02 15:04:05")}

	// —— 今日汇总 ——
	type todayAgg struct {
		Total   int64
		Bull    int64
		Bear    int64
		Neutral int64
		Stocks  int64
	}
	var t todayAgg
	// 异动类型关键词组合多变，SQL 侧难穷举利好/利空，按类型分桶取回后在 Go 侧归类
	rows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("type_name, count(*) as cnt, count(distinct stock_code) as stocks").
		Where("change_date = ?", today).
		Group("type_name").Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var typeName string
			var cnt, stocks int64
			_ = rows.Scan(&typeName, &cnt, &stocks)
			t.Total += cnt
			t.Stocks += stocks
			switch TdxUnusualTrend(typeName) {
			case 1:
				t.Bull += cnt
			case -1:
				t.Bear += cnt
			default:
				t.Neutral += cnt
			}
		}
	}
	d.Today = map[string]any{
		"date":         today,
		"total":        t.Total,
		"bull":         t.Bull,
		"bear":         t.Bear,
		"neutral":      t.Neutral,
		"activeStocks": t.Stocks,
	}

	// —— 近 N 日逐日趋势 ——
	dayRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("change_date, type_name, count(*) as cnt").
		Where("change_date >= ?", startDay).
		Group("change_date, type_name").Rows()
	if err == nil {
		defer dayRows.Close()
		type dayAgg struct {
			total, bull, bear int64
		}
		byDay := map[string]*dayAgg{}
		order := []string{}
		for dayRows.Next() {
			var day, typeName string
			var cnt int64
			_ = dayRows.Scan(&day, &typeName, &cnt)
			agg, ok := byDay[day]
			if !ok {
				agg = &dayAgg{}
				byDay[day] = agg
				order = append(order, day)
			}
			agg.total += cnt
			switch TdxUnusualTrend(typeName) {
			case 1:
				agg.bull += cnt
			case -1:
				agg.bear += cnt
			}
		}
		sort.Strings(order)
		for _, day := range order {
			agg := byDay[day]
			d.Trend = append(d.Trend, map[string]any{
				"date": day, "total": agg.total, "bull": agg.bull, "bear": agg.bear,
			})
		}
	}

	// —— 近 N 日活跃股 Top N（异动次数排行 + 连续异动天数 + 主要异动类型）——
	hotRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("stock_code, max(stock_name) as stock_name, count(*) as cnt, "+
			"min(change_date) as first_date, max(change_date || ' ' || change_time) as last_seen").
		Where("change_date >= ?", startDay).
		Group("stock_code").Order("cnt desc").Limit(hotTopN).Rows()
	if err == nil {
		defer hotRows.Close()
		type hotAgg struct {
			code, name string
			cnt        int64
			firstDate  string
			lastSeen   string
			days       map[string]bool
			types      map[string]int64
		}
		hots := []*hotAgg{}
		hotMap := map[string]*hotAgg{}
		for hotRows.Next() {
			var code, name, firstDate, lastSeen string
			var cnt int64
			_ = hotRows.Scan(&code, &name, &cnt, &firstDate, &lastSeen)
			h := &hotAgg{code: code, name: name, cnt: cnt, firstDate: firstDate, lastSeen: lastSeen,
				days: map[string]bool{}, types: map[string]int64{}}
			hots = append(hots, h)
			hotMap[code] = h
		}
		// 每只活跃股的异动日期集合与类型分布
		if len(hotMap) > 0 {
			codes := make([]string, 0, len(hotMap))
			for c := range hotMap {
				codes = append(codes, c)
			}
			detailRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
				Select("stock_code, change_date, type_name").
				Where("stock_code in ?", codes).
				Where("change_date >= ?", startDay).
				Rows()
			if err == nil {
				defer detailRows.Close()
				for detailRows.Next() {
					var code, day, typeName string
					_ = detailRows.Scan(&code, &day, &typeName)
					if h := hotMap[code]; h != nil {
						h.days[day] = true
						h.types[typeName]++
					}
				}
			}
		}
		for _, h := range hots {
			mainTypes := topKeys(h.types, 3)
			d.HotStocks = append(d.HotStocks, map[string]any{
				"stockCode": h.code,
				"stockName": h.name,
				"count":     h.cnt,
				"days":      len(h.days),
				"streak":    streakDays(h.days, today),
				"firstDate": h.firstDate,
				"lastSeen":  h.lastSeen,
				"mainTypes": strings.Join(mainTypes, "、"),
				"trend":     tdxUnusualTypesTrend(h.types),
			})
		}
	}

	// —— 近 N 日异动类型分布 Top N ——
	typeRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("type_name, count(*) as cnt, count(distinct stock_code) as stocks").
		Where("change_date >= ?", startDay).
		Group("type_name").Order("cnt desc").Limit(20).Rows()
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var typeName string
			var cnt, stocks int64
			_ = typeRows.Scan(&typeName, &cnt, &stocks)
			d.TopTypes = append(d.TopTypes, map[string]any{
				"typeName": typeName,
				"count":    cnt,
				"stocks":   stocks,
				"trend":    TdxUnusualTrend(typeName),
			})
		}
	}

	// —— 今日分时热度（30 分钟桶）——
	heatRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("change_time, count(*) as cnt").
		Where("change_date = ?", today).
		Group("change_time").Rows()
	if err == nil {
		defer heatRows.Close()
		bucket := map[string]int64{}
		var maxBucket int64
		for heatRows.Next() {
			var tm string
			var cnt int64
			_ = heatRows.Scan(&tm, &cnt)
			if len(tm) >= 2 {
				hour := tm[:2]
				minute := "00"
				if len(tm) >= 5 {
					m := tm[3:5]
					if m < "30" {
						minute = "00"
					} else {
						minute = "30"
					}
				}
				key := hour + ":" + minute
				bucket[key] += cnt
				if bucket[key] > maxBucket {
					maxBucket = bucket[key]
				}
			}
		}
		for _, slot := range tradingSlots() {
			if cnt, ok := bucket[slot]; ok {
				d.TimeHeat = append(d.TimeHeat, map[string]any{"slot": slot, "count": cnt})
			} else {
				d.TimeHeat = append(d.TimeHeat, map[string]any{"slot": slot, "count": 0})
			}
		}
	}

	// —— 今日首次异动的股票（此前任何交易日均无异动记录，潜在新题材/新资金关注）——
	type todayStockAgg struct {
		StockCode string
		StockName string
		Count     int64
		FirstSeen string
		Types     string
	}
	var todayStocks []todayStockAgg
	db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("stock_code, max(stock_name) as stock_name, count(*) as count, "+
			"min(change_date || ' ' || change_time) as first_seen, group_concat(distinct type_name) as types").
		Where("change_date = ?", today).
		Group("stock_code").Order("count desc").Limit(200).Scan(&todayStocks)
	// 历史出现过的代码集合
	seenCodes := map[string]bool{}
	seenRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Distinct("stock_code").
		Where("change_date < ?", today).Rows()
	if err == nil {
		defer seenRows.Close()
		for seenRows.Next() {
			var code string
			_ = seenRows.Scan(&code)
			seenCodes[code] = true
		}
	}
	for _, s := range todayStocks {
		if seenCodes[s.StockCode] {
			continue
		}
		d.NewActive = append(d.NewActive, map[string]any{
			"stockCode": s.StockCode,
			"stockName": s.StockName,
			"count":     s.Count,
			"firstSeen": s.FirstSeen,
			"types":     s.Types,
		})
		if len(d.NewActive) >= 30 {
			break
		}
	}

	// —— 近 N 日市场分布 ——
	mktRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Select("market, count(*) as cnt, count(distinct stock_code) as stocks").
		Where("change_date >= ?", startDay).
		Group("market").Rows()
	if err == nil {
		defer mktRows.Close()
		for mktRows.Next() {
			var market int64
			var cnt, stocks int64
			_ = mktRows.Scan(&market, &cnt, &stocks)
			name := "深市"
			switch market {
			case 1:
				name = "沪市"
			case 2:
				name = "北证"
			}
			d.MarketDist = append(d.MarketDist, map[string]any{"market": name, "count": cnt, "stocks": stocks})
		}
	}

	return d
}

// QueryTdxUnusualHotStocks 活跃股排行（近 N 日异动次数 Top N，含连续异动天数）。
func QueryTdxUnusualHotStocks(days, topN int) []map[string]any {
	d := GetTdxUnusualDashboard(days, topN)
	return d.HotStocks
}

// TdxUnusualEventPage 异动明细服务端分页结果。
type TdxUnusualEventPage struct {
	List       []models.TdxUnusualEvent `json:"list"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	TotalPages int                      `json:"totalPages"`
}

// QueryTdxUnusualEventPage 异动明细多条件服务端分页查询
// （代码/名称模糊匹配，typeNames 多选类型精确匹配，市场过滤，日期时间范围 YYYY-MM-DD[ HH:mm:ss]），时间倒序。
func QueryTdxUnusualEventPage(page, pageSize int, stockCode, stockName string, typeNames []string, market, startDate, endDate string) *TdxUnusualEventPage {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.TdxUnusualEvent{})
	if code := strings.TrimSpace(stockCode); code != "" {
		q = q.Where("stock_code LIKE ?", "%"+code+"%")
	}
	if name := strings.TrimSpace(stockName); name != "" {
		q = q.Where("stock_name LIKE ?", "%"+name+"%")
	}
	types := make([]string, 0, len(typeNames))
	for _, tn := range typeNames {
		if t := strings.TrimSpace(tn); t != "" {
			types = append(types, t)
		}
	}
	if len(types) == 1 {
		q = q.Where("type_name = ?", types[0])
	} else if len(types) > 1 {
		q = q.Where("type_name IN ?", types)
	}
	switch strings.ToLower(strings.TrimSpace(market)) {
	case "sh", "沪市":
		q = q.Where("market = ?", 1)
	case "sz", "深市":
		q = q.Where("market = ?", 0)
	case "bj", "北证", "北交所":
		q = q.Where("market = ?", 2)
	}
	// 日期+时间范围（change_date/change_time 分字段存储，做组合比较）
	if sd := strings.TrimSpace(startDate); sd != "" {
		date, tm := splitDateTime(sd, "00:00:00")
		q = q.Where("(change_date > ? OR (change_date = ? AND change_time >= ?))", date, date, tm)
	}
	if ed := strings.TrimSpace(endDate); ed != "" {
		date, tm := splitDateTime(ed, "23:59:59")
		q = q.Where("(change_date < ? OR (change_date = ? AND change_time <= ?))", date, date, tm)
	}

	result := &TdxUnusualEventPage{Page: page, PageSize: pageSize}
	if err := q.Count(&result.Total).Error; err != nil {
		logger.SugaredLogger.Warnf("通达信异动明细查询失败: %v", err)
		return result
	}
	result.TotalPages = int(result.Total) / pageSize
	if int(result.Total)%pageSize > 0 {
		result.TotalPages++
	}
	if err := q.Order("change_date desc, change_time desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&result.List).Error; err != nil {
		logger.SugaredLogger.Warnf("通达信异动明细查询失败: %v", err)
	}
	return result
}

// splitDateTime 拆分 "YYYY-MM-DD HH:mm:ss"（兼容仅有日期），时间缺省用 fallback。
func splitDateTime(s, fallback string) (string, string) {
	s = strings.TrimSpace(s)
	date, tm := s, fallback
	if i := strings.IndexAny(s, " T"); i > 0 {
		date = s[:i]
		if t := strings.TrimSpace(s[i+1:]); t != "" {
			tm = t
		}
	}
	return date, tm
}

// QueryTdxUnusualTypeOptions 异动类型下拉选项（全部去重类型名）。
func QueryTdxUnusualTypeOptions() []string {
	var names []string
	if err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Distinct().Order("type_name").
		Pluck("type_name", &names).Error; err != nil {
		logger.SugaredLogger.Warnf("通达信异动类型选项查询失败: %v", err)
		return []string{}
	}
	return names
}

// QueryTdxUnusualTypeLeaders 异动类型 → 领涨/领跌股票：每种类型下异动次数最多的股票。
func QueryTdxUnusualTypeLeaders(days, topN int) []map[string]any {
	if days <= 0 {
		days = 5
	}
	startDay := time.Now().In(TdxShanghaiTimezone).AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	// 先取近 N 日 Top 类型
	var types []string
	typeRows, err := db.Dao.Model(&models.TdxUnusualEvent{}).
		Distinct("type_name").
		Where("change_date >= ?", startDay).
		Order("type_name").Rows()
	if err != nil {
		return nil
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var name string
		_ = typeRows.Scan(&name)
		types = append(types, name)
	}
	result := make([]map[string]any, 0, len(types))
	for _, tn := range types {
		var cnt int64
		db.Dao.Model(&models.TdxUnusualEvent{}).
			Where("type_name = ? and change_date >= ?", tn, startDay).Count(&cnt)
		var leaders []struct {
			StockCode string
			StockName string
			Cnt       int64
		}
		db.Dao.Model(&models.TdxUnusualEvent{}).
			Select("stock_code, max(stock_name) as stock_name, count(*) as cnt").
			Where("type_name = ? and change_date >= ?", tn, startDay).
			Group("stock_code").Order("cnt desc").Limit(5).Scan(&leaders)
		names := make([]string, 0, len(leaders))
		for _, l := range leaders {
			names = append(names, fmt.Sprintf("%s(%s) %d次", l.StockName, l.StockCode, l.Cnt))
		}
		result = append(result, map[string]any{
			"typeName": tn,
			"count":    cnt,
			"trend":    TdxUnusualTrend(tn),
			"leaders":  strings.Join(names, "、"),
		})
	}
	// 按次数降序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j]["count"].(int64) > result[i]["count"].(int64) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if topN > 0 && len(result) > topN {
		result = result[:topN]
	}
	return result
}

// streakDays 连续异动天数：从 today 往前数，日期集合连续覆盖则累加。
func streakDays(days map[string]bool, today string) int {
	streak := 0
	cur, _ := time.ParseInLocation("2006-01-02", today, TdxShanghaiTimezone)
	for i := 0; i < 60; i++ {
		key := cur.Format("2006-01-02")
		if days[key] {
			streak++
		} else if i > 0 {
			break
		}
		cur = cur.AddDate(0, 0, -1)
	}
	return streak
}

// topKeys 取出现次数最多的前 n 个 key。
func topKeys(m map[string]int64, n int) []string {
	type kv struct {
		k string
		v int64
	}
	list := make([]kv, 0, len(m))
	for k, v := range m {
		list = append(list, kv{k, v})
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].v > list[i].v {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	if len(list) > n {
		list = list[:n]
	}
	keys := make([]string, 0, len(list))
	for _, item := range list {
		keys = append(keys, item.k)
	}
	return keys
}

// tdxUnusualTypesTrend 一组异动类型的整体倾向：1=偏利好，-1=偏利空，0=中性。
func tdxUnusualTypesTrend(types map[string]int64) int {
	score := int64(0)
	for name, cnt := range types {
		score += int64(TdxUnusualTrend(name)) * cnt
	}
	if score > 0 {
		return 1
	}
	if score < 0 {
		return -1
	}
	return 0
}

// tradingSlots A股交易时段按 30 分钟划分的槽位。
func tradingSlots() []string {
	slots := []string{}
	for _, hour := range []string{"09", "10", "11", "13", "14", "15"} {
		for _, minute := range []string{"00", "30"} {
			slot := hour + ":" + minute
			if hour == "09" && minute == "00" {
				continue // 09:30 前无盘中异动
			}
			if hour == "11" && minute == "30" {
				continue // 午休
			}
			if hour == "13" && minute == "00" {
				continue // 午休
			}
			if hour == "15" && minute == "30" {
				continue
			}
			slots = append(slots, slot)
		}
	}
	return slots
}
