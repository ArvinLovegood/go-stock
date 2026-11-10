package data

import (
	"fmt"
	"os"
	"testing"

	"go-stock/backend/db"
)

func init() {
	if os.Getenv("TEST_DB_SKIP_INIT") == "1" {
		return
	}
	db.Init("../../data/stock.db")
}

func TestTdxKLineApi_GetCallAuction(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetCallAuction("600519.SH", 0, 50)
	if data == nil {
		t.Fatal("GetCallAuction returned nil")
	}
	t.Logf("集合竞价数据条数: %d", len(*data))
	if len(*data) > 0 {
		first := (*data)[0]
		t.Logf("首条: time=%s price=%s matched=%s unmatched=%s flag=%s",
			first.Time, first.Price, first.Matched, first.Unmatched, first.Flag)
		last := (*data)[len(*data)-1]
		t.Logf("末条: time=%s price=%s matched=%s unmatched=%s flag=%s",
			last.Time, last.Price, last.Matched, last.Unmatched, last.Flag)
	}
}

func TestTdxKLineApi_GetCallAuctionAuto(t *testing.T) {
	api := NewTdxKLineApi()
	tests := []struct {
		code  string
		label string
	}{
		{"600519.SH", "A股-贵州茅台"},
		{"02202.HK", "港股-万科企业"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			list := api.GetCallAuctionAuto(tt.code, 0, 50)
			if list == nil {
				t.Errorf("[%s] GetCallAuctionAuto returned nil", tt.label)
				return
			}
			t.Logf("[%s] 集合竞价明细条数: %d", tt.label, len(*list))
			if len(*list) > 0 {
				first := (*list)[0]
				t.Logf("  首条: time=%s price=%s matched=%s unmatched=%s flag=%s",
					first.Time, first.Price, first.Matched, first.Unmatched, first.Flag)
				last := (*list)[len(*list)-1]
				t.Logf("  末条: time=%s price=%s matched=%s unmatched=%s flag=%s",
					last.Time, last.Price, last.Matched, last.Unmatched, last.Flag)
			} else {
				t.Logf("  [%s] 返回空（可能非竞价时段或 MAC 主服务器不支持 market=3/4 竞价）", tt.label)
			}
		})
	}
}

