package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// @Author spark
// @Date 2025/6/8 20:45
// @Desc
//--------------------------------------------------------------------------------

var ShanghaiTimezone = time.FixedZone("CST", 8*60*60)

func GetShanghaiTime() time.Time {
	return time.Now().In(ShanghaiTimezone)
}

func FormatShanghaiTime(t time.Time) string {
	return t.In(ShanghaiTimezone).Format("2006-01-02 15:04:05")
}

func (a *App) GetTimezone() map[string]any {
	return map[string]any{
		"offset":   8 * 60 * 60,
		"location": "Asia/Shanghai",
	}
}

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
func (a *App) EMDictCode(code string) []any {
	return data.NewMarketNewsApi().EMDictCode(code, a.cache)
}

func (a *App) AnalyzeSentiment(text string) models.SentimentResult {
	return data.AnalyzeSentiment(text)
}

func (a *App) HotStock(marketType string) *[]models.HotItem {
	return data.NewMarketNewsApi().XUEQIUHotStock(100, marketType)
}

func (a *App) HotEvent(size int) *[]models.HotEvent {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotEvent(size)
}
func (a *App) HotTopic(size int) []any {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotTopic(size)
}

func (a *App) InvestCalendarTimeLine(yearMonth string) []any {
	return data.NewMarketNewsApi().InvestCalendar(yearMonth)
}
func (a *App) ClsCalendar() []any {
	return data.NewMarketNewsApi().ClsCalendar()
}

func (a *App) GetUplimitHot(date string, limit int) map[string]any {
	return data.NewMarketNewsApi().GetUplimitHot(date, limit)
}

func (a *App) IsTradingTime() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	return isTradingTime(time.Now().In(loc))
}

func (a *App) IsHKTradingTime() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	return IsHKTradingTime(time.Now().In(loc))
}

func (a *App) IsUSTradingTime() bool {
	return IsUSTradingTime(time.Now())
}

// IsTradingDay 判断 yyyy-MM-dd 是否为 A 股交易日（周末、法定节假日为 false）。
func (a *App) IsTradingDay(date string) bool {
	date = strings.TrimSpace(date)
	if date == "" {
		return false
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	t, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return false
	}
	return isTradingDay(t)
}

func (a *App) GetLatestTradingDay() string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	now := time.Now().In(loc)
	if isTradingDay(now) {
		hour, minute, _ := now.Clock()
		if hour < 15 || (hour == 15 && minute == 0) {
			return now.AddDate(0, 0, -1).Format("2006-01-02")
		}
		return now.Format("2006-01-02")
	}
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i)
		if isTradingDay(d) {
			return d.Format("2006-01-02")
		}
	}
	return now.Format("2006-01-02")
}

func (a *App) SearchStock(words string) map[string]any {
	return data.NewSearchStockApi(words).SearchStock(5000)
}
func (a *App) GetHotStrategy() map[string]any {
	return data.NewSearchStockApi("").HotStrategy()
}

func (a *App) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	page, err := data.NewCustomStrategyApi().GetCustomStrategyList(&query)
	if err != nil {
		return &models.CustomStrategyPageData{}
	}
	return page
}

func (a *App) GetAllCustomStrategies() *[]models.CustomStrategy {
	return data.NewCustomStrategyApi().GetAllCustomStrategies()
}

func (a *App) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return data.NewCustomStrategyApi().SaveCustomStrategy(strategy)
}

func (a *App) DeleteCustomStrategy(id uint) string {
	return data.NewCustomStrategyApi().DeleteCustomStrategy(id)
}

func (a *App) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	return data.NewStockDataApi().GetAllStocks(page, pageSize, name, technicalIndicators)
}

func (a *App) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("ChatWithAgent panic: %v", r)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	a.agentMu.Lock()
	if a.agentCancel != nil {
		a.agentCancel()
	}
	a.agentCancel = cancel
	a.agentMu.Unlock()

	defer func() {
		a.agentMu.Lock()
		a.agentCancel = nil
		a.agentMu.Unlock()
	}()

	ch := agent.NewStockAiAgentApi().ChatWithContext(ctx, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode)
	for msg := range ch {
		runtime.EventsEmit(a.ctx, "agent-message", agentMessageToFrontendMap(msg))
	}
	runtime.EventsEmit(a.ctx, "agent-message", agentMessageToFrontendMap(&schema.Message{
		Role:    schema.Assistant,
		Content: "agent-DONE",
	}))
}

