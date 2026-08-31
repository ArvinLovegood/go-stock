package agent

// long_term_memory.go — 长期记忆向量检索模块。
//
// 基于 chromem-go（纯 Go 嵌入式向量库，零 CGO 依赖）实现"归档时入库 + 自进化时检索"
// 两个接入点，将历史问答经验转化为可语义检索的知识库。
//
// 设计原则：
//   - 纯 Go：chromem-go 无任何第三方依赖，与现有 glebarez/sqlite 构建兼容
//   - 不阻断主流程：所有向量操作失败仅记录日志，自动降级到文件名扫描 fallback
//   - 懒加载：向量库在首次使用时初始化，未触发 AddMemory/SearchRelevant 时不消耗资源
//   - 异步入库：AddMemory 在独立 goroutine 中执行，不阻塞 archiveAnalysisReport
//   - 复用现有 OpenAI 兼容 API：通过 chromem-go 的 NewEmbeddingFuncOpenAICompat
//     直接调用 data.AIConfig 配置的供应商 /v1/embeddings 接口
//
// 接入点：
//   - archiveAnalysisReport（agent_api.go）：归档后调用 AddMemory
//   - buildSelfEvolutionPrompt（agent_self_evolution.go）：调用 SearchRelevant 替换文件名扫描
//
// 持久化路径：<exe_dir>/memory/.vectorstore/  （与 memory/YYYY-MM-DD/ 同级）

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/philippgille/chromem-go"
	"github.com/samber/lo"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

const (
	// longTermMemoryCollectionName 向量库 collection 名（存储所有历史问答）
	longTermMemoryCollectionName = "qa_history"
	// defaultEmbeddingModel 默认 embedding 模型名（OpenAI 标准名）
	// 多数 OpenAI 兼容供应商（含 OpenAI、Azure OpenAI、各类代理）支持此模型。
	// 若供应商使用其他模型名（如 Qwen 的 text-embedding-v3），可通过环境变量
	// GO_STOCK_EMBEDDING_MODEL 覆盖，或修改 AIConfig.EmbeddingModel 字段。
	defaultEmbeddingModel = "text-embedding-3-small"
	// envEmbeddingModel 环境变量名，用于覆盖默认 embedding 模型
	envEmbeddingModel = "GO_STOCK_EMBEDDING_MODEL"
	// longTermMemoryDefaultTopK 默认检索 Top-K
	longTermMemoryDefaultTopK = 5
	// chunkTargetRunes 切片目标字符数（按 rune 计，中文友好）
	chunkTargetRunes = 500
	// chunkOverlapRunes 切片重叠字符数（提升边界语义连续性）
	chunkOverlapRunes = 60
	// chunkMaxRunes 单片最大字符数（硬上限，防止超长段落）
	chunkMaxRunes = 1200
	// vectorStoreDirName 向量库持久化目录名（位于 memory 目录下）
	vectorStoreDirName = ".vectorstore"
)

