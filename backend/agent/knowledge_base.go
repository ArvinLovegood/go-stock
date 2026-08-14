package agent

// knowledge_base.go — 自定义知识库向量管理模块。
//
// 在 chromem-go（已由 long_term_memory.go 集成）基础上扩展"多 collection"管理：
//   - 用户可按主题建立任意多个独立向量库（每个 collection 一个 KB）
//   - 支持将文本/文件切片后入库（复用 sliceForEmbedding 的切分思路，但去掉 Q/A 前缀）
//   - 支持按 KB 名称检索、列出文档、删除文档/KB
//
// 设计原则：
//   - 复用现有 chromem.DB 实例（同一 NewPersistentDB 可创建多个 collection），
//     避免重复构造 embedding 客户端，避免文件锁冲突
//   - KB 元信息（名称、描述、文档数、创建时间、文档索引）持久化到 JSON 文件，
//     与向量库同目录，便于排查
//   - 不阻断主流程：所有失败仅记日志，调用方拿到 error 自行决策
//   - 纯 Go：不引入新依赖；文件解析目前支持 .txt/.md，后续可扩展
//
// 持久化路径：
//   - 向量库：<exe_dir>/memory/.vectorstore/<collection_hash>/*.gob（由 chromem-go 管理）
//   - KB 元信息：<exe_dir>/memory/.vectorstore/.kb_meta.json（本模块管理）
//
// 文档索引说明：
//   chromem-go 0.7.0 的 Collection.documents 是私有字段，无公开 List/Iterate 方法。
//   为支持 ListDocumentsInKB，本模块在 KB 元信息 JSON 中额外维护一份文档索引
//   （docID → 来源/创建时间/切片序号），AddDocumentToKB 写入、DeleteDocumentFromKB 删除。
//   这会带来少量重复存储，但 KB 文档数通常 <10000，JSON 大小可控。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/philippgille/chromem-go"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

const (
	// kbMetaFileName KB 元信息持久化文件名（位于向量库目录下）
	kbMetaFileName = ".kb_meta.json"
	// kbNamePrefix KB collection 名称统一前缀，避免与 qa_history 等系统 collection 冲突
	// 实际 collection 名为 "kb_" + 用户传入的 name
	kbNamePrefix = "kb_"
	// kbDefaultTopK KB 检索默认 Top-K
	kbDefaultTopK = 5
	// kbMaxTopK KB 检索最大 Top-K
	kbMaxTopK = 20
)

// KnowledgeBaseInfo 知识库元信息（用于列表展示与持久化）
type KnowledgeBaseInfo struct {
	Name          string            `json:"name"`          // KB 名称（用户可读，不含前缀）
	Description   string            `json:"description"`   // KB 描述
	DocumentCount int               `json:"documentCount"` // 文档数（含切片；与 collection.Count() 一致）
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	Documents     []KBDocumentIndex `json:"documents,omitempty"` // 文档索引（按入库顺序）
	// AIConfigID 绑定的 AI 服务 ID（创建时指定，决定该 KB 用哪个供应商的 embedding 接口）。
	// 0 表示走全局默认逻辑（取首个 ApiKey+BaseUrl 非空的 AIConfig）。
	// 注意：embedding 配置在 KB 创建时绑定，之后不可修改（修改会导致新旧向量维度不一致）。
	AIConfigID uint `json:"aiConfigId"`
	// AIConfigName 绑定的 AI 服务名称（仅用于前端展示，不参与逻辑；后端从 AIConfigID 实时反查）
	AIConfigName string `json:"aiConfigName,omitempty"`
	// EmbeddingModel 该 KB 使用的向量模型名。
	// 为空时按优先级解析：env > AIConfig.EmbeddingModel > defaultEmbeddingModel
	EmbeddingModel string `json:"embeddingModel,omitempty"`
}

// KBDocumentIndex 文档索引条目（仅记录元数据，不存储内容）
type KBDocumentIndex struct {
	DocID       string    `json:"docId"`       // chromem Document ID
	Source      string    `json:"source"`      // 来源（文件名或 "inline"）
	ChunkIndex  int       `json:"chunkIndex"`  // 切片序号
	TotalChunks int       `json:"totalChunks"` // 该来源文件总切片数
	CreatedAt   time.Time `json:"createdAt"`   // 入库时间
}

// KnowledgeBaseDocument 知识库中文档的简要信息（用于列表展示）
type KnowledgeBaseDocument struct {
	ID             string            `json:"id"`         // 文档 ID
	Source         string            `json:"source"`     // 来源（文件名或 "inline"）
	ChunkIndex     int               `json:"chunkIndex"` // 切片序号
	TotalChunks    int               `json:"totalChunks"`
	CreatedAt      string            `json:"createdAt"`      // 入库时间 YYYY-MM-DD HH:MM:SS
	ContentPreview string            `json:"contentPreview"` // 内容预览（前 200 字）
	Metadata       map[string]string `json:"metadata"`
}

// KnowledgeBaseSearchResult 单条 KB 检索结果
type KnowledgeBaseSearchResult struct {
	KBName     string            `json:"kbName"`
	DocumentID string            `json:"documentId"`
	Source     string            `json:"source"`
	Content    string            `json:"content"`
	Similarity float32           `json:"similarity"`
	Metadata   map[string]string `json:"metadata"`
}

