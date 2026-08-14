package data

// @Author spark
// @Date 2024/12/10 9:21
// @Desc
//-----------------------------------------------------------------------------------
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"io"
	"io/ioutil"
	url2 "net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/robertkrimen/otto"
	"github.com/samber/lo"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

const sinaStockUrl = "http://hq.sinajs.cn/rn=%d&list=%s"

const txStockUrl = "http://qt.gtimg.cn/?_=%d&q=%s"

const tushareApiUrl = "http://api.tushare.pro"

type StockDataApi struct {
	client *resty.Client
	config *SettingConfig
}
type StockInfo struct {
	gorm.Model
	Date     string  `json:"日期" gorm:"index"`
	Time     string  `json:"时间" gorm:"index" `
	Code     string  `json:"股票代码" gorm:"index"`
	Name     string  `json:"股票名称" gorm:"index"`
	PrePrice float64 `json:"上次当前价格"`
	Price    string  `json:"当前价格"`
	Volume   string  `json:"成交的股票数"`
	Amount   string  `json:"成交金额"`
	Open     string  `json:"今日开盘价"`
	PreClose string  `json:"昨日收盘价"`
	High     string  `json:"今日最高价"`
	Low      string  `json:"今日最低价"`
	Bid      string  `json:"竞买价"`
	Ask      string  `json:"竞卖价"`
	B1P      string  `json:"买一报价"`
	B1V      string  `json:"买一申报"`
	B2P      string  `json:"买二报价"`
	B2V      string  `json:"买二申报"`
	B3P      string  `json:"买三报价"`
	B3V      string  `json:"买三申报"`
	B4P      string  `json:"买四报价"`
	B4V      string  `json:"买四申报"`
	B5P      string  `json:"买五报价"`
	B5V      string  `json:"买五申报"`
	A1P      string  `json:"卖一报价"`
	A1V      string  `json:"卖一申报"`
	A2P      string  `json:"卖二报价"`
	A2V      string  `json:"卖二申报"`
	A3P      string  `json:"卖三报价"`
	A3V      string  `json:"卖三申报"`
	A4P      string  `json:"卖四报价"`
	A4V      string  `json:"卖四申报"`
	A5P      string  `json:"卖五报价"`
	A5V      string  `json:"卖五申报"`
	Market   string  `json:"市场"`
	BA       string  `json:"盘前盘后"`
	BAChange string  `json:"盘前盘后涨跌幅"`

	//以下是字段值需二次计算
	ChangePercent     float64 `json:"changePercent"`     //涨跌幅
	ChangePrice       float64 `json:"changePrice"`       //涨跌额
	HighRate          float64 `json:"highRate"`          //最高涨跌
	LowRate           float64 `json:"lowRate"`           //最低涨跌
	CostPrice         float64 `json:"costPrice"`         //成本价
	CostVolume        int64   `json:"costVolume"`        //持仓数量
	Profit            float64 `json:"profit"`            //总盈亏率
	ProfitAmount      float64 `json:"profitAmount"`      //总盈亏金额
	ProfitAmountToday float64 `json:"profitAmountToday"` //今日盈亏金额

	Sort               int64   `json:"sort"` //排序
	AlarmChangePercent float64 `json:"alarmChangePercent"`
	AlarmPrice         float64 `json:"alarmPrice"`

	Groups []GroupStock `gorm:"-:all"`
}

func (receiver StockInfo) TableName() string {
	return "stock_info"
}

type TushareRequest struct {
	ApiName string `json:"api_name"`
	Token   string `json:"token"`
	Params  any    `json:"params"`
	Fields  string `json:"fields"`
}
type TushareResponse struct {
	RequestId string `json:"request_id"`
	Code      int    `json:"code"`
	Data      any    `json:"data"`
	Msg       string `json:"msg"`
}

/*
	字段	类型	说明
	ts_code	str	TS代码
	symbol	str	股票代码
	name	str	股票名称
	area	str	地域
	industry	str	所属行业
	fullname	str	股票全称
	enname	str	英文全称
	cnspell	str	拼音缩写
	market	str	市场类型
	exchange	str	交易所代码
	curr_type	str	交易货币
	list_status	str	上市状态 L上市 D退市 P暂停上市
	list_date	str	上市日期
	delist_date	str	退市日期
	is_hs	str	是否沪深港通标的，N否 H沪股通 S深股通
	act_name	str	实控人名称
	act_ent_type	str	实控人企业性质*/

type StockBasic struct {
	gorm.Model
	TsCode     string `json:"ts_code" gorm:"index"`
	Symbol     string `json:"symbol" gorm:"index"`
	Name       string `json:"name" gorm:"index"`
	Area       string `json:"area"`
	Industry   string `json:"industry" gorm:"index"`
	Fullname   string `json:"fullname"`
	Ename      string `json:"enname"`
	Cnspell    string `json:"cnspell"`
	Market     string `json:"market"`
	Exchange   string `json:"exchange"`
	CurrType   string `json:"curr_type"`
	ListStatus string `json:"list_status"`
	ListDate   string `json:"list_date"`
	DelistDate string `json:"delist_date"`
	IsHs       string `json:"is_hs"`
	ActName    string `json:"act_name"`
	ActEntType string `json:"act_ent_type"`
	BKName     string `json:"bk_name"`
	BKCode     string `json:"bk_code"`
}

type FollowedStock struct {
	StockCode          string
	Name               string
	Volume             int64
	CostPrice          float64
	Price              float64
	PriceChange        float64
	ChangePercent      float64
	AlarmChangePercent float64
	AlarmPrice         float64
	Time               time.Time
	Sort               int64
	Cron               *string
	IsDel              soft_delete.DeletedAt `gorm:"softDelete:flag"`
	Groups             []GroupStock          `gorm:"foreignKey:StockCode;references:StockCode"`
	AiConfigId         int
	EntryPrice         float64
	TakeProfitPrice    float64
	StopLossPrice      float64
}

func (receiver FollowedStock) TableName() string {
	return "followed_stock"
}

// TradingRecord 交易日志结构体
type TradingRecord struct {
	ID              uint   `gorm:"primaryKey"`
	StockCode       string `gorm:"index"`
	StockName       string
	Direction       string `gorm:"index"` // 买入/卖出
	Price           float64
	Volume          int64
	Amount          float64   `gorm:"-"` // 计算字段: Price * Volume
	TradingTime     time.Time `gorm:"index"`
	Reason          string    `gorm:"type:text"`
	StopLossPrice   float64
	TakeProfitPrice float64
	Fee             float64
	MarketValue     float64
	Mindset         string `gorm:"type:text"`
	// RecordedClosePrice 保存时写入的当日收盘价或现价快照，列表盈亏计算优先使用，减少重复请求行情
	RecordedClosePrice float64 `json:"recordedClosePrice" gorm:"column:recorded_close_price"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (receiver TradingRecord) TableName() string {
	return "trading_records"
}

// TradingRecordListQuery 交易日志列表查询（与前端分页、筛选参数一致）
type TradingRecordListQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Keyword   string `json:"keyword"`   // 股票代码或名称模糊匹配
	Direction string `json:"direction"` // 买入 / 卖出，空表示全部
	StartDate string `json:"startDate"` // yyyy-MM-dd，交易时间起始（含当日 0 点）
	EndDate   string `json:"endDate"`   // yyyy-MM-dd，交易时间结束（含当日）
}

// TradingRecordPageData 交易日志分页结果
type TradingRecordPageData struct {
	List       []TradingRecordItem `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}

// TradingRecordItem 交易日志项（包含盈亏信息）
type TradingRecordItem struct {
	TradingRecord
	ClosePrice    float64 `json:"closePrice"`    // 收盘价或最新价
	ProfitAmount  float64 `json:"profitAmount"`  // 盈亏金额
	ProfitPercent float64 `json:"profitPercent"` // 盈亏收益率
}

type TradingRecordStatistics struct {
	TotalBuyAmount  float64 `json:"totalBuyAmount"`
	TotalSellAmount float64 `json:"totalSellAmount"`
	TotalProfit     float64 `json:"totalProfit"`
	ProfitRate      float64 `json:"profitRate"`
	HoldingsAmount  float64 `json:"holdingsAmount"`
	CurrentValue    float64 `json:"currentValue"`
	StockCount      int64   `json:"stockCount"`
	// 当日交易盈亏与收益（基于今日交易记录计算）
	TodayBuyAmount      float64 `json:"todayBuyAmount"`      // 今日买入总额
	TodaySellAmount     float64 `json:"todaySellAmount"`     // 今日卖出总额
	TodayRealizedProfit float64 `json:"todayRealizedProfit"` // 今日已实现盈亏（卖出）
	TodayFloatingProfit float64 `json:"todayFloatingProfit"` // 今日浮动盈亏（今日买入按现价计算）
	TodayProfit         float64 `json:"todayProfit"`         // 今日总盈亏 = 已实现 + 浮动
	TodayProfitRate     float64 `json:"todayProfitRate"`     // 今日收益率
}

type TushareStockBasicResponse struct {
	TushareResponse
	Data StockBasicResponse `json:"data"`
}

type StockBasicResponse struct {
	Fields  []string `json:"fields"`
	Items   [][]any  `json:"items"`
	HasMore bool     `json:"has_more"`
	Count   int      `json:"count"`
}

func (receiver StockBasic) TableName() string {
	return "tushare_stock_basic"
}

func NewStockDataApi() *StockDataApi {
	return &StockDataApi{
		client: SharedHTTPClient,
		config: GetSettingConfig(),
	}
}

// GetIndexBasic 获取指数信息
func (receiver StockDataApi) GetIndexBasic() {
	res := &TushareStockBasicResponse{}
	fields := "ts_code,name,market,publisher,category,base_date,base_point,list_date,fullname,index_type,weight_rule,desc"
	_, err := receiver.client.R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: "index_basic",
			Token:   receiver.config.TushareToken,
			Params:  nil,
			Fields:  fields}).
		SetResult(res).
		Post(tushareApiUrl)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return
	}
	if res.Code != 0 {
		logger.SugaredLogger.Error(res.Msg)
		return
	}
	//ioutil.WriteFile("index_basic.json", resp.Body(), 0666)

	for _, item := range res.Data.Items {
		data := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			data[field] = item[idx]
		}
		index := &IndexBasic{}
		jsonData, _ := json.Marshal(data)
		err := json.Unmarshal(jsonData, index)
		if err != nil {
			continue
		}
		index.ID = 0
		db.Dao.Model(&IndexBasic{}).FirstOrCreate(index, &IndexBasic{TsCode: index.TsCode}).Where("ts_code = ?", index.TsCode).Updates(index)
	}

}

// map转换为结构体

func (receiver StockDataApi) GetStockBaseInfo() {
	res := &TushareStockBasicResponse{}
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	resp, err := receiver.client.R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: "stock_basic",
			Token:   receiver.config.TushareToken,
			Params:  nil,
			Fields:  fields,
		}).
		SetResult(res).
		Post(tushareApiUrl)
	//logger.SugaredLogger.Infof("GetStockBaseInfo %s", string(resp.Body()))
	ioutil.WriteFile("stock_basic.json", resp.Body(), 0666)
	//logger.SugaredLogger.Infof("GetStockBaseInfo %+v", res)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return
	}
	if res.Code != 0 {
		logger.SugaredLogger.Error(res.Msg)
		return
	}
	for _, item := range res.Data.Items {
		stock := &StockBasic{}
		data := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			//logger.SugaredLogger.Infof("field: %s", field)
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			data[field] = item[idx]
		}
		jsonData, _ := json.Marshal(data)
		err := json.Unmarshal(jsonData, stock)
		if err != nil {
			continue
		}
		stock.ID = 0
		db.Dao.Model(&StockBasic{}).FirstOrCreate(stock, &StockBasic{TsCode: stock.TsCode}).Where("ts_code = ?", stock.TsCode).Updates(stock)
	}

}

func (receiver StockDataApi) GetStockCodeRealTimeData(StockCodes ...string) (*[]StockInfo, error) {
	StockCodes = ConvertTushareCodeToStockCodes(StockCodes)

	stockInfos := make([]StockInfo, 0)

	hkcodes := slice.Filter(StockCodes, func(i int, s string) bool {
		return strutil.HasPrefixAny(s, []string{"hk", "HK", "sh", "sz"})
	})

	if hkcodes != nil && len(hkcodes) > 0 {
		hkcodesStr := slice.JoinFunc(hkcodes, ",", func(s string) string {
			if strutil.HasPrefixAny(s, []string{"hk", "HK"}) {
				return "r_" + strings.ToLower(s)
			} else {
				return strings.ToLower(s)
			}
		})
		url := fmt.Sprintf(txStockUrl, time.Now().Unix(), hkcodesStr)
		resp, err := receiver.client.R().
			SetHeader("Host", "qt.gtimg.cn").
			SetHeader("Referer", "https://gu.qq.com/").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
			Get(url)
		//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %s", url)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return &[]StockInfo{}, err
		}
		str := GB18030ToUTF8(resp.Body())
		dataStr := strutil.SplitAndTrim(strings.Trim(str, "\n"), ";")

		for _, data := range dataStr {
			stockData, err := ParseTxStockData(data)
			if err != nil {
				logger.SugaredLogger.Error(err.Error())
				continue
			}
			if stockData == nil {
				continue
			}
			stockInfos = append(stockInfos, *stockData)
			go func() {
				var count int64
				db.Dao.Model(&StockInfo{}).Where("code = ?", stockData.Code).Count(&count)
				if count == 0 {
					db.Dao.Model(&StockInfo{}).Create(stockData)
				} else {
					db.Dao.Model(&StockInfo{}).Where("code = ?", stockData.Code).Updates(stockData)
				}
			}()
		}
	}

	szzsusCodes := slice.Filter(StockCodes, func(i int, s string) bool {
		return !strutil.HasPrefixAny(s, []string{"hk", "HK", "sh", "sz"})
	})

	codes := slice.JoinFunc(szzsusCodes, ",", func(s string) string {
		if strings.HasPrefix(s, "us") {
			s = strings.Replace(s, "us", "gb_", 1)
		}
		if strings.HasPrefix(s, "US") {
			s = strings.Replace(s, "US", "gb_", 1)
		}
		return strings.ToLower(s)
	})

	url := fmt.Sprintf(sinaStockUrl, time.Now().Unix(), codes)
	//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %s", url)
	resp, err := receiver.client.R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return &[]StockInfo{}, err
	}

	str := GB18030ToUTF8(resp.Body())
	dataStr := strutil.SplitEx(str, "\n", true)

	for _, data := range dataStr {
		//logger.SugaredLogger.Info(data)
		stockData, err := ParseFullSingleStockData(data)
		//logger.SugaredLogger.Infof("GetStockCodeRealTimeData %v", stockData)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			continue
		}
		if stockData == nil {
			continue
		}
		stockInfos = append(stockInfos, *stockData)

		go func() {
			var count int64
			db.Dao.Model(&StockInfo{}).Where("code = ?", stockData.Code).Count(&count)
			if count == 0 {
				db.Dao.Model(&StockInfo{}).Create(stockData)
			} else {
				db.Dao.Model(&StockInfo{}).Where("code = ?", stockData.Code).Updates(stockData)
			}
		}()

	}

	return &stockInfos, err
}