// agentMessageToFrontendMap 用标准 JSON 将 schema.Message 转为 map 再 EventsEmit，
// 保证与 json 标签一致（如 reasoning_content、extra），避免 Wails 直接传结构体时前端字段名不一致。
func agentMessageToFrontendMap(msg *schema.Message) map[string]any {
	if msg == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return map[string]any{
			"role":              string(msg.Role),
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{
			"role":              string(msg.Role),
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	return m
}

func (a *App) AbortChatWithAgent() {
	a.agentMu.Lock()
	defer a.agentMu.Unlock()
	if a.agentCancel != nil {
		a.agentCancel()
		a.agentCancel = nil
	}
}

func (a *App) AnalyzeSentimentWithFreqWeight(text string) map[string]any {
	result, cleanFrequencies := data.NewsAnalyze(text, false)
	return map[string]any{
		"result":      result,
		"frequencies": cleanFrequencies,
	}
}

func (a *App) GetAIResponseResultList(query models.AIResponseResultQuery) *models.AIResponseResultPageData {
	page, err := data.NewAIResponseResultService().GetAIResponseResultList(query)
	if err != nil {
		return &models.AIResponseResultPageData{}
	}
	return page
}
func (a *App) DeleteAIResponseResult(id uint) string {
	err := data.NewAIResponseResultService().DeleteAIResponseResult(id)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}
func (a *App) BatchDeleteAIResponseResult(ids []uint) string {
	err := data.NewAIResponseResultService().BatchDeleteAIResponseResult(ids)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

func (a *App) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return data.NewStockChangesApi().GetStockChanges(changeTypes, pageIndex, pageSize)
}

func (a *App) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	all := data.NewStockChangesApi().GetAllStockChangesWithPaging(pageSize)
	historyService := data.NewStockChangeHistoryService()
	_, _ = historyService.SaveStockChangesWithDedup(all.Data)
	return all
}

func (a *App) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	result, err := data.NewStockChangeHistoryService().GetHistoryList(query)
	if err != nil {
		return &models.StockChangeHistoryPageData{}
	}
	return result
}

func (a *App) SaveStockChangesToHistory(changeTypes []int) string {
	api := data.NewStockChangesApi()
	result := api.GetStockChanges(changeTypes, 0, 500)
	if result == nil || len(result.Data) == 0 {
		return "没有获取到异动数据"
	}

	err := data.NewStockChangeHistoryService().SaveStockChanges(result.Data)
	if err != nil {
		return "保存失败: " + err.Error()
	}
	return fmt.Sprintf("成功保存 %d 条异动数据", len(result.Data))
}

func (a *App) DeleteStockChangeHistory(days int) string {
	err := data.NewStockChangeHistoryService().DeleteOldData(days)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除 %d 天前的历史数据", days)
}

func (a *App) GetDailyChangeStats(days int) []data.DailyChangeStats {
	result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
	if err != nil {
		return []data.DailyChangeStats{}
	}
	return result
}

func (a *App) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	result, err := data.NewStockChangeHistoryService().GetChangeTypeDailyStats(days)
	if err != nil {
		return []data.ChangeTypeDailyStats{}
	}
	return result
}

func (a *App) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
	if err != nil {
		return &data.ChangeRankResult{}
	}
	return result
}

func (a *App) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
	if err != nil {
		return []data.DailyDimensionStats{}
	}
	return result
}

func (a *App) GetTypeStatsByDate(date string) []data.TypeCountStats {
	result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
	if err != nil {
		return []data.TypeCountStats{}
	}
	return result
}

func (a *App) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	page, err := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&query)
	if err != nil {
		return &models.AiRecommendStocksPageData{}
	}
	return page
}
func (a *App) DeleteAiRecommendStocks(id uint) string {
	err := data.NewAiRecommendStocksService().DeleteAiRecommendStocks(id)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

func (a *App) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	err := data.NewAiRecommendStocksService().UpdateAiRecommendStocksAlert(id, enableAlert)
	if err != nil {
		return "更新预警状态失败"
	}
	return "更新预警状态成功"
}

