package data

// @Author spark
// @Date 2024/12/10 9:21
// @Desc
//-----------------------------------------------------------------------------------
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/robertkrimen/otto"
	"github.com/samber/lo"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
	"io"
	"io/ioutil"
	"strings"
	"time"
)

const sinaStockUrl = "http://hq.sinajs.cn/rn=%d&list=%s"
const txStockUrl = "http://qt.gtimg.cn/?_=%d&q=%s"

const tushareApiUrl = "http://api.tushare.pro"

type StockDataApi struct {
	client *resty.Client
	config *Settings
}
type StockInfo struct {
	gorm.Model
	Date     string  `json:"日期" gorm:"index"`
	Time     string  `json:"时间" gorm:"index"`
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
}

func (receiver FollowedStock) TableName() string {
	return "followed_stock"
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
		client: resty.New(),
		config: GetConfig(),
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
		logger.SugaredLogger.Infof("GetStockCodeRealTimeData %s", url)
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
	logger.SugaredLogger.Errorf("Follow-count %v", count)
	if count >= 63 {
		return "最多只能关注63只股票"
	}

	stockCode = strings.ToLower(stockCode)
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
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Delete(&FollowedStock{})
	return "取消关注成功"
}

func (receiver StockDataApi) SetCostPriceAndVolume(price float64, volume int64, stockCode string) string {
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Update("cost_price", price).Update("volume", volume).Error
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
	err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Updates(&map[string]any{
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
	//	stockCode = strings.ToLower(stockCode)
	//	stockCode = strings.Replace(stockCode, "gb_", "us", 1)
	//}

	// 获取当前排序值
	var currentStock FollowedStock
	if err := db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).First(&currentStock).Error; err != nil {
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
			Where("stock_code = ?", strings.ToLower(stockCode)).
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
			Where("stock_code = ?", strings.ToLower(stockCode)).
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
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(stockCode)).Update("cron", cron)

}
func (receiver StockDataApi) GetFollowList(groupId int) *[]FollowedStock {
	logger.SugaredLogger.Infof("GetFollowList %d", groupId)

	var result *[]FollowedStock
	if groupId == 0 {
		db.Dao.Model(&FollowedStock{}).Order("sort asc,time desc").Find(&result)
	} else {
		infos := NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
		codes := lo.FlatMap(infos, func(info GroupStock, idx int) []string {
			return []string{info.StockCode}
		})
		db.Dao.Model(&FollowedStock{}).Where("stock_code in ?", codes).Order("sort asc,time desc").Find(&result)
		logger.SugaredLogger.Infof("GetFollowList %+v", result)
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

	for _, item := range result2 {
		result = append(result, StockBasic{
			TsCode:   item.TsCode,
			Name:     item.Name,
			Fullname: item.FullName,
			Symbol:   item.Symbol,
			Market:   item.Market,
			ListDate: item.ListDate,
		})
	}
	for _, item := range result3 {
		result = append(result, StockBasic{
			TsCode:   item.Code,
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "HK",
		})
	}
	for _, item := range result4 {
		result = append(result, StockBasic{
			TsCode:   strings.ToLower(strings.Replace(item.Code, "us", "gb_", 1)),
			Name:     item.Name,
			Fullname: item.Name,
			Market:   "US",
		})
	}

	return result
}

func (receiver StockDataApi) GetFollowedStockByStockCode(code string) FollowedStock {
	var result FollowedStock
	db.Dao.Model(&FollowedStock{}).Where("stock_code = ?", strings.ToLower(code)).First(&result)
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

	logger.SugaredLogger.Infof("messages: %s", messages)
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
	logger.SugaredLogger.Infof("CrawlHKStockPriceInfo url:%s", url)
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

	logger.SugaredLogger.Infof("messages: %s", messages)
	return &messages
}

func getZSInfo(name, stockCode string, crawlTimeOut int64) string {
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

// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
func checkChromeOnWindows() (string, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
	if err != nil {
		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
		if err != nil {
			return "", false
		}
		defer key.Close()
	}
	defer key.Close()
	path, _, err := key.GetStringValue("Path")
	//logger.SugaredLogger.Infof("Chrome安装路径：%s", path)
	if err != nil {
		return "", false
	}
	return path + "\\chrome.exe", true
}

// CheckBrowserOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
func CheckBrowserOnWindows() (string, bool) {
	if path, ok := checkChromeOnWindows(); ok {
		return path, true
	}

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
	if err != nil {
		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
		if err != nil {
			return "", false
		}
		defer key.Close()
	}
	defer key.Close()
	path, _, err := key.GetStringValue("Path")
	//logger.SugaredLogger.Infof("Edge安装路径：%s", path)
	if err != nil {
		return "", false
	}
	return path + "\\msedge.exe", true
}

// 分时数据
func (receiver StockDataApi) GetStockMinutePriceData(stockCode string) (*[]MinuteData, string) {
	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/minute/query?code=%s", stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_", "GB_"}) {
		stockCode = strings.Replace(strings.ToUpper(stockCode), "GB_", "us", 1) + ".OQ"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US"}) {
		url = fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/UsMinute/query?code=%s", stockCode)
	}
	logger.SugaredLogger.Infof("GetStockMinutePriceData url:%s", url)
	res := make(map[string]interface{})
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)

	date := ""
	minuteDatas := &[]MinuteData{}

	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
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

	logger.SugaredLogger.Infof("GetHK_KLineData stockCode:%s,kLineType:%s,days:%d", stockCode, kLineType, days)
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
		logger.SugaredLogger.Errorf("err:%s", err.Error())
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
func (receiver StockDataApi) GetSinaHKStockInfo() {

	pageSize := 500
	for i := 1; i <= 3060/pageSize; i++ {
		infos := getSinaStockInfo(receiver, i, pageSize)
		for i, info := range *infos {
			logger.SugaredLogger.Infof("infos:%d,%s:%s", i, info.Symbol, info.Name)
		}
	}

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
		logger.SugaredLogger.Errorf("err:%s", err.Error())
	}
	return infos
}

