package agent

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

// 本文件实现 sysPrompt 的分层缓存，目标是减少每轮 ChatWithContext 的拼装开销：
//   - L1 进程级：静态规则段（强制规则/合规边界/错误恢复/并行引导/检索规范/任务规划/思考要求）
//     字符串为编译期常量，token 数用 sync.Once 启动时算一次
//   - L2 文件级：项目指令/用户偏好/SOUL.md/MEMORY.md/LEARNINGS.md，按文件 mtime 失效
//   - L3 短 TTL：buildAgentTimeContext 中的全球指数部分，30s TTL
//   - DB 查询缓存：GetPromptTemplateByID 按 ID 缓存 5 分钟
//
// 设计原则：语义等价（缓存命中与未命中的拼装结果完全相同）、失败静默降级、可一键关闭。
// 设置环境变量 GO_STOCK_NO_SYSPROMPT_CACHE=1 可禁用所有缓存，回到原始直接拼装路径。

// noSysPromptCache 用于一键关闭缓存（测试/排障用）。
var noSysPromptCache = os.Getenv("GO_STOCK_NO_SYSPROMPT_CACHE") == "1"

// =====================================================================
// L1：进程级静态规则段
// =====================================================================

// 静态规则段（放在时间上下文之前的两段：强制规则 + 合规边界）。
// 内容来自 agent_api.go 原 L205-221，原样搬移，不做任何修改。
const staticRulesHead = `

【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率等）——必须调用工具获取最新财报数据
3. 新闻资讯——必须调用工具获取最新新闻
4. 宏观经济数据——必须调用工具获取最新数据
任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息。如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"，绝不能编造数据。`

// 合规边界段，原样搬移自 agent_api.go L214-221。
const staticRulesCompliance = `

【合规边界】
- 仅提供数据分析与信息参考，不构成任何投资建议或买卖指令
- 不主动推荐具体个股，仅分析用户指定的标的
- 涉及盈亏预测、估值判断时必须附加风险提示
- 涉及 ST、*ST、退市风险股票时主动提示对应风险
- 回答末尾附加："以上分析基于公开数据，仅供参考，不构成投资建议"`

// 静态规则段（放在用户偏好之后的三段：错误恢复 + 并行引导 + 检索规范）。
// 原样搬移自 agent_api.go L234-256；新增「工具选择优先级」段。
const staticRulesTail = `

【错误恢复策略】
- 工具返回 status=empty：换数据源或换股票代码格式重试，最多 2 次
- 工具返回 status=error：检查参数格式（股票代码、日期等），若仍失败则告知用户并继续下一步
- 工具连续失败 3 次：停止当前任务，向用户报告具体问题
- 股票代码格式兼容：A股 600519/600519.SH/sh600519；港股 00700/00700.HK/hk00700；美股 AAPL/usAAPL/gb_aapl

【工具选择优先级】
- 内置数据工具（GetStockFundFlow/GetFuturesPosition/GetMACCapitalFlow 等 Get 前缀工具）与外部 MCP 工具功能看似重叠时，优先使用内置工具：内置工具针对本系统数据结构适配，参数简单（如品种代码 IF/IH/IC/IM）、返回格式稳定
- 用户点名指定工具名（如"调用 GetFuturesPosition"）时，必须精确调用该名称的内置工具，严禁替换为名称相似的外部 MCP 工具（如 ft_get_eastmoney_futures_position 等带服务器前缀的工具，它们的数据维度可能完全不同）
- 仅当内置工具列表中不存在对应能力时，才使用外部 MCP 工具
- MCP 工具返回空数据时，优先换用功能对应的内置工具重试，而非直接下"无数据"结论`

// 并行工具调用引导段，原样搬移自 agent_api.go L243-247。
const staticRulesParallel = `

【并行工具调用】
当需要查询多只股票的同类数据（如比较茅台和五粮液的财务指标、批量获取行业成分股行情）时，请在单次回复中并行发起多个 tool_calls，而非串行逐个调用。主流模型（OpenAI/Claude/DeepSeek/Qwen）均支持单次返回多个 tool_calls，可显著减少往返延迟。
注意：存在依赖关系的工具（如先查股票代码再查财务）仍需串行调用。`

