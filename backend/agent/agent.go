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
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

type Mode string

const (
	React       Mode = "react"
	PlanExecute Mode = "plan_execute"
	DeepAgents  Mode = "deepagents"
)

// summaryGenerateTimeout 摘要生成调用的独立超时。
// 摘要中间件触发时会在 ChatModel 节点内发起一次嵌套模型调用，若沿用主请求
// 临近到期的 deadline 会触发 "context deadline exceeded"。此处为摘要生成
// 单独留出充足的超时预算，且不受主请求 deadline 影响。
const summaryGenerateTimeout = 5 * time.Minute

type Instance struct {
	Mode       Mode
	ReactAgent *react.Agent
	AdkAgent   adk.ResumableAgent
	// ChatModel 主对话模型，用于工具结果摘要等场景（由 GetStockAiAgent 在创建时填充）。
	ChatModel model.ToolCallingChatModel
	// Tools 该实例实际注入给模型的工具列表，用于工具 schema token 预算估算。
	// 工具 schema 会被 eino 注入到每次模型请求，若不扣除会导致上下文超限。
	Tools []tool.BaseTool
}

func classifyComplexity(question string) Mode {
	intent := tools.DetectQuestionIntent(question)
	wordCount := len([]rune(question))

	switch intent {
	case tools.IntentComprehensiveReport:
		return PlanExecute
	case tools.IntentQuoteLookup, tools.IntentCodeLookup:
		if wordCount > 80 {
			return PlanExecute
		}
		return React
	case tools.IntentScreening:
		if wordCount > 50 || containsMultiSubject(question) {
			return PlanExecute
		}
		return React
	case tools.IntentMarketOverview, tools.IntentNewsResearch, tools.IntentMoneyFlow:
		if wordCount > 80 || containsMultiSubject(question) {
			return PlanExecute
		}
		return React
	default:
		groups := tools.ClassifyQuestion(question)
		if len(groups) >= 5 {
			return PlanExecute
		}
		if wordCount > 80 {
			return PlanExecute
		}
		return React
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

func GetStockAiAgent(ctx *context.Context, aiConfig data.AIConfig, question string, agentMode string) (*Instance, error) {
	// AIConfig 包含 ApiKey，禁止直接用 %v 打印整个配置，避免凭据进入日志。
	logger.SugaredLogger.Infof("GetStockAiAgent: config_id=%d name=%q model=%q base_url=%q mode=%q",
		aiConfig.ID, aiConfig.Name, aiConfig.ModelName, aiConfig.BaseUrl, agentMode)
	toolableChatModel, err := createChatModel(*ctx, aiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建聊天模型失败: %w", err)
	}

	//allTools := getAllTools()

	var mode Mode
	switch Mode(agentMode) {
	case React:
		mode = React
	case PlanExecute:
		mode = PlanExecute
	case DeepAgents:
		mode = DeepAgents
	default:
		mode = classifyComplexity(question)
	}

	// DeepAgents 通过 ToolSearch 动态检索 MCP 工具，恒保留 MCP；React/PlanExecute
	// 按问题是否命中 MCP 服务器名决定是否注入，避免 MCP 工具 schema 每轮固定占用 token。
	allTools := getToolsByQuestion(question, mode == DeepAgents)

	logger.SugaredLogger.Infof("Agent mode selected: %s (user=%q), question=%q, tools=%d", mode, agentMode, question, len(allTools))

	switch mode {
	case PlanExecute:
		inst, err := createPlanExecuteAgent(*ctx, toolableChatModel, allTools, aiConfig)
		if err != nil {
			return nil, err
		}
		inst.ChatModel = toolableChatModel
		inst.Tools = allTools
		return inst, nil
	case DeepAgents:
		inst, err := createDeepAgent(*ctx, toolableChatModel, allTools, aiConfig)
		if err != nil {
			return nil, err
		}
		inst.ChatModel = toolableChatModel
		inst.Tools = allTools
		return inst, nil
	default:
		inst, err := createReactAgent(*ctx, toolableChatModel, allTools, aiConfig)
		if err != nil {
			return nil, err
		}
		inst.ChatModel = toolableChatModel
		inst.Tools = allTools
		return inst, nil
	}
}

func createReactAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) (*Instance, error) {
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
			// 解析上下文窗口与输出上限，扣减输出预留后计算输入预算
			cw := resolveContextWindow(aiConfig)
			out := resolveOutputMaxTokens(aiConfig, cw)
			maxTokens := getMaxInputTokens(cw, out)
			// 工具 schema 由 eino 注入到每次模型请求，需从压缩预算中扣除，
			// 避免 React 长工具链累积后上下文超限。
			if toolTokens := estimateToolsTokens(allTools); toolTokens > 0 {
				maxTokens -= toolTokens
				if maxTokens < 4000 {
					maxTokens = 4000
				}
			}
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
		return nil, fmt.Errorf("创建 React Agent 失败: %w", err)
	}

	return &Instance{
		Mode:       React,
		ReactAgent: agent,
	}, nil
}

func createPlanExecuteAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) (*Instance, error) {
	planner, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ToolCallingChatModel: chatModel,
		GenInputFn:           genPlannerInput,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Planner 失败: %w", err)
	}
	maxStep := len(allTools)*2 + 10
	if maxStep < 30 {
		maxStep = 30
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
		MaxIterations: maxStep,
		GenInputFn:    genExecutorInput,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Executor 失败: %w", err)
	}

	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  chatModel,
		GenInputFn: genReplannerInput,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Replanner 失败: %w", err)
	}

	peAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: 7,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 PlanExecute Agent 失败: %w", err)
	}

	return &Instance{
		Mode:     PlanExecute,
		AdkAgent: peAgent,
	}, nil
}

// splitStaticAndDynamicTools 将工具列表划分为：
//   - staticTools：核心常驻工具（查询/行情/记忆/知识库/画像等），始终对模型可见；
//   - dynamicTools：大型/可选工具（外部 MCP），放入 ToolSearch 动态检索，按需加载。
func splitStaticAndDynamicTools(allTools []tool.BaseTool) (staticTools, dynamicTools []tool.BaseTool) {
	for _, t := range allTools {
		if t == nil {
			staticTools = append(staticTools, t)
			continue
		}
		if tools.IsDynamicTool(t) {
			dynamicTools = append(dynamicTools, t)
		} else {
			staticTools = append(staticTools, t)
		}
	}
	return staticTools, dynamicTools
}

