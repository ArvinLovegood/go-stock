package tools

// tdx_unusual_tool.go — 通达信主力异动事件查询工具。
//
// 数据来源：盘中「通达信异动通知」轮询采集并单独落库的 tdx_unusual_events 表
// （app_tdx_unusual_monitor.go），与东财体系的 GetStockChanges/GetStockChangeHistoryList
// 互为补充：本表覆盖北交所，且异动类型为通达信主力监控口径（大单托盘/高位回落/主力买入等）。

import (
	"fmt"
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// newTdxUnusualQueryTool 构建通达信异动事件多维度查询工具：
// 支持按股票代码/名称、异动类型、市场、日期与时间范围组合筛选，可分组统计，分页返回。
func newTdxUnusualQueryTool() *DataToolWrapper {
	return NewDataToolWrapper(
		"QueryTdxUnusualEvents",
		"查询通达信主力监控异动事件（盘中实时采集，含沪深北，北交所专属；与东财异动 GetStockChanges 互补）。支持按代码/名称/异动类型/日期/时间组合筛选，可按股票/类型/市场分组统计。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选），支持模糊匹配，如 600519、300750、920098",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选），支持模糊匹配，如 贵州茅台",
				Required: false,
			},
			"typeName": {
				Type:     "string",
				Desc:     "异动类型名称筛选（可选），模糊匹配，如：大单托盘/主力买入/加速拉升/高位回落/火箭发射/封涨停 等",
				Required: false,
			},
			"market": {
				Type:     "string",
				Desc:     "市场筛选（可选）：sh=沪市，sz=深市，bj=北证；不传为全部市场",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式 YYYY-MM-DD（可选），默认不限",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式 YYYY-MM-DD（可选），默认不限",
				Required: false,
			},
			"startTime": {
				Type:     "string",
				Desc:     "开始时间，格式 HH:MM:SS（可选），如 09:30:00",
				Required: false,
			},
			"endTime": {
				Type:     "string",
				Desc:     "结束时间，格式 HH:MM:SS（可选），如 15:00:00",
				Required: false,
			},
			"onlyToday": {
				Type:     "boolean",
				Desc:     "仅查询今日数据（可选），true 时忽略日期范围参数",
				Required: false,
			},
			"groupBy": {
				Type:     "string",
				Desc:     "分组统计维度（可选）：stock=按股票统计异动次数，type=按异动类型统计，market=按市场统计；不传则返回明细",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20，最大100",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := strings.TrimSpace(gjson.Get(args, "stockCode").String())
			stockName := strings.TrimSpace(gjson.Get(args, "stockName").String())
			typeName := strings.TrimSpace(gjson.Get(args, "typeName").String())
			market := strings.ToLower(strings.TrimSpace(gjson.Get(args, "market").String()))
			startDate := strings.TrimSpace(gjson.Get(args, "startDate").String())
			endDate := strings.TrimSpace(gjson.Get(args, "endDate").String())
			startTime := strings.TrimSpace(gjson.Get(args, "startTime").String())
			endTime := strings.TrimSpace(gjson.Get(args, "endTime").String())
			onlyToday := gjson.Get(args, "onlyToday").Bool()
			groupBy := strings.ToLower(strings.TrimSpace(gjson.Get(args, "groupBy").String()))
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 || pageSize > 100 {
				pageSize = 20
			}
			if onlyToday {
				startDate, endDate = "", ""
			}

			marketCode := -1
			switch market {
			case "sh", "沪", "沪市":
				marketCode = 1
			case "sz", "深", "深市":
				marketCode = 0
			case "bj", "北", "北证", "北交所":
				marketCode = 2
			}

			// baseQ 每次返回带筛选条件的新查询（避免 GORM 复用已执行链）
			baseQ := func() *gorm.DB {
				q := db.Dao.Model(&models.TdxUnusualEvent{})
				return applyTdxUnusualFilters(q, stockCode, stockName, typeName, marketCode, startDate, endDate, startTime, endTime)
			}

			// 分组统计：按股票 / 异动类型 / 市场维度聚合异动次数
			if groupBy == "stock" || groupBy == "type" || groupBy == "market" {
				type statRow struct {
					Key   string `md:"分组"`
					Count int64  `md:"异动次数"`
					First string `md:"最早时间"`
					Last  string `md:"最晚时间"`
				}
				selectExpr := map[string]string{
					"stock":  "stock_code as `key`, count(*) as count, min(change_time) as first, max(change_time) as last",
					"type":   "type_name as `key`, count(*) as count, min(change_time) as first, max(change_time) as last",
					"market": "market as `key`, count(*) as count, min(change_time) as first, max(change_time) as last",
				}[groupBy]
				groupExpr := map[string]string{"stock": "stock_code", "type": "type_name", "market": "market"}[groupBy]
				var rows []statRow
				if err := baseQ().Select(selectExpr).Group(groupExpr).
					Order("count desc").Limit(pageSize).Scan(&rows).Error; err != nil {
					return "", err
				}
				if len(rows) == 0 {
					return "未找到符合条件的通达信异动记录", nil
				}
				if groupBy == "market" {
					for i := range rows {
						rows[i].Key = tdxMarketName(rows[i].Key)
					}
				}
				label := map[string]string{"stock": "股票", "type": "类型", "market": "市场"}[groupBy]
				return util.MarkdownTableWithTitle("通达信异动"+label+"统计", rows), nil
			}

			// 明细分页查询
			var total int64
			if err := baseQ().Count(&total).Error; err != nil {
				return "", err
			}
			var events []models.TdxUnusualEvent
			if err := baseQ().Order("change_date desc, change_time desc").
				Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error; err != nil {
				return "", err
			}
			if len(events) == 0 {
				return "未找到符合条件的通达信异动记录", nil
			}
			type eventRow struct {
				ChangeDate string `md:"日期"`
				ChangeTime string `md:"时间"`
				StockCode  string `md:"代码"`
				StockName  string `md:"名称"`
				TypeName   string `md:"异动类型"`
				Value      string `md:"数值"`
				Market     string `md:"市场"`
			}
			rows := make([]eventRow, 0, len(events))
			for _, e := range events {
				rows = append(rows, eventRow{
					ChangeDate: e.ChangeDate,
					ChangeTime: e.ChangeTime,
					StockCode:  e.StockCode,
					StockName:  e.StockName,
					TypeName:   e.TypeName,
					Value:      e.Value,
					Market:     tdxMarketName(fmt.Sprintf("%d", e.Market)),
				})
			}
			summary := fmt.Sprintf("共找到 %d 条通达信异动记录，当前第 %d 页（每页 %d 条）", total, page, pageSize)
			return summary + "\n\n" + util.MarkdownTableWithTitle("通达信主力异动明细", rows), nil
		},
	)
}

// applyTdxUnusualFilters 组装多维度筛选条件（代码/名称/类型/市场/日期/时间）。
func applyTdxUnusualFilters(q *gorm.DB, stockCode, stockName, typeName string, marketCode int, startDate, endDate, startTime, endTime string) *gorm.DB {
	if stockCode != "" {
		q = q.Where("stock_code LIKE ?", "%"+stockCode+"%")
	}
	if stockName != "" {
		q = q.Where("stock_name LIKE ?", "%"+stockName+"%")
	}
	if typeName != "" {
		q = q.Where("type_name LIKE ?", "%"+typeName+"%")
	}
	if marketCode >= 0 {
		q = q.Where("market = ?", marketCode)
	}
	if startDate != "" {
		q = q.Where("change_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("change_date <= ?", endDate)
	}
	if startTime != "" {
		q = q.Where("change_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("change_time <= ?", endTime)
	}
	return q
}

// tdxMarketName 市场编号转可读名称。
func tdxMarketName(market string) string {
	switch market {
	case "0":
		return "深市"
	case "1":
		return "沪市"
	case "2":
		return "北证"
	default:
		return market
	}
}