// normalizeText 标准化文本换行符，兼容 Windows/旧Mac/Linux 及 Unicode 换行符。
//
// 处理内容：
//  1. 去 BOM 头（\uFEFF）
//  2. Unicode 行/段分隔符（\u2028/\u2029/\u0085）→ \n
//  3. Windows \r\n → \n
//  4. 残余 \r → \n（旧 Mac Classic）
//  5. 连续 3+ 空行压缩为 2 个（避免空段落过多影响切片）
//
// 用于 sliceForKB / sliceForEmbedding 入口，确保后续 strings.Split("\n\n") 正确分段。
func normalizeText(s string) string {
	// 1. 去 BOM
	s = strings.TrimPrefix(s, "\uFEFF")
	// 2. Unicode 换行符
	s = strings.ReplaceAll(s, "\u2028", "\n")   // 行分隔符
	s = strings.ReplaceAll(s, "\u2029", "\n\n") // 段落分隔符
	s = strings.ReplaceAll(s, "\u0085", "\n")   // NEL (Next Line)
	// 3. Windows \r\n → \n（必须在 \r 单独处理之前）
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// 4. 残余 \r → \n（旧 Mac Classic 用 \r 换行）
	s = strings.ReplaceAll(s, "\r", "\n")
	// 5. 压缩连续 3+ 空行为 2 个
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// longTermMemoryStore 单例向量库实例（懒加载）
var (
	longTermMemoryMu   sync.Mutex
	longTermMemoryDB   *chromem.DB
	longTermMemoryColl *chromem.Collection
	longTermMemoryErr  error // 初始化失败原因（用于日志与降级判断）
	longTermMemoryInit bool  // 是否已尝试初始化（失败后允许重试）
)

// MemoryRecall 单条历史经验检索结果
type MemoryRecall struct {
	Question   string  `json:"question"`   // 用户原始问题
	Reply      string  `json:"reply"`      // AI 回复片段（已切片，可能为整段或子段）
	Mode       string  `json:"mode"`       // Agent 模式（react/plan_execute/deepagents）
	Date       string  `json:"date"`       // 归档日期 YYYY-MM-DD
	ReportPath string  `json:"reportPath"` // 归档报告文件路径（便于追溯全文）
	Similarity float32 `json:"similarity"` // 与查询的余弦相似度 [-1, 1]
}

// initLongTermMemoryStore 懒加载初始化向量库。
//
// 初始化流程：
//  1. 解析持久化路径 <exe_dir>/memory/.vectorstore/
//  2. 从 data.GetSettingConfig() 选取首个可用的 AIConfig（ApiKey + BaseUrl 非空）
//  3. 构造 OpenAI 兼容 embedding 函数（chromem-go 内置）
//  4. 创建或读取持久化 collection
//
// 任意步骤失败均记录日志并设置 longTermMemoryErr，调用方据此判断是否降级。
// 该函数幂等，多次调用只初始化一次。
func initLongTermMemoryStore() {
	longTermMemoryMu.Lock()
	defer longTermMemoryMu.Unlock()
	// 已成功初始化则跳过
	if longTermMemoryDB != nil && longTermMemoryColl != nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			longTermMemoryErr = fmt.Errorf("panic: %v", r)
			logger.SugaredLogger.Errorf("initLongTermMemoryStore panic: %v", r)
		}
	}()

	rootDir := deepAgentRootDir()
	if rootDir == "" || rootDir == "." {
		longTermMemoryErr = fmt.Errorf("deepAgentRootDir 返回空或当前目录")
		return
	}
	storePath := filepath.Join(rootDir, memoryDirName, vectorStoreDirName)

	// 创建持久化 DB（目录不存在时 chromem-go 会自动创建）
	db, err := chromem.NewPersistentDB(storePath, false /* compress */)
	if err != nil {
		longTermMemoryErr = fmt.Errorf("创建持久化向量库失败: %w", err)
		logger.SugaredLogger.Errorf("长期记忆向量库初始化失败: %v (path=%s)", err, storePath)
		return
	}

	// 选取可用的 AIConfig 用于构造 embedding 函数
	embedFunc, aiCfgInfo, err := buildEmbeddingFunc()
	if err != nil {
		longTermMemoryErr = fmt.Errorf("构造 embedding 函数失败: %w", err)
		logger.SugaredLogger.Warnf("长期记忆向量库 embedding 未就绪: %v (将降级到文件名扫描)", err)
		return
	}

	// 创建或读取 collection（metadata 留空，所有文档统一存 qa_history）
	coll, err := db.GetOrCreateCollection(longTermMemoryCollectionName, nil, embedFunc)
	if err != nil {
		longTermMemoryErr = fmt.Errorf("创建 collection 失败: %w", err)
		logger.SugaredLogger.Errorf("长期记忆向量库 collection 创建失败: %v", err)
		return
	}

	longTermMemoryDB = db
	longTermMemoryColl = coll
	longTermMemoryErr = nil
	longTermMemoryInit = true
	logger.SugaredLogger.Infof("长期记忆向量库已就绪: path=%s collection=%s docs=%d embedding=%s",
		storePath, longTermMemoryCollectionName, coll.Count(), aiCfgInfo)
}