func (receiver StockDataApi) Follow(stockCode string) string {
	//logger.SugaredLogger.Infof("Follow %s", stockCode)
	stockInfos, err := receiver.GetStockCodeRealTimeData(stockCode)
	if err != nil || len(*stockInfos) == 0 {
		logger.SugaredLogger.Error(err)
		return "关注失败"
	}
	if strings.HasPrefix(stockCode, "us") {
		stockCode = strings.Replace(stockCode, "us", "gb_", 1)
	}
	if strings.HasPrefix(stockCode, "US") {
		stockCode = strings.Replace(stockCode, "US", "gb_", 1)
	}
	count := int64(0)
	db.Dao.Model(&FollowedStock{}).Where("is_del = ?", 0).Count(&count)
	//logger.SugaredLogger.Errorf("Follow-count %v", count)
	// VIP 用户（有效期内）不限制关注数量，非 VIP 用户最多关注 63 只
	if _, active := EffectiveSponsorVipLevel(); !active && count >= 63 {
		return "最多只能关注63只股票，升级VIP后不限数量"
	}

	stockCode = normalizeStockCode(stockCode)

	// 检查是否已经关注过该股票
	var existingStock FollowedStock
	result := db.Dao.Model(&FollowedStock{}).Where("stock_code = ? AND is_del = ?", stockCode, 0).First(&existingStock)
	if result.Error == nil {
		// 股票已经关注过
		return "已经关注了"
	}

	maxSort := int64(0)
	db.Dao.Model(&FollowedStock{}).Raw("select max(sort) as sort from followed_stock").Scan(&maxSort)

	//logger.SugaredLogger.Infof("Follow-maxSort %v", maxSort)

	stockInfo := (*stockInfos)[0]
	price, _ := convertor.ToFloat(stockInfo.Price)
	db.Dao.Model(&FollowedStock{}).FirstOrCreate(&FollowedStock{
		StockCode:          stockCode,
		Name:               stockInfo.Name,
		Price:              price,
		Time:               time.Now(),
		ChangePercent:      0,
		PriceChange:        0,
		Sort:               maxSort + 1,
		AlarmChangePercent: 3,
		AlarmPrice:         price + 1,
	}, &FollowedStock{StockCode: stockCode})
	return "关注成功"
}

func (receiver StockDataApi) UnFollow(stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(stockCode)).Delete(&FollowedStock{})
	return "取消关注成功"
}

func (receiver StockDataApi) SetCostPriceAndVolume(price float64, volume int64, stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(stockCode)).Update("cost_price", price).Update("volume", volume).Error
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "设置失败"
	}
	return "设置成功"
}

func (receiver StockDataApi) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(stockCode)).Updates(&map[string]any{
		"alarm_change_percent": val,
		"alarm_price":          alarmPrice,
	}).Error
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "设置失败"
	}
	return "设置成功"
}

func (receiver StockDataApi) SetStockSort(newSort int64, stockCode string) {
	//if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
	//	stockCode = normalizeStockCode(stockCode)
	//	stockCode = strings.Replace(stockCode, "gb_", "us", 1)
	//}

	// 获取当前排序值
	var currentStock FollowedStock
	if err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(stockCode)).First(&currentStock).Error; err != nil {
		logger.SugaredLogger.Error("找不到当前股票: ", err.Error())
		return
	}

	oldSort := currentStock.Sort

	// 如果排序值没有变化，直接返回
	if oldSort == newSort {
		return
	}
	// 检查新排序位置是否被占用
	var count int64
	if err := db.Dao.Model(&FollowedStock{}).Where("sort = ?", newSort).Count(&count).Error; err != nil {
		logger.SugaredLogger.Error("检查新排序位置被占用失败: ", err.Error())
		return
	}
	if count == 0 {
		// 新位置未被占用，直接更新当前记录
		if err := db.Dao.Model(&FollowedStock{}).
			Where("stock_code = ?", normalizeStockCode(stockCode)).
			Update("sort", newSort).Error; err != nil {
			logger.SugaredLogger.Error("更新排序位置失败: ", err.Error())
		}
	} else {
		// 新位置已被占用，需要移动其他记录
		if newSort < oldSort {
			// 向前移动：将中间记录向后移动
			if err := db.Dao.Model(&FollowedStock{}).
				Where("sort >= ? AND sort < ?", newSort, oldSort).
				Update("sort", gorm.Expr("sort + 1")).Error; err != nil {
				logger.SugaredLogger.Error("向前排序更新失败: ", err.Error())
			}
		} else {
			// 向后移动：将中间记录向前移动
			if err := db.Dao.Model(&FollowedStock{}).
				Where("sort > ? AND sort <= ?", oldSort, newSort).
				Update("sort", gorm.Expr("sort - 1")).Error; err != nil {
				logger.SugaredLogger.Error("向后排序更新失败: ", err.Error())
			}
		}

		// 更新目标记录的排序
		if err := db.Dao.Model(&FollowedStock{}).
			Where("stock_code = ?", normalizeStockCode(stockCode)).
			Update("sort", newSort).Error; err != nil {
			logger.SugaredLogger.Error("更新股票排序失败: ", err.Error())
		}
	}

}
func (receiver StockDataApi) SetStockAICron(cron string, stockCode string) {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(stockCode)).Update("cron", cron)

}
func (receiver StockDataApi) SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice float64, stockCode string) string {
	stockCode = strings.ToUpper(stockCode)
	if strings.HasSuffix(stockCode, ".SZ") {
		stockCode = "sz" + strings.TrimSuffix(stockCode, ".SZ")
	} else if strings.HasSuffix(stockCode, ".SH") {
		stockCode = "sh" + strings.TrimSuffix(stockCode, ".SH")
	} else if strings.HasSuffix(stockCode, ".HK") {
		stockCode = "hk" + strings.TrimSuffix(stockCode, ".HK")
	} else if strings.HasSuffix(stockCode, ".BJ") {
		stockCode = "bj" + strings.TrimSuffix(stockCode, ".BJ")
	} else if strings.HasPrefix(stockCode, "GB_") {
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	lowerStockCode := normalizeStockCode(stockCode)

	var stock FollowedStock
	if err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", lowerStockCode).First(&stock).Error; err != nil {
		return "股票未关注"
	}

	updates := &map[string]any{
		"entry_price":       entryPrice,
		"take_profit_price": takeProfitPrice,
		"stop_loss_price":   stopLossPrice,
		"cost_price":        costPrice,
	}
	result := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", lowerStockCode).Updates(updates)
	if result.Error != nil {
		return "设置失败"
	}
	if result.RowsAffected == 0 {
		return "设置失败"
	}
	return "设置成功"
}
func (receiver StockDataApi) GetFollowList(groupId int) *[]FollowedStock {
	//logger.SugaredLogger.Infof("GetFollowList %d", groupId)

	var result *[]FollowedStock
	if groupId == 0 {
		db.Dao.Model(&FollowedStock{}).Order("sort asc,time desc").Find(&result)
	} else {
		infos := NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
		codes := lo.FlatMap(infos, func(info GroupStock, idx int) []string {
			return []string{info.StockCode}
		})
		db.Dao.Model(&FollowedStock{}).Where("stock_code in ?", codes).Order("sort asc,time desc").Find(&result)
		//logger.SugaredLogger.Infof("GetFollowList %+v", result)
	}
	return result
}

func (receiver StockDataApi) GetStockList(key string) []StockBasic {
	var result []StockBasic
	db.Dao.Model(&StockBasic{}).Where("name like ? or ts_code like ?", "%"+key+"%", "%"+key+"%").Find(&result)
	var result2 []IndexBasic
	db.Dao.Model(&IndexBasic{}).Where("market in ?", []string{"SSE", "SZSE"}).Where("name like ? or ts_code like ?", "%"+key+"%", "%"+key+"%").Find(&result2)

	var result3 []models.StockInfoHK
	db.Dao.Model(&models.StockInfoHK{}).Where("name like ? or code like ?", "%"+key+"%", "%"+key+"%").Find(&result3)

	var result4 []models.StockInfoUS
	db.Dao.Model(&models.StockInfoUS{}).Where("name like ? or code like ? or e_name like ?", "%"+key+"%", "%"+key+"%", "%"+key+"%").Find(&result4)

	var result5 []models.AllStockInfo
	db.Dao.Model(&models.AllStockInfo{}).Where("secucode like ? or sec_uri_tynameabbr like ?", "%"+key+"%", "%"+key+"%").Find(&result5)

	// 创建一个 map 来存储已存在的股票，用于去重
	// 使用 TsCode 作为唯一标识符
	existingStocks := make(map[string]bool)
	for _, item := range result {
		existingStocks[item.TsCode] = true
	}
	for _, item := range result2 {
		if existingStocks[item.TsCode] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.TsCode,
			Name:     item.Name,
			Fullname: item.FullName,
			Symbol:   item.Symbol,
			Market:   item.Market,
			ListDate: item.ListDate,
		})
		existingStocks[item.TsCode] = true

	}
	for _, item := range result3 {
		if existingStocks[item.Code] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.Code,
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "HK",
		})
		existingStocks[item.Code] = true
	}
	for _, item := range result4 {
		code := strings.ToLower(strings.Replace(item.Code, "us", "gb_", 1))
		if existingStocks[code] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   code,
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "US",
		})
		existingStocks[code] = true
	}
	for _, item := range result5 {
		if existingStocks[item.SECUCODE] {
			continue
		}
		result = append(result, StockBasic{
			TsCode:   item.SECUCODE,
			Name:     item.SECURITYNAMEABBR,
			Fullname: item.SECURITYNAMEABBR,
			Market:   item.MARKET,
		})
		existingStocks[item.SECUCODE] = true
	}

	// 场内基金（ETF）也纳入搜索：GetFundList 会在本地 FundBasic 缺失时触发东方财富在线搜索并缓存。
	// 这样即使通达信同步未覆盖 ETF（未重启 / 同步失败 / 本地无缓存），也能立即搜到如 513310 中韩半导体。
	if key != "" {
		funds := NewFundApi().GetFundList(key)
		for _, fund := range funds {
			if !IsOnExchangeFund(fund.Code) {
				continue
			}
			tsCode := fundCodeToTsCode(fund.Code)
			if tsCode == "" || existingStocks[tsCode] {
				continue
			}
			result = append(result, StockBasic{
				TsCode:   tsCode,
				Name:     fund.Name,
				Fullname: fund.FullName,
				Market:   tsCode[len(tsCode)-2:],
			})
			existingStocks[tsCode] = true
		}
	}

	return result
}

// fundCodeToTsCode 场内基金纯代码转 ts_code（如 513310 → 513310.SH，159915 → 159915.SZ）
func fundCodeToTsCode(code string) string {
	if len(code) == 0 {
		return ""
	}
	if strings.HasPrefix(code, "5") || strings.HasPrefix(code, "6") {
		return code + ".SH"
	}
	if strings.HasPrefix(code, "1") || strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
		return code + ".SZ"
	}
	return code + ".SH"
}

func (receiver StockDataApi) GetFollowedStockByStockCode(code string) FollowedStock {
	var result FollowedStock
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", normalizeStockCode(code)).First(&result)
	return result
}

// GB18030ToUTF8 GB18030 转换为 UTF8
func GB18030ToUTF8(bs []byte) string {
	reader := transform.NewReader(bytes.NewReader(bs), simplifiedchinese.GB18030.NewDecoder())
	d, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	return string(d)
}

func ParseTxStockData(data string) (*StockInfo, error) {
	//v_r_hk09660="100~地平线机器人-W~09660~6.240~5.690~5.800~192659034.0~0~0~6.240~0~0~0~0~0~0~0~0~0~6.240~0~0~0~0~0~0~0~0~0~192659034.0~2025/04/29
	//13:41:04~0.550~9.67~6.450~5.710~6.240~192659034.0~1180471843.140~0~32.51~~0~0~13.01~691.1364~823.6983~HORIZONROBOT-W~0.00~10.380~3.320~1.07~-16.03~0~0~0~0~0~32.51~6.40~1.74~600~73.33~17.96~GP~19.70~11.51~-0.95~-18.54~44.44~13200293682.00~11075904412.00~32.51~0.000~6.127~56.39~HKD~1~30";
	//v_sz002241="51~歌尔股份~002241~22.26~22.27~0.00~0~0~0~22.26~1004~0.00~0~0.00~0~0.00~0~0.00~0~22.26~1004~0.00~558~0.00~0~0.00~0~0.00~0~~20250509092233~-0.01~-0.04~0.00~0.00~22.26/0/0~0~0~0.00~28.21~~0.00~0.00~0.00~686.46~777.09~2.31~24.50~20.04~0.00~-558~0.00~41.44~29.16~~~1.24~0.0000~0.0000~0~
	//~GP-A~-13.75~6.76~1.09~8.18~3.39~30.63~15.70~6.87~17.47~-23.95~3083811231~3490989083~-21.75~12.02~3083811231~~~39.36~-0.04~~CNY~0~~0.00~0";

	datas := strutil.SplitAndTrim(data, "=", "\"")
	if len(datas) < 2 {
		return nil, fmt.Errorf("invalid data format")
	}
	var result map[string]string
	var err error
	if strutil.ContainsAny(datas[0], []string{"v_r_hk", "v_hk", "v_sz", "v_sh"}) {
		result, err = ParseTxHKStockData(datas)
	}

	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)
	marshal, err := json.Marshal(result)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成marshal: %s", marshal)
	stockInfo := &StockInfo{}
	err = json.Unmarshal(marshal, &stockInfo)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成stockInfo: %+v", stockInfo)

	return stockInfo, nil

}

func ParseTxHKStockData(datas []string) (map[string]string, error) {
	//v_r_hk09660="
	//100~   0
	//地平线机器人-W~  1
	//09660~ 2
	//6.270~ 3 当前价
	//5.690~ 4 昨收价
	//5.800~ 5 开盘价
	//195083034.0~
	//0~
	//0~
	//6.270~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//0~
	//6.270~
	//0~0~0~0~0~0~0~0~0~
	//195083034.0~
	//2025/04/29 13:45:41~  30 当前时间
	//0.580~
	//10.19~
	//6.450~  最高价
	//5.710~  最低价
	//6.270~
	//195083034.0~
	//1195673623.140~
	//0~
	//32.66
	//~~0~0~13.01~694.4592~827.6584~HORIZONROBOT-W~0.00~10.380~3.320~1.06~-18.71~0~0~0~0~0~32.66~6.43~1.76~600~74.17~18.53~GP~19.70~11.51~-0.48~-18.15~45.14~13200293682.00~11075904412.00~32.66~0.000~6.129~57.14~HKD~1~30";
	result := make(map[string]string)

	stockCode := strutil.ReplaceWithMap(datas[0], map[string]string{
		"v_r_": "",
		"v_":   "",
	})
	result["股票代码"] = stockCode

	parts := strutil.SplitAndTrim(datas[1], "~")
	//logger.SugaredLogger.Infof("股票数据解析完成 len: %v", len(parts))
	if len(parts) < 35 {
		return nil, fmt.Errorf("invalid data format")
	}
	result["股票名称"] = parts[1]
	result["当前价格"] = parts[3]
	result["昨日收盘价"] = parts[4]
	result["今日开盘价"] = parts[5]

	result["今日最高价"] = parts[33]
	result["今日最低价"] = parts[34]

	if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
		result["买一报价"] = parts[9]
		result["买一申报"] = parts[10]
		result["买二报价"] = parts[11]
		result["买二申报"] = parts[12]
		result["买三报价"] = parts[13]
		result["买三申报"] = parts[14]
		result["买四报价"] = parts[15]
		result["买四申报"] = parts[16]
		result["买五报价"] = parts[17]
		result["买五申报"] = parts[18]

		result["卖一报价"] = parts[19]
		result["卖一申报"] = parts[20]
		result["卖二报价"] = parts[21]
		result["卖二申报"] = parts[22]
		result["卖三报价"] = parts[23]
		result["卖三申报"] = parts[24]
		result["卖四报价"] = parts[25]
		result["卖四申报"] = parts[26]
		result["卖五报价"] = parts[27]
		result["卖五申报"] = parts[28]

	}

	timestr := ""

	if strutil.ContainsAny(parts[30], []string{"/"}) {
		timestr = strutil.ReplaceWithMap(parts[30], map[string]string{
			"/":  "-",
			"\n": " ",
		})
		result["日期"] = strutil.SplitAndTrim(timestr, " ", "")[0]
		result["时间"] = strutil.SplitAndTrim(timestr, " ", "")[1]
	} else {
		result["日期"] = strutil.Trim(parts[29])[0:4] + "-" + strutil.Trim(parts[29])[4:6] + "-" + strutil.Trim(parts[29])[6:8]
		result["时间"] = strutil.Trim(parts[29])[8:10] + ":" + strutil.Trim(parts[29])[10:12] + ":" + strutil.Trim(parts[29])[12:14]
		result["今日最高价"] = parts[32]
		result["今日最低价"] = parts[33]
	}
	//logger.SugaredLogger.Infof("股票数据解析完成 %s %s 时间: %s,%s", parts[1], parts[3], parts[29], parts[30])

	//logger.SugaredLogger.Infof("股票数据解析完成 时间: %v", timestr)

	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)

	return result, nil
}

