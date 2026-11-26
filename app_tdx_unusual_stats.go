package main

// app_tdx_unusual_stats.go — 通达信异动统计分析的 Wails 绑定。
// 供前端「通达信异动」统计看板页面轮询调用；AI 工具走 backend/data 同一服务。

import (
	"encoding/json"

	"go-stock/backend/data"

	"github.com/tidwall/gjson"
)

// GetTdxUnusualDashboard 近 days 日异动统计汇总（今日概览/逐日趋势/活跃股排行/
// 类型分布/分时热度/今日首次异动/市场分布）。days<=0 默认 10，hotTopN<=0 默认 30。
func (a *App) GetTdxUnusualDashboard(days int, hotTopN int) *data.TdxUnusualDashboard {
	return data.GetTdxUnusualDashboard(days, hotTopN)
}

// GetTdxUnusualTypeLeaders 异动类型 → 领先股票（近 days 日，每种类型异动次数 Top 股票）。
func (a *App) GetTdxUnusualTypeLeaders(days int, topN int) []map[string]any {
	return data.QueryTdxUnusualTypeLeaders(days, topN)
}

// GetTdxUnusualEventList 异动明细服务端分页查询。
// query 支持的键：page、pageSize、stockCode、stockName、typeNames（字符串数组）、market（sh/sz/bj）、
// startDate、endDate（YYYY-MM-DD HH:mm:ss，支持仅日期）。
func (a *App) GetTdxUnusualEventList(query map[string]any) *data.TdxUnusualEventPage {
	raw, err := json.Marshal(query)
	if err != nil {
		return data.QueryTdxUnusualEventPage(1, 20, "", "", nil, "", "", "")
	}
	args := gjson.ParseBytes(raw)
	typeNames := make([]string, 0)
	for _, t := range args.Get("typeNames").Array() {
		if t.String() != "" {
			typeNames = append(typeNames, t.String())
		}
	}
	return data.QueryTdxUnusualEventPage(
		int(args.Get("page").Int()),
		int(args.Get("pageSize").Int()),
		args.Get("stockCode").String(),
		args.Get("stockName").String(),
		typeNames,
		args.Get("market").String(),
		args.Get("startDate").String(),
		args.Get("endDate").String(),
	)
}

// GetTdxUnusualTypeOptions 异动类型下拉选项（全部去重类型名）。
func (a *App) GetTdxUnusualTypeOptions() []string {
	return data.QueryTdxUnusualTypeOptions()
}
