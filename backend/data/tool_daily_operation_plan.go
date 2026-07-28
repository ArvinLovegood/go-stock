package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("AddDailyOperationPlan", handleAddDailyOperationPlan)
	registerToolHandler("GetDailyOperationPlanList", handleGetDailyOperationPlanList)
	registerToolHandler("UpdateDailyOperationPlan", handleUpdateDailyOperationPlan)
	registerToolHandler("UpdateDailyOperationPlanStatus", handleUpdateDailyOperationPlanStatus)
}

// handleAddDailyOperationPlan 处理 AddDailyOperationPlan 工具调用。
// 将 AI 生成的操作方案结构化保存为「每日操作计划」，支持多情景、操作纪律、盘中预警。
func handleAddDailyOperationPlan(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AddDailyOperationPlan", funcArguments)

	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())
	stockName := strings.TrimSpace(gjson.Get(funcArguments, "stockName").String())

	if stockCode == "" || stockName == "" {
		appendToolMessages(
			ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"❌ 参数 stockCode 和 stockName 不能为空。",
		)
		return nil
	}

	// 计划日期，默认今天
	planDate := strings.TrimSpace(gjson.Get(funcArguments, "planDate").String())
	if planDate == "" {
		planDate = time.Now().Format("2006-01-02")
	}

	// 解析情景方案
	scenarios := parseScenarios(funcArguments)
	if len(scenarios) == 0 {
		appendToolMessages(
			ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"❌ 参数 scenarios 不能为空，至少需要一个情景方案。",
		)
		return nil
	}
	scenariosJSON, _ := json.Marshal(scenarios)

	// 解析操作纪律
	discipline := parseDiscipline(funcArguments)
	disciplineJSON, _ := json.Marshal(discipline)

	// 通知渠道
	enableAlert := true
	if gjson.Get(funcArguments, "enableAlert").Exists() {
		enableAlert = gjson.Get(funcArguments, "enableAlert").Bool()
	}
	channels := parseNotifyChannelsArg(funcArguments)
	channelsJSON, _ := json.Marshal(channels)

	// 风险提示，默认文案
	riskWarning := strings.TrimSpace(gjson.Get(funcArguments, "riskWarning").String())
	if riskWarning == "" {
		riskWarning = "该股近期波动较大，日内振幅可能较高，属于高波动品种。以上分析基于公开数据，不构成投资建议。投资有风险，入市需谨慎。请根据自身风险承受能力理性决策。"
	}

	planEndDate := strings.TrimSpace(gjson.Get(funcArguments, "planEndDate").String())

	plan := models.DailyOperationPlan{
		PlanDate:        planDate,
		PlanEndDate:     planEndDate,
		StockCode:       stockCode,
		StockName:       stockName,
		OverallJudgment: gjson.Get(funcArguments, "overallJudgment").String(),
		Scenarios:       string(scenariosJSON),
		Discipline:      string(disciplineJSON),
		Summary:         gjson.Get(funcArguments, "summary").String(),
		RiskWarning:     riskWarning,
		Status:          "pending",
		EnableAlert:     enableAlert,
		NotifyChannels:  string(channelsJSON),
	}

	result := NewDailyOperationPlanApi().SaveDailyOperationPlan(plan)

	// 组装反馈内容
	var lines []string
	if strings.Contains(result, "成功") {
		lines = append(lines, fmt.Sprintf("✅ %s(%s) 操作计划已创建，计划日期 %s", stockName, stockCode, planDate))
		lines = append(lines, fmt.Sprintf("📋 共 %d 个情景方案", len(scenarios)))
		if len(discipline) > 0 {
			lines = append(lines, fmt.Sprintf("📌 共 %d 条操作纪律", len(discipline)))
		}
		if enableAlert {
			lines = append(lines, fmt.Sprintf("🔔 已开启盘中预警，通知渠道：%s", strings.Join(channelLabels(channels), "、")))
		} else {
			lines = append(lines, "🔕 未开启盘中预警")
		}
		lines = append(lines, "👉 可在「研究中心 → 每日操作计划」页面查看详情")
	} else {
		lines = append(lines, fmt.Sprintf("❌ 创建失败：%s", result))
	}

	appendToolMessages(
		ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments,
		strings.Join(lines, "\n"),
	)
	return nil
}

