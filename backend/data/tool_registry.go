package data

import (
	"strings"

	"github.com/samber/lo"
)

// ToolContext 封装一次工具调用时需要用到的上下文
type ToolContext struct {
	Question             string
	Messages             *[]map[string]any
	CurrentAIContent     *strings.Builder
	ReasoningContentText *strings.Builder
	CurrentCallID        string
	FuncName             string
	Ch                   chan map[string]any
	StreamResponseID     string
	Model                string
	Source               string
	SystemPrompt         string
}

// ToolHandler 统一的工具处理函数签名
type ToolHandler func(o *OpenAi, args string, ctx *ToolContext) error

var toolHandlers = map[string]ToolHandler{}

// registerToolHandler 注册一个工具处理函数
func registerToolHandler(name string, handler ToolHandler) {
	toolHandlers[name] = handler
}

// toolRequiredKey 工具名与所需 API Key 类型的映射
// key 为工具名，value 为所需的 API Key 类型标识
var toolRequiredKey = map[string]string{
	// IwencaiApiKey 依赖的工具（同花顺问财）
	"QueryIwencai":          "IwencaiApiKey",
	"QueryInsResearch":      "IwencaiApiKey",
	"QueryZhishu":           "IwencaiApiKey",
	"QueryEvent":            "IwencaiApiKey",
	"SelectAStock":          "IwencaiApiKey",
	"QueryMacro":            "IwencaiApiKey",
	"SelectSector":          "IwencaiApiKey",
	"QueryBasicInfo":        "IwencaiApiKey",
	"QueryFinance":          "IwencaiApiKey",
	"QueryIndustry":         "IwencaiApiKey",
	"QueryFutures":          "IwencaiApiKey",
	"SelectETF":             "IwencaiApiKey",
	"QueryManagement":       "IwencaiApiKey",
	"QueryStockConnect":     "IwencaiApiKey",
	"SelectFundManager":     "IwencaiApiKey",
	"SelectConvertibleBond": "IwencaiApiKey",
	"SelectFundCompany":     "IwencaiApiKey",
	"SelectFund":            "IwencaiApiKey",
	"SelectFuturesOption":   "IwencaiApiKey",
	"SelectHKStock":         "IwencaiApiKey",
	"SelectUSStock":         "IwencaiApiKey",
	"QueryFundFinance":      "IwencaiApiKey",
	"QueryBusinessData":     "IwencaiApiKey",

	// EmApiKey 依赖的工具（东方财富妙想）
	"StockEarningsReview":       "EmApiKey",
	"FinancialQA":               "EmApiKey",
	"IndustryResearch":          "EmApiKey",
	"TrackingReport":            "EmApiKey",
	"FinanceDataQuery":          "EmApiKey",
	"FinanceSearch":             "EmApiKey",
	"ComparableCompanyAnalysis": "EmApiKey",
	"HotspotDiscovery":          "EmApiKey",

	// QgqpBId 依赖的工具（东财用户标识，SearchStock系列）
	"SearchStockByIndicators": "QgqpBId",
	"SearchBk":                "QgqpBId",
	"SearchETF":               "QgqpBId",

	// DingRobot+DingPushEnable 依赖的工具（钉钉推送）
	"SendDingDingMessage": "DingRobot",
	"SendToDingDing":      "DingRobot",

	// FeishuRobot+FeishuPushEnable 依赖的工具（飞书推送）
	"SendFeishuMessage": "FeishuRobot",
	"SendToFeishu":      "FeishuRobot",
}

// isApiKeyConfigured 检查指定类型的 API Key 是否已配置
func isApiKeyConfigured(keyType string) bool {
	config := GetSettingConfig()
	if config == nil || config.Settings == nil {
		return false
	}
	switch keyType {
	case "IwencaiApiKey":
		return strings.TrimSpace(config.IwencaiApiKey) != ""
	case "EmApiKey":
		return strings.TrimSpace(config.EmApiKey) != ""
	case "QgqpBId":
		return strings.TrimSpace(config.QgqpBId) != ""
	case "DingRobot":
		return config.DingPushEnable && strings.TrimSpace(config.DingRobot) != ""
	case "FeishuRobot":
		return config.FeishuPushEnable && strings.TrimSpace(config.FeishuRobot) != ""
	}
	return true
}

// FilterToolsByApiKey 过滤掉未配置 API Key 的工具 Schema（用于 OpenAI 直连模式）
func FilterToolsByApiKey(tools []Tool) []Tool {
	return lo.Filter(tools, func(t Tool, _ int) bool {
		requiredKey, exists := toolRequiredKey[t.Function.Name]
		if !exists {
			return true // 无 Key 要求的工具保留
		}
		return isApiKeyConfigured(requiredKey)
	})
}

// IsToolKeyConfigured 检查单个工具所需的 API Key 是否已配置（用于 Eino Agent 模式）
func IsToolKeyConfigured(toolName string) bool {
	requiredKey, exists := toolRequiredKey[toolName]
	if !exists {
		return true
	}
	return isApiKeyConfigured(requiredKey)
}

// extractSystemPrompt 从消息列表中提取第一条 system 角色的消息内容，
// 用于在工具调用时关联保存推荐记录所使用的系统提示词。
func extractSystemPrompt(messages *[]map[string]any) string {
	if messages == nil {
		return ""
	}
	for _, m := range *messages {
		role, _ := m["role"].(string)
		if role != "system" {
			continue
		}
		if c, ok := m["content"].(string); ok && c != "" {
			return c
		}
	}
	return ""
}
