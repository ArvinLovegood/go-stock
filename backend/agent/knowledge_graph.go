package agent

// knowledge_graph.go — 知识库知识图谱构建与可视化模块。
//
// 在现有知识库（chromem-go 向量存储）基础上，用 LLM 从文档片段中抽取
// 实体与关系，聚合为知识图谱，供前端 echarts 力导向图可视化。
//
// 设计原则：
//   - 复用 KB 已有文档切片（chromem collection），不重复存储原文
//   - LLM 抽取采用全局首个 chat 类型 AIConfig（KB 绑定的是 embedding 服务）
//   - 图谱数据持久化到独立 JSON 文件，与 KB 元信息解耦
//   - 构建过程异步执行，复用向量化状态跟踪模式（内存态 + 前端轮询）
//   - 节点规模上限 500（按权重截断），避免前端渲染卡顿
//
// 持久化路径：
//   - 图谱数据：<exe_dir>/memory/.vectorstore/.kb_graph_<name>.json
//   - 构建状态：内存态（不持久化，重启后需重新构建）

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

const (
	// kbGraphFilePrefix 图谱文件前缀
	kbGraphFilePrefix = ".kb_graph_"
	// kbGraphFileSuffix 图谱文件后缀
	kbGraphFileSuffix = ".json"
	// kbGraphMaxNodes 图谱节点上限（按权重截断）
	kbGraphMaxNodes = 500
	// kbGraphLLMIdleTimeout LLM 流式响应空闲超时（AskAi 不 close channel，靠超时判定完成）
	kbGraphLLMIdleTimeout = 15 * time.Second
	// kbGraphLLMTotalTimeout 单次 LLM 调用总超时
	kbGraphLLMTotalTimeout = 5 * time.Minute
	// kbGraphChunkMaxChars 单次送 LLM 抽取的文本上限（避免超出 token 限制）
	kbGraphChunkMaxChars = 2000
)

// KBGraph 知识图谱数据（持久化 + 前端渲染用）
type KBGraph struct {
	KBName   string        `json:"kbName"`
	Nodes    []KBGraphNode `json:"nodes"`
	Edges    []KBGraphEdge `json:"edges"`
	BuiltAt  time.Time     `json:"builtAt"`
	DocCount int           `json:"docCount"` // 构建时的文档切片数
}

// KBGraphNode 图谱节点（实体）
type KBGraphNode struct {
	ID     string `json:"id"`     // 节点 ID（实体名归一化后的哈希，此处直接用归一化名）
	Name   string `json:"name"`   // 实体名称
	Type   string `json:"type"`   // 实体类型：人物/机构/公司/地点/概念/事件/指标/产品/其他
	Weight int    `json:"weight"` // 出现频次（越高节点越大）
}

// KBGraphEdge 图谱边（关系）
type KBGraphEdge struct {
	Source   string `json:"source"`   // 起点节点 ID
	Target   string `json:"target"`   // 终点节点 ID
	Relation string `json:"relation"` // 关系描述（如：投资/属于/位于/竞争对手）
	Weight   int    `json:"weight"`   // 共现频次
}

