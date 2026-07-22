package data

import "testing"

func TestAIConfigEffectiveBudgetsForModernModels(t *testing.T) {
	tests := []struct {
		model      string
		wantInput  int
		wantOutput int
		wantEffort string
		wantTemp   *float64
	}{
		{model: "gpt-5.6-sol", wantInput: 96000, wantOutput: 16384, wantEffort: "medium", wantTemp: nil},
		{model: "grok-4.5", wantInput: 96000, wantOutput: 8192, wantEffort: "medium", wantTemp: float64Ptr(0.2)},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cfg := AIConfig{ModelName: tt.model, MaxTokens: 81920, Temperature: 0.1, Thinking: true}
			if got := cfg.EffectiveMaxInputTokens(); got != tt.wantInput {
				t.Fatalf("input budget = %d, want %d", got, tt.wantInput)
			}
			if got := cfg.EffectiveMaxOutputTokens(); got != tt.wantOutput {
				t.Fatalf("output budget = %d, want %d", got, tt.wantOutput)
			}
			if got := cfg.EffectiveReasoningEffort(); got != tt.wantEffort {
				t.Fatalf("reasoning effort = %q, want %q", got, tt.wantEffort)
			}
			wantTimeout := 900
			if tt.model == "grok-4.5" {
				wantTimeout = 600
			}
			if got := cfg.EffectiveTimeout(); got != wantTimeout {
				t.Fatalf("timeout = %d, want %d", got, wantTimeout)
			}
			gotTemp := cfg.EffectiveTemperature()
			if tt.wantTemp == nil {
				if gotTemp != nil {
					t.Fatalf("temperature = %v, want omitted", *gotTemp)
				}
			} else if gotTemp == nil || *gotTemp != *tt.wantTemp {
				t.Fatalf("temperature = %v, want %v", gotTemp, *tt.wantTemp)
			}
		})
	}
}

func TestAIConfigLegacyGenericCompatibility(t *testing.T) {
	cfg := AIConfig{ModelName: "deepseek-chat", MaxTokens: 8192, Temperature: 0.1, TimeOut: 300}
	if got := cfg.EffectiveMaxInputTokens(); got != 4000 {
		t.Fatalf("legacy input budget = %d, want 4000", got)
	}
	if got := cfg.EffectiveMaxOutputTokens(); got != 8192 {
		t.Fatalf("legacy output budget = %d, want 8192", got)
	}
	if got := cfg.EffectiveTemperature(); got == nil || *got != 0.1 {
		t.Fatalf("legacy temperature = %v, want 0.1", got)
	}
}

func TestAIConfigExplicitValuesOverrideDefaults(t *testing.T) {
	temp := 0.35
	cfg := AIConfig{
		ModelName:       "gpt-5.6-sol",
		MaxInputTokens:  120000,
		MaxOutputTokens: 24000,
		ReasoningEffort: "high",
		TemperatureOpt:  &temp,
	}
	if got := cfg.EffectiveMaxInputTokens(); got != 120000 {
		t.Fatalf("input budget = %d", got)
	}
	if got := cfg.EffectiveMaxOutputTokens(); got != 24000 {
		t.Fatalf("output budget = %d", got)
	}
	if got := cfg.EffectiveReasoningEffort(); got != "high" {
		t.Fatalf("reasoning effort = %q", got)
	}
	if got := cfg.EffectiveTemperature(); got == nil || *got != temp {
		t.Fatalf("temperature = %v", got)
	}
}

func TestBuildOpenAICompatibleRequestUsesProviderAwareParameters(t *testing.T) {
	messages := []map[string]interface{}{{"role": "user", "content": "hello"}}

	gpt := AIConfig{ModelName: "gpt-5.6-sol", MaxTokens: 81920, Temperature: 0.1, Thinking: true}
	body := BuildOpenAICompatibleRequest(gpt, messages, nil, true, true)
	if _, ok := body["thinking"]; ok {
		t.Fatal("modern GPT request must not send non-standard thinking")
	}
	if _, ok := body["temperature"]; ok {
		t.Fatal("GPT reasoning request should omit temperature by default")
	}
	if got := body["reasoning_effort"]; got != "medium" {
		t.Fatalf("reasoning_effort = %v", got)
	}
	if got := body["max_completion_tokens"]; got != 16384 {
		t.Fatalf("max_completion_tokens = %v", got)
	}

	grok := AIConfig{ModelName: "grok-4.5", MaxTokens: 81920, Temperature: 0.1, Thinking: true}
	body = BuildOpenAICompatibleRequest(grok, messages, []Tool{{Type: "function"}}, true, true)
	if got := body["temperature"]; got != 0.2 {
		t.Fatalf("temperature = %v", got)
	}
	if got := body["reasoning_effort"]; got != "medium" {
		t.Fatalf("reasoning_effort = %v", got)
	}
	if got := body["max_completion_tokens"]; got != 8192 {
		t.Fatalf("max_completion_tokens = %v", got)
	}
}

func float64Ptr(v float64) *float64 { return &v }
