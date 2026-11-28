package agent

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/openrouter"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/duke-git/lancet/v2/strutil"
	ollamaapi "github.com/eino-contrib/ollama/api"
	"google.golang.org/genai"
)

type chatModelProvider int

const (
	providerOpenAICompatible chatModelProvider = iota
	providerVolcArk
	providerDashScope
	providerOpenRouter
	providerAnthropic
	providerOllama
	providerGemini
	providerDeepSeek
)

// maxOllamaNumCtx Ollama num_ctx 解析值（非用户显式配置）的默认上限。
// KV cache 显存占用随 num_ctx 线性增长，resolveContextWindow 对未知模型兜底
// 64000，直接透传可能撑爆本地显存/内存导致加载失败或推理骤降。
const maxOllamaNumCtx = 32768

// minOllamaNumCtx Ollama num_ctx 解析值的下限。旧配置兜底路径可能把小
// MaxTokens（如 100）误判为上下文窗口，透传会造成灾难性头部截断；Agent 的
// 工具 schema 本身就需要约 8k token，低于此值 Agent 无法正常工作。
const minOllamaNumCtx = 8192

func normalizeBaseURL(base string) string {
	return strings.TrimSuffix(strings.TrimSpace(base), "/")
}

func normalizeChatModelBaseURL(base string) string {
	base = normalizeBaseURL(base)
	return strings.TrimSuffix(base, "/chat/completions")
}

// normalizeOllamaBaseURL 归一化 Ollama 原生 API 的 BaseURL。
//
// Ollama 原生 API 端点固定在根路径（/api/chat、/api/generate），用户可能误填
// OpenAI 风格路径（如 /v1 或 /v1/chat/completions），导致 url.JoinPath 拼出
// /v1/api/chat 等不存在的端点，返回 Gin 默认 404 "404 page not found"。
// eino-contrib/ollama 的 stream 函数逐行 JSON 反序列化时，"404" 被解析为
// 有效 JSON 数字，随后的 'p'（page）触发 "invalid character 'p' after top-level value"。
func normalizeOllamaBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return "http://127.0.0.1:11434"
	}
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimSuffix(base, "/")
	}
	path := u.Path
	// 迭代剥离已知的 OpenAI 风格路径后缀（先去尾斜杠再匹配，支持多层组合）
	for {
		path = strings.TrimSuffix(path, "/")
		switch {
		case strings.HasSuffix(path, "/chat/completions"):
			path = strings.TrimSuffix(path, "/chat/completions")
		case path == "/v1" || strings.HasSuffix(path, "/v1"):
			path = strings.TrimSuffix(path, "/v1")
		default:
			goto done
		}
	}
done:
	u.Path = path
	u.RawPath = ""
	result := u.String()
	if result != base {
		logger.SugaredLogger.Infof("normalizeOllamaBaseURL: %q -> %q (已剥离 OpenAI 风格路径后缀)", base, result)
	}
	return result
}

func detectChatModelProvider(baseLower, modelName string) chatModelProvider {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))

	if strings.Contains(baseLower, "volces.com") && strings.Contains(baseLower, "ark") {
		return providerVolcArk
	}
	if strings.Contains(baseLower, "dashscope.aliyuncs.com") ||
		strings.Contains(baseLower, "dashscope-intl.aliyuncs.com") {
		return providerDashScope
	}
	if strings.Contains(baseLower, "openrouter.ai") {
		return providerOpenRouter
	}
	if strings.Contains(baseLower, "anthropic.com") || strings.Contains(baseLower, "api.anthropic") {
		return providerAnthropic
	}
	if strings.Contains(baseLower, ":11434") || strings.Contains(baseLower, "ollama") {
		return providerOllama
	}
	if isGeminiGoogleAI(baseLower, modelLower) {
		return providerGemini
	}
	if strings.Contains(baseLower, "api.deepseek.com") ||
		strutil.ContainsAny(modelLower, []string{"deepseek", "deepseek-v", "deepseek-r", "deepseek-chat", "deepseek-coder", "deepseek-reasoner"}) {
		return providerDeepSeek
	}
	return providerOpenAICompatible
}

