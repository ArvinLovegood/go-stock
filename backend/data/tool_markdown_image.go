package data

import (
	"strings"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("MarkdownToImage", handleMarkdownToImage)
}

// handleMarkdownToImage 处理 MarkdownToImage 工具调用（OpenAI 直连模式）。
// 复用 SaveMarkdownImageToFile（与飞书机器人图片回复同源的渲染链路），
// 输入支持 markdown 内联文本或 filePath 本地文件路径（二选一），
// 渲染成功后把图片保存路径回填到工具消息供模型转述。
func handleMarkdownToImage(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "MarkdownToImage", funcArguments)

	markdown := gjson.Get(funcArguments, "markdown").String()
	filePath := strings.TrimSpace(gjson.Get(funcArguments, "filePath").String())
	filename := strings.TrimSpace(gjson.Get(funcArguments, "filename").String())
	if strings.TrimSpace(markdown) == "" && filePath == "" {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"请提供 markdown 内联文本或 filePath 文件路径参数（二选一）",
		)
		return nil
	}

	path, err := SaveMarkdownImageToFile(markdown, filePath, filename)
	if err != nil {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"生成图片失败: "+err.Error(),
		)
		return nil
	}

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		"图片已生成并保存到："+path,
	)
	return nil
}
