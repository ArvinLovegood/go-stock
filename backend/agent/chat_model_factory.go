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

func normalizeBaseURL(base string) string {
	return strings.TrimSuffix(strings.TrimSpace(base), "/")
}

func normalizeChatModelBaseURL(base string) string {
	base = normalizeBaseURL(base)
	return strings.TrimSuffix(base, "/chat/completions")
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
		})

	case providerDashScope:
		cfg := &qwen.ChatModelConfig{
			APIKey:    aiConfig.ApiKey,
			BaseURL:   baseURL,
			Model:     aiConfig.ModelName,
			MaxTokens: outputMaxTokens,
			Timeout:   timeout,
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
			APIKey:    aiConfig.ApiKey,
			BaseURL:   baseURL,
			Model:     aiConfig.ModelName,
			MaxTokens: outputMaxTokens,
			Timeout:   timeout,
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
			APIKey:    aiConfig.ApiKey,
			Model:     aiConfig.ModelName,
			MaxTokens: maxOut,
			HTTPClient: &http.Client{
				Timeout: timeout,
			},
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
		base := strings.TrimSpace(aiConfig.BaseUrl)
		if base == "" {
			base = "http://127.0.0.1:11434"
		}
		opt := &ollamaapi.Options{}
		if aiConfig.Temperature > 0 {
			opt.Temperature = temperature
		}
		cfg := &ollama.ChatModelConfig{
			BaseURL: base,
			Model:   aiConfig.ModelName,
			Timeout: timeout,
			Options: opt,
		}
		if aiConfig.Thinking {
			tv := ollamaapi.ThinkValue{Value: true}
			cfg.Thinking = &tv
		}
		return ollama.NewChatModel(ctx, cfg)

	case providerGemini:
		cc := &genai.ClientConfig{APIKey: aiConfig.ApiKey}
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
		}
		if httpClient := buildChatModelHTTPClient(timeout, aiConfig.ExtraHeaders, aiConfig.SessionId); httpClient != nil {
			deepseekCfg.HTTPClient = httpClient
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
			MaxTokens:   outputMaxTokens,
			Timeout:     timeout,
			Temperature: &temperature,
			ExtraFields: extraFields,
		}
		if httpClient := buildChatModelHTTPClient(timeout, aiConfig.ExtraHeaders, aiConfig.SessionId); httpClient != nil {
			cfg.HTTPClient = httpClient
		}
		return einoopenai.NewChatModel(ctx, cfg)
	}
}

// buildProxyHTTPClient 构建带代理的HTTP客户端，若未配置代理则返回nil
func buildProxyHTTPClient(timeout time.Duration) *http.Client {
	config := data.GetSettingConfig()
	if config == nil || !config.HttpProxyEnabled || config.HttpProxy == "" {
		return nil
	}
	proxyURL, err := url.Parse(config.HttpProxy)
	if err != nil {
		logger.SugaredLogger.Warnf("解析HTTP代理失败: %v", err)
		return nil
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: timeout,
	}
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

// buildChatModelHTTPClient 构建同时支持代理和自定义 Header 注入的 HTTP 客户端。
// 若两者均未配置则返回 nil。extraHeaders 为 JSON 格式字符串，sessionId 用于模板变量展开。
func buildChatModelHTTPClient(timeout time.Duration, extraHeaders, sessionId string) *http.Client {
	headers := data.ParseHeaders(extraHeaders)
	hasHeaders := len(headers) > 0

	config := data.GetSettingConfig()
	hasProxy := config != nil && config.HttpProxyEnabled && config.HttpProxy != ""

	if !hasHeaders && !hasProxy {
		return nil
	}

	var transport http.RoundTripper = http.DefaultTransport
	if hasProxy {
		proxyURL, err := url.Parse(config.HttpProxy)
		if err != nil {
			logger.SugaredLogger.Warnf("解析HTTP代理失败: %v", err)
		} else {
			transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	if hasHeaders {
		transport = &headerInjectTransport{
			base:      transport,
			headers:   headers,
			sessionId: sessionId,
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
