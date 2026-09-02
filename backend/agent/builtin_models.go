package agent

import (
	"sort"
	"strings"
)

// 本文件提供模型上下文窗口与输出上限的内置参考表。
//
// 历史问题：原 getBuiltinModelMaxTokens（app.go）将「上下文窗口」与「输出上限」
// 混在同一张表里——部分模型的值是上下文窗口（如 deepseek-chat: 65536），部分是
// 输出上限（如 gpt-4o: 16384、claude-3-5-sonnet: 8192），导致该值被同时用于
// max_tokens API 参数和摘要阈值计算时产生冲突。
//
// 现拆分为两张独立表：GetBuiltinModelContextWindow 返回模型总上下文容量，
// GetBuiltinModelMaxOutput 返回模型单次生成的输出上限。两者均供 FetchAiModelInfo
// （app.go）和运行时 resolveContextWindow/resolveOutputMaxTokens 使用。
//
// 数据更新至 2026-08，覆盖 DeepSeek V4、GLM-5.2、GPT-5/5.5/5.6、Claude 5、
// Gemini 2.5/3 等主流模型。
//
// 数值口径统一采用十进制（宁小勿大）：
//
// 厂商宣传的「XK / XM」上下文或输出，统一按十进制换算（1K=1000、1M=1000000），
// 而非 1024 进制（1K=1024、1M=1048576）。原因：输出上限会直接作为 max_tokens 传给
// API，若填 1024 进制值（如 128K 填 131072）而厂商实际上限是十进制（128000），
// 会触发 400 InvalidParameter（智谱 GLM-5.2 已踩此坑）。统一用十进制可保证「即使
// 值偏小也不会超限报错」——上下文窗口偏小只会让摘要阈值略低、更早触发摘要（安全方向）；
// 输出上限偏小只会略微限制单次输出长度（可忽略）。

