package agent

// stock_basic_info_kb.go — A股"不经常变化"的 F10 数据自动向量化到知识库。
//
// 当调用 GetStockOrgBasicInfo / GetStockQtrMainFinance / GetStockOrgPredict /
// GetStockPredictSummary / GetStockHolderTrend 等工具时，自动检查"A股基础数据"知识库中
// 是否已存在该数据（按 sourceKey 去重），未入库则后台异步向量化，不阻塞工具返回。
// 后续可用 SearchKnowledgeBase 检索这些历史/低频更新的数据。
//
// 解耦说明：
//   - tools 包不能 import agent 包（agent→tools 已存在，反向会循环依赖），
//     故 tools 包定义函数变量 MaybeVectorizeStockBasicInfoFn / MaybeVectorizeStockDataFn，
//     由本包 init() 注入。

import (
	"fmt"
	"strings"
	"sync/atomic"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/logger"
)

// StockBasicInfoKBName A股基础数据知识库名称（公司基础资料、季度财务、机构预测等共用的统一知识库）
const StockBasicInfoKBName = "A股基础数据"

// kbEmbeddingUnavailable 缓存"全局 embedding 配置不可用"的判断结果。
// 首次发现不可用后置 true，后续 MaybeVectorizeStockData 直接静默跳过，避免反复触发
// initLongTermMemoryStore（其失败时无短路标志，会重复打 warn 日志）。
// 用户配置 embedding 服务后重启程序即可重置。
var kbEmbeddingUnavailable atomic.Bool

func init() {
	// 注入自动向量化函数到 tools 包，避免 tools→agent 循环依赖
	tools.MaybeVectorizeStockBasicInfoFn = MaybeVectorizeStockBasicInfo
	tools.MaybeVectorizeStockDataFn = MaybeVectorizeStockData
}

// MaybeVectorizeStockBasicInfo 检查指定股票的基础资料是否已向量化到 A股基础数据知识库，
// 未向量化时后台异步入库（不阻塞调用方）。兼容旧调用方式，内部转调 MaybeVectorizeStockData。
//   - stockCode: 股票代码（任意常见格式，内部归一化后作为 source 去重标记的一部分）
//   - content: 基础资料文本（Markdown）
func MaybeVectorizeStockBasicInfo(stockCode, content string) {
	code := normalizeStockCodeForKB(stockCode)
	if code == "" {
		return
	}
	MaybeVectorizeStockData("basicinfo:"+code, content, "公司基础资料")
}

// MaybeVectorizeStockData 通用自动向量化函数：检查指定 sourceKey 是否已存在于 A股基础数据
// 知识库，未存在时后台异步入库（不阻塞调用方）。所有数据类型共用同一 KB，通过 sourceKey
// 前缀区分类型（如 basicinfo:002008、qtrfinance:002008），便于按类型去重。
//   - sourceKey: 全局唯一的去重标记，建议格式 "<typePrefix>:<normalizedStockCode>"
//   - content: 文档内容（Markdown）
//   - dataType: 数据类型描述，用于 metadata 的 type 字段（如 "公司基础资料"、"季度主要财务"）
func MaybeVectorizeStockData(sourceKey, content, dataType string) {
	sourceKey = strings.TrimSpace(sourceKey)
	if sourceKey == "" || strings.TrimSpace(content) == "" {
		return
	}
	// 已确认 embedding 配置不可用则静默跳过（避免日志噪音与无效 API 调用）
	if kbEmbeddingUnavailable.Load() {
		return
	}
	// 确保 KB 存在（存在则跳过）。若 embedding 配置不可用导致创建失败，
	// 标记 kbEmbeddingUnavailable=true，后续调用直接静默跳过。
	if err := ensureStockBasicInfoKB(); err != nil {
		if getKBEmbedFunc() == nil {
			kbEmbeddingUnavailable.Store(true)
			logger.SugaredLogger.Warnf("MaybeVectorizeStockData: embedding 配置不可用，已禁用自动向量化直至重启: %v", err)
			return
		}
		logger.SugaredLogger.Warnf("MaybeVectorizeStockData: 确保 KB 失败: %v", err)
		return
	}
	// 已向量化则跳过
	if isSourceInKB(StockBasicInfoKBName, sourceKey) {
		return
	}
	// 后台异步入库
	go func() {
		extraMeta := map[string]string{
			"source_key": sourceKey,
			"type":       dataType,
		}
		docIDs, err := AddDocumentToKB(StockBasicInfoKBName, content, sourceKey, extraMeta)
		if err != nil {
			// 附带 embedding 配置诊断信息，便于定位"配置了非 embedding 类型的 AI 服务"等问题。
			// chromem-go 的错误只含 HTTP 状态码（如 "404 Not Found"）不含响应体，
			// 单看错误无法判断是哪个 AIConfig/模型出问题，故在此补充配置上下文。
			logger.SugaredLogger.Warnf("MaybeVectorizeStockData: 入库失败 sourceKey=%s type=%s kb=%s embedding=%s: %v",
				sourceKey, dataType, StockBasicInfoKBName, describeKBEmbeddingConfig(StockBasicInfoKBName), err)
			return
		}
		logger.SugaredLogger.Infof("MaybeVectorizeStockData: 入库成功 sourceKey=%s type=%s chunks=%d", sourceKey, dataType, len(docIDs))
	}()
}

