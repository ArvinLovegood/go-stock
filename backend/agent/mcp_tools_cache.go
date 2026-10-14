package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
)

// MCP 工具客户端全局缓存。
//
// 背景：getMCPTools 原先每个问题都为每个启用的 MCP 服务器重建客户端
// （CreateMCPClient + Initialize 握手 + tools/list 拉取），实测 9 个服务器
// 串行初始化约 10s，且 React/PlanExecute/DeepAgents 每轮都付这笔成本。
//
// 缓存策略：按服务器 ID 缓存工具列表，两级失效条件——
//  1. 配置指纹（URL/Headers/Type/Command/Args/Enable/Status/UpdatedAt）：
//     任何写操作（增删改/启停/测试连接）都会刷新 gorm 的 UpdatedAt，
//     指纹随下次 DB 查询自然变化，无需跨包通知，规避 agent↔data 循环依赖。
//  2. TTL（mcpToolsCacheTTL）：兜底远端工具列表变化而本地配置未变的场景。
//
// 并发安全：缓存表用 RWMutex 保护；重建按服务器粒度加锁（singleflight），
// 全局写锁仅在结果写回时短暂持有，网络 IO 不阻塞其他服务器的命中路径。

const (
	// mcpToolsCacheTTL 缓存有效期。远端 MCP 服务器更新工具而本地配置未变时，
	// 最迟一个 TTL 周期后重建。
	mcpToolsCacheTTL = 10 * time.Minute

	// mcpToolsStaleCloseDelay 旧客户端延迟关闭时间。缓存的工具对象内部持有
	// 客户端引用，仍被进行中的 Agent run 使用；替换后延迟关闭，超过任何单次
	// run 的生命周期（agent 总超时 600s），确保无进行中调用被中断。
	mcpToolsStaleCloseDelay = 12 * time.Minute

	// mcpLoadFailBackoff 加载失败后的退避窗口。授权失败/远端 4xx 这类持续性
	// 错误重试也不会成功，不退避的话每个问题都会白付一次建连超时。
	mcpLoadFailBackoff = 5 * time.Minute
)

type mcpToolsCacheEntry struct {
	tools       []tool.BaseTool
	cli         closer     // 重建成功后延迟关闭的旧连接（stdio 是长连接进程，不关会泄漏）
	fingerprint string     // 服务器配置指纹，配置变化即失效
	fetchedAt   time.Time  // 拉取时间，超过 TTL 失效
	failedAt    time.Time  // 最近一次加载失败时间，退避窗口内不重试（零值=无失败）
	lastUsedAt  atomic.Int64 // 最近命中时间（UnixNano），读锁内更新故用原子写
}

type closer interface {
	Close() error
}

var (
	mcpToolsCacheMu sync.RWMutex
	mcpToolsCache   = make(map[uint]*mcpToolsCacheEntry)

	// mcpServerRebuildLocks 按服务器粒度的重建锁。网络 IO 不再持有全局写锁，
	// 不同服务器可并行重建（loadMCPToolsForServers 并发下发），同一服务器的
	// 并发请求仍串行化（singleflight 语义保留）。
	mcpServerRebuildLocks sync.Map // uint -> *sync.Mutex
)

func mcpServerRebuildLock(id uint) *sync.Mutex {
	v, _ := mcpServerRebuildLocks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// mcpServerFingerprint 生成服务器配置指纹。任一字段变化（含 gorm 自动维护的
// UpdatedAt）都会使指纹变化，触发缓存重建。
func mcpServerFingerprint(s *models.MCPServer) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%v|%s|%d",
		s.URL, s.Headers, s.Type, s.Command, s.Args, s.Name, s.Enable, s.Status, s.UpdatedAt.UnixNano())
}