// kbMetaStore KB 元信息持久化（进程内单例 + JSON 文件）
var (
	kbMetaOnce sync.Once
	kbMetaMu   sync.RWMutex
	kbMetaPath string
	// kbMetaInMemory 内存中的 KB 元信息缓存（启动时从 JSON 加载）
	kbMetaInMemory map[string]*KnowledgeBaseInfo
)

// KBVectorizingStatus 知识库向量化进度状态（内存态，不持久化）
type KBVectorizingStatus struct {
	IsVectorizing  bool                 `json:"isVectorizing"`        // 是否正在向量化
	TotalFiles     int                  `json:"totalFiles"`           // 待处理文件总数
	ProcessedFiles int                  `json:"processedFiles"`       // 已处理文件数
	SuccessCount   int                  `json:"successCount"`         // 成功文件数
	FailedCount    int                  `json:"failedCount"`          // 失败文件数
	TotalChunks    int                  `json:"totalChunks"`          // 已入库切片数
	StartedAt      time.Time            `json:"startedAt"`            // 开始时间
	FinishedAt     *time.Time           `json:"finishedAt,omitempty"` // 结束时间（nil=进行中）
	Results        []KBFileImportResult `json:"results,omitempty"`    // 完成后填充每文件结果
	Error          string               `json:"error,omitempty"`      // 整体失败原因
}

// 向量化状态跟踪（内存 map，key=KB 名称）
var (
	kbVectorizingMu sync.RWMutex
	// kbVectorizingStatuses 保留每个 KB 最近一次向量化状态（含进行中与已完成）。
	// 已完成的状态在新一次导入开始时被覆盖，避免内存无限增长。
	kbVectorizingStatuses = make(map[string]*KBVectorizingStatus)

	// kbEmbedGlobalSem 限制 embedding API 全局并发数，避免高并发触发供应商限流。
	// 文件级并发 × 切片级并发 会叠加，用全局信号量兜底。
	kbEmbedGlobalSem = make(chan struct{}, 5)
)

// setKBVectorizing 设置 KB 的向量化状态（进行中）
func setKBVectorizing(kbName string, totalFiles int) *KBVectorizingStatus {
	kbVectorizingMu.Lock()
	defer kbVectorizingMu.Unlock()
	st := &KBVectorizingStatus{
		IsVectorizing: true,
		TotalFiles:    totalFiles,
		StartedAt:     time.Now(),
	}
	kbVectorizingStatuses[kbName] = st
	return st
}

// updateKBVectorizingProgress 更新进度（goroutine 内逐文件调用）
func updateKBVectorizingProgress(kbName string, processed, success, failed, chunks int) {
	kbVectorizingMu.Lock()
	defer kbVectorizingMu.Unlock()
	if st, ok := kbVectorizingStatuses[kbName]; ok {
		st.ProcessedFiles = processed
		st.SuccessCount = success
		st.FailedCount = failed
		st.TotalChunks = chunks
	}
}

// finishKBVectorizing 标记向量化完成，填充结果
func finishKBVectorizing(kbName string, results []KBFileImportResult, errMsg string) {
	kbVectorizingMu.Lock()
	defer kbVectorizingMu.Unlock()
	st, ok := kbVectorizingStatuses[kbName]
	if !ok {
		// 状态不存在时也创建一份已完成记录，便于前端读取结果
		st = &KBVectorizingStatus{StartedAt: time.Now()}
		kbVectorizingStatuses[kbName] = st
	}
	st.IsVectorizing = false
	now := time.Now()
	st.FinishedAt = &now
	st.Results = results
	st.Error = errMsg
	// 同步最终计数
	succ, fail, chunks := 0, 0, 0
	for _, r := range results {
		if r.Success {
			succ++
		} else {
			fail++
		}
		chunks += r.ChunkCount
	}
	st.SuccessCount = succ
	st.FailedCount = fail
	st.TotalChunks = chunks
	st.ProcessedFiles = len(results)
}

// GetKBVectorizingStatus 读取指定 KB 的向量化状态（可能 nil）
func GetKBVectorizingStatus(kbName string) *KBVectorizingStatus {
	kbVectorizingMu.RLock()
	defer kbVectorizingMu.RUnlock()
	if st, ok := kbVectorizingStatuses[kbName]; ok {
		// 返回副本，避免外部修改
		cp := *st
		if st.Results != nil {
			cp.Results = append([]KBFileImportResult(nil), st.Results...)
		}
		return &cp
	}
	return nil
}

// GetAllKBVectorizingStatuses 读取所有 KB 的向量化状态
func GetAllKBVectorizingStatuses() map[string]*KBVectorizingStatus {
	kbVectorizingMu.RLock()
	defer kbVectorizingMu.RUnlock()
	out := make(map[string]*KBVectorizingStatus, len(kbVectorizingStatuses))
	for k, v := range kbVectorizingStatuses {
		cp := *v
		if v.Results != nil {
			cp.Results = append([]KBFileImportResult(nil), v.Results...)
		}
		out[k] = &cp
	}
	return out
}

