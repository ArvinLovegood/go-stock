package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"time"
)

// BKFundFlowApi 板块资金流向采集 API
type BKFundFlowApi struct{}

func NewBKFundFlowApi() *BKFundFlowApi {
	return &BKFundFlowApi{}
}

// bkFundFlowResponse 东方财富板块资金接口返回结构
type bkFundFlowResponse struct {
	Rc   int `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			F12 string  `json:"f12"` // 板块代码
			F13 int     `json:"f13"` // 市场
			F14 string  `json:"f14"` // 板块名称
			F62 float64 `json:"f62"` // 主力净流入（元）
		} `json:"diff"`
	} `json:"data"`
}

// FetchAndSave 从东方财富抓取板块资金数据并保存到数据库
func (b *BKFundFlowApi) FetchAndSave() (int, error) {
	url := "https://data.eastmoney.com/dataapi/bkzj/getbkzj?key=f62&code=m%3A90%2Bs%3A4"

	resp, err := SharedHTTPClient.SetTimeout(30*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36").
		SetHeader("Referer", "https://data.eastmoney.com/").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("BKFundFlowApi.FetchAndSave request error: %v", err)
		return 0, err
	}

	var result bkFundFlowResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("BKFundFlowApi.FetchAndSave unmarshal error: %v", err)
		return 0, err
	}

	if result.Rc != 0 || len(result.Data.Diff) == 0 {
		logger.SugaredLogger.Warnf("BKFundFlowApi.FetchAndSave: rc=%d, diff count=%d", result.Rc, len(result.Data.Diff))
		return 0, nil
	}

	snapTime := time.Now().Format("2006-01-02 15:04:05")
	var records []models.BKFundFlow
	for _, item := range result.Data.Diff {
		records = append(records, models.BKFundFlow{
			Code:      item.F12,
			Name:      item.F14,
			NetInflow: int64(item.F62),
			SnapTime:  snapTime,
		})
	}

	if err := db.Dao.CreateInBatches(records, 200).Error; err != nil {
		logger.SugaredLogger.Errorf("BKFundFlowApi.FetchAndSave save error: %v", err)
		return 0, err
	}

	logger.SugaredLogger.Infof("BKFundFlowApi.FetchAndSave: saved %d records at %s", len(records), snapTime)
	return len(records), nil
}

// GetBKFundFlowList 获取某个板块的资金流向历史数据（用于折线图）
func (b *BKFundFlowApi) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	if limit <= 0 {
		limit = 240 // 默认最近240条（4小时）
	}

	var points []models.BKFundFlowPoint
	err := db.Dao.Model(&models.BKFundFlow{}).
		Select("snap_time, net_inflow").
		Where("code = ?", code).
		Order("snap_time ASC").
		Limit(limit).
		Find(&points).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetBKFundFlowList error: %v", err)
		return []models.BKFundFlowPoint{}
	}
	return points
}

// GetBKFundFlowListByDate 获取某个板块指定日期的资金流向历史数据
func (b *BKFundFlowApi) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	var points []models.BKFundFlowPoint
	err := db.Dao.Model(&models.BKFundFlow{}).
		Select("snap_time, net_inflow").
		Where("code = ? AND snap_time LIKE ?", code, date+"%").
		Order("snap_time ASC").
		Find(&points).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetBKFundFlowListByDate error: %v", err)
		return []models.BKFundFlowPoint{}
	}
	return points
}

// GetBKFundFlowTopList 获取最新一次快照的板块资金排名（净流入前N名）
func (b *BKFundFlowApi) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return b.GetBKFundFlowRankList(topN, "inflow")
}

// GetBKFundFlowRankList 获取最新一次快照的板块资金排名（非交易日自动回退到最近有数据的交易日）
// direction: "inflow" 净流入降序（流入榜，仅净流入>0）；"outflow" 净流入升序（流出榜，仅净流入<0）
func (b *BKFundFlowApi) GetBKFundFlowRankList(topN int, direction string) []models.BKFundFlow {
	if topN <= 0 {
		topN = 20
	}
	order := "net_inflow DESC"
	cond := "net_inflow > 0"
	if direction == "outflow" {
		order = "net_inflow ASC"
		cond = "net_inflow < 0"
	}

	// 先获取最新快照时间
	var latestTime string
	db.Dao.Model(&models.BKFundFlow{}).
		Select("MAX(snap_time)").
		Scan(&latestTime)
	if latestTime == "" {
		return []models.BKFundFlow{}
	}

	var list []models.BKFundFlow
	err := db.Dao.Where("snap_time = ? AND "+cond, latestTime).
		Order(order).
		Limit(topN).
		Find(&list).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetBKFundFlowRankList error: %v", err)
		return []models.BKFundFlow{}
	}
	return list
}

// GetBKFundFlowTopListByDate 获取指定日期最新快照的板块资金排名（含净流入与净流出板块，按主力净流入降序）
// 前端页面据此自行拆分流入榜/流出榜，故不能按方向过滤
func (b *BKFundFlowApi) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	if topN <= 0 {
		topN = 20
	}
	// 获取指定日期的最新快照时间
	var latestTime string
	db.Dao.Model(&models.BKFundFlow{}).
		Select("MAX(snap_time)").
		Where("snap_time LIKE ?", date+"%").
		Scan(&latestTime)
	if latestTime == "" {
		return []models.BKFundFlow{}
	}

	var list []models.BKFundFlow
	err := db.Dao.Where("snap_time = ?", latestTime).
		Order("net_inflow DESC").
		Limit(topN).
		Find(&list).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetBKFundFlowTopListByDate error: %v", err)
		return []models.BKFundFlow{}
	}
	return list
}

// GetBKFundFlowRankListByDate 获取指定日期最新快照的板块资金排名
// direction: "inflow" 净流入降序（流入榜，仅净流入>0）；"outflow" 净流入升序（流出榜，仅净流入<0）
func (b *BKFundFlowApi) GetBKFundFlowRankListByDate(date string, topN int, direction string) []models.BKFundFlow {
	if topN <= 0 {
		topN = 20
	}
	order := "net_inflow DESC"
	cond := "net_inflow > 0"
	if direction == "outflow" {
		order = "net_inflow ASC"
		cond = "net_inflow < 0"
	}

	// 获取指定日期的最新快照时间
	var latestTime string
	db.Dao.Model(&models.BKFundFlow{}).
		Select("MAX(snap_time)").
		Where("snap_time LIKE ?", date+"%").
		Scan(&latestTime)
	if latestTime == "" {
		return []models.BKFundFlow{}
	}

	var list []models.BKFundFlow
	err := db.Dao.Where("snap_time = ? AND "+cond, latestTime).
		Order(order).
		Limit(topN).
		Find(&list).Error
	if err != nil {
		logger.SugaredLogger.Errorf("GetBKFundFlowRankListByDate error: %v", err)
		return []models.BKFundFlow{}
	}
	return list
}

// BKCodeInfo 板块代码信息结构体
type BKCodeInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// GetAllBKCodes 获取所有板块代码和名称（实时从东方财富获取，用于下拉选择）
func (b *BKFundFlowApi) GetAllBKCodes() []map[string]string {
	url := "https://data.eastmoney.com/dataapi/bkzj/getbkzj?key=f62&code=m%3A90%2Bs%3A4"

	resp, err := SharedHTTPClient.SetTimeout(30*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36").
		SetHeader("Referer", "https://data.eastmoney.com/").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllBKCodes request error: %v", err)
		return []map[string]string{}
	}

	var result bkFundFlowResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("GetAllBKCodes unmarshal error: %v", err)
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
func (b *BKFundFlowApi) CleanOldData(days int) int64 {
	if days <= 0 {
		days = 3
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	result := db.Dao.Where("snap_time < ?", cutoff).Delete(&models.BKFundFlow{})
	if result.Error != nil {
		logger.SugaredLogger.Errorf("CleanOldData error: %v", result.Error)
		return 0
	}
	logger.SugaredLogger.Infof("CleanOldData: deleted %d records before %s", result.RowsAffected, cutoff)
	return result.RowsAffected
}
