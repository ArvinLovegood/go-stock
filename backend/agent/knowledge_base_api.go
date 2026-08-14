package agent

// knowledge_base_api.go — 知识库向量管理 Wails API 封装。
//
// 将 knowledge_base.go 中的核心能力（创建/列出/删除 KB、添加文档、检索）
// 包装为前端可调用的 API。Wails 通过反射暴露 struct 方法到前端 JS。
//
// 设计原则：
//   - 与 CronTaskApi / SettingsApi 模式保持一致：薄包装层，业务在核心模块
//   - 所有方法返回 (data, error) 或 error，Wails 自动序列化为 JS Promise
//   - 错误信息使用中文，便于用户理解
//   - 不做参数校验重复（核心模块已做），仅做必要的 nil 检查

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// KnowledgeBaseApi 知识库管理 API（Wails 绑定）
//
// 用法（在 app.go 中）：
//
//	agent.NewKnowledgeBaseApi().CreateKB("财报知识", "上市公司财报摘要")
type KnowledgeBaseApi struct{}

// NewKnowledgeBaseApi 构造知识库 API 实例
func NewKnowledgeBaseApi() *KnowledgeBaseApi {
	return &KnowledgeBaseApi{}
}

// CreateKB 创建知识库
//
// 参数：
//   - name: KB 名称（用户可读，不能为空）
//   - description: KB 描述（可空）
//   - aiConfigID: 绑定的 AI 服务 ID（>0 指定，=0 走全局默认）
//   - embeddingModel: 向量模型名（空时按优先级解析）
//
// 返回创建后的 KB 元信息。
func (a *KnowledgeBaseApi) CreateKB(name, description string, aiConfigID uint, embeddingModel string) (*KnowledgeBaseInfo, error) {
	info, err := CreateKnowledgeBase(name, description, aiConfigID, embeddingModel)
	if err != nil {
		logger.SugaredLogger.Warnf("CreateKB 失败: %v (name=%q aiConfigID=%d model=%q)", err, name, aiConfigID, embeddingModel)
		return nil, err
	}
	return info, nil
}

// ListAIServicesForKB 列出可用于知识库 embedding 的 AI 服务（前端下拉选择用）。
//
// 返回所有 ApiKey+BaseUrl 非空的 AIConfig 摘要（ID + 名称 + BaseUrl + 已配置的 embedding 模型）。
// 前端展示为下拉选项，用户选择后传 aiConfigID 给 CreateKB。
func (a *KnowledgeBaseApi) ListAIServicesForKB() ([]KBAIServiceOption, error) {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return nil, fmt.Errorf("未配置任何 AIConfig，请先在设置中添加 AI 服务")
	}
	opts := make([]KBAIServiceOption, 0, len(settingConfig.AiConfigs))
	for _, cfg := range settingConfig.AiConfigs {
		if cfg == nil || cfg.ApiKey == "" || cfg.BaseUrl == "" {
			continue
		}
		opts = append(opts, KBAIServiceOption{
			ID:             cfg.ID,
			Name:           cfg.Name,
			BaseUrl:        cfg.BaseUrl,
			ModelName:      cfg.ModelName,
			ModelType:      cfg.ModelType,
			EmbeddingModel: cfg.EmbeddingModel,
		})
	}
	if len(opts) == 0 {
		return nil, fmt.Errorf("没有可用的 AI 服务（需 ApiKey + BaseUrl 均非空）")
	}
	return opts, nil
}

// GetLongTermMemoryAiConfigId 读取长期记忆绑定的向量服务 ID。
// 返回 0 表示自动模式（未指定），>0 表示用指定 AIConfig。
func (a *KnowledgeBaseApi) GetLongTermMemoryAiConfigId() int {
	cfg := data.GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		return 0
	}
	return cfg.LongTermMemoryAiConfigId
}

// SetLongTermMemoryAiConfigId 设置长期记忆绑定的向量服务 ID。
// id=0 表示自动模式（优先 ModelType=embedding 的服务）。
// 直接更新 DB 单字段，避免要求前端加载/保存整个 SettingConfig。
//
// 注意：切换向量服务后，旧向量与新服务维度可能不一致，
// 建议同时清空 <exe_dir>/memory/.vectorstore/ 重建（本方法不自动清空）。
func (a *KnowledgeBaseApi) SetLongTermMemoryAiConfigId(id int) error {
	if id < 0 {
		id = 0
	}
	// 取首条 settings 记录的 ID 做条件更新（settings 表通常只有一行）
	var s data.Settings
	if err := db.Dao.First(&s).Error; err != nil {
		return fmt.Errorf("读取 settings 失败: %w", err)
	}
	if err := db.Dao.Model(&data.Settings{}).Where("id=?", s.ID).
		Update("long_term_memory_ai_config_id", id).Error; err != nil {
		return fmt.Errorf("更新长期记忆向量服务失败: %w", err)
	}
	// 刷新内存缓存
	data.GetSettingConfig()
	logger.SugaredLogger.Infof("长期记忆向量服务已设置为 aiConfigId=%d", id)
	return nil
}