func ParseFullSingleStockData(data string) (*StockInfo, error) {
	datas := strutil.SplitAndTrim(data, "=", "\"")
	if len(datas) < 2 {
		return nil, fmt.Errorf("invalid data format")
	}
	var result map[string]string
	var err error
	if strutil.ContainsAny(datas[0], []string{"hq_str_sz", "hq_str_sh", "hq_str_bj", "hq_str_sb"}) {
		result, err = ParseSHSZStockData(datas)
	}
	if strutil.ContainsAny(datas[0], []string{"hq_str_hk"}) {
		result, err = ParseHKStockData(datas)
	}
	if strutil.ContainsAny(datas[0], []string{"hq_str_gb"}) {
		result, err = ParseUSStockData(datas)
	}

	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)
	marshal, err := json.Marshal(result)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成marshal: %s", marshal)
	stockInfo := &StockInfo{}
	err = json.Unmarshal(marshal, &stockInfo)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%s", err.Error())
		return nil, err
	}
	//logger.SugaredLogger.Infof("股票数据解析完成stockInfo: %+v", stockInfo)

	return stockInfo, nil
}

func ParseUSStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"", ";")
	//parts := strings.Split(data, ",")
	//logger.SugaredLogger.Infof("股票数据解析完成: parts:%d", len(parts))
	if len(parts) < 35 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		谷歌,   0
		170.2100, 1 现价
		-2.57, 2 涨跌幅
		2025-02-28 09:38:50, 3 时间
		-4.4900, 4 涨跌额
		175.9400, 5 今日开盘价
		176.5900, 6 区间
		169.7520, 7 区间
		208.7000, 8 52周区间
		130.9500, 9 52周区间
		25930485, 10 成交量
		17083496, 11 10日均量
		2074859900000, 12 市值
		8.13, 13 每股收益
		20.940000 , 14 市盈率
		0.00,  15
		0.00,  16
		0.20,  17
		0.00,	18
		12190000000, 19
		71, 20
		170.2000, 21 盘前盘后盘
		-0.01, 22  盘前盘后涨跌幅
		-0.01, 23
		Feb 27 07:59PM EST, 24
		Feb 27 04:00PM EST, 25
		174.7000, 26 前收盘
		2917444, 27
		1, 28
		2025, 29
		4456143849.0000, 30
		176.1200, 31
		163.7039, 32
		496605933.1411, 33
		170.2100, 34 现价
		174.7000 35 前收盘
	*/
	result["股票代码"] = code
	result["股票名称"] = parts[0]
	result["今日开盘价"] = parts[5]

	if len(parts) >= 36 {
		result["昨日收盘价"] = strutil.ReplaceWithMap(strutil.RemoveNonPrintable(parts[26]), map[string]string{"\"": "", ";": ""})
	} else {
		result["昨日收盘价"] = strutil.ReplaceWithMap(strutil.RemoveNonPrintable(parts[len(parts)-1]), map[string]string{"\"": "", ";": ""})
	}

	result["今日最高价"] = parts[6]
	result["今日最低价"] = parts[7]
	result["当前价格"] = parts[1]
	result["盘前盘后"] = parts[21]
	result["盘前盘后涨跌幅"] = parts[22]
	result["日期"] = strutil.SplitAndTrim(parts[3], " ", "")[0]
	result["时间"] = strutil.SplitAndTrim(parts[3], " ", "")[1]
	//logger.SugaredLogger.Infof("美股股票数据解析完成: %v", result)
	return result, nil
}

func ParseHKStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"", ";")
	//parts := strings.Split(data, ",")
	if len(parts) < 19 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		XIAOMI-W,    0
		小米集团－Ｗ,  1 股票名称
		50.050,		 2 今日开盘价
		49.150,		 3 昨日收盘价
		51.950,      4 今日最高价
		49.700,      5 今日最低价
		51.700,      6 当前价格
		2.550,       7 涨跌额
		5.188,		 8 涨跌幅
		51.65000,    9
		51.70000,    10
		15770408249, 11 成交额
		308362585,   12 成交量
		0.000,       13
		0.000,       14
		51.950,		 15 52周最高
		12.560,		 16 52周最低
		2025/02/21,  17
		16:08        18
	*/
	result["股票代码"] = code
	result["股票名称"] = parts[1]
	result["今日开盘价"] = parts[2]
	result["昨日收盘价"] = parts[3]
	result["今日最高价"] = parts[4]
	result["今日最低价"] = parts[5]
	result["当前价格"] = parts[6]
	result["日期"] = strings.ReplaceAll(parts[17], "/", "-")
	result["时间"] = strings.ReplaceAll(parts[18], "\";", ":00")
	//logger.SugaredLogger.Infof("股票数据解析完成: %v", result)
	return result, nil
}

func ParseSHSZStockData(datas []string) (map[string]string, error) {
	code := strings.Split(datas[0], "hq_str_")[1]
	result := make(map[string]string)
	parts := strutil.SplitAndTrim(datas[1], ",", "\"")
	//parts := strings.Split(data, ",")
	if len(parts) < 32 {
		return nil, fmt.Errorf("invalid data format")
	}
	/*
		0：”大秦铁路”，股票名字；
		1：”27.55″，今日开盘价；
		2：”27.25″，昨日收盘价；
		3：”26.91″，当前价格；
		4：”27.55″，今日最高价；
		5：”26.20″，今日最低价；
		6：”26.91″，竞买价，即“买一”报价；
		7：”26.92″，竞卖价，即“卖一”报价；
		8：”22114263″，成交的股票数，由于股票交易以一百股为基本单位，所以在使用时，通常把该值除以一百；
		9：”589824680″，成交金额，单位为“元”，为了一目了然，通常以“万元”为成交金额的单位，所以通常把该值除以一万；
		10：”4695″，“买一”申报4695股，即47手；
		11：”26.91″，“买一”报价；
		12：”57590″，“买二”
		13：”26.90″，“买二”
		14：”14700″，“买三”
		15：”26.89″，“买三”
		16：”14300″，“买四”
		17：”26.88″，“买四”
		18：”15100″，“买五”
		19：”26.87″，“买五”
		20：”3100″，“卖一”申报3100股，即31手；
		21：”26.92″，“卖一”报价
		(22, 23), (24, 25), (26,27), (28, 29)分别为“卖二”至“卖四的情况”
		30：”2008-01-11″，日期；
		31：”15:05:32″，时间；*/
	result["股票代码"] = code
	result["股票名称"] = parts[0]
	result["今日开盘价"] = parts[1]
	result["昨日收盘价"] = parts[2]
	result["当前价格"] = parts[3]
	result["今日最高价"] = parts[4]
	result["今日最低价"] = parts[5]
	result["竞买价"] = parts[6]
	result["竞卖价"] = parts[7]
	result["成交的股票数"] = parts[8]
	result["成交金额"] = parts[9]
	result["买一申报"] = parts[10]
	result["买一报价"] = parts[11]
	result["买二申报"] = parts[12]
	result["买二报价"] = parts[13]
	result["买三申报"] = parts[14]
	result["买三报价"] = parts[15]
	result["买四申报"] = parts[16]
	result["买四报价"] = parts[17]
	result["买五申报"] = parts[18]
	result["买五报价"] = parts[19]
	result["卖一申报"] = parts[20]
	result["卖一报价"] = parts[21]
	result["卖二申报"] = parts[22]
	result["卖二报价"] = parts[23]
	result["卖三申报"] = parts[24]
	result["卖三报价"] = parts[25]
	result["卖四申报"] = parts[26]
	result["卖四报价"] = parts[27]
	result["卖五申报"] = parts[28]
	result["卖五报价"] = parts[29]
	result["日期"] = parts[30]
	result["时间"] = parts[31]
	return result, nil
}

type IndexBasic struct {
	gorm.Model
	TsCode        string  `json:"ts_code" gorm:"index"`
	Symbol        string  `json:"symbol" gorm:"index"`
	Name          string  `json:"name" gorm:"index"`
	FullName      string  `json:"fullname"`
	IndexType     string  `json:"index_type"`
	IndexCategory string  `json:"category"`
	Market        string  `json:"market"`
	ListDate      string  `json:"list_date"`
	BaseDate      string  `json:"base_date"`
	BasePoint     float64 `json:"base_point"`
	Publisher     string  `json:"publisher"`
	WeightRule    string  `json:"weight_rule"`
	DESC          string  `json:"desc"`
}

func (IndexBasic) TableName() string {
	return "tushare_index_basic"
}

type RealTimeStockPriceInfo struct {
	StockCode string
	Price     string `json:"当前价格"`
	Time      time.Time
}

func GetRealTimeStockPriceInfo(ctx context.Context, stockCode string) (price, priceTime string) {
	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz"}) {
		crawlerAPI := CrawlerApi{}
		crawlerBaseInfo := CrawlerBaseInfo{
			Name:        "EastmoneyCrawler",
			Description: "EastmoneyCrawler Description",
			BaseUrl:     "https://quote.eastmoney.com/",
			Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
		}
		crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)
		htmlContent, ok := crawlerAPI.GetHtml(fmt.Sprintf("https://quote.eastmoney.com/%s.html", stockCode), "div.zxj", true)
		if ok {
			price := ""
			priceTime := ""
			document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
			if err != nil {
				//logger.SugaredLogger.Errorf("GetRealTimeStockPriceInfo error: %v", err)
			}
			document.Find("div.zxj").Each(func(i int, selection *goquery.Selection) {
				price = selection.Text()
				//logger.SugaredLogger.Infof("股票代码: %s, 当前价格: %s", stockCode, price)
			})

			document.Find("span.quote_title_time").Each(func(i int, selection *goquery.Selection) {
				priceTime = selection.Text()
				//logger.SugaredLogger.Infof("股票代码: %s, 当前价格时间: %s", stockCode, priceTime)
			})
			return price, priceTime
		}
	}
	return price, priceTime
}

func SearchStockPriceInfo(stockName, stockCode string, crawlTimeOut int64) *[]string {

	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz", "bj"}) {
		//if strutil.HasPrefixAny(stockCode, []string{"bj", "BJ"}) {
		//	stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
		//		"bj": "",
		//		"BJ": "",
		//	}) + ".BJ"
		//}

		return getSHSZStockPriceInfo(stockName, stockCode, crawlTimeOut)
	}
	if strutil.HasPrefixAny(stockCode, []string{"HK", "hk"}) {
		return getHKStockPriceInfo(stockCode, crawlTimeOut)
	}
	if strutil.HasPrefixAny(stockCode, []string{"US", "us", "gb_"}) {
		return getUSStockPriceInfo(stockCode, crawlTimeOut)
	}
	return &[]string{}
}

