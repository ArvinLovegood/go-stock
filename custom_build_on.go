//go:build custom

package main

import "errors"

// CustomBuild 是否为定制版本编译（wails build -tags custom 开启）。
// 定制版本行为：启动及定时任务不检查新版本、禁用自动更新下载与替换，前端隐藏"关于"菜单、
// 不包含 OKX/合约相关功能与设置（保留买卖点信号监控）。
const CustomBuild = true

// errCustomBuildNoOKX 定制版本不含 OKX/合约功能时，相关绑定桩返回的统一错误。
func errCustomBuildNoOKX() error {
	return errors.New("定制版本不包含 OKX/合约相关功能")
}
