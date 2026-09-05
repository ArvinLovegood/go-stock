package data

import (
	"testing"
)

// TestGetBKConstituentStocksLive 验证东财成分股接口连通性与字段解析（行业板块 BK0475 银行 + 概念板块 BK0536 含可转债）
func TestGetBKConstituentStocksLive(t *testing.T) {
	api := NewBKConstituentsApi()
	stocks := api.GetBKConstituentStocks("BK0475")
	if len(stocks) == 0 {
		t.Fatal("no stocks returned for BK0475")
	}
	t.Logf("BK0475 got %d stocks, first: %+v", len(stocks), stocks[0])
	if stocks[0].Code == "" || stocks[0].Name == "" {
		t.Fatal("code/name empty")
	}

	// 概念板块（资金流向页概念榜单同为 BK 代码体系）
	conceptStocks := api.GetBKConstituentStocks("BK0536")
	if len(conceptStocks) == 0 {
		t.Fatal("no stocks returned for concept BK0536")
	}
	t.Logf("BK0536 got %d stocks, first: %+v", len(conceptStocks), conceptStocks[0])
}