func (a *App) GetPromptTemplateList(query models.PromptTemplateQuery) *models.PromptTemplatePageData {
	page, err := data.NewPromptTemplateApi().GetPromptTemplateList(&query)
	if err != nil {
		return &models.PromptTemplatePageData{}
	}
	return page
}

func (a *App) AddPromptTemplate(template models.PromptTemplate) string {
	return data.NewPromptTemplateApi().AddPrompt(template)
}

func (a *App) UpdatePromptTemplate(template models.PromptTemplate) string {
	return data.NewPromptTemplateApi().AddPrompt(template)
}

func (a *App) DeletePromptTemplate(id uint) string {
	return data.NewPromptTemplateApi().DelPrompt(id)
}

func (a *App) GetAllStockInfoList(query data.AllStockInfoQuery) *data.AllStockInfoPageData {
	page, err := data.NewStockDataApi().GetAllStockInfoList(&query)
	if err != nil {
		return &data.AllStockInfoPageData{}
	}
	return page
}

func (a *App) GetAllStockInfoById(id uint) *models.AllStockInfo {
	stock, err := data.NewStockDataApi().GetAllStockInfoById(id)
	if err != nil {
		return &models.AllStockInfo{}
	}
	return stock
}

func (a *App) AddAllStockInfo(stock models.AllStockInfo) string {
	err := data.NewStockDataApi().AddAllStockInfo(stock)
	if err != nil {
		return "操作失败: " + err.Error()
	}
	return "操作成功"
}

func (a *App) DeleteAllStockInfo(id uint) string {
	err := data.NewStockDataApi().DeleteAllStockInfo(id)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (a *App) BatchDeleteAllStockInfo(ids []uint) string {
	err := data.NewStockDataApi().BatchDeleteAllStockInfo(ids)
	if err != nil {
		return "批量删除失败: " + err.Error()
	}
	return "批量删除成功"
}

func (a *App) GetAllMarkets() []string {
	markets, err := data.NewStockDataApi().GetAllMarkets()
	if err != nil {
		return []string{}
	}
	return markets
}

func (a *App) GetAllIndustries() []string {
	industries, err := data.NewStockDataApi().GetAllIndustries()
	if err != nil {
		return []string{}
	}
	return industries
}

func (a *App) GetAllConcepts() []string {
	concepts, err := data.NewStockDataApi().GetAllConcepts()
	if err != nil {
		return []string{}
	}
	return concepts
}

func (a *App) GetStockRealTimePrice(stockCode string) map[string]any {
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return map[string]any{
			"code":    -1,
			"message": "获取股票价格失败",
			"price":   0,
		}
	}
	stock := (*stockDatas)[0]
	price, _ := convertor.ToFloat(stock.Price)
	if price == 0 {
		price, _ = convertor.ToFloat(stock.A1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.B1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.PreClose)
	}
	return map[string]any{
		"code":    0,
		"message": "success",
		"price":   price,
		"name":    stock.Name,
	}
}

// GetBKFundFlowList 获取板块资金流向历史数据（折线图用）
func (a *App) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowList(code, limit)
}

// GetBKFundFlowListByDate 获取板块指定日期的资金流向历史数据
func (a *App) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowListByDate(code, date)
}

// GetBKFundFlowTopList 获取最新板块资金排名
func (a *App) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopList(topN)
}

// GetBKFundFlowTopListByDate 获取指定日期的板块资金排名
func (a *App) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopListByDate(date, topN)
}

// GetAllBKCodes 获取所有已记录的板块代码
func (a *App) GetAllBKCodes() []map[string]string {
	return data.NewBKFundFlowApi().GetAllBKCodes()
}

// GetConceptFundFlowList 获取概念资金流向历史数据（折线图用）
func (a *App) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowList(code, limit)
}

// GetConceptFundFlowListByDate 获取概念指定日期的资金流向历史数据
func (a *App) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowListByDate(code, date)
}

// GetConceptFundFlowTopList 获取最新概念资金排名
func (a *App) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopList(topN)
}

// GetConceptFundFlowTopListByDate 获取指定日期的概念资金排名
func (a *App) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopListByDate(date, topN)
}

// GetAllConceptCodes 获取所有概念代码
func (a *App) GetAllConceptCodes() []map[string]string {
	return data.NewConceptFundFlowApi().GetAllConceptCodes()
}
