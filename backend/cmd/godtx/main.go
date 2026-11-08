package main

import (
	"github.com/bensema/gotdx"
)

func main() {
	// 新版 gotdx 已移除 WithAutoSelectFastest，连接池默认按分钟测速自动切换最快节点
	gotdx.New()
}