func isGeminiGoogleAI(baseLower, modelLower string) bool {
	if strings.Contains(baseLower, "generativelanguage.googleapis.com") ||
		strings.Contains(baseLower, "ai.google.dev") {
		return true
	}
	if strings.HasPrefix(modelLower, "gemini-") || strings.HasPrefix(modelLower, "gemini/") ||
		strings.HasPrefix(modelLower, "models/gemini") {
		return baseLower == "" || strings.Contains(baseLower, "googleapis.com")
	}
	return false
}

func parseAccessSecret(apiKey string) (ak, sk string) {
	s := strings.TrimSpace(apiKey)
	if s == "" {
		return "", ""
	}
	for _, sep := range []string{"|", ";"} {
		if idx := strings.Index(s, sep); idx > 0 && idx < len(s)-len(sep) {
			return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(sep):])
		}
	}
	if idx := strings.Index(s, ":"); idx > 0 && idx < len(s)-1 {
		// 避免误拆 http(s)://
		prefix := strings.ToLower(s[:idx])
		if strings.HasPrefix(prefix, "http") {
			return s, ""
		}
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return s, ""
}

func ptrFloat32(v float32) *float32 { return &v }
func ptrBool(v bool) *bool          { return &v }

// isOpenAIReasoningStyleModel 判断模型是否为 OpenAI 推理系。
// 推理系模型（o1/o3/o4 系列与 gpt-5 及以上）不接受 max_tokens 参数，
// 携带会返回 400："Unsupported parameter: 'max_tokens' is not supported with this
// model. Use 'max_completion_tokens' instead."，输出上限必须经 max_completion_tokens
// 下发（该上限包含推理 token）。gpt-4o/gpt-4.1/gpt-3.5 等仍支持 max_tokens，不受影响。
func isOpenAIReasoningStyleModel(modelName string) bool {
	m := strings.ToLower(strings.TrimSpace(modelName))
	// o 系列后须跟分隔符/数字，避免误匹配 openvoice 之类同前缀名字
	for _, p := range []string{"o1", "o3", "o4"} {
		if strings.HasPrefix(m, p) {
			rest := m[len(p):]
			if rest == "" || rest[0] == '-' || rest[0] == '.' || (rest[0] >= '0' && rest[0] <= '9') {
				return true
			}
		}
	}
	// gpt-5/6/7...（含 gpt-6.1-sol、gpt-5-mini 等小版本/变体）均为推理系
	for i := 5; i <= 9; i++ {
		if strings.HasPrefix(m, fmt.Sprintf("gpt-%d", i)) {
			return true
		}
	}
	return false
}