// parseScenarios 从工具参数解析情景方案数组（兼容数组或 JSON 字符串，后者用于 Agent 模式）
func parseScenarios(funcArguments string) []models.OperationScenario {
	scenariosResult := gjson.Get(funcArguments, "scenarios")
	if scenariosResult.Type == gjson.String {
		if parsed := gjson.Parse(scenariosResult.String()); parsed.IsArray() {
			scenariosResult = parsed
		}
	}
	if !scenariosResult.IsArray() {
		return nil
	}
	var scenarios []models.OperationScenario
	for _, sc := range scenariosResult.Array() {
		s := models.OperationScenario{
			Title:            sc.Get("title").String(),
			Condition:        sc.Get("condition").String(),
			ActionType:       sc.Get("actionType").String(),
			Action:           sc.Get("action").String(),
			Position:         sc.Get("position").String(),
			BuyPriceRange:    sc.Get("buyPriceRange").String(),
			StopLossPrice:    sc.Get("stopLossPrice").String(),
			Target1:          sc.Get("target1").String(),
			Target2:          sc.Get("target2").String(),
			Strategy:         sc.Get("strategy").String(),
			IsBest:           sc.Get("isBest").Bool(),
			TriggerPriceMin:  sc.Get("triggerPriceMin").Float(),
			TriggerPriceMax:  sc.Get("triggerPriceMax").Float(),
			StopLossPriceNum: sc.Get("stopLossPriceNum").Float(),
			Target1Min:       sc.Get("target1Min").Float(),
			Target1Max:       sc.Get("target1Max").Float(),
			Target2Min:       sc.Get("target2Min").Float(),
			Target2Max:       sc.Get("target2Max").Float(),
		}
		if s.ActionType == "" {
			s.ActionType = "buy"
		}
		scenarios = append(scenarios, s)
	}
	return scenarios
}

// parseDiscipline 从工具参数解析操作纪律数组（兼容数组或 JSON 字符串）
func parseDiscipline(funcArguments string) []models.OperationDiscipline {
	disciplineResult := gjson.Get(funcArguments, "discipline")
	if disciplineResult.Type == gjson.String {
		if parsed := gjson.Parse(disciplineResult.String()); parsed.IsArray() {
			disciplineResult = parsed
		}
	}
	if !disciplineResult.IsArray() {
		return nil
	}
	var discipline []models.OperationDiscipline
	for _, d := range disciplineResult.Array() {
		discipline = append(discipline, models.OperationDiscipline{
			Principle: d.Get("principle").String(),
			Detail:    d.Get("detail").String(),
		})
	}
	return discipline
}

// parseNotifyChannelsArg 从工具参数解析通知渠道数组（兼容数组或 JSON 字符串）
func parseNotifyChannelsArg(funcArguments string) []string {
	channelsResult := gjson.Get(funcArguments, "notifyChannels")
	if channelsResult.Type == gjson.String {
		if parsed := gjson.Parse(channelsResult.String()); parsed.IsArray() {
			channelsResult = parsed
		}
	}
	if !channelsResult.IsArray() {
		return []string{"app", "feishu", "dingding"}
	}
	var channels []string
	for _, ch := range channelsResult.Array() {
		c := strings.TrimSpace(ch.String())
		if c != "" {
			channels = append(channels, c)
		}
	}
	if len(channels) == 0 {
		return []string{"app", "feishu", "dingding"}
	}
	return channels
}

func channelLabels(channels []string) []string {
	labelMap := map[string]string{
		"app":      "软件内提醒",
		"feishu":   "飞书",
		"dingding": "钉钉",
	}
	labels := make([]string, 0, len(channels))
	for _, c := range channels {
		if label, ok := labelMap[c]; ok {
			labels = append(labels, label)
		} else {
			labels = append(labels, c)
		}
	}
	return labels
}

// ParseScenariosFromArgs 导出版本，供 Agent 模式调用
func ParseScenariosFromArgs(funcArguments string) []models.OperationScenario {
	return parseScenarios(funcArguments)
}

