package data

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// @Author spark
// @Date 2026/7/11
// @Desc 全局 Wails 应用上下文，供 AI 工具 handler（Path A 的 OpenAi.ctx 和 Path B 的 eino wrapper）
//       在修改分组/概念后向前端推送 stockDataChanged 事件，触发前端刷新 codeToConceptNames / codeToGroupNames 缓存。
//       app_windows.go / app_linux.go / app_darwin.go 在 startup 中调用 SetAppCtx(ctx) 设置。
// -----------------------------------------------------------------------------------

var (
	appCtx   context.Context
	appCtxMu sync.RWMutex
)

// SetAppCtx 设置全局 Wails 应用上下文（app startup 时调用）
func SetAppCtx(ctx context.Context) {
	appCtxMu.Lock()
	appCtx = ctx
	appCtxMu.Unlock()
}

// EmitStockDataChanged 向前端推送「股票数据已变更」事件，触发前端刷新分组/概念缓存。
// 安全调用：AppCtx 未设置时静默跳过（如测试环境）。异步 emit 避免阻塞工具 handler。
func EmitStockDataChanged() {
	appCtxMu.RLock()
	ctx := appCtx
	appCtxMu.RUnlock()
	if ctx == nil {
		return
	}
	go runtime.EventsEmit(ctx, "stockDataChanged", "")
}
