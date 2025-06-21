package main

import (
	"go-stock/backend/data"
	"go-stock/backend/models"
)

// @Author spark
// @Date 2025/6/8 20:45
// @Desc
//-----------------------------------------------------------------------------------

func (a *App) LongTigerRank(date string) *[]models.LongTigerRankData {
	return data.NewMarketNewsApi().LongTiger(date)
}

func (a *App) StockResearchReport(stockCode string) []any {
	return data.NewMarketNewsApi().StockResearchReport(stockCode, 7)
}
func (a *App) StockNotice(stockCode string) []any {
	return data.NewMarketNewsApi().StockNotice(stockCode)
}
