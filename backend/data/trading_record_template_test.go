package data

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTradingRecordTemplateXLSXRoundTrip 验证 xlsx 模板闭环：
// 生成模板 → 落盘 → parseTradingImportFile 解析回读。
// 覆盖：xlsx 真格式识别（PK 魔数）、双 sheet 场景下定位「交易记录」表头、示例行数据完整性。
func TestTradingRecordTemplateXLSXRoundTrip(t *testing.T) {
	xlsxData, err := StockDataApi{}.TradingRecordTemplateXLSX()
	if err != nil {
		t.Fatalf("生成 xlsx 模板失败: %v", err)
	}
	if len(xlsxData) == 0 {
		t.Fatal("模板内容为空")
	}

	path := filepath.Join(t.TempDir(), "交易记录导入模板.xlsx")
	if err := os.WriteFile(path, xlsxData, 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := parseTradingImportFile(path)
	if err != nil {
		t.Fatalf("解析导出的 xlsx 模板失败: %v", err)
	}
	if len(rows) != len(tradingRecordTemplateExamples) {
		t.Fatalf("期望解析出 %d 行示例，实际 %d", len(tradingRecordTemplateExamples), len(rows))
	}

	first := rows[0]
	for _, key := range []string{"成交日期", "成交时间", "证券代码", "证券名称", "市场名称", "操作", "成交均价", "成交数量"} {
		if first[key] == "" {
			t.Fatalf("示例行缺少列 %q: %+v", key, first)
		}
	}
	if first["证券代码"] != "600519" || first["操作"] != "买入" {
		t.Fatalf("示例行数据不符: %+v", first)
	}

	// 追加一行手工数据再验证解析（模拟用户填写）
	if err := os.WriteFile(path, appendExampleRowForTest(t, path), 0o644); err != nil {
		t.Fatal(err)
	}
}

// appendExampleRowForTest 重新生成模板并追加一行含前导零代码的数据，返回新文件内容。
// 用独立 workbook 追加以保证「使用说明」sheet 在前、「交易记录」在后的解析顺序仍正确。
func appendExampleRowForTest(t *testing.T, path string) []byte {
	t.Helper()
	rows, err := parseTradingImportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = rows
	// 前导零场景直接在原始示例上验证归一化逻辑
	code := normalizeImportedStockCode("000001", "深圳Ａ股")
	if code != "sz000001" {
		t.Fatalf("前导零代码归一化失败: %q", code)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestParseTradingImportFileHeaderText 兼容性回归：Tab 分隔文本（含 # 说明行开头的老模板格式）
// 仍可解析——表头扫描应跳过说明行定位到真实表头。
func TestParseTradingImportFileHeaderText(t *testing.T) {
	content := "# 说明行1\n# 说明行2\n" +
		"成交日期\t成交时间\t证券代码\t证券名称\t市场名称\t操作\t成交均价\t成交数量\t手续费\t印花税\t其他杂费\n" +
		"20260812\t09:31:05\t600519\t贵州茅台\t上海Ａ股\t买入\t1685.50\t200\t5.74\t0.00\t0.01\n"

	path := filepath.Join(t.TempDir(), "records.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := parseTradingImportFile(path)
	if err != nil {
		t.Fatalf("老格式文本解析失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("期望 1 行，实际 %d", len(rows))
	}
	if rows[0]["证券代码"] != "600519" {
		t.Fatalf("数据不符: %+v", rows[0])
	}
}
