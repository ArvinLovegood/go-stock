package tools

import (
	"fmt"
	"strings"

	"go-stock/backend/data"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

// MarkdownToImage AI 工具：将 markdown 文本或本地 markdown 文件渲染为 PNG 图片并保存到本地。
//
// 渲染链路提取自飞书机器人的图片回复（feishu_bot.go replyAsImage →
// data.MarkdownToImageBytes）：chromedp 无头浏览器截图精心设计的 HTML 模板，
// 自动跟随系统深色/浅色主题，支持标题、表格、代码块高亮、列表、加粗、涨跌红绿着色等排版。
// 与飞书场景的差异在于：飞书渲染后上传云端换取 image_key，本工具渲染后落盘
// 到 logs/agent_images/ 目录，把保存路径返回给模型，由模型转述给用户。
//
// 输入支持两种形式（二选一，同时提供时以 markdown 内联文本为准）：
//   - markdown：直接传入 markdown 文本
//   - filePath：本地 markdown 文件路径（相对路径优先按工作目录解析，失败后回退程序目录）
func newMarkdownToImageTool() tool.InvokableTool {
	return NewDataToolWrapper(
		"MarkdownToImage",
		"将 Markdown 文本或本地 Markdown 文件渲染为 PNG 图片并保存到本地文件，返回图片的绝对路径。"+
			"输入支持两种形式（二选一，同时提供时优先 markdown 内联文本）：markdown 直接传入文本；"+
			"filePath 传入本地文件路径（如 D:\\docs\\report.md，相对路径优先按工作目录解析、失败后回退程序所在目录）。"+
			"渲染基于无头 Chrome，自动跟随应用深色/浅色主题，支持标题、表格、代码块、列表、加粗、涨跌红绿着色等 Markdown 排版，"+
			"适合把较长的分析报告、表格密集型内容生成图片以便查看、分享或存档。"+
			"当用户要求把内容转成图片、生成图片、导出为图片、做成长图时使用。"+
			"注意：渲染耗时约 1~3 秒；输入内容超过 200KB 会直接失败。",
		map[string]*schema.ParameterInfo{
			"markdown": {
				Type:     "string",
				Desc:     "要渲染的 Markdown 内联文本，支持标题、表格、代码块、列表、加粗等语法。与 filePath 二选一，同时提供时优先使用本参数。",
				Required: false,
			},
			"filePath": {
				Type:     "string",
				Desc:     "本地 Markdown 文件路径（.md/.markdown/.txt 等文本文件），如 D:\\docs\\report.md 或 report.md。与 markdown 二选一；相对路径优先按进程工作目录解析，失败后回退到程序所在目录。",
				Required: false,
			},
			"filename": {
				Type:     "string",
				Desc:     "可选的输出文件名（不含 .png 扩展名），默认取 filePath 文件基名或按时间戳自动生成；仅允许字母、数字、中文、下划线与中横线。",
				Required: false,
			},
		},
		func(args string) (string, error) {
			markdown := gjson.Get(args, "markdown").String()
			filePath := strings.TrimSpace(gjson.Get(args, "filePath").String())
			filename := strings.TrimSpace(gjson.Get(args, "filename").String())
			if strings.TrimSpace(markdown) == "" && filePath == "" {
				return "请提供 markdown 内联文本或 filePath 文件路径参数（二选一）", nil
			}

			path, err := data.SaveMarkdownImageToFile(markdown, filePath, filename)
			if err != nil {
				return "", fmt.Errorf("MarkdownToImage 工具执行失败: %w", err)
			}
			return fmt.Sprintf("图片已生成并保存到：%s", path), nil
		},
	)
}
