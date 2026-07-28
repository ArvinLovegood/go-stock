package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 通知渠道常量
const (
	NotifyChannelApp      = "app"      // 软件内提醒（系统通知 + 前端 newsPush）
	NotifyChannelFeishu   = "feishu"   // 飞书
	NotifyChannelDingDing = "dingding" // 钉钉
)

// MonitorDailyOperationPlan 监控每日操作计划的触发条件，盘中达到条件时发送通知
func MonitorDailyOperationPlan(a *App) {
	// 任意一个市场开市即监控（计划可能涉及 A/港/美股）
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())
	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		return
	}

	today := time.Now().Format("2006-01-02")
	var plans []models.DailyOperationPlan
	db.Dao.Model(&models.DailyOperationPlan{}).
		Where("enable_alert = ?", true).
		Where("status IN ?", []string{"pending", "executing"}).
		Where(
			"((plan_end_date = '' OR plan_end_date IS NULL) AND plan_date = ?) "+
				"OR (plan_end_date <> '' AND plan_end_date IS NOT NULL AND plan_date <= ? AND plan_end_date >= ?)",
			today, today, today,
		).
		Find(&plans)

	if len(plans) == 0 {
		return
	}

	// 收集股票代码并建立反向索引
	stockCodes := make([]string, 0, len(plans))
	codeToPlans := make(map[string][]*models.DailyOperationPlan)
	for i := range plans {
		p := &plans[i]
		code := tools.GetStockCode(p.StockCode)
		if _, exists := codeToPlans[code]; !exists {
			stockCodes = append(stockCodes, code)
		}
		codeToPlans[code] = append(codeToPlans[code], p)
	}

	if len(stockCodes) == 0 {
		return
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取操作计划实时行情失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		code := tools.GetStockCode(stockInfo.Code)
		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}
		affectedPlans, ok := codeToPlans[code]
		if !ok {
			continue
		}
		for _, p := range affectedPlans {
			monitorPlanPrice(a, p, currentPrice)
		}
	}
}