// initKBMeta 懒加载 KB 元信息存储（读取 JSON 文件到内存）
// 幂等，多次调用只初始化一次。
func initKBMeta() {
	kbMetaOnce.Do(func() {
		rootDir := deepAgentRootDir()
		kbMetaPath = filepath.Join(rootDir, memoryDirName, vectorStoreDirName, kbMetaFileName)
		kbMetaInMemory = make(map[string]*KnowledgeBaseInfo)

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(kbMetaPath), 0o755); err != nil {
			logger.SugaredLogger.Warnf("initKBMeta: 创建元信息目录失败: %v (path=%s)", err, filepath.Dir(kbMetaPath))
			return
		}

		// 读取已有元信息
		data, err := os.ReadFile(kbMetaPath)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.SugaredLogger.Warnf("initKBMeta: 读取元信息文件失败: %v (path=%s)", err, kbMetaPath)
			}
			return
		}
		if err := json.Unmarshal(data, &kbMetaInMemory); err != nil {
			logger.SugaredLogger.Warnf("initKBMeta: 解析元信息 JSON 失败: %v (path=%s)", err, kbMetaPath)
			kbMetaInMemory = make(map[string]*KnowledgeBaseInfo)
		}
	})
}

// persistKBMeta 将内存中的 KB 元信息写入磁盘
// 调用方需自行持有 kbMetaMu 写锁
func persistKBMeta() error {
	if kbMetaPath == "" {
		return fmt.Errorf("KB 元信息路径未初始化")
	}
	data, err := json.MarshalIndent(kbMetaInMemory, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 KB 元信息失败: %w", err)
	}
	// 先写临时文件再原子替换，避免写入中断导致文件损坏
	tmpPath := kbMetaPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("写入 KB 元信息临时文件失败: %w", err)
	}
	return os.Rename(tmpPath, kbMetaPath)
}

// kbCollectionName 将用户可读的 KB 名转换为 chromem collection 名（加前缀避免冲突）
func kbCollectionName(name string) string {
	return kbNamePrefix + name
}

// getKBDB 获取共享的 chromem.DB 实例（与长期记忆共用）
// 注意：必须先调用 initLongTermMemoryStore() 确保数据库已初始化。
// 返回的 DB 可用于创建/获取/删除任意 collection。
func getKBDB() *chromem.DB {
	initLongTermMemoryStore()
	return longTermMemoryDB
}

// getKBEmbedFunc 获取全局默认 embedding 函数（与长期记忆共用）。
// 用于未指定 KB 级配置时的兜底。
func getKBEmbedFunc() chromem.EmbeddingFunc {
	initLongTermMemoryStore()
	if longTermMemoryErr != nil {
		return nil
	}
	embedFunc, _, err := buildEmbeddingFunc()
	if err != nil {
		logger.SugaredLogger.Warnf("getKBEmbedFunc: 构造 embedding 函数失败: %v", err)
		return nil
	}
	return embedFunc
}

// getKBEmbedFuncFor 按 KB 元信息解析对应的 embedding 函数。
//
// 解析规则：
//   - info.AIConfigID > 0：用指定 AIConfig + info.EmbeddingModel 构造
//   - info.AIConfigID == 0：走全局默认逻辑（getKBEmbedFunc）
//
// 用于 AddDocumentToKB / SearchKnowledgeBase，确保使用 KB 创建时绑定的 embedding 配置。
func getKBEmbedFuncFor(info *KnowledgeBaseInfo) chromem.EmbeddingFunc {
	if info == nil {
		return wrapEmbedFuncWithCache(getKBEmbedFunc(), "default")
	}
	if info.AIConfigID == 0 {
		return wrapEmbedFuncWithCache(getKBEmbedFunc(), "default")
	}
	embedFunc, summary, err := buildEmbeddingFuncWith(info.AIConfigID, info.EmbeddingModel)
	if err != nil {
		logger.SugaredLogger.Warnf("getKBEmbedFuncFor: 构造 KB %q 的 embedding 函数失败 (aiConfigID=%d model=%q): %v，回退到全局默认",
			info.Name, info.AIConfigID, info.EmbeddingModel, err)
		return wrapEmbedFuncWithCache(getKBEmbedFunc(), "default")
	}
	logger.SugaredLogger.Debugf("getKBEmbedFuncFor: KB %q 使用 %s", info.Name, summary)
	// 用 AIConfigID + EmbeddingModel 作为缓存 key 前缀，区分不同模型（详见 embedding_cache.go）
	cacheKey := fmt.Sprintf("aic%d:%s", info.AIConfigID, info.EmbeddingModel)
	return wrapEmbedFuncWithCache(embedFunc, cacheKey)
}

// resolveAIConfigName 根据 AIConfigID 反查 AI 服务名称（用于前端展示）。
// 找不到时返回空字符串。
func resolveAIConfigName(aiConfigID uint) string {
	if aiConfigID == 0 {
		return ""
	}
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		return ""
	}
	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.ID == aiConfigID {
			return cfg.Name
		}
	}
	return ""
}