// createChatModel 按 Eino 生态组件路由（参见 https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/chat_model/ ）
// 未命中专用实现时回退到 OpenAI 兼容 ChatModel（硅基流动、LM Studio、Azure OpenAI 等）。
func createChatModel(ctx context.Context, aiConfig data.AIConfig) (model.ToolCallingChatModel, error) {
	baseURL := normalizeChatModelBaseURL(aiConfig.BaseUrl)
	baseLower := strings.ToLower(baseURL)
	temperature := float32(aiConfig.Temperature)
	timeout := time.Duration(aiConfig.TimeOut) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	// 输出 token 上限：通过 resolveOutputMaxTokens 解析，确保不超过上下文窗口。
	// resolveOutputMaxTokens 优先用 aiConfig.MaxTokens（若 < contextWindow），
	// 其次内置模型表，最后安全默认。resolveContextWindow 同理解析上下文窗口。
	contextWindow := resolveContextWindow(aiConfig)
	resolvedOutput := resolveOutputMaxTokens(aiConfig, contextWindow)
	if resolvedOutput > contextWindow {
		resolvedOutput = contextWindow / 2 // 安全兜底：至少留一半给输入
	}
	outputMaxTokens := &resolvedOutput

	// HTTP 客户端统一构建一次：代理是否生效由 AI 配置项的代理开关决定（单项优先级最高），
	// transport 恒为显式实现，屏蔽 HTTP_PROXY/HTTPS_PROXY 环境变量对 AI 请求的隐式劫持。
	httpClient := buildChatModelHTTPClient(timeout, aiConfig)

	p := detectChatModelProvider(baseLower, aiConfig.ModelName)
	logger.SugaredLogger.Infof("createChatModel provider=%d base=%q model=%q", p, aiConfig.BaseUrl, aiConfig.ModelName)

	switch p {
	case providerVolcArk:
		var thinking *ark.Thinking
		if aiConfig.Thinking {
			thinking = &ark.Thinking{Type: "enabled"}
		}
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			BaseURL:     baseURL,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			MaxTokens:   outputMaxTokens,
			Temperature: &temperature,
			Thinking:    thinking,
			Timeout:     &timeout,
			HTTPClient:  httpClient,
		})

	case providerDashScope:
		cfg := &qwen.ChatModelConfig{
			APIKey:     aiConfig.ApiKey,
			BaseURL:    baseURL,
			Model:      aiConfig.ModelName,
			MaxTokens:  outputMaxTokens,
			Timeout:    timeout,
			HTTPClient: httpClient,
		}
		if aiConfig.Temperature > 0 {
			cfg.Temperature = ptrFloat32(temperature)
		}
		if aiConfig.Thinking {
			cfg.EnableThinking = ptrBool(true)
		}
		return qwen.NewChatModel(ctx, cfg)

	case providerOpenRouter:
		cfg := &openrouter.Config{
			APIKey:     aiConfig.ApiKey,
			BaseURL:    baseURL,
			Model:      aiConfig.ModelName,
			MaxTokens:  outputMaxTokens,
			Timeout:    timeout,
			HTTPClient: httpClient,
		}
		if aiConfig.Temperature > 0 {
			cfg.Temperature = ptrFloat32(temperature)
		}
		if aiConfig.Thinking {
			enabled := true
			cfg.Reasoning = &openrouter.Reasoning{Enabled: &enabled}
		}
		return openrouter.NewChatModel(ctx, cfg)

	case providerAnthropic:
		// Anthropic API 强制要求 max_tokens（无模型默认值）。推理模型思考内容占用
		// 大量输出预算，故适当调高上限，避免最终回答被截断。
		maxOut := 8000
		if outputMaxTokens != nil {
			maxOut = *outputMaxTokens
		}
		cfg := &claude.Config{
			APIKey:     aiConfig.ApiKey,
			Model:      aiConfig.ModelName,
			MaxTokens:  maxOut,
			HTTPClient: httpClient,
		}
		if aiConfig.Temperature > 0 {
			cfg.Temperature = ptrFloat32(temperature)
		}
		if b := baseURL; b != "" {
			cfg.BaseURL = &b
		}
		if aiConfig.Thinking {
			cfg.Thinking = &claude.Thinking{Enable: true, BudgetTokens: min(maxOut, 32000)}
		}
		return claude.NewChatModel(ctx, cfg)

	case providerOllama:
		base := normalizeOllamaBaseURL(aiConfig.BaseUrl)
		opt := &ollamaapi.Options{}
		if aiConfig.Temperature > 0 {
			opt.Temperature = temperature
		}
		// num_ctx：Ollama 默认上下文仅 4096，而 Agent 注入的工具 schema + 系统提示
		// 轻松超过 8000 token，超出部分被 Ollama 静默截断（丢弃最头部内容，即系统
		// 提示与工具定义），表现为模型无视指令、工具调用格式错乱等诡异行为。
		// 显式透传上下文窗口，与 React/PlanExecute/DeepAgents 的 token 预算计算对齐：
		//   - 用户显式配置 ContextWindow > 0：直接使用（用户了解自己硬件，不设上限）
		//   - 否则用解析值（内置表/旧 MaxTokens/默认 64000），但封顶 maxOllamaNumCtx
		opt.NumCtx = aiConfig.ContextWindow
		if opt.NumCtx <= 0 {
			opt.NumCtx = contextWindow
			if opt.NumCtx > maxOllamaNumCtx {
				opt.NumCtx = maxOllamaNumCtx
			}
			if opt.NumCtx < minOllamaNumCtx {
				opt.NumCtx = minOllamaNumCtx
			}
		}
		// num_predict：输出上限与云端 max_tokens 语义对齐，避免单轮输出挤占上下文
		opt.NumPredict = resolvedOutput
		logger.SugaredLogger.Infof("createChatModel ollama: num_ctx=%d num_predict=%d (config_ctx=%d resolved_ctx=%d)",
			opt.NumCtx, opt.NumPredict, aiConfig.ContextWindow, contextWindow)
		cfg := &ollama.ChatModelConfig{
			BaseURL:    base,
			Model:      aiConfig.ModelName,
			Timeout:    timeout,
			Options:    opt,
			HTTPClient: httpClient,
		}
		if aiConfig.Thinking {
			tv := ollamaapi.ThinkValue{Value: true}
			cfg.Thinking = &tv
		}
		return ollama.NewChatModel(ctx, cfg)

	case providerGemini:
		cc := &genai.ClientConfig{APIKey: aiConfig.ApiKey, HTTPClient: httpClient}
		if b := baseURL; b != "" {
			cc.HTTPOptions = genai.HTTPOptions{BaseURL: b}
		}
		client, err := genai.NewClient(ctx, cc)
		if err != nil {
			return nil, fmt.Errorf("gemini genai client: %w", err)
		}
		gcfg := &gemini.Config{
			Client: client,
			Model:  aiConfig.ModelName,
		}
		if aiConfig.Temperature > 0 {
			gcfg.Temperature = ptrFloat32(temperature)
		}
		if outputMaxTokens != nil {
			gcfg.MaxTokens = outputMaxTokens
		}
		if aiConfig.Thinking {
			gcfg.ThinkingConfig = &genai.ThinkingConfig{IncludeThoughts: true}
		}
		return gemini.NewChatModel(ctx, gcfg)

	case providerDeepSeek:
		// 视觉模型改走 OpenAI 兼容组件：eino-ext deepseek 专用组件（v0.1.7）不支持下发
		// image_url 内容块（UserInputMultiContent 被拼接为纯文本，图片丢失），而 DeepSeek
		// API 完全 OpenAI 兼容（参考 https://api-docs.deepseek.com/zh-cn/guides/vision/），
		// OpenAI 兼容组件可将 UserInputMultiContent 转为标准 vision 内容块。
		if aiConfig.SupportVision {
			cfg := &einoopenai.ChatModelConfig{
				BaseURL:     baseURL,
				Model:       aiConfig.ModelName,
				APIKey:      aiConfig.ApiKey,
				MaxTokens:   outputMaxTokens,
				Timeout:     timeout,
				Temperature: &temperature,
				HTTPClient:  httpClient,
			}
			logger.SugaredLogger.Infof("createChatModel: deepseek vision model %q uses OpenAI-compatible component", aiConfig.ModelName)
			return einoopenai.NewChatModel(ctx, cfg)
		}
		// deepseek 的 MaxTokens 为 int,omitempty：0 时字段被省略 → 走 API 默认。
		var dsMax int
		if outputMaxTokens != nil {
			dsMax = *outputMaxTokens
		}
		deepseekCfg := &deepseek.ChatModelConfig{
			BaseURL:     baseURL,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			MaxTokens:   dsMax,
			Temperature: temperature,
			Timeout:     timeout,
			HTTPClient:  httpClient,
		}
		return deepseek.NewChatModel(ctx, deepseekCfg)

	default:
		extraFields := map[string]any{}
		if aiConfig.Thinking {
			logger.SugaredLogger.Warnf("generic OpenAI-compatible agent model %q ignores thinking option to keep request parameters standard", aiConfig.ModelName)
		}
		cfg := &einoopenai.ChatModelConfig{
			BaseURL:     baseURL,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			Timeout:     timeout,
			Temperature: &temperature,
			ExtraFields: extraFields,
			HTTPClient:  httpClient,
		}
		if isOpenAIReasoningStyleModel(aiConfig.ModelName) {
			// OpenAI 推理系模型拒绝 max_tokens，输出上限必须经 max_completion_tokens 下发
			cfg.MaxCompletionTokens = outputMaxTokens
			logger.SugaredLogger.Infof("createChatModel: reasoning-style model %q uses max_completion_tokens=%d", aiConfig.ModelName, resolvedOutput)
		} else {
			cfg.MaxTokens = outputMaxTokens
		}
		return einoopenai.NewChatModel(ctx, cfg)
	}
}

