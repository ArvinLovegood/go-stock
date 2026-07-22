package data

import "strings"

const (
	AIProviderAuto    = "auto"
	AIProviderOpenAI  = "openai"
	AIProviderXAI     = "xai"
	AIProviderGeneric = "openai_compatible"
)

// ModelFamily is deliberately small: it captures only parameter semantics that
// differ at the OpenAI-compatible boundary. Provider-specific SDK routing stays
// in backend/agent/chat_model_factory.go.
type ModelFamily string

const (
	ModelFamilyGeneric ModelFamily = "generic"
	ModelFamilyGPT5    ModelFamily = "gpt5"
	ModelFamilyGrok    ModelFamily = "grok"
)

func (c AIConfig) Family() ModelFamily {
	model := strings.ToLower(strings.TrimSpace(c.ModelName))
	if strings.HasPrefix(model, "gpt-5") {
		return ModelFamilyGPT5
	}
	if strings.HasPrefix(model, "grok-4.5") {
		return ModelFamilyGrok
	}
	return ModelFamilyGeneric
}

func (c AIConfig) EffectiveMaxInputTokens() int {
	if c.MaxInputTokens > 0 {
		return c.MaxInputTokens
	}
	switch c.Family() {
	case ModelFamilyGPT5, ModelFamilyGrok:
		return 96000
	default:
		if c.MaxTokens <= 0 {
			return 120000
		}
		available := int(float64(c.MaxTokens) * 0.85)
		if available -= 20000; available >= 4000 {
			return available
		}
		return 4000
	}
}

func (c AIConfig) EffectiveMaxOutputTokens() int {
	if c.MaxOutputTokens > 0 {
		return c.MaxOutputTokens
	}
	switch c.Family() {
	case ModelFamilyGPT5:
		return 16384
	case ModelFamilyGrok:
		return 8192
	default:
		if c.MaxTokens > 0 {
			return c.MaxTokens
		}
		return 4096
	}
}

func (c AIConfig) EffectiveReasoningEffort() string {
	effort := strings.ToLower(strings.TrimSpace(c.ReasoningEffort))
	switch effort {
	case "none", "minimal", "low", "medium", "high", "xhigh":
		return effort
	}
	if c.Family() == ModelFamilyGPT5 || c.Family() == ModelFamilyGrok {
		return "medium"
	}
	return ""
}

// EffectiveTemperature returns nil when the parameter should be omitted. GPT-5
// reasoning models are most predictable when sampling is left to the provider.
func (c AIConfig) EffectiveTemperature() *float64 {
	if c.TemperatureOpt != nil {
		return c.TemperatureOpt
	}
	switch c.Family() {
	case ModelFamilyGPT5:
		return nil
	case ModelFamilyGrok:
		v := 0.2
		return &v
	default:
		if c.Temperature <= 0 {
			return nil
		}
		v := c.Temperature
		return &v
	}
}

func (c AIConfig) EffectiveTimeout() int {
	if c.TimeOut > 0 && c.TimeOut != 6000 {
		return c.TimeOut
	}
	if c.Family() == ModelFamilyGPT5 {
		return 900
	}
	if c.Family() == ModelFamilyGrok {
		return 600
	}
	if c.TimeOut > 0 {
		return c.TimeOut
	}
	return 300
}

// BuildOpenAICompatibleRequest centralizes request parameter semantics for the
// direct streaming path, avoiding drift between plain chat and tool calls.
func BuildOpenAICompatibleRequest(c AIConfig, messages []map[string]interface{}, tools []Tool, stream, reasoning bool) map[string]interface{} {
	body := map[string]interface{}{
		"model":    c.ModelName,
		"stream":   stream,
		"messages": messages,
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	if temp := c.EffectiveTemperature(); temp != nil {
		body["temperature"] = *temp
	}
	if out := c.EffectiveMaxOutputTokens(); out > 0 {
		if c.Family() == ModelFamilyGPT5 || c.Family() == ModelFamilyGrok {
			body["max_completion_tokens"] = out
		} else {
			body["max_tokens"] = out
		}
	}
	if reasoning {
		if effort := c.EffectiveReasoningEffort(); effort != "" {
			body["reasoning_effort"] = effort
		} else if c.Thinking {
			body["thinking"] = map[string]any{"type": "enabled"}
		}
	}
	return body
}
