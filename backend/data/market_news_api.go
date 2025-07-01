package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/robertkrimen/otto"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strconv"
	"strings"
	"time"
)

// @Author spark
// @Date 2025/4/23 14:54
// @Desc
// -----------------------------------------------------------------------------------
type MarketNewsApi struct {
}

func NewMarketNewsApi() *MarketNewsApi {
	return &MarketNewsApi{}
}

func (m MarketNewsApi) GetNewTelegraph(crawlTimeOut int64) *[]models.Telegraph {
	url := "https://www.cls.cn/telegraph"
	response, _ := resty.New().SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(fmt.Sprintf(url))
	var telegraphs []models.Telegraph
	//logger.SugaredLogger.Info(string(response.Body()))
	document, _ := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body())))

	document.Find(".telegraph-content-box").Each(func(i int, selection *goquery.Selection) {
		//logger.SugaredLogger.Info(selection.Text())
		telegraph := models.Telegraph{Source: "财联社电报"}
		spans := selection.Find("div.telegraph-content-box span")
		if spans.Length() == 2 {
			telegraph.Time = spans.First().Text()
			telegraph.Content = spans.Last().Text()
			if spans.Last().HasClass("c-de0422") {
				telegraph.IsRed = true
			}
		}

		labels := selection.Find("div a.label-item")
		labels.Each(func(i int, selection *goquery.Selection) {
			if selection.HasClass("link-label-item") {
				telegraph.Url = selection.AttrOr("href", "")
			} else {
				tag := &models.Tags{
					Name: selection.Text(),
					Type: "subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", selection.Text(), "subject").FirstOrCreate(&tag)
				telegraph.SubjectTags = append(telegraph.SubjectTags, selection.Text())
			}
		})
		stocks := selection.Find("div.telegraph-stock-plate-box a")
		stocks.Each(func(i int, selection *goquery.Selection) {
			telegraph.StocksTags = append(telegraph.StocksTags, selection.Text())
		})

		//telegraph = append(telegraph, ReplaceSensitiveWords(selection.Text()))
		if telegraph.Content != "" {
			telegraph.SentimentResult = AnalyzeSentiment(telegraph.Content).Description
			cnt := int64(0)
			db.Dao.Model(telegraph).Where("time=? and source=?", telegraph.Time, telegraph.Source).Count(&cnt)
			if cnt == 0 {
				db.Dao.Create(&telegraph)
				telegraphs = append(telegraphs, telegraph)
				for _, tag := range telegraph.SubjectTags {
					tagInfo := &models.Tags{}
					db.Dao.Model(models.Tags{}).Where("name=? and type=?", tag, "subject").First(&tagInfo)
					if tagInfo.ID > 0 {
						db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tagInfo.ID).FirstOrCreate(&models.TelegraphTags{
							TelegraphId: telegraph.ID,
							TagId:       tagInfo.ID,
						})
					}
				}
			}

		}
	})
	return &telegraphs
}
func (m MarketNewsApi) GetNewsList(source string, limit int) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("id desc").Limit(limit).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("id desc").Limit(limit).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}
func (m MarketNewsApi) GetTelegraphList(source string) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("id desc").Limit(20).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("id desc").Limit(20).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}