// CreateKnowledgeBase 创建一个新的知识库（即新建一个 chromem collection）。
//
// 参数：
//   - name: KB 名称（用户可读，不能为空，不能包含路径分隔符）
//   - description: KB 描述（可空）
//   - aiConfigID: 绑定的 AI 服务 ID（>0 用指定配置，=0 走全局默认）
//   - embeddingModel: 向量模型名（空时按优先级解析：env > AIConfig.EmbeddingModel > 默认）
//
// 返回创建后的 KB 元信息。若 KB 已存在则返回错误。
//
// 注意：embedding 配置在创建时绑定，后续不可修改（修改会导致新旧向量维度不一致，
// chromem-go 的 GetOrCreateCollection 在 collection 已存在时也会忽略新传入的 embedFunc）。
func CreateKnowledgeBase(name, description string, aiConfigID uint, embeddingModel string) (*KnowledgeBaseInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return nil, fmt.Errorf("知识库名称不能包含特殊字符 / \\ : * ? \" < > |")
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return nil, fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	embeddingModel = strings.TrimSpace(embeddingModel)
	// 构造 KB 专属 embedding 函数（提前校验，避免创建 collection 后才发现配置无效）
	embedFunc, embedSummary, err := buildEmbeddingFuncWith(aiConfigID, embeddingModel)
	if err != nil {
		return nil, fmt.Errorf("embedding 配置无效: %w", err)
	}

	kbMetaMu.Lock()
	defer kbMetaMu.Unlock()

	if _, exists := kbMetaInMemory[name]; exists {
		return nil, fmt.Errorf("知识库 %q 已存在", name)
	}

	collName := kbCollectionName(name)
	// metadata 记录用户可读名与描述，便于直接从 collection 元信息反查
	collMeta := map[string]string{
		"kb_name":     name,
		"description": description,
	}
	if _, err := db.GetOrCreateCollection(collName, collMeta, embedFunc); err != nil {
		return nil, fmt.Errorf("创建 collection 失败: %w", err)
	}

	now := time.Now()
	info := &KnowledgeBaseInfo{
		Name:           name,
		Description:    description,
		DocumentCount:  0,
		CreatedAt:      now,
		UpdatedAt:      now,
		AIConfigID:     aiConfigID,
		AIConfigName:   resolveAIConfigName(aiConfigID),
		EmbeddingModel: embeddingModel,
	}
	kbMetaInMemory[name] = info

	if err := persistKBMeta(); err != nil {
		logger.SugaredLogger.Warnf("CreateKnowledgeBase: 持久化元信息失败: %v", err)
	}

	logger.SugaredLogger.Infof("知识库创建成功: name=%q description=%q embedding=%s", name, description, embedSummary)
	return info, nil
}

// ListKnowledgeBases 列出所有知识库元信息（按创建时间升序）
// 返回的副本不含 Documents 索引（避免列表接口返回过大数据）；如需文档列表请用 ListDocumentsInKB。
func ListKnowledgeBases() []*KnowledgeBaseInfo {
	initKBMeta()
	kbMetaMu.RLock()
	defer kbMetaMu.RUnlock()

	result := make([]*KnowledgeBaseInfo, 0, len(kbMetaInMemory))
	for _, info := range kbMetaInMemory {
		// 复制一份再修改，避免污染缓存；列表接口不返回 Documents 索引
		infoCopy := *info
		infoCopy.Documents = nil
		// 实时反查 AI 服务名（AIConfig.Name 可能被用户修改）
		infoCopy.AIConfigName = resolveAIConfigName(info.AIConfigID)
		// 同步真实文档数（以 chromem collection 实际数为准）
		// ⚠️ 必须传入正确的 embedFunc 而非 nil：chromem-go 的 GetCollection 在
		// collection 刚从磁盘加载（c.embed==nil）时，会用传入的 embeddingFunc 初始化 c.embed；
		// 若传 nil 则回退到 NewEmbeddingFuncDefault()（硬编码 OpenAI URL），污染 collection
		// 导致后续 AddDocumentToKB 传入的正确 embedFunc 被忽略，embedding 请求误发到 OpenAI。
		if db := getKBDB(); db != nil {
			if coll := db.GetCollection(kbCollectionName(info.Name), getKBEmbedFuncFor(info)); coll != nil {
				infoCopy.DocumentCount = coll.Count()
			}
		}
		result = append(result, &infoCopy)
	}

	// 按创建时间升序
	sortKBByCreated(result)
	return result
}

// sortKBByCreated 按 CreatedAt 升序排序（轻量级内联实现，避免引入 sort 包到调用方）
func sortKBByCreated(items []*KnowledgeBaseInfo) {
	// 插入排序：KB 数量通常很小（<100），插入排序比 sort.Slice 还快
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j-1].CreatedAt.After(items[j].CreatedAt); j-- {
			items[j-1], items[j] = items[j], items[j-1]
		}
	}
}

// GetKnowledgeBase 获取指定 KB 的元信息（不存在返回 nil）
func GetKnowledgeBase(name string) *KnowledgeBaseInfo {
	initKBMeta()
	kbMetaMu.RLock()
	defer kbMetaMu.RUnlock()
	if info, ok := kbMetaInMemory[name]; ok {
		infoCopy := *info
		infoCopy.Documents = nil
		infoCopy.AIConfigName = resolveAIConfigName(info.AIConfigID)
		// ⚠️ 传入正确 embedFunc 而非 nil，避免 chromem-go 用 OpenAI 默认函数污染 collection。
		// 详见 ListKnowledgeBases 中的说明。
		if db := getKBDB(); db != nil {
			if coll := db.GetCollection(kbCollectionName(name), getKBEmbedFuncFor(info)); coll != nil {
				infoCopy.DocumentCount = coll.Count()
			}
		}
		return &infoCopy
	}
	return nil
}

// DeleteKnowledgeBase 删除指定知识库（包括 collection 与元信息）。
// 不存在则返回错误。
func DeleteKnowledgeBase(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("知识库名称不能为空")
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.Lock()
	defer kbMetaMu.Unlock()

	if _, exists := kbMetaInMemory[name]; !exists {
		return fmt.Errorf("知识库 %q 不存在", name)
	}

	if err := db.DeleteCollection(kbCollectionName(name)); err != nil {
		return fmt.Errorf("删除 collection 失败: %w", err)
	}

	delete(kbMetaInMemory, name)
	if err := persistKBMeta(); err != nil {
		logger.SugaredLogger.Warnf("DeleteKnowledgeBase: 持久化元信息失败: %v", err)
	}

	logger.SugaredLogger.Infof("知识库已删除: name=%q", name)
	return nil
}

