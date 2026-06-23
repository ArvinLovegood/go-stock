package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/machineid"
	"go-stock/backend/models"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/inconshreveable/go-update"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx                context.Context
	cache              *freecache.Cache
	cron               *cron.Cron
	cronEntrys         map[string]cron.EntryID
	cronEntrysMu       sync.Mutex
	AiTools            []data.Tool
	SponsorInfo        map[string]any
	VipLevel           int64
	summaryMu          sync.Mutex
	summaryCancel      context.CancelFunc
	agentMu            sync.Mutex
	agentCancel        context.CancelFunc
	stockAlertMu       sync.Mutex
	stockAlertLastSent map[string]time.Time
	priceAtAlertReset  map[string]float64
}

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()
	var tools []data.Tool
	tools = data.Tools(tools)
	return &App{
		cache:              cache,
		cron:               c,
		cronEntrys:         make(map[string]cron.EntryID),
		AiTools:            tools,
		stockAlertLastSent: make(map[string]time.Time),
		priceAtAlertReset:  make(map[string]float64),
	}
}

func (a *App) setCronEntry(key string, id cron.EntryID) {
	a.cronEntrysMu.Lock()
	a.cronEntrys[key] = id
	a.cronEntrysMu.Unlock()
}

func (a *App) getCronEntry(key string) (cron.EntryID, bool) {
	a.cronEntrysMu.Lock()
	id, exists := a.cronEntrys[key]
	a.cronEntrysMu.Unlock()
	return id, exists
}

func (a *App) removeCronEntry(key string) {
	a.cronEntrysMu.Lock()
	delete(a.cronEntrys, key)
	a.cronEntrysMu.Unlock()
}

func (a *App) GetSponsorInfo() map[string]any {
	return a.SponsorInfo
}

// GetEffectiveSponsorVip 从本地配置解密赞助信息并判断当前是否在 VIP 有效期内（与 ai-assistant-web / data.EffectiveSponsorVipLevel 一致）。
func (a *App) GetEffectiveSponsorVip() map[string]any {
	level, active := data.EffectiveSponsorVipLevel()
	return map[string]any{
		"vipLevel": level,
		"active":   active,
	}
}

func (a *App) GetMachineId() string {
	return machineid.GetMachineId()
}