func TestTdxKLineApi_GetKLineData(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetKLineData("600519.SH", "101", 10)
	if data == nil {
		t.Fatal("GetKLineData returned nil")
	}
	t.Logf("日K数据条数: %d", len(*data))
	if len(*data) > 0 {
		first := (*data)[0]
		t.Logf("首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
			first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
	}
}

func TestTdxKLineApi_AllPeriods(t *testing.T) {
	api := NewTdxKLineApi()
	periods := []struct {
		klt   string
		label string
	}{
		{"1", "1分钟"}, {"5", "5分钟"}, {"10", "10分钟(聚合)"},
		{"15", "15分钟"}, {"30", "30分钟"}, {"60", "60分钟"},
		{"120", "120分钟(聚合)"}, {"101", "日线"}, {"102", "周线"},
		{"103", "月线"}, {"104", "季线"}, {"105", "半年线(聚合)"},
		{"106", "年线"},
	}
	for _, p := range periods {
		t.Run(p.label, func(t *testing.T) {
			data := api.GetKLineData("600519.SH", p.klt, 5)
			if data == nil {
				t.Errorf("[%s] klt=%s returned nil", p.label, p.klt)
				return
			}
			count := len(*data)
			t.Logf("[%s] klt=%s 条数=%d", p.label, p.klt, count)
			if count > 0 {
				last := (*data)[count-1]
				t.Logf("  末条: day=%s open=%s close=%s high=%s low=%s vol=%s",
					last.Day, last.Open, last.Close, last.High, last.Low, last.Volume)
			}
		})
	}
}

func TestTdxKLineApi_GetF10Data(t *testing.T) {
	api := NewTdxKLineApi()
	bundle := api.GetF10Data("600519.SH")
	if bundle == nil {
		t.Fatal("GetF10Data returned nil")
	}
	t.Logf("公司信息分类数: %d", len(bundle.Sections))
	for _, s := range bundle.Sections {
		t.Logf("  分类: %s (内容长度: %d)", s.Name, len(s.Content))
		//t.Logf("  内容: %s", s.Content)
	}
	if bundle.Finance != nil {
		f := bundle.Finance
		t.Logf("财务信息: code=%s eps=%.4f netAssetsPerShare=%.4f totalShares=%.2f ipoDate=%s",
			f.Code, f.EPS, f.NetAssetsPerShare, f.TotalShares, f.IPODate)
	}
	t.Logf("除权除息记录数: %d", len(bundle.XDXR))
	if len(bundle.XDXR) > 0 {
		last := bundle.XDXR[len(bundle.XDXR)-1]
		t.Logf("  最近除权除息: date=%s name=%s", last.Date, last.Name)
	}

}

func TestTdxKLineApi_GetFinanceInfo(t *testing.T) {
	api := NewTdxKLineApi()
	f := api.GetFinanceInfo("600519.SH")
	if f == nil {
		t.Fatal("GetFinanceInfo returned nil")
	}
	t.Logf("code=%s eps=%.4f netAssetsPerShare=%.4f totalShares=%.2f floatShares=%.2f ipoDate=%s updatedDate=%s",
		f.Code, f.EPS, f.NetAssetsPerShare, f.TotalShares, f.FloatShares, f.IPODate, f.UpdatedDate)
	t.Logf("totalAssets=%.2f totalEquity=%.2f operatingRevenue=%.2f netProfit=%.2f shareholderCount=%.0f",
		f.TotalAssets, f.TotalEquity, f.OperatingRevenue, f.NetProfit, f.ShareholderCount)
}

func TestTdxKLineApi_GetXDXRInfo(t *testing.T) {
	api := NewTdxKLineApi()
	items := api.GetXDXRInfo("600519.SH")
	if items == nil {
		t.Fatal("GetXDXRInfo returned nil")
	}
	t.Logf("除权除息记录数: %d", len(*items))
	if len(*items) > 0 {
		for i := len(*items) - 3; i < len(*items); i++ {
			if i < 0 {
				continue
			}
			x := (*items)[i]
			fh := "-"
			if x.Fenhong != nil {
				fh = fmt.Sprintf("%.4f", *x.Fenhong)
			}
			t.Logf("  [%d] date=%s name=%s fenhong=%s", i, x.Date, x.Name, fh)
		}
	}
}

func TestTdxKLineApi_GetF10CategoryList(t *testing.T) {
	api := NewTdxKLineApi()
	cats := api.GetF10CategoryList("600519.SH")
	if cats == nil {
		t.Fatal("GetF10CategoryList returned nil")
	}
	t.Logf("分类数量: %d", len(*cats))
	for i, c := range *cats {
		t.Logf("  [%d] %s (filename=%s)", i+1, c.Name, c.Filename)
	}
}

func TestTdxKLineApi_GetMACKLineData(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetMACKLineData("600519.SH", "101", 10)
	if data == nil {
		t.Fatal("GetMACKLineData returned nil")
	}
	t.Logf("MAC日K数据条数: %d", len(*data))
	if len(*data) > 0 {
		first := (*data)[0]
		t.Logf("首条: day=%s open=%s close=%s high=%s low=%s vol=%s amount=%s turnover=%s changepct=%s",
			first.Day, first.Open, first.Close, first.High, first.Low, first.Volume, first.Amount, first.TurnoverRate, first.ChangePercent)
		last := (*data)[len(*data)-1]
		t.Logf("末条: day=%s open=%s close=%s high=%s low=%s vol=%s amount=%s turnover=%s changepct=%s",
			last.Day, last.Open, last.Close, last.High, last.Low, last.Volume, last.Amount, last.TurnoverRate, last.ChangePercent)
	}
}

func TestTdxKLineApi_GetMACKLineData_AllPeriods(t *testing.T) {
	api := NewTdxKLineApi()
	periods := []struct {
		klt   string
		label string
	}{
		{"1", "1分钟"}, {"5", "5分钟"}, {"15", "15分钟"},
		{"30", "30分钟"}, {"60", "60分钟"}, {"101", "日线"},
		{"102", "周线"}, {"103", "月线"}, {"104", "季线"}, {"106", "年线"},
	}
	for _, p := range periods {
		t.Run(p.label, func(t *testing.T) {
			data := api.GetMACKLineData("600519.SH", p.klt, 5)
			if data == nil {
				t.Errorf("[%s] klt=%s returned nil", p.label, p.klt)
				return
			}
			count := len(*data)
			t.Logf("[%s] klt=%s 条数=%d", p.label, p.klt, count)
			if count > 0 {
				last := (*data)[count-1]
				t.Logf("  末条: day=%s open=%s close=%s high=%s low=%s vol=%s turnover=%s",
					last.Day, last.Open, last.Close, last.High, last.Low, last.Volume, last.TurnoverRate)
			}
		})
	}
}

func TestTdxKLineApi_GetMACKLineData_SZ(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetMACKLineData("000001.SZ", "101", 10)
	if data == nil {
		t.Fatal("GetMACKLineData returned nil for SZ stock")
	}
	t.Logf("平安银行 MAC日K条数: %d", len(*data))
	if len(*data) > 0 {
		last := (*data)[len(*data)-1]
		t.Logf("末条: day=%s open=%s close=%s high=%s low=%s vol=%s turnover=%s",
			last.Day, last.Open, last.Close, last.High, last.Low, last.Volume, last.TurnoverRate)
	}
}

func TestTdxKLineApi_MinuteLineCompare(t *testing.T) {
	api := NewTdxKLineApi()
	minuteKlts := []struct {
		klt   string
		label string
	}{
		{"1", "1分钟"}, {"5", "5分钟"}, {"15", "15分钟"},
		{"30", "30分钟"}, {"60", "60分钟"},
	}
	for _, p := range minuteKlts {
		t.Run(p.label+"_主行情", func(t *testing.T) {
			data := api.GetKLineData("600519.SH", p.klt, 5)
			if data == nil || len(*data) == 0 {
				t.Errorf("[%s 主行情] 返回空", p.label)
				return
			}
			for i, item := range *data {
				t.Logf("  [%d] day=%s open=%s close=%s high=%s low=%s vol=%s", i, item.Day, item.Open, item.Close, item.High, item.Low, item.Volume)
			}
		})
		t.Run(p.label+"_MAC", func(t *testing.T) {
			data := api.GetMACKLineData("600519.SH", p.klt, 5)
			if data == nil || len(*data) == 0 {
				t.Errorf("[%s MAC] 返回空", p.label)
				return
			}
			for i, item := range *data {
				t.Logf("  [%d] day=%s open=%s close=%s high=%s low=%s vol=%s turnover=%s", i, item.Day, item.Open, item.Close, item.High, item.Low, item.Volume, item.TurnoverRate)
			}
		})
	}
}

func TestTdxKLineApi_MACKLineHKUS(t *testing.T) {
	api := NewTdxKLineApi()
	tests := []struct {
		code  string
		label string
	}{
		{"00700.HK", "腾讯控股"},
		{"AAPL.US", "苹果"},
		{"TSLA.US", "特斯拉"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			data := api.GetMACKLineData(tt.code, "101", 5)
			if data == nil || len(*data) == 0 {
				t.Errorf("[%s] MAC返回空", tt.label)
				return
			}
			for i, item := range *data {
				t.Logf("  [%d] day=%s open=%s close=%s high=%s low=%s vol=%s turnover=%s",
					i, item.Day, item.Open, item.Close, item.High, item.Low, item.Volume, item.TurnoverRate)
			}
		})
	}
}

