package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"time"
)

// ConceptFundFlowApi 概念资金流向采集 API
type ConceptFundFlowApi struct{}

func NewConceptFundFlowApi() *ConceptFundFlowApi {
	return &ConceptFundFlowApi{}
}

// conceptFundFlowResponse 东方财富概念资金接口返回结构
type conceptFundFlowResponse struct {
	Rc   int `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			F12 string  `json:"f12"` // 概念代码
			F13 int     `json:"f13"` // 市场
			F14 string  `json:"f14"` // 概念名称
			F62 float64 `json:"f62"` // 主力净流入（元）
		} `json:"diff"`
	} `json:"data"`
}

// FetchAndSave 从东方财富抓取概念资金数据并保存到数据库
func (c *ConceptFundFlowApi) FetchAndSave() (int, error) {
	url := "https://data.eastmoney.com/dataapi/bkzj/getbkzj?key=f62&code=m%3A90%2Bt%3A3"

	resp, err := SharedHTTPClient.SetTimeout(30*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36").
		SetHeader("Referer", "https://data.eastmoney.com/").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptFundFlowApi.FetchAndSave request error: %v", err)
		return 0, err
	}

	var result conceptFundFlowResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("ConceptFundFlowApi.FetchAndSave unmarshal error: %v", err)
		return 0, err
	}

	if result.Rc != 0 || len(result.Data.Diff) == 0 {
		logger.SugaredLogger.Warnf("ConceptFundFlowApi.FetchAndSave: rc=%d, diff count=%d", result.Rc, len(result.Data.Diff))
		return 0, nil
	}

	snapTime := time.Now().Format("2006-01-02 15:04:05")
	var records []models.ConceptFundFlow
	for _, item := range result.Data.Diff {
		records = append(records, models.ConceptFundFlow{
			Code:      item.F12,
			Name:      item.F14,
			NetInflow: int64(item.F62),
			SnapTime:  snapTime,
		})
	}

	if err := db.Dao.CreateInBatches(records, 200).Error; err != nil {
		logger.SugaredLogger.Errorf("ConceptFundFlowApi.FetchAndSave save error: %v", err)
		return 0, err
	}

	logger.SugaredLogger.Infof("ConceptFundFlowApi.FetchAndSave: saved %d records at %s", len(records), snapTime)
	return len(records), nil
}

// GetConceptFundFlowList 获取某个概念的资金流向历史数据（用于折线图）
func (c *ConceptFundFlowApi) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	if limit <= 0 {
		limit = 240 // 默认最近240条（4小时）
	}

	var points []models.ConceptFundFlowPoint
	err := db.Dao.Model(&models.ConceptFundFlow{}).
		Select("snap_time, net_inflow").
		Where("code = ?", code).
		Order("snap_time ASC").
		Limit(limit).
		Find(&points).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetConceptFundFlowList error: %v", err)
		return []models.ConceptFundFlowPoint{}
	}
	return points
}

// GetConceptFundFlowListByDate 获取某个概念指定日期的资金流向历史数据
func (c *ConceptFundFlowApi) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	var points []models.ConceptFundFlowPoint
	err := db.Dao.Model(&models.ConceptFundFlow{}).
		Select("snap_time, net_inflow").
		Where("code = ? AND snap_time LIKE ?", code, date+"%").
		Order("snap_time ASC").
		Find(&points).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetConceptFundFlowListByDate error: %v", err)
		return []models.ConceptFundFlowPoint{}
	}
	return points
}

// GetConceptFundFlowTopList 获取最新一次快照的概念资金排名（净流入前N名）
func (c *ConceptFundFlowApi) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	if topN <= 0 {
		topN = 20
	}

	// 先获取最新快照时间
	var latestTime string
	db.Dao.Model(&models.ConceptFundFlow{}).
		Select("MAX(snap_time)").
		Scan(&latestTime)
	if latestTime == "" {
		return []models.ConceptFundFlow{}
	}

	var list []models.ConceptFundFlow
	err := db.Dao.Where("snap_time = ?", latestTime).
		Order("net_inflow DESC").
		Limit(topN).
		Find(&list).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetConceptFundFlowTopList error: %v", err)
		return []models.ConceptFundFlow{}
	}
	return list
}

// GetConceptFundFlowTopListByDate 获取指定日期最新快照的概念资金排名
func (c *ConceptFundFlowApi) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	if topN <= 0 {
		topN = 20
	}

	// 获取指定日期的最新快照时间
	var latestTime string
	db.Dao.Model(&models.ConceptFundFlow{}).
		Select("MAX(snap_time)").
		Where("snap_time LIKE ?", date+"%").
		Scan(&latestTime)
	if latestTime == "" {
		return []models.ConceptFundFlow{}
	}

	var list []models.ConceptFundFlow
	err := db.Dao.Where("snap_time = ?", latestTime).
		Order("net_inflow DESC").
		Limit(topN).
		Find(&list).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetConceptFundFlowTopListByDate error: %v", err)
		return []models.ConceptFundFlow{}
	}
	return list
}

// GetAllConceptCodes 获取所有概念代码和名称（实时从东方财富获取，用于下拉选择）
func (c *ConceptFundFlowApi) GetAllConceptCodes() []map[string]string {
	url := "https://data.eastmoney.com/dataapi/bkzj/getbkzj?key=f62&code=m%3A90%2Bt%3A3"

	resp, err := SharedHTTPClient.SetTimeout(30*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36").
		SetHeader("Referer", "https://data.eastmoney.com/").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllConceptCodes request error: %v", err)
		return []map[string]string{}
	}

	var result conceptFundFlowResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("GetAllConceptCodes unmarshal error: %v", err)
		return []map[string]string{}
	}

	results := make([]map[string]string, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		results = append(results, map[string]string{
			"code": item.F12,
			"name": item.F14,
		})
	}
	return results
}

// CleanOldData 清理N天前的旧数据
func (c *ConceptFundFlowApi) CleanOldData(days int) int64 {
	if days <= 0 {
		days = 3
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	result := db.Dao.Where("snap_time < ?", cutoff).Delete(&models.ConceptFundFlow{})
	if result.Error != nil {
		logger.SugaredLogger.Errorf("CleanOldData error: %v", result.Error)
		return 0
	}
	logger.SugaredLogger.Infof("ConceptFundFlow CleanOldData: deleted %d records before %s", result.RowsAffected, cutoff)
	return result.RowsAffected
}