func (m MarketNewsApi) GetSinaNews(crawlTimeOut uint) *[]models.Telegraph {
	news := &[]models.Telegraph{}
	response, _ := resty.New().SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://zhibo.sina.com.cn/api/zhibo/feed?callback=callback&page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20&id=4161089&type=0&_=" + strconv.FormatInt(time.Now().Unix(), 10))
	js := string(response.Body())
	js = strutil.ReplaceWithMap(js, map[string]string{
		"try{callback(":  "var data=",
		");}catch(e){};": ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	_, err := vm.Run(js)
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	vm.Run("var result = data.result;")
	//vm.Run("var resultStr =JSON.stringify(data);")
	vm.Run("var resultData = result.data;")
	vm.Run("var feed = resultData.feed;")
	vm.Run("var feedStr = JSON.stringify(feed);")

	value, _ := vm.Get("feedStr")
	//resultStr, _ := vm.Get("resultStr")

	//logger.SugaredLogger.Info(resultStr)
	feed := make(map[string]any)
	err = json.Unmarshal([]byte(value.String()), &feed)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	var telegraphs []models.Telegraph

	if feed["list"] != nil {
		for _, item := range feed["list"].([]any) {
			telegraph := models.Telegraph{Source: "新浪财经"}
			data := item.(map[string]any)
			//logger.SugaredLogger.Infof("%s:%s", data["create_time"], data["rich_text"])
			telegraph.Content = data["rich_text"].(string)
			telegraph.Time = strings.Split(data["create_time"].(string), " ")[1]
			tags := data["tag"].([]any)
			telegraph.SubjectTags = lo.Map(tags, func(tagItem any, index int) string {
				name := tagItem.(map[string]any)["name"].(string)
				tag := &models.Tags{
					Name: name,
					Type: "sina_subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "sina_subject").FirstOrCreate(&tag)
				return name
			})
			if _, ok := lo.Find(telegraph.SubjectTags, func(item string) bool { return item == "焦点" }); ok {
				telegraph.IsRed = true
			}
			logger.SugaredLogger.Infof("telegraph.SubjectTags:%v %s", telegraph.SubjectTags, telegraph.Content)

			if telegraph.Content != "" {
				telegraph.SentimentResult = AnalyzeSentiment(telegraph.Content).Description
				cnt := int64(0)
				db.Dao.Model(telegraph).Where("time=? and source=?", telegraph.Time, telegraph.Source).Count(&cnt)
				if cnt == 0 {
					db.Dao.Create(&telegraph)
					telegraphs = append(telegraphs, telegraph)
					for _, tag := range telegraph.SubjectTags {
						tagInfo := &models.Tags{}
						db.Dao.Model(models.Tags{}).Where("name=? and type=?", tag, "sina_subject").First(&tagInfo)
						if tagInfo.ID > 0 {
							db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tagInfo.ID).FirstOrCreate(&models.TelegraphTags{
								TelegraphId: telegraph.ID,
								TagId:       tagInfo.ID,
							})
						}
					}
				}
			}
		}
		return &telegraphs
	}

	return news

}

func (m MarketNewsApi) GlobalStockIndexes(crawlTimeOut uint) map[string]any {
	response, _ := resty.New().SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/mstats").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/rank/indexRankDetail2")
	js := string(response.Body())
	res := make(map[string]any)
	json.Unmarshal([]byte(js), &res)
	return res["data"].(map[string]any)
}

func (m MarketNewsApi) GetIndustryRank(sort string, cnt int) map[string]any {

	url := fmt.Sprintf("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktHs/rank?l=%d&p=1&t=01/averatio&ordertype=&o=%s", cnt, sort)
	response, _ := resty.New().SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := make(map[string]any)
	json.Unmarshal([]byte(js), &res)
	return res
}

func (m MarketNewsApi) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_bk?page=1&num=20&sort=%s&asc=0&fenlei=%s", sort, fenlei)

	response, _ := resty.New().SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetMoneyRankSina(sort string) []map[string]any {
	if sort == "" {
		sort = "netamount"
	}
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_ssggzj?page=1&num=20&sort=%s&asc=0&bankuai=&shichang=", sort)
	response, _ := resty.New().SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_qsfx_zjlrqs?page=1&num=%d&sort=opendate&asc=0&daima=%s", days, stockCode)

	response, _ := resty.New().SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res

}

func (m MarketNewsApi) TopStocksRankingList(date string) {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/q/go.php/vInvestConsult/kind/lhb/index.phtml?tradedate=%s", date)
	response, _ := resty.New().SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").Get(url)

	html, _ := convertor.GbkToUtf8(response.Body())
	//logger.SugaredLogger.Infof("html:%s", html)
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return
	}
	document.Find("table.list_table").Each(func(i int, s *goquery.Selection) {
		title := strutil.Trim(s.Find("tr:first-child").First().Text())
		logger.SugaredLogger.Infof("title:%s", title)
		s.Find("tr:not(:first-child)").Each(func(i int, s *goquery.Selection) {
			logger.SugaredLogger.Infof("s:%s", strutil.RemoveNonPrintable(s.Text()))
		})
	})

}