// resetLongTermMemoryStore 重置长期记忆向量库单例（切换向量服务后调用）。
//
// 背景：chromem-go 的 GetCollection 在 collection 已加载且 embed 非空时，
// 会忽略新传入的 embeddingFunc——单例一旦用旧配置初始化，配置变更后本会话内
// 永远不会生效。重置后下次 initLongTermMemoryStore 会以新配置重建；
// DB 为持久化存储（NewPersistentDB），重建时自动从磁盘重新加载文档，
// 知识库（共用该 DB）的数据不受影响。
func resetLongTermMemoryStore() {
	longTermMemoryMu.Lock()
	defer longTermMemoryMu.Unlock()
	longTermMemoryDB = nil
	longTermMemoryColl = nil
	longTermMemoryErr = nil
	longTermMemoryInit = false
}

// buildEmbeddingFunc 从 data.AIConfig 构造 OpenAI 兼容 embedding 函数（长期记忆专用）。
//
// 选取规则：
//   - 若 Settings.LongTermMemoryAiConfigId > 0：用指定的 AIConfig（用户在设置中明确指定）
//   - 否则走自动模式（aiConfigID=0）：优先 ModelType=embedding 的服务
//
// 模型名优先级：env GO_STOCK_EMBEDDING_MODEL > (type=embedding 时 ModelName) > AIConfig.EmbeddingModel > default
//
// 返回 (embeddingFunc, 配置摘要, error)。配置摘要用于日志，格式如 "aiConfig#3:baseUrl=model"。
func buildEmbeddingFunc() (chromem.EmbeddingFunc, string, error) {
	// 读取长期记忆向量服务设置（用户可在知识库页指定）
	if cfg := data.GetSettingConfig(); cfg != nil && cfg.Settings != nil && cfg.LongTermMemoryAiConfigId > 0 {
		return buildEmbeddingFuncWith(uint(cfg.LongTermMemoryAiConfigId), "")
	}
	return buildEmbeddingFuncWith(0, "")
}

// buildEmbeddingFuncWith 根据指定的 AIConfig ID 与模型名构造 embedding 函数（支持 KB 级覆盖）。
//
// 参数：
//   - aiConfigID: 指定 AIConfig ID。<=0 时走全局默认（取首个 ApiKey+BaseUrl 非空的配置）
//   - embeddingModel: 指定模型名。为空时按优先级解析：env > AIConfig.EmbeddingModel > default
//
// 返回 (embeddingFunc, 配置摘要, error)。配置摘要用于日志，格式如 "aiConfig#3:baseUrl=model"。
//
// 用于自定义知识库场景：每个 KB 可绑定独立的 AI 服务与向量模型，
// 避免不同 KB 共用同一 embedding 维度导致的检索异常。
func buildEmbeddingFuncWith(aiConfigID uint, embeddingModel string) (chromem.EmbeddingFunc, string, error) {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return nil, "", fmt.Errorf("未配置任何 AIConfig")
	}

	var aiConfig *data.AIConfig
	if aiConfigID > 0 {
		// 精确匹配指定 ID（用于 KB 绑定特定 AI 服务）
		var found bool
		aiConfig, found = lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return item != nil && item.ID == aiConfigID
		})
		if !found || aiConfig == nil {
			return nil, "", fmt.Errorf("未找到 AIConfig ID=%d", aiConfigID)
		}
		if aiConfig.ApiKey == "" || aiConfig.BaseUrl == "" {
			return nil, "", fmt.Errorf("AIConfig %q (ID=%d) 的 ApiKey 或 BaseUrl 为空", aiConfig.Name, aiConfigID)
		}
	} else {
		// 全局默认（自动模式）：优先选 ModelType="embedding" 的服务（明确是向量模型服务），
		// 其次选配置了 EmbeddingModel 的对话服务，最后回退到首个 ApiKey+BaseUrl 非空的配置。
		aiConfig, _ = lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return item != nil && item.ApiKey != "" && item.BaseUrl != "" && item.ModelType == "embedding"
		})
		if aiConfig == nil {
			aiConfig, _ = lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
				return item != nil && item.ApiKey != "" && item.BaseUrl != "" && strings.TrimSpace(item.EmbeddingModel) != ""
			})
		}
		if aiConfig == nil {
			aiConfig, _ = lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
				return item != nil && item.ApiKey != "" && item.BaseUrl != ""
			})
			if aiConfig == nil {
				return nil, "", fmt.Errorf("没有可用的 AIConfig（需 ApiKey + BaseUrl 均非空）")
			}
			logger.SugaredLogger.Warnf("buildEmbeddingFuncWith: 无 ModelType=embedding 的服务，回退到 %q (ID=%d)，若该服务不支持 embedding 将失败",
				aiConfig.Name, aiConfig.ID)
		}
	}

	// 模型名解析：参数 > env > (type=embedding 时用 ModelName) > AIConfig.EmbeddingModel > default
	model := strings.TrimSpace(embeddingModel)
	if model == "" {
		if env := strings.TrimSpace(os.Getenv(envEmbeddingModel)); env != "" {
			model = env
		} else if aiConfig.ModelType == "embedding" && strings.TrimSpace(aiConfig.ModelName) != "" {
			// type=embedding 的服务，ModelName 即向量模型名
			model = strings.TrimSpace(aiConfig.ModelName)
		} else if em := strings.TrimSpace(aiConfig.EmbeddingModel); em != "" {
			model = em
		} else {
			model = defaultEmbeddingModel
		}
	}

	// chromem-go 的 NewEmbeddingFuncOpenAICompat 内部已实现：
	// - POST {baseURL}/embeddings  （注意：chromem-go 直接拼接 baseURL+"/embeddings"）
	// - Authorization: Bearer {apiKey}
	// - body: {"model": model, "input": text}
	// - 自动归一化向量
	// normalized 传 nil 让 chromem-go 自动检测并归一化（OpenAI 兼容供应商默认返回归一化向量）

	// trim BaseUrl 尾部斜杠，避免 chromem-go 拼接出双斜杠 URL（如 .../v1//embeddings）
	baseUrl := strings.TrimRight(aiConfig.BaseUrl, "/")

	embedFunc := chromem.NewEmbeddingFuncOpenAICompat(baseUrl, aiConfig.ApiKey, model, nil)

	summary := fmt.Sprintf("aiConfig#%d:%s=%s", aiConfig.ID, baseUrl, model)
	logger.SugaredLogger.Infof("buildEmbeddingFuncWith: 构造 embedding 函数: aiConfig=%q(ID=%d) baseUrl=%s model=%s",
		aiConfig.Name, aiConfig.ID, baseUrl, model)
	return embedFunc, summary, nil
}

