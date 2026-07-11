package data

import (
	"fmt"
	"go-stock/backend/db"
	"strings"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/7/10
// @Desc 「关注股票」AI 工具处理器：关注一只股票并可同时设置分组、概念标签、成本价、持仓量、止盈止损价位等附加信息
// -----------------------------------------------------------------------------------

func init() {
	registerToolHandler("FollowStock", handleFollowStock)
}

// handleFollowStock 处理 FollowStock 工具调用。
// 一次完成：关注 → 加分组（按名查找/创建）→ 加概念（按名查找/去重创建）→ 设成本/持仓 → 设价位线。
// "已经关注了"不视为错误，继续设置附加信息；"关注失败"/"最多只能关注63只..."则中止。
func handleFollowStock(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "FollowStock", funcArguments)

	stockCode := gjson.Get(funcArguments, "stockCode").String()
	if strings.TrimSpace(stockCode) == "" {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"❌ 参数 stockCode 不能为空。",
		)
		return nil
	}

	api := NewStockDataApi()
	followResult := api.Follow(stockCode)

	// 关注失败（非"已经关注了"）则中止
	if followResult != "关注成功" && followResult != "已经关注了" {
		content := fmt.Sprintf("❌ 关注失败：%s", followResult)
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			content,
		)
		return nil
	}

	// 归一化 stockCode：与 Follow 内部一致，由共享 helper 处理（us/US → gb_+小写；其余小写）
	normalizedCode := normalizeStockCode(stockCode)

	var lines []string
	if followResult == "已经关注了" {
		lines = append(lines, fmt.Sprintf("ℹ️ %s 已经关注过，继续设置附加信息。", stockCode))
	} else {
		lines = append(lines, fmt.Sprintf("✅ 关注成功：%s", stockCode))
	}

	// 分组：按名查找/创建并关联（共享 helper）
	groupNames := gjson.Get(funcArguments, "groupNames").String()
	if strings.TrimSpace(groupNames) != "" {
		addedGroups := addStockToGroupsByName(normalizedCode, groupNames)
		if len(addedGroups) > 0 {
			lines = append(lines, fmt.Sprintf("📂 加入分组：%s", strings.Join(addedGroups, "、")))
		}
	}

	// 概念：按名查找/去重创建并关联（共享 helper）
	conceptNames := gjson.Get(funcArguments, "conceptNames").String()
	if strings.TrimSpace(conceptNames) != "" {
		addedConcepts := addStockToConceptsByName(normalizedCode, conceptNames)
		if len(addedConcepts) > 0 {
			lines = append(lines, fmt.Sprintf("🏷️ 加入概念：%s", strings.Join(addedConcepts, "、")))
		}
	}

	// 成本价 / 持仓量
	costPrice := gjson.Get(funcArguments, "costPrice").Float()
	volume := gjson.Get(funcArguments, "volume").Int()
	if costPrice > 0 || volume > 0 {
		priceResult := api.SetCostPriceAndVolume(costPrice, volume, normalizedCode)
		if priceResult == "设置成功" {
			lines = append(lines, fmt.Sprintf("💵 成本价：%.2f，持仓：%d 股", costPrice, volume))
		} else {
			lines = append(lines, fmt.Sprintf("⚠️ 成本/持仓设置失败：%s", priceResult))
		}
	}

	// 价位线（开仓/止盈/止损/成本）
	entryPrice := gjson.Get(funcArguments, "entryPrice").Float()
	takeProfitPrice := gjson.Get(funcArguments, "takeProfitPrice").Float()
	stopLossPrice := gjson.Get(funcArguments, "stopLossPrice").Float()
	if entryPrice > 0 || takeProfitPrice > 0 || stopLossPrice > 0 {
		tpResult := api.SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, normalizedCode)
		if tpResult == "设置成功" {
			lines = append(lines, fmt.Sprintf("🎯 价位线：开仓 %.2f / 止盈 %.2f / 止损 %.2f", entryPrice, takeProfitPrice, stopLossPrice))
		} else {
			lines = append(lines, fmt.Sprintf("⚠️ 价位线设置失败：%s", tpResult))
		}
	}

	content := strings.Join(lines, "\n")

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)

	return nil
}

// processNamesAssign 按「英文逗号/中文逗号」切分名称，逐个查找或创建实体，关联到股票。
// findOrCreate 返回实体 ID；assign 把 ID 关联到股票。返回成功处理的名称列表。
func processNamesAssign(raw string, findOrCreate func(name string) (int, error), assign func(id int) bool) []string {
	var done []string
	for _, name := range splitNames(raw) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, err := findOrCreate(name)
		if err != nil || id <= 0 {
			continue
		}
		if assign(id) {
			done = append(done, name)
		}
	}
	return done
}

// splitNames 按英文逗号、中文逗号、中文顿号切分
func splitNames(raw string) []string {
	s := strings.ReplaceAll(raw, "，", ",")
	s = strings.ReplaceAll(s, "、", ",")
	return strings.Split(s, ",")
}

// findOrCreateGroup 按名查找分组（大小写无关），找不到则创建，返回分组 ID
func findOrCreateGroup(name string) (int, error) {
	groupApi := NewStockGroupApi(db.Dao)
	groups := groupApi.GetGroupList()
	lower := strings.ToLower(name)
	for _, g := range groups {
		if strings.ToLower(g.Name) == lower {
			return int(g.ID), nil
		}
	}
	// 创建新分组
	if !groupApi.AddGroup(Group{Name: name, Sort: 1}) {
		return 0, fmt.Errorf("创建分组失败：%s", name)
	}
	// 重新拉取获取 ID
	for _, g := range groupApi.GetGroupList() {
		if strings.ToLower(g.Name) == lower {
			return int(g.ID), nil
		}
	}
	return 0, fmt.Errorf("创建分组后未找到：%s", name)
}

// findOrCreateConcept 按名查找概念（大小写无关），找不到则创建（AddConcept 幂等去重），返回概念 ID
func findOrCreateConcept(name string) (int, error) {
	conceptApi := NewStockConceptApi(db.Dao)
	// AddConcept 幂等：已存在返回 true，不存在则创建
	if !conceptApi.AddConcept(Concept{Name: name, Sort: 1}) {
		return 0, fmt.Errorf("创建概念失败：%s", name)
	}
	lower := strings.ToLower(name)
	for _, c := range conceptApi.GetConceptList() {
		if strings.ToLower(c.Name) == lower {
			return int(c.ID), nil
		}
	}
	return 0, fmt.Errorf("创建概念后未找到：%s", name)
}