// monitorPlanPrice 对单个计划的所有情景进行价格监控
func monitorPlanPrice(a *App, p *models.DailyOperationPlan, currentPrice float64) {
	var scenarios []models.OperationScenario
	if err := json.Unmarshal([]byte(p.Scenarios), &scenarios); err != nil || len(scenarios) == 0 {
		return
	}

	channels := parseNotifyChannels(p.NotifyChannels)

	for idx, sc := range scenarios {
		scenarioLabel := fmt.Sprintf("情景%d", idx+1)
		if sc.Title != "" {
			scenarioLabel = sc.Title
		}

		// 1) 止损预警：实时价 <= 止损价（30分钟去重，避免频繁打扰）
		if sc.StopLossPriceNum > 0 && currentPrice <= sc.StopLossPriceNum {
			key := alertKey(p, idx, "STOPLOSS")
			if a.canSendAlert(key, 30*time.Minute) {
				title := fmt.Sprintf("【止损预警】%s", p.StockName)
				content := fmt.Sprintf("## %s 止损预警\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **止损价**: %.2f\n- **触发情景**: %s\n- **计划日期**: %s\n\n⛔ 建议严格执行止损纪律，无条件离场",
					p.StockName, p.StockCode, currentPrice, sc.StopLossPriceNum, scenarioLabel, p.PlanDate)
				plain := fmt.Sprintf("%s(%s) 止损预警\n当前价: %.2f ≤ 止损价: %.2f\n情景: %s",
					p.StockName, p.StockCode, currentPrice, sc.StopLossPriceNum, scenarioLabel)
				a.sendPlanNotification(channels, title, content, plain)
				a.updateAlertSentTime(key)
			}
			continue // 已止损则不再判断该情景的其他条件
		}

		// 2) 情景触发：实时价进入 [TriggerPriceMin, TriggerPriceMax] 区间
		// 使用退出/重新进入检测：价格在区间内时只推送一次，价格离开后重新进入才再次推送
		if sc.TriggerPriceMin > 0 && sc.TriggerPriceMax > 0 &&
			currentPrice >= sc.TriggerPriceMin && currentPrice <= sc.TriggerPriceMax {
			key := alertKey(p, idx, "TRIGGER")
			// priceAtAlertReset > 0 表示已推送过，价格仍在区间内则跳过
			if a.getPriceAtAlertReset(key) == 0 {
				title := fmt.Sprintf("【情景触发】%s", p.StockName)
				content := fmt.Sprintf("## %s 情景触发\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **触发区间**: %.2f - %.2f\n- **触发情景**: %s\n- **动作建议**: %s\n- **仓位**: %s\n- **买入区间**: %s\n- **计划日期**: %s",
					p.StockName, p.StockCode, currentPrice, sc.TriggerPriceMin, sc.TriggerPriceMax,
					scenarioLabel, orDefault(sc.Action, "—"), orDefault(sc.Position, "—"),
					orDefault(sc.BuyPriceRange, "—"), p.PlanDate)
				plain := fmt.Sprintf("%s(%s) 情景触发\n当前价: %.2f ∈ [%.2f, %.2f]\n动作: %s 仓位: %s",
					p.StockName, p.StockCode, currentPrice, sc.TriggerPriceMin, sc.TriggerPriceMax,
					orDefault(sc.Action, "—"), orDefault(sc.Position, "—"))
				a.sendPlanNotification(channels, title, content, plain)
				a.updatePriceAtAlertReset(key, currentPrice)
			}
		} else if sc.TriggerPriceMin > 0 && sc.TriggerPriceMax > 0 {
			// 价格离开触发区间，重置状态以便下次重新进入时再次推送
			key := alertKey(p, idx, "TRIGGER")
			if a.getPriceAtAlertReset(key) != 0 {
				a.updatePriceAtAlertReset(key, 0)
			}
		}

		// 3) 第一目标达成：实时价 >= Target1Min（当日只推送一次）
		if sc.Target1Min > 0 && currentPrice >= sc.Target1Min {
			key := alertKey(p, idx, "TARGET1")
			if a.canSendAlert(key, 24*time.Hour) {
				title := fmt.Sprintf("【第一目标达成】%s", p.StockName)
				tRange := priceRangeText(sc.Target1Min, sc.Target1Max)
				content := fmt.Sprintf("## %s 第一目标达成\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **目标区间**: %s\n- **触发情景**: %s\n- **计划日期**: %s\n\n💡 建议按计划减仓 1/2",
					p.StockName, p.StockCode, currentPrice, tRange, scenarioLabel, p.PlanDate)
				plain := fmt.Sprintf("%s(%s) 第一目标达成\n当前价: %.2f ≥ 目标: %s",
					p.StockName, p.StockCode, currentPrice, tRange)
				a.sendPlanNotification(channels, title, content, plain)
				a.updateAlertSentTime(key)
			}
		}

		// 4) 第二目标达成：实时价 >= Target2Min（当日只推送一次）
		if sc.Target2Min > 0 && currentPrice >= sc.Target2Min {
			key := alertKey(p, idx, "TARGET2")
			if a.canSendAlert(key, 24*time.Hour) {
				title := fmt.Sprintf("【第二目标达成】%s", p.StockName)
				tRange := priceRangeText(sc.Target2Min, sc.Target2Max)
				content := fmt.Sprintf("## %s 第二目标达成\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **目标区间**: %s\n- **触发情景**: %s\n- **计划日期**: %s\n\n💰 建议按计划全部止盈",
					p.StockName, p.StockCode, currentPrice, tRange, scenarioLabel, p.PlanDate)
				plain := fmt.Sprintf("%s(%s) 第二目标达成\n当前价: %.2f ≥ 目标: %s",
					p.StockName, p.StockCode, currentPrice, tRange)
				a.sendPlanNotification(channels, title, content, plain)
				a.updateAlertSentTime(key)
			}
		}
	}
}

// sendPlanNotification 按通知渠道发送预警
func (a *App) sendPlanNotification(channels []string, title, content, plainContent string) {
	useAll := len(channels) == 0
	for _, ch := range channels {
		switch ch {
		case NotifyChannelApp:
			go data.NewAlertWindowsApi("go-stock操作计划预警", title, content, "").SendNotification()
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    title,
				"isRed":   true,
				"source":  "go-stock",
				"content": plainContent,
			})
		case NotifyChannelFeishu:
			go data.NewFeishuAPI().SendToFeishu(title, content)
		case NotifyChannelDingDing:
			go data.NewDingDingAPI().SendToDingDing(title, content)
		}
	}
	if useAll {
		// 未配置渠道时，全部发送
		go data.NewAlertWindowsApi("go-stock操作计划预警", title, content, "").SendNotification()
		go data.NewFeishuAPI().SendToFeishu(title, content)
		go data.NewDingDingAPI().SendToDingDing(title, content)
		go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
			"time":    title,
			"isRed":   true,
			"source":  "go-stock",
			"content": plainContent,
		})
	}
}

// alertKey 生成预警去重 key：计划ID+情景序号+类型+日期
func alertKey(p *models.DailyOperationPlan, scenarioIdx int, alertType string) string {
	return fmt.Sprintf("plan:%d:sc:%d:%s:%s", p.ID, scenarioIdx, alertType, p.PlanDate)
}

// parseNotifyChannels 解析通知渠道 JSON
func parseNotifyChannels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var chs []string
	if err := json.Unmarshal([]byte(raw), &chs); err != nil {
		return nil
	}
	return chs
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func priceRangeText(min, max float64) string {
	if max > 0 {
		return fmt.Sprintf("%.2f - %.2f", min, max)
	}
	return fmt.Sprintf("%.2f", min)
}
