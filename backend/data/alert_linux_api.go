//go:build linux
// +build linux

package data

import (
	"go-stock/backend/logger"

	"github.com/gen2brain/beeep"
)

// AlertWindowsApi Linux 实现：通过 beeep（D-Bus/notify-send）发送桌面通知。
// 类型名与 Windows/macOS 保持一致，供 app.go 等平台无关代码统一调用。
type AlertWindowsApi struct {
	AppID string
	// 窗口标题
	Title string
	// 窗口内容
	Content string
	// 窗口图标
	Icon string
}

func NewAlertWindowsApi(AppID string, Title string, Content string, Icon string) *AlertWindowsApi {
	return &AlertWindowsApi{
		AppID:   AppID,
		Title:   Title,
		Content: Content,
		Icon:    Icon,
	}
}

func (a AlertWindowsApi) SendNotification() bool {
	if GetSettingConfig().LocalPushEnable == false {
		return false
	}

	// 无通知守护进程（如最小化安装的发行版）时 beeep 会返回错误，仅记录日志
	if err := beeep.Notify(a.Title, a.Content, a.Icon); err != nil {
		logger.SugaredLogger.Warnf("发送桌面通知失败: %v", err)
		return false
	}
	return true
}
