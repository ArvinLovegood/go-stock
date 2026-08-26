package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 说明：MarkdownToImage 工具的端到端渲染测试（chromedp 启动 Chrome）位于
// agent/tools/markdown_image_tool_test.go，本文件只覆盖 data 包私有函数的纯逻辑分支
// （不触 DB、不启动 Chrome），任何环境均可运行。
//
// 渲染样例 markdown（与 tools 包端到端测试同源）：
const saveMarkdownImageTestMD = `# 测试报告：MarkdownToImage 工具

| 指标 | 数值 | 变化 |
|------|------|------|
| 营收 | 100亿 | +10.5% |
| 净利润 | 30亿 | -2.3% |

- 评级: **买入**
`

// TestLoadMarkdownSource_Branches 来源解析分支：
// 空参数、markdown 优先、文件不存在、目录、空文件、BOM 剥离与基名提取。
func TestLoadMarkdownSource_Branches(t *testing.T) {
	// 1. markdown 与 filePath 均为空 → 报错
	if _, _, err := loadMarkdownSource("", "   "); err == nil {
		t.Error("两个参数均为空时应返回错误")
	}

	// 2. markdown 非空时优先，filePath 不存在也不影响
	content, base, err := loadMarkdownSource("inline content", "no-such-file.md")
	if err != nil {
		t.Fatalf("markdown 内联文本优先分支不应报错: %v", err)
	}
	if content != "inline content" || base != "" {
		t.Errorf("内联分支返回不符: content=%q base=%q", content, base)
	}

	dir := t.TempDir()

	// 3. 文件不存在 → 报错
	if _, _, err := loadMarkdownSource("", filepath.Join(dir, "nope.md")); err == nil {
		t.Error("文件不存在时应返回错误")
	}

	// 4. 路径是目录 → 报错
	if _, _, err := loadMarkdownSource("", dir); err == nil {
		t.Error("路径为目录时应返回错误")
	}

	// 5. 文件内容为空 → 报错
	emptyFile := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(emptyFile, []byte("   \n  "), 0o644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}
	if _, _, err := loadMarkdownSource("", emptyFile); err == nil {
		t.Error("文件内容为空时应返回错误")
	}

	// 6. UTF-8 BOM 剥离 + 基名提取
	bomFile := filepath.Join(dir, "bom_report.md")
	if err := os.WriteFile(bomFile, append([]byte("\uFEFF"), []byte("# 带BOM的标题")...), 0o644); err != nil {
		t.Fatalf("写入 BOM 文件失败: %v", err)
	}
	content, base, err = loadMarkdownSource("", bomFile)
	if err != nil {
		t.Fatalf("读取 BOM 文件不应报错: %v", err)
	}
	if strings.HasPrefix(content, "\uFEFF") {
		t.Error("UTF-8 BOM 应被剥离")
	}
	if !strings.Contains(content, "带BOM的标题") {
		t.Errorf("BOM 文件内容读取不符: %q", content)
	}
	if base != "bom_report" {
		t.Errorf("来源基名应为 bom_report，实际: %q", base)
	}
}

// TestSaveMarkdownImageToFile_Oversize 超长内容拒绝（在渲染前拦截，不启动 Chrome）。
func TestSaveMarkdownImageToFile_Oversize(t *testing.T) {
	big := strings.Repeat("a", maxMarkdownImageInputBytes+1)
	_, err := SaveMarkdownImageToFile(big, "", "")
	if err == nil {
		t.Fatal("超长内容应被拒绝")
	}
	if !strings.Contains(err.Error(), "过长") {
		t.Errorf("错误信息应包含'过长'提示，实际: %v", err)
	}

	// 文件超长同样在读取前由 stat 拦截
	dir := t.TempDir()
	bigFile := filepath.Join(dir, "big.md")
	if err := os.WriteFile(bigFile, []byte(big), 0o644); err != nil {
		t.Fatalf("写入超长文件失败: %v", err)
	}
	if _, err := SaveMarkdownImageToFile("", bigFile, ""); err == nil {
		t.Error("超长文件应被拒绝")
	}
}

// TestMarkdownImageSanitizeFilename 文件名净化：路径分隔符与 ".." 应被替换，
// 防止路径穿越；正常中文名保留。
func TestMarkdownImageSanitizeFilename(t *testing.T) {
	for _, in := range []string{"../../evil/path", `..\..\evil`, "/etc/passwd"} {
		out := markdownImageSanitizeFilename(in)
		if strings.ContainsAny(out, `/\`) {
			t.Errorf("净化后仍含路径分隔符: in=%q out=%q", in, out)
		}
		if strings.Contains(out, "..") {
			t.Errorf("净化后仍含 '..' 路径段: in=%q out=%q", in, out)
		}
	}

	// 中文与常规字符应保留
	if got := markdownImageSanitizeFilename("茅台分析_2026Q2"); got != "茅台分析_2026Q2" {
		t.Errorf("常规文件名不应被改写: %q", got)
	}
}