func TestTdxKLineApi_MACKLineHKUS_Minute(t *testing.T) {
	api := NewTdxKLineApi()
	tests := []struct {
		code  string
		label string
		klt   string
	}{
		{"AAPL.US", "苹果-1分钟", "1"},
		{"AAPL.US", "苹果-5分钟", "5"},
		{"00700.HK", "腾讯-1分钟", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			data := api.GetMACKLineData(tt.code, tt.klt, 5)
			if data == nil || len(*data) == 0 {
				t.Skipf("%s %s分钟线返回空（可能非交易时段）", tt.code, tt.klt)
				return
			}
			for i, item := range *data {
				t.Logf("  [%d] day=%s open=%s close=%s high=%s low=%s vol=%s", i, item.Day, item.Open, item.Close, item.High, item.Low, item.Volume)
			}
		})
	}
}

func TestTdxKLineApi_GetF10CategoryContent(t *testing.T) {
	api := NewTdxKLineApi()

	t.Run("公司概况", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "公司概况")
		if section == nil || section.Content == "" {
			t.Fatal("GetF10CategoryContent returned empty")
		}
		t.Logf("分类: %s, 内容长度: %d", section.Name, len(section.Content))
		t.Logf("内容前200字: %s", truncateStr(section.Content, 200))
	})

	t.Run("财务分析", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "财务分析")
		if section == nil || section.Content == "" {
			t.Fatal("GetF10CategoryContent returned empty")
		}
		t.Logf("分类: %s, 内容长度: %d", section.Name, len(section.Content))
	})

	t.Run("不存在的分类", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "不存在的分类")
		if section == nil {
			t.Fatal("GetF10CategoryContent should return non-nil even for missing category")
		}
		t.Logf("不存在的分类: name=%s, content长度=%d", section.Name, len(section.Content))
	})
}