// createDeepAgent 创建 DeepAgents 模式 Agent。
// DeepAgents 基于 eino ADK prebuilt/deep，内置任务规划（write_todos）、子 Agent 委派（task）
// 和通用子代理能力，适合复杂多步分析任务。
//
// 设计决策：
//   - Instruction 留空使用内置默认 prompt（含 write_todos/task 使用指引），股票领域知识
//     通过 messages[0] 系统消息注入，模型同时获得工具使用指引和领域专业知识。
//   - 始终启用文件系统 Backend（ls/read_file/write_file/edit_file/glob/grep）、StreamingShell
//     （execute）与 skill 中间件（渐进式展示），保证 DeepAgents 具备完整文件/工程能力。
//   - 保留 write_todos（任务规划）和 general-purpose 子代理（上下文隔离委派）。
//   - 自定义股票工具与内置工具自动合并，无需排除内置工具名。
func createDeepAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) (*Instance, error) {
	// 文件系统沙箱根：可执行文件所在目录，桌面应用启动时即为 go-stock 根目录
	rootDir := deepAgentRootDir()
	fsBackend := tools.NewLocalFilesystemBackend(rootDir)
	streamingShell := tools.NewLocalStreamingShell(rootDir, 60*time.Second)

	logger.SugaredLogger.Infof("DeepAgents 启用文件系统与 Shell: fs_root=%s, %s",
		fsBackend.RootDir(), streamingShell.ShellInfo())

	var handlers []adk.TypedChatModelAgentMiddleware[*schema.Message]
	// 构建 skill 中间件：组合文件系统技能（SKILL.md）与数据库技能（models.Skill）
	if skillHandler := buildSkillMiddleware(ctx, fsBackend, rootDir); skillHandler != nil {
		handlers = append(handlers, skillHandler)
		logger.SugaredLogger.Infof("DeepAgents 启用 skill 中间件（渐进式展示）")
	}

	// 把"大型/可选"工具（外部 MCP）划分到 ToolSearch 动态检索，核心数据工具保持常驻可见，
	// 避免所有工具 schema 每轮固定注入撑爆上下文。
	staticTools, dynamicTools := splitStaticAndDynamicTools(allTools)
	if len(dynamicTools) > 0 {
		tsHandler, tsErr := toolsearch.New(ctx, &toolsearch.Config{
			DynamicTools: dynamicTools,
		})
		if tsErr != nil {
			logger.SugaredLogger.Warnf("创建 ToolSearch 中间件失败: %v", tsErr)
		} else {
			handlers = append(handlers, tsHandler)
			logger.SugaredLogger.Infof("DeepAgents 启用 ToolSearch 动态工具检索: 常驻=%d, 动态(MCP)=%d",
				len(staticTools), len(dynamicTools))
		}
	} else {
		staticTools = allTools
	}

	// summarization 中间件：对话 token 超阈值时自动摘要历史，避免上下文溢出
	if summarizationHandler := buildSummarizationMiddleware(ctx, chatModel, rootDir, aiConfig); summarizationHandler != nil {
		handlers = append(handlers, summarizationHandler)
		logger.SugaredLogger.Infof("DeepAgents 启用 summarization 中间件（自动摘要）")
	}
	maxStep := len(allTools)*2 + 10
	if maxStep < 30 {
		maxStep = 30
	}
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
				// 仅静态核心工具常驻；外部 MCP 工具由 ToolSearch 中间件动态注入
				Tools:               staticTools,
				ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
				UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
					return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
				},
			},
		},
		MaxIteration: maxStep,
		// 始终挂载文件系统与 Shell，DeepAgents 具备完整文件/工程能力。
		Backend:        fsBackend,
		StreamingShell: streamingShell,
		Handlers:       handlers,
		// 保留 write_todos（任务规划）和 general-purpose 子代理（上下文隔离委派）。
	})
	if err != nil {
		return nil, fmt.Errorf("创建 DeepAgents Agent 失败: %w", err)
	}

	return &Instance{
		Mode:     DeepAgents,
		AdkAgent: deepAgent,
	}, nil
}

// buildSkillMiddleware 构建 skill 中间件，基于文件系统技能。
//
// 文件系统技能：扫描 rootDir/skills/ 下的一级子目录中的 SKILL.md 文件，
// 支持脚本（scripts/）、参考文档（references/）等资源，适合复杂多步技能。
//
// 若 skills 目录不存在或无技能，返回 nil（不注册中间件）。
func buildSkillMiddleware(ctx context.Context, fsBackend *tools.LocalFilesystemBackend, rootDir string) adk.ChatModelAgentMiddleware {
	skillsDir := filepath.Join(rootDir, "skills")

	// 使用容错后端：单个 SKILL.md 的 frontmatter 解析失败时仅跳过该技能并记日志，
	// 不会让整个 skill 列表加载失败、进而崩溃 DeepAgents Agent。
	// （eino 内置的 filesystemBackend 遇到任意一个坏技能会让 List/Get 返回错误，
	//  经 Info() 传播到 NewToolNode 导致整个 Agent 工具列表构建失败。）
	fileBackend := newTolerantSkillBackend(fsBackend, skillsDir)

	// 检查是否有可用技能（格式错误的技能已被跳过，详见 tolerantSkillBackend.List）
	matters, err := fileBackend.List(ctx)
	if err != nil {
		logger.SugaredLogger.Warnf("列出技能失败: %v", err)
		return nil
	}
	if len(matters) == 0 {
		// skills 目录不存在或无可用技能，不注册中间件
		return nil
	}
	logger.SugaredLogger.Infof("skill 中间件: 发现 %d 个文件系统技能", len(matters))

	handler, err := skill.NewMiddleware(ctx, &skill.Config{
		Backend: fileBackend,
	})
	if err != nil {
		logger.SugaredLogger.Warnf("创建 skill 中间件失败: %v", err)
		return nil
	}
	return handler
}