// AddDocumentToKB 向指定 KB 添加一段文本（自动切片入库）。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - content: 文本内容（按段落切片）
//   - source: 来源标记（如文件名、URL、"inline"）
//   - extraMetadata: 额外 metadata（可空）
//
// 返回新增的文档 ID 列表（每个切片一个 ID）。
func AddDocumentToKB(kbName, content, source string, extraMetadata map[string]string) ([]string, error) {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("文本内容不能为空")
	}
	if strings.TrimSpace(source) == "" {
		source = "inline"
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return nil, fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.RLock()
	kbInfo, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("知识库 %q 不存在", kbName)
	}

	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(kbInfo))
	if coll == nil {
		return nil, fmt.Errorf("获取 collection 失败: %q", kbName)
	}

	chunks := sliceForKB(content)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("切片结果为空（内容过短或无效）")
	}

	// 并发 worker pool 加速 embedding（chromem-go 的 AddDocument 线程安全，内部有读写锁）
	ts := time.Now().Unix()
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	total := len(chunks)

	// 预构造所有文档
	type chunkTask struct {
		idx int
		doc chromem.Document
	}
	tasks := make([]chunkTask, total)
	for i, chunk := range chunks {
		docID := fmt.Sprintf("%s_%d_%02d", kbName, ts, i)
		metadata := map[string]string{
			"source":       source,
			"chunk_index":  fmt.Sprintf("%d", i),
			"total_chunks": fmt.Sprintf("%d", total),
			"created_at":   nowStr,
			"kb_name":      kbName,
		}
		for k, v := range extraMetadata {
			metadata[k] = v
		}
		tasks[i] = chunkTask{idx: i, doc: chromem.Document{ID: docID, Metadata: metadata, Content: chunk}}
	}

	// 并发入库（切片级并发 8，但受全局信号量 kbEmbedGlobalSem 限制实际 API 并发为 5）
	const concurrency = 8
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	docIDs := make([]string, 0, total)
	docIndexes := make([]KBDocumentIndex, 0, total)
	var firstErr error

	for _, task := range tasks {
		wg.Add(1)
		go func(t chunkTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// 全局信号量限制 embedding API 实际并发，避免触发供应商限流
			kbEmbedGlobalSem <- struct{}{}
			defer func() { <-kbEmbedGlobalSem }()

			chunkCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			chunkStart := time.Now()
			if err := coll.AddDocument(chunkCtx, t.doc); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("写入 chunk %d/%d 失败: %w", t.idx+1, total, err)
				}
				mu.Unlock()
				logger.SugaredLogger.Warnf("AddDocumentToKB: chunk %d/%d 失败 (耗时=%v): %v",
					t.idx+1, total, time.Since(chunkStart), err)
				return
			}
			mu.Lock()
			docIDs = append(docIDs, t.doc.ID)
			docIndexes = append(docIndexes, KBDocumentIndex{
				DocID:       t.doc.ID,
				Source:      source,
				ChunkIndex:  t.idx,
				TotalChunks: total,
				CreatedAt:   now,
			})
			mu.Unlock()
			logger.SugaredLogger.Debugf("AddDocumentToKB: chunk %d/%d 成功 (耗时=%v)",
				t.idx+1, total, time.Since(chunkStart))
		}(task)
	}
	wg.Wait()

	// 更新元信息：追加文档索引、刷新文档数与时间
	kbMetaMu.Lock()
	if info, ok := kbMetaInMemory[kbName]; ok {
		info.Documents = append(info.Documents, docIndexes...)
		info.UpdatedAt = now
		info.DocumentCount = coll.Count()
	}
	if err := persistKBMeta(); err != nil {
		logger.SugaredLogger.Warnf("AddDocumentToKB: 持久化元信息失败: %v", err)
	}
	kbMetaMu.Unlock()

	logger.SugaredLogger.Infof("知识库文档入库完成: kb=%q source=%q chunks=%d 成功=%d 失败=%d",
		kbName, source, total, len(docIDs), total-len(docIDs))
	if firstErr != nil {
		return docIDs, fmt.Errorf("部分切片入库失败（成功 %d/%d）: %w", len(docIDs), total, firstErr)
	}
	return docIDs, nil
}