func (m MarketNewsApi) LongTiger(date string) *[]models.LongTigerRankData {
	ranks := &[]models.LongTigerRankData{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get"
	logger.SugaredLogger.Infof("url:%s", url)
	params := make(map[string]string)
	params["callback"] = "callback"
	params["sortColumns"] = "TURNOVERRATE,TRADE_DATE,SECURITY_CODE"
	params["sortTypes"] = "-1,-1,1"
	params["pageSize"] = "500"
	params["pageNumber"] = "1"
	params["reportName"] = "RPT_DAILYBILLBOARD_DETAILSNEW"
	params["columns"] = "SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,FREE_MARKET_CAP,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE,SECURITY_TYPE_CODE"
	params["source"] = "WEB"
	params["client"] = "WEB"
	params["filter"] = fmt.Sprintf("(TRADE_DATE<='%s')(TRADE_DATE>='%s')", date, date)
	resp, err := resty.New().SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/stock/tradedetail.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetQueryParams(params).
		Get(url)
	if err != nil {
		return ranks
	}
	js := string(resp.Body())
	logger.SugaredLogger.Infof("resp:%s", js)

	js = strutil.ReplaceWithMap(js, map[string]string{
		"callback(": "var data=",
		");":        ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	_, err = vm.Run(js)
	_, err = vm.Run("var data = JSON.stringify(data);")
	value, err := vm.Get("data")
	logger.SugaredLogger.Infof("resp-json:%s", value.String())
	data := gjson.Get(value.String(), "result.data")
	logger.SugaredLogger.Infof("resp:%v", data)
	err = json.Unmarshal([]byte(data.String()), ranks)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return ranks
	}
	for _, rankData := range *ranks {
		temp := &models.LongTigerRankData{}
		db.Dao.Model(temp).Where(&models.LongTigerRankData{
			TRADEDATE: rankData.TRADEDATE,
			SECUCODE:  rankData.SECUCODE,
		}).First(temp)
		if temp.SECURITYTYPECODE == "" {
			db.Dao.Model(temp).Create(&rankData)
		}
	}
	return ranks
}

func (m MarketNewsApi) IndustryResearchReport(industryCode string, days int) []any {
	beginDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	if strutil.Trim(industryCode) != "" {
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	logger.SugaredLogger.Infof("IndustryResearchReport-name:%s", industryCode)
	params := map[string]string{
		"industry":     "*",
		"industryCode": industryCode,
		"beginTime":    beginDate,
		"endTime":      endDate,
		"pageNo":       "1",
		"pageSize":     "50",
		"p":            "1",
		"pageNum":      "1",
		"pageNumber":   "1",
		"qType":        "1",
	}

	url := "https://reportapi.eastmoney.com/report/list"

	logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := resty.New().SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap["data"].([]any)
}
func (m MarketNewsApi) StockResearchReport(stockCode string, days int) []any {
	beginDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strings.Split(stockCode, ".")[0]
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	} else {
		stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
			"sh":  "",
			"sz":  "",
			"gb_": "",
			"us":  "",
			"us_": "",
		})
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	logger.SugaredLogger.Infof("StockResearchReport-stockCode:%s", stockCode)

	type Req struct {
		BeginTime    string      `json:"beginTime"`
		EndTime      string      `json:"endTime"`
		IndustryCode string      `json:"industryCode"`
		RatingChange string      `json:"ratingChange"`
		Rating       string      `json:"rating"`
		OrgCode      interface{} `json:"orgCode"`
		Code         string      `json:"code"`
		Rcode        string      `json:"rcode"`
		PageSize     int         `json:"pageSize"`
		PageNo       int         `json:"pageNo"`
		P            int         `json:"p"`
		PageNum      int         `json:"pageNum"`
		PageNumber   int         `json:"pageNumber"`
	}

	url := "https://reportapi.eastmoney.com/report/list2"

	logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := resty.New().SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetBody(&Req{
			Code:         stockCode,
			IndustryCode: "*",
			BeginTime:    beginDate,
			EndTime:      endDate,
			PageNo:       1,
			PageSize:     50,
			P:            1,
			PageNum:      1,
			PageNumber:   1,
		}).Post(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap["data"].([]any)
}

func (m MarketNewsApi) StockNotice(stock_list string) []any {
	var stockCodes []string
	for _, stockCode := range strings.Split(stock_list, ",") {
		if strutil.ContainsAny(stockCode, []string{"."}) {
			stockCode = strings.Split(stockCode, ".")[0]
			stockCodes = append(stockCodes, stockCode)
		} else {
			stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
				"sh":  "",
				"sz":  "",
				"gb_": "",
				"us":  "",
				"us_": "",
			})
			stockCodes = append(stockCodes, stockCode)
		}
	}

	url := "https://np-anotice-stock.eastmoney.com/api/security/ann?page_size=50&page_index=1&ann_type=SHA%2CCYB%2CSZA%2CBJA%2CINV&client_source=web&f_node=0&stock_list=" + strings.Join(stockCodes, ",")
	resp, err := resty.New().SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "np-anotice-stock.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/notices/hsa/5.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return (respMap["data"].(map[string]any))["list"].([]any)
}