// summaryContentEnsurer 包装 chatModel，确保 Generate 返回的 assistant 消息
// 始终带有非空 Content。
//
// 背景：eino summarization 中间件通过 getAssistantTextContent 读取响应的
// Content 字段作为摘要文本，不读取 ReasoningContent。当主 chatModel 是
// 思考/推理模型（DeepSeek-R1、Qwen3-thinking、Claude extended thinking、
// Ark Thinking、Ollama thinking 等）时，模型可能将整个摘要输出放入
// ReasoningContent 而留空 Content，导致中间件抛出
// "summary content is empty" 错误并中断 Agent。
//
// 本包装器在 Generate 返回时检测 Content 为空但 ReasoningContent 非空的
// assistant 消息，将 ReasoningContent 复制到 Content，使下游能正常提取摘要。
// Stream 透传不动，因 summarization 中间件仅调用 Generate。
//
// 仅用于 summarization 中间件，不影响主 Agent 行为。
type summaryContentEnsurer struct {
	model.BaseModel[*schema.Message]
}

func (w *summaryContentEnsurer) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 摘要生成使用独立的宽松超时：通过 context.WithoutCancel 断开与主请求
	// 接近到期的 deadline 的耦合（同时保留 trace 等上下文值），再叠加独立超时。
	// 否则嵌套的摘要模型调用会继承主请求已临近到期的 deadline，
	// 触发 "context deadline exceeded" 并中断整个 Agent 运行。
	summaryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), summaryGenerateTimeout)
	defer cancel()

	resp, err := w.BaseModel.Generate(summaryCtx, input, opts...)
	if err != nil {
		return resp, err
	}
	if resp != nil && resp.Role == schema.Assistant && resp.Content == "" && resp.ReasoningContent != "" {
		logger.SugaredLogger.Infof("summarization: 模型返回空 Content，回退使用 ReasoningContent (%d 字)", len([]rune(resp.ReasoningContent)))
		resp.Content = resp.ReasoningContent
	}
	return resp, nil
}

// buildSummarizationMiddleware 构建摘要中间件。
//
// 当对话历史 token 数超过阈值时，自动调用模型生成摘要，用摘要替换旧消息，
// 避免 DeepAgent 多轮迭代后上下文窗口溢出。实现对齐 eino 官方文档：
// https://www.cloudwego.io/zh/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_summarization/
//
// 触发条件（任一满足即触发）：
//   - ContextTokens > tokenThreshold：按模型上下文窗口自适应。阈值取
//     (contextWindow - outputMaxTokens) × 80%，先扣减输出预留（API 强制
//     input + max_tokens ≤ context_window），确保在请求真正超过模型上下文
//     之前就触发摘要（官方文档建议 80-90%）。上下文窗口由 resolveContextWindow
//     解析（ContextWindow 字段 > 内置表 > MaxTokens 兜底 > 默认 64000）。
//   - ContextMessages > 60：消息数兜底，防超长工具调用未达 token 阈值但消息过多。
//
// TokenCounter 使用本项目自带的 estimateTokens（区分中英文字符系数），比 eino 默认
// 估算（增量消息按 ~4 字符/token）对中文更准确，避免低估 token 数导致触发过晚。
// 摘要生成复用主 chatModel 并配置重试；原始对话转存到 logs/agent_transcript.md 供回溯。
// 模型经 summaryContentEnsurer 包装，兼容思考模型将输出写入 ReasoningContent 的情况。
func buildSummarizationMiddleware(ctx context.Context, chatModel model.BaseModel[*schema.Message], rootDir string, aiConfig data.AIConfig) adk.ChatModelAgentMiddleware {
	// 确保日志目录存在，供 TranscriptFilePath 使用
	logDir := filepath.Join(rootDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		logger.SugaredLogger.Warnf("summarization: 创建日志目录失败: %v", err)
	}

	// 自适应摘要阈值：先解析上下文窗口与输出上限，再扣减输出预留。
	// API 强制约束 input + max_tokens ≤ context_window，故可用输入 =
	// contextWindow - outputMaxTokens，阈值取其 80% 在溢出前触发并留足余量。
	contextWindow := resolveContextWindow(aiConfig)
	outputMaxTokens := resolveOutputMaxTokens(aiConfig, contextWindow)
	availableInput := contextWindow - outputMaxTokens
	if availableInput < 4000 {
		availableInput = 4000
	}
	tokenThreshold := int(float64(availableInput) * 0.8)
	if tokenThreshold < 8000 {
		tokenThreshold = 8000
	}
	logger.SugaredLogger.Infof("summarization 中间件: 上下文窗口=%d, 输出上限=%d, 可用输入=%d, token阈值=%d",
		contextWindow, outputMaxTokens, availableInput, tokenThreshold)

	transcriptPath := filepath.Join(logDir, "agent_transcript.md")

	retryMax := 2
	handler, err := summarization.New(ctx, &summarization.Config{
		Model: &summaryContentEnsurer{BaseModel: chatModel},
		// 自定义 token 计数：用项目自带估算（中文准确），覆盖 eino 默认的 ~4 字符/token。
		TokenCounter: func(ctx context.Context, input *summarization.TokenCounterInput) (int, error) {
			return estimateMessagesTokens(input.Messages) + estimateToolInfosTokens(input.Tools), nil
		},
		Trigger: &summarization.TriggerCondition{
			ContextTokens:   tokenThreshold,
			ContextMessages: 60,
		},
		TranscriptFilePath: transcriptPath,
		// 摘要生成重试：失败时重试，提高长对话压缩的可靠性
		Retry: &summarization.RetryConfig{
			MaxRetries: &retryMax,
		},
	})
	if err != nil {
		logger.SugaredLogger.Warnf("创建 summarization 中间件失败: %v", err)
		return nil
	}
	return &nonFatalSummaryMiddleware{inner: handler}
}

