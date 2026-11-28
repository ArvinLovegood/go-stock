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
		// ---- 2026-11 新增 ----
		{"gpt-6.2-astra", 1050000},         // GPT-6.2 Astra（2026-11-17）
		{"gpt-6.2-sol", 1050000},           // GPT-6.2 Sol
		{"gpt-6.2-pro-future", 1050000},    // gpt-6.2 前缀兜底
		{"glm-5.4", 1000000},               // GLM-5.4（2026-11-23，Loop Transformer）
		{"gemini-4-argon", 1000000},        // Gemini 4 Argon：统一 1M 窗口
		{"gemini-4-flash-future", 1000000}, // gemini-4 前缀兜底
		{"kimi-k3.1-pro", 1000000},         // Kimi K3.1 Pro（2026-10-23）
		{"qwen3.8-flash", 1000000},         // Qwen3.8-Flash：1M 上下文
		{"doubao-seed-evolving", 1000000},  // 豆包 Seed Evolving
		{"doubao-seed-evolving-latest", 1000000},
		{"doubao-seed-2.0-mini", 256000},       // 豆包 Seed 2.0
		{"doubao-seed-2.0-pro-future", 256000}, // doubao-seed 前缀兜底
		{"hy4", 1000000},                       // 腾讯混元 Hy4
		{"hy4-flash", 1000000},                 // 混元 Hy4-Flash
		{"spark-x2.6", 1000000},                // 星火 X2.6：1M 上下文
		{"spark-x", 1000000},                   // 星火深度推理 X 系列
		{"spark-x2-flash", 256000},             // Spark-X2-Flash（256K，精确匹配优先于 spark-x 前缀）
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
		// ---- 2026-11 新增 ----
		{"gpt-6.2-astra", 128000}, // GPT-6.2：128K 输出
		{"gpt-6.2-sol", 128000},
		{"gpt-6.2-pro-future", 128000}, // gpt-6.2 前缀兜底
		{"glm-5.4", 128000},            // GLM-5.4：128K 输出
		{"gemini-4-argon", 128000},     // Gemini 4 Argon（保守记 128K，窗口为输入输出共享）
		{"kimi-k3.1-pro", 131072},      // 与 K3 同口径默认
		{"qwen3.8-flash", 128000},      // 官方最大 131072，按十进制记 128000
		{"doubao-seed-evolving", 256000},
		{"doubao-seed-2.0-mini", 128000},
		{"doubao-seed-2.0-pro-future", 128000}, // doubao-seed 前缀兜底
		{"hy4", 64000},                         // 混元 Hy4：64K 输出
		{"hy4-flash", 64000},
		{"spark-x2.6", 128000}, // 星火 X 系列：128K 输出
		{"spark-x", 128000},
		{"spark-x2-flash", 32000}, // 输出上限未公开，宁小勿大记 32K
	}
	for _, c := range outCases {
		if got := GetBuiltinModelMaxOutput(c.model); got != c.want {
			t.Errorf("GetBuiltinModelMaxOutput(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}
