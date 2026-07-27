package tools

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

func GetMCPServerTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"ListMCPServers",
		"查询MCP服务器列表。可以按名称、状态、启用状态筛选，支持分页。MCP服务器是外部工具服务，提供额外的AI工具能力。",
		map[string]*schema.ParameterInfo{
			"name": {
				Type:     "string",
				Desc:     "按名称模糊搜索（可选）",
				Required: false,
			},
			"status": {
				Type:     "string",
				Desc:     "按状态筛选：available=可用, unavailable=不可用, stopped=已停止（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "按启用状态筛选（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := gjson.Get(args, "name").String()
			status := gjson.Get(args, "status").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 10
			}

			query := &models.MCPServerQuery{
				Name:     name,
				Status:   status,
				Page:     page,
				PageSize: pageSize,
			}
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				enable := enableVal.Bool()
				query.Enable = &enable
			}

			result := data.NewMCPServerApi().List(query)
			if result == nil || len(result.Data) == 0 {
				return "未找到符合条件的MCP服务器", nil
			}

			type serverRow struct {
				ID          uint   `md:"ID"`
				Name        string `md:"名称"`
				Description string `md:"描述"`
				URL         string `md:"URL"`
				Enable      bool   `md:"启用"`
				Status      string `md:"状态"`
				TestResult  string `md:"测试结果"`
			}
			var rows []serverRow
			for _, s := range result.Data {
				rows = append(rows, serverRow{
					ID:          s.ID,
					Name:        s.Name,
					Description: s.Description,
					URL:         s.URL,
					Enable:      s.Enable,
					Status:      s.Status,
					TestResult:  s.TestResult,
				})
			}
			summary := fmt.Sprintf("共找到 %d 个MCP服务器，当前第 %d 页", result.Total, page)
			return summary + "\n\n" + markdownTableWithTitle("MCP服务器列表", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMCPServerDetail",
		"根据ID获取MCP服务器的详细信息，包括名称、描述、URL、状态等",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			server, err := data.NewMCPServerApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP服务器", id), nil
			}
			var md strings.Builder
			md.WriteString(fmt.Sprintf("### MCP服务器详情\n\n"))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| ID | %d |\n", server.ID))
			md.WriteString(fmt.Sprintf("| 名称 | %s |\n", server.Name))
			md.WriteString(fmt.Sprintf("| 描述 | %s |\n", server.Description))
			md.WriteString(fmt.Sprintf("| URL | %s |\n", server.URL))
			md.WriteString(fmt.Sprintf("| 命令 | %s |\n", server.Command))
			md.WriteString(fmt.Sprintf("| 参数 | %s |\n", server.Args))
			md.WriteString(fmt.Sprintf("| 启用 | %v |\n", server.Enable))
			md.WriteString(fmt.Sprintf("| 状态 | %s |\n", server.Status))
			md.WriteString(fmt.Sprintf("| 测试结果 | %s |\n", server.TestResult))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", server.CreatedAt.Format("2006-01-02 15:04:05")))
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"CreateMCPServer",
		"创建一个新的MCP服务器配置。MCP服务器是外部工具服务，通过URL连接提供额外的AI工具能力。",
		map[string]*schema.ParameterInfo{
			"name": {
				Type:     "string",
				Desc:     "服务器名称",
				Required: true,
			},
			"description": {
				Type:     "string",
				Desc:     "服务器描述",
				Required: false,
			},
			"url": {
				Type:     "string",
				Desc:     "服务器URL地址",
				Required: true,
			},
			"command": {
				Type:     "string",
				Desc:     "启动命令（可选，用于本地MCP服务）",
				Required: false,
			},
			"args": {
				Type:     "string",
				Desc:     "命令参数（可选）",
				Required: false,
			},
			"headers": {
				Type:     "string",
				Desc:     "HTTP请求头，JSON格式（可选），例如 {\"X-api-key\":\"sk-xxx\"}",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用，默认true",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := gjson.Get(args, "name").String()
			url := gjson.Get(args, "url").String()
			if name == "" || url == "" {
				return "名称和URL不能为空", nil
			}
			enable := true
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				enable = enableVal.Bool()
			}
			server := &models.MCPServer{
				Name:        name,
				Description: gjson.Get(args, "description").String(),
				URL:         url,
				Command:     gjson.Get(args, "command").String(),
				Args:        gjson.Get(args, "args").String(),
				Headers:     gjson.Get(args, "headers").String(),
				Enable:      enable,
			}
			if err := data.NewMCPServerApi().Create(server); err != nil {
				return "创建MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器创建成功，ID: %d", server.ID), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"UpdateMCPServer",
		"更新MCP服务器配置信息",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "服务器ID",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "服务器名称（可选）",
				Required: false,
			},
			"description": {
				Type:     "string",
				Desc:     "服务器描述（可选）",
				Required: false,
			},
			"url": {
				Type:     "string",
				Desc:     "服务器URL地址（可选）",
				Required: false,
			},
			"command": {
				Type:     "string",
				Desc:     "启动命令（可选）",
				Required: false,
			},
			"args": {
				Type:     "string",
				Desc:     "命令参数（可选）",
				Required: false,
			},
			"headers": {
				Type:     "string",
				Desc:     "HTTP请求头，JSON格式（可选），例如 {\"X-api-key\":\"sk-xxx\"}",
				Required: false,
			},
			"env": {
				Type:     "string",
				Desc:     "环境变量，JSON格式（可选，仅stdio类型子进程使用）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用（可选）",
				Required: false,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			existing, err := data.NewMCPServerApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP服务器", id), nil
			}
			if nameVal := gjson.Get(args, "name"); nameVal.Exists() {
				existing.Name = nameVal.String()
			}
			if descVal := gjson.Get(args, "description"); descVal.Exists() {
				existing.Description = descVal.String()
			}
			if urlVal := gjson.Get(args, "url"); urlVal.Exists() {
				existing.URL = urlVal.String()
			}
			if cmdVal := gjson.Get(args, "command"); cmdVal.Exists() {
				existing.Command = cmdVal.String()
			}
			if argsVal := gjson.Get(args, "args"); argsVal.Exists() {
				existing.Args = argsVal.String()
			}
			if headersVal := gjson.Get(args, "headers"); headersVal.Exists() {
				existing.Headers = headersVal.String()
			}
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				existing.Enable = enableVal.Bool()
			}
			if err := data.NewMCPServerApi().Update(existing); err != nil {
				return "更新MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器 %s 更新成功", existing.Name), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"DeleteMCPServer",
		"删除MCP服务器配置",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			if err := data.NewMCPServerApi().Delete(id); err != nil {
				return "删除MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器(ID:%d)已删除", id), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"EnableMCPServer",
		"启用或禁用MCP服务器",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "true=启用，false=禁用",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			enable := gjson.Get(args, "enable").Bool()
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			if err := data.NewMCPServerApi().EnableServer(id, enable); err != nil {
				return "操作失败: " + err.Error(), nil
			}
			status := "禁用"
			if enable {
				status = "启用"
			}
			return fmt.Sprintf("MCP服务器(ID:%d)已%s", id, status), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"TestMCPServer",
		"测试MCP服务器连接是否可用，并获取该服务器提供的工具列表",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			result, err := data.NewMCPServerApi().TestConnection(id)
			if err != nil {
				return "测试连接失败: " + err.Error(), nil
			}
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"ListMCPServerTools",
		"获取MCP服务器提供的工具列表，包括工具名称和描述。可按服务器ID筛选，不传则返回所有服务器的工具。",
		map[string]*schema.ParameterInfo{
			"serverId": {
				Type:     "integer",
				Desc:     "MCP服务器ID，不传则返回所有服务器的工具",
				Required: false,
			},
		},
		func(args string) (string, error) {
			serverID := uint(gjson.Get(args, "serverId").Int())
			var toolList []models.MCPServerTool
			if serverID > 0 {
				toolList = data.NewMCPServerApi().GetToolsByServerID(serverID)
			} else {
				toolList = data.NewMCPServerApi().GetAllTools()
			}
			if len(toolList) == 0 {
				return "未找到MCP工具", nil
			}
			type toolRow struct {
				ID          uint   `md:"ID"`
				ServerID    uint   `md:"服务器ID"`
				ToolName    string `md:"工具名称"`
				Description string `md:"描述"`
			}
			var rows []toolRow
			for _, t := range toolList {
				desc := t.Description
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				rows = append(rows, toolRow{
					ID:          t.ID,
					ServerID:    t.MCPServerID,
					ToolName:    t.ToolName,
					Description: desc,
				})
			}
			return markdownTableWithTitle("MCP工具列表", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMCPToolDetail",
		"获取MCP工具的详细信息，包括完整的描述和参数定义（JSON Schema格式）。需要提供工具ID。",
		map[string]*schema.ParameterInfo{
			"toolId": {
				Type:     "integer",
				Desc:     "MCP工具ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			toolID := uint(gjson.Get(args, "toolId").Int())
			if toolID == 0 {
				return "请提供有效的MCP工具ID", nil
			}
			allTools := data.NewMCPServerApi().GetAllTools()
			var found *models.MCPServerTool
			for i := range allTools {
				if allTools[i].ID == toolID {
					found = &allTools[i]
					break
				}
			}
			if found == nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP工具", toolID), nil
			}
			var md strings.Builder
			md.WriteString(fmt.Sprintf("### MCP工具详情：%s\n\n", found.ToolName))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| ID | %d |\n", found.ID))
			md.WriteString(fmt.Sprintf("| 服务器ID | %d |\n", found.MCPServerID))
			md.WriteString(fmt.Sprintf("| 工具名称 | %s |\n", found.ToolName))
			md.WriteString(fmt.Sprintf("| 描述 | %s |\n", found.Description))
			if found.ParamsSchema != "" {
				md.WriteString(fmt.Sprintf("\n#### 参数定义（JSON Schema）\n\n```json\n%s\n```\n", found.ParamsSchema))
			} else {
				md.WriteString("\n#### 参数定义\n\n无参数\n")
			}
			return md.String(), nil
		},
	))

	return tools
}

func GetSkillTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"ListSkills",
		"查询技能列表。可以按名称、分类、启用状态筛选，支持分页。技能是AI Agent的专业能力配置，包含系统提示词和触发关键词。",
		map[string]*schema.ParameterInfo{
			"name": {
				Type:     "string",
				Desc:     "按名称模糊搜索（可选）",
				Required: false,
			},
			"category": {
				Type:     "string",
				Desc:     "按分类筛选（可选），如：股票分析、技术分析、基本面分析、量化策略、风险管理、资讯研究、通用",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "按启用状态筛选（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := gjson.Get(args, "name").String()
			category := gjson.Get(args, "category").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 10
			}

			query := &models.SkillQuery{
				Name:     name,
				Category: category,
				Page:     page,
				PageSize: pageSize,
			}
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				enable := enableVal.Bool()
				query.Enable = &enable
			}

			result := data.NewSkillApi().List(query)
			if result == nil || len(result.Data) == 0 {
				return "未找到符合条件的技能", nil
			}

			type skillRow struct {
				ID              uint   `md:"ID"`
				Name            string `md:"名称"`
				Category        string `md:"分类"`
				Description     string `md:"描述"`
				TriggerKeywords string `md:"触发关键词"`
				Enable          bool   `md:"启用"`
				SortOrder       int    `md:"排序"`
			}
			var rows []skillRow
			for _, s := range result.Data {
				rows = append(rows, skillRow{
					ID:              s.ID,
					Name:            s.Name,
					Category:        s.Category,
					Description:     s.Description,
					TriggerKeywords: s.TriggerKeywords,
					Enable:          s.Enable,
					SortOrder:       s.SortOrder,
				})
			}
			summary := fmt.Sprintf("共找到 %d 个技能，当前第 %d 页", result.Total, page)
			return summary + "\n\n" + markdownTableWithTitle("技能列表", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetSkillDetail",
		"根据ID获取技能的详细信息，包括系统提示词、示例对话、触发关键词等完整配置",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "技能ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的技能ID", nil
			}
			skill, err := data.NewSkillApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的技能", id), nil
			}
			var md strings.Builder
			md.WriteString(fmt.Sprintf("### 技能详情：%s\n\n", skill.Name))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| ID | %d |\n", skill.ID))
			md.WriteString(fmt.Sprintf("| 名称 | %s |\n", skill.Name))
			md.WriteString(fmt.Sprintf("| 分类 | %s |\n", skill.Category))
			md.WriteString(fmt.Sprintf("| 描述 | %s |\n", skill.Description))
			md.WriteString(fmt.Sprintf("| 触发关键词 | %s |\n", skill.TriggerKeywords))
			md.WriteString(fmt.Sprintf("| 绑定MCP服务ID | %s |\n", skill.MCPServerIDs))
			md.WriteString(fmt.Sprintf("| 启用 | %v |\n", skill.Enable))
			md.WriteString(fmt.Sprintf("| 排序 | %d |\n", skill.SortOrder))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", skill.CreatedAt.Format("2006-01-02 15:04:05")))
			if skill.SystemPrompt != "" {
				md.WriteString(fmt.Sprintf("\n#### 系统提示词\n\n%s\n", skill.SystemPrompt))
			}
			if skill.Examples != "" {
				md.WriteString(fmt.Sprintf("\n#### 示例对话\n\n%s\n", skill.Examples))
			}
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"CreateSkill",
		"创建一个新的技能。技能是AI Agent的专业能力配置，包含系统提示词和触发关键词，激活后会追加到Agent的系统提示词中。",
		map[string]*schema.ParameterInfo{
			"name": {
				Type:     "string",
				Desc:     "技能名称",
				Required: true,
			},
			"description": {
				Type:     "string",
				Desc:     "技能描述",
				Required: false,
			},
			"category": {
				Type:     "string",
				Desc:     "分类，如：股票分析、技术分析、基本面分析、量化策略、风险管理、资讯研究、通用",
				Required: false,
			},
			"systemPrompt": {
				Type:     "string",
				Desc:     "系统提示词，当此技能激活时追加到Agent的系统提示词中",
				Required: false,
			},
			"examples": {
				Type:     "string",
				Desc:     "示例对话，帮助Agent理解如何使用此技能",
				Required: false,
			},
			"triggerKeywords": {
				Type:     "string",
				Desc:     "触发关键词，用逗号分隔",
				Required: false,
			},
			"mcpServerIds": {
				Type:     "string",
				Desc:     "绑定的MCP服务器ID，用逗号分隔（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用，默认true",
				Required: false,
			},
			"sortOrder": {
				Type:     "integer",
				Desc:     "排序值，越小越靠前，默认0",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := gjson.Get(args, "name").String()
			if name == "" {
				return "技能名称不能为空", nil
			}
			enable := true
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				enable = enableVal.Bool()
			}
			skill := &models.Skill{
				Name:            name,
				Description:     gjson.Get(args, "description").String(),
				Category:        gjson.Get(args, "category").String(),
				SystemPrompt:    gjson.Get(args, "systemPrompt").String(),
				Examples:        gjson.Get(args, "examples").String(),
				TriggerKeywords: gjson.Get(args, "triggerKeywords").String(),
				MCPServerIDs:    gjson.Get(args, "mcpServerIds").String(),
				Enable:          enable,
				SortOrder:       int(gjson.Get(args, "sortOrder").Int()),
			}
			if err := data.NewSkillApi().Create(skill); err != nil {
				return "创建技能失败: " + err.Error(), nil
			}
			return fmt.Sprintf("技能 %s 创建成功，ID: %d", skill.Name, skill.ID), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"UpdateSkill",
		"更新技能配置信息",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "技能ID",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "技能名称（可选）",
				Required: false,
			},
			"description": {
				Type:     "string",
				Desc:     "技能描述（可选）",
				Required: false,
			},
			"category": {
				Type:     "string",
				Desc:     "分类（可选）",
				Required: false,
			},
			"systemPrompt": {
				Type:     "string",
				Desc:     "系统提示词（可选）",
				Required: false,
			},
			"examples": {
				Type:     "string",
				Desc:     "示例对话（可选）",
				Required: false,
			},
			"triggerKeywords": {
				Type:     "string",
				Desc:     "触发关键词，用逗号分隔（可选）",
				Required: false,
			},
			"mcpServerIds": {
				Type:     "string",
				Desc:     "绑定的MCP服务器ID，用逗号分隔（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用（可选）",
				Required: false,
			},
			"sortOrder": {
				Type:     "integer",
				Desc:     "排序值（可选）",
				Required: false,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的技能ID", nil
			}
			existing, err := data.NewSkillApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的技能", id), nil
			}
			if nameVal := gjson.Get(args, "name"); nameVal.Exists() {
				existing.Name = nameVal.String()
			}
			if descVal := gjson.Get(args, "description"); descVal.Exists() {
				existing.Description = descVal.String()
			}
			if catVal := gjson.Get(args, "category"); catVal.Exists() {
				existing.Category = catVal.String()
			}
			if spVal := gjson.Get(args, "systemPrompt"); spVal.Exists() {
				existing.SystemPrompt = spVal.String()
			}
			if exVal := gjson.Get(args, "examples"); exVal.Exists() {
				existing.Examples = exVal.String()
			}
			if tkVal := gjson.Get(args, "triggerKeywords"); tkVal.Exists() {
				existing.TriggerKeywords = tkVal.String()
			}
			if msiVal := gjson.Get(args, "mcpServerIds"); msiVal.Exists() {
				existing.MCPServerIDs = msiVal.String()
			}
			if enableVal := gjson.Get(args, "enable"); enableVal.Exists() {
				existing.Enable = enableVal.Bool()
			}
			if soVal := gjson.Get(args, "sortOrder"); soVal.Exists() {
				existing.SortOrder = int(soVal.Int())
			}
			if err := data.NewSkillApi().Update(existing); err != nil {
				return "更新技能失败: " + err.Error(), nil
			}
			return fmt.Sprintf("技能 %s 更新成功", existing.Name), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"DeleteSkill",
		"删除技能",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "技能ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的技能ID", nil
			}
			if err := data.NewSkillApi().Delete(id); err != nil {
				return "删除技能失败: " + err.Error(), nil
			}
			return fmt.Sprintf("技能(ID:%d)已删除", id), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"EnableSkill",
		"启用或禁用技能",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "技能ID",
				Required: true,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "true=启用，false=禁用",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			enable := gjson.Get(args, "enable").Bool()
			if id == 0 {
				return "请提供有效的技能ID", nil
			}
			if err := data.NewSkillApi().EnableSkill(id, enable); err != nil {
				return "操作失败: " + err.Error(), nil
			}
			status := "禁用"
			if enable {
				status = "启用"
			}
			return fmt.Sprintf("技能(ID:%d)已%s", id, status), nil
		},
	))

	return tools
}

func markdownTableWithTitle(title string, data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("生成表格失败: %v", err)
	}
	table, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		return fmt.Sprintf("生成表格失败: %v", err)
	}
	return "### " + title + "\n" + table
}