// KBGraphBuildStatus 图谱构建状态（内存态）
type KBGraphBuildStatus struct {
	IsBuilding    bool       `json:"isBuilding"`
	TotalDocs     int        `json:"totalDocs"`
	ProcessedDocs int        `json:"processedDocs"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	Error         string     `json:"error,omitempty"`
	NodeCount     int        `json:"nodeCount"`
	EdgeCount     int        `json:"edgeCount"`
}

// llmExtractionResult LLM 抽取结果（单次调用）
type llmExtractionResult struct {
	Entities  []llmEntity   `json:"entities"`
	Relations []llmRelation `json:"relations"`
}

type llmEntity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type llmRelation struct {
	Subject  string `json:"subject"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

// 图谱构建状态跟踪（内存 map，key=KB 名称）
var (
	kbGraphBuildMu       sync.RWMutex
	kbGraphBuildStatuses = make(map[string]*KBGraphBuildStatus)
)

// 图谱构建系统提示词
const kbGraphSystemPrompt = `你是一个专业的信息抽取助手。请从给定文本中抽取实体和关系，以 JSON 格式返回。

抽取规则：
1. 实体类型从以下选择：人物、机构、公司、地点、概念、事件、指标、产品、其他
2. 只抽取文本中明确出现的实体，不要臆测或补充
3. 关系应简明扼要（2-6字），如：投资、属于、位于、竞争对手、合作、收购、生产、研发、持有
4. 实体名称用规范简称，去掉多余的修饰词
5. 如果文本无明确实体关系，返回 {"entities":[],"relations":[]}

返回格式（严格 JSON，不要 markdown 代码块）：
{"entities":[{"name":"实体名","type":"类型"}],"relations":[{"subject":"实体1","relation":"关系","object":"实体2"}]}`

// ============ 公开 API ============

// BuildKBGraph 异步触发知识图谱构建。
//
// 参数：
//   - kbName: 目标 KB 名称
//   - aiConfigID: 指定用于抽取的对话 AI 服务 ID（>0 用指定，=0 自动取首个 chat 类型）
//
// 流程：
//  1. 校验 KB 存在 & 未在构建中
//  2. 遍历 KB 所有文档切片，逐片调 LLM 抽取实体关系
//  3. 聚合去重（同名实体合并、同关系合并，累加权重）
//  4. 按权重截断到 500 节点
//  5. 持久化到 .kb_graph_<name>.json
//
// 立即返回 nil 表示已开始后台构建；前端通过 GetKBGraphBuildStatus 轮询进度。
func BuildKBGraph(kbName string, aiConfigID uint) error {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return fmt.Errorf("知识库名称不能为空")
	}

	initKBMeta()
	kbMetaMu.RLock()
	_, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists {
		return fmt.Errorf("知识库 %q 不存在", kbName)
	}

	// 并发控制：同一 KB 不能重复构建
	kbGraphBuildMu.Lock()
	if st, ok := kbGraphBuildStatuses[kbName]; ok && st.IsBuilding {
		kbGraphBuildMu.Unlock()
		return fmt.Errorf("知识库 %q 的图谱正在构建中", kbName)
	}
	kbGraphBuildStatuses[kbName] = &KBGraphBuildStatus{
		IsBuilding: true,
		StartedAt:  time.Now(),
	}
	kbGraphBuildMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("BuildKBGraph panic: kb=%q err=%v", kbName, r)
				finishKBGraphBuild(kbName, 0, 0, fmt.Sprintf("内部错误: %v", r))
			}
		}()
		buildKBGraphSync(kbName, aiConfigID)
	}()
	return nil
}

// GetKBGraph 读取指定 KB 的知识图谱数据
func GetKBGraph(kbName string) (*KBGraph, error) {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return nil, fmt.Errorf("知识库名称不能为空")
	}
	path := kbGraphFilePath(kbName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 图谱尚未构建，返回 nil（非错误）
		}
		return nil, fmt.Errorf("读取图谱文件失败: %w", err)
	}
	var g KBGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("解析图谱 JSON 失败: %w", err)
	}
	return &g, nil
}

// GetKBGraphBuildStatus 查询指定 KB 的图谱构建状态
func GetKBGraphBuildStatus(kbName string) *KBGraphBuildStatus {
	kbGraphBuildMu.RLock()
	defer kbGraphBuildMu.RUnlock()
	if st, ok := kbGraphBuildStatuses[kbName]; ok {
		cp := *st
		return &cp
	}
	return nil
}

// DeleteKBGraph 删除指定 KB 的知识图谱
func DeleteKBGraph(kbName string) error {
	kbName = strings.TrimSpace(kbName)
	if kbName == "" {
		return fmt.Errorf("知识库名称不能为空")
	}
	path := kbGraphFilePath(kbName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除图谱文件失败: %w", err)
	}
	// 清除构建状态
	kbGraphBuildMu.Lock()
	delete(kbGraphBuildStatuses, kbName)
	kbGraphBuildMu.Unlock()
	logger.SugaredLogger.Infof("知识图谱已删除: kb=%q", kbName)
	return nil
}

// ============ 内部实现 ============

