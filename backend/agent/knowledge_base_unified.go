package agent

// knowledge_base_unified.go — 跨所有知识库 + 长期记忆统一检索。
//
// 在现有「单 KB 检索」(SearchKnowledgeBase) 与「长期记忆检索」(SearchRelevant) 之上
// 增加一层聚合：一次调用同时检索所有自定义 KB 与 qa_history 历史经验，合并结果后
// 按相似度全局重排，返回带来源标签的统一结果。
//
// 设计动机：
//   - Agent 若要"跨所有知识源"获取背景信息，原本需先 ListKnowledgeBases 再逐个
//     SearchKnowledgeBase + SearchLongTermMemory，多轮工具调用既慢又贵
//   - 前端"知识库问答"需要一键聚合检索后让 Agent 综合回答
//   - 统一结果类型便于前端按来源分组/着色展示，也便于注入 Agent 系统提示词
//
// 检索流程：
//   1. ListKnowledgeBases 取所有 KB 名
//   2. 并发检索每个 KB（复用 SearchKnowledgeBase）+ 长期记忆（SearchRelevant）
//      各源取 topK*2（多取一些便于全局重排后仍满足 topK）
//   3. 合并为 UnifiedKnowledgeHit 列表，按相似度全局降序排序
//   4. 截断到 topK
//
// 容错：单个源失败仅记日志，不影响其他源；全部失败返回空列表 + nil error。
//
// 相似度可比性说明：不同 KB 可能绑定不同 embedding 模型，跨模型相似度并非严格可比，
// 但多数场景下作为粗排依据足够；如需更精确可后续引入 cross-encoder 重排。

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"go-stock/backend/logger"
)

const (
	// UnifiedSourceKB 自定义知识库来源
	UnifiedSourceKB = "knowledge_base"
	// UnifiedSourceLTM 长期记忆（历史问答）来源
	UnifiedSourceLTM = "long_term_memory"
	// unifiedDefaultTopK 统一检索默认 Top-K
	unifiedDefaultTopK = 5
	// unifiedMaxTopK 统一检索最大 Top-K
	unifiedMaxTopK = 20
)

// UnifiedKnowledgeHit 跨知识库统一检索的单条命中结果。
//
// 同时承载自定义知识库片段与长期记忆历史问答，前端展示与 Agent 工具/系统提示注入统一消费。
type UnifiedKnowledgeHit struct {
	SourceType string            `json:"sourceType"` // "knowledge_base" | "long_term_memory"
	KBName     string            `json:"kbName"`     // KB 名（LTM 来源时为 "历史经验"）
	Source     string            `json:"source"`     // 来源标记（文件名 / inline / qa_history）
	Content    string            `json:"content"`    // 命中片段正文
	Similarity float32           `json:"similarity"` // 与查询的相似度 [-1, 1]
	Metadata   map[string]string `json:"metadata"`   // 原始 metadata（含 chunk_index 等）
	// 以下字段仅 LTM 来源填充，便于前端展示历史问答结构
	Question   string `json:"question,omitempty"`
	Mode       string `json:"mode,omitempty"`
	Date       string `json:"date,omitempty"`
	ReportPath string `json:"reportPath,omitempty"`
}

