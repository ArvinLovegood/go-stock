package data

import (
	"fmt"
	"go-stock/backend/db"
	"testing"
)

func init() {
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
