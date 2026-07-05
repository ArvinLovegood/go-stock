package data

import (
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("SendToFeishu", handleSendToFeishu)
}

// handleSendToFeishu 将标题和内容以 Markdown 卡片形式发送到飞书机器人
func handleSendToFeishu(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：SendToFeishu，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}
	title := gjson.Get(funcArguments, "title").String()
	message := gjson.Get(funcArguments, "message").String()
	if title == "" {
		title = "AI 助手"
	}
	cfg := GetSettingConfig()
	if cfg == nil || strings.TrimSpace(cfg.FeishuRobot) == "" {
		result := "飞书推送未配置机器人地址，请在设置中开启并填写 Webhook。"
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			result,
		)
		return nil
	}
	result := NewFeishuAPI().SendToFeishu(title, message)
	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		result,
	)
	return nil
}
