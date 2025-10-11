package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
	"runtime/debug"
	"strings"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/app.ico
var icon2 []byte

//go:embed build/screenshot/alipay.jpg
var alipay []byte

//go:embed build/screenshot/wxpay.jpg
var wxpay []byte

//go:embed build/screenshot/扫码_搜索联合传播样式-白色版.png
var wxgzh []byte

//go:embed build/stock_basic.json
var stocksBin []byte

//go:embed build/stock_base_info_hk.json
var stocksBinHK []byte

//go:embed build/stock_base_info_us.json
var stocksBinUS []byte

//go:generate cp -R ./data ./build/bin

var Version string
var VersionCommit string
var OFFICIAL_STATEMENT string
var BuildKey string

func main() {
	checkDir("data")
	db.Init("")
	go AutoMigrate()

	//db.Dao.Model(&data.Group{}).Where("id = ?", 0).FirstOrCreate(&data.Group{
	//	Name: "Default Group",
	//	Sort: 0,
	//})

	// Create an instance of the app structure
	app := NewApp()
	AppMenu := menu.NewMenu()
	if IsMacOS() {
		AppMenu.Append(menu.EditMenu())
	}
	//FileMenu := AppMenu.AddSubmenu("Settings")
	//FileMenu.AddText("Fullscreen", keys.CmdOrCtrl("f"), func(callback *menu.CallbackData) {
	//	runtime.WindowFullscreen(app.ctx)
	//})
	//FileMenu.AddText("Restore Window", keys.Key("Esc"), func(callback *menu.CallbackData) {
	//	runtime.WindowUnfullscreen(app.ctx)
	//})
	//FileMenu.AddText("Show Search Box", keys.CmdOrCtrl("s"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 1)
	//})
	//FileMenu.AddText("Hide Search Box", keys.CmdOrCtrl("d"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 0)
	//})
	//FileMenu.AddText("Refresh Data", keys.CmdOrCtrl("r"), func(callbackData *menu.CallbackData) {
	//	//runtime.EventsEmit(app.ctx, "refresh", "setting-"+time.Now().Format("2006-01-02 15:04:05"))
	//	runtime.EventsEmit(app.ctx, "refreshFollowList", "refresh-"+time.Now().Format("2006-01-02 15:04:05"))
	//})
	//FileMenu.AddSeparator()

	//if goruntime.GOOS == "windows" {
	//	FileMenu.AddText("Hide to Tray", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
	//		runtime.WindowHide(app.ctx)
	//	})
	//}

	//FileMenu.AddText("Exit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
	//	runtime.Quit(app.ctx)
	//})
	log.SugaredLogger.Info("version: " + Version)
	log.SugaredLogger.Info("commit: " + VersionCommit)
	// Create application with options
	//var width, height int
	//var err error
	//
	width, _, minWidth, minHeight, err := getScreenResolution()
	if err != nil {
		log.SugaredLogger.Error("get screen resolution error")
		width = 1456
		//height = 768
	}

	darkTheme := data.GetSettingConfig().DarkTheme
	backgroundColour := &options.RGBA{R: 255, G: 255, B: 255, A: 1}
	if darkTheme {
		backgroundColour = &options.RGBA{R: 27, G: 38, B: 54, A: 1}
	}

	//frameless := getFrameless()

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "go-stock: AI-Powered Stock Analysis✨",
		Width:     width * 4 / 5,
		Height:    920,
		MinWidth:  minWidth,
		MinHeight: minHeight,
		//MaxWidth:                 width,
		//MaxHeight:                height,
		DisableResize:            false,
		Fullscreen:               false,
		Frameless:                false,
		StartHidden:              false,
		HideWindowOnClose:        false,
		EnableDefaultContextMenu: true,
		BackgroundColour:         backgroundColour,
		Assets:                   assets,
		Menu:                     AppMenu,
		Logger:                   nil,
		LogLevel:                 logger.DEBUG,
		LogLevelProduction:       logger.ERROR,
		OnStartup:                app.startup,
		OnDomReady:               app.domReady,
		OnBeforeClose:            app.beforeClose,
		OnShutdown:               app.shutdown,
		WindowStartState:         options.Normal,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "go-stock-dev",
			OnSecondInstanceLaunch: OnSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: "",
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "go-stock",
				Message: "",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.SugaredLogger.Fatal(err)
	}

}

func updateMultipleModel() {
	oldSettings := &models.OldSettings{}
	db.Dao.Model(oldSettings).First(oldSettings)
	aiConfig := &data.AIConfig{}
	db.Dao.Model(aiConfig).First(aiConfig)
	if oldSettings.OpenAiEnable && oldSettings.OpenAiApiKey != "" && aiConfig.ID == 0 {
		aiConfig.Name = oldSettings.OpenAiModelName
		aiConfig.ApiKey = oldSettings.OpenAiApiKey
		aiConfig.BaseUrl = oldSettings.OpenAiBaseUrl
		aiConfig.ModelName = oldSettings.OpenAiModelName
		aiConfig.Temperature = oldSettings.OpenAiTemperature
		aiConfig.MaxTokens = oldSettings.OpenAiMaxTokens
		aiConfig.TimeOut = oldSettings.OpenAiApiTimeOut
		err := db.Dao.Model(aiConfig).Create(aiConfig).Error
		if err != nil {
			log.SugaredLogger.Error(err.Error())
		}
	}
}