// builtinContextWindow 精确匹配的模型上下文窗口（输入+输出总容量）。
var builtinContextWindow = map[string]int{
	// ---- DeepSeek V4（2026-08 正式版，1M 上下文）----
	"deepseek-v4-flash": 1000000, // V4 Flash：1M 上下文
	"deepseek-v4-pro":   1000000, // V4 Pro：1M 上下文
	// deepseek-chat / deepseek-reasoner 旧别名现已指向 V4-Flash（非思考/思考模式）
	"deepseek-chat":     1000000,
	"deepseek-reasoner": 1000000,
	// DeepSeek V3（自部署兜底）
	"deepseek-v3":    64000,
	"deepseek-r1":    64000,
	"deepseek-coder": 16000,

	// ---- GPT-5 系列（2025-2026）----
	"gpt-5":      400000, // GPT-5：400K 上下文
	"gpt-5-mini": 400000,
	"gpt-5-nano": 400000,
	"gpt-5-pro":  400000,
	"gpt-5.1":    400000,
	"gpt-5.2":    400000,
	"gpt-5.3":    400000,
	"gpt-5.4":    400000,
	"gpt-5.5":    400000,
	"gpt-5.6":    1050000, // GPT-5.6：1.05M 上下文

	// ---- GPT-4 系列（保留）----
	"gpt-4o":              128000,
	"gpt-4o-mini":         128000,
	"gpt-4o-2024-05-13":   128000,
	"gpt-4-turbo":         128000,
	"gpt-4-turbo-preview": 128000,
	"gpt-4":               8000,
	"gpt-4-32k":           32000,
	"gpt-3.5-turbo":       16000,
	"gpt-3.5-turbo-16k":   16000,
	"gpt-4.1":             1000000,
	"gpt-4.1-mini":        1000000,
	"gpt-4.1-nano":        1000000,

	// ---- OpenAI o 系列 ----
	"o1":         200000,
	"o1-mini":    128000,
	"o1-preview": 128000,
	"o3":         200000,
	"o3-mini":    200000,
	"o3-pro":     200000,
	"o4-mini":    200000,

	// ---- Claude 5 系列（2026，1M 上下文）----
	"claude-opus-5":   1000000,
	"claude-sonnet-5": 1000000,
	"claude-fable-5":  1000000,

	// ---- Claude 4/3 系列（保留）----
	"claude-3-5-sonnet": 200000,
	"claude-3-5-haiku":  200000,
	"claude-3-opus":     200000,
	"claude-3-sonnet":   200000,
	"claude-3-haiku":    200000,

	// ---- GLM-5 系列（2026，GLM-5.2 支持 1M 上下文）----
	"glm-5.2":     1000000, // GLM-5.2：1M 上下文，128K 输出
	"glm-5.1":     200000,  // GLM-5.1：200K 上下文
	"glm-5":       200000,
	"glm-5-turbo": 128000,
	"glm-5-flash": 128000,

	// ---- GLM-4 系列（保留）----
	"glm-4":         128000,
	"glm-4-plus":    128000,
	"glm-4-air":     128000,
	"glm-4-flash":   128000,
	"glm-4-long":    1000000,
	"chatglm-turbo": 8000,

	// ---- Gemini 系列（2025-2026）----
	// Gemini 2.5 Pro 拥有 2M 上下文，是当前最大上下文窗口
	// Gemini 2.5 Flash / 3 Flash / 3.1 Pro 均为 1M 上下文

	// ---- Moonshot ----
	// Kimi K3：官方宣传 1M 上下文，按本文件约定记十进制；多模态，reasoning low/high/max
	"kimi-k3": 1000000,
	// Kimi K2.6 / K2.7 系列：官方 256K 上下文，记十进制
	"kimi-k2.7-code":           256000,
	"kimi-k2.7-code-highspeed": 256000,
	"kimi-k2.6":                256000,
	// 以下为已退役型号（调用返回 404），仅为兼容历史配置保留，不属于当前可用序列：
	// 早期 K2 系列 2026-05-25 退役；kimi-k2.5 与 moonshot-v1 系列 2026-08-31 退役
	"kimi-k2.5":              256000, // deprecated 2026-08-31
	"kimi-k2-thinking":       256000, // deprecated 2026-05-25
	"kimi-k2-thinking-turbo": 256000, // deprecated 2026-05-25
	"kimi-k2-0905-preview":   256000, // deprecated 2026-05-25
	"kimi-k2-0711-preview":   256000, // deprecated 2026-05-25
	"kimi-k2-turbo-preview":  256000, // deprecated 2026-05-25
	"moonshot-v1-8k":         8000,   // deprecated 2026-08-31
	"moonshot-v1-32k":        32000,  // deprecated 2026-08-31
	"moonshot-v1-128k":       128000, // deprecated 2026-08-31

	// ---- Qwen 系列 ----
	"qwen-turbo":           1000000,
	"qwen-plus":            128000,
	"qwen-max":             32000,
	"qwen-long":            1000000,
	"qwen2.5-72b-instruct": 128000,
	"qwen3-max":            128000,
	"qwen3-coder":          1000000, // Qwen3-Coder-480B：1M 上下文

	// ---- 腾讯混元 ----
	"hunyuan-lite":     4000,
	"hunyuan-standard": 32000,
	"hunyuan-pro":      32000,
	"hunyuan-turbo":    32000,

	// ---- 讯飞星火 ----
	"spark-lite":      4000,
	"spark-pro":       8000,
	"spark-max":       8000,
	"spark-4.0-ultra": 8000,

	// ---- 零一万物 ----
	"yi-light":  16000,
	"yi-large":  32000,
	"yi-medium": 16000,
	"yi-spark":  16000,
	"yi-vision": 16000,

	// ---- MiniMax ----
	"abab6.5-chat":  8000,
	"abab6.5s-chat": 24000,
	"abab5.5-chat":  8000,

	// ---- 百川 ----
	"baichuan2-turbo": 32000,
	"baichuan2-53b":   32000,

	// ---- 文心一言 ----
	"ernie-4.0":   8000,
	"ernie-3.5":   8000,
	"ernie-speed": 4000,
	"ernie-lite":  4000,
}

