//go:build custom

package main

// CustomBuild 是否为定制版本编译（wails build -tags custom 开启）。
// 定制版本行为：启动及定时任务不检查新版本、禁用自动更新下载与替换，前端隐藏"关于"菜单。
const CustomBuild = true
