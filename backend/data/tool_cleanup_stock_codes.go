package data

import (
	"fmt"
	"go-stock/backend/db"
	"strings"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/7/27
// @Desc 「清理不规范股票代码」AI 工具处理器：扫描 followed_stock 和 group_stock_info 表，
//       把不规范的 stock_code（后缀格式 600938.SH、纯数字 600938、大写 SH600938 等）
//       归一化为前缀小写格式（sh600938），合并重复记录。
// -----------------------------------------------------------------------------------

func init() {
	registerToolHandler("CleanupStockCodes", handleCleanupStockCodes)
}

// CleanupResult 清理结果统计。
type CleanupResult struct {
	FollowedFixed   int // followed_stock 归一化记录数
	FollowedDeleted int // followed_stock 合并删除重复数
	FollowedSkipped int // followed_stock 跳过数（已是规范格式）
	GroupFixed      int // group_stock_info 归一化记录数
	GroupDeleted    int // group_stock_info 合并删除重复数
	GroupSkipped    int // group_stock_info 跳过数（已是规范格式）
}

// CleanupStockCodesTable 清理 followed_stock 和 group_stock_info 表的不规范 stock_code。
// 导出函数，供 agent 端（data_tools_wrapper.go）调用。
// dryRun=true 时只扫描不修改数据库；dryRun=false 时执行实际清理。
func CleanupStockCodesTable(dryRun bool) CleanupResult {
	fixedF, deletedF, skippedF := cleanupFollowedStockTable(dryRun)
	fixedG, deletedG, skippedG := cleanupGroupStockInfoTable(dryRun)
	return CleanupResult{
		FollowedFixed:   fixedF,
		FollowedDeleted: deletedF,
		FollowedSkipped: skippedF,
		GroupFixed:      fixedG,
		GroupDeleted:    deletedG,
		GroupSkipped:    skippedG,
	}
}

// handleCleanupStockCodes 处理 CleanupStockCodes 工具调用。
// 参数 dryRun=true 时只扫描并返回预览报告，不修改数据库；dryRun=false（默认）执行实际清理。
func handleCleanupStockCodes(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "CleanupStockCodes", funcArguments)

	dryRun := gjson.Get(funcArguments, "dryRun").Bool() // 默认 false（执行清理）

	var lines []string
	if dryRun {
		lines = append(lines, "🔍 【预览模式】仅扫描不规范记录，不修改数据库。")
	} else {
		lines = append(lines, "🧹 【执行模式】开始清理不规范的股票代码...")
	}

	// === 清理 followed_stock 表 ===
	followedFixed, followedDeleted, followedSkipped := cleanupFollowedStockTable(dryRun)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("📋 followed_stock 表："))
	lines = append(lines, fmt.Sprintf("   - 归一化记录：%d 条", followedFixed))
	lines = append(lines, fmt.Sprintf("   - 合并删除重复：%d 条", followedDeleted))
	lines = append(lines, fmt.Sprintf("   - 跳过（已是规范格式）：%d 条", followedSkipped))

	// === 清理 group_stock_info 表 ===
	groupFixed, groupDeleted, groupSkipped := cleanupGroupStockInfoTable(dryRun)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("📋 group_stock_info 表："))
	lines = append(lines, fmt.Sprintf("   - 归一化记录：%d 条", groupFixed))
	lines = append(lines, fmt.Sprintf("   - 合并删除重复：%d 条", groupDeleted))
	lines = append(lines, fmt.Sprintf("   - 跳过（已是规范格式）：%d 条", groupSkipped))

	if dryRun {
		lines = append(lines, "")
		lines = append(lines, "💡 如需执行实际清理，请再次调用本工具并设置 dryRun=false。")
	} else {
		lines = append(lines, "")
		lines = append(lines, "✅ 清理完成。所有 stock_code 已统一为前缀小写格式。")
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

// cleanupFollowedStockTable 清理 followed_stock 表的不规范 stock_code。
// 返回：(归一化记录数, 合并删除重复数, 跳过数)
func cleanupFollowedStockTable(dryRun bool) (int, int, int) {
	type followedRow struct {
		StockCode string
		Name      string
		CostPrice float64
		Volume    int64
		IsDel     int
	}

	var rows []followedRow
	db.Dao.Raw("SELECT stock_code, name, cost_price, volume, is_del FROM followed_stock").Scan(&rows)

	fixed, deleted, skipped := 0, 0, 0

	for _, row := range rows {
		normalized := normalizeStockCode(row.StockCode)
		if normalized == row.StockCode {
			skipped++
			continue
		}

		if dryRun {
			fixed++
			continue
		}

		// 检查归一化后的 stock_code 是否已存在其他记录（重复）
		var existingCount int64
		db.Dao.Raw("SELECT COUNT(*) FROM followed_stock WHERE stock_code = ? AND stock_code <> ?", normalized, row.StockCode).Scan(&existingCount)

		if existingCount > 0 {
			// 重复：合并 cost_price/volume（取非零值），然后删除当前记录
			if row.CostPrice > 0 {
				db.Dao.Exec("UPDATE followed_stock SET cost_price = ? WHERE stock_code = ?", row.CostPrice, normalized)
			}
			if row.Volume > 0 {
				db.Dao.Exec("UPDATE followed_stock SET volume = ? WHERE stock_code = ?", row.Volume, normalized)
			}
			db.Dao.Exec("DELETE FROM followed_stock WHERE stock_code = ?", row.StockCode)
			deleted++
		} else {
			// 不重复：直接 UPDATE stock_code 为归一化后的值
			db.Dao.Exec("UPDATE followed_stock SET stock_code = ? WHERE stock_code = ?", normalized, row.StockCode)
			fixed++
		}
	}

	return fixed, deleted, skipped
}

// cleanupGroupStockInfoTable 清理 group_stock_info 表的不规范 stock_code。
// 返回：(归一化记录数, 合并删除重复数, 跳过数)
func cleanupGroupStockInfoTable(dryRun bool) (int, int, int) {
	type groupRow struct {
		ID        int64
		StockCode string
		GroupID   int
	}

	var rows []groupRow
	db.Dao.Raw("SELECT id, stock_code, group_id FROM group_stock_info").Scan(&rows)

	fixed, deleted, skipped := 0, 0, 0

	for _, row := range rows {
		normalized := normalizeStockCode(row.StockCode)
		if normalized == row.StockCode {
			skipped++
			continue
		}

		if dryRun {
			fixed++
			continue
		}

		// 检查归一化后的 (stock_code, group_id) 是否已存在其他记录（重复）
		var existingCount int64
		db.Dao.Raw("SELECT COUNT(*) FROM group_stock_info WHERE stock_code = ? AND group_id = ? AND stock_code <> ?", normalized, row.GroupID, row.StockCode).Scan(&existingCount)

		if existingCount > 0 {
			// 重复：直接删除当前记录（前缀格式已存在）
			db.Dao.Exec("DELETE FROM group_stock_info WHERE id = ?", row.ID)
			deleted++
		} else {
			// 不重复：直接 UPDATE stock_code 为归一化后的值
			db.Dao.Exec("UPDATE group_stock_info SET stock_code = ? WHERE id = ?", normalized, row.ID)
			fixed++
		}
	}

	return fixed, deleted, skipped
}
