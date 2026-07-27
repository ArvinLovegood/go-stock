package agent

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"io"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

type AgentMode string

const (
	AgentModeReact       AgentMode = "react"
	AgentModePlanExecute AgentMode = "plan_execute"
	AgentModeDeepAgents  AgentMode = "deepagents"
)

type AgentInstance struct {
	Mode       AgentMode
	ReactAgent *react.Agent
	AdkAgent   adk.ResumableAgent
}

func classifyComplexity(question string) AgentMode {
	intent := tools.DetectQuestionIntent(question)
	wordCount := len([]rune(question))

	switch intent {
	case tools.IntentComprehensiveReport:
		return AgentModePlanExecute
	case tools.IntentQuoteLookup, tools.IntentCodeLookup:
		if wordCount > 80 {
			return AgentModePlanExecute
		}
		return AgentModeReact
	case tools.IntentScreening:
		if wordCount > 50 || containsMultiSubject(question) {
			return AgentModePlanExecute
		}
		return AgentModeReact
	case tools.IntentMarketOverview, tools.IntentNewsResearch, tools.IntentMoneyFlow:
		if wordCount > 80 || containsMultiSubject(question) {
			return AgentModePlanExecute
		}
		return AgentModeReact
	default:
		groups := tools.ClassifyQuestion(question)
		if len(groups) >= 5 {
			return AgentModePlanExecute
		}
		if wordCount > 80 {
			return AgentModePlanExecute
		}
		return AgentModeReact
	}
}

func containsMultiSubject(question string) bool {
	markers := []string{"以及", "并且", "同时", "分别", "对比", "比较", "多个", "哪些", "组合"}
	lowerQ := strings.ToLower(question)
	count := 0
	for _, m := range markers {
		if strings.Contains(lowerQ, m) {
			count++
		}
	}
	return count >= 1 && len([]rune(question)) > 40
}

func GetStockAiAgent(ctx *context.Context, aiConfig data.AIConfig, question string, agentMode string) *AgentInstance {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %v", aiConfig)
	toolableChatModel, err := createChatModel(*ctx, aiConfig)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	//allTools := getAllTools()
	allTools := getToolsByQuestion(question)

	var mode AgentMode
	switch AgentMode(agentMode) {
	case AgentModeReact:
		mode = AgentModeReact
	case AgentModePlanExecute:
		mode = AgentModePlanExecute
	case AgentModeDeepAgents:
		mode = AgentModeDeepAgents
	default:
		mode = classifyComplexity(question)
	}

	logger.SugaredLogger.Infof("Agent mode selected: %s (user=%q), question=%q, tools=%d", mode, agentMode, question, len(allTools))

	switch mode {
	case AgentModePlanExecute:
		return createPlanExecuteAgent(*ctx, toolableChatModel, allTools, aiConfig)
	case AgentModeDeepAgents:
		return createDeepAgent(*ctx, toolableChatModel, allTools, aiConfig)
	default:
		return createReactAgent(*ctx, toolableChatModel, allTools, aiConfig)
	}
}

func createReactAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) *AgentInstance {
	aiTools := compose.ToolsNodeConfig{
		Tools:               allTools,
		ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
		UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
			return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
		},
	}

	maxStep := len(allTools)*2 + 10
	if maxStep < 30 {
		maxStep = 30
	}
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      aiTools,
		MaxStep:          maxStep,
		MessageRewriter: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			maxTokens := getMaxInputTokens(aiConfig.MaxTokens)
			return compressMessages(input, maxTokens)
		},
		StreamToolCallChecker: func(ctx context.Context, modelOutput *schema.StreamReader[*schema.Message]) (bool, error) {
			// eino 合约：handler 返回前必须关闭 modelOutput stream（见 react.go L175 注释）。
			// 不关闭会导致 parentStreamReader 的 closedNum 永远不达阈值，底层 sr.Close() 不被触发，造成资源泄漏。
			defer modelOutput.Close()
			hasToolCall := false
			totalChunks := 0
			contentChunks := 0
			reasoningChunks := 0
			toolCallChunks := 0
			var lastFinishReason string
			for {
				msg, err := modelOutput.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					logger.SugaredLogger.Errorf("StreamToolCallChecker recv error: %v", err)
					return hasToolCall, err
				}
				if msg == nil {
					continue
				}
				totalChunks++
				if msg.Content != "" {
					contentChunks++
				}
				if msg.ReasoningContent != "" {
					reasoningChunks++
				}
				if len(msg.ToolCalls) > 0 {
					hasToolCall = true
					toolCallChunks++
				}
				if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason != "" {
					lastFinishReason = msg.ResponseMeta.FinishReason
				}
			}
			logger.SugaredLogger.Infof("StreamToolCallChecker: total=%d content=%d reasoning=%d tool_calls=%d has_tool_call=%v finish_reason=%s",
				totalChunks, contentChunks, reasoningChunks, toolCallChunks, hasToolCall, lastFinishReason)
			return hasToolCall, nil
		},
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建React Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Mode:       AgentModeReact,
		ReactAgent: agent,
	}
}

func createPlanExecuteAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) *AgentInstance {
	planner, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ToolCallingChatModel: chatModel,
		GenInputFn:           genPlannerInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Planner失败: %v", err)
		return nil
	}

	executor, err := planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               allTools,
				ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
				UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
					return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
				},
			},
		},
		MaxIterations: 40,
		GenInputFn:    genExecutorInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Executor失败: %v", err)
		return nil
	}

	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  chatModel,
		GenInputFn: genReplannerInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Replanner失败: %v", err)
		return nil
	}

	peAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: 7,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建PlanExecute Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Mode:     AgentModePlanExecute,
		AdkAgent: peAgent,
	}
}

// createDeepAgent 创建 DeepAgents 模式 Agent。
// DeepAgents 基于 eino ADK prebuilt/deep，内置任务规划（write_todos）、子 Agent 委派（task）
// 和通用子代理能力，适合复杂多步分析任务。
//
// 设计决策：
//   - Instruction 留空使用内置默认 prompt（含 write_todos/task 使用指引），股票领域知识
//     通过 messages[0] 系统消息注入，模型同时获得工具使用指引和领域专业知识。
//   - 不启用 Backend/Shell/StreamingShell（桌面应用无需文件系统/Shell，且避免安全风险）。
//   - 保留 write_todos（任务规划）和 general-purpose 子代理（上下文隔离委派）。
//   - 自定义股票工具与内置工具自动合并，无需排除内置工具名。
func createDeepAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) *AgentInstance {
	deepAgent, err := deep.New(ctx, &deep.Config{
		Name:        "StockDeepAgent",
		Description: "具备任务规划、子Agent委派能力的股票投资分析深度Agent",
		ChatModel:   chatModel,
		// Instruction 留空：使用内置默认 prompt（含 write_todos/task 使用指引）。
		// 股票领域专家提示词通过 messages[0] 系统消息注入，typedGenModelInput 会将
		// 内置 Instruction 作为第一个系统消息、传入 messages（含股票系统消息）紧随其后。
		Instruction: "",
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               allTools,
				ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
				UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
					return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
				},
			},
		},
		MaxIteration: 50,
		// 不启用文件系统/Shell：桌面股票应用无需文件系统操作，且避免安全风险。
		// 保留 write_todos（任务规划）和 general-purpose 子代理（上下文隔离委派）。
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建DeepAgents Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Mode:     AgentModeDeepAgents,
		AdkAgent: deepAgent,
	}
}

func errorRecoveryMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.ToolOutput, err error) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用 panic: %v\n%s", r, debug.Stack())
						errMsg := fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r)
						output = &compose.ToolOutput{
							Result: wrapToolResultMetadata(input.Name, errMsg, "error"),
						}
						err = nil
					}
				}()
				output, err = next(ctx, input)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错: %v", err)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCall(input.Name, "error", input.Arguments)
					}
					errMsg := fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err)
					return &compose.ToolOutput{
						Result: wrapToolResultMetadata(input.Name, errMsg, "error"),
					}, nil
				}
				if output != nil {
					status := detectToolResultStatus(output.Result)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCall(input.Name, status, input.Arguments)
					}
					output.Result = wrapToolResultMetadata(input.Name, output.Result, status)
					if len(output.Result) > 8000 {
						output.Result = trimToolResult(output.Result, 4000)
					}
				}
				return output, nil
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.StreamToolOutput, err error) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用(stream) panic: %v\n%s", r, debug.Stack())
						errMsg := fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r)
						output = &compose.StreamToolOutput{
							Result: schema.StreamReaderFromArray([]string{
								wrapToolResultMetadata(input.Name, errMsg, "error"),
							}),
						}
						err = nil
					}
				}()
				output, err = next(ctx, input)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错(stream): %v", err)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCall(input.Name, "error", input.Arguments)
					}
					errMsg := fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err)
					return &compose.StreamToolOutput{
						Result: schema.StreamReaderFromArray([]string{
							wrapToolResultMetadata(input.Name, errMsg, "error"),
						}),
					}, nil
				}
				if trace := AgentTurnTraceFromContext(ctx); trace != nil {
					trace.RecordToolCall(input.Name, "ok", input.Arguments)
				}
				return output, nil
			}
		},
	}
}

func buildSkillPrompt(question string) string {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return ""
	}

	var matched []models.Skill
	for _, skill := range skills {
		if skill.TriggerKeywords == "" {
			matched = append(matched, skill)
			continue
		}
		keywords := strings.Split(skill.TriggerKeywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(question, kw) {
				matched = append(matched, skill)
				break
			}
		}
	}

	if len(matched) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 你具备以下专业技能：\n")
	for _, skill := range matched {
		sb.WriteString(fmt.Sprintf("\n### %s\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n", skill.Description))
		}
		if skill.SystemPrompt != "" {
			sb.WriteString(fmt.Sprintf("%s\n", skill.SystemPrompt))
		}
		if skill.TriggerKeywords != "" {
			sb.WriteString(fmt.Sprintf("触发关键词：%s\n", skill.TriggerKeywords))
		}
		if skill.Examples != "" {
			sb.WriteString(fmt.Sprintf("示例对话：\n%s\n", skill.Examples))
		}
	}
	return sb.String()
}

func GetAllTools() []tool.BaseTool {
	var allTools []tool.BaseTool
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	//allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

func getToolsByQuestion(question string) []tool.BaseTool {
	var allTools []tool.BaseTool

	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	//allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	groups := tools.ClassifyQuestion(question)
	filtered := tools.FilterToolsByGroups(allTools, groups)

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, total=%d, filtered=%d",
		question, groupNames(groups), len(allTools), len(filtered))

	return filtered
}

func groupNames(groups map[tools.ToolGroup]bool) []string {
	var names []string
	for g := range groups {
		names = append(names, string(g))
	}
	return names
}

func getMCPTools() []tool.BaseTool {
	var mcpTools []tool.BaseTool

	var servers []models.MCPServer
	err := db.Dao.Where("enable = ? AND status = ?", true, "available").Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器列表失败: %v", err)
		return mcpTools
	}

	skillServerIDs := getSkillMCPServerIDs()
	for _, id := range skillServerIDs {
		found := false
		for _, s := range servers {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			var server models.MCPServer
			if err := db.Dao.Where("id = ? AND status = ?", id, "available").First(&server).Error; err == nil {
				servers = append(servers, server)
			}
		}
	}

	if len(servers) == 0 {
		return mcpTools
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, server := range servers {
		if server.URL == "" {
			continue
		}

		cli, err := data.InitMCPClient(ctx, &server)
		if err != nil {
			logger.SugaredLogger.Errorf("MCP客户端初始化失败 [%s]: %v", server.Name, err)
			continue
		}

		mcpToolList, err := einomcp.GetTools(ctx, &einomcp.Config{
			Cli:           cli,
			CustomHeaders: data.ParseHeaders(server.Headers),
		})
		if err != nil {
			logger.SugaredLogger.Errorf("获取MCP工具列表失败 [%s]: %v", server.Name, err)
			continue
		}

		if len(mcpToolList) > 0 {
			logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具", server.Name, len(mcpToolList))
			mcpTools = append(mcpTools, mcpToolList...)
		}
	}

	return mcpTools
}

