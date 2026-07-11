package data

import (
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/util"
	"strings"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/7/11
// @Desc 股票分组与概念标签管理的 AI 工具处理器（16 个工具）
//       与 tool_follow_stock.go 共享 helper：findOrCreateGroup/findOrCreateConcept/processNamesAssign/splitNames
// -----------------------------------------------------------------------------------

// === 共享 helper ===

// normalizeStockCode 归一化股票代码：us/US 前缀转 gb_ + 小写；其余直接小写。
// 与 Follow() 内部存储格式一致，所有写工具和按 code 读的工具都必须先调用。
func normalizeStockCode(stockCode string) string {
	if strings.HasPrefix(stockCode, "us") {
		return "gb_" + strings.ToLower(strings.Replace(stockCode, "us", "", 1))
	}
	if strings.HasPrefix(stockCode, "US") {
		return "gb_" + strings.ToLower(strings.Replace(stockCode, "US", "", 1))
	}
	return strings.ToLower(stockCode)
}

// addStockToGroupsByName 按名称查找/创建分组并关联股票（幂等）。返回成功处理的分组名称列表。
func addStockToGroupsByName(normalizedCode, groupNames string) []string {
	return processNamesAssign(groupNames, func(name string) (int, error) {
		return findOrCreateGroup(name)
	}, func(id int) bool {
		return NewStockGroupApi(db.Dao).AddStockGroup(id, normalizedCode)
	})
}

// addStockToConceptsByName 按名称查找/去重创建概念并关联股票（幂等）。返回成功处理的概念名称列表。
func addStockToConceptsByName(normalizedCode, conceptNames string) []string {
	return processNamesAssign(conceptNames, func(name string) (int, error) {
		return findOrCreateConcept(name)
	}, func(id int) bool {
		return NewStockConceptApi(db.Dao).AddStockConcept(id, normalizedCode)
	})
}

// findGroupByName 按名称查找分组（大小写无关），不创建。返回 (id, true) 或 (0, false)。
func findGroupByName(name string) (int, bool) {
	lower := strings.ToLower(name)
	for _, g := range NewStockGroupApi(db.Dao).GetGroupList() {
		if strings.ToLower(g.Name) == lower {
			return int(g.ID), true
		}
	}
	return 0, false
}

// findConceptByName 按名称查找概念（大小写无关），不创建。返回 (id, true) 或 (0, false)。
func findConceptByName(name string) (int, bool) {
	lower := strings.ToLower(name)
	for _, c := range NewStockConceptApi(db.Dao).GetConceptList() {
		if strings.ToLower(c.Name) == lower {
			return int(c.ID), true
		}
	}
	return 0, false
}

// finish 向 messages 追加工具结果（Path A 统一出口）。
func finish(ctx *ToolContext, funcArguments, content string) {
	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)
}

// finishWrite 同 finish，但额外向前端推送 stockDataChanged 事件，触发刷新分组/概念缓存。
// 用于所有写操作（增删改分组/概念归属）成功后调用。
func finishWrite(ctx *ToolContext, funcArguments, content string) {
	EmitStockDataChanged()
	finish(ctx, funcArguments, content)
}

// === 分组管理（P0）===

// 1. GetStockGroups 获取所有分组及组内股票
func handleGetStockGroups(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockGroups", funcArguments)
	groupApi := NewStockGroupApi(db.Dao)
	groups := groupApi.GetGroupList()
	allStocks := groupApi.GetAllGroupStocks()

	// 按分组ID聚合股票
	stocksByGroup := map[int][]string{}
	for _, gs := range allStocks {
		stocksByGroup[int(gs.GroupId)] = append(stocksByGroup[int(gs.GroupId)], gs.StockCode)
	}

	if len(groups) == 0 {
		finish(ctx, funcArguments, "暂无分组")
		return nil
	}

	type row struct {
		GroupId   int    `md:"分组ID"`
		GroupName string `md:"分组名称"`
		Sort      int    `md:"排序"`
		StockCode string `md:"股票代码"`
	}
	var rows []row
	for _, g := range groups {
		codes := stocksByGroup[int(g.ID)]
		if len(codes) == 0 {
			rows = append(rows, row{int(g.ID), g.Name, g.Sort, ""})
			continue
		}
		for _, code := range codes {
			rows = append(rows, row{int(g.ID), g.Name, g.Sort, code})
		}
	}
	finish(ctx, funcArguments, util.MarkdownTableWithTitle("股票分组列表", rows))
	return nil
}

