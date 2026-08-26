package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/db"
)

// TestMarkdownToImageTool_ArgValidation 工具入参校验分支（不启动 Chrome）：
// 1. markdown 与 filePath 均缺失 → 返回提示文本而非 error
// 2. filePath 指向不存在文件 → 返回带原始原因的 error
func TestMarkdownToImageTool_ArgValidation(t *testing.T) {
	tt := newMarkdownToImageTool()

	// 参数 schema 完整性：markdown / filePath / filename 三个参数均已声明
	info, err := tt.Info(nil)
	if err != nil {
		t.Fatalf("获取工具 Info 失败: %v", err)
	}
	if info.Name != "MarkdownToImage" {
		t.Fatalf("工具名不符: %s", info.Name)
	}
	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("参数 schema 转 JSON Schema 失败: %v", err)
	}
	if js.Properties == nil {
		t.Fatal("参数 schema 无 properties")
	}
	for _, name := range []string{"markdown", "filePath", "filename"} {
		if _, exists := js.Properties.Get(name); !exists {
			t.Errorf("参数 schema 缺少 %q", name)
		}
	}
	if js.Properties.Len() != 3 {
		t.Errorf("参数数量应为 3，实际: %d", js.Properties.Len())
	}

	ctx := context.Background()

	// 两参数均缺失 → 友好提示，无 error
	out, err := tt.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("参数缺失时不应返回 error: %v", err)
	}
	if !strings.Contains(out, "二选一") {
		t.Errorf("参数缺失提示不符: %q", out)
	}

	// filePath 指向不存在的文件 → 返回带原始原因的 error（不启动渲染）
	missing := filepath.Join(t.TempDir(), "no-such-file.md")
	_, err = tt.InvokableRun(ctx, `{"filePath":"`+filepath.ToSlash(missing)+`"}`)
	if err == nil {
		t.Fatal("filePath 不存在时应返回 error")
	}
	if !strings.Contains(err.Error(), "MarkdownToImage") {
		t.Errorf("错误应带工具名前缀便于定位，实际: %v", err)
	}
}

// initRenderTestDB 渲染链路读主题配置需要 db.Dao。用共享内存 SQLite（不落盘，
// 沙箱环境下真实 stock.db 会因磁盘 IO 受限报错），AutoMigrate 自动建表。
func initRenderTestDB(t *testing.T) {
	t.Helper()
	db.Init("file:mdimg_render_test?mode=memory&cache=shared")
}

// renderTestMD 渲染样例：含标题/表格/涨跌数据/加粗/引用，覆盖主要排版特性。
const renderTestMD = `# 测试报告：MarkdownToImage 工具

| 指标 | 数值 | 变化 |
|------|------|------|
| 营收 | 100亿 | +10.5% |
| 净利润 | 30亿 | -2.3% |
| ROE | 15.2% | +0.8% |

- 评级: **买入**
- 目标价: ` + "`2180元`" + `

> 本文件由自动化测试生成
`

// assertValidPNGFile 校验路径存在且为有效 PNG（魔数 \x89PNG\r\n\x1a\n），尺寸合理。
func assertValidPNGFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("输出图片不存在 %q: %v", path, err)
	}
	if info.Size() < 1000 {
		t.Fatalf("图片过小(%d bytes)，疑似渲染异常", info.Size())
	}
	head := make([]byte, 8)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开输出图片失败 %q: %v", path, err)
	}
	defer f.Close()
	if _, err := f.Read(head); err != nil {
		t.Fatalf("读取输出图片失败 %q: %v", path, err)
	}
	if !bytes.Equal(head, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("文件不是有效 PNG（魔数=%x）: %s", head, path)
	}
}

// extractSavedPathFromOutput 从工具输出文本中提取"保存到："后面的图片路径。
func extractSavedPathFromOutput(out string) string {
	idx := strings.Index(out, "：")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(out[idx+len("："):])
}

// TestMarkdownToImageTool_RenderInline 端到端：markdown 内联文本渲染。
// 真实启动 chromedp（需本机 Chrome），验证输出路径、自定义 filename、PNG 有效性。
func TestMarkdownToImageTool_RenderInline(t *testing.T) {
	initRenderTestDB(t)

	out, err := newMarkdownToImageTool().InvokableRun(context.Background(),
		`{"markdown":`+jsonString(renderTestMD)+`,"filename":"inline_render_test"}`)
	if err != nil {
		t.Fatalf("内联文本渲染失败: %v", err)
	}
	path := extractSavedPathFromOutput(out)
	if path == "" {
		t.Fatalf("输出未包含保存路径: %q", out)
	}
	defer os.Remove(path)
	if filepath.Base(path) != "inline_render_test.png" {
		t.Errorf("输出文件名应为 inline_render_test.png，实际: %s", path)
	}
	assertValidPNGFile(t, path)
	t.Logf("内联文本渲染成功: %s", path)
}

// TestMarkdownToImageTool_RenderFromFile 端到端：filePath 文件渲染。
// 验证默认输出文件名取源文件基名（report.md → report.png）。
func TestMarkdownToImageTool_RenderFromFile(t *testing.T) {
	initRenderTestDB(t)

	src := filepath.Join(t.TempDir(), "tool_report.md")
	if err := os.WriteFile(src, []byte(renderTestMD), 0o644); err != nil {
		t.Fatalf("写入源 markdown 文件失败: %v", err)
	}

	out, err := newMarkdownToImageTool().InvokableRun(context.Background(),
		`{"filePath":"`+filepath.ToSlash(src)+`"}`)
	if err != nil {
		t.Fatalf("文件路径渲染失败: %v", err)
	}
	path := extractSavedPathFromOutput(out)
	if path == "" {
		t.Fatalf("输出未包含保存路径: %q", out)
	}
	defer os.Remove(path)
	if filepath.Base(path) != "tool_report.png" {
		t.Errorf("默认输出文件名应取源文件基名 tool_report.png，实际: %s", path)
	}
	assertValidPNGFile(t, path)
	t.Logf("文件路径渲染成功: %s", path)
}

// TestMarkdownToImageTool_BadInputs 输入分支（不启动渲染，在渲染前拦截）：
// 超长内容、空文件、目录、markdown 优先于 filePath。
func TestMarkdownToImageTool_BadInputs(t *testing.T) {
	ctx := context.Background()
	tt := newMarkdownToImageTool()

	// 超长内容（>200KB）→ 渲染前拒绝
	if _, err := tt.InvokableRun(ctx,
		`{"markdown":"`+strings.Repeat("a", 201*1024)+`"}`); err == nil {
		t.Error("超长内容应被拒绝")
	} else if !strings.Contains(err.Error(), "过长") {
		t.Errorf("超长错误信息应含'过长'，实际: %v", err)
	}

	dir := t.TempDir()

	// 空文件 → 拒绝
	emptyFile := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(emptyFile, []byte("  \n "), 0o644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}
	if _, err := tt.InvokableRun(ctx, `{"filePath":"`+filepath.ToSlash(emptyFile)+`"}`); err == nil {
		t.Error("空文件应被拒绝")
	}

	// 目录 → 拒绝
	if _, err := tt.InvokableRun(ctx, `{"filePath":"`+filepath.ToSlash(dir)+`"}`); err == nil {
		t.Error("目录路径应被拒绝")
	}
}

// jsonString 把任意字符串编码为 JSON 字符串字面量（用于拼装工具入参 JSON）。
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
