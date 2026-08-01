//go:build windows

package tools

import (
	"os/exec"
	"syscall"
)

// applyHiddenWindow 在 Windows 平台隐藏子进程控制台窗口。
//
// 背景：go-stock 为 Wails GUI 应用（无控制台宿主）。当通过 exec.Command 启动
// powershell.exe 等控制台子进程时，Windows 会自动为子进程分配一个新的控制台
// 窗口，导致 Agent 执行 execute 工具时弹出可见的 PowerShell 窗口，干扰用户。
//
// 实现两个标志，双保险：
//   - CREATE_NO_WINDOW (0x08000000)：不创建任何控制台窗口（推荐方式，
//     比纯 HideWindow 更彻底，子进程的 GetConsoleWindow 返回 NULL）
//   - HideWindow=true：若进程已有控制台（罕见），将其隐藏
//
// 二者叠加可覆盖所有创建控制台的路径。
func applyHiddenWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
