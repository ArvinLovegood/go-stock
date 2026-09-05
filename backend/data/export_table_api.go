package data

// export_table_api.go — 通用表格 Excel 导出。
//
// 供前端把「指标选股」「形态选股」等表格数据（含二级表头）导出为 .xlsx：
// 前端传列定义（ExportTableColumn）+ 行数据（map），本模块用 excelize 生成
// 表头（加粗灰底，二级表头合并单元格）+ 数据行，App 层弹保存框落盘。
//
// Wails 绑定规则：所有 struct 一律具名（见项目约定，匿名 struct 会导致
// wailsjs/go/models.ts 生成非法代码）。

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// ExportTableColumn 导出表格列定义。
// Children 非空时为二级表头（父表头跨列合并，子表头占第二行）。
type ExportTableColumn struct {
	Title    string              `json:"title"`
	Key      string              `json:"key"`
	Children []ExportTableColumn `json:"children,omitempty"`
}

// ExportTableData 导出表格数据。
type ExportTableData struct {
	SheetName string                   `json:"sheetName"` // 工作表名，空则用"选股结果"
	Columns   []ExportTableColumn      `json:"columns"`   // 列定义（含二级表头）
	Rows      []map[string]interface{} `json:"rows"`      // 行数据，按列 Key 取值
}

// leafColumns 展开二级表头为叶子列列表（按出现顺序）。
func leafColumns(cols []ExportTableColumn) []ExportTableColumn {
	var leaves []ExportTableColumn
	for _, c := range cols {
		if len(c.Children) > 0 {
			leaves = append(leaves, leafColumns(c.Children)...)
		} else {
			leaves = append(leaves, c)
		}
	}
	return leaves
}

// BuildTableXLSX 将表格数据生成为 xlsx 字节。
// 二级表头：第一行父表头（跨子列合并居中），第二行子表头；一级表头只占第一行，
// 数据从二级表头场景的第三行、一级表头场景的第二行开始。
func (receiver StockDataApi) BuildTableXLSX(table ExportTableData) ([]byte, error) {
	if len(table.Columns) == 0 {
		return nil, fmt.Errorf("导出失败：列定义为空")
	}
	if len(table.Rows) == 0 {
		return nil, fmt.Errorf("导出失败：没有可导出的数据")
	}

	sheet := table.SheetName
	if sheet == "" {
		sheet = "选股结果"
	}

	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	leaves := leafColumns(table.Columns)
	hasChildren := len(leaves) != len(table.Columns)

	// 表头样式：加粗 + 灰底 + 居中
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9D9D9"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}

	// 写表头
	headerRow2 := 2 // 二级表头的子表头行
	col := 0
	for _, c := range table.Columns {
		col++
		if len(c.Children) > 0 {
			startCol, _ := excelize.ColumnNumberToName(col)
			endCol, _ := excelize.ColumnNumberToName(col + len(leafColumns(c.Children)) - 1)
			if err := f.SetCellValue(sheet, startCol+"1", c.Title); err != nil {
				return nil, err
			}
			if startCol != endCol {
				if err := f.MergeCell(sheet, startCol+"1", endCol+"1"); err != nil {
					return nil, err
				}
			}
			for _, child := range c.Children {
				cellName, _ := excelize.CoordinatesToCellName(col, headerRow2)
				if err := f.SetCellValue(sheet, cellName, child.Title); err != nil {
					return nil, err
				}
				col++
			}
			col--
		} else {
			cellName, _ := excelize.CoordinatesToCellName(col, 1)
			if err := f.SetCellValue(sheet, cellName, c.Title); err != nil {
				return nil, err
			}
			// 一级表头在有二级表头的表中纵向合并两行
			if hasChildren {
				bottomCell, _ := excelize.CoordinatesToCellName(col, headerRow2)
				if err := f.MergeCell(sheet, cellName, bottomCell); err != nil {
					return nil, err
				}
			}
		}
	}

	// 表头样式（含二级表头行）
	endHeaderCol, _ := excelize.ColumnNumberToName(len(leaves))
	headerEndRow := 1
	if hasChildren {
		headerEndRow = 2
	}
	if err := f.SetCellStyle(sheet, "A1", endHeaderCol+fmt.Sprintf("%d", headerEndRow), headerStyle); err != nil {
		return nil, err
	}

	// 写数据行
	dataStartRow := headerEndRow + 1
	for r, row := range table.Rows {
		for cIdx, leaf := range leaves {
			cellName, _ := excelize.CoordinatesToCellName(cIdx+1, dataStartRow+r)
			v, ok := row[leaf.Key]
			if !ok || v == nil {
				continue
			}
			if err := f.SetCellValue(sheet, cellName, v); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
