package data

import (
	"strings"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// TestNormalizeFundFlowParams 验证资金流向工具参数归一化（中英文别名）
func TestNormalizeFundFlowParams(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"industry", "industry"}, {"", "industry"}, {"板块", "industry"}, {"行业", "industry"},
		{"concept", "concept"}, {"概念", "concept"}, {"CONCEPT", "concept"},
		{"both", "both"}, {"全部", "both"},
	}
	for _, c := range cases {
		if got := normalizeFundFlowBoardType(c.in); got != c.want {
			t.Errorf("normalizeFundFlowBoardType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	dirCases := []struct {
		in   string
		want string
	}{
		{"inflow", "inflow"}, {"", "both"}, {"流入", "inflow"}, {"净流入", "inflow"},
		{"outflow", "outflow"}, {"流出", "outflow"},
		{"both", "both"}, {"双向", "both"},
	}
	for _, c := range dirCases {
		if got := normalizeFundFlowDirection(c.in); got != c.want {
			t.Errorf("normalizeFundFlowDirection(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSortBKConstituentStocks 验证成分股排序（字段归一化 + 升降序）
func TestSortBKConstituentStocks(t *testing.T) {
	stocks := []models.BKConstituentStock{
		{Code: "1", Name: "A", ChangePercent: 5.0, VolumeRatio: 2.0, TurnoverRate: 10.0, TotalMarketCap: 100, MainNetInflow: 50, MainNetInflowPct: 5.0},
		{Code: "2", Name: "B", ChangePercent: -3.0, VolumeRatio: 0.5, TurnoverRate: 30.0, TotalMarketCap: 300, MainNetInflow: -80, MainNetInflowPct: -8.0},
		{Code: "3", Name: "C", ChangePercent: 1.0, VolumeRatio: 1.5, TurnoverRate: 20.0, TotalMarketCap: 200, MainNetInflow: 30, MainNetInflowPct: 3.0},
	}

	// 涨跌幅降序：A(5) > C(1) > B(-3)
	s := append([]models.BKConstituentStock{}, stocks...)
	sortBKConstituentStocks(s, "changePercent", "desc")
	if s[0].Code != "1" || s[2].Code != "2" {
		t.Errorf("changePercent desc 顺序错误: %s,%s,%s", s[0].Code, s[1].Code, s[2].Code)
	}

	// 换手率升序：A(10) < C(20) < B(30)
	s = append([]models.BKConstituentStock{}, stocks...)
	sortBKConstituentStocks(s, "turnoverRate", "asc")
	if s[0].Code != "1" || s[2].Code != "2" {
		t.Errorf("turnoverRate asc 顺序错误: %s,%s,%s", s[0].Code, s[1].Code, s[2].Code)
	}

	// 中文别名"总市值"降序：B(300) > C(200) > A(100)
	s = append([]models.BKConstituentStock{}, stocks...)
	field, _ := normalizeBKConstituentSortKey("总市值")
	sortBKConstituentStocks(s, field, "desc")
	if s[0].Code != "2" || s[2].Code != "1" {
		t.Errorf("总市值 desc 顺序错误: %s,%s,%s", s[0].Code, s[1].Code, s[2].Code)
	}

	// 主力净流入占比降序：A(5) > C(3) > B(-8)
	s = append([]models.BKConstituentStock{}, stocks...)
	field, _ = normalizeBKConstituentSortKey("主力净流入占比")
	sortBKConstituentStocks(s, field, "desc")
	if s[0].Code != "1" || s[2].Code != "2" {
		t.Errorf("主力净流入占比 desc 顺序错误: %s,%s,%s", s[0].Code, s[1].Code, s[2].Code)
	}

	// 默认字段主力净流入降序：A(50) > C(30) > B(-80)
	s = append([]models.BKConstituentStock{}, stocks...)
	sortBKConstituentStocks(s, "", "desc")
	if s[0].Code != "1" || s[2].Code != "2" {
		t.Errorf("默认排序顺序错误: %s,%s,%s", s[0].Code, s[1].Code, s[2].Code)
	}
}

// TestFmtYiWan 验证金额自适应单位（项目约定：|v|≥1亿 → 亿，≥1万 → 万）
func TestFmtYiWan(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{1.5e9, "15.00亿"}, {75452522, "7545.25万"}, {105400000, "1.05亿"},
		{572438, "57.24万"}, {1234, "1234"}, {-2.5e8, "-2.50亿"},
	}
	for _, c := range cases {
		if got := fmtYiWan(c.in); got != c.want {
			t.Errorf("fmtYiWan(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGetBkFundFlowRankToMarkdownWithTempDB 用临时库验证资金榜 Markdown 输出（流入/流出榜 + 日期回退）
func TestGetBkFundFlowRankToMarkdownWithTempDB(t *testing.T) {
	// 自建临时库（主库可能被运行中 app WAL 锁定）
	tmpDir := t.TempDir()
	tmpDao, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: tmpDir + "/rank_test.db"}), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	// 备份主库句柄，测试完恢复并关闭临时库（Windows 下文件占用会阻塞 TempDir 清理）
	mainDao := db.Dao
	db.Dao = tmpDao
	defer func() {
		db.Dao = mainDao
		if sqlDB, err := tmpDao.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	if err := db.Dao.AutoMigrate(&models.BKFundFlow{}, &models.ConceptFundFlow{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	today := "2026-09-08"
	// 板块：流入 2 个 + 流出 1 个
	bkFlows := []models.BKFundFlow{
		{Code: "BK0001", Name: "测试板块A", NetInflow: 1500000000, SnapTime: today + " 14:30:00"},
		{Code: "BK0002", Name: "测试板块B", NetInflow: 500000000, SnapTime: today + " 14:30:00"},
		{Code: "BK0003", Name: "测试板块C", NetInflow: -300000000, SnapTime: today + " 14:30:00"},
	}
	if err := db.Dao.Create(&bkFlows).Error; err != nil {
		t.Fatalf("写入测试板块数据失败: %v", err)
	}
	conceptFlows := []models.ConceptFundFlow{
		{Code: "BK9001", Name: "测试概念A", NetInflow: 2000000000, SnapTime: today + " 14:30:00"},
		{Code: "BK9002", Name: "测试概念B", NetInflow: -100000000, SnapTime: today + " 14:30:00"},
	}
	if err := db.Dao.Create(&conceptFlows).Error; err != nil {
		t.Fatalf("写入测试概念数据失败: %v", err)
	}

	// 默认：行业板块 + 流入流出双向
	md := GetBkFundFlowRankToMarkdown("", today, "", 20)
	if !strings.Contains(md, "测试板块A") || !strings.Contains(md, "15.00亿") {
		t.Errorf("流入榜缺失，输出: %s", md)
	}
	if !strings.Contains(md, "测试板块C") || !strings.Contains(md, "-3.00亿") {
		t.Errorf("流出榜缺失，输出: %s", md)
	}
	if strings.Contains(md, "测试概念A") {
		t.Errorf("默认不应包含概念数据: %s", md)
	}
	t.Logf("行业板块双向榜:\n%s", md)

	// 概念 + 净流出
	md2 := GetBkFundFlowRankToMarkdown("concept", today, "outflow", 20)
	if !strings.Contains(md2, "测试概念B") || strings.Contains(md2, "测试概念A") {
		t.Errorf("概念流出榜结果异常: %s", md2)
	}

	// 指定无数据日期 → 提示
	md3 := GetBkFundFlowRankToMarkdown("industry", "2020-01-01", "inflow", 20)
	if !strings.Contains(md3, "无数据") {
		t.Errorf("无数据日期应返回提示: %s", md3)
	}
}

// TestGetBkConstituentStocksToMarkdownLive 实时验证成分股工具（名称解析 + 排序 + Markdown 输出）
func TestGetBkConstituentStocksToMarkdownLive(t *testing.T) {
	// 名称解析：银行 → BK0475，按涨跌幅降序取前 10
	md := GetBkConstituentStocksToMarkdown("银行", "changePercent", "desc", 10)
	if !strings.Contains(md, "银行") || !strings.Contains(md, "BK0475") {
		t.Fatalf("名称解析失败，输出: %s", md[:min(200, len(md))])
	}
	if !strings.Contains(md, "涨跌幅降序") {
		t.Errorf("排序标签缺失: %s", md[:min(200, len(md))])
	}
	t.Logf("银行成分股（涨跌幅降序 TOP10）:\n%s", md[:min(800, len(md))])

	// 代码直接输入 + 主力净流入占比升序
	md2 := GetBkConstituentStocksToMarkdown("BK0475", "mainNetInflowPct", "asc", 5)
	if !strings.Contains(md2, "主力净流入占比升序") {
		t.Errorf("占比升序标签缺失: %s", md2[:min(200, len(md2))])
	}

	// 概念名称解析：机器人概念（模糊包含匹配）
	md3 := GetBkConstituentStocksToMarkdown("机器人概念", "turnoverRate", "desc", 5)
	if strings.Contains(md3, "未能识别") {
		t.Logf("机器人概念未匹配（概念列表可能无该名称），输出: %s", md3[:min(200, len(md3))])
	} else {
		t.Logf("概念解析成功:\n%s", md3[:min(500, len(md3))])
	}

	// 无法识别的名称 → 友好提示
	md4 := GetBkConstituentStocksToMarkdown("不存在的板块xyz", "", "", 10)
	if !strings.Contains(md4, "未能识别") {
		t.Errorf("未知名称应返回提示，实际: %s", md4[:min(200, len(md4))])
	}
}