// 检索结果引用规范段，原样搬移自 agent_api.go L250-256。
const staticRulesRetrieval = `

【检索结果引用规范】
- 回答基于知识库检索结果（SearchKnowledgeBase/SearchAllKnowledge/SearchLongTermMemory）时，标注来源（知识库名/文档名）与相似度
- 区分"背景知识"（公司基础资料、历史经验、知识库文档）与"实时数据"（行情、新闻、最新财务）的引用时效
- 实时数据以工具返回的 [as_of=时间戳] 为准，背景知识注明来源知识库/文档
- 引用历史经验时标注"历史经验"前缀，避免与实时数据混淆让用户误判时效`

// 任务规划要求段（仅 PlanExecute 模式），原样搬移自 agent_api.go L260-269。
const staticRulesPlanExecute = `

【任务规划要求】
分析复杂问题时，按以下步骤执行：
1. 先输出任务清单（编号 + 步骤描述 + 状态标记）
2. 每完成一步更新状态（pending → in_progress → completed）
3. 全部完成后输出总结
任务清单格式：
- [1] <步骤描述> [pending]
- [2] <步骤描述> [pending]`

// 思考要求段（仅 thinkingMode），原样搬移自 agent_api.go L274-282。
const staticRulesThinking = `

【思考要求】
回答前先进行内部推理：
1. 理解用户意图（查询、分析、比较、还是预测？）
2. 判断需要哪些数据（行情、财务、新闻、宏观？）
3. 制定工具调用顺序（先查什么，后查什么）
4. 预判可能的风险（数据缺失、市场异常、参数错误）
然后再调用工具和输出回答。`

// 预拼接的完整静态规则串（head + compliance），减少运行时字符串拼接。
var (
	staticHeadOnce   sync.Once
	staticHeadPrompt string
	staticHeadTokens int

	staticTailOnce   sync.Once
	staticTailPrompt string
	staticTailTokens int

	staticPlanExecuteOnce   sync.Once
	staticPlanExecutePrompt string
	staticPlanExecuteTokens int

	staticThinkingOnce   sync.Once
	staticThinkingPrompt string
	staticThinkingTokens int
)

// getStaticHeadPrompt 返回"强制规则 + 合规边界"两段拼接及其 token 数。
// 进程级缓存，永不失效。
func getStaticHeadPrompt() (string, int) {
	if noSysPromptCache {
		s := staticRulesHead + staticRulesCompliance
		return s, estimateTokens(s)
	}
	staticHeadOnce.Do(func() {
		staticHeadPrompt = staticRulesHead + staticRulesCompliance
		staticHeadTokens = estimateTokens(staticHeadPrompt)
	})
	return staticHeadPrompt, staticHeadTokens
}

// getStaticTailPrompt 返回"错误恢复 + 并行引导 + 检索规范"三段拼接及其 token 数。
func getStaticTailPrompt() (string, int) {
	if noSysPromptCache {
		s := staticRulesTail + staticRulesParallel + staticRulesRetrieval
		return s, estimateTokens(s)
	}
	staticTailOnce.Do(func() {
		staticTailPrompt = staticRulesTail + staticRulesParallel + staticRulesRetrieval
		staticTailTokens = estimateTokens(staticTailPrompt)
	})
	return staticTailPrompt, staticTailTokens
}

// getStaticPlanExecutePrompt 返回任务规划段及其 token 数。
func getStaticPlanExecutePrompt() (string, int) {
	if noSysPromptCache {
		return staticRulesPlanExecute, estimateTokens(staticRulesPlanExecute)
	}
	staticPlanExecuteOnce.Do(func() {
		staticPlanExecutePrompt = staticRulesPlanExecute
		staticPlanExecuteTokens = estimateTokens(staticPlanExecutePrompt)
	})
	return staticPlanExecutePrompt, staticPlanExecuteTokens
}

// getStaticThinkingPrompt 返回思考要求段及其 token 数。
func getStaticThinkingPrompt() (string, int) {
	if noSysPromptCache {
		return staticRulesThinking, estimateTokens(staticRulesThinking)
	}
	staticThinkingOnce.Do(func() {
		staticThinkingPrompt = staticRulesThinking
		staticThinkingTokens = estimateTokens(staticThinkingPrompt)
	})
	return staticThinkingPrompt, staticThinkingTokens
}

