package agent

import "testing"

// TestBuiltinKimiModelMetadata 覆盖 Kimi 精确匹配、前缀兜底与未知型号行为，
// 防止模型元数据表和前缀优先级被改坏。
func TestBuiltinKimiModelMetadata(t *testing.T) {
	ctxCases := []struct {
		model string
		want  int
	}{
		{"kimi-k3", 1000000},         // 精确匹配（十进制约定）
		{"Kimi-K3", 1000000},         // 大小写不敏感
		{"kimi-k2.7-code", 256000},   // 精确匹配
		{"kimi-k2.6", 256000},        // 精确匹配
		{"kimi-k2.9-future", 256000}, // 未知 K2 系走 kimi-k2 前缀兜底
		{"kimi-k9", 0},               // 非已知系列不做推断
		{"moonshot-v1-8k", 8000},     // 历史型号仍兼容
		{"totally-unknown-model", 0}, // 未知返回 0
	}
	for _, c := range ctxCases {
		if got := GetBuiltinModelContextWindow(c.model); got != c.want {
			t.Errorf("GetBuiltinModelContextWindow(%q) = %d, want %d", c.model, got, c.want)
		}
	}

	outCases := []struct {
		model string
		want  int
	}{
		{"kimi-k3", 131072},       // 官方默认 max_completion_tokens
		{"kimi-k2.7-code", 32768}, // 官方默认 max_tokens
		{"kimi-k2.7-code-highspeed", 32768},
		{"kimi-k2.6", 32768},
		{"kimi-k2.9-future", 32768}, // kimi-k2 前缀兜底
		{"kimi-k9", 0},              // 非已知系列不做推断
		{"totally-unknown-model", 0},
	}
	for _, c := range outCases {
		if got := GetBuiltinModelMaxOutput(c.model); got != c.want {
			t.Errorf("GetBuiltinModelMaxOutput(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}

// TestBuiltinNewModelMetadata 覆盖 2026-08 ~ 2026-10 新增模型：
// GPT-6/6.1、DeepSeek V4.1 Flash、GLM-5.3/4.7、Claude Sonnet 5.5/Haiku 5.5、
// Qwen3.8-Max、MiniMax M3，以及 gpt-5.6/glm-5 系列修正后的输出上限。
func TestBuiltinNewModelMetadata(t *testing.T) {
	ctxCases := []struct {
		model string
		want  int
	}{
		{"gpt-6.1-sol", 1050000}, // 精确匹配
		{"gpt-6-luna", 1050000},  // 精确匹配
		{"gpt-6-turbo", 1050000}, // 未知 GPT-6 型号走 gpt-6 前缀兜底
		{"gpt-5.6", 1050000},     // GPT-5.6 系列 1.05M 上下文
		{"deepseek-v4.1-flash", 1000000},
		{"deepseek-flash", 1000000}, // 官方新别名
		{"glm-5.3", 1000000},
		{"glm-5.3-flashx", 1000000},
		{"glm-5.9-future", 200000}, // 未知 GLM-5 型号走 glm-5 前缀兜底（200K）
		{"glm-5-turbo", 200000},    // 官方概览修正：200K 上下文
		{"glm-4.7-flash", 200000},
		{"claude-sonnet-5-5", 1000000},
		{"claude-haiku-5-5", 1000000},
		{"claude-haiku-5-9-future", 1000000}, // claude-haiku-5 前缀兜底
		{"qwen3.8-max", 1000000},
		{"qwen3.8-max-0915", 1000000}, // qwen3.8 前缀兜底
		{"minimax-m3", 1000000},
		{"minimax-m3.1-flash-preview", 1000000},
	}
	for _, c := range ctxCases {
		if got := GetBuiltinModelContextWindow(c.model); got != c.want {
			t.Errorf("GetBuiltinModelContextWindow(%q) = %d, want %d", c.model, got, c.want)
		}
	}

	outCases := []struct {
		model string
		want  int
	}{
		{"gpt-6.1-sol", 128000},
		{"gpt-6-turbo", 128000},  // gpt-6 前缀兜底
		{"gpt-5.6-luna", 128000}, // gpt-5.6 前缀兜底（原 32000 过时）
		{"deepseek-v4.1-flash", 384000},
		{"deepseek-flash", 384000},
		{"glm-5.3-flashx", 128000},
		{"glm-5-turbo", 128000}, // 官方概览修正：原 4000 过时
		{"glm-4.7-flash", 128000},
		{"glm-4.7-flashx", 128000},
		{"claude-sonnet-5-5", 128000},
		{"claude-haiku-5-5", 128000},
		{"qwen3.8-max", 128000},
		{"qwen3.8-max-0915", 128000}, // qwen3.8 前缀兜底
		{"minimax-m3", 131072},       // 官方推荐 max_completion_tokens
		{"minimax-m3.1-flash-preview", 131072},
		{"minimax-m3.9-future", 131072}, // minimax-m3 前缀兜底
	}
	for _, c := range outCases {
		if got := GetBuiltinModelMaxOutput(c.model); got != c.want {
			t.Errorf("GetBuiltinModelMaxOutput(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}
