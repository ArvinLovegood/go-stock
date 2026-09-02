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
