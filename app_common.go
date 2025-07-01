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

func (a *App) IndustryResearchReport(industryCode string) []any {
	return data.NewMarketNewsApi().IndustryResearchReport(industryCode, 7)
}
func (a App) EMDictCode(code string) []any {
	return data.NewMarketNewsApi().EMDictCode(code, a.cache)
}

func (a App) AnalyzeSentiment(text string) data.SentimentResult {
	return data.AnalyzeSentiment(text)
}

func (a App) HotStock(marketType string) *[]models.HotItem {
	return data.NewMarketNewsApi().XUEQIUHotStock(100, marketType)
}

func (a App) HotEvent(size int) *[]models.HotEvent {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotEvent(size)
}
func (a App) HotTopic(size int) []any {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotTopic(size)
}

func (a App) InvestCalendarTimeLine(yearMonth string) []any {
	return data.NewMarketNewsApi().InvestCalendar(yearMonth)
}
func (a App) ClsCalendar() []any {
	return data.NewMarketNewsApi().ClsCalendar()
}

func (a App) SearchStock(words string) map[string]any {
	return data.NewSearchStockApi(words).SearchStock()
}