// =====================================================================
// L2：文件级 mtime 缓存
// =====================================================================

// fileCacheEntry 单个文件的缓存条目。
type fileCacheEntry struct {
	modTime time.Time
	content string // 文件原始内容（未做 TrimSpace）
	tokens  int
}

// fileContentCache 文件内容缓存，按 path + mtime 失效。
// 并发安全，读多写少场景优化为 RWMutex。
type fileContentCache struct {
	mu      sync.RWMutex
	entries map[string]fileCacheEntry
	hits    int64
	misses  int64
}

var globalFileCache = &fileContentCache{entries: make(map[string]fileCacheEntry)}

// readCachedFile 读取文件内容并按 mtime 缓存。
// 返回 (content, tokens, hit)。文件不存在/读取失败返回 ("", 0, false)。
// content 为文件原始内容（含首尾空白），调用方按需 TrimSpace。
// 非 NotExist 的错误会记 warn 日志，便于排障（与原 loadSoul/loadMemory 行为一致）。
func (c *fileContentCache) readCachedFile(path string) (string, int, bool) {
	if path == "" {
		return "", 0, false
	}

	fi, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.SugaredLogger.Warnf("stat file failed: %v (path=%s)", err, path)
		}
		return "", 0, false
	}
	if fi.IsDir() {
		return "", 0, false
	}

	// 快路径：RLock 检查缓存
	c.mu.RLock()
	if e, ok := c.entries[path]; ok && e.modTime.Equal(fi.ModTime()) {
		c.mu.RUnlock()
		atomic.AddInt64(&c.hits, 1)
		return e.content, e.tokens, true
	}
	c.mu.RUnlock()

	// 慢路径：读文件并更新缓存
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.SugaredLogger.Warnf("read file failed: %v (path=%s)", err, path)
		}
		return "", 0, false
	}
	content := string(data)
	tokens := estimateTokens(content)

	c.mu.Lock()
	c.entries[path] = fileCacheEntry{modTime: fi.ModTime(), content: content, tokens: tokens}
	c.mu.Unlock()

	atomic.AddInt64(&c.misses, 1)
	return content, tokens, false
}

// InvalidateFileCache 失效指定路径的文件缓存。
// 在外部已知文件变更时调用（如用户编辑 user_profile.md 后）。
func InvalidateFileCache(path string) {
	globalFileCache.mu.Lock()
	delete(globalFileCache.entries, path)
	globalFileCache.mu.Unlock()
}

// InvalidateFileCacheByPrefix 失效指定前缀路径的所有缓存（如清理某个目录下所有文件）。
func InvalidateFileCacheByPrefix(prefix string) {
	globalFileCache.mu.Lock()
	for path := range globalFileCache.entries {
		if strings.HasPrefix(path, prefix) {
			delete(globalFileCache.entries, path)
		}
	}
	globalFileCache.mu.Unlock()
}

// FileCacheStats 返回缓存命中/未命中统计，用于监控。
func FileCacheStats() (hits, misses int64) {
	return atomic.LoadInt64(&globalFileCache.hits), atomic.LoadInt64(&globalFileCache.misses)
}

// readCachedFileTrimmed 读取文件并 TrimSpace，按 mtime 缓存。
// 大多数调用方都需要 TrimSpace，封装为公共 helper 减少重复代码。
func readCachedFileTrimmed(path string) (string, int, bool) {
	content, tokens, hit := globalFileCache.readCachedFile(path)
	if content == "" {
		return "", 0, hit
	}
	return strings.TrimSpace(content), tokens, hit
}

// =====================================================================
// L3：短 TTL 缓存（全球指数状态）
// =====================================================================

var (
	marketStatusMu     sync.Mutex
	marketStatusCache  string
	marketStatusExpire time.Time
)

const marketStatusTTL = 30 * time.Second