// ParseDisciplineFromArgs 导出版本，供 Agent 模式调用
func ParseDisciplineFromArgs(funcArguments string) []models.OperationDiscipline {
	return parseDiscipline(funcArguments)
}

// ParseNotifyChannelsFromArgs 导出版本，供 Agent 模式调用
func ParseNotifyChannelsFromArgs(funcArguments string) []string {
	return parseNotifyChannelsArg(funcArguments)
}

// ChannelLabels 导出版本，供 Agent 模式调用
func ChannelLabels(channels []string) []string {
	return channelLabels(channels)
}

// handleGetDailyOperationPlanList 处理 GetDailyOperationPlanList 工具调用。
// 查询每日操作计划列表，支持按股票代码/名称/日期/状态筛选，返回 Markdown 格式的计划详情供 AI 分析。
func handleGetDailyOperationPlanList(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetDailyOperationPlanList", funcArguments)

	query := buildPlanQuery(funcArguments)
	result, err := NewDailyOperationPlanApi().GetDailyOperationPlanList(query)
	if err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "❌ 查询失败: "+err.Error())
		return nil
	}

	md := renderPlanListToMarkdown(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, md)
	return nil
}

// buildPlanQuery 从工具参数构建查询条件
func buildPlanQuery(funcArguments string) *models.DailyOperationPlanQuery {
	query := &models.DailyOperationPlanQuery{
		StockCode: strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String()),
		StockName: strings.TrimSpace(gjson.Get(funcArguments, "stockName").String()),
		PlanDate:  strings.TrimSpace(gjson.Get(funcArguments, "planDate").String()),
		Status:    strings.TrimSpace(gjson.Get(funcArguments, "status").String()),
		Page:      int(gjson.Get(funcArguments, "page").Int()),
		PageSize:  int(gjson.Get(funcArguments, "pageSize").Int()),
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	return query
}

