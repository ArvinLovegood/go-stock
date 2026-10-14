package main

import (
	"context"
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/go-resty/resty/v2"
)

// @Author spark
// @Date 2025/2/24 9:35
// @Desc
// -----------------------------------------------------------------------------------
func TestIsHKTradingTime(t *testing.T) {
	f := IsHKTradingTime(time.Now())
	t.Log(f)
}

func TestIsUSTradingTime(t *testing.T) {

	date := time.Now()
	hour, minute, _ := date.Clock()
	logger.SugaredLogger.Infof("当前时间: %d:%d", hour, minute)

	t.Log(IsUSTradingTime(time.Now()))
}

func TestCheckStockBaseInfo(t *testing.T) {
	db.Init("./data/stock.db")
	NewApp().CheckStockBaseInfo(context.Background())
}

func TestJson(t *testing.T) {
	db.Init("./data/stock.db")

	jsonStr := "{\n\t\t\"id\" : 3334,\n\t\t\"created_at\" : \"2025-02-28 16:49:31.8342514+08:00\",\n\t\t\"updated_at\" : \"2025-02-28 16:49:31.8342514+08:00\",\n\t\t\"deleted_at\" : null,\n\t\t\"code\" : \"PUK.US\",\n\t\t\"name\" : \"英国保诚集团\",\n\t\t\"full_name\" : \"\",\n\t\t\"e_name\" : \"\",\n\t\t\"exchange\" : \"NASDAQ\",\n\t\t\"type\" : \"stock\",\n\t\t\"is_del\" : 0,\n\t\t\"bk_name\" : null,\n\t\t\"bk_code\" : null\n\t}"

	v := &models.StockInfoUS{}
	json.Unmarshal([]byte(jsonStr), v)
	logger.SugaredLogger.Infof("v:%+v", v)

	db.Dao.Model(v).Updates(v)

}

func TestUpdateCheck(t *testing.T) {
	releaseVersion := &models.GitHubReleaseVersion{}
	_, err := resty.New().R().
		SetResult(releaseVersion).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		Get("https://api.github.com/repos/ArvinLovegood/go-stock/releases/latest")
	//  https://api.github.com/repos/OWNER/REPO/releases/latest
	if err != nil {
		logger.SugaredLogger.Errorf("get github release version error:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("releaseVersion:%+v", releaseVersion)
}

func TestGetScreenResolution(t *testing.T) {
	x, y, w, h, err := getScreenResolution()
	if err != nil {
		logger.SugaredLogger.Errorf("get screen resolution error:%s", err.Error())
		return
	}
	logger.SugaredLogger.Infof("x:%d,y:%d,w:%d,h:%d", x, y, w, h)

}

func TestCheckUpdate(t *testing.T) {
	db.Init("./data/stock.db")
	NewApp().CheckUpdate(1)
}

func TestGetAiRecommendStocksList(t *testing.T) {
	db.Init("./data/stock.db")

	str := "{\"startDate\": \"2026-03-20 00:00:00\", \"endDate\": \"2026-03-27 23:59:59\", \"page\": 1, \"pageSize\": 5000}"
	query := &models.AiRecommendStocksQuery{}
	json.Unmarshal([]byte(str), query)

	pageData, err := data.NewAiRecommendStocksService().GetAiRecommendStocksList(query)
	logger.SugaredLogger.Infof("pageData:%+v", pageData.List)
	if err != nil {
		pageData = &models.AiRecommendStocksPageData{}
	}
	var dataExport []models.AiRecommendStocksMdExport
	for _, v := range pageData.List {
		dataExport = append(dataExport, v.ToMdExportStruct())
	}
	content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)
	logger.SugaredLogger.Infof("content:%s", content)
}

func TestSummaryStockNews(t *testing.T) {
	db.Init("./data/stock.db")
	question := "分析今日的市场行情走势是否和券商的观点一致"
	app := NewApp()
	msgs := data.NewDeepSeekOpenAi(app.ctx, 0).NewSummaryStockNewsStreamWithTools(question, nil, app.AiTools, true, nil, nil)

	content := &strings.Builder{}
	for msg := range msgs {
		logger.SugaredLogger.Infof("msg:%+v", msg)
		content.WriteString(msg["content"].(string))
	}
	logger.SugaredLogger.Infof("content:%s", content.String())
}

func TestCalculateNextRunTime(t *testing.T) {
	db.Init("./data/stock.db")
	t.Log(NewApp().CalculateNextRunTime("0 0 0 * * ?"))
}

func TestFetchAiModels(t *testing.T) {
	app := NewApp()
	models := app.FetchAiModels("https://ark.cn-beijing.volces.com/api/v3", "", "")
	t.Log(models)

}
func TestGetLatestTradingDay(t *testing.T) {
	app := NewApp()
	date := app.GetLatestTradingDay()
	t.Log(date)
}

// TestSponsorInfoRace 复现 crash.log 的竞态场景：
// cron 定时任务（CheckUpdate）并发写 SponsorInfo，同时多个前端页面通过
// Wails 绑定 GetSponsorInfo 并发读。修复前为无锁并发读写，触发
// runtime "concurrent map writes" fatal；修复后应通过 -race 检测。
// 运行：go test -race -run TestSponsorInfoRace -count=1 .
func TestSponsorInfoRace(t *testing.T) {
	app := &App{}
	raw := []byte(`{"vipLevel":2,"expireDate":"2027-12-31","winDownUrl":"https://example.com/go-stock.exe","macDownUrl":"https://example.com/go-stock.zip","linuxDownUrl":"https://example.com/go-stock"}`)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// 写入方：模拟 cron 触发 CheckUpdate -> setSponsorInfo（整体替换引用）
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			if err := app.setSponsorInfo(raw); err != nil {
				t.Errorf("setSponsorInfo error: %v", err)
				return
			}
		}
		close(done)
	}()

	// 读取方：模拟 4 个前端页面并发调用 GetSponsorInfo（Wails 序列化发生在锁外）
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					info := app.GetSponsorInfo()
					_ = convertor.ToString(info["vipLevel"])
					_ = info["winDownUrl"]
				}
			}
		}()
	}

	wg.Wait()
}
