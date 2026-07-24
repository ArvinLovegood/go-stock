package tools

import (
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"strings"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/7/11
// @Desc Eino 路径（Path B）分组与概念标签管理的共享 helper。
//       与 data 包的 normalizeStockCode/findGroupByName/findConceptByName 逻辑一致，
//       因 data 包版本为未导出函数，此处独立实现（tools 包调用 data 包的 API）。
// -----------------------------------------------------------------------------------

// normalizeStockCodeEino 归一化股票代码：us/US 前缀转 gb_+小写；其余小写。
func normalizeStockCodeEino(stockCode string) string {
	if strings.HasPrefix(stockCode, "us") {
		return "gb_" + strings.ToLower(strings.Replace(stockCode, "us", "", 1))
	}
	if strings.HasPrefix(stockCode, "US") {
		return "gb_" + strings.ToLower(strings.Replace(stockCode, "US", "", 1))
	}
	return strings.ToLower(stockCode)
}

// findGroupByNameEino 按名称查找分组（大小写无关），不创建。
func findGroupByNameEino(name string) (int, bool) {
	lower := strings.ToLower(name)
	for _, g := range data.NewStockGroupApi(db.Dao).GetGroupList() {
		if strings.ToLower(g.Name) == lower {
			return int(g.ID), true
		}
	}
	return 0, false
}

// findConceptByNameEino 按名称查找概念（大小写无关），不创建。
func findConceptByNameEino(name string) (int, bool) {
	lower := strings.ToLower(name)
	for _, c := range data.NewStockConceptApi(db.Dao).GetConceptList() {
		if strings.ToLower(c.Name) == lower {
			return int(c.ID), true
		}
	}
	return 0, false
}

// findOrCreateGroupEino 查找分组，不存在则创建，返回分组 ID。
func findOrCreateGroupEino(name string) (int, error) {
	if id, ok := findGroupByNameEino(name); ok {
		return id, nil
	}
	groupApi := data.NewStockGroupApi(db.Dao)
	if !groupApi.AddGroup(data.Group{Name: name, Sort: 1}) {
		return 0, fmt.Errorf("创建分组失败：%s", name)
	}
	if id, ok := findGroupByNameEino(name); ok {
		return id, nil
	}
	return 0, fmt.Errorf("创建分组后未找到：%s", name)
}

// findOrCreateConceptEino 查找概念，不存在则创建（AddConcept 幂等去重），返回概念 ID。
func findOrCreateConceptEino(name string) (int, error) {
	if id, ok := findConceptByNameEino(name); ok {
		return id, nil
	}
	conceptApi := data.NewStockConceptApi(db.Dao)
	if !conceptApi.AddConcept(data.Concept{Name: name, Sort: 1}) {
		return 0, fmt.Errorf("创建概念失败：%s", name)
	}
	if id, ok := findConceptByNameEino(name); ok {
		return id, nil
	}
	return 0, fmt.Errorf("创建概念后未找到：%s", name)
}

// splitNamesEino 按英文逗号、中文逗号、中文顿号切分名称（与 data 包 splitNames 逻辑一致）。
func splitNamesEino(raw string) []string {
	s := strings.ReplaceAll(raw, "，", ",")
	s = strings.ReplaceAll(s, "、", ",")
	return strings.Split(s, ",")
}

// groupConceptAddStock 把股票加入分组或概念（kind="group"|"concept"），按名查找/创建，幂等。
func groupConceptAddStock(args, kind string) (string, error) {
	defer data.EmitStockDataChanged()
	stockCode := strings.TrimSpace(gjson.Get(args, "stockCode").String())
	var name string
	if kind == "group" {
		name = strings.TrimSpace(gjson.Get(args, "groupName").String())
	} else {
		name = strings.TrimSpace(gjson.Get(args, "conceptName").String())
	}
	if stockCode == "" {
		return "❌ 参数 stockCode 不能为空。", nil
	}
	if name == "" {
		return "❌ 参数名称不能为空。", nil
	}
	normalized := normalizeStockCodeEino(stockCode)
	if kind == "group" {
		gid, err := findOrCreateGroupEino(name)
		if err != nil || gid <= 0 {
			return fmt.Sprintf("❌ 创建分组失败：%s", name), nil
		}
		if data.NewStockGroupApi(db.Dao).AddStockGroup(gid, normalized) {
			return fmt.Sprintf("✅ 已将 %s 加入分组「%s」", stockCode, name), nil
		}
		return fmt.Sprintf("⚠️ 加入分组失败：%s", stockCode), nil
	}
	cid, err := findOrCreateConceptEino(name)
	if err != nil || cid <= 0 {
		return fmt.Sprintf("❌ 创建概念失败：%s", name), nil
	}
	if data.NewStockConceptApi(db.Dao).AddStockConcept(cid, normalized) {
		return fmt.Sprintf("✅ 已为 %s 打上概念「%s」", stockCode, name), nil
	}
	return fmt.Sprintf("⚠️ 打标签失败：%s", stockCode), nil
}

// groupConceptRemoveStock 把股票移出分组或概念（kind="group"|"concept"），按名查找（不创建）。
func groupConceptRemoveStock(args, kind string) (string, error) {
	defer data.EmitStockDataChanged()
	stockCode := strings.TrimSpace(gjson.Get(args, "stockCode").String())
	var name string
	if kind == "group" {
		name = strings.TrimSpace(gjson.Get(args, "groupName").String())
	} else {
		name = strings.TrimSpace(gjson.Get(args, "conceptName").String())
	}
	if stockCode == "" {
		return "❌ 参数 stockCode 不能为空。", nil
	}
	if name == "" {
		return "❌ 参数名称不能为空。", nil
	}
	normalized := normalizeStockCodeEino(stockCode)
	if kind == "group" {
		gid, ok := findGroupByNameEino(name)
		if !ok {
			return fmt.Sprintf("❌ 分组「%s」不存在", name), nil
		}
		if data.NewStockGroupApi(db.Dao).RemoveStockGroup(normalized, name, gid) {
			return fmt.Sprintf("✅ 已将 %s 从分组「%s」移出", stockCode, name), nil
		}
		return fmt.Sprintf("⚠️ 移出失败：%s（可能不在该分组中）", stockCode), nil
	}
	cid, ok := findConceptByNameEino(name)
	if !ok {
		return fmt.Sprintf("❌ 概念「%s」不存在", name), nil
	}
	if data.NewStockConceptApi(db.Dao).RemoveStockConcept(normalized, name, cid) {
		return fmt.Sprintf("✅ 已移除 %s 的概念「%s」", stockCode, name), nil
	}
	return fmt.Sprintf("⚠️ 移除失败：%s（可能未打该标签）", stockCode), nil
}

// batchAddStocks 批量把多只股票加入同一分组或概念（kind="group"|"concept"）。
func batchAddStocks(args, kind string) (string, error) {
	defer data.EmitStockDataChanged()
	var name string
	if kind == "group" {
		name = strings.TrimSpace(gjson.Get(args, "groupName").String())
	} else {
		name = strings.TrimSpace(gjson.Get(args, "conceptName").String())
	}
	if name == "" {
		return "❌ 参数名称不能为空。", nil
	}
	codesRaw := gjson.Get(args, "stockCodes").Array()
	if len(codesRaw) == 0 {
		return "❌ 参数 stockCodes 不能为空。", nil
	}
	var entityID int
	var err error
	if kind == "group" {
		entityID, err = findOrCreateGroupEino(name)
	} else {
		entityID, err = findOrCreateConceptEino(name)
	}
	if err != nil || entityID <= 0 {
		return fmt.Sprintf("❌ 创建失败：%s", name), nil
	}
	ok, fail := 0, []string{}
	if kind == "group" {
		groupApi := data.NewStockGroupApi(db.Dao)
		for _, c := range codesRaw {
			code := strings.TrimSpace(c.String())
			if code == "" {
				continue
			}
			if groupApi.AddStockGroup(entityID, normalizeStockCodeEino(code)) {
				ok++
			} else {
				fail = append(fail, code)
			}
		}
	} else {
		conceptApi := data.NewStockConceptApi(db.Dao)
		for _, c := range codesRaw {
			code := strings.TrimSpace(c.String())
			if code == "" {
				continue
			}
			if conceptApi.AddStockConcept(entityID, normalizeStockCodeEino(code)) {
				ok++
			} else {
				fail = append(fail, code)
			}
		}
	}
	label := "分组"
	if kind == "concept" {
		label = "概念"
	}
	content := fmt.Sprintf("✅ 已将 %d/%d 只股票加入%s「%s」", ok, len(codesRaw), label, name)
	if len(fail) > 0 {
		content += fmt.Sprintf("，失败：%s", strings.Join(fail, "、"))
	}
	return content, nil
}