func getUSStockPriceInfo(stockCode string, crawlTimeOut int64) *[]string {
	var messages []string
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "SinaCrawler",
		Description: "SinaCrawler Crawler Description",
		BaseUrl:     "https://stock.finance.sina.com.cn",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	url := fmt.Sprintf("https://stock.finance.sina.com.cn/usstock/quotes/%s.html", strings.ReplaceAll(stockCode, "gb_", ""))
	htmlContent, ok := crawlerAPI.GetHtml(url, "div#hqPrice", true)
	if !ok {
		return &[]string{}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	stockName := ""
	stockPrice := ""
	stockPriceTime := ""
	document.Find("div.hq_title >h1").Each(func(i int, selection *goquery.Selection) {
		stockName = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("股票名称-:%s", stockName)
	})

	document.Find("#hqPrice").Each(func(i int, selection *goquery.Selection) {
		stockPrice = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("现价: %s", stockPrice)
	})

	document.Find("div.hq_time").Each(func(i int, selection *goquery.Selection) {
		stockPriceTime = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("时间: %s", stockPriceTime)
	})

	messages = append(messages, fmt.Sprintf("%s:%s现价%s", stockPriceTime, stockName, stockPrice))
	//logger.SugaredLogger.Infof("股票: %s", messages)

	document.Find("div#hqDetails >table tbody tr").Each(func(i int, selection *goquery.Selection) {
		text := strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("股票名称-%s: %s", stockName, text)
		messages = append(messages, text)
	})

	//logger.SugaredLogger.Infof("messages: %s", messages)
	return &messages
}

func getHKStockPriceInfo(stockCode string, crawlTimeOut int64) *[]string {
	var messages []string
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "SinaCrawler",
		Description: "SinaCrawler Crawler Description",
		BaseUrl:     "https://stock.finance.sina.com.cn",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	url := fmt.Sprintf("https://stock.finance.sina.com.cn/hkstock/quotes/%s.html", strings.ReplaceAll(stockCode, "hk", ""))
	//logger.SugaredLogger.Infof("CrawlHKStockPriceInfo url:%s", url)
	htmlContent, ok := crawlerAPI.GetHtml(url, "div.deta_hqContainer >.deta03>ul ", false)
	if !ok {
		return &[]string{}
	}
	//logger.SugaredLogger.Infof("CrawlHKStockPriceInfo htmlContent:%s", htmlContent)
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	stockName := ""
	stockPrice := ""
	stockPriceTime := ""
	document.Find("#stock_cname").Each(func(i int, selection *goquery.Selection) {
		stockName = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("股票名称-:%s", stockName)
	})

	document.Find("#mts_stock_hk_price").Each(func(i int, selection *goquery.Selection) {
		stockPrice = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("现价: %s", stockPrice)
	})

	document.Find("#mts_stock_hk_time").Each(func(i int, selection *goquery.Selection) {
		stockPriceTime = strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("时间: %s", stockPriceTime)
	})

	messages = append(messages, fmt.Sprintf("%s:%s现价%s", stockPriceTime, stockName, stockPrice))
	//logger.SugaredLogger.Infof("股票: %s", messages)

	document.Find(".deta_hqContainer >.deta03 li").Each(func(i int, selection *goquery.Selection) {
		text := strutil.RemoveNonPrintable(selection.Text())
		//logger.SugaredLogger.Infof("股票名称-%s: %s", stockName, text)
		messages = append(messages, text)
	})

	//logger.SugaredLogger.Infof("messages: %s", messages)
	return &messages
}

func GetZSInfo(name, stockCode string, crawlTimeOut int64) string {
	url := "https://finance.sina.com.cn/realstock/company/" + stockCode + "/nc.shtml"
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://finance.sina.com.cn",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)
	html, ok := crawlerAPI.GetHtml(url, "div#hqDetails table", true)
	if !ok {
		return ""
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}

	//price
	price := strutil.RemoveWhiteSpace(document.Find("div#price").First().Text(), false)
	hqTime := strutil.RemoveWhiteSpace(document.Find("div#hqTime").First().Text(), false)

	if strutil.ContainsAny(price, []string{"-", "--"}) {
		return "暂无数据"
	}

	var markdown strings.Builder
	markdown.WriteString(fmt.Sprintf("### 时间：%s %s：%s \n", hqTime, name, price))
	GetTableMarkdown(document, "div#hqDetails table", &markdown)
	return markdown.String()
}

func getSHSZStockPriceInfo(stockName, stockCode string, crawlTimeOut int64) *[]string {
	url := "https://finance.sina.com.cn/realstock/company/" + stockCode + "/nc.shtml"
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://finance.sina.com.cn",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)
	html, ok := crawlerAPI.GetHtml(url, "div#hqDetails table", true)
	if !ok {
		return &[]string{""}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}

	//price
	price := strutil.RemoveWhiteSpace(document.Find("div#price").First().Text(), false)
	hqTime := strutil.RemoveWhiteSpace(document.Find("div#hqTime").First().Text(), false)

	var markdown strings.Builder
	markdown.WriteString(fmt.Sprintf("### %s现价：%s 现价时间：%s\n", stockName, price, hqTime))
	GetTableMarkdown(document, "div#hqDetails table", &markdown)
	return &[]string{markdown.String()}
}

func SearchStockInfo(stock, msgType string, crawlTimeOut int64) *[]string {
	crawler := CrawlerApi{
		crawlerBaseInfo: CrawlerBaseInfo{

			Name:        "财联社",
			BaseUrl:     "https://www.cls.cn",
			Description: "财联社",
			Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
		},
	}
	timeoutCtx, timeoutCtxCancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer timeoutCtxCancel()
	crawler = crawler.NewCrawler(timeoutCtx, crawler.crawlerBaseInfo)
	url := fmt.Sprintf("https://www.cls.cn/searchPage?keyword=%s&type=%s", RemoveAllBlankChar(stock), msgType)
	//logger.SugaredLogger.Infof("SearchStockInfo url:%s", url)
	waitVisible := ".search-telegraph-list,.subject-interest-list"
	htmlContent, ok := crawler.GetHtml(url, waitVisible, true)
	if !ok {
		return &[]string{}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return &[]string{}
	}
	var messages []string
	document.Find(waitVisible).Each(func(i int, selection *goquery.Selection) {
		text := strutil.RemoveNonPrintable(selection.Text())
		messages = append(messages, ReplaceSensitiveWords(text))
		//logger.SugaredLogger.Infof("搜索到消息-%s: %s", msgType, text)
	})
	return &messages
}

func SearchStockInfoByCode(stock string) *[]string {
	// 创建一个 chromedp 上下文
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(logger.SugaredLogger.Infof),
		chromedp.WithErrorf(logger.SugaredLogger.Errorf),
	)
	defer cancel()
	var htmlContent string
	stock = strings.ReplaceAll(stock, "sh", "")
	stock = strings.ReplaceAll(stock, "sz", "")
	url := fmt.Sprintf("https://gushitong.baidu.com/stock/ab-%s", stock)
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		// 等待页面加载完成，可以根据需要调整等待时间
		//chromedp.Sleep(3*time.Second),
		chromedp.WaitVisible("a.news-item-link", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return &[]string{}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return &[]string{}
	}
	var messages []string
	document.Find("a.news-item-link").Each(func(i int, selection *goquery.Selection) {
		text := strutil.RemoveNonPrintable(selection.Text())
		if strings.Contains(text, stock) {
			messages = append(messages, text)
			//logger.SugaredLogger.Infof("搜索到消息: %s", text)
		}
	})
	return &messages
}

// 分时数据
func (receiver StockDataApi) GetStockMinutePriceData(stockCode string) (*[]MinuteData, string) {

	stockCode = ConvertTushareCodeToStockCode(stockCode)

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/minute/query?code=%s", stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_", "GB_"}) {
		stockCode = strings.Replace(strings.ToUpper(stockCode), "GB_", "us", 1) + ".OQ"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US"}) {
		url = fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/UsMinute/query?code=%s", stockCode)
	}
	//logger.SugaredLogger.Infof("GetStockMinutePriceData url:%s", url)
	res := make(map[string]interface{})
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)

	date := ""
	minuteDatas := &[]MinuteData{}

	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return minuteDatas, date
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if res["data"] != nil && code == 0 {
		data := res["data"].(map[string]interface{})
		if stockData, ok := data[stockCode]; ok {
			m := stockData.(map[string]interface{})
			if d, ok := m["data"]; ok {
				if m2, ok := d.(map[string]any); ok {
					minutePriceData := m2["data"]
					datas := minutePriceData.([]any)
					for _, item := range datas {
						minuteDataSplit := strutil.SplitEx(strutil.ReplaceWithMap(item.(string), map[string]string{
							"\r\n": " ",
						}), " ", true)
						price, _ := convertor.ToFloat(minuteDataSplit[1])
						volume, _ := convertor.ToFloat(minuteDataSplit[2])
						amount := float64(0)
						if len(minuteDataSplit) >= 4 {
							amount, _ = convertor.ToFloat(minuteDataSplit[3])
						}
						minuteData := &MinuteData{
							Time:   minuteDataSplit[0][0:2] + ":" + minuteDataSplit[0][2:4],
							Price:  price,
							Volume: volume,
							Amount: amount,
						}
						*minuteDatas = append(*minuteDatas, *minuteData)
					}
					date = m2["date"].(string)
				}
			}
		}
	}
	return minuteDatas, date
}

func (receiver StockDataApi) GetKLineData(stockCode string, kLineType string, days int64) *[]KLineData {
	url := fmt.Sprintf("http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=yes&datalen=%d", stockCode, kLineType, days)
	K := &[]KLineData{}
	_, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "quotes.sina.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		SetResult(K).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	return K
}
func (receiver StockDataApi) GetHK_KLineData(stockCode string, kLineType string, days int64) *[]KLineData {

	//logger.SugaredLogger.Infof("GetHK_KLineData stockCode:%s,kLineType:%s,days:%d", stockCode, kLineType, days)
	if strutil.HasPrefixAny(stockCode, []string{"gb_", "GB_"}) {
		stockCode = strings.Replace(stockCode, "gb_", "us", 1) + ".OQ"
	}

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", stockCode, kLineType, days)
	//logger.SugaredLogger.Infof("url:%s", url)
	K := &[]KLineData{}
	res := make(map[string]interface{})
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if code != 0 {
		return K
	}
	if res["data"] != nil && code == 0 {
		data := res["data"].(map[string]interface{})[stockCode].(map[string]interface{})
		if data != nil {
			var day []any
			if data["qfqday"] != nil {
				day = data["qfqday"].([]any)
			}
			if data["day"] != nil {
				day = data["day"].([]any)
			}
			for _, v := range day {
				if v != nil {
					vv := v.([]any)
					KLine := &KLineData{
						Day:    convertor.ToString(vv[0]),
						Open:   convertor.ToString(vv[1]),
						Close:  convertor.ToString(vv[2]),
						High:   convertor.ToString(vv[3]),
						Low:    convertor.ToString(vv[4]),
						Volume: convertor.ToString(vv[5]),
					}
					*K = append(*K, *KLine)
				}
			}
		}
	}
	return K
}

func getSinaStockInfo(receiver StockDataApi, page, pageSize int) *[]models.SinaStockInfo {
	infos := &[]models.SinaStockInfo{}
	url := "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHKStockData?page=%d&num=%d&sort=symbol&asc=1&node=qbgg_hk&_s_r_a=init"
	_, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).SetProxy("http://localhost:10809").R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		SetResult(infos).
		Get(fmt.Sprintf(url, page, pageSize))

	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	return infos
}

func (receiver StockDataApi) getDCStockInfo(market string, page, pageSize int) {
	//m:105,m:106,m:107  //美股
	//m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2 //港股
	fs := "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048"
	switch market {
	case "hk":
		fs = "m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2"
	case "us":
		fs = "m:105,m:106,m:107"
	}

	url := "https://push2.eastmoney.com/api/qt/clist/get?np=1&fltt=1&invt=2&cb=data&fs=%s&fields=f12,f13,f14,f1,f2,f4,f3,f152,f5,f6,f7,f15,f18,f16,f17,f10,f8,f9,f23,f100,f265&fid=f3&pn=%d&pz=%d&po=1&dect=1&wbp2u=|0|0|0|web&_=%d"
	sprintfUrl := fmt.Sprintf(url, fs, page, pageSize, time.Now().UnixMilli())
	//logger.SugaredLogger.Infof("page:%d  url:%s", page, sprintfUrl)
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "push2.eastmoney.com").
		SetHeader("Referer", "https://quote.eastmoney.com/center/gridlist.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:146.0) Gecko/20100101 Firefox/146.0").
		Get(sprintfUrl)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("vm.Run error:%v", err.Error())
	}
	value, _ := val.Object().Value().Export()
	marshal, err := json.Marshal(value)
	data := make(map[string]any)
	err = json.Unmarshal(marshal, &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	//logger.SugaredLogger.Infof("resp:%s", data)
	if data["data"] != nil {
		datas := data["data"].(map[string]any)
		_ = datas["total"].(float64)
		diff := datas["diff"].([]any)
		//logger.SugaredLogger.Infof("total:%d", int(total))
		for _, item := range diff {
			stock := item.(map[string]any)
			//logger.SugaredLogger.Infof("k:%d,%s:%s:%s %s:%s", k, stock["f14"], stock["f12"], DCToTsCode(stock["f12"].(string)), stock["f100"], stock["f265"])

			if market == "" {
				stockInfo := &StockBasic{
					Symbol: stock["f12"].(string),
					TsCode: DCToTsCode(stock["f12"].(string)),
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&StockBasic{}).Create(stockInfo)
				} else {
					stockInfo = &StockBasic{
						Symbol: stock["f12"].(string),
						TsCode: DCToTsCode(stock["f12"].(string)),
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).Updates(stockInfo)
				}
			}

			if market == "hk" {
				stockInfo := &models.StockInfoHK{
					Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".HK",
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
				} else {
					stockInfo = &models.StockInfoHK{
						Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".HK",
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stockInfo.Code).Updates(stockInfo)
				}
			}

			if market == "us" {
				stockInfo := &models.StockInfoUS{
					Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".US",
					Name:   stock["f14"].(string),
					BKName: stock["f100"].(string),
					BKCode: stock["f265"].(string),
				}
				db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				//logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
				} else {
					stockInfo = &models.StockInfoUS{
						Code:   strutil.PadStart(stock["f12"].(string), 5, "0") + ".US",
						Name:   stock["f14"].(string),
						BKName: stock["f100"].(string),
						BKCode: stock["f265"].(string),
					}
					db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stockInfo.Code).Updates(stockInfo)
				}
			}

		}

	}
}

func DCToTsCode(dcCode string) string {
	//北京证券交易所	8（83、87、88 等）	创新型中小企业（专精特新为主）
	//上海证券交易所	6（60、688 等）	大盘蓝筹、科创板（高新技术）
	//深圳证券交易所	0、3（000、002、30 等）	中小盘、创业板（成长型创新企业）
	switch dcCode[0:1] {
	case "8":
		return dcCode + ".BJ"
	case "9":
		return dcCode + ".BJ"
	case "6":
		return dcCode + ".SH"
	case "0":
		return dcCode + ".SZ"
	case "3":
		return dcCode + ".SZ"
	}
	return ""
}

func (receiver StockDataApi) GetHKStockInfo(pageSize int) {
	url := "https://stock.gtimg.cn/data/hk_rank.php?board=main_all&metric=price&pageSize=%d&reqPage=1&order=desc&var_name=list_data"
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "stock.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(fmt.Sprintf(url, pageSize))
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return
	}
	js := "var " + string(resp.Body())
	vm := otto.New()
	_, err = vm.Run(js)
	_, err = vm.Run("var data = JSON.stringify(list_data);")
	if err != nil {
		return
	}
	value, err := vm.Get("data")
	data := make(map[string]any)
	err = json.Unmarshal([]byte(value.String()), &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	//logger.SugaredLogger.Infof("resp:%s", data)
	if data["code"] != nil && data["code"].(float64) == 0 {
		d := data["data"].(map[string]any)
		saveHKStockInfo(d)

		page_count := int64(d["page_count"].(float64))
		//logger.SugaredLogger.Infof("page_count:%d", page_count)
		page := int64(1)
		for page > page_count {
			urlx := fmt.Sprintf("https://stock.gtimg.cn/data/hk_rank.php?board=main_all&metric=price&pageSize=%d&reqPage=%d&order=desc&var_name=list_data", pageSize, page)
			//logger.SugaredLogger.Infof("url:%s", urlx)
			resp, err = receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
				SetHeader("Host", "stock.gtimg.cn").
				SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
				Get(urlx)
			if err != nil {
				//logger.SugaredLogger.Errorf("err:%s", err.Error())
				break
			}
			js = "var " + string(resp.Body())
			_, err = vm.Run(js)
			_, err = vm.Run("var data = JSON.stringify(list_data);")
			if err != nil {
				return
			}
			value, err = vm.Get("data")
			data = make(map[string]any)
			err = json.Unmarshal([]byte(value.String()), &data)
			if err != nil {
				//logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
			}
			//logger.SugaredLogger.Infof("resp:%s", data)
			if data != nil && data["code"] != nil && data["code"].(float64) == 0 {
				if data["data"] != nil {
					d = data["data"].(map[string]any)
					saveHKStockInfo(d)
				}
			}
			page++
		}
		//
	}

}

func saveHKStockInfo(d map[string]any) {
	for _, v := range d["page_data"].([]any) {
		vv := v.(string)
		splits := strings.Split(vv, "~")
		stock := &models.StockInfoHK{
			Code: strutil.PadStart(splits[0], 5, "0") + ".HK",
			Name: splits[1],
		}
		//logger.SugaredLogger.Infof("vv:%s", vv)
		db.Dao.Model(stock).Where("code = ?", stock.Code).First(stock)
		if stock.ID == 0 {
			//logger.SugaredLogger.Infof("stock:%+v", stock)
			db.Dao.Model(&models.StockInfoHK{}).Create(stock)
		}
	}
}

