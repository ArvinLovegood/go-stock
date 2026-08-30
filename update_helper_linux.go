//go:build linux
// +build linux

package main

import (
	"errors"
	"os"
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