func (m MarketNewsApi) EMDictCode(code string, cache *freecache.Cache) []any {
	respMap := map[string]any{}

	d, _ := cache.Get([]byte(code))
	if d != nil {
		json.Unmarshal(d, &respMap)
		return respMap["data"].([]any)
	}

	url := "https://reportapi.eastmoney.com/report/bk"

	params := map[string]string{
		"bkCode": code,
	}
	resp, err := resty.New().SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/industry.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	cache.Set([]byte(code), resp.Body(), 60*60*24)
	return respMap["data"].([]any)
}

func (m MarketNewsApi) TradingViewNews() *[]models.TVNews {
	TVNews := &[]models.TVNews{}
	url := "https://news-mediator.tradingview.com/news-flow/v2/news?filter=lang:zh-Hans&filter=provider:panews,reuters&client=screener&streaming=false"
	resp, err := resty.New().SetProxy("http://127.0.0.1:10809").SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "news-mediator.tradingview.com").
		SetHeader("Origin", "https://cn.tradingview.com").
		SetHeader("Referer", "https://cn.tradingview.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("TradingViewNews err:%s", err.Error())
		return TVNews
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	if err != nil {
		return TVNews
	}
	items, err := json.Marshal(respMap["items"])
	if err != nil {
		return TVNews
	}
	json.Unmarshal(items, TVNews)
	return TVNews
}

