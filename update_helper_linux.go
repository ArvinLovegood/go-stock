//go:build linux
// +build linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// ApplyMacUpdate 仅 macOS 实现（.app bundle 整体替换），其他平台为编译占位
func ApplyMacUpdate(zipPath string) error {
	return errors.New("unsupported platform")
}

func IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}

func (a *App) RestartAsAdmin() error {
	return nil
}

// RestartApp 以当前用户权限重启应用，用于加载已替换完成的新版本。
// 通过 -wait-parent 参数让新进程等待当前进程退出后再启动（见 main.go），
// 避免新进程被 Wails SingleInstanceLock 当作第二实例而直接退出
func (a *App) RestartApp() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	// 停止定时任务，减少收尾期间的新写入
	if a.cron != nil {
		a.cron.Stop()
	}
	if err = exec.Command(exePath, buildRestartArgs()...).Start(); err != nil {
		logger.SugaredLogger.Errorf("restart app error: %s", err.Error())
		return err
	}
	// os.Exit(0) 会跳过 defer：必须先合并 WAL 并关闭数据库再退出，
	// 避免新进程启动时扫描恢复巨型 WAL 而长时间无响应
	db.Close(20 * time.Second)
	_ = logger.SugaredLogger.Sync()
	os.Exit(0)
	return nil
}

// waitForProcessExit 等待指定进程退出（超时后返回），用于重启时等待旧实例释放单实例锁
func waitForProcessExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// 信号 0 仅探测进程是否存在，返回 ESRCH 表示已退出
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