func TestTdxKLineApi_GetMACSymbolBelongBoard(t *testing.T) {
	api := NewTdxKLineApi()

	t.Run("A股-贵州茅台", func(t *testing.T) {
		items := api.GetMACSymbolBelongBoard("600519.SH")
		if items == nil || len(*items) == 0 {
			t.Fatal("GetMACSymbolBelongBoard returned empty for 600519.SH")
		}
		t.Logf("600519.SH 所属板块数: %d", len(*items))
		for _, item := range *items {
			t.Logf("  type=%s code=%s name=%s price=%.2f preClose=%.2f 涨停=%.0f 跌停=%.0f",
				item.BoardType, item.BoardCode, item.BoardName, item.Price, item.PreClose, item.LimitUpCount, item.LimitDownCount)
		}
	})

	t.Run("A股-平安银行", func(t *testing.T) {
		items := api.GetMACSymbolBelongBoard("000001.SZ")
		if items == nil || len(*items) == 0 {
			t.Fatal("GetMACSymbolBelongBoard returned empty for 000001.SZ")
		}
		t.Logf("000001.SZ 所属板块数: %d", len(*items))
		for i, item := range *items {
			if i >= 5 {
				break
			}
			t.Logf("  type=%s name=%s price=%.2f", item.BoardType, item.BoardName, item.Price)
		}
	})

	t.Run("港股-腾讯控股", func(t *testing.T) {
		items := api.GetMACSymbolBelongBoard("00700.HK")
		t.Logf("00700.HK 所属板块数: %d", len(*items))
		for _, item := range *items {
			t.Logf("  type=%s name=%s price=%.2f", item.BoardType, item.BoardName, item.Price)
		}
	})
}