// 2. CreateStockGroup 创建分组（幂等）
func handleCreateStockGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "CreateStockGroup", funcArguments)
	name := strings.TrimSpace(gjson.Get(funcArguments, "groupName").String())
	if name == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 groupName 不能为空。")
		return nil
	}
	id, err := findOrCreateGroup(name)
	if err != nil || id <= 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 创建分组失败：%s", name))
		return nil
	}
	finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 分组「%s」已就绪（ID: %d）", name, id))
	return nil
}

// 3. UpdateStockGroup 重命名分组
func handleUpdateStockGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "UpdateStockGroup", funcArguments)
	id := gjson.Get(funcArguments, "groupId").Int()
	name := strings.TrimSpace(gjson.Get(funcArguments, "groupName").String())
	if id <= 0 || name == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 groupId 和 groupName 均不能为空。")
		return nil
	}
	if NewStockGroupApi(db.Dao).UpdateGroup(int(id), name) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已将分组 ID=%d 重命名为「%s」", id, name))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 重命名失败（可能 ID=%d 不存在）", id))
	}
	return nil
}

// 4. DeleteStockGroup 删除分组（级联删归属）
func handleDeleteStockGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "DeleteStockGroup", funcArguments)
	id := gjson.Get(funcArguments, "groupId").Int()
	if id <= 0 {
		finishWrite(ctx, funcArguments, "❌ 参数 groupId 不能为空。")
		return nil
	}
	if NewStockGroupApi(db.Dao).RemoveGroup(int(id)) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已删除分组 ID=%d（其下归属已一并清除，股票未被取消关注）", id))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 删除分组失败（可能 ID=%d 不存在）", id))
	}
	return nil
}

// 5. AddStockToGroup 股票加入分组（幂等，不触发关注）
func handleAddStockToGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AddStockToGroup", funcArguments)
	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())
	groupName := strings.TrimSpace(gjson.Get(funcArguments, "groupName").String())
	if stockCode == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCode 不能为空。")
		return nil
	}
	if groupName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 groupName 不能为空。")
		return nil
	}
	normalizedCode := normalizeStockCode(stockCode)
	added := addStockToGroupsByName(normalizedCode, groupName)
	if len(added) > 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已将 %s 加入分组「%s」", stockCode, groupName))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 加入分组失败：%s", stockCode))
	}
	return nil
}

// 6. RemoveStockFromGroup 股票移出分组
func handleRemoveStockFromGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "RemoveStockFromGroup", funcArguments)
	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())
	groupName := strings.TrimSpace(gjson.Get(funcArguments, "groupName").String())
	if stockCode == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCode 不能为空。")
		return nil
	}
	if groupName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 groupName 不能为空。")
		return nil
	}
	gid, ok := findGroupByName(groupName)
	if !ok {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 分组「%s」不存在", groupName))
		return nil
	}
	normalizedCode := normalizeStockCode(stockCode)
	if NewStockGroupApi(db.Dao).RemoveStockGroup(normalizedCode, groupName, gid) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已将 %s 从分组「%s」移出", stockCode, groupName))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 移出失败：%s（可能不在该分组中）", stockCode))
	}
	return nil
}

