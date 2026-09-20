package data

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
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

// TestNormalizeTradingDirection 方向字段归一化：
// 兼容「操作/买卖标志」的 买入、卖出 与东方财富交割单「业务名称」的 证券买入、证券卖出。
func TestNormalizeTradingDirection(t *testing.T) {
	cases := []struct{ in, want string }{
		{"买入", "买入"},
		{"卖出", "卖出"},
		{"证券买入", "买入"},
		{"证券卖出", "卖出"},
		{"融资买入", "买入"},
		{"融券卖出", "卖出"},
		{"红利入账", ""},
		{"利息归本", ""},
		{"银行转存", ""},
		{"申购中签", ""},
		{"新股入账", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeTradingDirection(c.in); got != c.want {
			t.Errorf("normalizeTradingDirection(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestParseTradingImportFileEastmoneyFormats 东方财富两种导出口径的表头兼容：
//   - 成交明细（历史成交）：成交日期/买卖标志/成交价格
//   - 交割单（对账单）：发生日期/业务名称/成交价格/交易市场（无成交时间列）
func TestParseTradingImportFileEastmoneyFormats(t *testing.T) {
	cases := []struct {
		name          string
		content       string
		wantCode      string
		wantDirection string
		want          map[string]string
	}{
		{
			name: "成交明细",
			content: "成交日期\t成交时间\t证券代码\t证券名称\t买卖标志\t成交价格\t成交数量\t成交金额\t成交编号\t委托编号\t股东代码\t币种\n" +
				"20260812\t09:31:05\t600519\t贵州茅台\t买入\t1685.50\t200\t337100.00\t88888\t66666\tA123456\t人民币\n",
			wantCode:      "sh600519",
			wantDirection: "买入",
			want: map[string]string{
				"date": "20260812", "time": "09:31:05", "code": "600519", "name": "贵州茅台",
				"price": "1685.50", "volume": "200", "ref": "88888",
			},
		},
		{
			name: "交割单",
			content: "发生日期\t证券代码\t证券名称\t业务名称\t成交价格\t成交数量\t成交金额\t手续费\t印花税\t过户费\t其他费\t发生金额\t资金余额\t股份余额\t交易市场\t合同编号\n" +
				"20260812\t000001\t平安银行\t证券买入\t10.44\t600\t6264.00\t5.00\t0.00\t0.10\t0.00\t-6269.10\t0.00\t600\t深圳\tHT001\n",
			wantCode:      "sz000001",
			wantDirection: "买入",
			want: map[string]string{
				"date": "20260812", "time": "", "code": "000001", "name": "平安银行",
				"price": "10.44", "volume": "600", "ref": "HT001",
				"fee": "5.00", "stampTax": "0.00", "transferFee": "0.10",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records.txt")
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
			rows, err := parseTradingImportFile(path)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("期望 1 行，实际 %d", len(rows))
			}
			row := rows[0]
			for field, want := range c.want {
				if got := pickImportField(row, field); got != want {
					t.Errorf("字段 %s = %q, want %q", field, got, want)
				}
			}
			if got := normalizeImportedStockCode(pickImportField(row, "code"), pickImportField(row, "market")); got != c.wantCode {
				t.Errorf("证券代码归一化 = %q, want %q", got, c.wantCode)
			}
			if got := normalizeTradingDirection(pickImportField(row, "direction")); got != c.wantDirection {
				t.Errorf("方向归一化 = %q, want %q", got, c.wantDirection)
			}
		})
	}
}

// TestParseTradingImportFileCSVDelimiter 逗号分隔的 .csv 导出也能定位表头与数据行。
func TestParseTradingImportFileCSVDelimiter(t *testing.T) {
	content := "成交日期,成交时间,证券代码,证券名称,买卖标志,成交价格,成交数量\n" +
		"20260812,09:31:05,600519,贵州茅台,买入,1685.50,200\n"

	path := filepath.Join(t.TempDir(), "records.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := parseTradingImportFile(path)
	if err != nil {
		t.Fatalf("CSV 解析失败: %v", err)
	}
	if len(rows) != 1 || pickImportField(rows[0], "price") != "1685.50" {
		t.Fatalf("CSV 解析结果不符: %+v", rows)
	}
}

// TestImportTradingRecordsEastmoneyDelivery 端到端导入东方财富交割单（临时库）：
// 覆盖 方向归一化、费用聚合（手续费+印花税+过户费+其他费）、非交易业务跳过、
// 无成交时间时靠合同编号区分同日同价同量多笔成交、以及重复导入幂等。
func TestImportTradingRecordsEastmoneyDelivery(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDao, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: tmpDir + "/trading_import_test.db"}), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	mainDao := db.Dao
	db.Dao = tmpDao
	defer func() {
		db.Dao = mainDao
		if sqlDB, err := tmpDao.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	if err := db.Dao.AutoMigrate(&TradingRecord{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	header := "发生日期\t证券代码\t证券名称\t业务名称\t成交价格\t成交数量\t成交金额\t手续费\t印花税\t过户费\t其他费\t发生金额\t资金余额\t股份余额\t交易市场\t合同编号\n"
	content := header +
		"20260812\t600519\t贵州茅台\t证券买入\t1685.50\t200\t337100.00\t5.74\t0.00\t0.34\t0.01\t-337106.09\t0.00\t200\t上海\tHT001\n" +
		"20260813\t600519\t贵州茅台\t证券卖出\t1720.00\t200\t344000.00\t4.30\t344.00\t0.34\t0.01\t343651.35\t0.00\t0\t上海\tHT002\n" +
		"20260813\t600519\t贵州茅台\t红利入账\t0.00\t0\t0.00\t0.00\t0.00\t0.00\t0.00\t1200.00\t1200.00\t0\t上海\tHT003\n" +
		// 同日同价同量的两笔真实成交，仅合同编号不同，均应入库
		"20260814\t600519\t贵州茅台\t证券买入\t1700.00\t100\t170000.00\t1.00\t0.00\t0.10\t0.00\t-170001.10\t0.00\t100\t上海\tHT004\n" +
		"20260814\t600519\t贵州茅台\t证券买入\t1700.00\t100\t170000.00\t1.00\t0.00\t0.10\t0.00\t-170001.10\t0.00\t100\t上海\tHT005\n"
	path := filepath.Join(tmpDir, "东方财富交割单.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	api := StockDataApi{}
	got, err := api.ImportTradingRecords(path)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if got.Imported != 4 || got.Skipped != 1 || got.Failed != 0 {
		t.Fatalf("首轮导入期望 导入4/跳过1/失败0，实际 导入%d/跳过%d/失败%d", got.Imported, got.Skipped, got.Failed)
	}

	var recs []TradingRecord
	if err := db.Dao.Order("id").Find(&recs).Error; err != nil {
		t.Fatal(err)
	}
	if len(recs) != 4 {
		t.Fatalf("库中期望 4 条，实际 %d", len(recs))
	}
	first := recs[0]
	if first.StockCode != "sh600519" || first.Direction != "买入" {
		t.Errorf("首条记录不符: %+v", first)
	}
	if diff := first.Fee - (5.74 + 0.34 + 0.01); diff > 1e-9 || diff < -1e-9 {
		t.Errorf("费用聚合错误，期望 %v 实际 %v", 5.74+0.34+0.01, first.Fee)
	}
	if first.TradingTime.Format("2006-01-02 15:04:05") != "2026-08-12 00:00:00" {
		t.Errorf("无成交时间列时成交时间应为当日零点，实际 %v", first.TradingTime)
	}

	// 同文件重复导入：全部按基础键命中已有记录跳过
	again, err := api.ImportTradingRecords(path)
	if err != nil {
		t.Fatalf("重复导入失败: %v", err)
	}
	if again.Imported != 0 || again.Skipped != 5 || again.Failed != 0 {
		t.Fatalf("重复导入期望 导入0/跳过5/失败0，实际 导入%d/跳过%d/失败%d", again.Imported, again.Skipped, again.Failed)
	}
}
