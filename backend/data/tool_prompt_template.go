package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/models"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("ListPromptTemplates", handleListPromptTemplates)
	registerToolHandler("GetPromptTemplate", handleGetPromptTemplate)
	registerToolHandler("SavePromptTemplate", handleSavePromptTemplate)
	registerToolHandler("DeletePromptTemplate", handleDeletePromptTemplate)
}

// handleListPromptTemplates 查询提示词模板列表
// 支持按 name（精确）和 type（精确）筛选，为空则返回全部
func handleListPromptTemplates(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "ListPromptTemplates", funcArguments)
	name := gjson.Get(funcArguments, "name").String()
	promptType := gjson.Get(funcArguments, "type").String()
	templates := NewPromptTemplateApi().GetPromptTemplates(name, promptType)

	// 构建精简摘要列表（不含 content 全文，避免占用过多 token）
	type summary struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		Content   string `json:"content"`
		UpdatedAt string `json:"updatedAt"`
	}
	var list []summary
	for _, t := range *templates {
		preview := t.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		list = append(list, summary{
			ID:        t.ID,
			Name:      t.Name,
			Type:      t.Type,
			Content:   preview,
			UpdatedAt: t.UpdatedAt.Format(time.DateTime),
		})
	}
	jsonBytes, _ := json.Marshal(list)
	if len(list) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "未找到匹配的提示词模板")
		return nil
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

// handleGetPromptTemplate 按 ID 获取单个提示词模板的完整内容
func handleGetPromptTemplate(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "GetPromptTemplate", funcArguments)
	id := gjson.Get(funcArguments, "id").Int()
	if id <= 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 id 不能为空且必须为正整数")
		return nil
	}
	content := NewPromptTemplateApi().GetPromptTemplateByID(int(id))
	if content == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, fmt.Sprintf("未找到 id=%d 的提示词模板", id))
		return nil
	}
	result := map[string]any{
		"id":      id,
		"content": content,
	}
	jsonBytes, _ := json.Marshal(result)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, string(jsonBytes))
	return nil
}

// handleSavePromptTemplate 创建或更新提示词模板
// 当 id > 0 时为更新，否则为新建
func handleSavePromptTemplate(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "SavePromptTemplate", funcArguments)
	name := gjson.Get(funcArguments, "name").String()
	content := gjson.Get(funcArguments, "content").String()
	promptType := gjson.Get(funcArguments, "type").String()
	id := gjson.Get(funcArguments, "id").Int()

	if name == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 name 不能为空")
		return nil
	}
	if content == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 content 不能为空")
		return nil
	}

	template := models.PromptTemplate{
		Name:    name,
		Content: content,
		Type:    promptType,
	}
	if id > 0 {
		template.ID = int(id)
	}

	result := NewPromptTemplateApi().AddPrompt(template)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}

// handleDeletePromptTemplate 按 ID 删除提示词模板
func handleDeletePromptTemplate(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "DeletePromptTemplate", funcArguments)
	idStr := gjson.Get(funcArguments, "id").String()
	if idStr == "" {
		idStr = strconv.FormatInt(gjson.Get(funcArguments, "id").Int(), 10)
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "参数 id 不能为空且必须为正整数")
		return nil
	}
	result := NewPromptTemplateApi().DelPrompt(uint(id))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, result)
	return nil
}