func AutoMigrate() {
	db.Dao.AutoMigrate(&data.StockInfo{})
	db.Dao.AutoMigrate(&data.StockBasic{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.IndexBasic{})
	db.Dao.AutoMigrate(&data.Settings{})
	db.Dao.AutoMigrate(&models.AIResponseResult{})
	db.Dao.AutoMigrate(&models.StockInfoHK{})
	db.Dao.AutoMigrate(&models.StockInfoUS{})
	db.Dao.AutoMigrate(&data.FollowedFund{})
	db.Dao.AutoMigrate(&data.FundBasic{})
	db.Dao.AutoMigrate(&models.PromptTemplate{})
	db.Dao.AutoMigrate(&data.Group{})
	db.Dao.AutoMigrate(&data.GroupStock{})
	db.Dao.AutoMigrate(&models.Tags{})
	db.Dao.AutoMigrate(&models.Telegraph{})
	db.Dao.AutoMigrate(&models.TelegraphTags{})
	db.Dao.AutoMigrate(&models.LongTigerRankData{})
	db.Dao.AutoMigrate(&data.AIConfig{})
	db.Dao.AutoMigrate(&models.BKDict{})

	updateMultipleModel()
}

func initStockDataUS(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoUS
	err := json.Unmarshal(stocksBinUS, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data us %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoUS{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data us %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoUS{}).Create(&item)
		}
	}
}

func initStockDataHK(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoHK
	err := json.Unmarshal(stocksBinHK, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data hk %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoHK{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data hk %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoHK{}).Create(&item)
		}
	}

}

func updateBasicInfo() {
	config := data.GetSettingConfig()
	if config.UpdateBasicInfoOnStart {
		//Update basic information
		go data.NewStockDataApi().GetStockBaseInfo()
		go data.NewStockDataApi().GetIndexBasic()
	}
}

func initStockData(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	log.SugaredLogger.Info("init stock data")
	res := &data.TushareStockBasicResponse{}
	err := json.Unmarshal(stocksBin, res)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}

	for _, item := range res.Data.Items {
		stock := &data.StockBasic{}
		stockData := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			//logger.SugaredLogger.Infof("field: %s", field)
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			stockData[field] = item[idx]
		}
		jsonData, _ := json.Marshal(stockData)
		err := json.Unmarshal(jsonData, stock)
		if err != nil {
			continue
		}
		stock.ID = 0
		var count int64
		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
		if count > 0 {
			continue
		} else {
			db.Dao.Create(stock)
		}

		//db.Dao.Model(&data.StockBasic{}).FirstOrCreate(stock, &data.StockBasic{TsCode: stock.TsCode}).Where("ts_code = ?", stock.TsCode).Updates(stock)
	}

	//for _, item := range res.Data.Items {
	//	stock := &data.StockBasic{}
	//	stock.Exchange = convertor.ToString(item[0])
	//	stock.IsHs = convertor.ToString(item[1])
	//	stock.Name = convertor.ToString(item[2])
	//	stock.Industry = convertor.ToString(item[3])
	//	stock.ListStatus = convertor.ToString(item[4])
	//	stock.ActName = convertor.ToString(item[5])
	//	stock.ID = uint(item[6].(float64))
	//	stock.CurrType = convertor.ToString(item[7])
	//	stock.Area = convertor.ToString(item[8])
	//	stock.ListDate = convertor.ToString(item[9])
	//	stock.DelistDate = convertor.ToString(item[10])
	//	stock.ActEntType = convertor.ToString(item[11])
	//	stock.TsCode = convertor.ToString(item[12])
	//	stock.Symbol = convertor.ToString(item[13])
	//	stock.Cnspell = convertor.ToString(item[14])
	//	stock.Fullname = convertor.ToString(item[20])
	//	stock.Ename = convertor.ToString(item[21])
	//
	//	var count int64
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
	//	if count > 0 {
	//		continue
	//	} else {
	//		db.Dao.Create(stock)
	//	}
	//}
}

func checkDir(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
		log.SugaredLogger.Info("create dir: " + dir)
	}
	if BuildKey == "" {
		BuildKey = "cc1e0d684e32f176c56ff1fcf384dcd9"
	}
}

// PanicHandler wrapper function to recover from panic
func PanicHandler() {
	if r := recover(); r != nil {
		fmt.Printf("Recovered from panic: %v\n", r)
		debug.PrintStack()
	}
}
