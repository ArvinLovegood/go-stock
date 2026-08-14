package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/agent/tools"
)

// TestTolerantSkillBackendSkipsBadFrontmatter 验证容错后端在遇到 frontmatter
// 格式错误的 SKILL.md 时跳过该技能，而非让整个 List/Get 失败。
//
// 复现线上报错：name 字段被写成 YAML 序列，导致
//
//	yaml: unmarshal errors:
//	  line 2: cannot unmarshal !!seq into string
//
// 原本会让 eino filesystemBackend.List 返回错误，经 Info() 传播崩溃整个 DeepAgents。
func TestTolerantSkillBackendSkipsBadFrontmatter(t *testing.T) {
	skillsDir := t.TempDir()

	// 1) 正常技能
	goodDir := filepath.Join(skillsDir, "good-skill")
	if err := os.MkdirAll(goodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goodDir, "SKILL.md"), []byte(
		"---\nname: good-skill\ndescription: a good skill\n---\n\n# Good\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2) 坏技能：name 写成序列（复现 "line 2: cannot unmarshal !!seq into string"）
	badDir := filepath.Join(skillsDir, "bad-skill")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "SKILL.md"), []byte(
		"---\nname:\n  - bad-skill\ndescription: bad\n---\n\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3) 缺失 frontmatter 分隔符的技能
	noFmDir := filepath.Join(skillsDir, "no-fm-skill")
	if err := os.MkdirAll(noFmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noFmDir, "SKILL.md"), []byte("# No frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := newTolerantSkillBackend(tools.NewLocalFilesystemBackend(skillsDir), skillsDir)

	matters, err := backend.List(context.Background())
	if err != nil {
		t.Fatalf("List 不应因坏技能而报错: %v", err)
	}
	if len(matters) != 1 {
		t.Fatalf("期望仅 1 个可用技能（good-skill），实际 %d: %+v", len(matters), matters)
	}
	if matters[0].Name != "good-skill" {
		t.Fatalf("期望 good-skill，实际 %q", matters[0].Name)
	}

	// Get 正常技能应成功
	s, err := backend.Get(context.Background(), "good-skill")
	if err != nil {
		t.Fatalf("Get good-skill 失败: %v", err)
	}
	if s.Content == "" {
		t.Fatal("期望非空正文内容")
	}

	// Get 坏技能应返回未找到（已被跳过）
	if _, err := backend.Get(context.Background(), "bad-skill"); err == nil {
		t.Fatal("期望 Get bad-skill 返回未找到错误")
	}
}