func (receiver StockDataApi) GetCommonKLineData(stockCode string, kLineType string, days int64) *[]KLineData {

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", stockCode, kLineType, days)
	//logger.SugaredLogger.Infof("url:%s", url)
	K := &[]KLineData{}
	res := make(map[string]interface{})
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	//logger.SugaredLogger.Infof("resp:%s", resp.Body())
	json.Unmarshal(resp.Body(), &res)
	code, _ := convertor.ToInt(res["code"])
	if code != 0 {
		return K
	}
	if res["data"] != nil && code == 0 {
		data := res["data"].(map[string]interface{})[stockCode].(map[string]interface{})
		if data != nil {
			var day []any
			if data["qfqday"] != nil {
				day = data["qfqday"].([]any)
			}
			if data["day"] != nil {
				day = data["day"].([]any)
			}
			for _, v := range day {
				if v != nil {
					vv := v.([]any)
					KLine := &KLineData{
						Day:    convertor.ToString(vv[0]),
						Open:   convertor.ToString(vv[1]),
						Close:  convertor.ToString(vv[2]),
						High:   convertor.ToString(vv[3]),
						Low:    convertor.ToString(vv[4]),
						Volume: convertor.ToString(vv[5]),
					}
					*K = append(*K, *KLine)
				}
			}
		}
	}
	return K
}

// GetStockHistoryMoneyData 获取股票历史资金流向数据
func (receiver StockDataApi) GetStockHistoryMoneyData(stockCode string) []models.StockMoneyDataHis {

	stockCode = ConvertStockCodeToTushareCode(stockCode)

	var hisData []models.StockMoneyDataHis

	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
			"SH": "1",
			"sh": "1",
			"SZ": "0",
			"sz": "0",
			"BJ": "0",
			"bj": "0",
		})
	} else {
		if strutil.HasPrefixAny(stockCode, []string{"60", "688"}) {
			stockCode = stockCode + ".1"
		} else {
			stockCode = stockCode + ".0"
		}
	}
	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strings.Split(stockCode, ".")[1] + "." + strings.Split(stockCode, ".")[0]
	}

	baseURL := "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"

	params := url2.Values{}
	params.Set("cb", "data")
	params.Set("lmt", "0")
	params.Set("klt", "101")
	params.Set("fields1", "f1,f2,f3,f7")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65")
	params.Set("ut", "b2884a393a59ad64002292a3e90d46a5")
	params.Set("secid", stockCode)
	params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	//
	//// 配置强制 IPv4 优先的 Transport，解决 IPv6 连接问题
	//dialer := &net.Dialer{
	//	Timeout:       10 * time.Second,
	//	KeepAlive:     30 * time.Second,
	//	DualStack:     false, // 禁用双栈
	//	FallbackDelay: -1,    // 禁用 Happy Eyeballs
	//}
	//receiver.client.SetTransport(&http.Transport{
	//	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
	//		// 强制只使用 IPv4
	//		host, port, err := net.SplitHostPort(addr)
	//		if err != nil {
	//			return nil, err
	//		}
	//		// 解析 A 记录（IPv4）
	//		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	//		if err != nil {
	//			return nil, err
	//		}
	//		if len(ips) == 0 {
	//			return nil, fmt.Errorf("no IPv4 address found for %s", host)
	//		}
	//		ipv4 := ips[0].String()
	//		return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ipv4, port))
	//	},
	//	TLSClientConfig: &tls.Config{
	//		MinVersion: tls.VersionTLS12,
	//		ServerName: "push2.eastmoney.com",
	//	},
	//	DisableCompression:  true, // 禁用自动压缩，手动处理 gzip
	//	MaxIdleConns:        100,
	//	MaxIdleConnsPerHost: 10,
	//	IdleConnTimeout:     90 * time.Second,
	//	ForceAttemptHTTP2:   false, // 强制使用 HTTP/1.1
	//})

	//logger.SugaredLogger.Infof("url:%s", reqURL)
	req := receiver.client.SetHeader("User-Agent", getRandomUA()).R()
	setEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
	// 使用缓存的 Cookie，pageURL 参数传空字符串由函数内部使用默认值
	//cookieHeader, err := FetchEastMoneyCookiesViaChromedp("", time.Second*3, reqURL)
	//if err == nil {
	//	//logger.SugaredLogger.Infof("Cookie: %s", cookieHeader)
	//	req.SetHeader("Cookie", cookieHeader)
	//}

	resp, err := req.Get(reqURL)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	value, err := val.Export()
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	marshal, err := json.Marshal(value)
	if err != nil {
		return hisData
	}
	var resData models.StockHistoryMoneyDataResp
	err = json.Unmarshal(marshal, &resData)
	if err != nil {
		return hisData
	}
	if len(resData.Data.Klines) > 0 {
		for _, v := range resData.Data.Klines {
			vals := strings.Split(v, ",")
			//logger.SugaredLogger.Infof("kline:%v", vals)
			hisData = append(hisData, models.StockMoneyDataHis{
				Date: convertor.ToString(vals[0]),
				F62:  convertor.ToString(vals[1]),
				F84:  convertor.ToString(vals[2]),
				F78:  convertor.ToString(vals[3]),
				F72:  convertor.ToString(vals[4]),
				F66:  convertor.ToString(vals[5]),
				F184: convertor.ToString(vals[6]),
				F87:  convertor.ToString(vals[7]),
				F81:  convertor.ToString(vals[8]),
				F75:  convertor.ToString(vals[9]),
				F69:  convertor.ToString(vals[10]),
				F2:   convertor.ToString(vals[11]),
				F3:   convertor.ToString(vals[12]),
			})
		}
	}

	return hisData

}

// GetStockMoneyData 获取个股资金流数据
func (receiver StockDataApi) GetStockMoneyData() models.StockMoneyDataResp {

	var resData models.StockMoneyDataResp
	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=data&fid=f62&po=1&pz=50&pn=1&np=1&fltt=2&invt=2&ut=8dec03ba335b81bf4ebdf7b29ec27d15&fs=m:0+t:6+f:!2,m:0+t:13+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2,m:0+t:7+f:!2,m:1+t:3+f:!2&fields=f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f204,f205,f124,f1,f13,f100,f265"
	req := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut) * time.Second).R()

	setEastMoneyKlineBrowserHeaders(req, "https://quote.eastmoney.com")
	// 使用缓存的 Cookie，pageURL 参数传空字符串由函数内部使用默认值
	//cookieHeader, err := FetchEastMoneyCookiesViaChromedp("", time.Second*3, quoteEastMoneyPage)
	//if err == nil {
	//	//logger.SugaredLogger.Infof("Cookie: %s", cookieHeader)
	//	req.SetHeader("Cookie", cookieHeader)
	//}

	resp, err := req.
		SetHeader("Host", "push2.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	value, err := val.Export()
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	marshal, err := json.Marshal(value)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return models.StockMoneyDataResp{}
	}
	err = json.Unmarshal(marshal, &resData)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return models.StockMoneyDataResp{}
	}
	return resData
}

// GetMutualTop10Deal 获取互联互通（沪股通/深股通/港股通）十大成交股数据
// mutualType: 001=沪股通十大成交股, 002=港股通(沪)十大成交股 , 003=深股通十大成交股, 004=港股通(深)十大成交股
// tradeDate: 交易日期，格式如 2026-03-16
func (receiver StockDataApi) GetMutualTop10Deal(mutualType, tradeDate string, page, pageSize int) *models.MutualTop10DealResp {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	filter := fmt.Sprintf("(MUTUAL_TYPE=\"%s\")(TRADE_DATE='%s')", mutualType, tradeDate)
	encodedFilter := url2.QueryEscape(filter)

	url := fmt.Sprintf("https://datacenter-web.eastmoney.com/web/api/data/v1/get?callback=data&sortColumns=RANK&sortTypes=1&pageSize=%d&pageNumber=%d&reportName=RPT_MUTUAL_TOP10DEAL&columns=ALL&source=WEB&client=WEB&filter=%s&_=%d",
		pageSize, page, encodedFilter, time.Now().UnixMilli())

	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal err:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	body := string(resp.Body())
	//logger.SugaredLogger.Infof("GetMutualTop10Deal resp:%s", body)

	vm := otto.New()
	// 将 JSONP 回调 data(...) 转成普通对象
	_, err = vm.Run("function data(res){return res};")
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal vm func error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal vm run error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	value, err := val.Export()
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal export error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	marshal, err := json.Marshal(value)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal marshal error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}

	var resData models.MutualTop10DealResp
	err = json.Unmarshal(marshal, &resData)
	if err != nil {
		//logger.SugaredLogger.Errorf("GetMutualTop10Deal unmarshal error:%s", err.Error())
		return &models.MutualTop10DealResp{}
	}
	return &resData
}

// 获取股票概念题材信息
func (receiver StockDataApi) GetStockConceptInfo(stockCode string) models.StockConceptInfoResp {
	//601138.SH
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_CORETHEME_BOARDTYPE&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CNEW_BOARD_CODE%2CBOARD_NAME%2CSELECTED_BOARD_REASON%2CIS_PRECISE%2CBOARD_RANK%2CBOARD_YIELD%2CDERIVE_BOARD_CODE&quoteColumns=f3~05~NEW_BOARD_CODE~BOARD_YIELD&filter=(SECUCODE%3D%22" + stockCode + "%22)(IS_PRECISE%3D%221%22)&pageNumber=1&pageSize=&sortTypes=1&sortColumns=BOARD_RANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	//logger.SugaredLogger.Infof("url:%s", url2.QueryEscape(url))
	var data models.StockConceptInfoResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return models.StockConceptInfoResp{}
	}
	return data
}

func (receiver StockDataApi) GetStockFinancialInfo(stockCode string) *models.StockFinancialInfoResp {

	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}

	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_FINANCE_DUPONT&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CORG_CODE%2CORG_TYPE%2CREPORT_DATE%2CREPORT_TYPE%2CREPORT_DATE_NAME%2CSECURITY_TYPE_CODE%2CNOTICE_DATE%2CUPDATE_DATE%2CCURRENCY%2CNETPROFIT%2CTOTAL_OPERATE_INCOME%2CTOTAL_ASSETS%2CTOTAL_LIABILITIES%2CTOTAL_CURRENT_ASSETS%2CTOTAL_NONCURRENT_ASSETS%2CPARENT_NETPROFIT%2CSALE_NPR%2CTOTAL_ASSETS_TR%2CJROA%2CPARENT_NETPROFIT_RATIO%2CEQUITY_MULTIPLIER%2CROE%2CDEBT_ASSET_RATIO%2CTOTAL_INCOME%2CTOTAL_COST%2CTOTAL_EXPENSE%2CMONETARYFUNDS%2CTRADE_FINASSET%2CNOTE_RECE%2CACCOUNTS_RECE%2CFINANCE_RECE%2COTHER_RECE%2CINVENTORY%2CCREDITOR_INVEST%2CLONG_EQUITY_INVEST%2CINVEST_REALESTATE%2CFIXED_ASSET%2CCIP%2CUSERIGHT_ASSET%2CINTANGIBLE_ASSET%2CDEVELOP_EXPENSE%2CGOODWILL%2CLONG_PREPAID_EXPENSE%2CDEFER_TAX_ASSET%2CINVEST_INCOME%2CEXCHANGE_INCOME%2CFAIRVALUE_CHANGE_INCOME%2CASSET_DISPOSAL_INCOME%2COPERATE_COST%2CSURRENDER_VALUE%2CNET_COMPENSATE_EXPENSE%2CNET_CONTRACT_RESERVE%2CPOLICY_BONUS_EXPENSE%2COPERATE_TAX_ADD%2CINCOME_TAX%2CASSET_IMPAIRMENT_INCOME%2CCREDIT_IMPAIRMENT_INCOME%2CNONBUSINESS_EXPENSE%2CFINANCE_EXPENSE%2CSALE_EXPENSE%2CMANAGE_EXPENSE%2CRESEARCH_EXPENSE%2CINTEREST_NI%2CFEE_COMMISSION_NI%2CEARNED_PREMIUM%2CBUSINESS_MANAGE_EXPENSE%2COTHER_CREDITOR_INVEST%2COTHER_EQUITY_INVEST%2CLONG_RECE%2CAVAILABLE_SALE_FINASSET%2CHOLD_MATURITY_INVEST%2CFEE_COMMISSION_EXPENSE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=12&sortTypes=-1&sortColumns=REPORT_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	//logger.SugaredLogger.Infof("url:%s", url)
	var data models.StockFinancialInfoResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		//SetResult(&data).
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	//logger.SugaredLogger.Infof("resp:%s", string(resp.Body()))
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return &models.StockFinancialInfoResp{}
	}
	//logger.SugaredLogger.Infof("data:%v", data)
	return &data
}

func (receiver StockDataApi) GetStockHolderNum(stockCode string) *models.StockHolderNumResp {
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_EH_HOLDERNUM&columns=SECUCODE%2CSECURITY_CODE%2CEND_DATE%2CHOLDER_TOTAL_NUM%2CTOTAL_NUM_RATIO%2CAVG_FREE_SHARES%2CAVG_FREESHARES_RATIO%2CHOLD_FOCUS%2CPRICE%2CAVG_HOLD_AMT%2CHOLD_RATIO_TOTAL%2CFREEHOLD_RATIO_TOTAL&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=12&sortTypes=-1&sortColumns=END_DATE&source=HSF10&client=PC&v=" + strconv.Itoa(time.Now().Nanosecond())
	//logger.SugaredLogger.Infof("url:%s", url)
	var data models.StockHolderNumResp
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		//SetResult(&data).
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return &models.StockHolderNumResp{}
	}
	return &data
}

