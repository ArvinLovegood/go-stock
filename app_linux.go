//go:build linux
// +build linux

package main

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"log"
	"time"

	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx   context.Context
	cache *freecache.Cache
	cron  *data.CronTaskManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	return &App{
		cache: cache,
	}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	defer PanicHandler()

	data.ConfigureFromSettings(data.GetSettingConfig())

	runtime.EventsOn(ctx, "frontendError", func(optionalData ...interface{}) {
		logger.SugaredLogger.Errorf("Frontend error: %v\n", optionalData)
	})
	logger.SugaredLogger.Infof("Version:%s", Version)
	// Perform your setup here
	a.ctx = ctx

	// 设置全局 Wails 上下文，供 AI 工具修改分组/概念后向前端推送刷新事件
	data.SetAppCtx(ctx)

	// 应用启动时自动创建已启用的定时任务
	a.InitCronTasks()

	preCacheTradingDays()

	// 监听设置更新事件
	runtime.EventsOn(ctx, "updateSettings", func(optionalData ...interface{}) {
		config := data.GetSettingConfig()
		//logger.SugaredLogger.Infof("updateSettings config:%+v", config)
		if config.DarkTheme {
			runtime.WindowSetBackgroundColour(ctx, 27, 38, 54, 1)
			runtime.WindowSetDarkTheme(ctx)
		} else {
			runtime.WindowSetBackgroundColour(ctx, 255, 255, 255, 1)
			runtime.WindowSetLightTheme(ctx)
		}
		runtime.WindowReloadApp(ctx)
	})

	// 创建 Linux 托盘通知
	go func() {
		err := beeep.Notify("go-stock", "应用程序已启动", "")
		if err != nil {
			log.Fatalf("系统通知失败：%v", err)
		}
	}()

	logger.SugaredLogger.Infof(" application startup Version:%s", Version)
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	// Add your action here
	//ticker := time.NewTicker(time.Second)
	//defer ticker.Stop()
	////定时更新数据
	//go func() {
	//	for range ticker.C {
	//		runtime.WindowSetTitle(ctx, "go-stock "+time.Now().Format("2006-01-02 15:04:05"))
	//	}
	//}()
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	defer PanicHandler()

	// 记录当前窗口大小，供下次启动时还原
	if a.ctx != nil {
		w, h := runtime.WindowGetSize(ctx)
		logger.SugaredLogger.Infof(" window size: %dx%d", w, h)
		if w > 0 && h > 0 {
			cfg := data.GetSettingConfig()
			cfg.WindowWidth = w
			cfg.WindowHeight = h
			data.UpdateConfig(cfg)
			logger.SugaredLogger.Infof("save window size: %dx%d", w, h)
		}
	}

	// 在 Linux 上使用 MessageDialog 显示确认窗口
	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:         runtime.QuestionDialog,
		Title:        "go-stock",
		Message:      "确定关闭吗？",
		Buttons:      []string{"确定", "取消"},
		Icon:         icon2,
		CancelButton: "取消",
	})

	if err != nil {
		logger.SugaredLogger.Errorf("dialog error:%s", err.Error())
		return false
	}

	logger.SugaredLogger.Debugf("dialog:%s", dialog)
	if dialog == "取消" || dialog == "No" {
		return true // 如果选择了取消，不关闭应用
	} else {
		// 在 Linux 上应用退出时执行清理工作
		if a.cron != nil {
			a.cron.Stop()
		}
		return false // 如果选择了确定，继续关闭应用
	}
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) *data.StockInfo {
	stockDatas, _ := data.NewStockDataApi().GetStockCodeRealTimeData(name)
	stockData := (*stockDatas)[0]
	return &stockData
}

func (a *App) Follow(stockCode string) string {
	return data.NewStockDataApi().Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return data.NewStockDataApi().UnFollow(stockCode)
}

func (a *App) GetFollowList() []data.FollowedStock {
	return data.NewStockDataApi().GetFollowList()
}

func (a *App) GetStockList(key string) []data.StockBasic {
	return data.NewStockDataApi().GetStockList(key)
}

func (a *App) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return data.NewStockDataApi().SetCostPriceAndVolume(price, volume, stockCode)
}

func (a *App) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return data.NewStockDataApi().SetAlarmChangePercent(val, alarmPrice, stockCode)
}

func (a *App) SendDingDingMessage(message string, stockCode string) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
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

func (a *App) SetStockSort(sort int64, stockCode string) {
	data.NewStockDataApi().SetStockSort(sort, stockCode)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), getMsgTypeTTL(msgType))
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	return data.NewDingDingAPI().SendDingDingMessage(message)
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

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
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
	default:
		return "未知类型"
	}
}
func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	return data.UpdateConfig(settingConfig)
}

func (a *App) GetConfig() *data.SettingConfig {
	return data.GetSettingConfig()
}

// OnSecondInstanceLaunch 处理第二实例启动时的通知
func OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	err := beeep.Notify("go-stock", "程序已经在运行了", "")
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	time.Sleep(time.Second * 3)
}

// MonitorStockPrices 监控股票价格
func MonitorStockPrices(a *App) {
	// 检查是否至少有一个市场开市
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	// 如果所有市场都不在交易时间，则提前返回
	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过价格监控")
		return
	}

	logger.SugaredLogger.Debugf("市场状态 - A股: %v, 港股: %v, 美股: %v", isAStockOpen, isHKStockOpen, isUSStockOpen)

	dest := &[]data.FollowedStock{}
	db.Dao.Model(&data.FollowedStock{}).Find(dest)
	total := float64(0)

	// 股票信息处理逻辑
	stockInfos := GetStockInfos(*dest...)
	for _, stockInfo := range *stockInfos {
		if strutil.HasPrefixAny(stockInfo.Code, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}

		total += stockInfo.ProfitAmountToday
		price, _ := convertor.ToFloat(stockInfo.Price)

		if stockInfo.PrePrice != price {
			go runtime.EventsEmit(a.ctx, "stock_price", stockInfo)
		}
	}

	// 计算总收益并更新状态
	if total != 0 {
		// 使用通知替代 systray 更新 Tooltip
		title := "go-stock " + time.Now().Format(time.DateTime) + fmt.Sprintf("  %.2f¥", total)

		// 发送通知显示实时数据
		err := beeep.Notify("go-stock", title, "")
		if err != nil {
			logger.SugaredLogger.Errorf("发送通知失败：%v", err)
		}
	}

	// 触发实时利润事件
	go runtime.EventsEmit(a.ctx, "realtime_profit", fmt.Sprintf("  %.2f", total))
}

// getFrameless 返回是否使用无边框窗口
func getFrameless() bool {
	return false
}

// getScreenResolution 返回屏幕分辨率
func getScreenResolution() (int, int, int, int, error) {
	// Linux 上使用简单的默认值
	// 可以通过 xrandr 或其他工具获取实际分辨率
	return 1412, 834, 900, 600, nil
}