func (m MarketNewsApi) XUEQIUHotStock(size int, marketType string) *[]models.HotItem {
	request := resty.New().SetTimeout(time.Duration(30) * time.Second).R()
	_, err := request.
		SetHeader("Host", "xueqiu.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get("https://xueqiu.com/hq#hot")

	//cookies := resp.Header().Get("Set-Cookie")
	//logger.SugaredLogger.Infof("cookies:%s", cookies)

	url := fmt.Sprintf("https://stock.xueqiu.com/v5/stock/hot_stock/list.json?page=1&size=%d&_type=%s&type=%s", size, marketType, marketType)
	res := &models.XUEQIUHot{}
	_, err = request.
		SetHeader("Host", "stock.xueqiu.com").
		SetHeader("Origin", "https://xueqiu.com").
		SetHeader("Referer", "https://xueqiu.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		//SetHeader("Cookie", "cookiesu=871730774144180; device_id=ee75cebba8a35005c9e7baf7b7dead59; s=ch12b12pfi; Hm_lvt_1db88642e346389874251b5a1eded6e3=1746247619; xq_a_token=361dcfccb1d32a1d9b5b65f1a188b9c9ed1e687d; xqat=361dcfccb1d32a1d9b5b65f1a188b9c9ed1e687d; xq_r_token=450d1db0db9659a6af7cc9297bfa4fccf1776fae; xq_id_token=eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJ1aWQiOi0xLCJpc3MiOiJ1YyIsImV4cCI6MTc1MzgzODAwNiwiY3RtIjoxNzUxMjUxMzc2MDY3LCJjaWQiOiJkOWQwbjRBWnVwIn0.TjEtQ5WEN4ajnVjVnY3J-Qq9LjL-F0eat9Cefv_tLJLqsPhzD2y8Lc1CeIu0Ceqhlad7O_yW1tR9nb2dIjDpyOPzWKxvwSOKXLm8XMoz4LMgE2pysBCH4TsetzHsEOhBsY467q-JX3WoFuqo-dqv1FfLSondZCspjEMFdgPFt2V-2iXJY05YUwcBVUvL74mT9ZjNq0KaDeRBJk_il6UR8yibG7RMbe9xWYz5dSO_wJwWuxvnZ8u9EXC2m-TV7-QHVxFHR_5e8Fodrzg0yIcLU4wBTSoIIQDUKqngajX2W-nUAdo6fr78NNDmoswFVH7T7XMuQciMAqj9MpMCVW3Sog; u=871730774144180; ssxmod_itna=iq+h7KAImDORKYQ4Y5G=nxBKDtD7D3qCD0dGMDxeq7tDRDFqApKDHtA68oon7ziBA0+PbZ9xGN4oYxiNDAPq0iDC+Wjxs9Orw5KQb9iqP4MAn0TbNsbtU22eqbCe=S3vTv6xoDHxY=DU1GzeieDx=PD5xDTDWeDGDD3DmnsDi5YD0KDjBYpH+omDYPDEBYDaxDbDimwY4GCrDDCtc5Dw6bmzDDzznL5WWAPzWffZg3YcFgxf8GwD7y3Dla4rMhw23=cz0Efdk0A5hYDXotDvhoY1/H6neEvOt3o=Q0ruT+5RuxoRhDxCmh5tGP32xBD5G0xS2xcb4quDK0Dy2ZmY/DDWM0qmEeSEDeOCIq1fw1misCY=WAzoOtMwDzGdUjpRk5Z0xQBDI2IMw4H7qNiNBLxWiDD; ssxmod_itna2=iq+h7KAImDORKYQ4Y5G=nxBKDtD7D3qCD0dGMDxeq7tDRDFqApKDHtA68oon7ziBA0+PbZYxD3boBmiEPtDFOEPAeFmDDsuGSxf46oGKwGHd8wtUjFe+oV1lxUzutkGly=nCyCjq=UTHxMxFCr1DsFiKPuEpPVO7GrOyk5Aymnc0+11AFND7v16PvwrFQH4I72=3O1OpK7rGw+poWNCxjj=Ka5QDFWAvEzrDFQcIH=GpKpS90FAyIzGcTyck+yhQKaojn96dRqeIh=HkaFrlGnKwzO+a49=F7/c/MejoR3QM20K9IIOymrMN2bsk2TRdKFiaf4O0ut2MauiOER=iQNW2WVgDrkKzD=57r577wEx2hwkqhf8T8BDvkHZRDirC0bNK4O=G3TSkd3wYwq8bst0t9qF/e3M87NYtU2IWYWzqd=BqEfdqGq0R8wxmqLzpeGeuwSTq1OAiB87gDrozjnGkwDKRdrLz8uDjQKVlGhWk8Wd/rXQjx4pG=BNqpW/6TS1wpfxzGf5CrUhtt0j0wC5AUFo2GbX+QXPzD2guxKXrx8lZUQlwWIHyEUz+OLh0eWUkfHfM0YWXlgOejnuUa06rW9y5maDPipGms751hxKcqLq62pQty4iX3QDF6SRQd3tfEBf3CH7r2xe2qq0qdOI5Ge=GezD/Us5Z0xQBwVAZ2N/XvD0HDD").
		SetResult(res).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("XUEQIUHotStock err:%s", err.Error())
		return &[]models.HotItem{}
	}
	logger.SugaredLogger.Infof("XUEQIUHotStock:%+v", res)
	return &res.Data.Items
}

func (m MarketNewsApi) HotEvent(size int) *[]models.HotEvent {
	events := &[]models.HotEvent{}
	url := fmt.Sprintf("https://xueqiu.com/hot_event/list.json?count=%d", size)
	resp, err := resty.New().SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "xueqiu.com").
		SetHeader("Origin", "https://xueqiu.com").
		SetHeader("Referer", "https://xueqiu.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Cookie", "cookiesu=2617378771242871; s=c2121pp1u71; device_id=237a58584ec58d8e4d4e1040700a644f1; Hm_lvt_1db88642e346389874251b5a1eded6e3=1744100219,1744599115; xq_a_token=b7259d09435458cc3f1a963479abb270a1a016ce; xqat=b7259d09435458cc3f1a963479abb270a1a016ce; xq_r_token=28108bfa1d92ac8a46bbb57722633746218621a3; xq_id_token=eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJ1aWQiOi0xLCJpc3MiOiJ1YyIsImV4cCI6MTc1MjU0MTk4OCwiY3RtIjoxNzUwMjMwNjA2NzI0LCJjaWQiOiJkOWQwbjRBWnVwIn0.kU_fz0luJoE7nr-K4UrNUi5-mAG-vMdXtuC4mUKIppILId4UpF70LB70yunxGiNSw6tPFR3-hyLvztKAHtekCUTm3XjUl5b3tEDP-ZUVqHnWXO_5hoeMI8h-Cfx6ZGlIr5x3icvTPkT0OV5CD5A33-ZDTKhKPf-DhJ_-m7CG5GbX4MseOBeMXuLUQUiYHPKhX1QUc0GTGrCzi8Mki0z49D0LVqCSgbsx3UGfowOOyx85_cXb4OAFvIjwbs2p0o_h-ibIT0ngVkkAyEDetVvlcZ_bkardhseCB7k9BEMgH2z8ihgkVxyy3P0degLmDUruhmqn5uZOCi1pVBDvCv9lBg; u=261737877124287; ssxmod_itna=QuG=D5AKiKDIqCqGKi7G7DgmmPlSDWFqKGHDyx4YK0CDmxjKiddDUQivnb8xpnQcGyGYoYhoqEeDBubrDSxD67DK4GTm+ogiw1o3B=xedQHDgBtN=7/i1K53N+rOjquLMU=kbqYxB3DExGkqj0tPi4DxaPD5xDTDWeDGDD3DnnsDQKDRx0kL0oDIxD1D0bmHUEvh38mDYePLmOmDYPYx94Y8KoDeEgsD7HUl/vIGGEAqjLPFegXLD0HolCqr4DCid1qDm+ECfkjDn9sD0KP8fn+CRoDv=tYr4ibx+o=W+8vstf9mjGe3cXseWdBmoFrmf4DA3bFAxnAxD7vYxADaDoerDGHPoxHF+PKGPtDKmiqQGeB5qbi4eg4KDHKDe3DeG0qeEP9xVUoHDDWMYYM0ICr4FBimBDM7D0x4QOECmhul5QCN/m5/74lGm=7x9Wp7A+i7xQ7wlMD4D; ssxmod_itna2=QuG=D5AKiKDIqCqGKi7G7DgmmPlSDWFqKGHDyx4YK0CDmxjKiddDUQivnb8xpnQcGyGYoYhoqoDirSDhPmGD24GajjDuGE3m7or4DlxOSGewHl6iaus2Q62SRX5CFjCds6ltF9xy6iaUuB262UkhRA8UXST=4/b+y3kGKzlGE8T29FA008ljy9jXXC7f7m7QsK667mlUooWrofk=qGZjxtcUrN1NtuAnne1hj+rQP5UnlFkxf+o7VjmatH7u7bCDlbTt3cz6CH9Fl4vye16W/ellc8I3Q37W7ZwiLGD/zPpZcnd2nsqqo/+zRbKAmz4plzwaDqGUe7f9E+P0IFRKqpRv+buQFHBSpcbwND7Q+9XWmnjI2UwKd98jIS3gPXwxvbx4OuiyH8gZ+OEt7DgE/AY/9W4VxDZrlFWyWnC4y4/I0IpAfaGKpbPmauKbkqawqv93vSf+9HamGe0Dt2PNgT3yiEB4vQP2/DdVpcGBOjFujWoHP32OshLPYI20LRCKddwEGkKqPzPwKPc3X5zuB=w2fUdtwKsAW5kQtsl8clNwjC5uDYrxR0h9xaj0xmD+YuI3GPT7xYTalRImPj2wL2=+91a304xa4bTWtP=dLGARhb/efRi0uktaz8i8C04G0x/ZWUzqRza8GGU=FfRfvb4GZM/q2rVsl0nLvRjGeAKgocLouyXs/uwZu3YxbAx30qCbjG1A533zAxIeIgD=0VAc3ixD").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotEvent err:%s", err.Error())
		return events
	}
	//logger.SugaredLogger.Infof("HotEvent:%s", resp.Body())
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	items, err := json.Marshal(respMap["list"])
	if err != nil {
		return events
	}
	json.Unmarshal(items, events)
	return events

}

