package main

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 后台买卖点信号监控的 Go 侧职责只有一件：把前端算出的信号落到各提醒通道。
//
// 信号计算刻意留在前端 —— 买卖点/ TEMA 转折算法只存在一份 TypeScript 实现（kline/calc.ts），
// 移植到 Go 会造成永久双份维护且必然漂移（图上箭头与监控信号对不上）。
// Go 侧负责的是前端做不到的事：按分钟唤醒（窗口最小化时 Chromium 会把前端定时器深度节流）
// 以及复用已有的系统通知 / 飞书 / 钉钉通道。

// signalMonitorTick 后台信号监控节拍：按「扫描间隔」设置唤醒前端引擎（默认 60s，可 5~600s）。
// 前端会在收到后自行判断交易时段、监控开关与监控池，Go 侧不做业务判断。
// 刻意不导出（不暴露为前端绑定）：节拍只应由 cron 驱动。
func (a *App) signalMonitorTick() {
	go runtime.EventsEmit(a.ctx, "signalMonitorTick")
}

// GetSignalScanInterval 信号扫描节拍间隔（秒）。前端启动时用它迁移旧版全局设置，并按三套监控
// 各自扫描周期的最小值回写（Go tick 只按最小周期唤醒，各实例在前端自行节流）。
func (a *App) GetSignalScanInterval() int {
	return data.GetSignalScanIntervalSec()
}

// SetSignalScanInterval 设置信号扫描节拍间隔（1~600 秒）：落库、热更新 Go cron 并通知前端
// 重排兜底定时器，无需重启应用。由前端引擎在三套监控的扫描周期变化后按最小值调用。
// 返回空串表示成功，否则为错误信息。
func (a *App) SetSignalScanInterval(sec int) string {
	if sec < 1 || sec > 600 {
		return "扫描间隔仅支持 1~600 秒"
	}
	if err := data.SetSignalScanIntervalSec(sec); err != nil {
		logger.SugaredLogger.Errorf("SetSignalScanInterval error:%s", err.Error())
		return err.Error()
	}
	a.restartSignalMonitorTickCron(sec)
	go runtime.EventsEmit(a.ctx, "signalScanIntervalChanged", sec)
	return ""
}

// restartSignalMonitorTickCron 按 newSec 秒重注册信号监控节拍 cron（先移除旧任务再加新任务）。
func (a *App) restartSignalMonitorTickCron(newSec int) {
	if id, ok := a.getCronEntry("SignalMonitorTick"); ok {
		a.cron.Remove(id)
	}
	id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", newSec), func() {
		a.signalMonitorTick()
	})
	if err != nil {
		logger.SugaredLogger.Errorf("AddFunc SignalMonitorTick(%ds) error:%s", newSec, err.Error())
		return
	}
	a.setCronEntry("SignalMonitorTick", id)
	logger.SugaredLogger.Infof("信号扫描节拍已更新为 %ds", newSec)
}

// NotifySignal 信号提醒推送出口。
// title 通知标题；content 为 markdown（飞书/钉钉）；plain 为纯文本（系统通知气泡与前端横幅，markdown 在系统通知里会显示成裸符号）。
// channels 取 app/feishu/dingding，为空表示全部渠道，语义与「每日操作计划预警」保持一致。
func (a *App) NotifySignal(title, content, plain string, channels []string) string {
	if strings.TrimSpace(title) == "" {
		return "empty title"
	}
	useAll := len(channels) == 0
	send := func(ch string) bool {
		if useAll {
			return true
		}
		for _, c := range channels {
			if c == ch {
				return true
			}
		}
		return false
	}

	toastText := orDefault(plain, title)

	if send(NotifyChannelApp) {
		go data.NewAlertWindowsApi("go-stock买卖点信号", title, toastText, "").SendNotification()
		go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
			"time":    title,
			"isRed":   true,
			"source":  "go-stock",
			"content": toastText,
		})
	}
	if send(NotifyChannelFeishu) {
		go data.NewFeishuAPI().SendToFeishu(title, content)
	}
	if send(NotifyChannelDingDing) {
		go data.NewDingDingAPI().SendToDingDing(title, content)
	}
	return "ok"
}

