package data

import "testing"

// 验证涨停梯队 Smart 查询：指定日期原样返回 + 空日期返回实际数据日期
func TestGetUplimitHotSmart(t *testing.T) {
	// 指定周末日期：应原样返回该日期（数据为空是正常语义，不回退显式日期）
	res, actualDate := NewMarketNewsApi().GetUplimitHotSmart("2026-09-05", 5)
	if actualDate != "2026-09-05" {
		t.Errorf("显式日期不应被改写: got %s", actualDate)
	}
	if res == nil {
		t.Fatal("res 为 nil")
	}

	// 空日期：自动定位最近有数据的交易日（交易日盘中/盘后有数据）
	res2, actualDate2 := NewMarketNewsApi().GetUplimitHotSmart("", 5)
	t.Logf("空日期回退结果: actualDate=%s code=%v", actualDate2, res2["code"])
	if code, _ := res2["code"].(float64); int(code) == 20000 {
		dataMap, _ := res2["data"].(map[string]any)
		stocks, _ := dataMap["stocks"].(string)
		if stocks != "" && actualDate2 == "" {
			t.Errorf("有数据但未返回数据日期")
		}
	}
}