// getMCPToolsForServer 返回指定服务器的工具列表（带缓存）。
// 服务器记录来自每次调用方的 DB 查询（廉价本地查询），指纹基于该记录计算，
// 因此配置变化在下一次问题时自动失效，无需主动通知。
//
// 失败退避：加载失败（授权失败/远端 4xx/建连超时）后 mcpLoadFailBackoff 窗口内
// 直接返回空，不再反复重试拖慢每轮问答；配置变化（指纹变化）会立即走出退避。
func getMCPToolsForServer(ctx context.Context, server *models.MCPServer) []tool.BaseTool {
	if server == nil || server.URL == "" {
		return nil
	}

	fp := mcpServerFingerprint(server)
	now := time.Now()

	// 快路径：读锁检查命中（绝大多数请求走此分支，微秒级返回）
	mcpToolsCacheMu.RLock()
	entry, ok := mcpToolsCache[server.ID]
	if ok && entry.fingerprint == fp {
		entry.lastUsedAt.Store(now.UnixNano())
		if entry.failedAt.IsZero() {
			if now.Sub(entry.fetchedAt) < mcpToolsCacheTTL {
				toolsOut := entry.tools
				mcpToolsCacheMu.RUnlock()
				return toolsOut
			}
		} else if now.Sub(entry.failedAt) < mcpLoadFailBackoff {
			// 上次加载失败且仍在退避窗口内：不重试，直接返回空
			mcpToolsCacheMu.RUnlock()
			return nil
		}
	}
	mcpToolsCacheMu.RUnlock()

	// 慢路径：按服务器粒度加锁后双重检查，再执行建连。
	// 同一服务器并发只重建一次；不同服务器互不阻塞，可被上层并行调度。
	rebuildMu := mcpServerRebuildLock(server.ID)
	rebuildMu.Lock()
	defer rebuildMu.Unlock()

	mcpToolsCacheMu.RLock()
	entry, ok = mcpToolsCache[server.ID]
	mcpToolsCacheMu.RUnlock()
	if ok && entry.fingerprint == fp {
		entry.lastUsedAt.Store(time.Now().UnixNano())
		if entry.failedAt.IsZero() {
			if time.Since(entry.fetchedAt) < mcpToolsCacheTTL {
				return entry.tools
			}
		} else if time.Since(entry.failedAt) < mcpLoadFailBackoff {
			return nil
		}
	}

	newTools, newCli := loadMCPToolsForServerFn(ctx, server)

	newEntry := &mcpToolsCacheEntry{
		tools:       newTools,
		cli:         newCli,
		fingerprint: fp,
		fetchedAt:   time.Now(),
	}
	if newTools == nil {
		// 加载失败（或服务器未暴露工具）：记入退避窗口
		newEntry.failedAt = newEntry.fetchedAt
	}

	mcpToolsCacheMu.Lock()
	old := mcpToolsCache[server.ID]
	mcpToolsCache[server.ID] = newEntry
	newEntry.lastUsedAt.Store(time.Now().UnixNano())
	mcpToolsCacheMu.Unlock()

	// 旧连接延迟关闭：等仍持有旧工具引用的 run 结束
	if old != nil && old.cli != nil {
		stale := old.cli
		go func() {
			time.Sleep(mcpToolsStaleCloseDelay)
			_ = stale.Close()
		}()
	}

	if newTools != nil {
		logger.SugaredLogger.Infof("MCP工具缓存重建 [%s]: %d 个工具 (fp=%s)", server.Name, len(newTools), shortFingerprint(fp))
	} else {
		logger.SugaredLogger.Warnf("MCP工具加载失败 [%s]，%.0f 分钟内跳过重试 (fp=%s)",
			server.Name, mcpLoadFailBackoff.Minutes(), shortFingerprint(fp))
	}
	return newTools
}

// touchMCPToolsCache 已并入 getMCPToolsForServer 的锁内更新，无需单独函数。

// loadMCPToolsForServerFn 可注入的加载函数（测试替换以隔离网络）。
var loadMCPToolsForServerFn = loadMCPToolsForServer

// loadMCPToolsForServer 建连并拉取工具列表（原 getMCPTools 单服务器逻辑）。
// 返回工具列表与客户端（客户端由缓存持有，用于后续延迟关闭）。
func loadMCPToolsForServer(ctx context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
	var mcpTools []tool.BaseTool

	cli, err := data.InitMCPClient(ctx, server)
	if err != nil {
		logger.SugaredLogger.Errorf("MCP客户端初始化失败 [%s]: %v", server.Name, err)
		return nil, nil
	}

	mcpToolList, err := einomcp.GetTools(ctx, &einomcp.Config{
		Cli:           cli,
		CustomHeaders: data.ResolveMCPHeaders(server),
	})
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP工具列表失败 [%s]: %v", server.Name, err)
		_ = cli.Close()
		return nil, nil
	}

	if len(mcpToolList) > 0 {
		logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具", server.Name, len(mcpToolList))
		mcpTools = append(mcpTools, mcpToolList...)
	} else {
		// 无工具：不留连接
		_ = cli.Close()
		return nil, nil
	}
	return mcpTools, cli
}

// InvalidateMCPToolsCache 失效指定服务器的工具缓存（下次问题重建）。
// id 为 0 时清空全部。供测试与外部显式刷新场景使用。
func InvalidateMCPToolsCache(id uint) {
	mcpToolsCacheMu.Lock()
	defer mcpToolsCacheMu.Unlock()
	if id == 0 {
		mcpToolsCache = make(map[uint]*mcpToolsCacheEntry)
		return
	}
	delete(mcpToolsCache, id)
}

// sweepStaleMCPToolsCache 清理超过 TTL 且近期未使用的缓存条目。
// 由 loadMCPToolsForServers 顺带触发，防止启用的服务器列表收缩后残留条目（含 stdio 连接）泄漏。
func sweepStaleMCPToolsCache(activeIDs map[uint]bool) {
	mcpToolsCacheMu.Lock()
	defer mcpToolsCacheMu.Unlock()
	now := time.Now()
	for id, entry := range mcpToolsCache {
		if activeIDs[id] {
			continue
		}
		// 不在当前启用列表（被删除/禁用）且已闲置超过关闭延迟，释放连接
		// 注意：lastUsedAt 存的是纳秒，须作为 time.Unix 的第二参数（nsec），
		// 误传第一参数（sec）会溢出成天文数字导致永不过期。
		lastUsed := time.Unix(0, entry.lastUsedAt.Load())
		if now.Sub(lastUsed) > mcpToolsStaleCloseDelay {
			if entry.cli != nil {
				stale := entry.cli
				go func() { _ = stale.Close() }()
			}
			delete(mcpToolsCache, id)
		}
	}
}

func shortFingerprint(fp string) string {
	if len(fp) > 16 {
		return fp[:16] + "..."
	}
	return fp
}