// SaveSignalRecords 信号流水落库（SQLite）。
// 返回空串表示成功，否则是错误信息；前端不阻塞等待，落库失败不影响提醒本身。
// 落库成功后，本次真正新增的信号会进入 OKX 信号自动交易调度（模拟盘，开关在设置页）；
// 定制版本（-tags custom）不含 OKX 功能，跳过该调度。
func (a *App) SaveSignalRecords(items []models.SignalRecord) string {
	news, err := data.NewSignalRecordService().SaveSignalRecords(items)
	if err != nil {
		logger.SugaredLogger.Errorf("SaveSignalRecords error:%s", err.Error())
		return err.Error()
	}
	if len(news) > 0 && !CustomBuild {
		data.DispatchOKXSignalTrades(news)
	}
	return ""
}

// GetSignalRecordPage 分页查询信号流水（支持代码/名称关键词与命中时刻区间）。
// 查询条件由前端组装，Go 侧只做条件拼装与分页。
func (a *App) GetSignalRecordPage(query models.SignalRecordQuery) models.SignalRecordPageData {
	page, err := data.NewSignalRecordService().GetSignalRecordPage(query)
	if err != nil {
		logger.SugaredLogger.Errorf("GetSignalRecordPage error:%s", err.Error())
		return models.SignalRecordPageData{List: []models.SignalRecord{}}
	}
	return *page
}

// ClearSignalRecords 清空信号流水。source=crypto/stock 时只清对应来源（股票监控与
// OKX 加密监控两套引擎各自的清空互不影响），空串清全部。
func (a *App) ClearSignalRecords(source string) string {
	if err := data.NewSignalRecordService().ClearSignalRecords(source); err != nil {
		logger.SugaredLogger.Errorf("ClearSignalRecords error:%s", err.Error())
		return err.Error()
	}
	return ""
}

// GetSignalStats 统计区间内按买卖点信号操作下来的收益与胜率（整体 + 按股票）。
// 区间由前端按「今日」算好（K 线时间的 Unix 秒），Go 侧只做配对与汇总。
func (a *App) GetSignalStats(query models.SignalStatQuery) models.SignalStatResult {
	res, err := data.NewSignalRecordService().GetSignalStats(query)
	if err != nil {
		logger.SugaredLogger.Errorf("GetSignalStats error:%s", err.Error())
		return models.SignalStatResult{Stocks: []models.SignalStatItem{}}
	}
	return *res
}

// GetSignalTradePage 分页查询 OKX 信号模拟交易流水（面板「模拟交易」页签）。
// 定制版本（-tags custom）不含 OKX 功能，前端不展示该页签，这里直接返回空页。
func (a *App) GetSignalTradePage(query models.SignalTradeQuery) models.SignalTradePageData {
	if CustomBuild {
		return models.SignalTradePageData{List: []models.SignalTradeRecord{}}
	}
	page, err := data.NewOKXSignalTradeService().GetSignalTradePage(query)
	if err != nil {
		logger.SugaredLogger.Errorf("GetSignalTradePage error:%s", err.Error())
		return models.SignalTradePageData{List: []models.SignalTradeRecord{}}
	}
	return *page
}

// ClearSignalTradeRecords 清空 OKX 信号自动交易流水；env 为「模拟盘」/「实盘」时只清该环境。
// 定制版本（-tags custom）不含 OKX 功能，直接返回成功（无可清数据）。
func (a *App) ClearSignalTradeRecords(env string) string {
	if CustomBuild {
		return ""
	}
	if err := data.NewOKXSignalTradeService().ClearSignalTradeRecords(env); err != nil {
		logger.SugaredLogger.Errorf("ClearSignalTradeRecords error:%s", err.Error())
		return err.Error()
	}
	return ""
}
