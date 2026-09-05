package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSeedSoulIfMissing 验证内置 SOUL.md 种子的落盘与幂等行为：
//  1. SOUL.md 不存在时落盘内置种子
//  2. 已存在（含用户自定义/清空）时绝不覆盖
func TestSeedSoulIfMissing(t *testing.T) {
	dir := t.TempDir()
	soulPath := filepath.Join(dir, soulFileName)

	// 1) 不存在 → 落盘种子
	seedSoulIfMissing(dir)
	data, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("期望 SOUL.md 已落盘: %v", err)
	}
	if string(data) != soulSeedContent {
		t.Fatalf("落盘内容应与内置种子一致（长度 %d vs %d）", len(data), len(soulSeedContent))
	}

	// 2) 用户自定义 → 不覆盖
	custom := "# 我的自定义 SOUL"
	if err := os.WriteFile(soulPath, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	seedSoulIfMissing(dir)
	data, err = os.ReadFile(soulPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Fatalf("已存在的 SOUL.md 不应被覆盖，实际: %q", string(data))
	}

	// 3) 用户清空（禁用）→ 不覆盖
	if err := os.WriteFile(soulPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	seedSoulIfMissing(dir)
	if fi, err := os.Stat(soulPath); err != nil || fi.Size() != 0 {
		t.Fatalf("清空的 SOUL.md 不应被重新写入 (err=%v size=%d)", err, fi.Size())
	}

	// 4) 空 rootDir → 安全跳过
	seedSoulIfMissing("")
	seedSoulIfMissing(".")
}

// TestBuildSelfEvolutionPromptWithSeed 验证 SOUL.md 缺失时 buildSelfEvolutionPrompt
// 自动落盘种子并注入进化规则（自进化层开箱即用，不再是休眠空串）。
func TestBuildSelfEvolutionPromptWithSeed(t *testing.T) {
	dir := t.TempDir()

	prompt := buildSelfEvolutionPrompt(dir, "")
	if prompt == "" {
		t.Fatal("SOUL.md 种子落盘后，自进化 prompt 不应为空")
	}
	for _, want := range []string{"【自我进化层】", "进化规则", "P0", "不确定 → 立即查证"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("自进化 prompt 应包含 %q，实际:\n%s", want, prompt)
		}
	}

	// SOUL.md 应已落盘
	if _, err := os.Stat(filepath.Join(dir, soulFileName)); err != nil {
		t.Fatalf("期望 SOUL.md 已自动落盘: %v", err)
	}
}