// 7. BatchMoveStocksToGroup 批量多股票入同一分组
func handleBatchMoveStocksToGroup(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "BatchMoveStocksToGroup", funcArguments)
	groupName := strings.TrimSpace(gjson.Get(funcArguments, "groupName").String())
	if groupName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 groupName 不能为空。")
		return nil
	}
	codesRaw := gjson.Get(funcArguments, "stockCodes").Array()
	if len(codesRaw) == 0 {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCodes 不能为空。")
		return nil
	}
	gid, err := findOrCreateGroup(groupName)
	if err != nil || gid <= 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 创建分组失败：%s", groupName))
		return nil
	}
	groupApi := NewStockGroupApi(db.Dao)
	ok, fail := 0, []string{}
	for _, c := range codesRaw {
		code := strings.TrimSpace(c.String())
		if code == "" {
			continue
		}
		if groupApi.AddStockGroup(gid, normalizeStockCode(code)) {
			ok++
		} else {
			fail = append(fail, code)
		}
	}
	content := fmt.Sprintf("✅ 已将 %d/%d 只股票加入分组「%s」", ok, len(codesRaw), groupName)
	if len(fail) > 0 {
		content += fmt.Sprintf("，失败：%s", strings.Join(fail, "、"))
	}
	finishWrite(ctx, funcArguments, content)
	return nil
}

// === 概念标签管理（P0）===

// 8. GetStockConcepts 获取概念列表（可按股票筛）
func handleGetStockConcepts(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetStockConcepts", funcArguments)
	conceptApi := NewStockConceptApi(db.Dao)
	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())

	type row struct {
		ConceptId   int    `md:"概念ID"`
		ConceptName string `md:"概念名称"`
		StockCode   string `md:"股票代码"`
	}
	var rows []row

	if stockCode != "" {
		list := conceptApi.GetStockConceptsByStockCode(normalizeStockCode(stockCode))
		for _, cs := range list {
			rows = append(rows, row{int(cs.ConceptId), cs.ConceptInfo.Name, cs.StockCode})
		}
	} else {
		list := conceptApi.GetAllStockConcepts()
		for _, cs := range list {
			rows = append(rows, row{int(cs.ConceptId), cs.ConceptInfo.Name, cs.StockCode})
		}
	}
	if len(rows) == 0 {
		finish(ctx, funcArguments, "暂无概念标签")
		return nil
	}
	finish(ctx, funcArguments, util.MarkdownTableWithTitle("概念标签列表", rows))
	return nil
}

// 9. CreateStockConcept 创建概念（幂等去重）
func handleCreateStockConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "CreateStockConcept", funcArguments)
	name := strings.TrimSpace(gjson.Get(funcArguments, "conceptName").String())
	if name == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptName 不能为空。")
		return nil
	}
	id, err := findOrCreateConcept(name)
	if err != nil || id <= 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 创建概念失败：%s", name))
		return nil
	}
	finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 概念「%s」已就绪（ID: %d）", name, id))
	return nil
}

// 10. UpdateStockConcept 重命名概念
func handleUpdateStockConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "UpdateStockConcept", funcArguments)
	id := gjson.Get(funcArguments, "conceptId").Int()
	name := strings.TrimSpace(gjson.Get(funcArguments, "conceptName").String())
	if id <= 0 || name == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptId 和 conceptName 均不能为空。")
		return nil
	}
	if NewStockConceptApi(db.Dao).UpdateConcept(int(id), name) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已将概念 ID=%d 重命名为「%s」", id, name))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 重命名失败（可能 ID=%d 不存在或名称已被占用）", id))
	}
	return nil
}

// 11. DeleteStockConcept 删除概念（级联删归属）
func handleDeleteStockConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "DeleteStockConcept", funcArguments)
	id := gjson.Get(funcArguments, "conceptId").Int()
	if id <= 0 {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptId 不能为空。")
		return nil
	}
	if NewStockConceptApi(db.Dao).RemoveConcept(int(id)) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已删除概念 ID=%d（其下归属已一并清除）", id))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 删除概念失败（可能 ID=%d 不存在）", id))
	}
	return nil
}

// 12. AddStockToConcept 给股票打概念标签（幂等，不触发关注）
func handleAddStockToConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AddStockToConcept", funcArguments)
	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())
	conceptName := strings.TrimSpace(gjson.Get(funcArguments, "conceptName").String())
	if stockCode == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCode 不能为空。")
		return nil
	}
	if conceptName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptName 不能为空。")
		return nil
	}
	normalizedCode := normalizeStockCode(stockCode)
	added := addStockToConceptsByName(normalizedCode, conceptName)
	if len(added) > 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已为 %s 打上概念「%s」", stockCode, conceptName))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 打标签失败：%s", stockCode))
	}
	return nil
}