// AddMemory 将一次问答经验切片后写入向量库。
//
// 切片策略：按段落（\n\n）切分，单段过长时按 chunkTargetRunes 二次切分并保留 chunkOverlapRunes 重叠。
// 每个切片作为一个 Document 入库，metadata 含 {question, mode, date, report_path, chunk_index, total_chunks}。
//
// 调用约定：
//   - 异步执行（内部 goroutine），调用方立即返回
//   - 失败仅记日志，不影响主流程
//   - reportPath 可为空（仅用于追溯，不影响检索）
//   - userKey 为空时按机器维度处理（CurrentUserKey("")），写入 metadata "user" 供按用户过滤
func AddMemory(question, response string, mode Mode, reportPath, userKey string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("AddMemory panic: %v", r)
		}
	}()

	if strings.TrimSpace(question) == "" || strings.TrimSpace(response) == "" {
		return
	}
	if userKey == "" {
		userKey = CurrentUserKey("")
	}

	// 异步入库：不阻塞 archiveAnalysisReport
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := addMemorySync(ctx, question, response, mode, reportPath, userKey); err != nil {
			logger.SugaredLogger.Warnf("AddMemory 入库失败（不影响主流程）: %v", err)
		}
	}()
}

// addMemorySync 同步执行切片+入库逻辑（供 AddMemory 的 goroutine 调用）。
func addMemorySync(ctx context.Context, question, response string, mode Mode, reportPath, userKey string) error {
	initLongTermMemoryStore()
	if longTermMemoryColl == nil {
		return fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	chunks := sliceForEmbedding(question, response)
	if len(chunks) == 0 {
		return nil
	}

	date := time.Now().Format("2006-01-02")
	modeStr := string(mode)
	total := len(chunks)

	// 串行写入（chromem-go 内部已有锁；串行是为了让持久化文件按顺序落盘，便于排查）
	// 文档 ID 采用 日期+时间戳+chunk序号，确保同一问题多次归档不会互相覆盖
	ts := time.Now().Unix()
	for i, chunk := range chunks {
		docID := fmt.Sprintf("%s_%d_%02d", date, ts, i)

		metadata := map[string]string{
			"question":     truncateForLog(question, 300),
			"mode":         modeStr,
			"date":         date,
			"report_path":  reportPath,
			"chunk_index":  fmt.Sprintf("%d", i),
			"total_chunks": fmt.Sprintf("%d", total),
			"user":         userKey,
		}

		doc := chromem.Document{
			ID:       docID,
			Metadata: metadata,
			Content:  chunk,
			// Embedding 留空，由 collection.embed 自动生成
		}

		if err := longTermMemoryColl.AddDocument(ctx, doc); err != nil {
			return fmt.Errorf("写入 chunk %d/%d 失败: %w", i, total, err)
		}
	}

	logger.SugaredLogger.Infof("长期记忆入库完成: question=%q chunks=%d mode=%s",
		truncateForLog(question, 60), total, modeStr)
	return nil
}

// sliceForEmbedding 将 (question, response) 切分为适合 embedding 的文本片段。
//
// 切分规则：
//  1. 标准化换行符（兼容 Windows/Mac/Linux/Unicode）
//  2. 用 go-chunker Markdown 策略递归切分 response：标题 → 段落 → 句号 → 空格
//  3. 每片目标 ~500 字符，重叠 60 字符，过滤 <20 字的过短片段
//  4. 每个最终片段前缀加 "Q: {question}\nA: "，让 embedding 同时捕获问题语义
//
// 返回切片后的字符串数组。输入为空时返回 nil。
func sliceForEmbedding(question, response string) []string {
	q := strings.TrimSpace(question)
	r := strings.TrimSpace(response)
	if q == "" || r == "" {
		return nil
	}
	r = normalizeText(r)
	chunks, err := chunkText(r)
	if err != nil {
		logger.SugaredLogger.Warnf("sliceForEmbedding: chunkText 失败，回退到整体返回: %v", err)
		chunks = []string{r}
	}
	if len(chunks) == 0 {
		return nil
	}
	// 每个片段前缀加问题，让 embedding 同时捕获问题与回复的语义
	prefix := "Q: " + q + "\nA: "
	result := make([]string, 0, len(chunks))
	for _, c := range chunks {
		result = append(result, prefix+c)
	}
	return result
}

// SearchRelevant 检索与查询语义相关的历史问答经验。
//
// 参数：
//   - query: 当前用户问题（或问题摘要）
//   - topK: 返回的最大结果数（<=0 时使用默认值 5）
//   - userKey: 用户标识。非空时只检索该用户的记忆，不再自动回退到其他用户的私有记忆；
//     为空时才表示调用方明确请求全库检索。
//
// 返回按相似度降序排列的 MemoryRecall 列表。向量库未初始化或检索失败时返回 nil，
// 调用方可据此降级到文件名扫描 fallback。
//
// 注意：同一问题可能切分成多个 chunk 入库，这里按 question 字段去重，
// 保留相似度最高的那条 chunk 作为代表。
func SearchRelevant(ctx context.Context, query string, topK int, userKey string) []MemoryRecall {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("SearchRelevant panic: %v", r)
		}
	}()

	if strings.TrimSpace(query) == "" {
		return nil
	}
	if topK <= 0 {
		topK = longTermMemoryDefaultTopK
	}

	initLongTermMemoryStore()
	if longTermMemoryColl == nil {
		return nil
	}
	if longTermMemoryColl.Count() == 0 {
		return nil
	}

	// 用户记忆严格隔离。无结果时返回空，由调用方决定是否检索公共知识。
	if userKey != "" {
		return searchRelevantFiltered(ctx, query, topK, map[string]string{"user": userKey})
	}
	return searchRelevantFiltered(ctx, query, topK, nil)
}