// kbGraphFilePath 图谱文件路径
func kbGraphFilePath(kbName string) string {
	rootDir := deepAgentRootDir()
	return filepath.Join(rootDir, memoryDirName, vectorStoreDirName,
		kbGraphFilePrefix+kbName+kbGraphFileSuffix)
}

// setKBGraphProgress 更新构建进度
func setKBGraphProgress(kbName string, processed, total, nodeCount, edgeCount int) {
	kbGraphBuildMu.Lock()
	defer kbGraphBuildMu.Unlock()
	if st, ok := kbGraphBuildStatuses[kbName]; ok {
		st.ProcessedDocs = processed
		st.TotalDocs = total
		st.NodeCount = nodeCount
		st.EdgeCount = edgeCount
	}
}

// finishKBGraphBuild 标记构建完成
func finishKBGraphBuild(kbName string, nodeCount, edgeCount int, errMsg string) {
	kbGraphBuildMu.Lock()
	defer kbGraphBuildMu.Unlock()
	st, ok := kbGraphBuildStatuses[kbName]
	if !ok {
		st = &KBGraphBuildStatus{StartedAt: time.Now()}
		kbGraphBuildStatuses[kbName] = st
	}
	st.IsBuilding = false
	now := time.Now()
	st.FinishedAt = &now
	st.NodeCount = nodeCount
	st.EdgeCount = edgeCount
	st.Error = errMsg
}

// buildKBGraphSync 同步构建知识图谱（在 goroutine 内执行）
func buildKBGraphSync(kbName string, aiConfigID uint) {
	db := getKBDB()
	if db == nil {
		finishKBGraphBuild(kbName, 0, 0, fmt.Sprintf("向量库未初始化: %v", longTermMemoryErr))
		return
	}

	// 解析用于抽取的对话 AIConfig：
	// aiConfigID > 0 时用用户指定的；=0 时自动取首个 chat 类型
	effectiveID := aiConfigID
	if effectiveID == 0 {
		effectiveID = resolveChatAIConfigID()
	}
	if effectiveID == 0 {
		finishKBGraphBuild(kbName, 0, 0, "未找到可用的对话 AI 服务（需 ModelType=chat 且 ApiKey+BaseUrl 非空）")
		return
	}

	// 获取 KB 元信息与文档索引
	kbMetaMu.RLock()
	info, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists || info == nil {
		finishKBGraphBuild(kbName, 0, 0, fmt.Sprintf("知识库 %q 不存在", kbName))
		return
	}

	// ⚠️ 传入正确 embedFunc 而非 nil，避免 chromem-go 用 OpenAI 默认函数污染 collection
	// （本函数虽不调用 embedding，但污染会影响后续 AddDocumentToKB 等写操作）。
	coll := db.GetCollection(kbCollectionName(kbName), getKBEmbedFuncFor(info))
	if coll == nil {
		finishKBGraphBuild(kbName, 0, 0, fmt.Sprintf("获取 collection 失败: %q", kbName))
		return
	}

	// 复制文档索引快照
	kbMetaMu.RLock()
	indexes := make([]KBDocumentIndex, len(info.Documents))
	copy(indexes, info.Documents)
	kbMetaMu.RUnlock()

	total := len(indexes)
	if total == 0 {
		finishKBGraphBuild(kbName, 0, 0, "")
		return
	}
	setKBGraphProgress(kbName, 0, total, 0, 0)

	// 聚合容器
	nodeMap := make(map[string]*KBGraphNode) // key=归一化实体名
	edgeMap := make(map[string]*KBGraphEdge) // key="src|relation|tgt"
	ctx := context.Background()

	for i, idx := range indexes {
		// 读取文档完整内容
		doc, err := coll.GetByID(ctx, idx.DocID)
		if err != nil || doc.Content == "" {
			logger.SugaredLogger.Warnf("buildKBGraphSync: 读取文档失败 kb=%q docID=%q err=%v", kbName, idx.DocID, err)
			setKBGraphProgress(kbName, i+1, total, len(nodeMap), len(edgeMap))
			continue
		}

		// 截断超长文本（避免超出 LLM token 限制）
		text := doc.Content
		if len(text) > kbGraphChunkMaxChars {
			text = text[:kbGraphChunkMaxChars]
		}

		// 调 LLM 抽取
		result, err := extractEntitiesWithLLM(effectiveID, text)
		if err != nil {
			logger.SugaredLogger.Warnf("buildKBGraphSync: LLM 抽取失败 kb=%q doc#%d err=%v", kbName, i+1, err)
			setKBGraphProgress(kbName, i+1, total, len(nodeMap), len(edgeMap))
			continue
		}

		// 聚合到图谱
		mergeExtraction(nodeMap, edgeMap, result)
		setKBGraphProgress(kbName, i+1, total, len(nodeMap), len(edgeMap))
	}

	// 截断到上限节点数
	nodes, edges := truncateGraph(nodeMap, edgeMap, kbGraphMaxNodes)

	// 持久化
	graph := &KBGraph{
		KBName:   kbName,
		Nodes:    nodes,
		Edges:    edges,
		BuiltAt:  time.Now(),
		DocCount: total,
	}
	if err := persistKBGraph(graph); err != nil {
		finishKBGraphBuild(kbName, len(nodes), len(edges), err.Error())
		return
	}

	finishKBGraphBuild(kbName, len(nodes), len(edges), "")
	logger.SugaredLogger.Infof("知识图谱构建完成: kb=%q nodes=%d edges=%d docs=%d",
		kbName, len(nodes), len(edges), total)
}