func TestTdxKLineApi_GetMACCapitalFlow(t *testing.T) {
	api := NewTdxKLineApi()

	t.Run("A股-贵州茅台", func(t *testing.T) {
		row := api.GetMACCapitalFlow("600519.SH")
		if row == nil {
			t.Fatal("GetMACCapitalFlow returned nil for 600519.SH")
		}
		t.Logf("600519.SH 资金流向:")
		t.Logf("  今日主力流入=%.0f 流出=%.0f 净流入=%.0f", row.TodayMainIn, row.TodayMainOut, row.TodayMainNetIn)
		t.Logf("  今日散户流入=%.0f 流出=%.0f 净流入=%.0f", row.TodayRetailIn, row.TodayRetailOut, row.TodayRetailNetIn)
		t.Logf("  5日主力买入=%.0f 卖出=%.0f 净流入=%.0f", row.FiveDayMainBuy, row.FiveDayMainSell, row.FiveDayMainNetIn)
		t.Logf("  5日超大单=%.0f 大单=%.0f 中单=%.0f 小单=%.0f", row.FiveDaySuperNet, row.FiveDayLargeNet, row.FiveDayMediumNet, row.FiveDaySmallNet)
	})

	t.Run("A股-平安银行", func(t *testing.T) {
		row := api.GetMACCapitalFlow("000001.SZ")
		if row == nil {
			t.Fatal("GetMACCapitalFlow returned nil for 000001.SZ")
		}
		t.Logf("000001.SZ 今日主力净流入=%.0f 散户净流入=%.0f 5日主力净流入=%.0f",
			row.TodayMainNetIn, row.TodayRetailNetIn, row.FiveDayMainNetIn)
	})

	t.Run("港股-腾讯控股", func(t *testing.T) {
		// MAC主客户端不一定支持港股资金流向，失败时返回 nil，不视为用例失败
		row := api.GetMACCapitalFlow("00700.HK")
		if row == nil {
			t.Logf("00700.HK 资金流向: 获取失败或无数据（MAC可能不支持港股）")
			return
		}
		t.Logf("00700.HK 今日主力净流入=%.0f 5日主力净流入=%.0f", row.TodayMainNetIn, row.FiveDayMainNetIn)
	})
}

// ===== 纯函数单元测试（不触网，TEST_DB_SKIP_INIT=1 可配合 -run 过滤只跑本组） =====

func TestKlineCutoffKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"8位日期", "20261027", "2026-10-27 00:00"},
		{"14位日期时间", "20261027143500", "2026-10-27 14:35"},
		{"带分隔日期", "2026-10-27", "2026-10-27 00:00"},
		{"带分隔日期时间", "2026-10-27 14:35", "2026-10-27 14:35"},
		{"ISO分隔符", "2026-10-27T14:35", "2026-10-27 14:35"},
		{"秒部分被忽略", "2026-10-27 14:35:00", "2026-10-27 14:35"},
		{"首尾空白", " 20261027 ", "2026-10-27 00:00"},
		{"空串", "", ""},
		{"垃圾输入", "abc", ""},
		{"纯分隔符", "---", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := klineCutoffKey(tt.input)
			if got != tt.want {
				t.Errorf("klineCutoffKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestKlineKeyLE(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		cut      string
		dateOnly bool
		want     bool
	}{
		// dateOnly（8位日期end）：日线 Day 含 15:00 时分，end 当日必须包含（本次修复核心回归点）
		{"dateOnly含当日日线15点", "2026-10-27 15:00", "2026-10-27 00:00", true, true},
		{"dateOnly排除次日", "2026-10-28 09:30", "2026-10-27 00:00", true, false},
		{"dateOnly含更早日期", "2026-10-26 15:00", "2026-10-27 00:00", true, true},
		{"dateOnly当日整点键", "2026-10-27 00:00", "2026-10-27 00:00", true, true},
		// 14位分钟精度end：全键比较
		{"分钟精度相等", "2026-10-27 14:35", "2026-10-27 14:35", false, true},
		{"分钟精度更晚1分钟", "2026-10-27 14:36", "2026-10-27 14:35", false, false},
		{"分钟精度更早1分钟", "2026-10-27 14:34", "2026-10-27 14:35", false, true},
		{"分钟精度同日更晚小时", "2026-10-27 15:01", "2026-10-27 14:35", false, false},
		// 空键：无法解析的 Day 不参与比较
		{"空键dateOnly", "", "2026-10-27 00:00", true, false},
		{"空键分钟精度", "", "2026-10-27 14:35", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := klineKeyLE(tt.key, tt.cut, tt.dateOnly)
			if got != tt.want {
				t.Errorf("klineKeyLE(%q, %q, %v) = %v, want %v", tt.key, tt.cut, tt.dateOnly, got, tt.want)
			}
		})
	}
}