// SearchRelevantGlobal 显式检索全局长期记忆。
// 只有公共知识检索、迁移或管理场景应使用它；普通对话应使用 SearchRelevant 并传入 userKey。
func SearchRelevantGlobal(ctx context.Context, query string, topK int) []MemoryRecall {
	return SearchRelevant(ctx, query, topK, "")
}

// searchRelevantFiltered 在可选 where 过滤下执行语义检索并去重。
func searchRelevantFiltered(ctx context.Context, query string, topK int, where map[string]string) []MemoryRecall {
	// 多检索一些（Top-K*3），便于按 question 去重后仍有足够结果
	fetchN := topK * 3
	if fetchN < topK {
		fetchN = topK
	}
	// chromem-go 的 Query 要求 nResults <= 文档数，否则报错；自动截断
	if docCount := longTermMemoryColl.Count(); fetchN > docCount {
		fetchN = docCount
	}

	results, err := longTermMemoryColl.Query(ctx, query, fetchN, where, nil)
	if err != nil {
		if strings.Contains(err.Error(), "vectors must have the same length") {
			// 维度不一致：曾切换过向量服务（embedding 模型），旧向量与新模型维度不同
			logger.SugaredLogger.Errorf("长期记忆检索失败: 向量维度不一致（疑似切换过向量服务）。"+
				"请清空 %s 目录下的向量库后重启重建 (query=%q)", vectorStoreDirName, truncateForLog(query, 60))
		} else {
			logger.SugaredLogger.Warnf("长期记忆检索失败: %v (query=%q)", err, truncateForLog(query, 60))
		}
		return nil
	}
	if len(results) == 0 {
		return nil
	}

	// 按 question 去重，保留相似度最高的
	seen := make(map[string]int, len(results)) // question -> index in dedup
	dedup := make([]MemoryRecall, 0, len(results))
	for _, r := range results {
		q := r.Metadata["question"]
		if q == "" {
			q = "(未记录问题)"
		}
		recall := MemoryRecall{
			Question:   q,
			Reply:      truncateForLog(r.Content, 400),
			Mode:       r.Metadata["mode"],
			Date:       r.Metadata["date"],
			ReportPath: r.Metadata["report_path"],
			Similarity: r.Similarity,
		}
		if idx, ok := seen[q]; ok {
			// 已存在，仅当相似度更高时替换
			if recall.Similarity > dedup[idx].Similarity {
				dedup[idx] = recall
			}
			continue
		}
		seen[q] = len(dedup)
		dedup = append(dedup, recall)
	}

	if len(dedup) > topK {
		dedup = dedup[:topK]
	}
	return dedup
}