func (receiver StockDataApi) GetIndustryValuation(bkName string) *models.IndustryValuationResp {
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&reportName=RPT_VALUEINDUSTRY_STA&columns=ALL&quoteColumns=&source=WEB&client=WEB&pageNumber=1&filter=%28BOARD_NAME%3D%22" + url2.QueryEscape(bkName) + "%22%29&_=" + strconv.Itoa(time.Now().Nanosecond())
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	body := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")
	val, err := vm.Run(body)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	value, err := val.Export()
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	marshal, err := json.Marshal(value)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	//logger.SugaredLogger.Infof("data:%s", string(marshal))
	data := models.IndustryValuationResp{}
	err = json.Unmarshal(marshal, &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	return &data
}

func (receiver StockDataApi) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	indicators := ""
	// 将 TechnicalIndicators 转换为 map 并遍历构建查询条件
	indicatorConditions := []string{}

	// 使用反射获取结构体字段值
	v := reflect.ValueOf(technicalIndicators)
	t := reflect.TypeOf(technicalIndicators)

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 只处理布尔类型的字段
		if value.Kind() == reflect.Bool && value.Bool() {
			// 获取 JSON 标签作为字段名
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" {
				// 构建查询条件格式：(FIELD_NAME="1")
				condition := fmt.Sprintf("(%s=\"1\")", jsonTag)
				indicatorConditions = append(indicatorConditions, condition)
			}
		}
		if value.Kind() == reflect.Int && value.Int() != 0 {
			// 获取 JSON 标签作为字段名
			jsonTag := field.Tag.Get("json")
			operator := field.Tag.Get("operator")

			if jsonTag != "" {
				// 构建查询条件格式：如 (UPP_DAYS>=3)
				condition := fmt.Sprintf("(%s%s%d)", jsonTag, operator, value.Int())
				indicatorConditions = append(indicatorConditions, condition)
			}
		}
	}
	// 拼接所有条件
	if len(indicatorConditions) > 0 {
		indicators = strings.Join(indicatorConditions, "")
	}
	//logger.SugaredLogger.Infof("indicators:%s", indicators)

	//logger.SugaredLogger.Infof("GetAllStocks page:%d,pageSize:%d,name:%s", page, pageSize, name)
	search := ""
	if name != "" {
		search = fmt.Sprintf("(SECURITY_NAME_ABBR in (\"%s\"))", name)
	}
	url := "https://data.eastmoney.com/dataapi/xuangu/list?st=CHANGE_RATE&sr=-1&ps=" + convertor.ToString(pageSize) + "&p=" + convertor.ToString(page) + "&sty=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CNEW_PRICE%2CCHANGE_RATE%2CVOLUME_RATIO%2CHIGH_PRICE%2CLOW_PRICE%2CPRE_CLOSE_PRICE%2CVOLUME%2CDEAL_AMOUNT%2CTURNOVERRATE%2CMARKET%2CCONCEPT%2CINDUSTRY&filter=%28MARKET+in+%28%22%E4%B8%8A%E4%BA%A4%E6%89%80%E4%B8%BB%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E4%B8%BB%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E5%88%9B%E4%B8%9A%E6%9D%BF%22%2C%22%E4%B8%8A%E4%BA%A4%E6%89%80%E7%A7%91%E5%88%9B%E6%9D%BF%22%2C%22%E4%B8%8A%E4%BA%A4%E6%89%80%E9%A3%8E%E9%99%A9%E8%AD%A6%E7%A4%BA%E6%9D%BF%22%2C%22%E6%B7%B1%E4%BA%A4%E6%89%80%E9%A3%8E%E9%99%A9%E8%AD%A6%E7%A4%BA%E6%9D%BF%22%2C%22%E5%8C%97%E4%BA%AC%E8%AF%81%E5%88%B8%E4%BA%A4%E6%98%93%E6%89%80%22%29%29" + url2.QueryEscape(search+indicators) + "&source=SELECT_SECURITIES&client=WEB&hyversion=v2"
	//logger.SugaredLogger.Infof("url:%s", url)
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	data := models.AllStocksResp{}
	err = json.Unmarshal(resp.Body(), &data)
	if err != nil {
		//logger.SugaredLogger.Errorf("err:%s", err.Error())
		return &models.AllStocksResp{}
	}
	//for _, info := range data.Result.Data {
	//	toAllStockInfo := info.ToAllStockInfo()
	//	oldInfo := NewStockDataApi().GetStockInfoByCode(info.SECUCODE)
	//	toAllStockInfo.ID = oldInfo.ID
	//	err := NewStockDataApi().AddAllStockInfo(toAllStockInfo)
	//	if err != nil {
	//		logger.SugaredLogger.Errorf("err:%s", err.Error())
	//	}
	//}
	return &data
}

// JSONToMarkdownTable 将JSON数据转换为Markdown表格
func JSONToMarkdownTable(jsonData []byte) (string, error) {
	var data []map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return "", err
	}

	if len(data) == 0 {
		return "", nil
	}

	// 获取表头
	headers := []string{}
	for key := range data[0] {
		headers = append(headers, key)
	}

	// 构建表头行
	headerRow := "|"
	for _, header := range headers {
		headerRow += fmt.Sprintf(" %s |", header)
	}
	headerRow += "\n"

	// 构建分隔行
	separatorRow := "|"
	for range headers {
		separatorRow += " --- |"
	}
	separatorRow += "\n"

	// 构建数据行
	bodyRows := ""
	for _, rowData := range data {
		bodyRow := "|"
		for _, header := range headers {
			value := rowData[header]
			bodyRow += fmt.Sprintf(" %v |", value)
		}
		bodyRows += bodyRow + "\n"
	}

	return headerRow + separatorRow + bodyRows, nil
}

type KLineData struct {
	Day           string            `json:"day" md:"时间/日期"`
	Open          string            `json:"open" md:"开盘价"`
	Close         string            `json:"close" md:"收盘价"`
	High          string            `json:"high" md:"最高价"`
	Low           string            `json:"low" md:"最低价"`
	Volume        string            `json:"volume" md:"成交量"`
	Amount        string            `json:"amount" md:"成交额"`
	ChangePercent string            `json:"changePercent" md:"涨跌幅"`
	ChangeValue   string            `json:"changeValue" md:"涨跌额"`
	Amplitude     string            `json:"amplitude" md:"振幅"`
	TurnoverRate  string            `json:"turnoverRate" md:"换手率"`
	VolumeRatio   string            `json:"volumeRatio" md:"量比"`
	MA            map[string]string `json:"ma,omitempty" md:"均线"` // 周期 -> 均线值，如 "5":"12.34"，由 GetKLineWithMA 填充
}

