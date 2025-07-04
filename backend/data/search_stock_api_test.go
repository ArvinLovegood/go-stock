package data

import (
	"encoding/json"
	"github.com/duke-git/lancet/v2/convertor"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"testing"
)

func TestSearchStock(t *testing.T) {
	db.Init("../../data/stock.db")

	res := NewSearchStockApi("算力股;净利润连续3年增长").SearchStock(10)
	data := res["data"].(map[string]any)
	result := data["result"].(map[string]any)
	dataList := result["dataList"].([]any)
	columns := result["columns"].([]any)
	headers := map[string]string{}
	for _, v := range columns {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		//logger.SugaredLogger.Infof("key:%s title:%s dateMsg:%s unit:%s", d["key"], d["title"], d["dateMsg"], d["unit"])
		title := convertor.ToString(d["title"])
		if convertor.ToString(d["dateMsg"]) != "" {
			title = title + "[" + convertor.ToString(d["dateMsg"]) + "]"
		}
		if convertor.ToString(d["unit"]) != "" {
			title = title + "(" + convertor.ToString(d["unit"]) + ")"
		}
		headers[d["key"].(string)] = title
	}
	table := &[]map[string]any{}
	for _, v := range dataList {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		tmp := map[string]any{}
		for key, title := range headers {
			//logger.SugaredLogger.Infof("%s:%s", title, convertor.ToString(d[key]))
			tmp[title] = convertor.ToString(d[key])
		}
		*table = append(*table, tmp)
		//logger.SugaredLogger.Infof("--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	}
	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	logger.SugaredLogger.Infof("markdownTable=\n%s", markdownTable)
}

func TestSearchStockApi_HotStrategy(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").HotStrategy()
	logger.SugaredLogger.Infof("res:%+v", res)
	dataList := res["data"].([]any)
	for _, v := range dataList {
		d := v.(map[string]any)
		logger.SugaredLogger.Infof("v:%+v", d)
	}
}