// FormatMemoryRecall 将检索结果格式化为系统提示词可注入的文本。
//
// 输出格式示例：
//
//	历史相关经验（按相似度排序）:
//	1. [2026-01-15] [react] 相似度 0.82
//	   Q: 贵州茅台最近财报如何？
//	   A: 根据最新财报，贵州茅台 2024 年营收...
//	2. [2026-01-10] [deepagents] 相似度 0.71
//	   Q: 白酒板块今天为什么涨？
//	   A: ...
//
// 空列表返回空字符串。topN 用于限制展示条数（<=0 表示全部）。
func FormatMemoryRecall(recalls []MemoryRecall, topN int) string {
	if len(recalls) == 0 {
		return ""
	}
	if topN > 0 && len(recalls) > topN {
		recalls = recalls[:topN]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("历史相关经验（按相似度排序，共 %d 条）:\n", len(recalls)))
	for i, r := range recalls {
		sb.WriteString(fmt.Sprintf("%d. [%s] [%s] 相似度 %.2f\n", i+1, r.Date, r.Mode, r.Similarity))
		sb.WriteString(fmt.Sprintf("   Q: %s\n", truncateForLog(r.Question, 200)))
		// 回复片段去掉前缀 "Q: ...\nA: "，只保留 A 部分并截断
		reply := r.Reply
		if idx := strings.Index(reply, "\nA: "); idx >= 0 {
			reply = reply[idx+4:]
		}
		sb.WriteString(fmt.Sprintf("   A: %s\n", truncateForLog(reply, 300)))
		if r.ReportPath != "" {
			sb.WriteString(fmt.Sprintf("   报告: %s\n", r.ReportPath))
		}
	}
	sb.WriteString("\n请在回答时参考上述历史经验；若用户问题与历史相关可承接结论；若历史经验已过时请主动说明。\n")
	return sb.String()
}
