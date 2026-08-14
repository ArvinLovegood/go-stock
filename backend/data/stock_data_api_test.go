package data

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
)

// @Author spark
// @Date 2024/12/10 9:55
// @Desc
//-----------------------------------------------------------------------------------

func TestGetTelegraph(t *testing.T) {
	db.Init("../../data/stock.db")

	//telegraphs := GetTelegraphList(30)
	//for _, telegraph := range *telegraphs {
	//	logger.SugaredLogger.Info(telegraph)
	//}
	list := NewMarketNewsApi().GetNewTelegraph(30)
	for _, telegraph := range *list {
		logger.SugaredLogger.Infof("telegraph:%+v", telegraph)
	}
}

func TestGetFinancialReports(t *testing.T) {
	db.Init("../../data/stock.db")
	//GetFinancialReports("sz000802", 30)
	//GetFinancialReports("hk00927", 30)
	//GetFinancialReports("gb_aapl", 30)
	GetFinancialReportsByXUEQIU("sz000802", 30)
	GetFinancialReportsByXUEQIU("gb_aapl", 30)
	GetFinancialReportsByXUEQIU("hk00927", 30)

}

func TestGetTelegraphSearch(t *testing.T) {
	db.Init("../../data/stock.db")
	searchWords := "半导体 新能源汽车 机器人"
	//url := "https://www.cls.cn/searchPage?keyword=%E9%97%BB%E6%B3%B0%E7%A7%91%E6%8A%80&type=telegram"
	messages := SearchStockInfo(searchWords, "telegram", 30)
	for _, message := range *messages {
		logger.SugaredLogger.Info(message)
	}

	//https://www.cls.cn/stock?code=sh600745
}
func TestCailianpressWeb(t *testing.T) {
	db.Init("../../data/stock.db")
	searchWords := ""
	res := NewMarketNewsApi().CailianpressWeb(searchWords)
	md := util.MarkdownTableWithTitle(searchWords+"财联社新闻", res.List)
	logger.SugaredLogger.Info(md)
}
func TestGetAllStocks(t *testing.T) {
	db.Init("../../data/stock.db")
	db.Dao.AutoMigrate(&models.AllStockInfo{})

	db.Dao.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{})
	for page := 1; page < 3; page++ {
		res := NewStockDataApi().GetAllStocks(page, 3000, "", models.TechnicalIndicators{
			BEARISHENGULFING: true,
			BLACKCLOUDTOPS:   true,
		})
		var datas []models.AllStockInfo
		for _, data := range (*res).Result.Data {
			datas = append(datas, data.ToAllStockInfo())
		}
		err := db.Dao.CreateInBatches(&datas, 500).Error
		if err != nil {
			logger.SugaredLogger.Errorf("db.Dao.CreateInBatches error:%s", err.Error())
		}
	}
}
func TestFilterStocks(t *testing.T) {
	db.Init("../../data/stock.db")

	res := NewStockDataApi().GetAllStocks(1, 100, "", models.TechnicalIndicators{
		CONCERN_RANK_7DAYS: 50,
	})
	logger.SugaredLogger.Infof("%+#v", len((*res).Result.Data))

}
func TestSearchStockInfoByCode(t *testing.T) {
	db.Init("../../data/stock.db")
	SearchStockInfoByCode("sh600745")
}

func TestSearchStockPriceInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	SearchStockPriceInfo("博安生物", "hk06955", 30)
	SearchStockPriceInfo("上海贝岭", "sh600171", 30)
	//SearchStockPriceInfo("苹果公司", "gb_aapl", 30)
	//SearchStockPriceInfo("微创光电", "bj430198", 30)
	//getZSInfo("创业板指数", "sz399006", 30)
	//getZSInfo("上证综合指数", "sh000001", 30)
	//getZSInfo("沪深300指数", "sh000300", 30)

}
func TestGetStockMinutePriceData(t *testing.T) {
	db.Init("../../data/stock.db")
	data, date := NewStockDataApi().GetStockMinutePriceData("sh600171")
	logger.SugaredLogger.Infof("date:%s", date)
	logger.SugaredLogger.Infof("%+#v", *data)
}
func TestGetKLineData(t *testing.T) {
	db.Init("../../data/stock.db")
	k := NewStockDataApi().GetKLineData("sh600171", "240", 30)
	//for _, kline := range *k {
	//	logger.SugaredLogger.Infof("%+#v", kline)
	//}
	jsonData, _ := json.Marshal(*k)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
	}
	logger.SugaredLogger.Infof("markdownTable:\n%s", markdownTable)

}
func TestGetHK_KLineData(t *testing.T) {
	db.Init("../../data/stock.db")
	k := NewStockDataApi().GetHK_KLineData("hk01810", "day", 1)
	jsonData, _ := json.Marshal(*k)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Marshal error:%s", err.Error())
	}
	logger.SugaredLogger.Infof("markdownTable:\n%s", markdownTable)

}

func TestGetHKStockInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	//NewStockDataApi().GetHKStockInfo(200)
	//NewStockDataApi().GetSinaHKStockInfo()
	//m:105,m:106,m:107  //美股
	//m:128+t:3,m:128+t:4,m:128+t:1,m:128+t:2 //港股
	//274  224 605
	for i := 197; i <= 274; i++ {
		NewStockDataApi().getDCStockInfo("", i, 20)
		time.Sleep(time.Duration(random.RandInt(2, 5)) * time.Second)
	}
}

func TestParseTxStockData(t *testing.T) {
	str := "v_r_hk09660=\"100~地平线机器人-W~09660~6.340~5.690~5.800~210980204.0~0~0~6.340~0~0~0~0~0~0~0~0~0~6.340~0~0~0~0~0~0~0~0~0~210980204.0~2025/04/29\n14:14:52~0.650~11.42~6.450~5.710~6.340~210980204.0~1295585259.040~0~33.03~~0~0~13.01~702.2123~836.8986~HORIZONROBOT-W~0.00~10.380~3.320~1.00~-53.74~0~0~0~0~0~33.03~6.50~1.90~600~76.11~19.85~GP~19.70~11.51~0.63~-17.23~46.76~13200293682.00~11075904412.00~33.03~0.000~6.141~58.90~HKD~1~30\";"
	//str = "v_sz002241=\"51~歌尔股份~002241~22.26~22.27~0.00~0~0~0~22.26~1004~0.00~0~0.00~0~0.00~0~0.00~0~22.26~1004~0.00~558~0.00~0~0.00~0~0.00~0~~20250509092233~-0.01~-0.04~0.00~0.00~22.26/0/0~0~0~0.00~28.21~~0.00~0.00~0.00~686.46~777.09~2.31~24.50~20.04~0.00~-558~0.00~41.44~29.16~~~1.24~0.0000~0.0000~0~\n~GP-A~-13.75~6.76~1.09~8.18~3.39~30.63~15.70~6.87~17.47~-23.95~3083811231~3490989083~-21.75~12.02~3083811231~~~39.36~-0.04~~CNY~0~~0.00~0\";"
	str = "v_sz002241=\"51~歌尔股份~002241~21.92~22.27~22.14~109872~40211~69642~21.91~25~21.90~961~21.89~257~21.88~748~21.87~665~21.92~86~21.93~168~21.94~556~21.95~171~21.96~85~~20250509094209~-0.35~-1.57~22.16~21.84~21.92/109872/241183171~109872~24118~0.36~27.78~~22.16~21.84~1.44~675.97~765.22~2.27~24.50~20.04~2.57~1590~21.95~40.80~28.71~~~1.24~24118.3171~0.0000~0~\n~GP-A~-15.07~5.13~1.11~8.18~3.39~30.63~15.70~5.23~15.67~-25.11~3083811231~3490989083~42.72~10.31~3083811231~~~37.23~0.18~~CNY~0~~21.85~1952\";"
	//str = "v_r_hk09660=\"100~地平线机器人-W~09660~6.860~7.000~7.010~21157200.0~0~0~6.860~0~0~0~0~0~0~0~0~0~6.860~0~0~0~0~0~0~0~0~0~21157200.0~2025/05/09\n09:43:13~-0.140~-2.00~7.030~6.730~6.860~21157200.0~144331073.000~0~35.74~~0~0~4.29~759.8070~905.5401~HORIZONROBOT-W~0.00~10.380~3.320~2.93~11.10~0~0~0~0~0~35.74~7.04~0.19~600~90.56~4.73~GP~19.70~11.51~17.26~48.48~13.58~13200293682.00~11075904412.00~35.74~0.000~6.822~71.93~HKD~1~30\";"
	info, _ := ParseTxStockData(str)
	logger.SugaredLogger.Infof("%+#v", info)
}

func TestGetRealTimeStockPriceInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	text, texttime := GetRealTimeStockPriceInfo(ctx, "sh600171")
	logger.SugaredLogger.Infof("res:%s,%s", text, texttime)

	text, texttime = GetRealTimeStockPriceInfo(ctx, "sh600438")
	logger.SugaredLogger.Infof("res:%s,%s", text, texttime)

	texttime = strings.ReplaceAll(texttime, "）", "")
	texttime = strings.ReplaceAll(texttime, "（", "")
	parts := strings.Split(texttime, " ")
	logger.SugaredLogger.Infof("parts:%+v", parts)

	//去除中文字符
	// 正则表达式匹配中文字符
	re := regexp.MustCompile(`\p{Han}+`)
	texttime = re.ReplaceAllString(texttime, "")

	logger.SugaredLogger.Infof("texttime:%s", texttime)
	location, err := time.ParseInLocation("2006-01-02 15:04:05", texttime, time.Local)
	if err != nil {
		return
	}
	logger.SugaredLogger.Infof("location:%s", location.Format("2006-01-02 15:04:05"))
}

func TestParseFullSingleStockData(t *testing.T) {
	resp, err := resty.New().R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0").
		Get(fmt.Sprintf(sinaStockUrl, time.Now().Unix(), "sh600584,sz000938,hk01810,hk00856,gb_aapl,gb_tsla,sb873721,bj430300"))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	data := GB18030ToUTF8(resp.Body())
	strs := strutil.SplitEx(data, "\n", true)
	for _, str := range strs {
		logger.SugaredLogger.Info(str)
		stockData, err := ParseFullSingleStockData(str)
		if err != nil {
			return
		}
		logger.SugaredLogger.Infof("%+#v", stockData)
	}

	result, er := ParseFullSingleStockData("var hq_str_gb_tsla = \"特斯拉,268.8472,-5.55,2025-03-04 22:52:56,-15.8028,270.9300,278.2800,268.1000,488.5400,138.8030,23618295,88214389,864751599149,2.23,120.550000,0.00,0.00,0.00,0.00,3216517037,61,0.0000,0.00,0.00,,Mar 04 09:52AM EST,284.6500,0,1,2025,6458502467.0000,0.0000,0.0000,0.0000,0.0000,284.6500\";")
	if er != nil {
		logger.SugaredLogger.Error(er.Error())
	}
	logger.SugaredLogger.Infof("%+#v", result)
}

func TestNewStockDataApi(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	datas, _ := stockDataApi.GetStockCodeRealTimeData("sz002352", "sh600859", "sh600745", "gb_tsla", "hk09660", "hk00700")
	for _, data := range *datas {
		t.Log(data)
	}
	datas, _ = stockDataApi.GetStockCodeRealTimeData("002352.SZ", "600859.SH", "600745.SH", "09660.HK", "00700.HK")
	for _, data := range *datas {
		t.Log(data)
	}
}

func TestGetStockBaseInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	stockDataApi.GetStockBaseInfo()
	//stocks := &[]StockBasic{}
	//db.Dao.Model(&StockBasic{}).Find(stocks)
	//for _, stock := range *stocks {
	//	NewStockDataApi().GetStockCodeRealTimeData(getSinaCode(stock.TsCode))
	//}

}
func getSinaCode(code string) string {
	c := strings.Split(code, ".")
	return strings.ToLower(c[1]) + c[0]
}