// resolveChatAIConfigID 查找首个可用的 chat 类型 AIConfig。
// KB 绑定的是 embedding 服务，图谱抽取需要对话模型，故全局查找。
func resolveChatAIConfigID() uint {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		return 0
	}
	for _, cfg := range settingConfig.AiConfigs {
		if cfg == nil || cfg.ApiKey == "" || cfg.BaseUrl == "" {
			continue
		}
		// ModelType 为空或 "chat" 均视为对话模型（兼容旧数据）
		if cfg.ModelType == "" || cfg.ModelType == "chat" {
			return cfg.ID
		}
	}
	return 0
}

// extractEntitiesWithLLM 调用 LLM 抽取实体与关系
func extractEntitiesWithLLM(aiConfigID uint, text string) (*llmExtractionResult, error) {
	resp, err := callLLMSync(aiConfigID, kbGraphSystemPrompt, text)
	if err != nil {
		return nil, err
	}
	// 从响应中提取 JSON（LLM 可能包裹 markdown 代码块）
	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		return nil, fmt.Errorf("LLM 响应中未找到有效 JSON: %s", truncateForLog(resp, 200))
	}
	var result llmExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("解析 LLM 抽取结果失败: %w (json=%s)", err, truncateForLog(jsonStr, 200))
	}
	return &result, nil
}

// callLLMSync 同步调用 LLM（基于 AskAi 流式 + 空闲超时收集完整响应）。
// AskAi 在 FinishReason=="stop" 时 return 但不 close channel，故用空闲超时判定完成。
func callLLMSync(aiConfigID uint, systemPrompt, userContent string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kbGraphLLMTotalTimeout)
	defer cancel()

	o := data.NewDeepSeekOpenAi(ctx, int(aiConfigID))
	if o.GetAPIKey() == "" || o.GetBaseURL() == "" {
		return "", fmt.Errorf("AI 服务配置不完整（ApiKey 或 BaseUrl 为空）")
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userContent},
	}

	ch := make(chan map[string]any, 100)
	go data.AskAi(o, nil, messages, ch, "", false)

	var sb strings.Builder
	for {
		select {
		case msg := <-ch:
			code, _ := msg["code"].(int)
			if code == 0 {
				content, _ := msg["content"].(string)
				return "", fmt.Errorf("LLM 调用失败: %s", content)
			}
			if content, ok := msg["content"].(string); ok {
				sb.WriteString(content)
			}
		case <-time.After(kbGraphLLMIdleTimeout):
			// 空闲超时，认为流式传输已完成
			return sb.String(), nil
		case <-ctx.Done():
			return sb.String(), fmt.Errorf("LLM 调用超时: %w", ctx.Err())
		}
	}
}

