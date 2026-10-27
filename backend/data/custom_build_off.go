//go:build !custom

package data

// CustomBuild 是否为定制版本编译（默认关闭）。开启方式：wails build -tags custom
// 定制版本不包含 OKX/合约相关功能（AI 工具注册表中排除币安/Bitget 合约与 OKX 交易工具）。
const CustomBuild = false