// SearchAllKnowledge 跨所有自定义知识库 + 长期记忆统一检索。
//
// 参数：
//   - ctx: 上下文（用于 embedding API 调用与超时控制）
//   - query: 检索查询语句
//   - topK: 返回结果数（<=0 用默认 5，>20 截断到 20）
//
// 返回按相似度全局降序排列的统一结果列表。所有源均无结果时返回空列表 + nil error。
func SearchAllKnowledge(ctx context.Context, query string, topK int) ([]UnifiedKnowledgeHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("查询语句不能为空")
	}
	if topK <= 0 {
		topK = unifiedDefaultTopK
	}
	if topK > unifiedMaxTopK {
		topK = unifiedMaxTopK
	}

	// 各源多取一些（topK*2），便于全局重排后仍满足 topK；同时受 unifiedMaxTopK 约束
	fetchPerSource := topK * 2
	if fetchPerSource > unifiedMaxTopK {
		fetchPerSource = unifiedMaxTopK
	}

	var mu sync.Mutex
	hits := make([]UnifiedKnowledgeHit, 0)
	var wg sync.WaitGroup

	// 检索所有自定义 KB
	for _, kb := range ListKnowledgeBases() {
		kbName := kb.Name
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("SearchAllKnowledge: KB %q panic: %v", name, r)
				}
			}()
			results, err := SearchKnowledgeBaseHybrid(ctx, name, query, fetchPerSource)
			if err != nil {
				logger.SugaredLogger.Warnf("SearchAllKnowledge: 检索 KB %q 失败(不影响其他源): %v", name, err)
				return
			}
			mu.Lock()
			for _, r := range results {
				hits = append(hits, UnifiedKnowledgeHit{
					SourceType: UnifiedSourceKB,
					KBName:     r.KBName,
					Source:     r.Source,
					Content:    r.Content,
					Similarity: r.Similarity,
					Metadata:   r.Metadata,
				})
			}
			mu.Unlock()
		}(kbName)
	}

	// 检索长期记忆（qa_history）
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("SearchAllKnowledge: LTM panic: %v", r)
			}
		}()
		recalls := SearchRelevant(ctx, query, fetchPerSource, CurrentUserKey(""))
		mu.Lock()
		for _, r := range recalls {
			hits = append(hits, UnifiedKnowledgeHit{
				SourceType: UnifiedSourceLTM,
				KBName:     "历史经验",
				Source:     "qa_history",
				Content:    r.Reply,
				Similarity: r.Similarity,
				Question:   r.Question,
				Mode:       r.Mode,
				Date:       r.Date,
				ReportPath: r.ReportPath,
			})
		}
		mu.Unlock()
	}()

	wg.Wait()

	// 全局按相似度降序排序（稳定排序保留同分时各源写入顺序）
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].Similarity > hits[j].Similarity
	})

	// 截断到 topK
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// FormatUnifiedHits 将统一检索结果格式化为 markdown，供 Agent 工具输出与系统提示注入复用。
//
// 输出格式示例：
//
//	知识库统一检索结果（跨所有自定义知识库 + 历史经验，按相似度排序，共 3 条）:
//
//	1. [相似度 0.85] 来源: 贵州茅台_2024年报.txt (知识库: 财报知识)
//	   内容:
//	   2024 年公司实现营业收入...
//
//	2. [相似度 0.78] 来源: 历史问答 2026-01-10 [react]
//	   Q: 白酒板块今天为什么涨？
//	   内容:
//	   ...
func FormatUnifiedHits(hits []UnifiedKnowledgeHit) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("知识库统一检索结果（跨所有自定义知识库 + 历史经验，按相似度排序，共 %d 条）:\n\n", len(hits)))
	for i, h := range hits {
		sourceLabel := h.Source
		if sourceLabel == "" {
			sourceLabel = "未知来源"
		}
		if h.SourceType == UnifiedSourceLTM {
			sourceLabel = "历史问答"
			if h.Date != "" {
				sourceLabel += " " + h.Date
			}
			if h.Mode != "" {
				sourceLabel += " [" + h.Mode + "]"
			}
		}
		sb.WriteString(fmt.Sprintf("%d. [相似度 %.2f] 来源: %s", i+1, h.Similarity, sourceLabel))
		if h.SourceType == UnifiedSourceKB {
			sb.WriteString(" (知识库: " + h.KBName + ")")
		}
		sb.WriteString("\n")
		if h.SourceType == UnifiedSourceLTM && h.Question != "" {
			sb.WriteString(fmt.Sprintf("   Q: %s\n", truncateForLog(h.Question, 200)))
		}
		sb.WriteString("   内容:\n")
		content := strings.TrimSpace(h.Content)
		for _, line := range strings.Split(content, "\n") {
			sb.WriteString("   " + line + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("请在回答时优先参考上述检索内容；若内容过时或不适用请主动说明；如需实时行情/财务数据请调用对应数据工具。\n")
	return sb.String()
}

// BuildKBQASystemPrompt 构造「知识库问答」专用的系统提示词。
//
// 将检索到的统一命中片段拼装为上下文，指导 Agent 基于这些内容回答用户问题。
// 用于 ChatWithContext 的 sysPromptOverride（optsOverride[0]）。
//
// 提示词策略：
//   - 明确告知 Agent 当前已提供检索到的知识库内容，优先据此回答
//   - 不再要求 Agent 重复调用知识库检索工具（避免冗余）
//   - 保留实时数据工具调用能力（行情/财务等需走数据工具）
//   - 内容不足时如实告知，不得编造
func BuildKBQASystemPrompt(hits []UnifiedKnowledgeHit) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业的知识库问答助手。用户的问题已通过统一检索从所有自定义知识库与历史问答经验中召回相关片段，" +
		"请基于以下检索到的内容回答用户问题。\n\n")
	sb.WriteString("回答要求：\n")
	sb.WriteString("1. 优先依据下方「检索内容」作答，引用时标注来源（知识库名/历史问答日期）\n")
	sb.WriteString("2. 若检索内容不足以完整回答，请明确指出缺失部分，不得编造未在检索内容中出现的信息\n")
	sb.WriteString("3. 若检索内容之间存在矛盾，请分别列示并提示用户甄别\n")
	sb.WriteString("4. 如需实时行情、财务数据等动态信息，可调用对应数据查询工具获取，不要依赖检索内容中的过时数字\n")
	sb.WriteString("5. 无需再次调用 SearchKnowledgeBase / SearchLongTermMemory / SearchAllKnowledge 等知识库检索工具，所需上下文已在下方提供\n\n")
	sb.WriteString("================ 检索内容 ================\n")
	contextText := FormatUnifiedHits(hits)
	if contextText == "" {
		sb.WriteString("（未检索到相关内容，请如实告知用户当前知识库与历史经验中无匹配信息）\n")
	} else {
		sb.WriteString(contextText)
	}
	sb.WriteString("================ 检索内容结束 ================\n")
	return sb.String()
}