func TestReadFile(t *testing.T) {
	file, err := ioutil.ReadFile("../../stock_basic.json")
	if err != nil {
		t.Log(err)
		return
	}
	res := &TushareStockBasicResponse{}
	json.Unmarshal(file, res)
	db.Init("../../data/stock.db")
	//[EXCHANGE IS_HS NAME INDUSTRY LIST_STATUS ACT_NAME ID CURR_TYPE AREA LIST_DATE DELIST_DATE ACT_ENT_TYPE TS_CODE SYMBOL CN_SPELL ASSET_CLASS ACT_TYPE CREATE_TIME CREATE_BY UPDATE_TIME FULLNAME ENNAME UPDATE_BY]
	for _, item := range res.Data.Items {
		stock := &StockBasic{}
		stock.Exchange = convertor.ToString(item[0])
		stock.IsHs = convertor.ToString(item[1])
		stock.Name = convertor.ToString(item[2])
		stock.Industry = convertor.ToString(item[3])
		stock.ListStatus = convertor.ToString(item[4])
		stock.ActName = convertor.ToString(item[5])
		stock.ID = uint(item[6].(float64))
		stock.CurrType = convertor.ToString(item[7])
		stock.Area = convertor.ToString(item[8])
		stock.ListDate = convertor.ToString(item[9])
		stock.DelistDate = convertor.ToString(item[10])
		stock.ActEntType = convertor.ToString(item[11])
		stock.TsCode = convertor.ToString(item[12])
		stock.Symbol = convertor.ToString(item[13])
		stock.Cnspell = convertor.ToString(item[14])
		stock.Fullname = convertor.ToString(item[20])
		stock.Ename = convertor.ToString(item[21])
		t.Logf("%+v", stock)
		db.Dao.Model(&StockBasic{}).FirstOrCreate(stock, &StockBasic{TsCode: stock.TsCode}).Updates(stock)
	}

	//t.Log(res.Data.Fields)
}

func TestFollowedList(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	stockDataApi.GetFollowList(1)

}

func TestStockDataApi_GetIndexBasic(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	stockDataApi.GetIndexBasic()
}

func TestName(t *testing.T) {
	db.Init("../../data/stock.db")

	stockBasics := &[]StockBasic{}
	resty.New().SetProxy("").R().
		SetHeader("user", "go-stock").
		SetResult(stockBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_basic.json")

	logger.SugaredLogger.Infof("%+v", stockBasics)
	//db.Dao.Unscoped().Model(&StockBasic{}).Where("1=1").Delete(&StockBasic{})
	//err := db.Dao.CreateInBatches(stockBasics, 400).Error
	//if err != nil {
	//	t.Log(err.Error())
	//}

}
func TestGetStockMoneyData(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetStockMoneyData()
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle("今日个股资金流向Top50", res.Data.Diff))
}

func TestGetStockConceptInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetStockConceptInfo("601138.SH")
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle("601138.SH所属概念/板块信息", res.Result.Data))

}

func TestGetStockHistoryMoneyData(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetStockHistoryMoneyData("sh601138")
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle("601138.SH历史资金流向一览", res))

}

func TestGetIndustryValuation(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetIndustryValuation("消费电子")
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle(" 消费电子行业估值", res.Result.Data))
}

func Test11(t *testing.T) {
	url := "https://www.iwencai.com/customized/chart/get-robot-data"
	body := `{
		"source": "Ths_iwencai_Xuangu",
			"version": "2.0",
			"query_area": "",
			"block_list": "",
			"add_info": "{\"urp\":{\"scene\":1,\"company\":1,\"business\":1},\"contentType\":\"json\",\"searchInfo\":true}",
			"question": "立讯精密",
			"perpage": "50",
			"page": 1,
			"secondary_intent": "stock",
			"log_info": "{\"input_type\":\"typewrite\"}",
			"rsh": ""
	}`
	resp, err := resty.New().R().
		SetHeader("hexin-v", "AwdKLt3GIiwTSKag8Wve7senlrrUDN9ZNeNfTtnUJrC98S2u4dxrPkWw747q").
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		//SetHeader("Cookie", "v=AwdKLt3GIiwTSKag8Wve7senlrrUDN9ZNeNfTtnUJrC98S2u4dxrPkWw747q; other_uid=Ths_iwencai_Xuangu_dt6xi9pjkvlje9ogva7vw9v24diwgcr4;").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36").
		SetBody(body).Post(url)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return
	}
	logger.SugaredLogger.Infof("%s", resp.String())
}

func TestGetStockRZRQInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetStockRZRQInfo("SZ001389")
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle("SZ001389融资融券信息", res.Result.Data))
}

func TestGetMutualTop10Deal(t *testing.T) {
	db.Init("../../data/stock.db")
	stockDataApi := NewStockDataApi()
	res := stockDataApi.GetMutualTop10Deal("002", "2026-03-17", 1, 10)
	logger.SugaredLogger.Infof("%s", util.MarkdownTableWithTitle("SZ000001 mutual top 10 deal", res.Result.Data))
}

func getIwencaiAPIKey(t *testing.T) string {
	t.Helper()
	apiKey := os.Getenv("IWENCAI_API_KEY")
	if apiKey == "" {
		t.Skip("IWENCAI_API_KEY environment variable not set, skipping test")
	}
	return apiKey
}