// builtinContextWindowPrefix 前缀匹配的上下文窗口，用于精确匹配未命中时兜底。
// 查找时按前缀长度降序排列，确保更具体的前缀优先匹配（如 "deepseek-v4-" 优先于 "deepseek"）。
var builtinContextWindowPrefix = map[string]int{
	"deepseek-v4":    1000000, // DeepSeek V4 系列
	"deepseek":       64000,   // DeepSeek V3 及更早（自部署兜底）
	"gpt-5.6":        1050000, // GPT-5.6：1.05M
	"gpt-5":          400000,  // GPT-5 系列（5/5.1-5.5）
	"gpt-4o":         128000,
	"gpt-4-turbo":    128000,
	"gpt-4.1":        1000000,
	"gpt-4-":         8000,
	"gpt-3.5":        16000,
	"o3-":            200000,
	"o4-":            200000,
	"o1-":            128000,
	"claude-opus-5":  1000000,
	"claude-sonnet":  1000000, // 覆盖 sonnet-5 和 sonnet-4.x
	"claude-fable":   1000000,
	"claude-3":       200000,
	"glm-5":          200000,  // GLM-5/5.1（5.2 由精确匹配命中）
	"kimi-k3":        1000000, // Kimi K3：1M 上下文（记十进制）
	"kimi-k2":        256000,  // Kimi K2 系列兜底（仅限已知 K2 系，不推断未来型号）
	"glm-4":          128000,
	"chatglm":        8000,
	"gemini-2.5-pro": 2000000, // Gemini 2.5 Pro：2M
	"gemini-2.5":     1000000, // Gemini 2.5 Flash 等
	"gemini-3":       1000000, // Gemini 3 系列
	"gemini-1.5":     1000000,
	"gemini-2":       1000000,
	"moonshot-v1":    8000,
	"qwen3":          128000, // Qwen3 系列
	"qwen-":          8000,
	"qwen2":          128000,
	"hunyuan-":       32000,
	"spark-":         8000,
	"yi-":            16000,
	"abab":           8000,
	"baichuan":       32000,
	"ernie-":         8000,
	"llama-3":        8000,
	"llama3":         8000,
	"mistral-":       32000,
	"mixtral-":       32000,
	"codestral-":     32000,
	"command-r":      128000,
	"Qwen/Qwen":      128000,
	"deepseek-ai/":   64000,
	"meta-llama/":    8000,
	"mistralai/":     32000,
	"Pro/deepseek-":  1000000, // 硅基流动等代理的 DeepSeek V4
	"Pro/qwen-":      128000,
}