func getSkillMCPServerIDs() []uint {
	skills := data.NewSkillApi().GetEnabledSkills()
	var ids []uint
	seen := make(map[uint]bool)
	for _, skill := range skills {
		if skill.MCPServerIDs == "" {
			continue
		}
		for _, id := range data.NewSkillApi().GetMCPServerIDs(&skill) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func extractUserQuestion(messages []adk.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User && messages[i].Content != "" {
			return cleanUserInput(messages[i].Content)
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Content != "" {
			return cleanUserInput(messages[i].Content)
		}
	}
	return ""
}

func genPlannerInput(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
	question := extractUserQuestion(userInput)
	if question == "" {
		return userInput, nil
	}

	systemMsg := schema.SystemMessage(`你是一个任务规划助手。请将用户的问题拆解为3-5个具体的执行步骤。
规则：
1. 每个步骤必须具体、可独立执行，且标明要调用的工具名称（格式：「调用XX工具：...」）
2. 第一步建议：调用 GetCurrentTime 确认当前时间；若涉及具体股票，调用 QueryStockCodeInfo 解析代码
3. 涉及股价、涨跌幅、财务指标等数字的步骤，必须指定 GetStockInfo / GetStockFinancialInfo 等数据工具，禁止写「分析基本面」等模糊步骤
4. 步骤之间按逻辑顺序排列
5. 最后一个步骤必须是汇总分析并给出最终答案（仅基于前面工具返回的数据）
6. 你必须通过调用 plan 工具来输出计划（参数为JSON格式，包含 steps 字段，值为字符串数组）
7. 不要仅用文字描述计划，必须实际调用 plan 工具`)

	userMsg := schema.UserMessage(question)

	return []adk.Message{systemMsg, userMsg}, nil
}

// safeTruncateString 安全地截断字符串，确保不会破坏UTF-8编码的中文字符
func safeTruncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// 找到最接近maxBytes的UTF-8字符边界
	for i := maxBytes; i > maxBytes-4 && i >= 0; i-- {
		if utf8.ValidString(s[:i]) {
			if i < len(s) {
				return s[:i] + "...(已截断)"
			}
			return s[:i]
		}
	}

	// 如果找不到合适的边界，返回安全的前缀
	return s[:maxBytes-10] + "...(已截断)"
}

// normalizeCompressLines 按行切分、去掉空行与装饰行、合并连续重复行（trim 后相同则丢弃），
// 便于后续分类并减少无意义 tokens。
func normalizeCompressLines(content string) []string {
	raw := strings.Split(content, "\n")
	out := make([]string, 0, len(raw))
	var last string
	for _, line := range raw {
		t := strings.TrimSpace(line)
		if t == "" || isSkippableNoiseLine(t) {
			continue
		}
		if t == last {
			continue
		}
		last = t
		out = append(out, t)
	}
	return collapseLongMarkdownTableRuns(out)
}

// isMarkdownTableRow 判断是否为 Markdown 管道表格行（表头或数据行）。
func isMarkdownTableRow(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	return strings.Count(t, "|") >= 2
}

// collapseLongMarkdownTableRuns 将超长连续表格行折叠为「前若干行 + 占位 + 后若干行」，显著省 tokens。
func collapseLongMarkdownTableRuns(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	const head, tail = 5, 5
	const minRun = 16
	res := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if !isMarkdownTableRow(lines[i]) {
			res = append(res, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && isMarkdownTableRow(lines[i]) {
			i++
		}
		run := lines[start:i]
		if len(run) < minRun || len(run) <= head+tail {
			res = append(res, run...)
			continue
		}
		omit := len(run) - head - tail
		res = append(res, run[:head]...)
		res = append(res, fmt.Sprintf("…（省略 %d 行 markdown 表格）…", omit))
		res = append(res, run[len(run)-tail:]...)
	}
	return res
}

// isSupplementaryFactLine 将未命中 isDataLine 但含日期/证券代码/金额 等事实的行并入数据类，便于尾部保留策略覆盖。
func isSupplementaryFactLine(s string) bool {
	if len(s) > 260 {
		return false
	}
	if containsApproxISODate(s) {
		return true
	}
	if maxConsecutiveDigitRun(s) >= 6 {
		return true
	}
	if strings.Contains(s, "元") && containsNumbers(s) {
		return true
	}
	return false
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// containsApproxISODate 检测 YYYY-MM-DD 形式日期子串。
func containsApproxISODate(s string) bool {
	for i := 0; i+10 <= len(s); i++ {
		if !isASCIIDigit(s[i]) {
			continue
		}
		if isASCIIDigit(s[i+1]) && isASCIIDigit(s[i+2]) && isASCIIDigit(s[i+3]) &&
			s[i+4] == '-' &&
			isASCIIDigit(s[i+5]) && isASCIIDigit(s[i+6]) &&
			s[i+7] == '-' &&
			isASCIIDigit(s[i+8]) && isASCIIDigit(s[i+9]) {
			return true
		}
	}
	return false
}

func maxConsecutiveDigitRun(s string) int {
	best, cur := 0, 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

// isSkippableNoiseLine 过滤对语义贡献极小的行（Markdown 装饰、分隔线等）。
func isSkippableNoiseLine(s string) bool {
	if len(s) > 120 {
		return false
	}
	switch {
	case strings.HasPrefix(s, "```"):
		return true
	case strings.HasPrefix(s, "---") && strings.Trim(s, "-") == "":
		return true
	case strings.HasPrefix(s, "***") && strings.Trim(s, "*") == "":
		return true
	case strings.HasPrefix(s, "___") && strings.Trim(s, "_") == "":
		return true
	}
	// 仅由 | - : 空格构成的 Markdown 表格分隔行
	allSep := true
	for _, r := range s {
		switch r {
		case '|', '-', ':', ' ', '\t':
		default:
			allSep = false
			break
		}
	}
	// 至少 8 字符，避免把 Markdown 列表项「- x」或短横线误判为表格分隔行
	return allSep && strings.Contains(s, "-") && len(s) >= 8
}

// smartContentCompress 按字节预算压缩文本：优先保留标题与摘要，其次数据与其它；
// 摘要/数据/其它在截断时优先保留尾部（工具输出常见「结论在后」）。
// 参数 maxBytes 为 UTF-8 字节上限（调用方如 compressExecutedStepResult 按字节给预算）。
func smartContentCompress(content string, maxBytes int) string {
	if maxBytes <= 0 {
		return content
	}

	metaLines, body := splitToolMetadataPrefix(content)
	metaBytes := 0
	if len(metaLines) > 0 {
		metaBytes = len(joinToolMetadataAndBody(metaLines, ""))
		if metaBytes >= maxBytes {
			return safeTruncateString(joinToolMetadataAndBody(metaLines, ""), maxBytes)
		}
	}
	bodyBudget := maxBytes - metaBytes
	if bodyBudget < 1 {
		bodyBudget = 1
	}

	lines := normalizeCompressLines(body)
	if len(lines) == 0 {
		return joinToolMetadataAndBody(metaLines, "")
	}

	var normJoined strings.Builder
	for i, ln := range lines {
		if i > 0 {
			normJoined.WriteByte('\n')
		}
		normJoined.WriteString(ln)
	}
	nj := normJoined.String()
	if len(nj) <= bodyBudget {
		return joinToolMetadataAndBody(metaLines, nj)
	}

	var headers, dataLines, summaryLines, otherLines []string
	for _, line := range lines {
		switch {
		case isHeaderLine(line):
			headers = append(headers, line)
		case isDataLine(line) || isSupplementaryFactLine(line):
			dataLines = append(dataLines, line)
		case isSummaryLine(line):
			summaryLines = append(summaryLines, line)
		default:
			otherLines = append(otherLines, line)
		}
	}

	result := make([]string, 0, len(lines))
	used := 0
	tryAdd := func(parts []string) bool {
		for _, p := range parts {
			need := len(p)
			if used > 0 {
				need++
			}
			if used+need > bodyBudget {
				return false
			}
			result = append(result, p)
			used += need
		}
		return true
	}

	// 1) 标题：保留前缀，且单独设上限，避免大量「#」标题占满预算
	headerCap := bodyBudget / 5
	if headerCap > 1000 {
		headerCap = 1000
	}
	if headerCap < 120 && len(headers) > 0 {
		headerCap = min(bodyBudget/3, 400)
	}
	tryAdd(smartTruncateLines(headers, headerCap))

	remaining := bodyBudget - used
	if remaining < 1 {
		return joinToolMetadataAndBody(metaLines, strings.Join(result, "\n"))
	}

	// 2) 摘要：约 35% 剩余预算，从尾部取（「综上所述」等常出现在末尾）
	summaryBudget := max(1, remaining*35/100)
	if len(summaryLines) > 0 && summaryBudget < 80 {
		summaryBudget = min(remaining, 240)
	}
	tryAdd(smartTruncateLinesFromEnd(summaryLines, summaryBudget))

	remaining = bodyBudget - used
	if remaining < 1 {
		out := strings.Join(result, "\n")
		if len(out) > bodyBudget {
			out = safeTruncateString(out, bodyBudget)
		}
		return joinToolMetadataAndBody(metaLines, out)
	}

	// 3) 数据行：约 55% 剩余预算，从尾部取（最新行情/指标常靠后）
	dataBudget := max(1, remaining*55/100)
	if len(dataLines) > 0 && dataBudget < 80 {
		dataBudget = min(remaining, 320)
	}
	tryAdd(smartTruncateLinesFromEnd(dataLines, dataBudget))

	remaining = bodyBudget - used
	if remaining > 48 {
		tryAdd(smartTruncateLinesFromEnd(otherLines, remaining))
	}

	finalContent := strings.Join(result, "\n")
	if len(finalContent) > bodyBudget {
		finalContent = safeTruncateString(finalContent, bodyBudget)
	}
	return joinToolMetadataAndBody(metaLines, finalContent)
}

// compressExecutedStepResult 将已完成步骤的结果注入 executor/replanner 提示时的分级压缩：
// 最近 2 步保留更多原文，更早步骤更强压缩，降低多轮 PlanExecute 中重复累计的 prompt tokens，
// 对当前要执行的步骤影响很小（当前步由 FirstStep() 单独标出，且上一步往往仍在「最近」窗口内）。
func compressExecutedStepResult(result string, stepIndex, totalSteps int, forReplanner bool) string {
	const (
		execRecent    = 1400
		execOlder     = 750
		replanRecent  = 800
		replanOlder   = 480
		minByteBudget = 400
		recentTail    = 2
	)
	recent := totalSteps <= recentTail || stepIndex >= totalSteps-recentTail

	var budget int
	if forReplanner {
		if recent {
			budget = replanRecent
		} else {
			budget = replanOlder
		}
	} else {
		if recent {
			budget = execRecent
		} else {
			budget = execOlder
		}
	}
	if budget < minByteBudget {
		budget = minByteBudget
	}
	return smartContentCompress(result, budget)
}

// isHeaderLine 判断是否为标题行
func isHeaderLine(line string) bool {
	// 包含常见标题关键词
	headerKeywords := []string{
		"分析", "结论", "建议", "总结", "评估", "预测", "风险", "机会",
		"价格", "涨跌", "涨幅", "成交量", "市值", "市盈率", "市净率",
		"营收", "利润", "增长率", "ROE", "ROA", "毛利率", "净利率",
	}

	for _, keyword := range headerKeywords {
		if strings.Contains(line, keyword) && len(line) < 100 {
			return true
		}
	}

	// 包含数字+单位的行（通常是关键指标）
	if containsKeyMetrics(line) {
		return true
	}

	return false
}

// isDataLine 判断是否为数据行
func isDataLine(line string) bool {
	// 包含数字和百分比的行
	return strings.Contains(line, "%") ||
		strings.Contains(line, "亿元") ||
		strings.Contains(line, "万元") ||
		(strings.Count(line, " ") > 2 && containsNumbers(line))
}

// isSummaryLine 判断是否为摘要行
func isSummaryLine(line string) bool {
	summaryKeywords := []string{
		"总体", "整体", "综合", "综上所述", "总体来看", "整体而言",
		"建议", "推荐", "关注", "警惕", "规避",
	}

	for _, keyword := range summaryKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

// containsKeyMetrics 检查是否包含关键指标
func containsKeyMetrics(line string) bool {
	metrics := []string{
		"市盈率", "市净率", "ROE", "ROA", "毛利率", "净利率",
		"营收", "利润", "增长率", "股价", "市值", "成交量",
	}

	for _, metric := range metrics {
		if strings.Contains(line, metric) {
			return true
		}
	}
	return false
}

// containsNumbers 检查是否包含数字
func containsNumbers(line string) bool {
	for _, r := range line {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// smartTruncateLines 智能截断行列表
func smartTruncateLines(lines []string, maxBytes int) []string {
	var result []string
	var currentSize int

	for _, line := range lines {
		if currentSize+len(line) > maxBytes {
			break
		}
		result = append(result, line)
		currentSize += len(line) + 1 // +1 for newline
	}

	return result
}

// smartTruncateLinesFromEnd 从列表尾部向前累加行，直到达到 maxBytes（含行间换行），
// 适合保留工具输出末尾的结论与最新数据。
func smartTruncateLinesFromEnd(lines []string, maxBytes int) []string {
	if maxBytes <= 0 || len(lines) == 0 {
		return nil
	}
	var result []string
	currentSize := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		lineLen := len(line)
		if len(result) > 0 {
			lineLen++ // newline before existing block
		}
		if currentSize+lineLen > maxBytes {
			break
		}
		result = append([]string{line}, result...)
		currentSize += lineLen
	}
	return result
}

// cleanUserInput 清理用户输入，确保UTF-8编码正确
func cleanUserInput(input string) string {
	// 1. 确保字符串是有效的UTF-8
	if !utf8.ValidString(input) {
		// 如果包含无效UTF-8字符，进行修复
		valid := make([]rune, 0, len(input))
		for i, r := range input {
			if r == utf8.RuneError {
				// 跳过无效字符，但记录位置用于调试
				logger.SugaredLogger.Warnf("发现无效UTF-8字符在位置 %d", i)
				continue
			}
			valid = append(valid, r)
		}
		input = string(valid)
	}

	// 2. 标准化空白字符
	input = strings.TrimSpace(input)

	// 3. 移除可能的控制字符（除了换行和制表符）
	var cleaned strings.Builder
	for _, r := range input {
		if r == '\n' || r == '\t' || r == '\r' {
			cleaned.WriteRune(r)
		} else if r >= 32 && r <= 126 { // ASCII可打印字符
			cleaned.WriteRune(r)
		} else if r > 126 { // 非ASCII字符（包括中文）
			cleaned.WriteRune(r)
		}
		// 跳过其他控制字符
	}

	return cleaned.String()
}

func genExecutorInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		logger.SugaredLogger.Errorf("Plan MarshalJSON error: %v", err)
		return nil, err
	}

	nSteps := len(in.ExecutedSteps)
	var stepsContent strings.Builder
	for i, s := range in.ExecutedSteps {
		result := compressExecutedStepResult(s.Result, i, nSteps, false)
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	question := extractUserQuestion(in.UserInput)

	systemMsg := schema.SystemMessage(`你是一个任务执行助手。请按照计划执行当前步骤，通过调用可用的工具获取所需数据，并给出该步骤的分析结果。
注意：
1. 只执行当前步骤，不要跳过；若步骤中指定了工具名，必须调用该工具
2. 充分利用工具获取真实数据，不要编造数据；工具返回 status=empty/error 时如实说明，不得补数
3. 回答中的具体数字必须来自本轮工具返回（含 [as_of=...] 元数据），不得引用训练记忆或历史对话
4. 给出简洁、精准的分析结果`)

	userMsg := schema.UserMessage(fmt.Sprintf("用户问题: %s\n\n当前计划:\n%s\n\n已完成的步骤及结果:\n%s\n【请执行当前步骤】: %s",
		question, string(planContent), stepsContent.String(), in.Plan.FirstStep()))

	return []adk.Message{systemMsg, userMsg}, nil
}

func genReplannerInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		logger.SugaredLogger.Errorf("Plan MarshalJSON error: %v", err)
		return nil, err
	}

	nSteps := len(in.ExecutedSteps)
	var stepsContent strings.Builder
	for i, s := range in.ExecutedSteps {
		result := compressExecutedStepResult(s.Result, i, nSteps, true)
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	var remainingStepsContent string
	var planSteps struct {
		Steps []string `json:"steps"`
	}
	if err := sonic.Unmarshal(planContent, &planSteps); err == nil {
		executedSet := make(map[string]bool, len(in.ExecutedSteps))
		for _, s := range in.ExecutedSteps {
			executedSet[s.Step] = true
		}
		var remaining []string
		for _, step := range planSteps.Steps {
			if !executedSet[step] {
				remaining = append(remaining, step)
			}
		}
		if len(remaining) > 0 {
			remainingStepsContent = fmt.Sprintf("⚠️ 还有 %d 个步骤未执行：\n", len(remaining))
			for i, step := range remaining {
				remainingStepsContent += fmt.Sprintf("  %d. %s\n", i+1, step)
			}
			remainingStepsContent += "\n【必须调用 plan 工具继续执行，不能跳过这些步骤】\n"
		} else {
			remainingStepsContent = "✅ 所有计划步骤已执行完毕。\n"
		}
	}

	question := extractUserQuestion(in.UserInput)

	systemMsg := schema.SystemMessage(`你是一个任务审核助手。请根据已完成的步骤结果，决定下一步操作。

你只能选择以下两种操作之一（必须通过工具调用function call执行，禁止仅用文字描述）：
1. 调用 respond 工具：当且仅当所有计划步骤都已执行完毕时使用，参数为 {"response": "完整的最终分析答复"}
2. 调用 plan 工具：当还有未执行的步骤时使用，参数为 {"steps": ["剩余步骤1", "剩余步骤2", ...]}，不要包含已完成的步骤

判断方法：查看下方"未执行的步骤"部分，如果有内容则调用plan继续执行；如果显示"所有步骤已完成"则调用respond给出最终答复。
禁止：仅用文字描述将要做什么而不实际调用工具。`)

	userMsg := schema.UserMessage(fmt.Sprintf("用户问题: %s\n\n原始计划:\n%s\n\n已完成的步骤及结果:\n%s\n%s",
		question, string(planContent), stepsContent.String(), remainingStepsContent))

	return []adk.Message{systemMsg, userMsg}, nil
}