// 13. RemoveStockFromConcept 移除股票的概念标签
func handleRemoveStockFromConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "RemoveStockFromConcept", funcArguments)
	stockCode := strings.TrimSpace(gjson.Get(funcArguments, "stockCode").String())
	conceptName := strings.TrimSpace(gjson.Get(funcArguments, "conceptName").String())
	if stockCode == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCode 不能为空。")
		return nil
	}
	if conceptName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptName 不能为空。")
		return nil
	}
	cid, ok := findConceptByName(conceptName)
	if !ok {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 概念「%s」不存在", conceptName))
		return nil
	}
	normalizedCode := normalizeStockCode(stockCode)
	if NewStockConceptApi(db.Dao).RemoveStockConcept(normalizedCode, conceptName, cid) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已移除 %s 的概念「%s」", stockCode, conceptName))
	} else {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 移除失败：%s（可能未打该标签）", stockCode))
	}
	return nil
}

// 14. BatchAddStocksToConcept 批量多股票打同一概念
func handleBatchAddStocksToConcept(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "BatchAddStocksToConcept", funcArguments)
	conceptName := strings.TrimSpace(gjson.Get(funcArguments, "conceptName").String())
	if conceptName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 conceptName 不能为空。")
		return nil
	}
	codesRaw := gjson.Get(funcArguments, "stockCodes").Array()
	if len(codesRaw) == 0 {
		finishWrite(ctx, funcArguments, "❌ 参数 stockCodes 不能为空。")
		return nil
	}
	cid, err := findOrCreateConcept(conceptName)
	if err != nil || cid <= 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 创建概念失败：%s", conceptName))
		return nil
	}
	conceptApi := NewStockConceptApi(db.Dao)
	ok, fail := 0, []string{}
	for _, c := range codesRaw {
		code := strings.TrimSpace(c.String())
		if code == "" {
			continue
		}
		if conceptApi.AddStockConcept(cid, normalizeStockCode(code)) {
			ok++
		} else {
			fail = append(fail, code)
		}
	}
	content := fmt.Sprintf("✅ 已为 %d/%d 只股票打上概念「%s」", ok, len(codesRaw), conceptName)
	if len(fail) > 0 {
		content += fmt.Sprintf("，失败：%s", strings.Join(fail, "、"))
	}
	finishWrite(ctx, funcArguments, content)
	return nil
}

// === 组合操作（P1）===

// 15. MergeStockConcepts 合并概念：源概念股票转移到目标概念，再删源概念
func handleMergeStockConcepts(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "MergeStockConcepts", funcArguments)
	srcName := strings.TrimSpace(gjson.Get(funcArguments, "sourceConceptName").String())
	tgtName := strings.TrimSpace(gjson.Get(funcArguments, "targetConceptName").String())
	if srcName == "" || tgtName == "" {
		finishWrite(ctx, funcArguments, "❌ 参数 sourceConceptName 和 targetConceptName 均不能为空。")
		return nil
	}
	srcId, ok := findConceptByName(srcName)
	if !ok {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 源概念「%s」不存在", srcName))
		return nil
	}
	// 源==目标（大小写无关）短路
	if strings.ToLower(srcName) == strings.ToLower(tgtName) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("ℹ️ 源概念与目标概念相同，无需合并"))
		return nil
	}
	tgtId, err := findOrCreateConcept(tgtName)
	if err != nil || tgtId <= 0 {
		finishWrite(ctx, funcArguments, fmt.Sprintf("❌ 创建目标概念失败：%s", tgtName))
		return nil
	}
	// 收集源概念下所有股票
	conceptApi := NewStockConceptApi(db.Dao)
	all := conceptApi.GetAllStockConcepts()
	var codes []string
	for _, cs := range all {
		if int(cs.ConceptId) == srcId {
			codes = append(codes, cs.StockCode)
		}
	}
	// 转移到目标概念（幂等）
	moved := 0
	for _, code := range codes {
		if conceptApi.AddStockConcept(tgtId, code) {
			moved++
		}
	}
	// 删除源概念（级联删归属）
	if !conceptApi.RemoveConcept(srcId) {
		finishWrite(ctx, funcArguments, fmt.Sprintf("⚠️ 已将 %d 只股票从「%s」转移到「%s」，但删除源概念失败：请手动删除", moved, srcName, tgtName))
		return nil
	}
	finishWrite(ctx, funcArguments, fmt.Sprintf("✅ 已将 %d 只股票从「%s」合并到「%s」，并删除源概念", moved, srcName, tgtName))
	return nil
}