// AddFileToKB 解析指定文件并入库到 KB。
//
// 当前支持的文件类型：
//   - .txt / .md / .markdown：直接读取 UTF-8 文本
//   - 其他类型：返回错误（后续可扩展 PDF/DOCX 解析）
//
// 文件大小限制：10MB（避免一次入库超大文件导致内存暴涨）
func AddFileToKB(kbName, filePath string) ([]string, error) {
	kbName = strings.TrimSpace(kbName)
	filePath = strings.TrimSpace(filePath)
	if kbName == "" || filePath == "" {
		return nil, fmt.Errorf("知识库名称与文件路径均不能为空")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在或不可访问: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("路径是目录而非文件: %s", filePath)
	}
	if info.Size() > 10*1024*1024 {
		return nil, fmt.Errorf("文件过大（%d 字节，超过 10MB 限制）", info.Size())
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var content string
	switch ext {
	case ".txt", ".md", ".markdown":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
		content = string(data)
	case ".pdf", ".docx", ".doc":
		return nil, fmt.Errorf("暂不支持 %s 文件，请先转换为 .txt 或 .md（后续版本将支持）", ext)
	default:
		// 允许尝试以文本方式读取（但提示可能乱码）
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
		content = string(data)
	}

	source := filepath.Base(filePath)
	return AddDocumentToKB(kbName, content, source, map[string]string{
		"file_path": filePath,
		"file_size": fmt.Sprintf("%d", info.Size()),
	})
}

// KBFileImportResult 单个文件的导入结果（用于批量导入返回）
type KBFileImportResult struct {
	FilePath   string   `json:"filePath"`   // 文件路径
	FileName   string   `json:"fileName"`   // 文件名（展示用）
	Success    bool     `json:"success"`    // 是否成功
	DocIDs     []string `json:"docIds"`     // 成功时新增的文档 ID 列表（每个切片一个）
	ChunkCount int      `json:"chunkCount"` // 成功时切片数
	Error      string   `json:"error"`      // 失败原因（成功时为空）
}

// KBBatchImportSummary 批量导入汇总
type KBBatchImportSummary struct {
	TotalFiles   int                  `json:"totalFiles"`   // 总文件数
	SuccessCount int                  `json:"successCount"` // 成功文件数
	FailedCount  int                  `json:"failedCount"`  // 失败文件数
	TotalChunks  int                  `json:"totalChunks"`  // 新增切片总数
	Results      []KBFileImportResult `json:"results"`      // 每个文件的结果
}

// AddFilesToKB 批量导入多个文件到指定 KB（同步执行，会阻塞直到全部处理完）。
//
// 逐个文件处理（复用 AddFileToKB），单个文件失败不影响其他文件继续导入。
// 处理过程中实时更新向量化状态（setKBVectorizing / updateKBVectorizingProgress / finishKBVectorizing），
// 前端可通过 GetKBVectorizingStatus 轮询进度。
//
// 返回每个文件的导入结果 + 汇总统计。
// 注意：本函数同步执行，若需后台处理请用 StartBatchImport（goroutine 包装）。
func AddFilesToKB(kbName string, filePaths []string) (*KBBatchImportSummary, error) {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("文件路径列表为空")
	}

	// 设置向量化状态（进行中）
	setKBVectorizing(kbName, len(filePaths))

	summary := &KBBatchImportSummary{
		TotalFiles: len(filePaths),
		Results:    make([]KBFileImportResult, 0, len(filePaths)),
	}

	// 并发处理文件（3 worker，与切片级 5 worker 叠加后总并发约 15，平衡速度与 API 限流）
	const fileConcurrency = 3
	fileSem := make(chan struct{}, fileConcurrency)
	var fileWg sync.WaitGroup
	var fileMu sync.Mutex
	processed := 0

	for _, fp := range filePaths {
		fp = strings.TrimSpace(fp)
		if fp == "" {
			continue
		}
		fileWg.Add(1)
		go func(filePath string) {
			defer fileWg.Done()
			fileSem <- struct{}{}
			defer func() { <-fileSem }()

			result := KBFileImportResult{
				FilePath: filePath,
				FileName: filepath.Base(filePath),
			}
			docIDs, err := AddFileToKB(kbName, filePath)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				result.DocIDs = docIDs
				result.ChunkCount = len(docIDs)
			} else {
				result.Success = true
				result.DocIDs = docIDs
				result.ChunkCount = len(docIDs)
			}

			fileMu.Lock()
			if result.Success {
				summary.SuccessCount++
			} else {
				summary.FailedCount++
			}
			summary.TotalChunks += len(docIDs)
			summary.Results = append(summary.Results, result)
			processed++
			updateKBVectorizingProgress(kbName, processed, summary.SuccessCount, summary.FailedCount, summary.TotalChunks)
			fileMu.Unlock()

			logger.SugaredLogger.Infof("批量导入进度: kb=%q file=%s success=%v chunks=%d (%d/%d)",
				kbName, result.FileName, result.Success, result.ChunkCount, processed, summary.TotalFiles)
		}(fp)
	}
	fileWg.Wait()

	// 标记完成（保留结果供前端轮询读取）
	finishKBVectorizing(kbName, summary.Results, "")

	logger.SugaredLogger.Infof("批量导入完成: kb=%q total=%d success=%d failed=%d chunks=%d",
		kbName, summary.TotalFiles, summary.SuccessCount, summary.FailedCount, summary.TotalChunks)
	return summary, nil
}