// nonFatalSummaryMiddleware 包装 summarization 中间件，使摘要生成失败时不中断
// 整个 Agent 运行。摘要属于旁路优化（压缩历史避免上下文溢出），失败时应降级
// 跳过（保留原始未压缩消息），而非杀死 Agent。
//
// 常见失败场景：本地 Ollama 模型 BaseURL 配置错误返回 404、模型超时、模型服务
// 暂时不可用等。这些错误不应导致用户无法使用 Agent。
type nonFatalSummaryMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	inner adk.ChatModelAgentMiddleware
}

func (w *nonFatalSummaryMiddleware) BeforeModelRewriteState(
	ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	newCtx, newState, err := w.inner.BeforeModelRewriteState(ctx, state, mc)
	if err != nil {
		logger.SugaredLogger.Warnf("summarization 中间件失败，跳过摘要压缩（Agent 继续运行）: %v", err)
		return ctx, state, nil
	}
	return newCtx, newState, nil
}

// deepAgentRootDir 返回 DeepAgents 文件系统沙箱的根目录。
//
// 默认使用可执行文件所在目录（os.Executable），保证 Agent 运行所产生的
// 临时文件（如 logs/agent_transcript.md）与 skills 目录都落在程序所在目录，
// 不受进程启动时工作目录（os.Getwd）影响——用户从任意目录启动 go-stock
// 都会得到一致的沙箱根。若获取可执行文件路径失败，降级到当前工作目录。
// 可通过环境变量 GO_STOCK_ROOT_DIR 覆盖（用于测试或指定部署目录）。
func deepAgentRootDir() string {
	if env := strings.TrimSpace(os.Getenv("GO_STOCK_ROOT_DIR")); env != "" {
		return env
	}
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		return filepath.Dir(exePath)
	}
	// 降级：可执行文件路径不可用时回退到当前工作目录
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return wd
	}
	return "."
}

func errorRecoveryMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.ToolOutput, err error) {
				if run := AgentRunFromContext(ctx); run != nil {
					if budgetErr := run.ReserveTool(); budgetErr != nil {
						message := fmt.Sprintf("工具调用已被运行预算拦截: %v。请基于已有数据回答，或明确告知用户任务未完成。", budgetErr)
						RecordAgentRunTool(ctx, input.Name, "budget_exceeded", input.Arguments, message)
						if trace := AgentTurnTraceFromContext(ctx); trace != nil {
							trace.RecordToolCall(input.Name, "budget_exceeded", input.Arguments)
						}
						return &compose.ToolOutput{Result: wrapToolResultMetadata(input.Name, message, "budget_exceeded")}, nil
					}
					SetAgentRunPhase(ctx, "tool_calling")
				}
				RecordAgentRunTool(ctx, input.Name, "started", input.Arguments, "")
				defer func() {
					status := "error"
					result := ""
					if output != nil {
						result = output.Result
						status = detectToolResultStatus(result)
					}
					RecordAgentRunTool(ctx, input.Name, status, input.Arguments, result)
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用 panic: %v\n%s", r, debug.Stack())
						errMsg := fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r)
						if trace := AgentTurnTraceFromContext(ctx); trace != nil {
							trace.RecordToolCall(input.Name, "error", input.Arguments)
						}
						output = &compose.ToolOutput{
							Result: wrapToolResultMetadata(input.Name, errMsg, "error"),
						}
						err = nil
					}
				}()
				// 工具调用前预告：通过 channel 向前端发送 ReasoningContent 进度消息
				sendToolProgress(ctx, buildToolPreflightMsg(input.Name, input.Arguments))
				start := time.Now()
				output, err = next(ctx, input)
				elapsed := time.Since(start)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错: %v", err)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCallWithElapsed(input.Name, "error", input.Arguments, elapsed)
					}
					errMsg := fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err)
					return &compose.ToolOutput{
						Result: wrapToolResultMetadata(input.Name, errMsg, "error"),
					}, nil
				}
				if output != nil {
					status := detectToolResultStatus(output.Result)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCallWithElapsed(input.Name, status, input.Arguments, elapsed)
					}
					output.Result = wrapToolResultMetadata(input.Name, output.Result, status)
					if len(output.Result) > 8000 {
						output.Result = trimToolResult(ctx, output.Result, 4000)
					}
					// 工具调用后摘要：发送结果摘要到前端
					sendToolProgress(ctx, buildToolResultSummaryMsg(input.Name, output.Result, elapsed))
				}
				SetAgentRunPhase(ctx, "executing")
				return output, nil
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.StreamToolOutput, err error) {
				if run := AgentRunFromContext(ctx); run != nil {
					if budgetErr := run.ReserveTool(); budgetErr != nil {
						message := fmt.Sprintf("工具调用已被运行预算拦截: %v。请基于已有数据回答，或明确告知用户任务未完成。", budgetErr)
						RecordAgentRunTool(ctx, input.Name, "budget_exceeded", input.Arguments, message)
						if trace := AgentTurnTraceFromContext(ctx); trace != nil {
							trace.RecordToolCall(input.Name, "budget_exceeded", input.Arguments)
						}
						return &compose.StreamToolOutput{
							Result: schema.StreamReaderFromArray([]string{
								wrapToolResultMetadata(input.Name, message, "budget_exceeded"),
							}),
						}, nil
					}
					SetAgentRunPhase(ctx, "tool_calling")
				}
				RecordAgentRunTool(ctx, input.Name, "started", input.Arguments, "")
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用(stream) panic: %v\n%s", r, debug.Stack())
						errMsg := fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r)
						RecordAgentRunTool(ctx, input.Name, "error", input.Arguments, errMsg)
						output = &compose.StreamToolOutput{
							Result: schema.StreamReaderFromArray([]string{
								wrapToolResultMetadata(input.Name, errMsg, "error"),
							}),
						}
						err = nil
					}
				}()
				sendToolProgress(ctx, buildToolPreflightMsg(input.Name, input.Arguments))
				start := time.Now()
				output, err = next(ctx, input)
				elapsed := time.Since(start)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错(stream): %v", err)
					if trace := AgentTurnTraceFromContext(ctx); trace != nil {
						trace.RecordToolCallWithElapsed(input.Name, "error", input.Arguments, elapsed)
					}
					errMsg := fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err)
					RecordAgentRunTool(ctx, input.Name, "error", input.Arguments, errMsg)
					return &compose.StreamToolOutput{
						Result: schema.StreamReaderFromArray([]string{
							wrapToolResultMetadata(input.Name, errMsg, "error"),
						}),
					}, nil
				}
				if trace := AgentTurnTraceFromContext(ctx); trace != nil {
					trace.RecordToolCallWithElapsed(input.Name, "ok", input.Arguments, elapsed)
				}
				RecordAgentRunTool(ctx, input.Name, "ok", input.Arguments, "stream result")
				SetAgentRunPhase(ctx, "executing")
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

	// 长期记忆检索工具：让 Agent 在回答过程中可主动召回历史问答经验
	allTools = append(allTools, NewSearchLongTermMemoryTool())

	// 自定义知识库工具：让 Agent 可检索用户上传的文档（按主题分多个 KB）
	allTools = append(allTools, NewListKnowledgeBasesTool())
	allTools = append(allTools, NewSearchKnowledgeBaseTool())
	// 跨所有 KB + 长期记忆统一检索工具：一次调用聚合所有知识源
	allTools = append(allTools, NewSearchAllKnowledgeTool())

	// 用户画像工具：让 Agent 读取/更新用户偏好画像（走安全 API 写，不依赖文件系统沙箱）
	allTools = append(allTools, NewGetUserProfileTool())
	allTools = append(allTools, NewUpdateUserProfileTool())

	allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

