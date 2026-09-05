package data

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// newBytesReader 避免与包内既有命名冲突的小包装。
func newBytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

// TestBuildTableXLSX 验证通用表格导出：
// 1. 二级表头场景：父表头合并、子表头占第二行、数据从第三行开始
// 2. 一级表头场景：表头一行、数据从第二行开始
// 3. 空列/空数据报错
func TestBuildTableXLSX(t *testing.T) {
	api := StockDataApi{}

	// 场景 1：二级表头
	table := ExportTableData{
		SheetName: "指标选股",
		Columns: []ExportTableColumn{
			{Title: "代码", Key: "SECURITY_CODE"},
			{Title: "名称", Key: "SECURITY_SHORT_NAME"},
			{
				Title: "市盈率[PE]",
				Children: []ExportTableColumn{
					{Title: "2026-08-30", Key: "PE_1"},
					{Title: "2026-09-04", Key: "PE_2"},
				},
			},
		},
		Rows: []map[string]interface{}{
			{"SECURITY_CODE": "600519", "SECURITY_SHORT_NAME": "贵州茅台", "PE_1": 20.5, "PE_2": 21.1},
			{"SECURITY_CODE": "300750", "SECURITY_SHORT_NAME": "宁德时代", "PE_1": 18.0, "PE_2": 19.2},
		},
	}
	data, err := api.BuildTableXLSX(table)
	if err != nil {
		t.Fatalf("生成 xlsx 失败: %v", err)
	}

	f, err := excelize.OpenReader(newBytesReader(data))
	if err != nil {
		t.Fatalf("回读 xlsx 失败: %v", err)
	}
	defer f.Close()

	// 父表头合并 + 一级表头纵向合并（C1:D1 横向，A1:A2 纵向）
	if merged, err := f.GetMergeCells("指标选股"); err != nil || len(merged) < 2 {
		t.Fatalf("期望至少 2 个合并单元格（父表头横向+一级表头纵向），实际 %d (err=%v)", len(merged), err)
	}
	// 子表头在第二行
	if v, _ := f.GetCellValue("指标选股", "C2"); v != "2026-08-30" {
		t.Fatalf("子表头 C2 应为 2026-08-30，实际 %q", v)
	}
	// 数据从第三行开始
	if v, _ := f.GetCellValue("指标选股", "A3"); v != "600519" {
		t.Fatalf("数据 A3 应为 600519，实际 %q", v)
	}
	if v, _ := f.GetCellValue("指标选股", "D4"); v != "19.2" {
		t.Fatalf("数据 D4 应为 19.2，实际 %q", v)
	}

	// 场景 2：一级表头
	table2 := ExportTableData{
		Columns: []ExportTableColumn{{Title: "代码", Key: "code"}},
		Rows:    []map[string]interface{}{{"code": "000001"}},
	}
	data2, err := api.BuildTableXLSX(table2)
	if err != nil {
		t.Fatalf("一级表头生成失败: %v", err)
	}
	f2, err := excelize.OpenReader(newBytesReader(data2))
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	if v, _ := f2.GetCellValue("选股结果", "A1"); v != "代码" {
		t.Fatalf("默认 sheet 名或表头不符: %q", v)
	}
	if v, _ := f2.GetCellValue("选股结果", "A2"); v != "000001" {
		t.Fatalf("一级表头数据应从第二行开始，实际 %q", v)
	}

	// 场景 3：空数据报错
	if _, err := api.BuildTableXLSX(ExportTableData{Columns: []ExportTableColumn{{Title: "a", Key: "a"}}}); err == nil {
		t.Fatal("空 rows 应报错")
	}
	if _, err := api.BuildTableXLSX(ExportTableData{Rows: []map[string]interface{}{{"a": 1}}}); err == nil {
		t.Fatal("空 columns 应报错")
	}
}
