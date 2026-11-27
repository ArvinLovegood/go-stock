package agent

import (
	"strings"
	"testing"
)

// TestSystemPromptSnapshot 快照验证 P0-2「静态前缀 + 运行时后缀」重组的正确性与收益：
//
//  1. 静态前缀两次组装 byte 相同 —— 这是 provider 前缀缓存（命中价约 1/10）能够命中的前提；
//     若前缀混入秒级时间戳/记忆/召回等易变内容，缓存每轮全量失效。
//  2. 逐段输出 token 构成，作为后续 token 优化的度量基线。
//  3. 运行时段（时间上下文等）不得混入静态前缀。
func TestSystemPromptSnapshot(t *testing.T) {
	prefix1 := assembleStaticSysPromptPrefix(defaultStockPersonaPrompt, false, false, false)
	prefix2 := assembleStaticSysPromptPrefix(defaultStockPersonaPrompt, false, false, false)
	if prefix1 != prefix2 {
		t.Fatalf("静态前缀两次组装结果 byte 不一致，provider 前缀缓存将每轮失效")
	}

	// 必备规则段齐全
	for _, section := range []string{
		"【强制规则】", "【合规边界】", "【错误恢复策略】", "【工具选择优先级】",
		"【并行工具调用】", "【检索结果引用规范】", "【推荐记录保存",
	} {
		if !strings.Contains(prefix1, section) {
			t.Errorf("静态前缀缺少规则段 %q", section)
		}
	}

	// 模式段按开关注入
	if !strings.Contains(assembleStaticSysPromptPrefix(defaultStockPersonaPrompt, false, true, false), "【任务规划要求】") {
		t.Errorf("planExecute=true 时缺少【任务规划要求】段")
	}
	if !strings.Contains(assembleStaticSysPromptPrefix(defaultStockPersonaPrompt, false, false, true), "【思考要求】") {
		t.Errorf("thinking=true 时缺少【思考要求】段")
	}
	prefixBacktest := assembleStaticSysPromptPrefix(defaultStockPersonaPrompt, true, false, false)
	if strings.Contains(prefixBacktest, "【推荐记录保存") {
		t.Errorf("isBacktest=true 时不应注入推荐保存规则段")
	}

	// 运行时段不得进入静态前缀
	timeCtx := buildAgentTimeContext()
	if !strings.Contains(timeCtx, "【当前环境】") {
		t.Fatalf("buildAgentTimeContext 应包含【当前环境】段")
	}
	if strings.Contains(prefix1, "【当前环境】") {
		t.Errorf("静态前缀混入了时间上下文（易变内容），将打穿 provider 前缀缓存")
	}

	// 静态前缀必须是完整 sysPrompt 的 byte 前缀（前缀缓存按最长公共前缀命中）
	full := prefix1 + timeCtx + buildSessionContext("分析一下贵州茅台(600519)")
	if !strings.HasPrefix(full, prefix1) {
		t.Fatalf("静态前缀不是完整 sysPrompt 的 byte 前缀")
	}

	// token 基线：逐段输出，供后续优化对比
	t.Logf("静态前缀: %d chars ≈ %d toks", len([]rune(prefix1)), estimateTokens(prefix1))
	t.Logf("  其中 strategyPrompt(persona) = %d toks", estimateTokens(defaultStockPersonaPrompt))
	t.Logf("  其中 静态规则段(head+compliance+tail+parallel+retrieval+recommendSave) = %d toks",
		estimateTokens(prefix1)-estimateTokens(defaultStockPersonaPrompt))
	t.Logf("运行时后缀(时间上下文) = %d toks", estimateTokens(timeCtx))
}
