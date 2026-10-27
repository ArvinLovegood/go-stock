//go:build custom

package data

// CustomBuild 是否为定制版本编译（wails build -tags custom 开启）。
// 定制版本不包含 OKX/合约相关功能：AI 工具注册表中排除币安/Bitget 合约与 OKX 交易工具，
// 上层（package main）的 OKX 绑定与信号自动交易调度在定制版本中均已桩化/跳过。
const CustomBuild = true