// builtinMaxOutput 精确匹配的模型输出上限（单次生成的最大 token 数）。
var builtinMaxOutput = map[string]int{
	// ---- DeepSeek V4（384K 输出）----
	"deepseek-v4-flash": 384000, // V4 Flash：384K 输出
	"deepseek-v4-pro":   384000, // V4 Pro：384K 输出
	"deepseek-chat":     384000, // 旧别名→V4 Flash 非思考
	"deepseek-reasoner": 384000, // 旧别名→V4 Flash 思考
	// DeepSeek V3（自部署兜底）
	"deepseek-v3":    8000,
	"deepseek-r1":    32000,
	"deepseek-coder": 4000,

	// ---- GPT-5 系列 ----
	"gpt-5":      32000,
	"gpt-5-mini": 32000,
	"gpt-5-nano": 32000,
	"gpt-5-pro":  32000,
	"gpt-5.1":    32000,
	"gpt-5.2":    32000,
	"gpt-5.3":    32000,
	"gpt-5.4":    32000,
	"gpt-5.5":    32000,
	"gpt-5.6":    32000,

	// ---- GPT-4 系列（保留）----
	"gpt-4o":              16000,
	"gpt-4o-mini":         16000,
	"gpt-4o-2024-05-13":   4000,
	"gpt-4-turbo":         4000,
	"gpt-4-turbo-preview": 4000,
	"gpt-4":               4000,
	"gpt-4-32k":           4000,
	"gpt-3.5-turbo":       4000,
	"gpt-3.5-turbo-16k":   4000,
	"gpt-4.1":             32000,
	"gpt-4.1-mini":        32000,
	"gpt-4.1-nano":        32000,

	// ---- OpenAI o 系列 ----
	"o1":         100000,
	"o1-mini":    64000,
	"o1-preview": 32000,
	"o3":         100000,
	"o3-mini":    100000,
	"o3-pro":     100000,
	"o4-mini":    100000,

	// ---- Claude 5 系列 ----
	"claude-opus-5":   128000, // 128K 输出
	"claude-sonnet-5": 128000, // Sonnet 5 输出已提升至 128K
	"claude-fable-5":  128000,

	// ---- Claude 4/3 系列（保留）----
	"claude-3-5-sonnet": 8000,
	"claude-3-5-haiku":  8000,
	"claude-3-opus":     4000,
	"claude-3-sonnet":   4000,
	"claude-3-haiku":    4000,

	// ---- GLM-5 系列 ----
	"glm-5.2":     128000, // GLM-5.2：128K 输出（智谱 API max_tokens 上限为 128000，非 131072）
	"glm-5.1":     32000,
	"glm-5":       32000,
	"glm-5-turbo": 4000,
	"glm-5-flash": 4000,

	// ---- GLM-4 系列（保留）----
	"glm-4":         4000,
	"glm-4-plus":    4000,
	"glm-4-air":     4000,
	"glm-4-flash":   4000,
	"glm-4-long":    4000,
	"chatglm-turbo": 4000,

	// ---- Moonshot ----
	"kimi-k3": 131072, // 官方默认 max_completion_tokens（默认值，非绝对上限；官方允许最高 1048576）
	// K2.6 / K2.7 系列：官方默认 max_tokens = 32768
	"kimi-k2.7-code":           32768,
	"kimi-k2.7-code-highspeed": 32768,
	"kimi-k2.6":                32768,
	// 以下为已退役型号，仅为兼容历史配置保留
	"kimi-k2.5":              32768, // deprecated 2026-08-31
	"kimi-k2-thinking":       32768, // deprecated 2026-05-25
	"kimi-k2-thinking-turbo": 32768, // deprecated 2026-05-25
	"moonshot-v1-8k":         8000,  // deprecated 2026-08-31
	"moonshot-v1-32k":        8000,  // deprecated 2026-08-31
	"moonshot-v1-128k":       8000,  // deprecated 2026-08-31

	// ---- Qwen 系列 ----
	"qwen-turbo":           8000,
	"qwen-plus":            8000,
	"qwen-max":             8000,
	"qwen-long":            8000,
	"qwen2.5-72b-instruct": 8000,
	"qwen3-max":            8000,
	"qwen3-coder":          64000, // Qwen3-Coder：64K 输出

	// ---- 腾讯混元 ----
	"hunyuan-lite":     4000,
	"hunyuan-standard": 4000,
	"hunyuan-pro":      4000,
	"hunyuan-turbo":    4000,

	// ---- 讯飞星火 ----
	"spark-lite":      4000,
	"spark-pro":       4000,
	"spark-max":       4000,
	"spark-4.0-ultra": 4000,

	// ---- 零一万物 ----
	"yi-light":  4000,
	"yi-large":  4000,
	"yi-medium": 4000,
	"yi-spark":  4000,
	"yi-vision": 4000,

	// ---- MiniMax ----
	"abab6.5-chat":  8000,
	"abab6.5s-chat": 8000,
	"abab5.5-chat":  4000,

	// ---- 百川 ----
	"baichuan2-turbo": 4000,
	"baichuan2-53b":   4000,

	// ---- 文心一言 ----
	"ernie-4.0":   4000,
	"ernie-3.5":   4000,
	"ernie-speed": 4000,
	"ernie-lite":  4000,
}

