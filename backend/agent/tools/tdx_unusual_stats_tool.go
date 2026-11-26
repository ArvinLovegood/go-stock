package tools

// tdx_unusual_stats_tool.go — 通达信主力异动事件的统计分析工具集。
//
// 目标：基于本地 tdx_unusual_events 落库数据做「挖掘潜力股 + 判断市场方向」的统计分析：
//   1. AnalyzeTdxUnusualMarketPulse  市场脉搏：逐日异动趋势、利好/利空占比、市场分布、分时热度
//   2. QueryTdxUnusualHotStocks      活跃股排行：近 N 日异动次数 Top、连续异动天数、主要异动类型
//   3. QueryTdxUnusualNewActiveStocks 今日首次异动的股票（潜在新题材/新资金关注）
//   4. QueryTdxUnusualTypeLeaders    异动类型 → 领先股票（每种类型下异动最频繁的个股）
//   5. QueryTdxUnusualStockProfile   个股异动画像：指定股票的异动历史、类型分布、活跃时段

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

// trendLabel 倾向转可读文字。
func trendLabel(trend int) string {
	switch trend {
	case 1:
		return "偏多"
	case -1:
		return "偏空"
	default:
		return "中性"
	}
}

func newTdxUnusualMarketPulseTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"AnalyzeTdxUnusualMarketPulse",
		"分析通达信主力异动反映的市场方向（市场脉搏）。返回近N日逐日异动趋势（总数/利好/利空）、"+
			"今日利好利空占比、异动类型分布Top、沪深北市场分布、日内分时热度。用于判断市场情绪偏多还是偏空、"+
			"资金活跃度变化方向、盘面节奏。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "趋势统计天数，如5/10/20，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 10
			}
			d := data.GetTdxUnusualDashboard(days, 30)
			if d == nil || len(d.Trend) == 0 {
				return "暂无通达信异动统计数据（需在交易时段开启通达信异动通知进行采集）", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("## 通达信异动市场脉搏（近%d日）\n\n", days))

			// 今日概览
			if today := d.Today; today != nil && today["total"].(int64) > 0 {
				sb.WriteString(fmt.Sprintf("### 今日概览（%s）\n\n", today["date"]))
				type todayRow struct {
					Total       int64 `md:"异动总数"`
					Bull        int64 `md:"利好异动"`
					Bear        int64 `md:"利空异动"`
					Neutral     int64 `md:"中性异动"`
					ActiveStock int64 `md:"活跃股票数"`
				}
				bull, _ := today["bull"].(int64)
				bear, _ := today["bear"].(int64)
				neutral, _ := today["neutral"].(int64)
				total, _ := today["total"].(int64)
				active, _ := today["activeStocks"].(int64)
				sb.WriteString(util.MarkdownTableWithTitle("今日异动汇总", []todayRow{{total, bull, bear, neutral, active}}))
				if bull+bear > 0 {
					ratio := float64(bull) / float64(bull+bear) * 100
					sb.WriteString(fmt.Sprintf("\n利好/利空比：%.1f%%（vs 利空）→ 市场%s\n", ratio, trendLabel(map[bool]int{true: 1, false: -1}[bull >= bear])))
				}
			}

			// 逐日趋势
			type trendRow struct {
				Date      string `md:"日期"`
				Total     int64  `md:"异动总数"`
				Bull      int64  `md:"利好异动"`
				Bear      int64  `md:"利空异动"`
				Direction string `md:"当日倾向"`
			}
			var trendRows []trendRow
			for _, item := range d.Trend {
				total, _ := item["total"].(int64)
				bull, _ := item["bull"].(int64)
				bear, _ := item["bear"].(int64)
				direction := "中性"
				if bull > bear {
					direction = "偏多"
				} else if bear > bull {
					direction = "偏空"
				}
				trendRows = append(trendRows, trendRow{
					Date: fmt.Sprintf("%v", item["date"]), Total: total, Bull: bull, Bear: bear, Direction: direction,
				})
			}
			sb.WriteString("\n" + util.MarkdownTableWithTitle("逐日异动趋势", trendRows))

			// 类型分布
			if len(d.TopTypes) > 0 {
				type typeRow struct {
					TypeName string `md:"异动类型"`
					Count    int64  `md:"次数"`
					Stocks   int64  `md:"涉及股票"`
					Trend    string `md:"倾向"`
				}
				var typeRows []typeRow
				for _, item := range d.TopTypes {
					cnt, _ := item["count"].(int64)
					stocks, _ := item["stocks"].(int64)
					trend, _ := item["trend"].(int)
					typeRows = append(typeRows, typeRow{
						TypeName: fmt.Sprintf("%v", item["typeName"]),
						Count:    cnt,
						Stocks:   stocks,
						Trend:    trendLabel(trend),
					})
				}
				sb.WriteString("\n" + util.MarkdownTableWithTitle("异动类型分布 Top", typeRows))
			}

			// 市场分布
			if len(d.MarketDist) > 0 {
				type mktRow struct {
					Market string `md:"市场"`
					Count  int64  `md:"异动次数"`
					Stocks int64  `md:"涉及股票"`
				}
				var mktRows []mktRow
				for _, item := range d.MarketDist {
					cnt, _ := item["count"].(int64)
					stocks, _ := item["stocks"].(int64)
					mktRows = append(mktRows, mktRow{Market: fmt.Sprintf("%v", item["market"]), Count: cnt, Stocks: stocks})
				}
				sb.WriteString("\n" + util.MarkdownTableWithTitle("市场分布", mktRows))
			}

			return sb.String(), nil
		},
	)
}

func newTdxUnusualHotStocksTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"QueryTdxUnusualHotStocks",
		"查询通达信异动活跃股排行（潜力股挖掘核心工具）。按近N日异动次数排行，返回每只股票的"+
			"异动次数、连续异动天数、覆盖天数、主要异动类型、多空倾向、最近一次异动时间。"+
			"连续多日异动且偏多的股票往往代表资金持续关注，是挖掘潜力股的重要线索。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "统计天数，如1/3/5/10，默认5",
				Required: false,
			},
			"topN": {
				Type:     "integer",
				Desc:     "返回前N只，默认20，最大50",
				Required: false,
			},
			"trend": {
				Type:     "string",
				Desc:     "倾向过滤（可选）：bull=仅偏多，bear=仅偏空；不传返回全部",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 5
			}
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 20
			}
			if topN > 50 {
				topN = 50
			}
			trendFilter := strings.ToLower(gjson.Get(args, "trend").String())
			stocks := data.QueryTdxUnusualHotStocks(days, topN)
			if len(stocks) == 0 {
				return "暂无通达信异动活跃股数据（需在交易时段开启通达信异动通知进行采集）", nil
			}
			type hotRow struct {
				Rank      int    `md:"排名"`
				StockCode string `md:"代码"`
				StockName string `md:"名称"`
				Count     int64  `md:"异动次数"`
				Streak    int    `md:"连续天数"`
				Days      int    `md:"覆盖天数"`
				MainTypes string `md:"主要异动类型"`
				Trend     string `md:"倾向"`
				LastSeen  string `md:"最近异动"`
			}
			var rows []hotRow
			rank := 0
			for _, item := range stocks {
				trend, _ := item["trend"].(int)
				if trendFilter == "bull" && trend != 1 {
					continue
				}
				if trendFilter == "bear" && trend != -1 {
					continue
				}
				rank++
				cnt, _ := item["count"].(int64)
				streak, _ := item["streak"].(int)
				cover, _ := item["days"].(int)
				rows = append(rows, hotRow{
					Rank:      rank,
					StockCode: fmt.Sprintf("%v", item["stockCode"]),
					StockName: fmt.Sprintf("%v", item["stockName"]),
					Count:     cnt,
					Streak:    streak,
					Days:      cover,
					MainTypes: fmt.Sprintf("%v", item["mainTypes"]),
					Trend:     trendLabel(trend),
					LastSeen:  fmt.Sprintf("%v", item["lastSeen"]),
				})
				if rank >= topN {
					break
				}
			}
			if len(rows) == 0 {
				return "过滤后无符合条件的活跃股", nil
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("近%d日通达信异动活跃股排行（Top%d）", days, len(rows)), rows), nil
		},
	)
}

func newTdxUnusualNewActiveTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"QueryTdxUnusualNewActiveStocks",
		"查询今日首次出现通达信异动的股票（此前任何交易日均无异动记录）。"+
			"新出现的异动股票往往代表新题材启动、新资金进场，是挖掘潜力股的早期线索。",
		map[string]*schema.ParameterInfo{
			"topN": {
				Type:     "integer",
				Desc:     "返回前N只，默认20，最大30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 20
			}
			if topN > 30 {
				topN = 30
			}
			d := data.GetTdxUnusualDashboard(1, 0)
			if d == nil || len(d.NewActive) == 0 {
				return "今日暂无首次异动的股票", nil
			}
			type newRow struct {
				StockCode string `md:"代码"`
				StockName string `md:"名称"`
				Count     int64  `md:"今日异动次数"`
				FirstSeen string `md:"首次异动时间"`
				Types     string `md:"异动类型"`
			}
			var rows []newRow
			for i, item := range d.NewActive {
				if i >= topN {
					break
				}
				cnt, _ := item["count"].(int64)
				rows = append(rows, newRow{
					StockCode: fmt.Sprintf("%v", item["stockCode"]),
					StockName: fmt.Sprintf("%v", item["stockName"]),
					Count:     cnt,
					FirstSeen: fmt.Sprintf("%v", item["firstSeen"]),
					Types:     fmt.Sprintf("%v", item["types"]),
				})
			}
			return util.MarkdownTableWithTitle("今日首次异动股票（潜在新题材）", rows), nil
		},
	)
}

func newTdxUnusualTypeLeadersTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"QueryTdxUnusualTypeLeaders",
		"按异动类型统计并给出每种类型下异动最频繁的领先股票。例如「主力买入」最多的5只股票、"+
			"「大单托盘」最多的5只股票。用于按异动类型挖掘潜力股（如批量找主力买入集中的股票）。"+
			"也可用于发现当前市场哪种异动类型最活跃。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "统计天数，如1/3/5，默认3",
				Required: false,
			},
			"topN": {
				Type:     "integer",
				Desc:     "返回前N种类型，默认15",
				Required: false,
			},
			"typeName": {
				Type:     "string",
				Desc:     "仅统计指定异动类型（可选，模糊匹配），如 主力买入、大单托盘",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 3
			}
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 15
			}
			typeFilter := strings.TrimSpace(gjson.Get(args, "typeName").String())
			leaders := data.QueryTdxUnusualTypeLeaders(days, 0)
			if len(leaders) == 0 {
				return "暂无通达信异动类型统计数据", nil
			}
			type leaderRow struct {
				TypeName string `md:"异动类型"`
				Count    int64  `md:"总次数"`
				Trend    string `md:"倾向"`
				Leaders  string `md:"领先股票"`
			}
			var rows []leaderRow
			for _, item := range leaders {
				tn := fmt.Sprintf("%v", item["typeName"])
				if typeFilter != "" && !strings.Contains(tn, typeFilter) {
					continue
				}
				cnt, _ := item["count"].(int64)
				trend, _ := item["trend"].(int)
				rows = append(rows, leaderRow{TypeName: tn, Count: cnt, Trend: trendLabel(trend), Leaders: fmt.Sprintf("%v", item["leaders"])})
				if len(rows) >= topN {
					break
				}
			}
			if len(rows) == 0 {
				return "未找到符合条件的异动类型", nil
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("近%d日异动类型与领先股票", days), rows), nil
		},
	)
}

func newTdxUnusualStockProfileTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"QueryTdxUnusualStockProfile",
		"查询指定股票的通达信异动画像：近N日全部异动明细、异动类型分布、多空倾向、活跃时段分布。"+
			"用于验证某只股票是否被资金持续关注（如连续大单托盘+主力买入），是潜力股确认工具。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 600519、300750、920098",
				Required: true,
			},
			"days": {
				Type:     "integer",
				Desc:     "回溯天数，如1/3/5/10，默认5",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := strings.TrimSpace(gjson.Get(args, "stockCode").String())
			if stockCode == "" {
				return "请提供股票代码参数 stockCode", nil
			}
			stockCode = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(stockCode, "sh"), "sz"), "bj")
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 5
			}
			startDay := time.Now().In(data.TdxShanghaiTimezone).AddDate(0, 0, -(days - 1)).Format("2006-01-02")

			var events []models.TdxUnusualEvent
			if err := db.Dao.Where("stock_code = ? and change_date >= ?", stockCode, startDay).
				Order("change_date desc, change_time desc").Limit(200).
				Find(&events).Error; err != nil {
				return "", err
			}
			if len(events) == 0 {
				return fmt.Sprintf("股票 %s 近%d日无通达信异动记录", stockCode, days), nil
			}

			stockName := events[0].StockName
			typeDist := map[string]int64{}
			timeDist := map[string]int64{}
			bull, bear, neutral := int64(0), int64(0), int64(0)
			for _, e := range events {
				typeDist[e.TypeName]++
				if len(e.ChangeTime) >= 2 {
					timeDist[e.ChangeTime[:2]+"时"]++
				}
				switch data.TdxUnusualTrend(e.TypeName) {
				case 1:
					bull++
				case -1:
					bear++
				default:
					neutral++
				}
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("## %s(%s) 近%d日通达信异动画像\n\n", stockName, stockCode, days))
			sb.WriteString(fmt.Sprintf("异动总数 %d（偏多 %d / 偏空 %d / 中性 %d）→ 整体%s\n\n",
				len(events), bull, bear, neutral, trendLabel(map[bool]int{true: 1, false: -1}[bull >= bear])))

			type distRow struct {
				TypeName string `md:"异动类型"`
				Count    int64  `md:"次数"`
			}
			var distRows []distRow
			keys := make([]string, 0, len(typeDist))
			for k := range typeDist {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return typeDist[keys[i]] > typeDist[keys[j]] })
			for _, k := range keys {
				distRows = append(distRows, distRow{TypeName: k, Count: typeDist[k]})
			}
			sb.WriteString(util.MarkdownTableWithTitle("异动类型分布", distRows))

			type detailRow struct {
				Date     string `md:"日期"`
				Time     string `md:"时间"`
				TypeName string `md:"异动类型"`
				Value    string `md:"数值"`
			}
			var detailRows []detailRow
			for _, e := range events {
				detailRows = append(detailRows, detailRow{Date: e.ChangeDate, Time: e.ChangeTime, TypeName: e.TypeName, Value: e.Value})
			}
			sb.WriteString("\n" + util.MarkdownTableWithTitle("异动明细（最近200条）", detailRows))

			// 活跃时段
			hourKeys := make([]string, 0, len(timeDist))
			for k := range timeDist {
				hourKeys = append(hourKeys, k)
			}
			sort.Slice(hourKeys, func(i, j int) bool { return timeDist[hourKeys[i]] > timeDist[hourKeys[j]] })
			hourParts := make([]string, 0, len(hourKeys))
			for _, k := range hourKeys {
				hourParts = append(hourParts, fmt.Sprintf("%s(%d次)", k, timeDist[k]))
			}
			sb.WriteString("\n活跃时段：" + strings.Join(hourParts, "、"))

			return sb.String(), nil
		},
	)
}