func getToolsByQuestion(question string, alwaysIncludeMCP bool) []tool.BaseTool {
	var allTools []tool.BaseTool

	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	//allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	// 长期记忆检索工具：让 Agent 在回答过程中可主动召回历史问答经验
	allTools = append(allTools, NewSearchLongTermMemoryTool())

	// 自定义知识库工具：让 Agent 可检索用户上传的文档（按主题分多个 KB）
	allTools = append(allTools, NewListKnowledgeBasesTool())
	allTools = append(allTools, NewSearchKnowledgeBaseTool())
	// 跨所有 KB + 长期记忆统一检索工具：一次调用聚合所有知识源
	allTools = append(allTools, NewSearchAllKnowledgeTool())

	// 用户画像工具：让 Agent 读取/更新用户偏好画像（走安全 API 写，不依赖文件系统沙箱）
	allTools = append(allTools, NewGetUserProfileTool())
	allTools = append(allTools, NewUpdateUserProfileTool())

	//allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	// 外部 MCP 工具：React/PlanExecute 按问题与 MCP 服务器名/工具名/描述匹配决定是否注入；
	// DeepAgents 恒保留，交由 ToolSearch 中间件动态检索。避免 MCP 工具 schema 每轮固定占用 token。
	mcpTools := maybeGetMCPTools(question, alwaysIncludeMCP)
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	groups := tools.ClassifyQuestion(question)
	filtered := tools.FilterToolsByGroups(allTools, groups)

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, total=%d, filtered=%d, include_mcp=%v",
		question, groupNames(groups), len(allTools), len(filtered), len(mcpTools) > 0)

	return filtered
}

// maybeGetMCPTools 决定并加载外部 MCP 工具。
//   - alwaysIncludeMCP：恒加载（DeepAgents 交由 ToolSearch 动态检索）。
//   - 否则：先做廉价预判（服务器名命中 / 问题含 MCP 相关信号），只有可能用到 MCP 时
//     才初始化 MCP 客户端；随后按「服务器名 / 工具名 / 工具描述」匹配问题决定是否注入。
func maybeGetMCPTools(question string, alwaysIncludeMCP bool) []tool.BaseTool {
	if alwaysIncludeMCP {
		return getMCPTools()
	}
	if len(enabledMCPServerNames()) == 0 {
		return nil
	}
	// 廉价预判：避免在无关问题时初始化 MCP 客户端（有成本）
	if !mcpServerNameMatch(question) && !mcpRelevantHint(question) {
		return nil
	}
	// 一次性获取，避免匹配与注入重复初始化
	mcpTools := getMCPTools()
	if mcpServerNameMatch(question) || mcpToolsMatch(question, mcpTools) {
		return mcpTools
	}
	return nil
}