func getEmAPIKey(t *testing.T) string {
	t.Helper()
	apiKey := os.Getenv("EM_API_KEY")
	if apiKey == "" {
		t.Skip("EM_API_KEY environment variable not set, skipping test")
	}
	return apiKey
}

func emTestHeaders(apiKey string) map[string]string {
	emBaseInfo, _ := json.Marshal(map[string]string{"productType": "mx"})
	return map[string]string{
		"Content-Type": "application/json",
		"em_base_info": string(emBaseInfo),
		"em_api_key":   apiKey,
	}
}

func TestEmValidateEntity(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"em_api_key":   apiKey,
		}).
		SetBody(map[string]string{"content": "贵州茅台"}).
		Post("https://ai-saas.eastmoney.com/proxy/entity/dialogTagsV2")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	t.Logf("Response: %s", resp.String())
}

func TestEmFinancialQA(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"em_api_key":   apiKey,
		}).
		SetBody(map[string]any{"question": "贵州茅台最新股价是多少"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/ask")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmFinanceDataQuery(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]any{
			"query": "贵州茅台最新价",
			"toolContext": map[string]any{
				"callId":   "call_00000001",
				"userInfo": map[string]string{"userId": "user_00000001"},
			},
		}).
		Post("https://ai-saas.eastmoney.com/proxy/b/mcp/tool/searchData")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmFinanceSearch(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]any{
			"query": "贵州茅台",
			"toolContext": map[string]any{
				"callId": "call_00000002",
			},
		}).
		Post("https://ai-saas.eastmoney.com/proxy/b/mcp/tool/searchNews")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmEarningsReview(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]string{
			"query":      "600519.SH",
			"reportDate": "2025-03-31",
		}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/write/performance/comment")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmIndustryResearch(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]string{"query": "白酒行业"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/write/industry/research")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmTrackingReport(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]string{"query": "贵州茅台"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/write/tracking/report")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmReportList(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(emTestHeaders(apiKey)).
		SetBody(map[string]string{"emCode": "600519.SH"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/write/choice/reportList")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	t.Logf("Response: %s", resp.String())
}

func TestEmComparableCompanyAnalysis(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"em_api_key":   apiKey,
		}).
		SetBody(map[string]string{"question": "东方财富"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/comparable-company-analysis")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestEmHotspotDiscovery(t *testing.T) {
	apiKey := getEmAPIKey(t)

	resp, err := resty.New().R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"em_api_key":   apiKey,
		}).
		SetBody(map[string]string{"question": "今日热点"}).
		Post("https://ai-saas.eastmoney.com/proxy/app-robo-advisor-api/assistant/hotspot-discovery")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())
	body := resp.String()
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("Response: %s", body)
}

func TestIwencaiQueryAPI(t *testing.T) {
	apiKey := getIwencaiAPIKey(t)

	query := "贵州茅台最新价"
	page := 1
	limit := 5

	reqBody := map[string]any{
		"query":        query,
		"page":         fmt.Sprintf("%d", page),
		"limit":        fmt.Sprintf("%d", limit),
		"is_cache":     "1",
		"expand_index": "true",
		"app_id":       "AIME_SKILL",
	}

	resp, err := resty.New().R().
		SetHeaders(iwencaiCommonHeaders(apiKey, "query2data", "1.0.0")).
		SetBody(reqBody).
		Post("https://openapi.iwencai.com/v1/query2data")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())

	if resp.StatusCode() == 200 {
		var result IwencaiResponse
		if err := json.Unmarshal(resp.Body(), &result); err == nil {
			t.Logf("Status Code: %d, Msg: %s, CodeCount: %d, Datas: %d",
				result.StatusCode, result.StatusMsg, result.CodeCount, len(result.Datas))
			if len(result.Datas) > 0 {
				t.Logf("第一条数据: %v", result.Datas[0])
			}
		}
	} else {
		t.Logf("Response Body: %s", resp.String())
	}
}

