//go:build !windows

package tools

import "os/exec"

// applyHiddenWindow 在非 Windows 平台为空实现。
//
// Linux/macOS 通过 /bin/sh -c 启动的子进程本就不会弹出独立窗口
// （终端复用器/桌面环境不会为后台进程创建新窗口），无需特殊处理。
//
// 此文件存在的目的仅是提供与 streaming_shell_windows.go 相同的函数签名，
// 使 streaming_shell.go 在所有平台都能通过编译（构建标签互斥）。
func applyHiddenWindow(_ *exec.Cmd) {}