// renderPlanListToMarkdown 将操作计划列表渲染为 Markdown，供 AI 分析
func renderPlanListToMarkdown(result *models.DailyOperationPlanPageData) string {
	if result == nil || len(result.List) == 0 {
		return "暂无操作计划数据"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 操作计划列表（第%d页/共%d页，每页%d条，共%d条）\n\n", result.Page, result.TotalPages, result.PageSize, result.Total))
	for _, plan := range result.List {
		sb.WriteString(renderPlanToMarkdown(&plan))
		sb.WriteString("\n---\n\n")
	}
	return sb.String()
}

// renderPlanToMarkdown 将单个操作计划渲染为 Markdown
func renderPlanToMarkdown(plan *models.DailyOperationPlan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（%s）操作计划 - %s\n", plan.StockName, plan.StockCode, plan.PlanDate))
	sb.WriteString(fmt.Sprintf("- **状态**：%s\n", planStatusText(plan.Status)))
	sb.WriteString(fmt.Sprintf("- **盘中预警**：%s\n", planBoolText(plan.EnableAlert, "已开启", "未开启")))
	if plan.OverallJudgment != "" {
		sb.WriteString(fmt.Sprintf("- **总体判断**：%s\n", plan.OverallJudgment))
	}

	// 情景方案
	if plan.Scenarios != "" {
		var scenarios []models.OperationScenario
		if err := json.Unmarshal([]byte(plan.Scenarios), &scenarios); err == nil && len(scenarios) > 0 {
			sb.WriteString("\n### 情景方案\n")
			for i, sc := range scenarios {
				best := ""
				if sc.IsBest {
					best = " ⭐最优"
				}
				sb.WriteString(fmt.Sprintf("\n**情景%d：%s**%s\n", i+1, sc.Title, best))
				sb.WriteString(fmt.Sprintf("- 操作类型：%s\n", actionTypeText(sc.ActionType)))
				if sc.Condition != "" {
					sb.WriteString(fmt.Sprintf("- 触发条件：%s\n", sc.Condition))
				}
				if sc.Action != "" {
					sb.WriteString(fmt.Sprintf("- 动作：%s\n", sc.Action))
				}
				if sc.Position != "" {
					sb.WriteString(fmt.Sprintf("- 仓位：%s\n", sc.Position))
				}
				if sc.BuyPriceRange != "" {
					sb.WriteString(fmt.Sprintf("- 买入区间：%s\n", sc.BuyPriceRange))
				}
				if sc.StopLossPrice != "" {
					sb.WriteString(fmt.Sprintf("- 止损价：%s\n", sc.StopLossPrice))
				}
				if sc.Target1 != "" {
					sb.WriteString(fmt.Sprintf("- 第一目标：%s\n", sc.Target1))
				}
				if sc.Target2 != "" {
					sb.WriteString(fmt.Sprintf("- 第二目标：%s\n", sc.Target2))
				}
				if sc.TriggerPriceMin > 0 || sc.TriggerPriceMax > 0 {
					sb.WriteString(fmt.Sprintf("- 量化触发价：%.2f ~ %.2f\n", sc.TriggerPriceMin, sc.TriggerPriceMax))
				}
				if sc.StopLossPriceNum > 0 {
					sb.WriteString(fmt.Sprintf("- 量化止损价：%.2f\n", sc.StopLossPriceNum))
				}
				if sc.Target1Min > 0 || sc.Target1Max > 0 {
					sb.WriteString(fmt.Sprintf("- 量化目标1：%.2f ~ %.2f\n", sc.Target1Min, sc.Target1Max))
				}
				if sc.Target2Min > 0 || sc.Target2Max > 0 {
					sb.WriteString(fmt.Sprintf("- 量化目标2：%.2f ~ %.2f\n", sc.Target2Min, sc.Target2Max))
				}
				if sc.Strategy != "" {
					sb.WriteString(fmt.Sprintf("- 策略说明：%s\n", sc.Strategy))
				}
			}
		}
	}

	// 操作纪律
	if plan.Discipline != "" {
		var discipline []models.OperationDiscipline
		if err := json.Unmarshal([]byte(plan.Discipline), &discipline); err == nil && len(discipline) > 0 {
			sb.WriteString("\n### 操作纪律\n")
			for i, d := range discipline {
				sb.WriteString(fmt.Sprintf("%d. **%s**：%s\n", i+1, d.Principle, d.Detail))
			}
		}
	}

	if plan.Summary != "" {
		sb.WriteString(fmt.Sprintf("\n**总结**：%s\n", plan.Summary))
	}
	if plan.RiskWarning != "" {
		sb.WriteString(fmt.Sprintf("\n**风险提示**：%s\n", plan.RiskWarning))
	}
	return sb.String()
}

func planStatusText(status string) string {
	m := map[string]string{"pending": "待执行", "executing": "执行中", "done": "已完成", "cancelled": "已取消"}
	if t, ok := m[status]; ok {
		return t
	}
	return status
}

func planBoolText(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}

func actionTypeText(t string) string {
	m := map[string]string{"buy": "买入", "sell": "卖出", "watch": "观望"}
	if t, ok := m[t]; ok {
		return t
	}
	return t
}

// handleUpdateDailyOperationPlan 处理 UpdateDailyOperationPlan 工具调用。
// 根据计划 ID 编辑已有操作计划，仅更新传入的字段（部分更新），未传入的字段保持原值。
func handleUpdateDailyOperationPlan(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "UpdateDailyOperationPlan", funcArguments)

	planID := uint(gjson.Get(funcArguments, "planId").Int())
	if planID == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "❌ 参数 planId 不能为空。可先调用 GetDailyOperationPlanList 查询计划列表获取 ID。")
		return nil
	}

	// 查询已有计划
	existing, err := NewDailyOperationPlanApi().GetDailyOperationPlanByID(planID)
	if err != nil || existing.ID == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("❌ 计划 ID %d 不存在", planID))
		return nil
	}

	// 部分更新：仅覆盖传入的非空字段
	updated := *existing
	if v := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String()); v != "" {
		updated.StockCode = v
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "stockName").String()); v != "" {
		updated.StockName = v
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "planDate").String()); v != "" {
		updated.PlanDate = v
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "planEndDate").String()); v != "" {
		updated.PlanEndDate = v
	}
	if v := gjson.Get(funcArguments, "overallJudgment"); v.Exists() {
		updated.OverallJudgment = v.String()
	}
	if v := gjson.Get(funcArguments, "summary"); v.Exists() {
		updated.Summary = v.String()
	}
	if v := gjson.Get(funcArguments, "riskWarning"); v.Exists() {
		updated.RiskWarning = v.String()
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "status").String()); v != "" {
		updated.Status = v
	}
	// 情景方案：仅当传入 scenarios 时才更新
	if gjson.Get(funcArguments, "scenarios").Exists() {
		scenarios := parseScenarios(funcArguments)
		if len(scenarios) > 0 {
			scenariosJSON, _ := json.Marshal(scenarios)
			updated.Scenarios = string(scenariosJSON)
		}
	}
	// 操作纪律：仅当传入 discipline 时才更新
	if gjson.Get(funcArguments, "discipline").Exists() {
		discipline := parseDiscipline(funcArguments)
		disciplineJSON, _ := json.Marshal(discipline)
		updated.Discipline = string(disciplineJSON)
	}
	// 预警开关
	if gjson.Get(funcArguments, "enableAlert").Exists() {
		updated.EnableAlert = gjson.Get(funcArguments, "enableAlert").Bool()
	}
	// 通知渠道
	if gjson.Get(funcArguments, "notifyChannels").Exists() {
		channels := parseNotifyChannelsArg(funcArguments)
		channelsJSON, _ := json.Marshal(channels)
		updated.NotifyChannels = string(channelsJSON)
	}

	result := NewDailyOperationPlanApi().SaveDailyOperationPlan(updated)

	var lines []string
	if strings.Contains(result, "成功") {
		lines = append(lines, fmt.Sprintf("✅ %s(%s) 操作计划已更新", updated.StockName, updated.StockCode))
		lines = append(lines, fmt.Sprintf("📋 计划ID：%d，计划日期：%s，状态：%s", planID, updated.PlanDate, planStatusText(updated.Status)))
		lines = append(lines, "👉 可在「研究中心 → 每日操作计划」页面查看详情")
	} else {
		lines = append(lines, fmt.Sprintf("❌ 更新失败：%s", result))
	}

	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, strings.Join(lines, "\n"))
	return nil
}