func TestFilterKLineDataBefore(t *testing.T) {
	build := func(days ...string) []KLineData {
		list := make([]KLineData, 0, len(days))
		for _, d := range days {
			list = append(list, KLineData{Day: d, Close: "1.00"})
		}
		return list
	}

	t.Run("dateOnly保留end当日并排除之后", func(t *testing.T) {
		in := build("2026-10-23 15:00", "2026-10-24 15:00", "2026-10-27 15:00", "2026-10-28 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 0)
		if len(out) != 3 || out[2].Day != "2026-10-27 15:00" {
			t.Fatalf("want 3 roots ending at 2026-10-27, got %+v", out)
		}
	})

	t.Run("截取最后limit根", func(t *testing.T) {
		in := build("2026-10-20 15:00", "2026-10-21 15:00", "2026-10-22 15:00", "2026-10-23 15:00", "2026-10-24 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261024"), true, 2)
		if len(out) != 2 || out[0].Day != "2026-10-23 15:00" || out[1].Day != "2026-10-24 15:00" {
			t.Fatalf("want last 2 roots, got %+v", out)
		}
	})

	t.Run("limit大于匹配数时全保留", func(t *testing.T) {
		in := build("2026-10-23 15:00", "2026-10-24 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 10)
		if len(out) != 2 {
			t.Fatalf("want 2 roots, got %+v", out)
		}
	})

	t.Run("limit为0时返回全部匹配", func(t *testing.T) {
		in := build("2026-10-23 15:00", "2026-10-24 15:00", "2026-10-27 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 0)
		if len(out) != 3 {
			t.Fatalf("want 3 roots, got %+v", out)
		}
	})

	t.Run("无匹配返回空", func(t *testing.T) {
		in := build("2026-10-28 15:00", "2026-10-29 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 5)
		if len(out) != 0 {
			t.Fatalf("want 0 roots, got %+v", out)
		}
	})

	t.Run("分钟精度end按全键比较", func(t *testing.T) {
		in := build("2026-10-27 14:34", "2026-10-27 14:35", "2026-10-27 14:36", "2026-10-27 14:37")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027143500"), false, 0)
		if len(out) != 2 || out[1].Day != "2026-10-27 14:35" {
			t.Fatalf("want 2 roots ending at 14:35, got %+v", out)
		}
	})

	t.Run("无法解析Day的根被跳过", func(t *testing.T) {
		in := build("bad-day", "2026-10-26 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 0)
		if len(out) != 1 || out[0].Day != "2026-10-26 15:00" {
			t.Fatalf("want 1 root (bad-day skipped), got %+v", out)
		}
	})

	t.Run("输入顺序保持正序不变", func(t *testing.T) {
		in := build("2026-10-24 15:00", "2026-10-25 15:00", "2026-10-26 15:00")
		out := filterKLineDataBefore(in, klineCutoffKey("20261027"), true, 0)
		for i := 1; i < len(out); i++ {
			if out[i-1].Day >= out[i].Day {
				t.Fatalf("output not in ascending order: %v", out)
			}
		}
	})
}