// extractJSON 从可能包含 markdown 代码块的文本中提取 JSON 字符串
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去除 markdown 代码块包裹
	if strings.HasPrefix(s, "```") {
		// 去掉首行 ```json 或 ```
		if idx := strings.Index(s, "\n"); idx > 0 {
			s = s[idx+1:]
		}
		// 去掉末尾 ```
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	// 找到第一个 { 和最后一个 }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

// mergeExtraction 将单次抽取结果聚合到图谱容器中（去重 + 累加权重）
func mergeExtraction(nodeMap map[string]*KBGraphNode, edgeMap map[string]*KBGraphEdge, result *llmExtractionResult) {
	// 实体名 -> 归一化 ID 的映射（供关系引用）
	nameToID := make(map[string]string)

	for _, e := range result.Entities {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		id := normalizeEntityID(name)
		nameToID[name] = id
		etype := strings.TrimSpace(e.Type)
		if etype == "" {
			etype = "其他"
		}
		if node, ok := nodeMap[id]; ok {
			node.Weight++
			// 保留首次的类型，但若首次是"其他"而后次有具体类型则更新
			if node.Type == "其他" && etype != "其他" {
				node.Type = etype
			}
		} else {
			nodeMap[id] = &KBGraphNode{
				ID:     id,
				Name:   name,
				Type:   etype,
				Weight: 1,
			}
		}
	}

	for _, r := range result.Relations {
		subj := strings.TrimSpace(r.Subject)
		obj := strings.TrimSpace(r.Object)
		rel := strings.TrimSpace(r.Relation)
		if subj == "" || obj == "" || rel == "" {
			continue
		}
		srcID, ok1 := nameToID[subj]
		tgtID, ok2 := nameToID[obj]
		if !ok1 || !ok2 {
			// 关系引用的实体未在 entities 中出现，自动补建
			if !ok1 {
				srcID = normalizeEntityID(subj)
				if _, exists := nodeMap[srcID]; !exists {
					nodeMap[srcID] = &KBGraphNode{ID: srcID, Name: subj, Type: "其他", Weight: 1}
				}
			}
			if !ok2 {
				tgtID = normalizeEntityID(obj)
				if _, exists := nodeMap[tgtID]; !exists {
					nodeMap[tgtID] = &KBGraphNode{ID: tgtID, Name: obj, Type: "其他", Weight: 1}
				}
			}
		}
		edgeKey := srcID + "|" + rel + "|" + tgtID
		if edge, ok := edgeMap[edgeKey]; ok {
			edge.Weight++
		} else {
			edgeMap[edgeKey] = &KBGraphEdge{
				Source:   srcID,
				Target:   tgtID,
				Relation: rel,
				Weight:   1,
			}
		}
	}
}

// normalizeEntityID 将实体名归一化为节点 ID（去空白 + 转小写）
func normalizeEntityID(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// truncateGraph 按节点权重降序截断到 maxNodes，并过滤悬空边
func truncateGraph(nodeMap map[string]*KBGraphNode, edgeMap map[string]*KBGraphEdge, maxNodes int) ([]KBGraphNode, []KBGraphEdge) {
	// 节点按权重降序排序
	nodes := lo.Values(nodeMap)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Weight > nodes[j].Weight
	})
	if len(nodes) > maxNodes {
		nodes = nodes[:maxNodes]
	}
	// 保留的节点 ID 集合
	keepIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		keepIDs[n.ID] = true
	}
	// 过滤悬空边（起点或终点被截断）
	edges := make([]KBGraphEdge, 0, len(edgeMap))
	for _, e := range edgeMap {
		if keepIDs[e.Source] && keepIDs[e.Target] {
			edges = append(edges, *e)
		}
	}
	// 转换节点为值类型
	nodeValues := make([]KBGraphNode, len(nodes))
	for i, n := range nodes {
		nodeValues[i] = *n
	}
	return nodeValues, edges
}

// persistKBGraph 持久化图谱到 JSON 文件（原子写入）
func persistKBGraph(g *KBGraph) error {
	path := kbGraphFilePath(g.KBName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建图谱目录失败: %w", err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化图谱失败: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("写入图谱临时文件失败: %w", err)
	}
	return os.Rename(tmpPath, path)
}