// builtinMaxOutputPrefix 前缀匹配的输出上限。
var builtinMaxOutputPrefix = map[string]int{
	"kimi-k3":        131072, // Kimi K3：官方默认 max_completion_tokens
	"kimi-k2":        32768,  // Kimi K2 系列兜底（官方默认 max_tokens）
	"deepseek-v4":    384000, // DeepSeek V4：384K 输出
	"deepseek":       8000,   // DeepSeek V3 及更早
	"gpt-5.6":        32000,
	"gpt-5":          32000,
	"gpt-4o":         16000,
	"gpt-4-turbo":    4000,
	"gpt-4.1":        32000,
	"gpt-4-":         4000,
	"gpt-3.5":        4000,
	"o3-":            100000,
	"o4-":            100000,
	"o1-":            64000,
	"claude-opus-5":  128000,
	"claude-sonnet":  128000, // Sonnet 5 输出 128K
	"claude-fable":   128000,
	"claude-3":       8000,
	"glm-5.2":        128000,
	"glm-5":          32000,
	"glm-4":          4000,
	"chatglm":        4000,
	"gemini-2.5-pro": 64000, // Gemini 2.5 Pro：64K 输出
	"gemini-2.5":     64000,
	"gemini-3":       64000,
	"gemini-1.5":     8000,
	"gemini-2":       8000,
	"moonshot-v1":    8000,
	"qwen3":          8000,
	"qwen-":          8000,
	"qwen2":          8000,
	"hunyuan-":       4000,
	"spark-":         4000,
	"yi-":            4000,
	"abab":           8000,
	"baichuan":       4000,
	"ernie-":         4000,
	"llama-3":        8000,
	"llama3":         8000,
	"mistral-":       8000,
	"mixtral-":       8000,
	"codestral-":     8000,
	"command-r":      4000,
	"Qwen/Qwen":      8000,
	"deepseek-ai/":   8000,
	"meta-llama/":    8000,
	"mistralai/":     8000,
	"Pro/deepseek-":  384000, // 硅基流动等代理的 DeepSeek V4
	"Pro/qwen-":      8000,
}

// sortedPrefixes 按前缀长度降序排列 map 的 key，确保更具体（更长）的前缀优先匹配。
// 例如 "deepseek-v4" 优先于 "deepseek"，"gpt-5.6" 优先于 "gpt-5"。
func sortedPrefixes(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

// 缓存排序后的前缀列表，避免每次调用都重新排序。
var sortedContextWindowPrefixes = sortedPrefixes(builtinContextWindowPrefix)
var sortedMaxOutputPrefixes = sortedPrefixes(builtinMaxOutputPrefix)

// GetBuiltinModelContextWindow 返回模型上下文窗口大小（输入+输出总容量）。
// 先精确匹配（大小写不敏感），再按前缀匹配（长度降序，更具体优先）；未命中返回 0。
func GetBuiltinModelContextWindow(modelName string) int {
	if modelName == "" {
		return 0
	}
	lower := strings.ToLower(modelName)
	if v, ok := builtinContextWindow[lower]; ok {
		return v
	}
	for _, prefix := range sortedContextWindowPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return builtinContextWindowPrefix[prefix]
		}
	}
	return 0
}

// GetBuiltinModelMaxOutput 返回模型输出上限（单次生成的最大 token 数）。
// 先精确匹配（大小写不敏感），再按前缀匹配（长度降序，更具体优先）；未命中返回 0。
func GetBuiltinModelMaxOutput(modelName string) int {
	if modelName == "" {
		return 0
	}
	lower := strings.ToLower(modelName)
	if v, ok := builtinMaxOutput[lower]; ok {
		return v
	}
	for _, prefix := range sortedMaxOutputPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return builtinMaxOutputPrefix[prefix]
		}
	}
	return 0
}