// 16. ReorganizeStockGroups 批量重整多股票分组归属（可选先清旧归属）
func handleReorganizeStockGroups(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "ReorganizeStockGroups", funcArguments)
	clearExisting := gjson.Get(funcArguments, "clearExisting").Bool()
	assigns := gjson.Get(funcArguments, "assignments").Array()
	if len(assigns) == 0 {
		finishWrite(ctx, funcArguments, "❌ 参数 assignments 不能为空。")
		return nil
	}
	groupApi := NewStockGroupApi(db.Dao)
	// clearExisting 时预加载一次全量归属，按股票码索引
	var stocksByCode map[string][]int
	if clearExisting {
		stocksByCode = map[string][]int{}
		for _, gs := range groupApi.GetAllGroupStocks() {
			stocksByCode[gs.StockCode] = append(stocksByCode[gs.StockCode], int(gs.GroupId))
		}
	}
	ok, fail := 0, []string{}
	for _, a := range assigns {
		rawCode := strings.TrimSpace(a.Get("stockCode").String())
		gName := strings.TrimSpace(a.Get("groupName").String())
		if rawCode == "" || gName == "" {
			continue
		}
		normalizedCode := normalizeStockCode(rawCode)
		gid, err := findOrCreateGroup(gName)
		if err != nil || gid <= 0 {
			fail = append(fail, rawCode)
			continue
		}
		// 先清旧归属
		if clearExisting {
			for _, oldGid := range stocksByCode[normalizedCode] {
				if oldGid != gid {
					groupApi.RemoveStockGroup(normalizedCode, gName, oldGid)
				}
			}
		}
		if groupApi.AddStockGroup(gid, normalizedCode) {
			ok++
		} else {
			fail = append(fail, rawCode)
		}
	}
	clearLabel := "否"
	if clearExisting {
		clearLabel = "是"
	}
	content := fmt.Sprintf("✅ 已重整 %d/%d 只股票分组（清除旧归属：%s）", ok, len(assigns), clearLabel)
	if len(fail) > 0 {
		content += fmt.Sprintf("，失败：%s", strings.Join(fail, "、"))
	}
	finishWrite(ctx, funcArguments, content)
	return nil
}

func init() {
	registerToolHandler("GetStockGroups", handleGetStockGroups)
	registerToolHandler("CreateStockGroup", handleCreateStockGroup)
	registerToolHandler("UpdateStockGroup", handleUpdateStockGroup)
	registerToolHandler("DeleteStockGroup", handleDeleteStockGroup)
	registerToolHandler("AddStockToGroup", handleAddStockToGroup)
	registerToolHandler("RemoveStockFromGroup", handleRemoveStockFromGroup)
	registerToolHandler("BatchMoveStocksToGroup", handleBatchMoveStocksToGroup)
	registerToolHandler("GetStockConcepts", handleGetStockConcepts)
	registerToolHandler("CreateStockConcept", handleCreateStockConcept)
	registerToolHandler("UpdateStockConcept", handleUpdateStockConcept)
	registerToolHandler("DeleteStockConcept", handleDeleteStockConcept)
	registerToolHandler("AddStockToConcept", handleAddStockToConcept)
	registerToolHandler("RemoveStockFromConcept", handleRemoveStockFromConcept)
	registerToolHandler("BatchAddStocksToConcept", handleBatchAddStocksToConcept)
	registerToolHandler("MergeStockConcepts", handleMergeStockConcepts)
	registerToolHandler("ReorganizeStockGroups", handleReorganizeStockGroups)
}
