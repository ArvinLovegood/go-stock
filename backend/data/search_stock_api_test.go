package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"testing"
)

func TestSearchStock(t *testing.T) {
	db.Init("../../data/stock.db")

	res := NewSearchStockApi("算力股;净利润连续3年增长").SearchStock()
	data := res["data"].(map[string]any)
	result := data["result"].(map[string]any)
	dataList := result["dataList"].([]any)
	for _, v := range dataList {
		d := v.(map[string]any)
		logger.SugaredLogger.Infof("%s:%s", d["INDUSTRY"], d["SECURITY_SHORT_NAME"])
	}
	//columns := result["columns"].([]any)
	//for _, v := range columns {
	//	logger.SugaredLogger.Infof("v:%+v", v)
	//}

}