func (m MarketNewsApi) HotTopic(size int) []any {
	url := "https://gubatopic.eastmoney.com/interface/GetData.aspx?path=newtopic/api/Topic/HomePageListRead"
	resp, err := resty.New().SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "gubatopic.eastmoney.com").
		SetHeader("Origin", "https://gubatopic.eastmoney.com").
		SetHeader("Referer", "https://gubatopic.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetFormData(map[string]string{
			"param": fmt.Sprintf("ps=%d&p=1&type=0", size),
			"path":  "newtopic/api/Topic/HomePageListRead",
			"env":   "2",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotTopic err:%s", err.Error())
		return []any{}
	}
	//logger.SugaredLogger.Infof("HotTopic:%s", resp.Body())
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	return respMap["re"].([]any)

}

func (m MarketNewsApi) InvestCalendar(yearMonth string) []any {
	if yearMonth == "" {
		yearMonth = time.Now().Format("2006-01")
	}

	url := "https://app.jiuyangongshe.com/jystock-app/api/v1/timeline/list"
	resp, err := resty.New().SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "app.jiuyangongshe.com").
		SetHeader("Origin", "https://www.jiuyangongshe.com").
		SetHeader("Referer", "https://www.jiuyangongshe.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetHeader("token", "1cc6380a05c652b922b3d85124c85473").
		SetHeader("platform", "3").
		SetHeader("Cookie", "SESSION=NDZkNDU2ODYtODEwYi00ZGZkLWEyY2ItNjgxYzY4ZWMzZDEy").
		SetHeader("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10)).
		SetBody(map[string]string{
			"date":  yearMonth,
			"grade": "0",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("InvestCalendar err:%s", err.Error())
		return []any{}
	}
	//logger.SugaredLogger.Infof("InvestCalendar:%s", resp.Body())
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	return respMap["data"].([]any)

}

func (m MarketNewsApi) ClsCalendar() []any {
	url := "https://www.cls.cn/api/calendar/web/list?app=CailianpressWeb&flag=0&os=web&sv=8.4.6&type=0&sign=4b839750dc2f6b803d1c8ca00d2b40be"
	resp, err := resty.New().SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "www.cls.cn").
		SetHeader("Origin", "https://www.cls.cn").
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ClsCalendar err:%s", err.Error())
		return []any{}
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	return respMap["data"].([]any)
}