type MinuteData struct {
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// AllStockInfoQuery 分页查询参数
type AllStockInfoQuery struct {
	Page          int    `form:"page" json:"page"`                 // 页码
	PageSize      int    `form:"pageSize" json:"pageSize"`         // 每页大小
	SecurityCode  string `form:"securityCode" json:"securityCode"` // 股票代码筛选
	SecurityName  string `form:"securityName" json:"securityName"` // 股票名称筛选
	Market        string `form:"market" json:"market"`             // 交易所筛选
	Industry      string `form:"industry" json:"industry"`         // 行业筛选
	Concept       string `form:"concept" json:"concept"`           // 概念筛选
	MinPrice      string `form:"minPrice" json:"minPrice"`         // 最低价筛选
	MaxPrice      string `form:"maxPrice" json:"maxPrice"`         // 最高价筛选
	MinChange     string `form:"minChange" json:"minChange"`       // 最小涨跌幅筛选
	MaxChange     string `form:"maxChange" json:"maxChange"`       // 最大涨跌幅筛选
	SearchKeyWord string `form:"searchKeyWord" json:"searchKeyWord"`
}

// AllStockInfoPageData 分页查询结果
type AllStockInfoPageData struct {
	List       []models.AllStockInfo `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
	TotalPages int                   `json:"totalPages"`
}

// GetAllStockInfoList 分页查询AllStockInfo记录
func (receiver StockDataApi) GetAllStockInfoList(query *AllStockInfoQuery) (*AllStockInfoPageData, error) {
	var list []models.AllStockInfo
	var total int64

	q := db.Dao.Model(&models.AllStockInfo{})

	// 构建查询条件
	if query.SecurityCode != "" {
		q = q.Where("secucode LIKE ?", "%"+query.SecurityCode+"%")
	}
	if query.SecurityName != "" {
		q = q.Where("sec_uri_tynameabbr LIKE ?", "%"+query.SecurityName+"%")
	}
	if query.Market != "" {
		q = q.Where("MARKET = ?", query.Market)
	}
	if query.Industry != "" {
		q = q.Where("INDUSTRY LIKE ?", "%"+query.Industry+"%")
	}
	if query.Concept != "" {
		q = q.Where("CONCEPT LIKE ?", "%"+query.Concept+"%")
	}
	if query.SearchKeyWord != "" {
		q = q.Where("secucode LIKE ? OR sec_uri_tynameabbr LIKE ?", "%"+query.SearchKeyWord+"%", "%"+query.SearchKeyWord+"%")
		q.Or("CONCEPT LIKE ? OR INDUSTRY LIKE ?", "%"+query.SearchKeyWord+"%", "%"+query.SearchKeyWord+"%")
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("maxtradedate DESC, secucode ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &AllStockInfoPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAllStockInfoById 根据ID获取单个AllStockInfo记录
func (receiver StockDataApi) GetAllStockInfoById(id uint) (*models.AllStockInfo, error) {
	var stock models.AllStockInfo
	err := db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", id).First(&stock).Error
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

// AddAllStockInfo 添加或更新AllStockInfo记录
func (receiver StockDataApi) AddAllStockInfo(stock models.AllStockInfo) error {
	if stock.ID > 0 {
		// 更新操作
		return db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", stock.ID).Updates(stock).Error
	} else {
		// 新增操作
		return db.Dao.Model(&models.AllStockInfo{}).Create(&stock).Error
	}
}

// DeleteAllStockInfo 删除AllStockInfo记录
func (receiver StockDataApi) DeleteAllStockInfo(id uint) error {
	return db.Dao.Model(&models.AllStockInfo{}).Where("id = ?", id).Delete(&models.AllStockInfo{}).Error
}

// BatchDeleteAllStockInfo 批量删除AllStockInfo记录
func (receiver StockDataApi) BatchDeleteAllStockInfo(ids []uint) error {
	return db.Dao.Model(&models.AllStockInfo{}).Where("id IN ?", ids).Delete(&models.AllStockInfo{}).Error
}

// GetAllMarkets 获取所有交易所列表
func (receiver StockDataApi) GetAllMarkets() ([]string, error) {
	var markets []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("MARKET").Where("MARKET IS NOT NULL AND MARKET != ''").Order("MARKET").Pluck("MARKET", &markets).Error
	return markets, err
}

// GetAllIndustries 获取所有行业列表
func (receiver StockDataApi) GetAllIndustries() ([]string, error) {
	var industries []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("INDUSTRY").Where("INDUSTRY IS NOT NULL AND INDUSTRY != ''").Order("INDUSTRY").Pluck("INDUSTRY", &industries).Error
	return industries, err
}

// GetAllConcepts 获取所有概念列表
func (receiver StockDataApi) GetAllConcepts() ([]string, error) {
	var concepts []string
	err := db.Dao.Model(&models.AllStockInfo{}).Distinct("CONCEPT").Where("CONCEPT IS NOT NULL AND CONCEPT != ''").Order("CONCEPT").Pluck("CONCEPT", &concepts).Error
	return concepts, err
}

func (receiver StockDataApi) GetStockInfoByCode(secucode string) models.AllStockInfo {
	var stock models.AllStockInfo
	db.Dao.Model(&models.AllStockInfo{}).Where("secucode = ?", secucode).First(&stock)
	return stock
}

// GetStockRZRQInfo 获取融资融券信息
func (receiver StockDataApi) GetStockRZRQInfo(stockCode string) models.StockRZRQInfoResp {
	var StockRZRQInfoResp models.StockRZRQInfoResp
	if !strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = ConvertStockCodeToTushareCode(stockCode)
	}
	filter := url2.QueryEscape(fmt.Sprintf("(SECUCODE=\"%s\")", stockCode))
	url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_RZRQ_STOCKS_DETAIL&columns=MARKET_NAME%2CMARKET_CODE%2CTRADE_DATE%2CSECURITY_CODE%2CSECUCODE%2CSECURITY_NAME_ABBR%2CFIN_BALANCE%2CFIN_BUY_AMT%2CFIN_REPAY_AMT%2CLOAN_BALANCE%2CLOAN_SELL_VOL%2CLOAN_REPAY_VOL%2CMARGIN_BALANCE%2CLOAN_BALANCE_VOL%2CFIN_NETBUY_AMT&quoteColumns=&filter=" + filter + "&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=TRADE_DATE&source=Datacenter&client=PC&v=" + convertor.ToString(time.Now().Unix())
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return StockRZRQInfoResp
	}
	json.Unmarshal(resp.Body(), &StockRZRQInfoResp)
	return StockRZRQInfoResp
}

// AddTradingRecord 添加交易日志
func (receiver StockDataApi) AddTradingRecord(record TradingRecord) (uint, error) {
	// 必填字段校验
	if strings.TrimSpace(record.StockCode) == "" {
		return 0, fmt.Errorf("股票代码不能为空")
	}
	if strings.TrimSpace(record.StockName) == "" {
		return 0, fmt.Errorf("股票名称不能为空")
	}
	if record.Direction != "买入" && record.Direction != "卖出" {
		return 0, fmt.Errorf("交易方向只能为买入或卖出")
	}
	if record.Price <= 0 {
		return 0, fmt.Errorf("价格必须大于0")
	}
	if record.Volume <= 0 {
		return 0, fmt.Errorf("成交数量必须大于0")
	}
	if record.Fee < 0 {
		return 0, fmt.Errorf("手续费不能为负数")
	}
	if record.StopLossPrice < 0 || record.TakeProfitPrice < 0 {
		return 0, fmt.Errorf("止损价/止盈价不能为负数")
	}

	// 设置交易时间为当前时间（如果未提供）
	if record.TradingTime.IsZero() {
		record.TradingTime = time.Now()
	}
	record.TradingTime = record.TradingTime.In(time.Local)

	// 频繁交易检查仅在前端提示用户，后端不再阻止添加（CheckFrequentTrading 接口保留供前端调用）

	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)

	receiver.fillTradingRecordCloseSnapshot(&record)

	// 保存到数据库
	err := db.Dao.Model(&TradingRecord{}).Create(&record).Error
	if err != nil {
		logger.SugaredLogger.Errorf("添加交易日志失败: %s", err.Error())
		return 0, err
	}

	return record.ID, nil
}

// tradingRecordFIFOLot 交易日志先入先出批次
type tradingRecordFIFOLot struct {
	Volume int64
	Price  float64
}

// fifoAvgUnitCost 按先入先出计算卖出数量对应的单位持仓成本（不修改批次）
func fifoAvgUnitCost(lots []tradingRecordFIFOLot, sellVol int64) (avg float64, ok bool) {
	if sellVol <= 0 {
		return 0, false
	}
	var cost float64
	var got int64
	for i := range lots {
		if got >= sellVol {
			break
		}
		if lots[i].Volume <= 0 {
			continue
		}
		need := sellVol - got
		take := need
		if take > lots[i].Volume {
			take = lots[i].Volume
		}
		cost += float64(take) * lots[i].Price
		got += take
	}
	if got < sellVol {
		return 0, false
	}
	return cost / float64(got), true
}

// normalizeTradingRecordAPI 将交易日志中的代码转为实时/K 线接口使用的代码
func normalizeTradingRecordAPI(stockCode string) string {
	apiCode := stockCode
	if strings.Contains(apiCode, " - ") {
		apiCode = strings.Split(apiCode, " - ")[0]
	}
	apiCode = strings.ToLower(apiCode)
	if strings.HasSuffix(apiCode, ".sh") {
		apiCode = "sh" + strings.TrimSuffix(apiCode, ".sh")
	} else if strings.HasSuffix(apiCode, ".sz") {
		apiCode = "sz" + strings.TrimSuffix(apiCode, ".sz")
	} else if strings.HasSuffix(apiCode, ".bj") {
		apiCode = "bj" + strings.TrimSuffix(apiCode, ".bj")
	} else if strings.HasPrefix(apiCode, "6") || len(apiCode) == 6 {
		apiCode = "sh" + apiCode
	} else if strings.HasPrefix(apiCode, "0") || strings.HasPrefix(apiCode, "3") {
		apiCode = "sz" + apiCode
	} else if strings.HasPrefix(apiCode, "4") || strings.HasPrefix(apiCode, "8") {
		apiCode = "bj" + apiCode
	}
	return apiCode
}

// resolveTradingRecordClosePrice 按交易日期解析收盘价或现价（无缓存，供写入快照与列表补拉共用）
func (receiver StockDataApi) resolveTradingRecordClosePrice(apiCode string, tradingTime time.Time, fallback float64) float64 {
	if strings.TrimSpace(apiCode) == "" {
		return fallback
	}
	tradingTime = tradingTime.In(time.Local)
	now := time.Now()
	tradingDateStr := tradingTime.Format("2006-01-02")
	todayStr := now.Format("2006-01-02")
	closePrice := fallback
	isToday := tradingDateStr == todayStr
	isFuture := tradingTime.After(now)
	if isToday || isFuture {
		stockDatas, err := receiver.GetStockCodeRealTimeData(apiCode)
		if err == nil && stockDatas != nil && len(*stockDatas) > 0 {
			price, _ := convertor.ToFloat((*stockDatas)[0].Price)
			if price > 0 {
				closePrice = price
			}
		}
	} else {
		// 按交易日期距今天数动态计算 K 线查询数量，避免 30 根不够导致查不到早期记录
		daysSince := int(now.Sub(tradingTime).Hours()/24) + 10
		if daysSince < 30 {
			daysSince = 30
		}
		if daysSince > 1000 {
			daysSince = 1000
		}
		klines := receiver.GetCommonKLineData(apiCode, "day", int64(daysSince))
		if klines != nil {
			for _, k := range *klines {
				if k.Day == tradingDateStr {
					cp, _ := convertor.ToFloat(k.Close)
					if cp > 0 {
						closePrice = cp
					}
					break
				}
			}
		}
	}
	return closePrice
}

// fillTradingRecordCloseSnapshot 写入/刷新记录的收盘价快照（添加、修改时调用）
func (receiver StockDataApi) fillTradingRecordCloseSnapshot(record *TradingRecord) {
	if record == nil {
		return
	}
	apiCode := normalizeTradingRecordAPI(record.StockCode)
	record.RecordedClosePrice = receiver.resolveTradingRecordClosePrice(apiCode, record.TradingTime, record.Price)
}

// GetTradingRecordList 获取交易日志列表（分页、关键词、方向、交易日期范围）
func (receiver StockDataApi) GetTradingRecordList(query TradingRecordListQuery) (*TradingRecordPageData, error) {
	var records []TradingRecord
	q := db.Dao.Model(&TradingRecord{})

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("stock_code LIKE ? OR stock_name LIKE ?", like, like)
	}
	if dir := strings.TrimSpace(query.Direction); dir != "" {
		q = q.Where("direction = ?", dir)
	}
	if sd := strings.TrimSpace(query.StartDate); sd != "" {
		if start, err := time.ParseInLocation("2006-01-02", sd, time.Local); err == nil {
			q = q.Where("trading_time >= ?", start)
		}
	}
	if ed := strings.TrimSpace(query.EndDate); ed != "" {
		if end, err := time.ParseInLocation("2006-01-02", ed, time.Local); err == nil {
			q = q.Where("trading_time < ?", end.Add(24*time.Hour))
		}
	}

	var total int64
	err := q.Count(&total).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志总数失败: %s", err.Error())
		return nil, err
	}

	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("trading_time DESC, id DESC").Find(&records).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志列表失败: %s", err.Error())
		return nil, err
	}

	needProfitByID := make(map[uint]struct{}, len(records))
	for _, r := range records {
		needProfitByID[r.ID] = struct{}{}
	}

	var allGlobal []TradingRecord
	if err := db.Dao.Model(&TradingRecord{}).Order("trading_time ASC, id ASC").Find(&allGlobal).Error; err != nil {
		logger.SugaredLogger.Errorf("获取交易日志全局序失败: %s", err.Error())
		return nil, err
	}

	type rowProfit struct {
		closePrice    float64
		profitAmount  float64
		profitPercent float64
	}
	profitByID := make(map[uint]rowProfit, len(records))

	closeCache := make(map[string]float64)

	resolveClose := func(apiCode string, tradingTime time.Time, fallback float64, recorded float64) float64 {
		// 当天或未来日期的记录始终获取实时行情，不使用缓存快照
		tradingDateStr := tradingTime.Format("2006-01-02")
		key := apiCode + "|" + tradingDateStr
		if tradingDateStr == time.Now().Format("2006-01-02") || tradingTime.After(time.Now()) {
			closePrice := receiver.resolveTradingRecordClosePrice(apiCode, tradingTime, fallback)
			closeCache[key] = closePrice
			return closePrice
		}
		// 历史记录优先使用已保存的快照
		if recorded > 0 {
			return recorded
		}
		if v, ok := closeCache[key]; ok {
			return v
		}
		closePrice := receiver.resolveTradingRecordClosePrice(apiCode, tradingTime, fallback)
		closeCache[key] = closePrice
		return closePrice
	}

	// ===== FIFO 持仓引擎 =====
	// buyLot 记录每次买入的剩余数量（被后续卖出按 FIFO 扣减），recordID 用于回溯关联
	type buyLot struct {
		recordID uint
		volume   int64 // 剩余数量
		price    float64
	}
	holdings := make(map[string][]buyLot)

	// 需要回写收盘价快照的历史记录：RecordedClosePrice == 0 且成功获取到 closePrice
	type closeBackfill struct {
		id         uint
		closePrice float64
	}
	var backfills []closeBackfill
	todayStr := time.Now().Format("2006-01-02")
	now := time.Now()

	// Phase 1: 按时间顺序遍历全部记录，构建 FIFO 持仓、计算卖出已实现盈亏
	for _, r := range allGlobal {
		_, need := needProfitByID[r.ID]
		apiCode := normalizeTradingRecordAPI(r.StockCode)
		tradingDateStr := r.TradingTime.In(time.Local).Format("2006-01-02")

		if r.Direction == "买入" {
			// 加入 FIFO 持仓
			holdings[r.StockCode] = append(holdings[r.StockCode], buyLot{
				recordID: r.ID,
				volume:   r.Volume,
				price:    r.Price,
			})
			// 预解析收盘价并回写快照（仅当前页记录，填充缓存供 Phase 2 使用）
			if need {
				closePrice := resolveClose(apiCode, r.TradingTime, r.Price, r.RecordedClosePrice)
				if r.RecordedClosePrice == 0 && closePrice > 0 && tradingDateStr != todayStr && !r.TradingTime.After(now) {
					backfills = append(backfills, closeBackfill{id: r.ID, closePrice: closePrice})
				}
			}
		} else if r.Direction == "卖出" {
			// FIFO 扣减持仓并计算卖出对应的成本（合并计算与扣减，避免状态不一致）
			remaining := r.Volume
			var fifoCost float64
			var effectiveVol int64
			for i := range holdings[r.StockCode] {
				if remaining <= 0 {
					break
				}
				lot := &holdings[r.StockCode][i]
				if lot.volume <= 0 {
					continue
				}
				take := remaining
				if take > lot.volume {
					take = lot.volume
				}
				fifoCost += float64(take) * lot.price
				lot.volume -= take
				remaining -= take
				effectiveVol += take
			}
			if effectiveVol < r.Volume && effectiveVol >= 0 {
				logger.SugaredLogger.Warnf("交易日志: 股票 %s 卖出 %d 股超出持仓 %d 股，仅按 %d 股计算",
					r.StockCode, r.Volume, effectiveVol, effectiveVol)
			}

			if need {
				closePrice := resolveClose(apiCode, r.TradingTime, r.Price, r.RecordedClosePrice)
				if r.RecordedClosePrice == 0 && closePrice > 0 && tradingDateStr != todayStr && !r.TradingTime.After(now) {
					backfills = append(backfills, closeBackfill{id: r.ID, closePrice: closePrice})
				}
				if effectiveVol > 0 {
					// 已实现盈亏 = 卖出收入 - FIFO成本 - 手续费
					profitAmount := r.Price*float64(effectiveVol) - fifoCost - r.Fee
					profitPercent := 0.0
					if fifoCost > 0 {
						profitPercent = profitAmount / fifoCost * 100
					}
					profitByID[r.ID] = rowProfit{
						closePrice:    closePrice,
						profitAmount:  profitAmount,
						profitPercent: profitPercent,
					}
				} else {
					// 无持仓却卖出，数据异常，仅显示收盘价不计算盈亏
					profitByID[r.ID] = rowProfit{closePrice: closePrice}
				}
			}
		}
	}

	// Phase 2: 计算买入记录的浮动盈亏（仅对剩余未卖出部分）
	for _, r := range records {
		if r.Direction != "买入" {
			continue
		}
		apiCode := normalizeTradingRecordAPI(r.StockCode)
		closePrice := resolveClose(apiCode, r.TradingTime, r.Price, r.RecordedClosePrice)

		// 查找该买入记录的剩余数量（FIFO 扣减后的余额）
		remainingVol := int64(0)
		for _, lot := range holdings[r.StockCode] {
			if lot.recordID == r.ID {
				remainingVol += lot.volume
			}
		}

		if remainingVol > 0 {
			// 浮动盈亏 = (现价 - 买入价) * 剩余数量 - 按比例分摊的手续费
			profitAmount := (closePrice - r.Price) * float64(remainingVol)
			if r.Volume > 0 && r.Fee > 0 {
				profitAmount -= r.Fee * float64(remainingVol) / float64(r.Volume)
			}
			profitPercent := 0.0
			costBase := r.Price * float64(remainingVol)
			if costBase > 0 {
				profitPercent = profitAmount / costBase * 100
			}
			profitByID[r.ID] = rowProfit{
				closePrice:    closePrice,
				profitAmount:  profitAmount,
				profitPercent: profitPercent,
			}
		} else {
			// 已全部卖出，盈亏已体现在卖出记录中，买入行显示 0
			profitByID[r.ID] = rowProfit{closePrice: closePrice, profitAmount: 0, profitPercent: 0}
		}
	}

	seenBackfill := make(map[uint]struct{}, len(backfills))
	for _, bf := range backfills {
		if _, dup := seenBackfill[bf.id]; dup {
			continue
		}
		seenBackfill[bf.id] = struct{}{}
		res := db.Dao.Model(&TradingRecord{}).Where("id = ? AND (recorded_close_price IS NULL OR recorded_close_price = 0)", bf.id).
			Update("recorded_close_price", bf.closePrice)
		if res.Error != nil {
			logger.SugaredLogger.Warnf("回写交易记录收盘价快照失败 id=%d: %s", bf.id, res.Error.Error())
		}
	}

	items := make([]TradingRecordItem, 0, len(records))
	for _, r := range records {
		r.Amount = r.Price * float64(r.Volume)
		item := TradingRecordItem{TradingRecord: r}
		if p, ok := profitByID[r.ID]; ok {
			item.ClosePrice = p.closePrice
			item.ProfitAmount = p.profitAmount
			item.ProfitPercent = p.profitPercent
		}
		items = append(items, item)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	return &TradingRecordPageData{
		List:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTradingRecordStatistics 获取交易日志统计数据
// 统计始终基于全部历史记录构建FIFO，确保总盈亏与当日盈亏真实准确，不受列表筛选条件影响
func (receiver StockDataApi) GetTradingRecordStatistics() (*TradingRecordStatistics, error) {
	// FIFO 持仓批次，使用指针便于卖出扣减时直接修改剩余数量
	type BuyRecord struct {
		Volume  int64
		Price   float64
		IsToday bool // 标记是否今日买入，用于遍历结束后计算今日浮动盈亏
	}

	// 统计基于全部记录，FIFO需要完整历史才能正确计算持仓成本与已实现盈亏
	var records []TradingRecord
	err := db.Dao.Model(&TradingRecord{}).Order("trading_time ASC, id ASC").Find(&records).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志统计失败: %s", err.Error())
		return nil, err
	}

	stockMap := make(map[string][]*BuyRecord)
	totalBuyAmount := 0.0
	totalSellAmount := 0.0
	holdingsCost := 0.0
	holdingsValue := 0.0
	costOfSoldShares := 0.0
	totalFee := 0.0 // 累计买入+卖出手续费（仅有效记录），与列表行 profitAmount 口径一致

	// 当日盈亏统计
	todayStr := time.Now().Format("2006-01-02")
	todayBuyAmount := 0.0
	todaySellAmount := 0.0
	todayRealizedProfit := 0.0
	todayFloatingProfit := 0.0
	todayCostOfSold := 0.0 // 今日卖出对应的FIFO成本，用于收益率分母
	todayBuyFee := 0.0     // 今日买入手续费（在 todayProfit 中统一扣除，避免按比例分配到批次）

	for _, r := range records {
		isToday := r.TradingTime.In(time.Local).Format("2006-01-02") == todayStr
		if r.Direction == "买入" {
			amount := r.Price * float64(r.Volume)
			totalFee += r.Fee
			totalBuyAmount += amount
			stockMap[r.StockCode] = append(stockMap[r.StockCode], &BuyRecord{Volume: r.Volume, Price: r.Price, IsToday: isToday})
			if isToday {
				todayBuyAmount += amount
				todayBuyFee += r.Fee
			}
		} else if r.Direction == "卖出" {
			// 检查卖出数量是否超出当前持仓，超出部分不计入卖出收入与成本（避免利润虚高）
			availableVolume := int64(0)
			for _, br := range stockMap[r.StockCode] {
				if br.Volume > 0 {
					availableVolume += br.Volume
				}
			}
			effectiveSellVolume := r.Volume
			if r.Volume > availableVolume {
				if availableVolume > 0 {
					logger.SugaredLogger.Warnf("交易日志统计: 股票 %s 卖出量 %d 超出持仓 %d，仅按 %d 股计算",
						r.StockCode, r.Volume, availableVolume, availableVolume)
					effectiveSellVolume = availableVolume
				} else {
					logger.SugaredLogger.Warnf("交易日志统计: 股票 %s 无持仓却记录卖出 %d 股，跳过该记录",
						r.StockCode, r.Volume)
					effectiveSellVolume = 0
				}
			}
			if effectiveSellVolume <= 0 {
				// 无有效卖出，不计入手续费（数据异常由用户修正）
				continue
			}
			// 有效卖出才累加手续费（含截断情况，实际已支付）
			totalFee += r.Fee
			// 卖出收入按有效卖出数量折算
			sellRevenue := r.Price * float64(effectiveSellVolume)
			totalSellAmount += sellRevenue
			// FIFO 扣减并累加成本，记录扣减前后的差值用于当日已实现盈亏计算
			costBefore := costOfSoldShares
			remainingVolume := effectiveSellVolume
			for i := range stockMap[r.StockCode] {
				if remainingVolume == 0 {
					break
				}
				lot := stockMap[r.StockCode][i]
				if lot.Volume <= remainingVolume {
					costOfSoldShares += float64(lot.Volume) * lot.Price
					remainingVolume -= lot.Volume
					lot.Volume = 0
				} else {
					costOfSoldShares += float64(remainingVolume) * lot.Price
					lot.Volume -= remainingVolume
					remainingVolume = 0
				}
			}
			if isToday {
				deltaCost := costOfSoldShares - costBefore
				todaySellAmount += sellRevenue
				todayCostOfSold += deltaCost
				// 卖出已实现盈亏（含亏损），扣除卖出手续费
				todayRealizedProfit += sellRevenue - deltaCost - r.Fee
			}
		}
	}

	var stockCount int64
	for code, buyRecords := range stockMap {
		currentVolume := int64(0)
		currentCost := 0.0
		// 今日买入未卖出的剩余数量与成本
		todayBuyRemainingVolume := int64(0)
		todayBuyRemainingCost := 0.0
		for _, br := range buyRecords {
			if br.Volume > 0 {
				currentVolume += br.Volume
				currentCost += float64(br.Volume) * br.Price
				if br.IsToday {
					todayBuyRemainingVolume += br.Volume
					todayBuyRemainingCost += float64(br.Volume) * br.Price
				}
			}
		}
		if currentVolume > 0 {
			stockCount++
			holdingsCost += currentCost

			apiCode := normalizeTradingRecordAPI(code)
			stockDatas, err := receiver.GetStockCodeRealTimeData(apiCode)
			if err == nil && stockDatas != nil && len(*stockDatas) > 0 {
				stock := (*stockDatas)[0]
				price, _ := convertor.ToFloat(stock.Price)
				if price == 0 {
					price, _ = convertor.ToFloat(stock.A1P)
				}
				if price > 0 {
					holdingsValue += price * float64(currentVolume)

					// 当日浮动盈亏 = 历史持仓今日浮盈变化 + 今日买入未卖出浮盈
					historicalVolume := currentVolume - todayBuyRemainingVolume
					// 历史持仓（昨日及之前买入今日仍持有）按 (现价 - 昨收) 计算今日涨跌
					if historicalVolume > 0 {
						zrsp, _ := convertor.ToFloat(stock.PreClose)
						if zrsp > 0 {
							todayFloatingProfit += (price - zrsp) * float64(historicalVolume)
						}
					}
					// 今日买入未卖出部分按 (现价 - 买入价) 计算浮盈
					if todayBuyRemainingVolume > 0 {
						todayFloatingProfit += price*float64(todayBuyRemainingVolume) - todayBuyRemainingCost
					}
				}
			}
		}
	}

	// 总盈亏 = 已实现盈亏(卖出收入-卖出成本) + 未实现浮盈(持仓市值-持仓成本) - 累计手续费
	totalProfit := totalSellAmount - costOfSoldShares + (holdingsValue - holdingsCost) - totalFee
	// 收益率分母 = 总投入成本（剩余持仓成本 + 已卖出部分成本），避免部分清仓后收益率被放大
	denom := holdingsCost + costOfSoldShares
	if denom <= 0 && totalBuyAmount > 0 {
		// 完全清仓且无持仓时，回退到总买入额
		denom = totalBuyAmount
	}
	profitRate := 0.0
	if denom > 0 {
		profitRate = (totalProfit / denom) * 100
	}

	// 当日总盈亏 = 已实现盈亏(扣卖出手续费) + 浮动盈亏 - 今日买入手续费
	// 浮动盈亏包含：历史持仓今日涨跌(现价-昨收) + 今日买入未卖出浮盈(现价-买入价)
	todayProfit := todayRealizedProfit + todayFloatingProfit - todayBuyFee
	// 收益率分母 = 今日买入金额 + 今日卖出对应的FIFO成本
	todayDenom := todayBuyAmount + todayCostOfSold
	todayProfitRate := 0.0
	if todayDenom > 0 {
		todayProfitRate = (todayProfit / todayDenom) * 100
	}

	return &TradingRecordStatistics{
		TotalBuyAmount:      totalBuyAmount,
		TotalSellAmount:     totalSellAmount,
		TotalProfit:         totalProfit,
		ProfitRate:          profitRate,
		HoldingsAmount:      holdingsCost,
		CurrentValue:        holdingsValue,
		StockCount:          stockCount,
		TodayBuyAmount:      todayBuyAmount,
		TodaySellAmount:     todaySellAmount,
		TodayRealizedProfit: todayRealizedProfit,
		TodayFloatingProfit: todayFloatingProfit,
		TodayProfit:         todayProfit,
		TodayProfitRate:     todayProfitRate,
	}, nil
}

// GetTradingRecordById 根据ID获取单个交易日志
func (receiver StockDataApi) GetTradingRecordById(id uint) (*TradingRecord, error) {
	var record TradingRecord
	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.SugaredLogger.Errorf("获取交易日志失败: %s", err.Error())
		return nil, err
	}
	return &record, nil
}

// UpdateTradingRecord 更新交易日志
func (receiver StockDataApi) UpdateTradingRecord(record TradingRecord) error {
	if record.ID == 0 {
		return fmt.Errorf("记录ID不能为空")
	}
	if strings.TrimSpace(record.StockCode) == "" {
		return fmt.Errorf("股票代码不能为空")
	}
	if record.Direction != "买入" && record.Direction != "卖出" {
		return fmt.Errorf("交易方向只能为买入或卖出")
	}
	if record.Price <= 0 {
		return fmt.Errorf("价格必须大于0")
	}
	if record.Volume <= 0 {
		return fmt.Errorf("成交数量必须大于0")
	}
	if record.Fee < 0 {
		return fmt.Errorf("手续费不能为负数")
	}
	if record.StopLossPrice < 0 || record.TakeProfitPrice < 0 {
		return fmt.Errorf("止损价/止盈价不能为负数")
	}

	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)

	if record.TradingTime.IsZero() {
		record.TradingTime = time.Now()
	}
	record.TradingTime = record.TradingTime.In(time.Local)

	// 查询原记录：仅当交易时间变化或快照为0时才重新拉取收盘价
	var old TradingRecord
	if err := db.Dao.Model(&TradingRecord{}).Where("id = ?", record.ID).First(&old).Error; err != nil {
		logger.SugaredLogger.Errorf("查询原交易日志失败: %s", err.Error())
		return err
	}
	oldTradingDate := old.TradingTime.In(time.Local).Format("2006-01-02")
	newTradingDate := record.TradingTime.Format("2006-01-02")
	needRefreshSnapshot := old.RecordedClosePrice == 0 || oldTradingDate != newTradingDate
	if needRefreshSnapshot {
		receiver.fillTradingRecordCloseSnapshot(&record)
	} else {
		// 保留原快照，避免编辑历史记录时被错误覆盖
		record.RecordedClosePrice = old.RecordedClosePrice
	}

	// 使用 map 更新避免 Gorm struct 模式忽略零值字段（止损价/止盈价/手续费/Reason/Mindset 等）
	// 注意：Amount 标记为 gorm:"-"（计算字段，非数据库列），不应出现在 updates 中
	updates := map[string]any{
		"stock_code":           record.StockCode,
		"stock_name":           record.StockName,
		"direction":            record.Direction,
		"price":                record.Price,
		"volume":               record.Volume,
		"trading_time":         record.TradingTime,
		"reason":               record.Reason,
		"stop_loss_price":      record.StopLossPrice,
		"take_profit_price":    record.TakeProfitPrice,
		"fee":                  record.Fee,
		"market_value":         record.MarketValue,
		"mindset":              record.Mindset,
		"recorded_close_price": record.RecordedClosePrice,
	}
	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", record.ID).Updates(updates).Error
	if err != nil {
		logger.SugaredLogger.Errorf("更新交易日志失败: %s", err.Error())
		return err
	}
	return nil
}

// DeleteTradingRecord 删除交易日志
func (receiver StockDataApi) DeleteTradingRecord(id uint) error {
	err := db.Dao.Model(&TradingRecord{}).Where("id = ?", id).Delete(&TradingRecord{}).Error
	if err != nil {
		logger.SugaredLogger.Errorf("删除交易日志失败: %s", err.Error())
		return err
	}
	return nil
}

// TradingRecordImportResult 交易日志批量导入结果
type TradingRecordImportResult struct {
	Total    int    `json:"total"`    // 文件总记录数
	Imported int    `json:"imported"` // 成功导入条数
	Skipped  int    `json:"skipped"`  // 跳过（已存在/重复）条数
	Failed   int    `json:"failed"`   // 解析失败条数
	Message  string `json:"message"`  // 汇总提示
}

// parseTradingImportFile 解析券商导出的成交记录文件。
// 支持 UTF-8 与 GBK 编码的 Tab 分隔文本（即使扩展名为 .xls/.csv，内容仍为表格文本）。
// 返回以表头名为 key 的原始数据行数组。
func parseTradingImportFile(filePath string) ([]map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	// GBK → UTF-8（仅当内容不是合法 UTF-8 时转码）
	if !utf8.Valid(data) {
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, reader); err == nil {
			data = buf.Bytes()
		}
	}

	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("文件内容为空")
	}

	header := strings.Split(strings.TrimSpace(lines[0]), "\t")
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		colIdx[strings.TrimSpace(name)] = i
	}
	if _, ok := colIdx["成交日期"]; !ok {
		return nil, fmt.Errorf("无法识别的成交记录文件格式：缺少表头列「成交日期」")
	}

	rows := make([]map[string]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		row := make(map[string]string, len(colIdx))
		for name, i := range colIdx {
			if i < len(fields) {
				row[name] = strings.TrimSpace(fields[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// normalizeImportedStockCode 将券商导出的证券代码归一化为前缀格式。
// 结合「市场名称」（上海Ａ股/深圳Ａ股/北京…）确定交易所前缀，并将纯数字代码补齐为 6 位。
func normalizeImportedStockCode(code, market string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	lower := strings.ToLower(code)
	// 已带前缀（sh/sz/bj/hk/us）直接走统一归一化
	if strings.HasPrefix(lower, "sh") || strings.HasPrefix(lower, "sz") ||
		strings.HasPrefix(lower, "bj") || strings.HasPrefix(lower, "hk") ||
		strings.HasPrefix(lower, "us") {
		return normalizeStockCode(code)
	}
	if strings.Contains(code, ".") {
		return normalizeStockCode(code)
	}

	// 提取纯数字部分
	var digits strings.Builder
	for _, c := range code {
		if c >= '0' && c <= '9' {
			digits.WriteRune(c)
		}
	}
	numStr := digits.String()
	if numStr == "" {
		return ""
	}

	// 按市场名称确定前缀
	var prefix string
	switch {
	case strings.Contains(market, "上海"):
		prefix = "sh"
	case strings.Contains(market, "深圳"):
		prefix = "sz"
	case strings.Contains(market, "北京"):
		prefix = "bj"
	case strings.Contains(market, "港"):
		prefix = "hk"
	}
	if prefix == "" {
		// 无市场信息时按首位数字兜底
		return normalizeStockCode(code)
	}
	if prefix == "hk" {
		// 港股代码 5 位，不足补前导 0
		for len(numStr) < 5 {
			numStr = "0" + numStr
		}
		return prefix + numStr
	}
	// A 股/北交所 6 位，不足补前导 0
	for len(numStr) < 6 {
		numStr = "0" + numStr
	}
	if len(numStr) > 6 {
		numStr = numStr[:6]
	}
	return prefix + numStr
}

// parseTradingImportTime 解析成交日期与成交时间（如 20260812 + 14:15:41）。
func parseTradingImportTime(dateStr, timeStr string) (time.Time, error) {
	ds := strings.TrimSpace(dateStr)
	ts := strings.TrimSpace(timeStr)
	if ds == "" {
		return time.Time{}, fmt.Errorf("成交日期为空")
	}
	datePart := strings.ReplaceAll(ds, "-", "")
	if len(datePart) != 8 {
		return time.Time{}, fmt.Errorf("成交日期格式错误: %s", ds)
	}
	layout := "20060102"
	s := datePart
	if ts != "" {
		s += " " + ts
		layout += " 15:04:05"
	}
	t, err := time.ParseInLocation(layout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("成交时间解析失败: %s %s", ds, ts)
	}
	return t, nil
}

// parseFloatSafe 安全解析浮点字符串，失败返回 0。
func parseFloatSafe(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// ImportTradingRecords 批量导入券商导出的成交记录。
// 同一文件中重复或与数据库已存在（股票代码+方向+交易时间+价格+数量完全一致）的记录会跳过。
func (receiver StockDataApi) ImportTradingRecords(filePath string) (*TradingRecordImportResult, error) {
	rows, err := parseTradingImportFile(filePath)
	if err != nil {
		return nil, err
	}
	result := &TradingRecordImportResult{Total: len(rows)}

	// 加载已有记录作为去重键（个人交易日志数据量有限，全量加载即可）
	existingKeys := make(map[string]struct{})
	var existing []TradingRecord
	if err := db.Dao.Model(&TradingRecord{}).Find(&existing).Error; err == nil {
		for _, r := range existing {
			existingKeys[tradingRecordDedupKey(r.StockCode, r.Direction, r.TradingTime, r.Price, r.Volume)] = struct{}{}
		}
	}
	seenInFile := make(map[string]struct{})

	var toCreate []TradingRecord
	for _, row := range rows {
		direction := row["操作"]
		if direction != "买入" && direction != "卖出" {
			result.Failed++
			continue
		}
		stockName := row["证券名称"]
		stockCode := normalizeImportedStockCode(row["证券代码"], row["市场名称"])
		if stockCode == "" || strings.TrimSpace(stockName) == "" {
			result.Failed++
			continue
		}
		price := parseFloatSafe(row["成交均价"])
		volume := int64(parseFloatSafe(row["成交数量"]))
		if price <= 0 || volume <= 0 {
			result.Failed++
			continue
		}
		t, err := parseTradingImportTime(row["成交日期"], row["成交时间"])
		if err != nil {
			result.Failed++
			continue
		}
		// 手续费 = 手续费 + 印花税 + 其他杂费，使盈亏计算更准确
		fee := parseFloatSafe(row["手续费"]) + parseFloatSafe(row["印花税"]) + parseFloatSafe(row["其他杂费"])

		rec := TradingRecord{
			StockCode:   stockCode,
			StockName:   stockName,
			Direction:   direction,
			Price:       price,
			Volume:      volume,
			Fee:         fee,
			TradingTime: t,
			Amount:      price * float64(volume),
		}
		key := tradingRecordDedupKey(rec.StockCode, rec.Direction, rec.TradingTime, rec.Price, rec.Volume)
		if _, ok := existingKeys[key]; ok {
			result.Skipped++
			continue
		}
		if _, ok := seenInFile[key]; ok {
			result.Skipped++
			continue
		}
		seenInFile[key] = struct{}{}
		toCreate = append(toCreate, rec)
	}

	// 批量写入（事务，分批插入），不逐条拉取收盘价快照（由列表/统计按需回填，避免大量网络请求）
	if len(toCreate) > 0 {
		err := db.Dao.Transaction(func(tx *gorm.DB) error {
			const batchSize = 200
			for i := 0; i < len(toCreate); i += batchSize {
				end := i + batchSize
				if end > len(toCreate) {
					end = len(toCreate)
				}
				if err := tx.Create(toCreate[i:end]).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			logger.SugaredLogger.Errorf("批量导入交易日志失败: %s", err.Error())
			return nil, err
		}
	}
	result.Imported = len(toCreate)
	result.Message = fmt.Sprintf("共 %d 条：成功导入 %d 条，跳过 %d 条，失败 %d 条",
		result.Total, result.Imported, result.Skipped, result.Failed)
	return result, nil
}

// tradingRecordDedupKey 构造交易记录去重键（股票代码|方向|交易时间|价格|数量）
func tradingRecordDedupKey(stockCode, direction string, t time.Time, price float64, volume int64) string {
	return fmt.Sprintf("%s|%s|%s|%.4f|%d", stockCode, direction, t.In(time.Local).Format("2006-01-02 15:04:05"), price, volume)
}

// CheckFrequentTrading 检查是否频繁交易
// 返回值：(是否可以交易, 提示消息)
func (receiver StockDataApi) CheckFrequentTrading(stockCode string) (bool, string) {
	// 检查最近24小时内是否有同一只股票的交易日志
	var count int64
	cutoffTime := time.Now().Add(-24 * time.Hour)

	err := db.Dao.Model(&TradingRecord{}).Where("stock_code = ? AND direction = ? AND trading_time > ?", stockCode, "买入", cutoffTime).Count(&count).Error
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count > 0 {
		return false, "最近24小时内已对该股票进行过买入操作，为避免频繁交易，建议稍后再操作"
	}

	// 检查最近7天内的交易次数是否超过限制（例如：5次）
	cutoffTime7Days := time.Now().Add(-7 * 24 * time.Hour)
	err = db.Dao.Model(&TradingRecord{}).Where("direction = ? AND trading_time > ?", "买入", cutoffTime7Days).Count(&count).Error
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count >= 5 {
		return false, "最近7天内交易次数已达上限（5次），为避免频繁交易，建议稍后再操作"
	}

	return true, "可以交易"
}