func (a *App) CheckDeviceBinding(token string, apiBase string) map[string]any {
	uuid := machineid.GetMachineId()
	result := map[string]any{
		"bound":       false,
		"deviceCount": 0,
		"maxDevices":  5,
	}

	if token == "" || apiBase == "" {
		return result
	}

	url := fmt.Sprintf("%s/user/device-check?uuid=%s", apiBase, uuid)
	resp, err := data.SharedHTTPClient.R().
		SetHeader("Authorization", "Bearer "+token).
		Get(url)
	if err != nil {
		return result
	}

	var respData struct {
		Code int `json:"code"`
		Data struct {
			Bound       bool `json:"bound"`
			DeviceCount int  `json:"deviceCount"`
			MaxDevices  int  `json:"maxDevices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &respData); err != nil {
		return result
	}
	if respData.Code != 0 {
		return result
	}

	result["bound"] = respData.Data.Bound
	result["deviceCount"] = respData.Data.DeviceCount
	result["maxDevices"] = respData.Data.MaxDevices
	return result
}

func (a *App) QuitApp() {
	if a.ctx != nil {
		if a.cron != nil {
			a.cron.Stop()
		}
		runtime.Quit(a.ctx)
	}
}
func (a *App) CheckSponsorCode(sponsorCode string) map[string]any {
	sponsorCode = strutil.Trim(sponsorCode)
	if sponsorCode != "" {
		encrypted, err := hex.DecodeString(sponsorCode)
		if err != nil {
			return map[string]any{
				"code": 0,
				"msg":  "赞助码格式错误,请输入正确的赞助码!",
			}
		}
		key, err := hex.DecodeString(BuildKey)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return map[string]any{
				"code": 0,
				"msg":  "版本错误，不支持赞助码!",
			}
		}
		decrypt := cryptor.AesEcbDecrypt(encrypted, key)
		if decrypt == nil || len(decrypt) == 0 {
			return map[string]any{
				"code": 0,
				"msg":  "赞助码错误，请输入正确的赞助码!",
			}
		}

		// 校验通过后，将赞助码持久化到 Settings 中
		config := data.GetSettingConfig()
		// 只在赞助码变更时写库，避免无谓更新
		if config.SponsorCode != sponsorCode {
			config.SponsorCode = sponsorCode
			data.UpdateConfig(config)
		}

		return map[string]any{
			"code": 1,
			"msg":  "赞助码校验成功，感谢您的支持!",
		}
	} else {
		return map[string]any{"code": 0, "message": "赞助码不能为空,请输入正确的赞助码!"}
	}
}

func (a *App) CheckUpdate(flag int) {
	sponsorCode := strutil.Trim(a.GetConfig().SponsorCode)
	if sponsorCode != "" {
		encrypted, err := hex.DecodeString(sponsorCode)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return
		}
		key, err := hex.DecodeString(BuildKey)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return
		}
		decrypt := string(cryptor.AesEcbDecrypt(encrypted, key))
		err = json.Unmarshal([]byte(decrypt), &a.SponsorInfo)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return
		}
	}

	updateChannel := a.GetConfig().UpdateChannel
	if updateChannel == "" {
		updateChannel = "release"
	}

	githubApiHeaders := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}

	releaseVersion := &models.GitHubReleaseVersion{}
	if updateChannel == "release" {
		resp, err := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(releaseVersion).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/releases/latest")
		if err != nil {
			logger.SugaredLogger.Errorf("get github release version error:%s", err.Error())
			return
		}
		if resp.StatusCode() != 200 {
			logger.SugaredLogger.Errorf("get github release version failed, status:%d", resp.StatusCode())
			return
		}
	} else {
		var releases []models.GitHubReleaseVersion
		resp, err := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(&releases).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/releases")
		if err != nil {
			logger.SugaredLogger.Errorf("get github releases error:%s", err.Error())
			return
		}
		if resp.StatusCode() != 200 {
			logger.SugaredLogger.Errorf("get github releases failed, status:%d", resp.StatusCode())
			return
		}
		if len(releases) == 0 {
			logger.SugaredLogger.Errorf("no releases found")
			return
		}
		if updateChannel == "pre" {
			for _, r := range releases {
				if !r.Draft {
					releaseVersion = &r
					break
				}
			}
			if releaseVersion.TagName == "" {
				releaseVersion = &releases[0]
			}
		} else {
			releaseVersion = &releases[0]
		}
	}

	if _, vipLevel, ok := a.isVip(sponsorCode, "", releaseVersion); ok {
		level, _ := convertor.ToInt(vipLevel)
		a.VipLevel = level
		if level >= 2 {
			go a.syncNews()
		}
	}

	if releaseVersion.TagName != Version {
		tag := &models.Tag{}
		tagResp, tagErr := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(tag).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/git/ref/tags/" + releaseVersion.TagName)
		if tagErr == nil && tagResp.StatusCode() == 200 && tag.Object.Url != "" {
			releaseVersion.Tag = *tag
			commit := &models.Commit{}
			commitResp, commitErr := data.SharedHTTPClient.R().
				SetHeaders(githubApiHeaders).
				SetResult(commit).
				Get(tag.Object.Url)
			if commitErr == nil && commitResp.StatusCode() == 200 {
				releaseVersion.Commit = *commit
			}
		}

		commitMessage := releaseVersion.Body
		if releaseVersion.Commit.Message != "" {
			commitMessage = releaseVersion.Commit.Message
		}

		downloadUrl := ""
		assetName := ""
		if IsWindows() {
			if IsArm64() {
				assetName = "go-stock-windows-arm64.exe"
			} else {
				assetName = "go-stock-windows-amd64.exe"
			}
		} else if IsMacOS() {
			assetName = "go-stock-darwin-universal"
		} else if IsLinux() {
			assetName = "go-stock-linux-amd64"
		}

		for _, asset := range releaseVersion.Assets {
			if asset.Name == assetName {
				downloadUrl = asset.BrowserDownloadUrl
				break
			}
		}

		if downloadUrl == "" {
			downloadUrl = fmt.Sprintf("https://github.com/ArvinLovegood/go-stock/releases/download/%s/%s", releaseVersion.TagName, assetName)
		}

		originalDownloadUrl := downloadUrl
		downloadUrl, _, _ = a.isVip(sponsorCode, downloadUrl, releaseVersion)
		mirrorDownloadUrl := "https://gh.927223.xyz/" + originalDownloadUrl
		manualDownloadTip := fmt.Sprintf("\n手动下载链接(加速镜像): %s\n手动下载链接(原始地址): %s\n下载后请替换当前程序文件即可完成更新。", mirrorDownloadUrl, originalDownloadUrl)

		go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
			"time":    "发现新版本：" + releaseVersion.TagName,
			"isRed":   true,
			"source":  "go-stock",
			"content": commitMessage + "\n正在下载新版本，请耐心等待...",
		})

		tmpFile, err := os.CreateTemp("", "go-stock-update-*.tmp")
		if err != nil {
			logger.SugaredLogger.Errorf("create temp file error: %s", err.Error())
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本下载失败(无法创建临时文件)。" + manualDownloadTip,
			})
			return
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		downloadClient := data.CreateDownloadClient()

		downloadUrls := []string{mirrorDownloadUrl, downloadUrl}
		var downloadSuccess bool
		for _, url := range downloadUrls {
			_, err = downloadClient.R().
				SetHeader("User-Agent", "go-stock-updater").
				SetOutput(tmpPath).
				Get(url)
			if err != nil {
				logger.SugaredLogger.Warnf("download from %s error: %s, trying next...", url, err.Error())
				continue
			}
			fileInfo, statErr := os.Stat(tmpPath)
			if statErr != nil || fileInfo.Size() < 1024*500 {
				logger.SugaredLogger.Warnf("download from %s file size invalid, trying next...", url)
				continue
			}
			downloadSuccess = true
			break
		}

		if !downloadSuccess {
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本自动下载失败，请手动下载更新。" + manualDownloadTip,
			})
			return
		}

		body, err := os.ReadFile(tmpPath)
		if err != nil {
			logger.SugaredLogger.Errorf("read downloaded file error: %s", err.Error())
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本下载失败(无法读取临时文件)。" + manualDownloadTip,
			})
			return
		}

		err = update.Apply(bytes.NewReader(body), update.Options{})
		if err != nil {
			logger.SugaredLogger.Error("更新失败: ", err.Error())
			if !IsRunningAsAdmin() {
				go runtime.EventsEmit(a.ctx, "updateNeedAdmin", map[string]any{
					"version": releaseVersion.TagName,
					"message": commitMessage,
				})
			} else {
				go runtime.EventsEmit(a.ctx, "updateVersion", releaseVersion)
			}
			return
		} else {
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": "版本更新完成,下次重启软件生效.",
			})
		}
	} else {
		if flag == 1 {
			go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
				"time":    "当前版本：" + Version,
				"isRed":   true,
				"source":  "go-stock",
				"content": "当前版本无更新",
			})
		}

	}
}

func (a *App) isVip(sponsorCode string, downloadUrl string, releaseVersion *models.GitHubReleaseVersion) (string, string, bool) {
	isVip := false
	vipLevel := "0"
	sponsorCode = strutil.Trim(a.GetConfig().SponsorCode)
	if sponsorCode != "" {
		encrypted, err := hex.DecodeString(sponsorCode)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return "", "0", false
		}
		key, err := hex.DecodeString(BuildKey)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return "", "0", false
		}
		decrypt := string(cryptor.AesEcbDecrypt(encrypted, key))
		err = json.Unmarshal([]byte(decrypt), &a.SponsorInfo)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return "", "0", false
		}
		vipLevel = a.SponsorInfo["vipLevel"].(string)
		vipStartTime, err := time.ParseInLocation("2006-01-02 15:04:05", a.SponsorInfo["vipStartTime"].(string), time.Local)
		vipEndTime, err := time.ParseInLocation("2006-01-02 15:04:05", a.SponsorInfo["vipEndTime"].(string), time.Local)
		vipAuthTime, err := time.ParseInLocation("2006-01-02 15:04:05", a.SponsorInfo["vipAuthTime"].(string), time.Local)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return "", vipLevel, false
		}

		if time.Now().After(vipAuthTime) && time.Now().After(vipStartTime) && time.Now().Before(vipEndTime) {
			isVip = true
		}

		if IsWindows() {
			winAssetName := "go-stock-windows-amd64.exe"
			if IsArm64() {
				winAssetName = "go-stock-windows-arm64.exe"
			}
			if isVip {
				if a.SponsorInfo["winDownUrl"] == nil {
					downloadUrl = fmt.Sprintf("https://gh.927223.xyz/https://github.com/ArvinLovegood/go-stock/releases/download/%s/%s", releaseVersion.TagName, winAssetName)
				} else {
					downloadUrl = a.SponsorInfo["winDownUrl"].(string)
				}
			} else {
				downloadUrl = fmt.Sprintf("https://github.com/ArvinLovegood/go-stock/releases/download/%s/%s", releaseVersion.TagName, winAssetName)
			}
		}
		if IsMacOS() {
			if isVip {
				if a.SponsorInfo["macDownUrl"] == nil {
					downloadUrl = fmt.Sprintf("https://gh.927223.xyz/https://github.com/ArvinLovegood/go-stock/releases/download/%s/go-stock-darwin-universal", releaseVersion.TagName)
				} else {
					downloadUrl = a.SponsorInfo["macDownUrl"].(string)
				}
			} else {
				downloadUrl = fmt.Sprintf("https://github.com/ArvinLovegood/go-stock/releases/download/%s/go-stock-darwin-universal", releaseVersion.TagName)
			}
		}
		if IsLinux() {
			if isVip {
				if a.SponsorInfo["linuxDownUrl"] == nil {
					downloadUrl = fmt.Sprintf("https://gh.927223.xyz/https://github.com/ArvinLovegood/go-stock/releases/download/%s/go-stock-linux-amd64", releaseVersion.TagName)
				} else {
					downloadUrl = a.SponsorInfo["linuxDownUrl"].(string)
				}
			} else {
				downloadUrl = fmt.Sprintf("https://github.com/ArvinLovegood/go-stock/releases/download/%s/go-stock-linux-amd64", releaseVersion.TagName)
			}
		}

	}
	return downloadUrl, vipLevel, isVip
}

func (a *App) syncNews() {
	defer PanicHandler()
	client := data.SharedHTTPClient
	url := fmt.Sprintf("http://go-stock.sparkmemory.top:16666/FinancialNews/json?since=%d", time.Now().Add(-24*time.Hour).Unix())
	//logger.SugaredLogger.Infof("syncNews:%s", url)
	resp, err := client.R().SetDoNotParseResponse(true).Get(url)
	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Errorf("syncNews error:%s", err.Error())
	}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		//line := scanner.Text()
		//logger.SugaredLogger.Infof("Received data: %s", line)
		news := &models.NtfyNews{}
		err := json.Unmarshal(scanner.Bytes(), news)
		if err != nil {
			return
		}
		dataTime := time.UnixMilli(int64(news.Time * 1000))

		if slice.ContainAny(news.Tags, []string{"外媒资讯", "财联社电报", "新浪财经", "外媒简讯", "外媒"}) {
			isRed := false
			if slice.Contain(news.Tags, "rotating_light") {
				isRed = true
			}
			telegraph := &models.Telegraph{
				Title:           news.Title,
				Content:         news.Message,
				DataTime:        &dataTime,
				IsRed:           isRed,
				Time:            dataTime.Format("15:04:05"),
				Source:          GetSource(news.Tags),
				SentimentResult: data.AnalyzeSentiment(news.Message).Description,
			}
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt == 0 {
				db.Dao.Model(telegraph).Create(&telegraph)
				//计算时间差如果<5分钟则推送
				if time.Now().Sub(dataTime) < 5*time.Minute {
					a.NewsPush(&[]models.Telegraph{*telegraph})
				}
				tags := slice.Filter(news.Tags, func(index int, item string) bool {
					return !(item == "rotating_light" || item == "loudspeaker")
				})
				for _, subject := range tags {
					tag := &models.Tags{
						Name: subject,
						Type: "subject",
					}
					db.Dao.Model(tag).Where("name=? and type=?", subject, "subject").FirstOrCreate(&tag)
					db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
						TelegraphId: telegraph.ID,
						TagId:       tag.ID,
					})
				}
			}
		}
	}
}

func GetSource(tags []string) string {
	if slice.ContainAny(tags, []string{"外媒简讯", "外媒资讯", "外媒"}) {
		return "外媒"
	}
	if slices.Contains(tags, "财联社电报") {
		return "财联社电报"
	}
	if slices.Contains(tags, "新浪财经") {
		return "新浪财经"
	}
	return ""
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		// 增加延迟确保前端已准备好接收事件
		go func() {
			time.Sleep(2 * time.Second)
			runtime.EventsEmit(a.ctx, "loadingMsg", "done")
		}()
	}()

	//if stocksBin != nil && len(stocksBin) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查A股基础信息...")
	//	go initStockData(a.ctx)
	//}
	//
	//if stocksBinHK != nil && len(stocksBinHK) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查港股基础信息...")
	//	go initStockDataHK(a.ctx)
	//}
	//
	//if stocksBinUS != nil && len(stocksBinUS) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查美股基础信息...")
	//	go initStockDataUS(a.ctx)
	//}
	updateBasicInfo()

	// Add your action here
	//定时更新数据
	config := data.GetSettingConfig()
	go func() {
		go data.NewMarketNewsApi().TelegraphList(30)
		go data.NewMarketNewsApi().GetSinaNews(30)
		go data.NewMarketNewsApi().TradingViewNews()

		interval := config.RefreshInterval
		if interval <= 0 {
			interval = 1
		}
		//ticker := time.NewTicker(time.Second * time.Duration(interval))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorStockPrices(a)
		//}
		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
			MonitorStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorStockPrices", id)
		}
		entryID, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			//news := data.NewMarketNewsApi().GetNewTelegraph(30)
			news := data.NewMarketNewsApi().TelegraphList(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newTelegraph", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("GetNewTelegraph", entryID)
		}

		entryIDSina, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().GetSinaNews(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newSinaNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("newSinaNews", entryIDSina)
		}

		entryIDTradingViewNews, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().TradingViewNews()
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "tradingViewNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("tradingViewNews", entryIDTradingViewNews)
		}
	}()

	//刷新基金净值信息
	go func() {
		//ticker := time.NewTicker(time.Second * time.Duration(60))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorFundPrices(a)
		//}
		if config.EnableFund {
			id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
				MonitorFundPrices(a)
			})
			if err != nil {
				logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
			} else {
				a.setCronEntry("MonitorFundPrices", id)
			}
		}

		// AI 推荐股票价格监控定时器
		idAiStock, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorAiRecommendStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorAiRecommendStockPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorAiRecommendStockPrices", idAiStock)
		}

		// 自选股成本价监控定时器
		idCostPrice, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorFollowedStockCostPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorFollowedStockCostPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorFollowedStockCostPrices", idCostPrice)
		}

	}()

	if config.EnableNews {
		//go func() {
		//	ticker := time.NewTicker(time.Second * time.Duration(60))
		//	defer ticker.Stop()
		//	for range ticker.C {
		//		telegraph := refreshTelegraphList()
		//		if telegraph != nil {
		//			go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
		//		}
		//	}
		//
		//}()

		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			telegraph := refreshTelegraphList()
			if telegraph != nil {
				go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("refreshTelegraphList", id)
		}

		go runtime.EventsEmit(a.ctx, "telegraph", refreshTelegraphList())
	}
	go MonitorStockPrices(a)
	if config.EnableFund {
		go MonitorFundPrices(a)
		go data.NewFundApi().AllFund()
	}
	// AI 推荐股票价格监控
	go MonitorAiRecommendStockPrices(a)
	// 自选股成本价监控
	go MonitorFollowedStockCostPrices(a)
	// 市场统计数据采集（交易日每5分钟）
	go func() {
		a.FetchAndSaveMarketStatistic()
		idMarketStat, err := a.cron.AddFunc("0 */5 9-15 * * 1-5", func() {
			a.FetchAndSaveMarketStatistic()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc FetchAndSaveMarketStatistic error:%s", err.Error())
		} else {
			a.setCronEntry("FetchAndSaveMarketStatistic", idMarketStat)
		}
	}()
	// 板块资金流向数据采集（交易日每60秒）
	go func() {
		data.NewBKFundFlowApi().FetchAndSave()
		idBKFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewBKFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc BKFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("BKFundFlowFetchAndSave", idBKFundFlow)
		}
	}()
	// 概念资金流向数据采集（交易日每60秒）
	go func() {
		data.NewConceptFundFlowApi().FetchAndSave()
		idConceptFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewConceptFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc ConceptFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("ConceptFundFlowFetchAndSave", idConceptFundFlow)
		}
	}()
	//检查新版本
	go func() {
		a.CheckUpdate(0)
		go a.CheckStockBaseInfo(a.ctx)
		go syncAllStockInfo(a.ctx)

		a.cron.AddFunc("0 0 2 * * *", func() {
			logger.SugaredLogger.Errorf("Checking for updates...")
			a.CheckStockBaseInfo(a.ctx)
		})
		a.cron.AddFunc("30 05 8,12,20 * * *", func() {
			logger.SugaredLogger.Errorf("Checking for updates...")
			a.CheckUpdate(0)
		})
		a.cron.AddFunc("30 05 8,12,20 * * *", func() {
			syncAllStockInfo(a.ctx)
		})
	}()

	//检查谷歌浏览器
	//go func() {
	//	f := checkChromeOnWindows()
	//	if !f {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "谷歌浏览器未安装,ai分析功能可能无法使用")
	//	}
	//}()

	//检查Edge浏览器
	//go func() {
	//	path, e := checkEdgeOnWindows()
	//	if !e {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "Edge浏览器未安装,ai分析功能可能无法使用")
	//	} else {
	//		logger.SugaredLogger.Infof("Edge浏览器已安装，路径为: %s", path)
	//	}
	//}()
	followList := data.NewStockDataApi().GetFollowList(0)
	for _, follow := range *followList {
		if follow.Cron == nil || *follow.Cron == "" {
			continue
		}
		entryID, err := a.cron.AddFunc(*follow.Cron, a.AddCronTask(follow))
		if err != nil {
			logger.SugaredLogger.Errorf("添加自动分析任务失败:%s cron=%s entryID:%v", follow.Name, *follow.Cron, entryID)
			continue
		}
		a.setCronEntry(follow.StockCode, entryID)
	}
	//logger.SugaredLogger.Infof("domReady-cronEntrys:%+v", a.cronEntrys)

}

func syncAllStockInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	db.Dao.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{})
	for page := 1; page < 3; page++ {
		res := data.NewStockDataApi().GetAllStocks(page, 3000, "", models.TechnicalIndicators{})
		var datas []models.AllStockInfo
		for _, data := range (*res).Result.Data {
			datas = append(datas, data.ToAllStockInfo())
		}
		err := db.Dao.CreateInBatches(&datas, 1000).Error
		if err != nil {
			logger.SugaredLogger.Errorf("db.Dao.CreateInBatches error:%s", err.Error())
		}
	}
}
func (a *App) CheckStockBaseInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	stockBasics := &[]data.StockBasic{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_basic.json")

	db.Dao.Unscoped().Model(&data.StockBasic{}).Where("1=1").Delete(&data.StockBasic{})
	err := db.Dao.CreateInBatches(stockBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockBasic股票基础信息失败:%s", err.Error())
	}

	//count := int64(0)
	//db.Dao.Model(&data.StockBasic{}).Count(&count)
	//if count == int64(len(*stockBasics)) {
	//	return
	//}
	//for _, stock := range *stockBasics {
	//	stockInfo := &data.StockBasic{
	//		TsCode: stock.TsCode,
	//		Name:   stock.Name,
	//		Symbol: stock.Symbol,
	//		BKCode: stock.BKCode,
	//		BKName: stock.BKName,
	//	}
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&data.StockBasic{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Updates(stockInfo)
	//	}
	//}

	stockHKBasics := &[]models.StockInfoHK{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockHKBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_hk.json")

	db.Dao.Unscoped().Model(&models.StockInfoHK{}).Where("1=1").Delete(&models.StockInfoHK{})
	err = db.Dao.CreateInBatches(stockHKBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoHK股票基础信息失败:%s", err.Error())
	}

	//for _, stock := range *stockHKBasics {
	//	stockInfo := &models.StockInfoHK{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}
	stockUSBasics := &[]models.StockInfoUS{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockUSBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_us.json")

	db.Dao.Unscoped().Model(&models.StockInfoUS{}).Where("1=1").Delete(&models.StockInfoUS{})
	err = db.Dao.CreateInBatches(stockUSBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoUS股票基础信息失败:%s", err.Error())
	}
	//for _, stock := range *stockUSBasics {
	//	stockInfo := &models.StockInfoUS{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}

}
func (a *App) NewsPush(news *[]models.Telegraph) {

	follows := data.NewStockDataApi().GetFollowList(0)
	stockNames := slice.Map(*follows, func(index int, item data.FollowedStock) string {
		return item.Name
	})

	for _, telegraph := range *news {
		if a.GetConfig().EnableOnlyPushRedNews {
			if telegraph.IsRed || strutil.ContainsAny(telegraph.Content, stockNames) {
				go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
			}
		} else {
			go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
		}
		//go data.NewAlertWindowsApi("go-stock", telegraph.Source+" "+telegraph.Time, telegraph.Content, string(icon)).SendNotification()
		//}
	}
}

func (a *App) AddCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(a.ctx, "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		ai := data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId)
		thinking := data.GetSettingConfig().GetAIConfigThinking(follow.AiConfigId)
		msgs := ai.NewChatStream(follow.Name, follow.StockCode, "", nil, a.AiTools, thinking)
		var res strings.Builder

		chatId := ""
		question := ""
		for msg := range msgs {
			if v, ok := msg["extraContent"].(string); ok && v != "" {
				res.WriteString(v + "\n")
			}
			if v, ok := msg["content"].(string); ok && v != "" {
				res.WriteString(v)
			}
			if v, ok := msg["chatId"].(string); ok {
				chatId = v
			}
			if v, ok := msg["question"].(string); ok {
				question = v
			}
		}

		data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), chatId, question)
		go runtime.EventsEmit(a.ctx, "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)

	}
}

func refreshTelegraphList() *[]string {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	response, err := data.SharedHTTPClient.R().
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
				telegraph = append(telegraph, content)
			}
		}
	}
	return &telegraph
}

// isTradingDay 判断是否是交易日
var tradingDayCache = freecache.NewCache(64 * 1024)

func isTradingDay(date time.Time) bool {
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte(dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type holidayResp struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool   `json:"holiday"`
			Name    string `json:"name"`
		} `json:"holiday"`
	}
	var result holidayResp
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date))
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if result.Code == 0 && result.Holiday.Holiday {
		return true, true
	}
	return false, true
}

func preCacheTradingDays() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiTimezone
	}
	now := time.Now().In(loc)
	go func() {
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			isTradingDay(d)
		}
	}()
	go func() {
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			isHKTradingDay(d)
		}
	}()
	go func() {
		est, _ := time.LoadLocation("America/New_York")
		for i := -7; i <= 7; i++ {
			d := now.AddDate(0, 0, i)
			if est != nil {
				d = d.In(est)
			}
			isUSTradingDay(d)
		}
	}()
}

// isTradingTime 判断是否是交易时间
func isTradingTime(date time.Time) bool {
	if !isTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	// 判断是否在9:15到11:30之间
	if (hour == 9 && minute >= 15) || (hour == 10) || (hour == 11 && minute <= 30) {
		return true
	}

	// 判断是否在13:00到15:00之间
	if (hour == 13) || (hour == 14) || (hour == 15 && minute <= 0) {
		return true
	}

	return false
}

// IsHKTradingTime 判断当前时间是否在港股交易时间内
func IsHKTradingTime(date time.Time) bool {
	if !isHKTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	if (hour == 9 && minute >= 0) || (hour == 9 && minute <= 30) {
		return true
	}

	if (hour == 9 && minute > 30) || (hour >= 10 && hour < 12) || (hour == 12 && minute == 0) {
		return true
	}

	if (hour == 13 && minute >= 0) || (hour >= 14 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	if (hour == 16 && minute >= 0) || (hour == 16 && minute <= 10) {
		return true
	}
	return false
}

func isHKTradingDay(date time.Time) bool {
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte("hk:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHKHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHKHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type klineResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var result klineResp
	dateClean := strings.ReplaceAll(date, "-", "")
	apiURL := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=100.HSI&fields1=f1&fields2=f51&klt=101&fqt=0&beg=%s&end=%s", dateClean, dateClean)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if result.Data.Klines != nil && len(result.Data.Klines) > 0 {
		return false, true
	}
	return true, true
}

// IsUSTradingTime 判断当前时间是否在美股交易时间内
func IsUSTradingTime(date time.Time) bool {
	est, err := time.LoadLocation("America/New_York")
	var estTime time.Time
	if err != nil {
		estTime = date.Add(time.Hour * -12)
	} else {
		estTime = date.In(est)
	}

	if !isUSTradingDay(estTime) {
		return false
	}

	hour, minute, _ := estTime.Clock()

	if (hour == 4) || (hour == 5) || (hour == 6) || (hour == 7) || (hour == 8) || (hour == 9 && minute < 30) {
		return true
	}

	if (hour == 9 && minute >= 30) || (hour >= 10 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	if (hour == 16 && minute > 0) || (hour >= 17 && hour < 20) || (hour == 20 && minute == 0) {
		return true
	}

	return false
}

func isUSTradingDay(estTime time.Time) bool {
	weekday := estTime.Weekday()
	dateStr := estTime.Format("2006-01-02")

	cacheKey := []byte("us:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkUSHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkUSHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type usHolidayResp struct {
		IsHoliday    bool   `json:"is_holiday"`
		IsEarlyClose bool   `json:"is_early_close"`
		IsWeekend    bool   `json:"is_weekend"`
		Status       string `json:"status"`
	}
	var result usHolidayResp
	apiURL := fmt.Sprintf("https://fincalapi.com/v1/day_status?calendar=NYSE&date=%s", date)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	return result.IsHoliday, true
}
func MonitorFundPrices(a *App) {
	// 检查 A 股是否开市（基金交易时间与 A 股一致）
	if !isTradingTime(time.Now()) {
		logger.SugaredLogger.Debugf("当前 A 股未开市，跳过基金价格监控")
		return
	}

	logger.SugaredLogger.Debugf("A 股市场已开市，开始基金价格监控")

	dest := &[]data.FollowedFund{}
	db.Dao.Model(&data.FollowedFund{}).Find(dest)
	for _, follow := range *dest {
		_, err := data.NewFundApi().CrawlFundBasic(follow.Code)
		if err != nil {
			logger.SugaredLogger.Errorf("获取基金基本信息失败，基金代码：%s，错误信息：%s", follow.Code, err.Error())
			continue
		}
		data.NewFundApi().CrawlFundNetEstimatedUnit(follow.Code)
		data.NewFundApi().CrawlFundNetUnitValue(follow.Code)
	}
}

// MonitorAiRecommendStockPrices 监控 AI 推荐股票的价格，当股价达到预警线时发送通知
func MonitorAiRecommendStockPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过 AI 推荐股票价格监控")
		return
	}

	var aiRecommendStocks []models.AiRecommendStocks
	db.Dao.Model(&models.AiRecommendStocks{}).Where("enable_alert = ?", true).Find(&aiRecommendStocks)

	if len(aiRecommendStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockCodeMap := make(map[string]*models.AiRecommendStocks)
	for i := range aiRecommendStocks {
		stock := &aiRecommendStocks[i]
		stopLossPrice, _ := convertor.ToFloat(stock.RecommendStopLossPrice)
		if stock.RecommendBuyPriceMin <= 0 && stock.RecommendStopProfitPriceMin <= 0 && stopLossPrice <= 0 {
			continue
		}
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockCodeMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	if len(stockCodes) == 0 {
		logger.SugaredLogger.Debugf("没有设置预警价格的 AI 推荐股票，跳过价格监控")
		return
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取 AI 推荐股票实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		aiStock, ok := stockCodeMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		baseAlertKey := fmt.Sprintf("%s:%s", aiStock.StockCode, aiStock.DataTime.Format("20060102"))

		buyAlertKey := baseAlertKey + ":BUY"
		if aiStock.RecommendBuyPriceMin > 0 && currentPrice <= aiStock.RecommendBuyPriceMin {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if priceSinceLastBuyAlert == 0 || priceSinceLastBuyAlert > aiStock.RecommendBuyPriceMin {
				title := fmt.Sprintf("【买入预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议买入价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议买入价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax)
				if a.canSendAlert(buyAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(buyAlertKey)
					a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		} else {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if currentPrice > aiStock.RecommendBuyPriceMin && (priceSinceLastBuyAlert == 0 || currentPrice > priceSinceLastBuyAlert) {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		}

		profitAlertKey := baseAlertKey + ":PROFIT"
		if aiStock.RecommendStopProfitPriceMin > 0 && currentPrice >= aiStock.RecommendStopProfitPriceMin {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if priceSinceLastProfitAlert == 0 || priceSinceLastProfitAlert < aiStock.RecommendStopProfitPriceMin {
				title := fmt.Sprintf("【止盈预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止盈价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止盈价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax)
				if a.canSendAlert(profitAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(profitAlertKey)
					a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		} else {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if currentPrice < aiStock.RecommendStopProfitPriceMin && (priceSinceLastProfitAlert == 0 || currentPrice < priceSinceLastProfitAlert) {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		}

		stopLossAlertKey := baseAlertKey + ":LOSS"
		stopLossPrice, _ := convertor.ToFloat(aiStock.RecommendStopLossPrice)
		if stopLossPrice > 0 && currentPrice <= stopLossPrice {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if priceSinceLastLossAlert == 0 || priceSinceLastLossAlert > stopLossPrice {
				title := fmt.Sprintf("【止损预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止损价**: %s\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止损价: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice)
				if a.canSendAlert(stopLossAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(stopLossAlertKey)
					a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		} else {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if currentPrice > stopLossPrice && (priceSinceLastLossAlert == 0 || currentPrice > priceSinceLastLossAlert) {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		}
	}
}

// MonitorFollowedStockCostPrices 监控自选股的持仓成本价，当股价低于成本价时发送预警
func MonitorFollowedStockCostPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过自选股成本价监控")
		return
	}

	var followedStocks []data.FollowedStock
	db.Dao.Model(&data.FollowedStock{}).Where("cost_price > 0").Find(&followedStocks)

	if len(followedStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockMap := make(map[string]*data.FollowedStock)
	for i := range followedStocks {
		stock := &followedStocks[i]
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取自选股实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		followedStock, ok := stockMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		costPrice := followedStock.CostPrice
		if costPrice <= 0 {
			continue
		}

		alertKey := fmt.Sprintf("COST:%s:%s", followedStock.StockCode, followedStock.Time.Format("20060102"))

		if currentPrice < costPrice {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if priceSinceLastAlert == 0 || priceSinceLastAlert >= costPrice {
				dropPercent := ((costPrice - currentPrice) / costPrice) * 100
				title := fmt.Sprintf("【成本价预警】%s", followedStock.Name)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **持仓成本价**: %.2f\n- **亏损比例**: %.2f%%\n- **关注时间**: %s",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent,
					followedStock.Time.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n成本价: %.2f\n亏损: %.2f%%",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent)
				if a.canSendAlert(alertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(alertKey)
					a.updatePriceAtAlertReset(alertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		} else {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if currentPrice >= costPrice && (priceSinceLastAlert == 0 || currentPrice < priceSinceLastAlert) {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		}
	}
}

// canSendAlert 检查是否可以发送预警，避免重复发送
// alertKey: 预警的唯一标识
// interval: 发送间隔
// 返回 true 表示可以发送，false 表示需要在间隔后才能发送
func (a *App) canSendAlert(alertKey string, interval time.Duration) bool {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()

	lastSent, exists := a.stockAlertLastSent[alertKey]
	if !exists {
		return true
	}

	return time.Since(lastSent) >= interval
}

// updateAlertSentTime 更新预警发送时间
func (a *App) updateAlertSentTime(alertKey string) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.stockAlertLastSent[alertKey] = time.Now()
}

// getPriceAtAlertReset 获取预警重置后的价格（用于判断是否需要重新触发预警）
func (a *App) getPriceAtAlertReset(alertKey string) float64 {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	return a.priceAtAlertReset[alertKey]
}

// updatePriceAtAlertReset 更新预警重置后的价格
func (a *App) updatePriceAtAlertReset(alertKey string, price float64) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.priceAtAlertReset[alertKey] = price
}

func GetStockInfos(follows ...data.FollowedStock) *[]data.StockInfo {
	stockInfos := make([]data.StockInfo, 0)
	stockCodes := make([]string, 0)
	for _, follow := range follows {
		if strutil.HasPrefixAny(follow.StockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}
		stockCodes = append(stockCodes, follow.StockCode)
	}
	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil {
		return &stockInfos
	}
	for _, info := range *stockData {
		v, ok := slice.FindBy(follows, func(idx int, follow data.FollowedStock) bool {
			if strutil.HasPrefixAny(follow.StockCode, []string{"US", "us"}) {
				return strings.ToLower(strings.Replace(follow.StockCode, "us", "gb_", 1)) == info.Code
			}

			return follow.StockCode == info.Code
		})
		if ok {
			addStockFollowData(v, &info)
			stockInfos = append(stockInfos, info)
		}
	}
	return &stockInfos
}
func getStockInfo(follow data.FollowedStock) *data.StockInfo {
	stockCode := follow.StockCode
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	stockData := (*stockDatas)[0]
	addStockFollowData(follow, &stockData)
	return &stockData
}

func addStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price //上次当前价格
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice //成本价
	stockData.CostVolume = follow.Volume   //成本量
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups

	//当前价格
	price, _ := convertor.ToFloat(stockData.Price)
	//当前价格为0 时 使用卖一价格作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	//当前价格依然为0 时 使用买一报价作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	//昨日收盘价
	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)

	//当前价格依然为0 时 使用昨日收盘价为当前价格
	if price == 0 {
		price = preClosePrice
	}

	//今日最高价
	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	//今日最低价
	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}
	//开盘价
	//openPrice, _ := convertor.ToFloat(stockData.Open)

	if price > 0 && preClosePrice > 0 {
		stockData.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		stockData.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 3)
	}
	if highPrice > 0 && preClosePrice > 0 {
		stockData.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		stockData.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if follow.CostPrice > 0 && follow.Volume > 0 {
		if price > 0 {
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(price-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((price-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(follow.Volume), 2)
		} else {
			//未开盘时当前价格为昨日收盘价
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			// 未开盘时，今日盈亏为 0
			stockData.ProfitAmountToday = 0
		}

	}

	//logger.SugaredLogger.Debugf("stockData:%+v", stockData)
	if follow.Price != price && price > 0 {
		go db.Dao.Model(follow).Where("stock_code = ?", follow.StockCode).Updates(map[string]interface{}{
			"price": price,
		})
	}
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	defer PanicHandler()
	// 记录当前窗口大小，供下次启动时还原
	if a.ctx != nil {
		if w, h := runtime.WindowGetSize(a.ctx); w > 0 && h > 0 {
			cfg := data.GetSettingConfig()
			cfg.WindowWidth = w
			cfg.WindowHeight = h
			data.UpdateConfig(cfg)
			//logger.SugaredLogger.Infof("save window size: %dx%d", w, h)
		}
	}
	//logger.SugaredLogger.Infof("application shutdown Version:%s", Version)
}

// Greet returns a greeting for the given name
func (a *App) Greet(stockCode string) *data.StockInfo {
	//stockInfo, _ := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)

	follow := &data.FollowedStock{
		StockCode: stockCode,
	}
	db.Dao.Model(follow).Where("stock_code = ?", stockCode).Preload("Groups").Preload("Groups.GroupInfo").First(follow)
	stockInfo := getStockInfo(*follow)
	return stockInfo
}

func (a *App) Follow(stockCode string) string {
	return data.NewStockDataApi().Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return data.NewStockDataApi().UnFollow(stockCode)
}

func (a *App) GetFollowList(groupId int) *[]data.FollowedStock {
	return data.NewStockDataApi().GetFollowList(groupId)
}

func (a *App) GetStockList(key string) []data.StockBasic {
	return data.NewStockDataApi().GetStockList(key)
}

func (a *App) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return data.NewStockDataApi().SetCostPriceAndVolume(price, volume, stockCode)
}

func (a *App) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
}

func (a *App) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return data.NewStockDataApi().SetAlarmChangePercent(val, alarmPrice, stockCode)
}
func (a *App) SetStockSort(sort int64, stockCode string) {
	data.NewStockDataApi().SetStockSort(sort, stockCode)
}
func (a *App) SendDingDingMessage(message string, stockCode string) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	//logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), 60*5)
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {

	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
		return "非A股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
		return "非港股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
		return "非美股交易时间"
	}

	ttl, _ := a.cache.TTL([]byte(stockCode))
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), getMsgTypeTTL(msgType))
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	stockInfo := &data.StockInfo{}
	db.Dao.Model(stockInfo).Where("code = ?", stockCode).First(stockInfo)
	go data.NewAlertWindowsApi("go-stock消息通知", getMsgTypeName(msgType), GenNotificationMsg(stockInfo), "").SendNotification()

	go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    "📈 " + getMsgTypeName(msgType),
		"isRed":   true,
		"source":  "go-stock",
		"content": GenNotificationMsg(stockInfo),
	})

	return data.NewDingDingAPI().SendDingDingMessage(message)
}

func (a *App) NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Errorf("NewChatStream panic: %v", err)
			runtime.EventsEmit(a.ctx, "newChatStream", map[string]any{
				"code":    0,
				"content": fmt.Sprintf("AI分析异常: %v", err),
			})
			runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
		}
	}()
	var msgs <-chan map[string]any
	if enableTools {
		msgs = data.NewDeepSeekOpenAi(a.ctx, aiConfigId).NewChatStream(stock, stockCode, question, sysPromptId, a.AiTools, think)
	} else {
		msgs = data.NewDeepSeekOpenAi(a.ctx, aiConfigId).NewChatStream(stock, stockCode, question, sysPromptId, []data.Tool{}, think)
	}
	for msg := range msgs {
		runtime.EventsEmit(a.ctx, "newChatStream", msg)
	}
	runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
}

func (a *App) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	data.NewDeepSeekOpenAi(a.ctx, aiConfigId).SaveAIResponseResult(stockCode, stockName, result, chatId, question)
}
func (a *App) GetAIResponseResult(stock string) *models.AIResponseResult {
	return data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stock)
}

func (a *App) GetVersionInfo() *models.VersionInfo {
	return &models.VersionInfo{
		Version:           Version,
		Icon:              GetImageBase(icon),
		Alipay:            GetImageBase(alipay),
		Wxpay:             GetImageBase(wxpay),
		Wxgzh:             GetImageBase(wxgzh),
		Content:           VersionCommit,
		OfficialStatement: OFFICIAL_STATEMENT,
	}
}

func (a *App) GetUserManual() string {
	return string(userManual)
}

//// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
//func checkChromeOnWindows() bool {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	_, _, err = key.GetValue("Path", nil)
//	return err == nil
//}
//
//// checkEdgeOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
//func checkEdgeOnWindows() (string, bool) {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return "", false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	path, _, err := key.GetStringValue("Path")
//	if err != nil {
//		return "", false
//	}
//	return path, true
//}

func GetImageBase(bytes []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
}

func GenNotificationMsg(stockInfo *data.StockInfo) string {
	Price, err := convertor.ToFloat(stockInfo.Price)
	if err != nil {
		Price = 0
	}
	PreClose, err := convertor.ToFloat(stockInfo.PreClose)
	if err != nil {
		PreClose = 0
	}
	var RF float64
	if PreClose > 0 {
		RF = mathutil.RoundToFloat(((Price-PreClose)/PreClose)*100, 2)
	}

	return "[" + stockInfo.Name + "] " + stockInfo.Price + " " + convertor.ToString(RF) + "% " + stockInfo.Date + " " + stockInfo.Time
}

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
	case 4:
		return 60 * 5
	case 5:
		return 60 * 5
	default:
		return 60 * 5
	}
}

func getMsgTypeName(msgType int) string {
	switch msgType {
	case 1:
		return "涨跌报警"
	case 2:
		return "股价报警"
	case 3:
		return "成本价报警"
	case 4:
		return "止盈报警"
	case 5:
		return "止损报警"
	default:
		return "未知类型"
	}
}

func onExit(a *App) {
	// 清理操作
	//logger.SugaredLogger.Infof("systray onExit")
	//systray.Quit()
	//runtime.Quit(a.ctx)
}

func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	//s1, _ := json.Marshal(settingConfig)
	//logger.SugaredLogger.Infof("UpdateConfig:%s", s1)
	if settingConfig.RefreshInterval > 0 {
		if entryID, exists := a.getCronEntry("MonitorStockPrices"); exists {
			a.cron.Remove(entryID)
		}
		id, _ := a.cron.AddFunc(fmt.Sprintf("@every %ds", settingConfig.RefreshInterval), func() {
			MonitorStockPrices(a)
		})
		a.setCronEntry("MonitorStockPrices", id)
	}

	return data.UpdateConfig(settingConfig)
}

func (a *App) GetConfig() *data.SettingConfig {
	return data.GetSettingConfig()
}

func (a *App) ExportConfig() string {
	config := data.NewSettingsApi().Export()
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "导出配置文件",
		CanCreateDirectories: true,
		DefaultFilename:      "config.json",
	})
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	err = os.WriteFile(file, []byte(config), os.ModePerm)
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	return "导出成功:" + file
}

func (a *App) ShareAnalysis(stockCode, stockName string) string {
	//http://go-stock.sparkmemory.top:16688/upload
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006/01/02")
		//logger.SugaredLogger.Infof("%s analysisTime:%s", res.CreatedAt, analysisTime)
		response, err := data.SharedHTTPClient.R().SetHeader("ua-x", "go-stock").SetFormData(map[string]string{
			"text":         res.Content,
			"stockCode":    stockCode,
			"stockName":    stockName,
			"analysisTime": analysisTime,
		}).Post("http://go-stock.sparkmemory.top:16688/upload")
		if err != nil {
			return err.Error()
		}
		return response.String()
	} else {
		return "分析结果异常"
	}
}

// ShareText 直接把文本分享到社区（用于 AI 助手等非 AIResponseResult 场景）
func (a *App) ShareText(text, title string) string {
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)
	if text == "" {
		return "内容为空"
	}
	if title == "" {
		title = "AI助手"
	}
	analysisTime := time.Now().Format("2006/01/02")
	response, err := data.SharedHTTPClient.R().SetHeader("ua-x", "go-stock").SetFormData(map[string]string{
		"text":         text,
		"stockCode":    title,
		"stockName":    title,
		"analysisTime": analysisTime,
	}).Post("http://go-stock.sparkmemory.top:16688/upload")
	if err != nil {
		return err.Error()
	}
	return response.String()
}

func (a *App) GetfundList(key string) []data.FundBasic {
	return data.NewFundApi().GetFundList(key)
}
func (a *App) GetFollowedFund() []data.FollowedFund {
	return data.NewFundApi().GetFollowedFund()
}
func (a *App) FollowFund(fundCode string) string {
	return data.NewFundApi().FollowFund(fundCode)
}
func (a *App) UnFollowFund(fundCode string) string {
	return data.NewFundApi().UnFollowFund(fundCode)
}
func (a *App) GetFundKLine(fundCode string, klt string, limit int) *data.KLineSourceResult {
	return data.NewFundKLineApi().GetFundKLineWithFallback(fundCode, klt, limit)
}
func (a *App) GetFundHistoryNetValue(fundCode string, pageSize int, startDate string, endDate string) []data.FundHistoryNetValue {
	res, _ := data.NewFundApi().GetFundHistoryNetValue(fundCode, 1, pageSize, startDate, endDate)
	if res == nil {
		return []data.FundHistoryNetValue{}
	}
	return res
}
func (a *App) GetFundTop10Holdings(fundCode string) []data.FundHoldingStock {
	res, err := data.NewFundApi().GetFundTop10Holdings(fundCode)
	if err != nil || res == nil {
		return []data.FundHoldingStock{}
	}
	return res
}
func (a *App) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) *data.FundRankingResult {
	res, err := data.NewFundApi().GetFundRanking(marketType, fundType, sortField, sortOrder, pageIndex, pageSize)
	if err != nil || res == nil {
		return &data.FundRankingResult{}
	}
	return res
}
func (a *App) SearchFundCodes(keyword string) []data.FundSearchItem {
	return data.NewFundApi().SearchFundCodes(keyword)
}
func (a *App) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *data.FollowedFundPagedResult {
	return data.NewFundApi().GetFollowedFundPaged(pageIndex, pageSize, keyword)
}
func (a *App) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
		file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "保存为Markdown",
			DefaultFilename: fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime),
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown",
					Pattern:     "*.md;*.markdown",
				},
			},
		})
		if err != nil {
			return err.Error()
		}
		err = os.WriteFile(file, []byte(res.Content), 0644)
		return "已保存至：" + file
	}
	return "分析结果异常,无法保存。"
}

func (a *App) GetPromptTemplates(name, promptType string) *[]models.PromptTemplate {
	return data.NewPromptTemplateApi().GetPromptTemplates(name, promptType)
}
func (a *App) AddPrompt(prompt models.Prompt) string {
	promptTemplate := models.PromptTemplate{
		ID:      prompt.ID,
		Content: prompt.Content,
		Name:    prompt.Name,
		Type:    prompt.Type,
	}
	return data.NewPromptTemplateApi().AddPrompt(promptTemplate)
}
func (a *App) DelPrompt(id uint) string {
	return data.NewPromptTemplateApi().DelPrompt(id)
}
func (a *App) SetStockAICron(cronText, stockCode string) {
	data.NewStockDataApi().SetStockAICron(cronText, stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	if entryID, exists := a.getCronEntry(stockCode); exists {
		a.cron.Remove(entryID)
	}
	follow := data.NewStockDataApi().GetFollowedStockByStockCode(stockCode)
	id, _ := a.cron.AddFunc(cronText, a.AddCronTask(follow))
	a.setCronEntry(stockCode, id)

}
func (a *App) AddGroup(group data.Group) string {
	ok := data.NewStockGroupApi(db.Dao).AddGroup(group)
	if ok {
		return "添加成功"
	} else {
		return "添加失败"
	}
}
func (a *App) GetGroupList() []data.Group {
	return data.NewStockGroupApi(db.Dao).GetGroupList()
}

func (a *App) UpdateGroupSort(id int, newSort int) bool {
	return data.NewStockGroupApi(db.Dao).UpdateGroupSort(id, newSort)
}

func (a *App) InitializeGroupSort() bool {
	return data.NewStockGroupApi(db.Dao).InitializeGroupSort()
}

func (a *App) GetGroupStockList(groupId int) []data.GroupStock {
	return data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
}

func (a *App) AddStockGroup(groupId int, stockCode string) string {
	ok := data.NewStockGroupApi(db.Dao).AddStockGroup(groupId, stockCode)
	if ok {
		return "添加成功"
	} else {
		return "添加失败"
	}
}

func (a *App) RemoveStockGroup(code, name string, groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveStockGroup(code, name, groupId)
	if ok {
		return "移除成功"
	} else {
		return "移除失败"
	}
}

func (a *App) RemoveGroup(groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveGroup(groupId)
	if ok {
		return "移除成功"
	} else {
		return "移除失败"
	}
}

func (a *App) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return data.NewStockDataApi().GetHK_KLineData(stockCode, "day", days)
}

func (a *App) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	res := make(map[string]any, 4)
	priceData, date := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
	res["priceData"] = priceData
	res["date"] = date
	res["stockName"] = stockName
	res["stockCode"] = stockCode
	return res
}

func (a *App) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return data.NewStockDataApi().GetCommonKLineData(stockCode, "day", days)
}

// GetStockEastMoneyKLine 东方财富多周期 K 线（分钟：1/5/10/60/120；日 101、周 102、半年 105、年 106）。
// klt 与东方财富接口一致；10 分钟由 1 分钟数据聚合。limit 为根数上限（最大 5000）。
func (a *App) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return a.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, "")
}

// GetStockEastMoneyKLinePage 分页拉取 K 线：end 为东财 end 参数（YYYYMMDD 或 YYYYMMDDHHmmss），空字符串表示取最新一段（同 GetStockEastMoneyKLine）。
func (a *App) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "1"
	}
	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	end = strings.TrimSpace(end)
	//if klt == "10" {
	//	fetchN := limit * 10
	//	if fetchN > 5000 {
	//		fetchN = 5000
	//	}
	//	raw := api.GetKLineDataBefore(stockCode, "1", "", fetchN, end)
	//	return data.AggregateKLineEveryN(raw, 10)
	//}
	return api.GetKLineDataBefore(stockCode, klt, "", limit, end)
}

// GetStockKLineWithFallback 多数据源自动切换 K 线：优先东方财富，不可用时自动切换新浪财经。
// 返回 KLineSourceResult，包含 data（K 线数组）和 source（实际使用的数据源标识：eastmoney / sina）。
func (a *App) GetStockKLineWithFallback(stockCode, stockName string, klt string, limit int) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, "")
}

// GetStockKLinePageWithFallback 多数据源自动切换 K 线（分页）：优先东方财富，不可用时自动切换新浪财经。
// end 参数仅对东方财富有效；新浪数据源不支持分页，将返回最新一段数据。
func (a *App) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	end = strings.TrimSpace(end)
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, end)
}

// GetChipDistribution 获取/计算股票筹码分布（筹码图）数据（用于前端绘图）。
// days：近多少个交易日；bins：分箱数量；adjustFlag：""/qfq/hfq
func (a *App) GetChipDistribution(stockCode string, days int, bins int, adjustFlag string) (*data.ChipDistributionResult, error) {
	stockCode = strings.TrimSpace(stockCode)
	if stockCode == "" {
		return nil, fmt.Errorf("stockCode 不能为空")
	}
	if days <= 0 {
		days = 120
	}
	if bins <= 0 {
		bins = 80
	}
	adjustFlag = strings.TrimSpace(strings.ToLower(adjustFlag))
	if adjustFlag != "" && adjustFlag != "qfq" && adjustFlag != "hfq" {
		adjustFlag = "qfq"
	}

	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	if !api.ValidateStockCode(stockCode) {
		return nil, fmt.Errorf("股票代码无效：%s", stockCode)
	}

	var kLines *[]data.KLineData

	if adjustFlag != "" {
		kLines = api.GetKLineData(stockCode, "101", adjustFlag, days)
	} else {
		result := data.FetchKLineWithFallback(stockCode, "", "101", days, "")
		if result != nil && result.Data != nil {
			kLines = result.Data
		}
	}

	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("未获取到K线数据")
	}
	calculator := data.NewChipDistributionCalculator()
	return calculator.Calculate(stockCode, *kLines, bins)
}

// GetTdxCallAuction 通过通达信协议获取集合竞价数据。
// stockCode 格式如 600519.SH、000001.SZ、430047.BJ；start 为起始位置（0=最新）；count 为请求数量（最大 500）。
func (a *App) GetTdxCallAuction(stockCode string, start uint32, count uint32) *[]data.TdxCallAuctionData {
	if count <= 0 {
		count = 500
	}
	api := data.NewTdxKLineApi()
	return api.GetCallAuction(stockCode, start, count)
}

func (a *App) GetTdxCompanyInfo(stockCode string) *data.TdxCompanyInfoBundle {
	api := data.NewTdxKLineApi()
	return api.GetF10Data(stockCode)
}

func (a *App) GetTdxFinanceInfo(stockCode string) *data.TdxFinanceInfo {
	api := data.NewTdxKLineApi()
	return api.GetFinanceInfo(stockCode)
}

func (a *App) GetTdxXDXRInfo(stockCode string) *[]data.TdxXDXRItem {
	api := data.NewTdxKLineApi()
	return api.GetXDXRInfo(stockCode)
}

func (a *App) GetTdxCompanyCategoryList(stockCode string) *[]data.TdxCompanyCategory {
	api := data.NewTdxKLineApi()
	return api.GetF10CategoryList(stockCode)
}

func (a *App) GetTdxCompanyCategoryContent(stockCode string, categoryName string) *data.TdxCompanyInfoSection {
	api := data.NewTdxKLineApi()
	return api.GetF10CategoryContent(stockCode, categoryName)
}

// GetTdxSymbolBelongBoard 通过通达信 MAC 接口获取股票所属板块信息
func (a *App) GetTdxSymbolBelongBoard(stockCode string) *[]data.MACBelongBoardItem {
	api := data.NewTdxKLineApi()
	return api.GetMACSymbolBelongBoard(stockCode)
}

func (a *App) GetTelegraphList(source string) *[]*models.Telegraph {
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

func (a *App) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	//data.NewMarketNewsApi().GetNewTelegraph(30)
	go data.NewMarketNewsApi().TelegraphList(30)
	go data.NewMarketNewsApi().GetSinaNews(30)
	go data.NewMarketNewsApi().TradingViewNews()
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

func (a *App) GlobalStockIndexes() map[string]any {
	return data.NewMarketNewsApi().GlobalStockIndexes(30)
}

// GlobalStockIndexesReadable 将全球指数 JSON 转为 AI 易读 Markdown 文本。
func (a *App) GlobalStockIndexesReadable() string {
	return data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
}

func (a *App) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	ctx, cancel := context.WithCancel(a.ctx)

	// 保存当前会话的 cancel，用于前端中断
	a.summaryMu.Lock()
	if a.summaryCancel != nil {
		a.summaryCancel()
	}
	a.summaryCancel = cancel
	a.summaryMu.Unlock()

	// 允许前端自定义事件名，避免不同页面之间的事件冲突
	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}

	// 解析对话历史（AI 助手记忆）：空字符串或解析失败则无历史
	var history []map[string]interface{}
	if strings.TrimSpace(historyJSON) != "" {
		var list []models.AiAssistantMessage
		if err := json.Unmarshal([]byte(historyJSON), &list); err == nil && len(list) > 0 {
			history = make([]map[string]interface{}, 0, len(list))
			for _, m := range list {
				item := map[string]interface{}{"role": m.Role, "content": m.Content}
				if m.Role == "assistant" && m.Reasoning != "" {
					item["reasoning_content"] = m.Reasoning
				}
				history = append(history, item)
			}
		}
	}

	var msgs <-chan map[string]any
	if enableTools {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStreamWithTools(question, sysPromptId, a.AiTools, think, history)
	} else {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStream(question, sysPromptId, think, history)
	}

	for msg := range msgs {
		runtime.EventsEmit(a.ctx, eventName, msg)
	}

	a.summaryMu.Lock()
	a.summaryCancel = nil
	a.summaryMu.Unlock()

	runtime.EventsEmit(a.ctx, eventName, "DONE")
}
func (a *App) GetIndustryRank(sort string, cnt int) []any {
	res := data.NewMarketNewsApi().GetIndustryRank(sort, cnt)
	return res["data"].([]any)
}
func (a *App) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
	return res
}
func (a *App) GetMoneyRankSina(sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetMoneyRankSina(sort)
	return res
}

func (a *App) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	res := data.NewMarketNewsApi().GetStockMoneyTrendByDay(stockCode, days)
	slice.Reverse(res)
	return res
}

// OpenURL
//
//	@Description:  跨平台打开默认浏览器
//	@receiver a
//	@param url
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// SaveImage
//
//	@Description: 跨平台保存图片
//	@receiver a
//	@param name
//	@param base64Data
//	@return error
func (a *App) SaveImage(name, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: name + "AI分析.png",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG 图片",
				Pattern:     "*.png",
			},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}

	err = os.WriteFile(filepath.Clean(filePath), decodeString, os.ModePerm)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// SaveWordFile
//
//	@Description: // 跨平台保存word
//	@receiver a
//	@param filename
//	@param base64Data
//	@return error
func (a *App) SaveWordFile(filename string, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Word 文件",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Word 文件", Pattern: "*.docx"},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}
	err = os.WriteFile(filepath.Clean(filePath), decodeString, 0777)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// GetAiConfigs
//
//	@Description: // 获取 AiConfig 列表
//	@receiver a
//	@return error
func (a *App) GetAiConfigs() []*data.AIConfig {
	return data.GetSettingConfig().AiConfigs
}

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的
func (a *App) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return data.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (a *App) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return data.SaveAiAssistantSession(sessionId, messages)
}

// FetchAiModels
//
//	@Description: 根据接口地址与 apiKey 自动获取支持的模型列表（OpenAI/DeepSeek 兼容 /models 接口）
//	@receiver a
//	@param baseUrl 接口地址（如 https://api.deepseek.com）
//	@param apiKey  鉴权令牌
//	@return []string 模型 ID 列表
func (a *App) FetchAiModels(baseUrl, apiKey string) []string {
	baseUrl = strutil.Trim(baseUrl)
	apiKey = strutil.Trim(apiKey)
	if baseUrl == "" || apiKey == "" {
		return []string{}
	}

	type modelItem struct {
		ID string `json:"id"`
	}
	var respData struct {
		Data []modelItem `json:"data"`
	}

	client := data.SharedHTTPClient
	client.SetBaseURL(baseUrl)

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData).
		Get("/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	modelsList := make([]string, 0, len(respData.Data))
	for _, m := range respData.Data {
		if strings.TrimSpace(m.ID) != "" {
			modelsList = append(modelsList, m.ID)
		}
	}
	return modelsList
}

type AiModelInfo struct {
	ModelName string `json:"modelName"`
	MaxTokens int    `json:"maxTokens"`
	Source    string `json:"source"`
}

func (a *App) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	baseUrl = strutil.Trim(baseUrl)
	modelName = strutil.Trim(modelName)
	if baseUrl == "" || modelName == "" {
		return nil
	}

	info := &AiModelInfo{
		ModelName: modelName,
		MaxTokens: 0,
		Source:    "",
	}

	if apiKey != "" {
		type modelDetail struct {
			ID             string `json:"id"`
			MaxContextLen  int    `json:"max_context_length"`
			ContextLength  int    `json:"context_length"`
			MaxOutputTok   int    `json:"max_output_tokens"`
			MaxTokensField int    `json:"max_tokens"`
		}
		var detail modelDetail

		client := data.SharedHTTPClient
		client.SetBaseURL(baseUrl)

		resp, err := client.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetResult(&detail).
			Get("/models/" + modelName)

		if err == nil && !resp.IsError() && detail.ID != "" {
			if detail.MaxContextLen > 0 {
				info.MaxTokens = detail.MaxContextLen
				info.Source = "api"
			} else if detail.ContextLength > 0 {
				info.MaxTokens = detail.ContextLength
				info.Source = "api"
			} else if detail.MaxOutputTok > 0 {
				info.MaxTokens = detail.MaxOutputTok
				info.Source = "api"
			} else if detail.MaxTokensField > 0 {
				info.MaxTokens = detail.MaxTokensField
				info.Source = "api"
			}
		}
	}

	if info.MaxTokens == 0 {
		if maxTokens := getBuiltinModelMaxTokens(modelName); maxTokens > 0 {
			info.MaxTokens = maxTokens
			info.Source = "builtin"
		}
	}

	return info
}

func getBuiltinModelMaxTokens(modelName string) int {
	modelTokenMap := map[string]int{
		"deepseek-chat":        65536,
		"deepseek-reasoner":    65536,
		"deepseek-coder":       16384,
		"deepseek-v3":          65536,
		"deepseek-r1":          65536,
		"gpt-4o":               16384,
		"gpt-4o-mini":          16384,
		"gpt-4o-2024-05-13":    4096,
		"gpt-4-turbo":          4096,
		"gpt-4-turbo-preview":  4096,
		"gpt-4":                8192,
		"gpt-4-32k":            32768,
		"gpt-3.5-turbo":        4096,
		"gpt-3.5-turbo-16k":    16384,
		"gpt-4.1":              32768,
		"gpt-4.1-mini":         32768,
		"gpt-4.1-nano":         32768,
		"o1":                   100000,
		"o1-mini":              65536,
		"o1-preview":           32768,
		"o3-mini":              100000,
		"o4-mini":              100000,
		"claude-3-5-sonnet":    8192,
		"claude-3-5-haiku":     8192,
		"claude-3-opus":        4096,
		"claude-3-sonnet":      4096,
		"claude-3-haiku":       4096,
		"glm-4":                8192,
		"glm-4-plus":           4096,
		"glm-4-air":            4096,
		"glm-4-flash":          4096,
		"glm-4-long":           4096,
		"chatglm-turbo":        4096,
		"moonshot-v1-8k":       8192,
		"moonshot-v1-32k":      32768,
		"moonshot-v1-128k":     131072,
		"qwen-turbo":           8192,
		"qwen-plus":            131072,
		"qwen-max":             8192,
		"qwen-long":            65536,
		"qwen2.5-72b-instruct": 32768,
		"hunyuan-lite":         4096,
		"hunyuan-standard":     4096,
		"hunyuan-pro":          4096,
		"hunyuan-turbo":        4096,
		"spark-lite":           4096,
		"spark-pro":            4096,
		"spark-max":            4096,
		"spark-4.0-ultra":      4096,
		"yi-light":             16384,
		"yi-large":             16384,
		"yi-medium":            16384,
		"yi-spark":             16384,
		"yi-vision":            16384,
		"abab6.5-chat":         8192,
		"abab6.5s-chat":        8192,
		"abab5.5-chat":         4096,
		"baichuan2-turbo":      4096,
		"baichuan2-53b":        4096,
		"ernie-4.0":            4096,
		"ernie-3.5":            4096,
		"ernie-speed":          4096,
		"ernie-lite":           4096,
	}

	if maxTokens, ok := modelTokenMap[modelName]; ok {
		return maxTokens
	}

	for prefix, maxTokens := range map[string]int{
		"deepseek":      65536,
		"gpt-4o":        16384,
		"gpt-4-turbo":   4096,
		"gpt-4-":        8192,
		"gpt-3.5":       4096,
		"gpt-4.1":       32768,
		"o1-":           65536,
		"o3-":           100000,
		"o4-":           100000,
		"claude-3":      8192,
		"glm-4":         8192,
		"chatglm":       4096,
		"moonshot-v1":   8192,
		"qwen-":         8192,
		"qwen2":         32768,
		"hunyuan-":      4096,
		"spark-":        4096,
		"yi-":           16384,
		"abab":          8192,
		"baichuan":      4096,
		"ernie-":        4096,
		"llama-3":       8192,
		"llama3":        8192,
		"mistral-":      8192,
		"mixtral-":      32768,
		"codestral-":    32768,
		"gemini-1.5":    8192,
		"gemini-2":      8192,
		"command-r":     4096,
		"Qwen/Qwen":     32768,
		"deepseek-ai/":  65536,
		"meta-llama/":   8192,
		"mistralai/":    32768,
		"Pro/deepseek-": 65536,
		"Pro/qwen-":     32768,
	} {
		if strings.HasPrefix(modelName, prefix) {
			return maxTokens
		}
	}

	return 0
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	cronApi := agent.NewCronTaskApi()
	if !cronApi.ExistsByTaskType("stock_change_save") {
		task := &models.CronTask{
			Name:        "异动数据保存",
			CronExpr:    "0 */1 * * * *",
			TaskType:    "stock_change_save",
			Enable:      true,
			Status:      "active",
			Description: "每分钟自动保存A股异动数据（火箭发射、快速反弹、大笔买入、封涨停板等），交易时间外自动跳过",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建异动数据保存任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建异动数据保存定时任务")
		}
	}
	tasks := cronApi.GetAll()
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		taskCopy := t
		entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
			err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
			if err != nil {
				logger.SugaredLogger.Errorf("启动任务失败：%v %s", err, taskCopy.Name)
				return
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建定时任务失败：%v %s", err, taskCopy.Name)
			continue
		}
		a.setCronEntry(convertor.ToString(taskCopy.ID)+"_"+taskCopy.Name, entryID)
	}
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (a *App) AbortSummaryStockNews() {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	if a.summaryCancel != nil {
		a.summaryCancel()
		a.summaryCancel = nil
	}
}

// CreateCronTask
//
//	@Description: 创建定时任务
//	@receiver a
//	@param task 定时任务信息
//	@return string 操作结果
func (a *App) CreateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Create(task)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return "任务创建成功,但定时失败"
	}
	return "创建成功"
}

func (a *App) UpdateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Update(task)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	if entryID, exists := a.getCronEntry(convertor.ToString(task.ID) + "_" + task.Name); exists {
		a.cron.Remove(entryID)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

// DeleteCronTask
//
//	@Description: 删除定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) DeleteCronTask(id uint) string {
	err := agent.NewCronTaskApi().Delete(id)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
	}
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

// GetCronTaskByID
//
//	@Description: 根据 ID 获取定时任务
//	@receiver a
//	@param id 任务 ID
//	@return *models.CronTask 任务信息
func (a *App) GetCronTaskByID(id uint) *models.CronTask {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return nil
	}
	return task
}

// GetCronTaskList
//
//	@Description: 获取定时任务列表
//	@receiver a
//	@param query 查询条件
//	@return *models.CronTaskPageResp 分页结果
func (a *App) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	return agent.NewCronTaskApi().List(query)
}

// EnableCronTask
//
//	@Description: 启用/禁用定时任务
//	@receiver a
func (a *App) EnableCronTask(id uint, enable bool) string {
	err := agent.NewCronTaskApi().EnableTask(id, enable)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
		if enable {
			taskCopy := *task
			entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
				err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
				if err != nil {
					logger.SugaredLogger.Errorf("%s 执行任务失败：%v", taskCopy.Name, err)
					return
				}
			})
			a.setCronEntry(convertor.ToString(id)+"_"+task.Name, entryID)
			if err != nil {
				return "操作成功,但定时失败"
			}
		}

	}
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

// ExecuteCronTaskNow
//
//	@Description: 立即执行定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}

	go func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
		}
	}()

	return "任务执行中"
}

// GetCronTaskTypes
//
//	@Description: 获取所有任务类型
//	@receiver a
//	@return []lo.Tuple2[string, string] 任务类型列表
func (a *App) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

// ValidateCronExpr
//
//	@Description: 验证 Cron 表达式
//	@receiver a
//	@param expr Cron 表达式
//	@return string 验证结果
func (a *App) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

// SearchCronTasks
//
//	@Description: 搜索定时任务
//	@receiver a
//	@param keyword 搜索关键词
//	@return []models.CronTask 搜索结果
func (a *App) SearchCronTasks(keyword string) []models.CronTask {
	return agent.NewCronTaskApi().SearchTasks(keyword)
}

// CalculateNextRunTime 根据 Cron 表达式计算下一次运行时间
// 参数:
//   - cron: Cron 表达式，用于定义任务调度的时间规则
//
// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串
func (a *App) CalculateNextRunTime(cron string) string {
	nextRunTime := agent.NewCronTaskApi().CalculateNextRunTime(cron)
	return nextRunTime.Format("2006-01-02 15:04:05")
}

// CalculateNextRunTimes 根据 Cron 表达式计算未来多次运行时间
// 参数:
//   - cron: Cron 表达式
//   - count: 需要计算的次数
//
// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"
func (a *App) CalculateNextRunTimes(cron string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cron, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// AddTradingRecord 添加交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - uint: 新添加的交易记录ID
//   - error: 错误信息
func (a *App) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().AddTradingRecord(record)
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）
func (a *App) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	page, err := data.NewStockDataApi().GetTradingRecordList(query)
	if err != nil {
		return &data.TradingRecordPageData{}
	}
	return page
}

// GetTradingRecordById 根据ID获取单个交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - *data.TradingRecord: 交易记录指针
//   - error: 错误信息
func (a *App) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return data.NewStockDataApi().GetTradingRecordById(id)
}

// GetTradingRecordStatistics 获取交易记录统计数据
//
// 返回值:
//   - *data.TradingRecordStatistics: 统计数据指针
func (a *App) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := data.NewStockDataApi().GetTradingRecordStatistics()
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return stats
}

// UpdateTradingRecord 更新交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - error: 错误信息
func (a *App) UpdateTradingRecord(record data.TradingRecord) error {
	return data.NewStockDataApi().UpdateTradingRecord(record)
}

// DeleteTradingRecord 删除交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - error: 错误信息
func (a *App) DeleteTradingRecord(id uint) error {
	return data.NewStockDataApi().DeleteTradingRecord(id)
}

// CheckFrequentTrading 检查是否频繁交易
// 参数:
//   - stockCode: 股票代码
//
// 返回值:
//   - map[string]any: 包含 canTrade (bool) 和 msg (string)
func (a *App) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := data.NewStockDataApi().CheckFrequentTrading(stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}

func (a *App) FetchAndSaveMarketStatistic() {
	if !isTradingTime(time.Now()) {
		logger.SugaredLogger.Debugf("当前非交易时间，跳过市场统计数据采集")
		return
	}
	err := data.NewMarketStatisticApi().FetchAndSave()
	if err != nil {
		logger.SugaredLogger.Errorf("获取市场统计数据失败: %v", err)
	}
}

func (a *App) GetTodayMarketStatistic() []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetTodayData()
}

func (a *App) GetMarketStatisticByDate(date string) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetByDate(date)
}

func (a *App) GetRecentDaysMarketStatistic(days int) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetRecentDaysData(days)
}

func (a *App) CreateMCPServer(server *models.MCPServer) string {
	err := data.NewMCPServerApi().Create(server)
	if err != nil {
		logger.SugaredLogger.Errorf("创建MCP服务器失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

func (a *App) UpdateMCPServer(server *models.MCPServer) string {
	err := data.NewMCPServerApi().Update(server)
	if err != nil {
		logger.SugaredLogger.Errorf("更新MCP服务器失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

func (a *App) DeleteMCPServer(id uint) string {
	err := data.NewMCPServerApi().Delete(id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除MCP服务器失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (a *App) GetMCPServerByID(id uint) *models.MCPServer {
	server, err := data.NewMCPServerApi().GetByID(id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器失败: %v", err)
		return nil
	}
	return server
}

func (a *App) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	return data.NewMCPServerApi().List(query)
}

func (a *App) EnableMCPServer(id uint, enable bool) string {
	err := data.NewMCPServerApi().EnableServer(id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用MCP服务器失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

func (a *App) TestMCPServer(id uint) string {
	result, err := data.NewMCPServerApi().TestConnection(id)
	if err != nil {
		logger.SugaredLogger.Errorf("测试MCP服务器连接失败: %v", err)
		return "测试失败: " + err.Error()
	}
	return result
}

func (a *App) CreateSkill(skill *models.Skill) string {
	err := data.NewSkillApi().Create(skill)
	if err != nil {
		logger.SugaredLogger.Errorf("创建技能失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

func (a *App) UpdateSkill(skill *models.Skill) string {
	err := data.NewSkillApi().Update(skill)
	if err != nil {
		logger.SugaredLogger.Errorf("更新技能失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

func (a *App) DeleteSkill(id uint) string {
	err := data.NewSkillApi().Delete(id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除技能失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (a *App) GetSkillByID(id uint) *models.Skill {
	skill, err := data.NewSkillApi().GetByID(id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取技能失败: %v", err)
		return nil
	}
	return skill
}

func (a *App) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	return data.NewSkillApi().List(query)
}

func (a *App) EnableSkill(id uint, enable bool) string {
	err := data.NewSkillApi().EnableSkill(id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用技能失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

func (a *App) GetAllSkills() []models.Skill {
	return data.NewSkillApi().GetAll()
}

func (a *App) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	return data.NewMCPServerApi().GetToolsByServerID(serverID)
}

func (a *App) GetAllMCPTools() []models.MCPServerTool {
	return data.NewMCPServerApi().GetAllTools()
}
