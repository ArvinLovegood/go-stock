//go:build linux
// +build linux

package main

// Linux 平台适配：仅保留平台差异部分，通用逻辑统一在 app.go。
// （历史版本曾整份复制 app.go，重构后与 app.go 重复声明导致 Linux 无法编译。）

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	defer PanicHandler()

	data.ConfigureFromSettings(data.GetSettingConfig())

	runtime.EventsOn(ctx, "frontendError", func(optionalData ...interface{}) {
		logger.SugaredLogger.Errorf("Frontend error: %v\n", optionalData)
	})
	logger.SugaredLogger.Infof("Version:%s", Version)
	a.ctx = ctx

	// 设置全局 Wails 上下文，供 AI 工具修改分组/概念后向前端推送刷新事件
	data.SetAppCtx(ctx)

	// 应用启动时自动创建已启用的定时任务
	a.InitCronTasks()

	preCacheTradingDays()

	// 监听设置更新事件
	runtime.EventsOn(ctx, "updateSettings", func(optionalData ...interface{}) {
		config := data.GetSettingConfig()
		if config.DarkTheme {
			runtime.WindowSetBackgroundColour(ctx, 27, 38, 54, 1)
			runtime.WindowSetDarkTheme(ctx)
		} else {
			runtime.WindowSetBackgroundColour(ctx, 255, 255, 255, 1)
			runtime.WindowSetLightTheme(ctx)
		}
		runtime.WindowReloadApp(ctx)
	})

	// 启动通知（无通知守护进程的发行版可能失败，仅记录日志，不中断启动）
	go func() {
		if err := beeep.Notify("go-stock", "应用程序已启动", ""); err != nil {
			logger.SugaredLogger.Warnf("系统通知失败: %v", err)
		}
	}()

	logger.SugaredLogger.Infof(" application startup Version:%s", Version)
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
	}
	// 应用退出时执行清理工作
	if a.cron != nil {
		a.cron.Stop()
	}
	return false // 如果选择了确定，继续关闭应用
}

// OnSecondInstanceLaunch 处理第二实例启动时的通知
func OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	if err := beeep.Notify("go-stock", "程序已经在运行了", ""); err != nil {
		logger.SugaredLogger.Warnf("系统通知失败: %v", err)
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

	// 计算总收益并通过通知展示（Linux 无 systray）
	if total != 0 {
		title := "go-stock " + time.Now().Format(time.DateTime) + fmt.Sprintf("  %.2f¥", total)
		if err := beeep.Notify("go-stock", title, ""); err != nil {
			logger.SugaredLogger.Warnf("发送通知失败: %v", err)
		}
	}

	// 触发实时利润事件
	go runtime.EventsEmit(a.ctx, "realtime_profit", fmt.Sprintf("  %.2f", total))
}

// getFrameless 返回是否使用无边框窗口
func getFrameless() bool {
	return false
}

// getScreenResolution 返回屏幕分辨率（Linux 使用合理默认值）
func getScreenResolution() (int, int, int, int, error) {
	return 1412, 834, 900, 600, nil
}