// getMarketStatusShortTTL 返回全球指数状态，30s TTL 缓存。
// 缓存未命中时调用 data.NewMarketNewsApi().GlobalStockIndexesReadable(30)。
// 失败返回空字符串（不阻断主流程）。
func getMarketStatusShortTTL() string {
	if noSysPromptCache {
		return strings.TrimSpace(data.NewMarketNewsApi().GlobalStockIndexesReadable(30))
	}

	marketStatusMu.Lock()
	defer marketStatusMu.Unlock()

	if marketStatusCache != "" && time.Now().Before(marketStatusExpire) {
		return marketStatusCache
	}

	status := strings.TrimSpace(data.NewMarketNewsApi().GlobalStockIndexesReadable(30))
	marketStatusCache = status
	marketStatusExpire = time.Now().Add(marketStatusTTL)
	return status
}

// =====================================================================
// DB 查询缓存：PromptTemplate
// =====================================================================

var (
	promptTemplateCache   = make(map[int]promptTemplateEntry)
	promptTemplateCacheMu sync.RWMutex
)

type promptTemplateEntry struct {
	content  string
	tokens   int
	loadedAt time.Time
}

const promptTemplateTTL = 5 * time.Minute

// getCachedPromptTemplate 按 ID 查询 PromptTemplate，5 分钟 TTL 缓存。
// 缓存未命中时回查 DB；DB 错误返回空字符串（与原行为一致）。
func getCachedPromptTemplate(id int) string {
	if id <= 0 {
		return ""
	}

	if noSysPromptCache {
		return data.NewPromptTemplateApi().GetPromptTemplateByID(id)
	}

	promptTemplateCacheMu.RLock()
	if e, ok := promptTemplateCache[id]; ok && time.Since(e.loadedAt) < promptTemplateTTL {
		promptTemplateCacheMu.RUnlock()
		return e.content
	}
	promptTemplateCacheMu.RUnlock()

	content := data.NewPromptTemplateApi().GetPromptTemplateByID(id)

	promptTemplateCacheMu.Lock()
	promptTemplateCache[id] = promptTemplateEntry{
		content:  content,
		tokens:   estimateTokens(content),
		loadedAt: time.Now(),
	}
	promptTemplateCacheMu.Unlock()

	return content
}

// getCachedPromptTemplateWithTokens 同时返回 token 数（缓存命中时零成本）。
func getCachedPromptTemplateWithTokens(id int) (string, int) {
	if id <= 0 {
		return "", 0
	}

	if noSysPromptCache {
		s := data.NewPromptTemplateApi().GetPromptTemplateByID(id)
		return s, estimateTokens(s)
	}

	promptTemplateCacheMu.RLock()
	if e, ok := promptTemplateCache[id]; ok && time.Since(e.loadedAt) < promptTemplateTTL {
		promptTemplateCacheMu.RUnlock()
		return e.content, e.tokens
	}
	promptTemplateCacheMu.RUnlock()

	content := data.NewPromptTemplateApi().GetPromptTemplateByID(id)
	tokens := estimateTokens(content)

	promptTemplateCacheMu.Lock()
	promptTemplateCache[id] = promptTemplateEntry{
		content:  content,
		tokens:   tokens,
		loadedAt: time.Now(),
	}
	promptTemplateCacheMu.Unlock()

	return content, tokens
}

// InvalidatePromptTemplateCache 失效指定 ID 的模板缓存。
// 在用户编辑/删除模板后调用。
func InvalidatePromptTemplateCache(id int) {
	promptTemplateCacheMu.Lock()
	delete(promptTemplateCache, id)
	promptTemplateCacheMu.Unlock()
}

// InvalidateAllPromptTemplateCache 失效所有模板缓存。
func InvalidateAllPromptTemplateCache() {
	promptTemplateCacheMu.Lock()
	promptTemplateCache = make(map[int]promptTemplateEntry)
	promptTemplateCacheMu.Unlock()
}

// =====================================================================
// 调试辅助：sysPrompt 拼装调试
// =====================================================================

// LogCacheStats 输出当前缓存统计到日志（debug 用）。
func LogCacheStats() {
	hits, misses := FileCacheStats()
	logger.SugaredLogger.Infof("sysprompt cache stats: fileCache hits=%d misses=%d", hits, misses)
}
