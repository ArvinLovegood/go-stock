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
	if effective := aiConfig.EffectiveTemperature(); effective != nil {
		temperature = float32(*effective)
	}
	timeout := time.Duration(aiConfig.EffectiveTimeout()) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	maxTok := aiConfig.EffectiveMaxOutputTokens()
	if maxTok <= 0 {
		maxTok = 4096
	}

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
			MaxTokens:   &maxTok,
			Temperature: &temperature,
			Thinking:    thinking,
			Timeout:     &timeout,
		})

	case providerDashScope:
		cfg := &qwen.ChatModelConfig{
			APIKey:    aiConfig.ApiKey,
			BaseURL:   baseURL,
			Model:     aiConfig.ModelName,
			Timeout:   timeout,
			MaxTokens: &maxTok,
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
			Timeout:   timeout,
			MaxTokens: &maxTok,
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
		maxOut := aiConfig.MaxTokens
		if maxOut <= 0 {
			maxOut = 8192
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
		if maxTok > 0 {
			gcfg.MaxTokens = &maxTok
		}
		if aiConfig.Temperature > 0 {
			gcfg.Temperature = ptrFloat32(temperature)
		}
		if aiConfig.Thinking {
			gcfg.ThinkingConfig = &genai.ThinkingConfig{IncludeThoughts: true}
		}
		return gemini.NewChatModel(ctx, gcfg)

	case providerDeepSeek:
		deepseekCfg := &deepseek.ChatModelConfig{
			BaseURL:     baseURL,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			MaxTokens:   maxTok,
			Temperature: temperature,
			Timeout:     timeout,
		}
		if proxyClient := buildProxyHTTPClient(timeout); proxyClient != nil {
			deepseekCfg.HTTPClient = proxyClient
		}
		return deepseek.NewChatModel(ctx, deepseekCfg)

	default:
		extraFields := map[string]any{}
		if aiConfig.Thinking && aiConfig.EffectiveReasoningEffort() != "" {
			extraFields["reasoning_effort"] = aiConfig.EffectiveReasoningEffort()
		} else if aiConfig.Thinking {
			logger.SugaredLogger.Warnf("generic OpenAI-compatible agent model %q ignores thinking option to keep request parameters standard", aiConfig.ModelName)
		}
		cfg := &einoopenai.ChatModelConfig{
			BaseURL:     baseURL,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			Timeout:     timeout,
			ExtraFields: extraFields,
		}
		if aiConfig.Family() == data.ModelFamilyGPT5 || aiConfig.Family() == data.ModelFamilyGrok {
			extraFields["max_completion_tokens"] = maxTok
		} else {
			cfg.MaxTokens = &maxTok
		}
		if aiConfig.EffectiveTemperature() != nil {
			cfg.Temperature = &temperature
		}
		if proxyClient := buildProxyHTTPClient(timeout); proxyClient != nil {
			cfg.HTTPClient = proxyClient
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
