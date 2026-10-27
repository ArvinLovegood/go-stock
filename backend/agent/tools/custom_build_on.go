//go:build custom

package tools

// CustomBuild 是否为定制版本编译（wails build -tags custom 开启）。
// 定制版本不包含 OKX/合约相关功能：智能体数据工具注册表中排除币安合约与 OKX 交易工具。
const CustomBuild = true