// describeKBEmbeddingConfig 返回指定 KB 实际使用的 embedding 配置摘要，用于入库失败时的诊断日志。
// 格式如 "aiConfig#1(name=通义千问,type=chat,baseUrl=https://...,model=text-embedding-3-small)"。
// KB 不存在或配置异常时返回 "<unknown>"。
func describeKBEmbeddingConfig(kbName string) string {
	initKBMeta()
	kbMetaMu.RLock()
	info, exists := kbMetaInMemory[kbName]
	kbMetaMu.RUnlock()
	if !exists || info == nil {
		return "<kb not found>"
	}

	// 解析实际使用的模型名（与 buildEmbeddingFuncWith 同优先级）
	model := strings.TrimSpace(info.EmbeddingModel)
	if model == "" {
		// 走全局默认逻辑，模型解析交由 buildEmbeddingFunc，此处仅尽力展示
		model = "<auto>"
	}

	// 反查 AIConfig 信息
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return fmt.Sprintf("<no AIConfig> model=%s", model)
	}

	if info.AIConfigID > 0 {
		for _, cfg := range settingConfig.AiConfigs {
			if cfg != nil && cfg.ID == info.AIConfigID {
				return fmt.Sprintf("aiConfig#%d(name=%s,type=%s,baseUrl=%s,model=%s)",
					cfg.ID, cfg.Name, cfg.ModelType, cfg.BaseUrl, model)
			}
		}
		return fmt.Sprintf("<aiConfig#%d not found> model=%s", info.AIConfigID, model)
	}

	// AIConfigID=0 走自动模式：复刻 buildEmbeddingFuncWith 的选取逻辑展示实际命中的配置
	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.ApiKey != "" && cfg.BaseUrl != "" && cfg.ModelType == "embedding" {
			effectiveModel := model
			if effectiveModel == "<auto>" && cfg.ModelName != "" {
				effectiveModel = cfg.ModelName
			}
			return fmt.Sprintf("aiConfig#%d(name=%s,type=embedding,baseUrl=%s,model=%s) [auto]",
				cfg.ID, cfg.Name, cfg.BaseUrl, effectiveModel)
		}
	}
	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.ApiKey != "" && cfg.BaseUrl != "" && strings.TrimSpace(cfg.EmbeddingModel) != "" {
			effectiveModel := model
			if effectiveModel == "<auto>" {
				effectiveModel = cfg.EmbeddingModel
			}
			return fmt.Sprintf("aiConfig#%d(name=%s,type=%s,baseUrl=%s,model=%s) [auto fallback: has EmbeddingModel]",
				cfg.ID, cfg.Name, cfg.ModelType, cfg.BaseUrl, effectiveModel)
		}
	}
	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.ApiKey != "" && cfg.BaseUrl != "" {
			effectiveModel := model
			if effectiveModel == "<auto>" {
				effectiveModel = "text-embedding-3-small [default, may not be supported]"
			}
			return fmt.Sprintf("aiConfig#%d(name=%s,type=%s,baseUrl=%s,model=%s) [auto fallback: no embedding service, this provider may NOT support /embeddings]",
				cfg.ID, cfg.Name, cfg.ModelType, cfg.BaseUrl, effectiveModel)
		}
	}
	return fmt.Sprintf("<no usable AIConfig> model=%s", model)
}

// ensureStockBasicInfoKB 确保 A股基础数据知识库存在（存在则跳过，不存在则创建）。
// 使用自动模式（aiConfigID=0）绑定全局默认 embedding 配置。
func ensureStockBasicInfoKB() error {
	initKBMeta()
	kbMetaMu.RLock()
	_, exists := kbMetaInMemory[StockBasicInfoKBName]
	kbMetaMu.RUnlock()
	if exists {
		return nil
	}
	_, err := CreateKnowledgeBase(StockBasicInfoKBName,
		"A股上市公司基础资料（东方财富F10 RPT_F10_ORG_BASICINFO），调用股票基础资料时自动入库",
		0, "")
	if err != nil {
		// 并发创建时另一个 goroutine 可能已创建成功
		kbMetaMu.RLock()
		_, exists2 := kbMetaInMemory[StockBasicInfoKBName]
		kbMetaMu.RUnlock()
		if exists2 {
			return nil
		}
		return err
	}
	return nil
}

// isSourceInKB 检查指定 source 是否已存在于 KB 的文档索引中（按 Source 字段精确匹配）。
func isSourceInKB(kbName, source string) bool {
	initKBMeta()
	kbMetaMu.RLock()
	defer kbMetaMu.RUnlock()
	info, exists := kbMetaInMemory[kbName]
	if !exists || info == nil {
		return false
	}
	for _, doc := range info.Documents {
		if doc.Source == source {
			return true
		}
	}
	return false
}

// normalizeStockCodeForKB 归一化股票代码作为 KB source 标记。
// 去除 .SH/.SZ/.BJ 后缀与 sh/sz/bj 前缀，使 "002008"/"002008.SZ"/"sz002008" 统一为 "002008"。
func normalizeStockCodeForKB(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	upper := strings.ToUpper(code)
	for _, suffix := range []string{".SH", ".SZ", ".BJ"} {
		upper = strings.TrimSuffix(upper, suffix)
	}
	for _, prefix := range []string{"SH", "SZ", "BJ"} {
		upper = strings.TrimPrefix(upper, prefix)
	}
	return upper
}