func TestIwencaiSearchAPI(t *testing.T) {
	apiKey := getIwencaiAPIKey(t)

	reqBody := map[string]any{
		"channels": []string{"news"},
		"app_id":   "AIME_SKILL",
		"query":    "贵州茅台",
	}

	resp, err := resty.New().R().
		SetHeaders(iwencaiCommonHeaders(apiKey, "news-search", "1.0.0")).
		SetBody(reqBody).
		Post("https://openapi.iwencai.com/v1/comprehensive/search")

	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	t.Logf("HTTP Status: %d", resp.StatusCode())

	if resp.StatusCode() == 200 {
		var result IwencaiSearchResponse
		if err := json.Unmarshal(resp.Body(), &result); err == nil {
			t.Logf("Search Results Count: %d", len(result.Data))
			if len(result.Data) > 0 {
				for i, item := range result.Data {
					if i >= 3 {
						break
					}
					t.Logf("  [%d] %s (%s)", i+1, item.Title, item.PublishDate)
				}
			}
		}
	} else {
		t.Logf("Response Body: %s", resp.String())
	}
}

func TestF10LatestFinance(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	resp, err := api.GetStockLatestFinance("300390.SZ")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if resp.Result != nil {
		t.Logf("Success: %v, Count: %d", resp.Success, resp.Result.Count)
	} else {
		t.Logf("Result is nil, Success: %v, Message: %s", resp.Success, resp.Message)
	}
	md := api.GetStockLatestFinanceToMarkdown("300390.SZ")
	t.Logf("Markdown :\n%s", func() string {
		return md
	}())
}

func TestHKF10LatestFinance(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	// 测试不同代码格式归一化
	for _, code := range []string{"00700.HK", "00700", "hk00700", "HK00700", "700.HK"} {
		t.Logf("normalizeHKF10Code(%s) = %s", code, normalizeHKF10Code(code))
	}
	resp, err := api.GetHKStockLatestFinance("00700.HK")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if resp.Data == nil {
		t.Fatalf("Data is nil, msg: %s", resp.Msg)
	}
	t.Logf("Status: %d, msg: %s, abgq rows: %d, an rows: %d",
		resp.Status, resp.Msg, len(resp.Data.Abgq), len(resp.Data.An))
	md := api.GetHKStockLatestFinanceToMarkdown("00700.HK")
	t.Logf("Latest Markdown:\n%s", md)
	mdAnnual := api.GetHKStockAnnualFinanceToMarkdown("00700.HK")
	t.Logf("Annual Markdown:\n%s", mdAnnual)
}

func TestHKCodeForRoute(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		// 港股代码（应为 true）
		{"00700", true},
		{"00700.HK", true},
		{"hk00700", true},
		{"HK00700", true},
		{"700.HK", true},
		{"00700.hk", true},
		{"09988", true},
		// A 股代码（应为 false）
		{"600519", false},
		{"000001", false},
		{"600519.SH", false},
		{"000001.SZ", false},
		{"300390", false},
		// 美股/指数（应为 false）
		{"AAPL.US", false},
		{"100.DJIA", false},
		{"930599.CSI", false},
		// 边界
		{"", false},
		{"  ", false},
	}
	for _, c := range cases {
		got := IsHKCodeForRoute(c.code)
		status := "✓"
		if got != c.want {
			status = "✗"
		}
		t.Logf("%s IsHKCodeForRoute(%q) = %v, want %v", status, c.code, got, c.want)
		if got != c.want {
			t.Errorf("IsHKCodeForRoute(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestF10QtrMainFinance(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockQtrMainFinanceToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10OrgPredict(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockOrgPredictToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10PredictSummary(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockPredictSummaryToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10ValuationPercentile(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockValuationPercentileToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10MarginTrading(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockMarginTradingToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10BlockTrade(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockBlockTradeToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10HolderTrend(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockHolderTrendToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10Billboard(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockBillboardToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10OperationDeptTrade(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	md := api.GetStockOperationDeptTradeToMarkdown("600519")
	t.Logf("Markdown:\n%s", md)
}

func TestF10OrgBasicInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	api := NewStockDataApi()
	for _, code := range []string{"002008", "600519", "000001.SZ"} {
		resp, err := api.GetStockOrgBasicInfo(code)
		if err != nil {
			t.Fatalf("[%s] 请求失败: %v", code, err)
		}
		if resp.Result == nil || resp.Result.Count == 0 {
			t.Fatalf("[%s] 无数据: success=%v msg=%s", code, resp.Success, resp.Message)
		}
		t.Logf("[%s] Success=%v Count=%d", code, resp.Success, resp.Result.Count)
	}
	md := api.GetStockOrgBasicInfoToMarkdown("002008")
	t.Logf("Markdown:\n%s", md)
}