// resolveAIProxyForConfig 解析单个 AI 配置实际生效的代理地址（空串表示直连）。
//
// AI 配置项的代理开关优先级最高：
//   - 关闭：直连，即使全局代理已启用
//   - 开启：仅使用配置项内填写的代理地址；未填地址视为直连，不回退全局代理
func resolveAIProxyForConfig(aiConfig *data.AIConfig) string {
	if aiConfig == nil {
		return ""
	}
	return resolveAIProxyAddr(aiConfig.HttpProxyEnabled, aiConfig.HttpProxy)
}

// resolveAIProxyAddr 按优先级解析实际生效的代理地址（空串表示直连）：
// 单项关闭 → 直连（全局代理不劫持 AI 请求）；单项开启 → 仅使用单项代理地址，
// 不回退全局代理地址（AI 配置有专属地址输入框，开启但未填视为直连）。
func resolveAIProxyAddr(itemEnabled bool, itemProxy string) string {
	if !itemEnabled {
		return ""
	}
	return strings.TrimSpace(itemProxy)
}

// buildChatModelHTTPClient 构建带代理解析与自定义 Header 注入的 HTTP 客户端。
// 代理是否生效由 AI 配置项的代理开关决定（见 resolveAIProxyForConfig）。
// extraHeaders 为 JSON 格式字符串，sessionId 用于 Header 模板变量展开。
func buildChatModelHTTPClient(timeout time.Duration, aiConfig data.AIConfig) *http.Client {
	headers := data.ParseHeaders(aiConfig.ExtraHeaders)

	proxyURL := resolveAIProxyForConfig(&aiConfig)
	configuredTransport := newChatModelTransport(proxyURL)

	var roundTripper http.RoundTripper = configuredTransport
	if len(headers) > 0 {
		roundTripper = &headerInjectTransport{
			base:      roundTripper,
			headers:   headers,
			sessionId: aiConfig.SessionId,
		}
	}

	return &http.Client{
		Transport: roundTripper,
		Timeout:   timeout,
	}
}

// newChatModelTransport clones the standard transport but clears its
// environment-derived proxy. proxyURL 为空时直连；非空时显式走该代理。
// 恒返回显式 transport（Proxy 不为 nil 的 DefaultTransport），使 SDK 回退
// http.DefaultTransport 时不会因 ProxyFromEnvironment 静默走上系统代理。
func newChatModelTransport(proxyURL string) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = nil

	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return transport
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		logger.SugaredLogger.Warnf("解析HTTP代理失败: %v", err)
		return transport
	}
	transport.Proxy = http.ProxyURL(parsed)
	return transport
}

// headerInjectTransport 包装 http.RoundTripper，在每次请求时注入自定义 Header
// （支持模板变量展开，如 {{sessionId}}、{{uuid}}）。
type headerInjectTransport struct {
	base      http.RoundTripper
	headers   map[string]string // 含模板变量的原始 header 值
	sessionId string
}

func (t *headerInjectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for k, v := range t.headers {
		cloned.Header.Set(k, data.ExpandHeaderVars(v, t.sessionId))
	}
	return t.base.RoundTrip(cloned)
}