// StartBatchImport 异步启动批量导入（goroutine 后台处理，立即返回）。
//
// 与 AddFilesToKB 的区别：本函数不阻塞，立即返回 nil；导入在后台 goroutine 中执行，
// 前端可通过 GetKBVectorizingStatus 轮询进度。完成后状态保留在内存中供查询。
//
// 用于"导入后可关闭抽屉，后台继续处理"场景。
// 若该 KB 已在向量化中则返回错误，避免并发写冲突。
func StartBatchImport(kbName string, filePaths []string) error {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return fmt.Errorf("知识库名称不能为空")
	}
	if len(filePaths) == 0 {
		return fmt.Errorf("文件路径列表为空")
	}
	// 校验 KB 存在
	initKBMeta()
	kbMetaMu.RLock()
	_, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists {
		return fmt.Errorf("知识库 %q 不存在", kbName)
	}
	// 防止并发：检查是否已在向量化
	if st := GetKBVectorizingStatus(kbName); st != nil && st.IsVectorizing {
		return fmt.Errorf("知识库 %q 正在向量化中，请等待完成", kbName)
	}

	// 在 goroutine 启动前就设置进行中状态，确保 StartBatchImport 返回后前端轮询立即可见
	setKBVectorizing(kbName, len(filePaths))

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("StartBatchImport panic: kb=%q err=%v", kbName, r)
				finishKBVectorizing(kbName, nil, fmt.Sprintf("内部错误: %v", r))
			}
		}()
		_, err := AddFilesToKB(kbName, filePaths)
		if err != nil {
			finishKBVectorizing(kbName, nil, err.Error())
			logger.SugaredLogger.Warnf("StartBatchImport 失败: kb=%q err=%v", kbName, err)
		}
	}()

	logger.SugaredLogger.Infof("已启动后台批量导入: kb=%q files=%d", kbName, len(filePaths))
	return nil
}

// SearchKnowledgeBase 在指定 KB 中检索语义相关的文档片段。
//
// 参数：
//   - ctx: 上下文（用于 embedding API 调用与超时控制）
//   - kbName: 目标 KB 名称
//   - query: 检索查询语句
//   - topK: 返回结果数（<=0 用默认 5，>20 截断到 20）
//
// 返回按相似度降序排列的结果列表。KB 不存在或为空时返回空列表 + nil error。
func SearchKnowledgeBase(ctx context.Context, kbName, query string, topK int) ([]KnowledgeBaseSearchResult, error) {
	kbName = strings.TrimSpace(kbName)
	query = strings.TrimSpace(query)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	if query == "" {
		return nil, fmt.Errorf("查询语句不能为空")
	}
	if topK <= 0 {
		topK = kbDefaultTopK
	}
	if topK > kbMaxTopK {
		topK = kbMaxTopK
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return nil, fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.RLock()
	kbInfo, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("知识库 %q 不存在", kbName)
	}

	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(kbInfo))
	if coll == nil {
		return nil, fmt.Errorf("获取 collection 失败: %q", kbName)
	}
	docCount := coll.Count()
	if docCount == 0 {
		return []KnowledgeBaseSearchResult{}, nil
	}
	// chromem-go 的 Query 要求 nResults <= 文档数，否则报错；自动截断
	if topK > docCount {
		topK = docCount
	}

	results, err := coll.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("检索失败: %w", err)
	}

	out := make([]KnowledgeBaseSearchResult, 0, len(results))
	for _, r := range results {
		out = append(out, KnowledgeBaseSearchResult{
			KBName:     kbName,
			DocumentID: r.ID,
			Source:     r.Metadata["source"],
			Content:    r.Content,
			Similarity: r.Similarity,
			Metadata:   r.Metadata,
		})
	}
	return out, nil
}

// SearchKnowledgeBaseHybrid 知识库检索（当前退化为纯向量检索）。
//
// 说明：早期曾引入 BM25 关键词倒排索引通道与 RRF 融合（bm25_index.go），
// 但真实数据测试表明：知识库 chunk 多为超长表格/章节模板，BM25 会产生大量
// 与查询语义无关的假命中，且精确命中文档因权重机制被压制在 Top-K 之外，
// 混合召回效果反而不如纯向量。因此本函数现直接委托纯向量检索
// SearchKnowledgeBase，保持调用方（SearchAllKnowledge / 工具）接口不变。
func SearchKnowledgeBaseHybrid(ctx context.Context, kbName, query string, topK int) ([]KnowledgeBaseSearchResult, error) {
	return SearchKnowledgeBase(ctx, kbName, query, topK)
}

// ListDocumentsInKB 列出指定 KB 中的所有文档切片（基于 KB 元信息中的文档索引）。
//
// 注意：返回的是 KB 元信息中维护的文档索引（按入库顺序）。
// 若索引与 chromem 实际 documents 不一致（例如手动删除了 .gob 文件），
// DocumentCount 仍以 chromem collection.Count() 为准。
//
// 每条结果包含内容预览（前 200 字），需读取 chromem document。
// 若 chromem 中文档已被删除但索引未同步，对应条目的 ContentPreview 会为空。
func ListDocumentsInKB(kbName string) ([]KnowledgeBaseDocument, error) {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return nil, fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.RLock()
	info, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists || info == nil {
		return nil, fmt.Errorf("知识库 %q 不存在", kbName)
	}

	// ⚠️ 传入正确 embedFunc 而非 nil，避免 chromem-go 用 OpenAI 默认函数污染 collection
	// （即使本函数不调用 embedding，污染会影响后续 AddDocumentToKB 等写操作）。
	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(info))
	if coll == nil {
		return nil, fmt.Errorf("获取 collection 失败: %q", kbName)
	}

	ctx := context.Background()

	// 复制索引快照（避免在读取过程中被并发修改）
	kbMetaMu.RLock()
	indexes := make([]KBDocumentIndex, len(info.Documents))
	copy(indexes, info.Documents)
	kbMetaMu.RUnlock()

	result := make([]KnowledgeBaseDocument, 0, len(indexes))
	for _, idx := range indexes {
		// 尝试从 chromem 读取文档内容做预览
		var preview string
		if doc, err := coll.GetByID(ctx, idx.DocID); err == nil {
			preview = truncateForLog(doc.Content, 200)
		}

		result = append(result, KnowledgeBaseDocument{
			ID:             idx.DocID,
			Source:         idx.Source,
			ChunkIndex:     idx.ChunkIndex,
			TotalChunks:    idx.TotalChunks,
			CreatedAt:      idx.CreatedAt.Format("2006-01-02 15:04:05"),
			ContentPreview: preview,
		})
	}
	return result, nil
}