func (receiver StockDataApi) getDCStockInfo(market string, page, pageSize int) {
	//m:105,m:106,m:107  //美股
	//m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2 //港股
	fs := ""
	switch market {
	case "hk":
		fs = "m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2"
	case "us":
		fs = "m:105,m:106,m:107"
	}

	url := "https://push2.eastmoney.com/api/qt/clist/get?cb=jQuery371047843066631541353_1745889398012&fs=%s&fields=f12,f13,f14,f19,f1,f2,f4,f3,f152,f17,f18,f15,f16,f5,f6&fid=f3&pn=%d&pz=%d&po=1&dect=1"
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "push2.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(fmt.Sprintf(url, fs, page, pageSize))
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return
	}
	body := string(resp.Body())
	body = strutil.ReplaceWithMap(body, map[string]string{
		"jQuery371047843066631541353_1745889398012(": "",
		");": "",
	})
	js := "var d=" + body
	vm := otto.New()
	_, err = vm.Run(js)
	_, err = vm.Run("var data = JSON.stringify(d);")
	value, err := vm.Get("data")
	data := make(map[string]any)
	err = json.Unmarshal([]byte(value.String()), &data)
	if err != nil {
		logger.SugaredLogger.Errorf("json:%s", value.String())
		logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	logger.SugaredLogger.Infof("resp:%s", data)
	if data["data"] != nil {
		datas := data["data"].(map[string]any)
		total := datas["total"].(float64)
		diff := datas["diff"].(map[string]any)
		logger.SugaredLogger.Infof("total:%d", int(total))
		for k, item := range diff {
			stock := item.(map[string]any)
			logger.SugaredLogger.Infof("k:%s,%s:%s", k, stock["f14"], stock["f12"])

			if market == "hk" {
				stockInfo := &models.StockInfoHK{
					Code: strutil.PadStart(stock["f12"].(string), 5, "0") + ".HK",
					Name: stock["f14"].(string),
				}
				db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
				}
			}

			if market == "us" {
				stockInfo := &models.StockInfoUS{
					Code: strutil.PadStart(stock["f12"].(string), 5, "0") + ".US",
					Name: stock["f14"].(string),
				}
				db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stockInfo.Code).First(stockInfo)
				logger.SugaredLogger.Infof("stockInfo:%+v", stockInfo)
				if stockInfo.ID == 0 {
					db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
				}
			}

		}

	}
}

func (receiver StockDataApi) GetHKStockInfo(pageSize int) {
	url := "https://stock.gtimg.cn/data/hk_rank.php?board=main_all&metric=price&pageSize=%d&reqPage=1&order=desc&var_name=list_data"
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "stock.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(fmt.Sprintf(url, pageSize))
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
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
		logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	logger.SugaredLogger.Infof("resp:%s", data)
	if data["code"] != nil && data["code"].(float64) == 0 {
		d := data["data"].(map[string]any)
		saveHKStockInfo(d)

		page_count := int64(d["page_count"].(float64))
		logger.SugaredLogger.Infof("page_count:%d", page_count)
		page := int64(1)
		for page > page_count {
			urlx := fmt.Sprintf("https://stock.gtimg.cn/data/hk_rank.php?board=main_all&metric=price&pageSize=%d&reqPage=%d&order=desc&var_name=list_data", pageSize, page)
			logger.SugaredLogger.Infof("url:%s", urlx)
			resp, err = receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
				SetHeader("Host", "stock.gtimg.cn").
				SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
				Get(urlx)
			if err != nil {
				logger.SugaredLogger.Errorf("err:%s", err.Error())
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
				logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
			}
			logger.SugaredLogger.Infof("resp:%s", data)
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
		logger.SugaredLogger.Infof("vv:%s", vv)
		db.Dao.Model(stock).Where("code = ?", stock.Code).First(stock)
		if stock.ID == 0 {
			logger.SugaredLogger.Infof("stock:%+v", stock)
			db.Dao.Model(&models.StockInfoHK{}).Create(stock)
		}
	}
}

func (receiver StockDataApi) GetCommonKLineData(stockCode string, kLineType string, days int64) *[]KLineData {

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", stockCode, kLineType, days)
	logger.SugaredLogger.Infof("url:%s", url)
	K := &[]KLineData{}
	res := make(map[string]interface{})
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("err:%s", err.Error())
		return K
	}
	logger.SugaredLogger.Infof("resp:%s", resp.Body())
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
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

type MinuteData struct {
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}
