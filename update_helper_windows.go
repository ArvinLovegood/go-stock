//go:build windows
// +build windows

package main

import (
	"errors"

	"go-stock/backend/logger"
	"os"
	"unsafe"

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
