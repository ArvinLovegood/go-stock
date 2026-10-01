package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

func checkIsIndexBasic(stock string) bool {
	count := int64(0)
	db.Dao.Model(&IndexBasic{}).Where("name =  ?", stock).Count(&count)
	return count > 0
}

func SearchGuShiTongStockInfo(stock string, crawlTimeOut int64) *[]string {
	// 币安永续合约为 24/7 衍生品，百度股市通无对应个股资讯页，直接返回空
	if IsBinanceFuturesCode(stock) {
		return &[]string{}
	}
	// Bitget 美股永续合约为 bt: 前缀衍生品代码，百度股市通无对应页面，直接返回空
	if IsBitgetFuturesCode(stock) {
		return &[]string{}
	}
	crawlerAPI := CrawlerApi{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()

	crawlerAPI = crawlerAPI.NewCrawler(ctx, CrawlerBaseInfo{
		Name:    "百度股市通",
		BaseUrl: "https://gushitong.baidu.com",
		Headers: map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	})
	url := "https://gushitong.baidu.com/stock/ab-" + RemoveAllNonDigitChar(stock)

	if strutil.HasPrefixAny(stock, []string{"HK", "hk"}) {
		url = "https://gushitong.baidu.com/stock/hk-" + RemoveAllNonDigitChar(stock)
	}
	if strutil.HasPrefixAny(stock, []string{"SZ", "SH", "sh", "sz"}) {
		url = "https://gushitong.baidu.com/stock/ab-" + RemoveAllNonDigitChar(stock)
	}
	if strutil.HasPrefixAny(stock, []string{"us", "US", "gb_", "gb"}) {
		url = "https://gushitong.baidu.com/stock/us-" + strings.Replace(stock, "gb_", "", 1)
	}

	//logger.SugaredLogger.Infof("SearchGuShiTongStockInfo搜索股票-%s: %s", stock, url)
	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitVisible("div.cos-tab"),
		chromedp.Click("div.cos-tab:nth-child(5)", chromedp.ByQuery),
		chromedp.ScrollIntoView("div.body-box"),
		chromedp.WaitVisible("div.body-col"),
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil),
		chromedp.Sleep(1 * time.Second),
	}
	htmlContent, success := crawlerAPI.GetHtmlWithActions(&actions, true)
	var messages []string
	if success {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return &[]string{}
		}
		document.Find("div.finance-hover,div.list-date").Each(func(i int, selection *goquery.Selection) {
			text := strutil.RemoveWhiteSpace(selection.Text(), false)
			messages = append(messages, ReplaceSensitiveWords(text))
			//logger.SugaredLogger.Infof("SearchGuShiTongStockInfo搜索到消息-%s: %s", "", text)
		})
		//logger.SugaredLogger.Infof("messages:%d", len(messages))
	}
	return &messages
}

func GetFinancialReportsByXUEQIU(stockCode string, crawlTimeOut int64) *[]string {
	// 币安永续合约无财务报表，直接返回空
	if IsBinanceFuturesCode(stockCode) {
		return &[]string{}
	}
	// Bitget 美股永续合约代码无对应雪球个股页，直接返回空
	if IsBitgetFuturesCode(stockCode) {
		return &[]string{}
	}
	if strutil.HasPrefixAny(stockCode, []string{"HK", "hk"}) {
		stockCode = strings.ReplaceAll(stockCode, "hk", "")
		stockCode = strings.ReplaceAll(stockCode, "HK", "")
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "gb_"}) {
		stockCode = strings.ReplaceAll(stockCode, "us", "")
		stockCode = strings.ReplaceAll(stockCode, "gb_", "")
	}
	url := fmt.Sprintf("https://xueqiu.com/snowman/S/%s/detail#/ZYCWZB", stockCode)
	waitVisible := "div.tab-table-responsive table"
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://xueqiu.com",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	var markdown strings.Builder
	markdown.WriteString("\n## 财务数据：\n")
	html, ok := crawlerAPI.GetHtml(url, waitVisible, true)
	if !ok {
		return &[]string{""}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	GetTableMarkdown(document, waitVisible, &markdown)
	return &[]string{markdown.String()}
}

func GetFinancialReports(stockCode string, crawlTimeOut int64) *[]string {
	// 币安永续合约无财务指标页，直接返回空
	if IsBinanceFuturesCode(stockCode) {
		return &[]string{}
	}
	// Bitget 美股永续合约无财务指标页，直接返回空
	if IsBitgetFuturesCode(stockCode) {
		return &[]string{}
	}
	url := "https://emweb.securities.eastmoney.com/pc_hsf10/pages/index.html?type=web&code=" + stockCode + "#/cwfx"
	waitVisible := "div.report_table table"
	if strutil.HasPrefixAny(stockCode, []string{"HK", "hk"}) {
		stockCode = strings.ReplaceAll(stockCode, "hk", "")
		stockCode = strings.ReplaceAll(stockCode, "HK", "")
		url = "https://emweb.securities.eastmoney.com/PC_HKF10/pages/home/index.html?code=" + stockCode + "&type=web&color=w#/NewFinancialAnalysis"
		waitVisible = "div table.commonTable"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "gb_"}) {
		stockCode = strings.ReplaceAll(stockCode, "us", "")
		stockCode = strings.ReplaceAll(stockCode, "gb_", "")
		url = "https://emweb.securities.eastmoney.com/pc_usf10/pages/index.html?type=web&code=" + stockCode + "#/cwfx"
		waitVisible = "div.zyzb_table_detail table"

	}

	//logger.SugaredLogger.Infof("GetFinancialReports搜索股票-%s: %s", stockCode, url)

	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://emweb.securities.eastmoney.com",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	var markdown strings.Builder
	markdown.WriteString("\n## 财务数据：\n")
	html, ok := crawlerAPI.GetHtml(url, waitVisible, true)
	if !ok {
		return &[]string{""}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	GetTableMarkdown(document, waitVisible, &markdown)
	return &[]string{markdown.String()}
}

func GetTelegraphList(crawlTimeOut int64) *[]string {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	response, err := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		Get(clsURL)
	if err != nil {
		return &[]string{}
	}
	res := map[string]any{}
	if err := json.Unmarshal(response.Body(), &res); err != nil {
		return &[]string{}
	}
	var telegraph []string
	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return &[]string{}
		}
		dataMap, ok := res["data"].(map[string]any)
		if !ok {
			return &[]string{}
		}
		rollData, ok := dataMap["roll_data"].([]any)
		if !ok {
			return &[]string{}
		}
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			content, _ := news["content"].(string)
			if content != "" {
				telegraph = append(telegraph, ReplaceSensitiveWords(content))
			}
		}
	}
	return &telegraph
}

func GetTopNewsList(crawlTimeOut int64) *[]string {
	url := "https://www.cls.cn"
	response, err := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	if err != nil {
		return &[]string{}
	}
	//logger.SugaredLogger.Info(string(response.Body()))
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body())))
	if err != nil {
		return &[]string{}
	}
	var telegraph []string
	document.Find("div.home-article-title a,div.home-article-rec a").Each(func(i int, selection *goquery.Selection) {
		//logger.SugaredLogger.Info(selection.Text())
		telegraph = append(telegraph, ReplaceSensitiveWords(selection.Text()))
	})
	return &telegraph
}