// KBAIServiceOption 知识库创建时可选的 AI 服务条目（前端下拉选项）
type KBAIServiceOption struct {
	ID             uint   `json:"id"`             // AIConfig ID
	Name           string `json:"name"`           // AI 服务名称
	BaseUrl        string `json:"baseUrl"`        // API 基础地址
	ModelName      string `json:"modelName"`      // 模型名（type=embedding 时即向量模型名）
	ModelType      string `json:"modelType"`      // 模型类型："chat" / "embedding"
	EmbeddingModel string `json:"embeddingModel"` // 兼容旧字段：对话服务配置的 embedding 模型
}

// ListKB 列出所有知识库（按创建时间升序）
func (a *KnowledgeBaseApi) ListKB() []*KnowledgeBaseInfo {
	return ListKnowledgeBases()
}

// GetKB 获取指定知识库的元信息
func (a *KnowledgeBaseApi) GetKB(name string) (*KnowledgeBaseInfo, error) {
	info := GetKnowledgeBase(name)
	if info == nil {
		return nil, fmt.Errorf("知识库 %q 不存在", name)
	}
	return info, nil
}

// DeleteKB 删除指定知识库（包括所有文档与 collection）
func (a *KnowledgeBaseApi) DeleteKB(name string) error {
	return DeleteKnowledgeBase(name)
}

// AddDocument 向指定 KB 添加一段文本（自动切片入库）。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - content: 文本内容
//   - source: 来源标记（可空，默认 "inline"）
//
// 返回新增的文档 ID 列表（每个切片一个 ID）。
func (a *KnowledgeBaseApi) AddDocument(kbName, content, source string) ([]string, error) {
	return AddDocumentToKB(kbName, content, source, nil)
}

// UploadFile 解析指定文件并入库到 KB（支持 .txt/.md，其他类型返回错误）。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - filePath: 文件绝对路径
//
// 返回新增的文档 ID 列表。
// 注意：前端通过 Wails OpenFileDialog 选文件后传路径过来，避免大文本通过 IPC 传输。
func (a *KnowledgeBaseApi) UploadFile(kbName, filePath string) ([]string, error) {
	return AddFileToKB(kbName, filePath)
}

// UploadFiles 批量导入多个文件到 KB（异步，立即返回，后台 goroutine 处理）。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - filePaths: 文件绝对路径数组
//
// 立即返回 nil 表示已开始后台处理；前端通过 GetKBVectorizingStatus 轮询进度。
// 单个文件失败不影响其他文件。若该 KB 已在向量化中则返回错误。
func (a *KnowledgeBaseApi) UploadFiles(kbName string, filePaths []string) error {
	return StartBatchImport(kbName, filePaths)
}

// GetKBVectorizingStatus 查询指定 KB 的向量化状态（进行中/已完成/nil）
func (a *KnowledgeBaseApi) GetKBVectorizingStatus(kbName string) *KBVectorizingStatus {
	return GetKBVectorizingStatus(kbName)
}

// GetAllKBVectorizingStatuses 查询所有 KB 的向量化状态（前端轮询用）
func (a *KnowledgeBaseApi) GetAllKBVectorizingStatuses() map[string]*KBVectorizingStatus {
	return GetAllKBVectorizingStatuses()
}

// SearchKBRequest 检索请求参数（结构化便于前端传参）
type SearchKBRequest struct {
	KBName string `json:"kbName"`
	Query  string `json:"query"`
	TopK   int    `json:"topK"`
}

// SearchKB 在指定 KB 中检索语义相关文档。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - query: 检索查询语句
//   - topK: 返回结果数（<=0 用默认 5，>20 截断到 20）
//
// 返回按相似度降序排列的结果列表。
func (a *KnowledgeBaseApi) SearchKB(kbName, query string, topK int) ([]KnowledgeBaseSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return SearchKnowledgeBase(ctx, kbName, query, topK)
}

// ListDocuments 列出指定 KB 中的所有文档切片（基于 KB 元信息文档索引）。
func (a *KnowledgeBaseApi) ListDocuments(kbName string) ([]KnowledgeBaseDocument, error) {
	return ListDocumentsInKB(kbName)
}

