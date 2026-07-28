package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"time"
)

type MarketStatisticApi struct {
}

func NewMarketStatisticApi() *MarketStatisticApi {
	return &MarketStatisticApi{}
}

type clsMarketDataResp struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data clsMarketData `json:"data"`
}

type clsMarketData struct {
	IndexQuote []clsIndexQuote `json:"index_quote"`
	UpDownDis  clsUpDownDis    `json:"up_down_dis"`
}

type clsIndexQuote struct {
	SecuCode string  `json:"secu_code"`
	SecuName string  `json:"secu_name"`
	LastPx   float64 `json:"last_px"`
	Change   float64 `json:"change"`
	ChangePx float64 `json:"change_px"`
	UpNum    int     `json:"up_num"`
	DownNum  int     `json:"down_num"`
	FlatNum  int     `json:"flat_num"`
}

type clsUpDownDis struct {
	UpNum       int     `json:"up_num"`
	DownNum     int     `json:"down_num"`
	AverageRise float64 `json:"average_rise"`
	RiseNum     int     `json:"rise_num"`
	FallNum     int     `json:"fall_num"`
	Down10      int     `json:"down_10"`
	Down8       int     `json:"down_8"`
	Down6       int     `json:"down_6"`
	Down4       int     `json:"down_4"`
	Down2       int     `json:"down_2"`
	FlatNum     int     `json:"flat_num"`
	Up2         int     `json:"up_2"`
	Up4         int     `json:"up_4"`
	Up6         int     `json:"up_6"`
	Up8         int     `json:"up_8"`
	Up10        int     `json:"up_10"`
	SuspendNum  int     `json:"suspend_num"`
	Status      bool    `json:"status"`
}

func (a *MarketStatisticApi) FetchAndSave() error {
	url := "https://x-quote.cls.cn/quote/index/home?app=CailianpressWeb&os=web&sv=8.4.6"

	resp, err := SharedHTTPClient.R().
		SetHeader("User-Agent", util.GetUserAgent()).
		SetHeader("Referer", "https://www.cls.cn/").
		Get(url)

	if err != nil {
		logger.SugaredLogger.Errorf("获取市场数据失败: %v", err)
		return err
	}

	var result clsMarketDataResp
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("解析市场数据失败: %v", err)
		return err
	}

	if result.Code != 200 {
		logger.SugaredLogger.Errorf("API返回错误: code=%d, msg=%s", result.Code, result.Msg)
		return nil
	}

	data := result.Data
	now := time.Now()
	dataDate := now.Format("2006-01-02")
	dataTime := now.Format("15:04")

	var shUp, shDown, szUp, szDown int
	for _, index := range data.IndexQuote {
		if index.SecuCode == "sh000001" || index.SecuName == "上证指数" {
			shUp = index.UpNum
			shDown = index.DownNum
		}
		if index.SecuCode == "sz399001" || index.SecuName == "深证成指" {
			szUp = index.UpNum
			szDown = index.DownNum
		}
	}

	totalUp := data.UpDownDis.RiseNum
	totalDown := data.UpDownDis.FallNum
	limitUp := data.UpDownDis.UpNum
	limitDown := data.UpDownDis.DownNum

	var upRatio, upDownRatio, limitRatio float64
	var sentimentDesc string
	total := totalUp + totalDown
	if total > 0 {
		upRatio = float64(totalUp) / float64(total) * 100
	}
	if totalDown > 0 {
		upDownRatio = float64(totalUp) / float64(totalDown)
	} else if totalUp > 0 {
		upDownRatio = float64(totalUp)
	}
	sentimentDesc = getSentimentDesc(upDownRatio)
	if limitDown > 0 {
		limitRatio = float64(limitUp) / float64(limitDown)
	} else if limitUp > 0 {
		limitRatio = float64(limitUp)
	}

	stat := models.MarketStatistic{
		DataDate:      dataDate,
		DataTime:      dataTime,
		UpCount:       totalUp,
		DownCount:     totalDown,
		UpRatio:       upRatio,
		UpDownRatio:   upDownRatio,
		SentimentDesc: sentimentDesc,
		LimitUp:       limitUp,
		LimitDown:     limitDown,
		LimitRatio:    limitRatio,
		ShUpCount:     shUp,
		ShDownCount:   shDown,
		SzUpCount:     szUp,
		SzDownCount:   szDown,
	}

	var existing models.MarketStatistic
	result2 := db.Dao.Where("data_date = ? AND data_time = ?", dataDate, dataTime).First(&existing)
	if result2.Error == nil {
		db.Dao.Model(&existing).Updates(stat)
		logger.SugaredLogger.Infof("更新市场统计数据: %s %s 涨跌家数(%d/%d) 涨跌停(%d/%d)",
			dataDate, dataTime, totalUp, totalDown, limitUp, limitDown)
	} else {
		db.Dao.Create(&stat)
		logger.SugaredLogger.Infof("保存市场统计数据: %s %s 涨跌家数(%d/%d) 涨跌停(%d/%d)",
			dataDate, dataTime, totalUp, totalDown, limitUp, limitDown)
	}

	return nil
}

func (a *MarketStatisticApi) GetTodayData() []models.MarketStatistic {
	today := time.Now().Format("2006-01-02")
	var data []models.MarketStatistic
	db.Dao.Where("data_date = ?", today).Order("data_time ASC").Find(&data)
	if len(data) > 0 {
		return data
	}
	var latest models.MarketStatistic
	if err := db.Dao.Order("data_date DESC, data_time DESC").First(&latest).Error; err == nil {
		db.Dao.Where("data_date = ?", latest.DataDate).Order("data_time ASC").Find(&data)
	}
	return data
}

func (a *MarketStatisticApi) GetRecentDaysData(days int) []models.MarketStatistic {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var data []models.MarketStatistic
	db.Dao.Where("data_date >= ?", startDate).Order("data_date ASC, data_time ASC").Find(&data)
	return data
}

func (a *MarketStatisticApi) GetByDate(date string) []models.MarketStatistic {
	var data []models.MarketStatistic
	db.Dao.Where("data_date = ?", date).Order("data_time ASC").Find(&data)
	return data
}

func getSentimentDesc(upDownRatio float64) string {
	switch {
	case upDownRatio >= 2:
		return "普涨(极强)"
	case upDownRatio >= 1.5:
		return "偏强"
	case upDownRatio > 1:
		return "稍强"
	case upDownRatio == 1:
		return "中性"
	case upDownRatio > 0.5:
		return "稍弱"
	case upDownRatio > 0:
		return "偏弱"
	default:
		return "普跌(冰点)"
	}
}
