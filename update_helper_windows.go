//go:build windows
// +build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"time"
	"unsafe"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"golang.org/x/sys/windows"
)

// ApplyMacUpdate 仅 macOS 实现（.app bundle 整体替换），其他平台为编译占位
func ApplyMacUpdate(zipPath string) error {
	return errors.New("unsupported platform")
}

func IsRunningAsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		logger.SugaredLogger.Errorf("AllocateAndInitializeSid error: %s", err.Error())
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	h, err := windows.GetCurrentProcess()
	if err != nil {
		return false
	}
	err = windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

func (a *App) RestartAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	shell32 := windows.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exePath)
	params, _ := windows.UTF16PtrFromString("")
	directory, _ := windows.UTF16PtrFromString("")

	shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(directory)),
		1,
	)

	os.Exit(0)
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
	cmd := exec.Command(exePath, buildRestartArgs()...)
	// GUI 程序重启时避免闪烁控制台窗口
	cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	if err = cmd.Start(); err != nil {
		logger.SugaredLogger.Errorf("restart app error: %s", err.Error())
		return err
	}
	// os.Exit(0) 会跳过 defer：必须先合并 WAL 并关闭数据库再退出，
	// 否则新进程启动时要全量扫描恢复巨型 WAL（实测 2.1GB 卡 db.Init 62 秒无窗口）
	db.Close(20 * time.Second)
	_ = logger.SugaredLogger.Sync()
	os.Exit(0)
	return nil
}

// waitForProcessExit 等待指定进程退出（超时后返回），用于重启时等待旧实例释放单实例锁
func waitForProcessExit(pid int, timeout time.Duration) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// 进程已退出或无法打开，无需等待
		return
	}
	defer windows.CloseHandle(h)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := windows.WaitForSingleObject(h, 2000)
		if err != nil || status == windows.WAIT_OBJECT_0 {
			return
		}
	}
}