// ListDocumentsPaged 分页返回指定 KB 的文档列表（后台分页，避免全量加载卡顿）
func (a *KnowledgeBaseApi) ListDocumentsPaged(kbName string, page, pageSize int) (*KBDocumentsPage, error) {
	return ListDocumentsInKBPaged(kbName, page, pageSize)
}

// DeleteDocument 从指定 KB 中删除单个文档
//
// 参数：
//   - kbName: 目标 KB 名称
//   - docID: 文档 ID（AddDocument/UploadFile 返回值之一，或从 ListDocuments 获取）
func (a *KnowledgeBaseApi) DeleteDocument(kbName, docID string) error {
	return DeleteDocumentFromKB(kbName, docID)
}

// ============ 知识图谱 ============

// BuildKBGraph 异步构建知识库的知识图谱。
// 遍历 KB 所有文档切片，用 LLM 抽取实体与关系，聚合后持久化。
// aiConfigID>0 用指定对话服务，=0 自动取首个 chat 类型。
// 立即返回 nil 表示已开始后台构建；前端通过 GetKBGraphBuildStatus 轮询进度。
func (a *KnowledgeBaseApi) BuildKBGraph(kbName string, aiConfigID uint) error {
	return BuildKBGraph(kbName, aiConfigID)
}

// GetKBGraph 读取指定 KB 的知识图谱数据（未构建时返回 nil, nil）
func (a *KnowledgeBaseApi) GetKBGraph(kbName string) (*KBGraph, error) {
	return GetKBGraph(kbName)
}

// GetKBGraphBuildStatus 查询指定 KB 的图谱构建状态（可能 nil）
func (a *KnowledgeBaseApi) GetKBGraphBuildStatus(kbName string) *KBGraphBuildStatus {
	return GetKBGraphBuildStatus(kbName)
}

// DeleteKBGraph 删除指定 KB 的知识图谱
func (a *KnowledgeBaseApi) DeleteKBGraph(kbName string) error {
	return DeleteKBGraph(kbName)
}

// ============ 长期记忆 ============

// LTMInfo 长期记忆向量库信息（前端展示用）
type LTMInfo struct {
	Ready        bool   `json:"ready"`                  // 向量库是否就绪
	DocCount     int    `json:"docCount"`               // 文档切片数
	Error        string `json:"error,omitempty"`        // 未就绪原因
	AIConfigID   int    `json:"aiConfigId"`             // 绑定的向量服务 ID（0=自动模式）
	AIConfigName string `json:"aiConfigName,omitempty"` // 绑定的向量服务名
}

// GetLongTermMemoryInfo 获取长期记忆向量库信息
func (a *KnowledgeBaseApi) GetLongTermMemoryInfo() *LTMInfo {
	initLongTermMemoryStore()
	info := &LTMInfo{}
	if longTermMemoryErr != nil {
		info.Error = longTermMemoryErr.Error()
		return info
	}
	if longTermMemoryColl == nil {
		info.Error = "向量库未初始化"
		return info
	}
	info.Ready = true
	info.DocCount = longTermMemoryColl.Count()
	if cfg := data.GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		info.AIConfigID = cfg.LongTermMemoryAiConfigId
		if info.AIConfigID > 0 {
			info.AIConfigName = resolveAIConfigName(uint(info.AIConfigID))
		}
	}
	return info
}

// SearchLongTermMemory 检索长期记忆（语义召回历史问答）
//
// 参数：
//   - query: 查询语句
//   - topK: 返回结果数（<=0 用默认5，>20 截断到20）
func (a *KnowledgeBaseApi) SearchLongTermMemory(query string, topK int) ([]MemoryRecall, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("查询不能为空")
	}
	recalls := SearchRelevant(ctx, query, topK, CurrentUserKey(""))
	return recalls, nil
}

// SearchAllKnowledge 跨所有自定义知识库 + 长期记忆统一检索。
//
// 参数：
//   - query: 检索查询语句
//   - topK: 返回结果数（<=0 用默认 5，>20 截断到 20）
//
// 一次调用并发检索所有 KB 与 qa_history，合并后按相似度全局降序排序。
// 返回带来源标签（knowledge_base / long_term_memory）的统一结果列表。
func (a *KnowledgeBaseApi) SearchAllKnowledge(query string, topK int) ([]UnifiedKnowledgeHit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("查询不能为空")
	}
	return SearchAllKnowledge(ctx, query, topK)
}