// KBDocumentsPage 文档列表分页结果（后台分页）
type KBDocumentsPage struct {
	Items    []KnowledgeBaseDocument `json:"items"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}

// ListDocumentsInKBPaged 分页返回指定 KB 的文档列表。
//
// 与 ListDocumentsInKB 的区别：只读取当前页的文档内容预览（GetByID），
// 避免文档数多时全量读取导致卡顿。
//
// 参数：
//   - page: 页码，从 1 开始（<1 自动修正为 1）
//   - pageSize: 每页条数（<1 自动修正为 20，>100 截断为 100）
func ListDocumentsInKBPaged(kbName string, page, pageSize int) (*KBDocumentsPage, error) {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return nil, fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.RLock()
	info, exists := kbMetaInMemory[kbName]
	if !exists || info == nil {
		kbMetaMu.RUnlock()
		return nil, fmt.Errorf("知识库 %q 不存在", kbName)
	}
	// 复制索引快照（避免在读取过程中被并发修改）
	indexes := make([]KBDocumentIndex, len(info.Documents))
	copy(indexes, info.Documents)
	kbMetaMu.RUnlock()

	total := len(indexes)
	// 计算分页边界
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := indexes[start:end]

	// ⚠️ 传入正确 embedFunc 而非 nil，避免 chromem-go 用 OpenAI 默认函数污染 collection
	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(info))
	if coll == nil {
		return nil, fmt.Errorf("获取 collection 失败: %q", kbName)
	}

	ctx := context.Background()
	result := make([]KnowledgeBaseDocument, 0, len(pageItems))
	for _, idx := range pageItems {
		var preview string
		if doc, err := coll.GetByID(ctx, idx.DocID); err == nil {
			preview = truncateForLog(doc.Content, 200)
		}
		result = append(result, KnowledgeBaseDocument{
			ID:             idx.DocID,
			Source:         idx.Source,
			ChunkIndex:     idx.ChunkIndex,
			TotalChunks:    idx.TotalChunks,
			CreatedAt:      idx.CreatedAt.Format("2006-01-02 15:04:05"),
			ContentPreview: preview,
		})
	}

	return &KBDocumentsPage{
		Items:    result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// DeleteDocumentFromKB 从指定 KB 中删除单个文档（按 docID）。
//
// 注意：chromem-go 的 Delete 需要 where/whereDocument/ids 中至少一个，
// 这里用 ids 精确删除单个文档。同时同步更新 KB 元信息中的文档索引。
func DeleteDocumentFromKB(kbName, docID string) error {
	kbName = strings.TrimSpace(kbName)
	docID = strings.TrimSpace(docID)
	if kbName == "" || docID == "" {
		return fmt.Errorf("知识库名称与文档 ID 均不能为空")
	}

	initKBMeta()
	db := getKBDB()
	if db == nil {
		return fmt.Errorf("向量库未初始化: %v", longTermMemoryErr)
	}

	kbMetaMu.RLock()
	info, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists {
		return fmt.Errorf("知识库 %q 不存在", kbName)
	}

	// ⚠️ 传入正确 embedFunc 而非 nil，避免 chromem-go 用 OpenAI 默认函数污染 collection。
	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(info))
	if coll == nil {
		return fmt.Errorf("获取 collection 失败: %q", kbName)
	}

	ctx := context.Background()
	if err := coll.Delete(ctx, nil, nil, docID); err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}

	// 同步更新元信息文档索引
	kbMetaMu.Lock()
	if info, ok := kbMetaInMemory[kbName]; ok {
		filtered := make([]KBDocumentIndex, 0, len(info.Documents))
		for _, d := range info.Documents {
			if d.DocID != docID {
				filtered = append(filtered, d)
			}
		}
		info.Documents = filtered
		info.UpdatedAt = time.Now()
		info.DocumentCount = coll.Count()
	}
	if err := persistKBMeta(); err != nil {
		logger.SugaredLogger.Warnf("DeleteDocumentFromKB: 持久化元信息失败: %v", err)
	}
	kbMetaMu.Unlock()

	logger.SugaredLogger.Infof("知识库文档已删除: kb=%q docID=%q", kbName, docID)
	return nil
}

// sliceForKB 将知识库文本切片（基于 go-chunker 的 Markdown 递归策略）。
//
// 切分规则：
//  1. 标准化换行符（兼容 Windows/Mac/Linux/Unicode）
//  2. 用 go-chunker Markdown 策略递归切分：标题 → 段落 → 句号 → 空格
//  3. 每片目标 ~500 字符，重叠 60 字符，过滤 <20 字的过短片段
//
// 返回切片后的字符串数组。输入为空时返回 nil。
func sliceForKB(content string) []string {
	c := strings.TrimSpace(content)
	if c == "" {
		return nil
	}
	c = normalizeText(c)
	chunks, err := chunkText(c)
	if err != nil {
		logger.SugaredLogger.Warnf("sliceForKB: chunkText 失败，回退到整体返回: %v", err)
		return []string{c}
	}
	return chunks
}