// mcpServerNameMatch 判断问题是否命中启用的 MCP 服务器名（不区分大小写子串匹配）。
func mcpServerNameMatch(question string) bool {
	if strings.TrimSpace(question) == "" {
		return false
	}
	lowerQ := strings.ToLower(question)
	for _, n := range enabledMCPServerNames() {
		if n != "" && strings.Contains(lowerQ, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// mcpRelevantHint 判断问题是否含 MCP 相关信号（服务器名之外的通用提示词），
// 作为是否值得初始化 MCP 客户端做工具匹配的廉价预判。
func mcpRelevantHint(question string) bool {
	hints := []string{"mcp", "工具", "调用", "发送", "通知", "消息", "服务器", "server",
		"webhook", "api", "查询外部", "机器人", "slack", "钉钉", "飞书", "企业微信"}
	lowerQ := strings.ToLower(question)
	for _, h := range hints {
		if strings.Contains(lowerQ, strings.ToLower(h)) {
			return true
		}
	}
	return false
}

// mcpToolsMatch 判断问题是否命中任一 MCP 工具名或其描述关键词。
// 匹配方式：工具全名、按分隔符/驼峰拆分的名称片段、以及描述中与问题共享的 2~4 字窗口。
func mcpToolsMatch(question string, mcpTools []tool.BaseTool) bool {
	if strings.TrimSpace(question) == "" || len(mcpTools) == 0 {
		return false
	}
	lowerQ := strings.ToLower(question)
	for _, t := range mcpTools {
		info, err := t.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		if info.Name != "" && strings.Contains(lowerQ, strings.ToLower(info.Name)) {
			return true
		}
		for _, part := range splitNameParts(info.Name) {
			if len(part) >= 2 && strings.Contains(lowerQ, strings.ToLower(part)) {
				return true
			}
		}
		if info.Desc != "" && sharedWindowsMatch(lowerQ, info.Desc) {
			return true
		}
	}
	return false
}

// splitNameCamelRe 在「小写/数字 → 大写」的驼峰边界匹配（捕获前后两个字符），
// splitNameSepRe 匹配下划线/空白分隔符。
//
// Go regexp 使用 RE2 语法，不支持 lookahead/lookbehind（(?=...) / (?<=...)），
// 故无法用 `(?<=[a-z0-9])(?=[A-Z])` 零宽断言做驼峰分割——regexp.MustCompile 会
// 直接 panic（被 newStockAiAgent 的 recover 捕获，表现为「AI 配置不存在或无效」）。
// 改为先用捕获组在边界插入空格、再按分隔符分割的等价写法。正则提到包级避免重复编译。
var (
	splitNameCamelRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	splitNameSepRe   = regexp.MustCompile(`[_\s]+`)
)

// splitNameParts 将工具名按下划线、双下划线（MCP 服务器/工具分隔符）及驼峰边界拆分。
func splitNameParts(name string) []string {
	expanded := splitNameCamelRe.ReplaceAllString(name, "$1 $2")
	parts := splitNameSepRe.Split(expanded, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sharedWindowsMatch 检查 text 中是否出现 question 的任一 2~4 字窗口。
// 用于中英文都适用的"问题与工具描述共享关键词"匹配（中文无需分词）。
func sharedWindowsMatch(lowerQuestion, desc string) bool {
	lowerDesc := strings.ToLower(desc)
	q := []rune(lowerQuestion)
	if len(q) < 2 {
		return false
	}
	maxWin := len(q)
	if maxWin > 4 {
		maxWin = 4
	}
	for w := 2; w <= maxWin; w++ {
		for i := 0; i+w <= len(q); i++ {
			if strings.Contains(lowerDesc, string(q[i:i+w])) {
				return true
			}
		}
	}
	return false
}

// enabledMCPServerNames 返回当前启用且可用的 MCP 服务器名列表（用于问题匹配）。
func enabledMCPServerNames() []string {
	var servers []models.MCPServer
	if err := db.Dao.Where("enable = ? AND status = ?", true, "available").Find(&servers).Error; err != nil {
		return nil
	}
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		if s.Name != "" {
			names = append(names, s.Name)
		}
	}
	return names
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

	// 按服务器走全局缓存（指纹+TTL 失效），避免每个问题重连所有 MCP 服务器。
	// 命中时微秒级返回；未命中（首次/配置变更/TTL 过期）才建连拉取。
	activeIDs := make(map[uint]bool, len(servers))
	for _, server := range servers {
		if server.URL == "" {
			continue
		}
		activeIDs[server.ID] = true
		mcpTools = append(mcpTools, getMCPToolsForServer(ctx, &server)...)
	}
	sweepStaleMCPToolsCache(activeIDs)

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
	// Markdown 表格行（以 | 开头，工具返回的常见格式）
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "|") {
		return true
	}
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