// handleUpdateDailyOperationPlanStatus 处理 UpdateDailyOperationPlanStatus 工具调用。
// 快速更新操作计划状态（pending/executing/done/cancelled）。
func handleUpdateDailyOperationPlanStatus(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "UpdateDailyOperationPlanStatus", funcArguments)

	planID := uint(gjson.Get(funcArguments, "planId").Int())
	status := strings.TrimSpace(gjson.Get(funcArguments, "status").String())

	if planID == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "❌ 参数 planId 不能为空。")
		return nil
	}
	validStatus := map[string]bool{"pending": true, "executing": true, "done": true, "cancelled": true}
	if !validStatus[status] {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments,
			"❌ 参数 status 无效，可选值：pending(待执行)、executing(执行中)、done(已完成)、cancelled(已取消)")
		return nil
	}

	existing, err := NewDailyOperationPlanApi().GetDailyOperationPlanByID(planID)
	if err != nil || existing.ID == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("❌ 计划 ID %d 不存在", planID))
		return nil
	}

	if err := NewDailyOperationPlanApi().UpdateDailyOperationPlanStatus(planID, status); err != nil {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "❌ 状态更新失败: "+err.Error())
		return nil
	}

	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments,
		fmt.Sprintf("✅ %s(%s) 操作计划状态已更新为「%s」", existing.StockName, existing.StockCode, planStatusText(status)))
	return nil
}

// RenderPlanListToMarkdown 导出版本，供 Agent 模式调用
func RenderPlanListToMarkdown(result *models.DailyOperationPlanPageData) string {
	return renderPlanListToMarkdown(result)
}

// BuildPlanQuery 导出版本，供 Agent 模式调用
func BuildPlanQuery(funcArguments string) *models.DailyOperationPlanQuery {
	return buildPlanQuery(funcArguments)
}
