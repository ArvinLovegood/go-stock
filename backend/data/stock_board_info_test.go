package data

import (
	"testing"

	"go-stock/backend/db"
)

// TestGetStockBoardInfoList 验证板块/概念映射能从本地 all_stock_info 正确读出（行业/概念均非空，概念为 JSON 数组）
func TestGetStockBoardInfoList(t *testing.T) {
	db.Init("../../data/stock.db")
	list := NewStockDataApi().GetStockBoardInfoList()
	if len(list) == 0 {
		t.Fatalf("GetStockBoardInfoList returned empty list")
	}
	industryCnt, conceptCnt := 0, 0
	for _, b := range list {
		if b.Industry != "" {
			industryCnt++
		}
		if len(b.Concepts) > 0 {
			conceptCnt++
		}
	}
	t.Logf("total=%d industry=%d concept=%d sample=%+v", len(list), industryCnt, conceptCnt, list[0])
	if industryCnt == 0 || conceptCnt == 0 {
		t.Fatalf("industry or concepts empty: industry=%d concept=%d", industryCnt, conceptCnt)
	}
}
