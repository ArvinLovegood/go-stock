package agent

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
)

// TestDeepAgentsLatency 分段测量 DeepAgents 模式响应延迟，定位"等太久"的瓶颈。
//
// 阶段：
//  1. MCP 工具加载（强制冷缓存，逐服务器计时）— 生产路径 TTL 10 分钟过期后需重付
//  2. MCP 工具加载（热缓存）
//  3. getToolsByQuestion 全量工具装配（DeepAgents alwaysIncludeMCP=true）
//  4. 系统提示词组装（自进化层/项目指令/用户画像）
//  5. Agent 实例构建（模型工厂 + Agent 图构建）
//  6. 实际运行：首事件时间（TTFT）与总耗时
//
// 需要真实网络与 SQLite：go test 时须绕过沙箱运行。
func TestDeepAgentsLatency(t *testing.T) {
	db.Init("../../data/stock.db")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// ---------- 阶段 1：MCP 冷缓存（并行重建，生产路径 loadMCPToolsForServers） ----------
	servers := activeMCPServers()
	t.Logf("启用 MCP 服务器数: %d", len(servers))

	InvalidateMCPToolsCache(0) // 强制冷缓存，复现 TTL 过期后的生产路径
	t0 := time.Now()
	coldGroups := loadMCPToolsForServers(servers)
	coldTotal := time.Since(t0)
	loaded := 0
	for _, g := range coldGroups {
		loaded += len(g.tools)
	}
	t.Logf("MCP 冷加载（并行）: %v, 服务器=%d, 工具=%d", coldTotal.Round(time.Millisecond), len(coldGroups), loaded)

	// 失败退避验证：立即二次加载，失败服务器应命中退避窗口微秒级返回
	t0 = time.Now()
	_ = loadMCPToolsForServers(servers)
	t.Logf("MCP 二次加载（热缓存+失败退避）: %v", time.Since(t0).Round(time.Microsecond))

	// ---------- 阶段 2：MCP 热缓存 ----------
	t0 = time.Now()
	_ = loadMCPToolsGroupedByServer()
	t.Logf("MCP 热缓存加载: %v", time.Since(t0).Round(time.Microsecond))

	// ---------- 阶段 3：全量工具装配（DeepAgents 真实路径） ----------
	t0 = time.Now()
	allTools := getToolsByQuestion("你好，用一句话介绍你自己", mcpInjectContext{}, true)
	t.Logf("getToolsByQuestion(DeepAgents): %v, tools=%d", time.Since(t0).Round(time.Microsecond), len(allTools))

	// ---------- 阶段 4：系统提示词组装 ----------
	t0 = time.Now()
	sp := buildSelfEvolutionPrompt(deepAgentRootDir(), "你好，用一句话介绍你自己")
	t.Logf("buildSelfEvolutionPrompt: %v, len=%d chars", time.Since(t0).Round(time.Microsecond), len(sp))

	// ---------- 阶段 5：Agent 构建（含模型工厂） ----------
	aiConfigs := data.GetSettingConfig().AiConfigs
	if len(aiConfigs) == 0 {
		t.Skip("DB 中无 AI 配置，跳过模型实测（前 4 阶段结果已输出）")
	}
	aiCfg := *aiConfigs[0]
	t.Logf("使用配置: name=%q model=%q base=%q", aiCfg.Name, aiCfg.ModelName, aiCfg.BaseUrl)

	t0 = time.Now()
	inst, err := GetStockAiAgent(&ctx, aiCfg, "你好，用一句话介绍你自己", string(DeepAgents), "")
	if err != nil {
		t.Fatalf("GetStockAiAgent 失败: %v", err)
	}
	t.Logf("GetStockAiAgent(DeepAgents): %v, tools=%d", time.Since(t0).Round(time.Millisecond), len(inst.Tools))

	// ---------- 阶段 6：实际运行，测首事件时间 ----------
	messages := []*schema.Message{
		{Role: schema.System, Content: "你是一个股票分析助手。"},
		{Role: schema.User, Content: "你好，用一句话介绍你自己"},
	}

	if inst.AdkAgent == nil {
		t.Fatal("AdkAgent 为 nil")
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: inst.AdkAgent})
	iter := runner.Run(ctx, messages)

	start := time.Now()
	var firstEventAt time.Duration
	var chunks int
	var sb strings.Builder
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			t.Logf("事件错误: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			msg := event.Output.MessageOutput.Message
			if msg.Content != "" || msg.ReasoningContent != "" {
				chunks++
				if firstEventAt == 0 {
					firstEventAt = time.Since(start)
					t.Logf(">>> 首个内容事件: %v", firstEventAt.Round(time.Millisecond))
				}
				sb.WriteString(msg.Content)
			}
		}
	}
	total := time.Since(start)
	t.Logf("运行总耗时: %v, 内容事件数=%d, 首事件=%v", total.Round(time.Millisecond), chunks, firstEventAt.Round(time.Millisecond))
	t.Logf("响应前200字: %s", truncForTest(strings.TrimSpace(sb.String()), 200))

	if firstEventAt > 3*time.Second {
		t.Logf("⚠️ 首 token 延迟超过 3s，疑似前置阶段（工具/提示词/规划）拖慢")
	}
}

// TestDeepAgentsToolTokens 量化 DeepAgents 工具清单的 token 开销，列出最重的 10 个工具。
// 同时度量 splitStaticAndDynamicTools 分流后的常驻/动态分布——常驻部分才是每轮
// 固定注入模型的 schema 开销（P1 优化的直接指标）。
func TestDeepAgentsToolTokens(t *testing.T) {
	db.Init("../../data/stock.db")

	allTools := getToolsByQuestion("分析一下贵州茅台的最新行情和资金流向", mcpInjectContext{}, true)
	total := estimateToolsTokens(allTools)
	t.Logf("DeepAgents 全量工具数=%d, 工具 schema 总 token 估算=%d", len(allTools), total)

	staticTools, dynamicTools := splitStaticAndDynamicTools(allTools)
	t.Logf("分流后: 常驻=%d (%d toks), 动态(ToolSearch)=%d (%d toks)",
		len(staticTools), estimateToolsTokens(staticTools), len(dynamicTools), estimateToolsTokens(dynamicTools))

	type tc struct {
		name string
		toks int
		mcp  bool
	}
	var list []tc
	for _, tool := range allTools {
		info, err := tool.Info(nil)
		if err != nil || info == nil {
			continue
		}
		n := 0
		if data, err := sonic.Marshal(info); err == nil {
			n = estimateTokens(string(data))
		}
		list = append(list, tc{name: info.Name, toks: n, mcp: tools.IsDynamicTool(tool)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].toks > list[j].toks })
	for i := 0; i < len(list) && i < 10; i++ {
		t.Logf("  top%-2d %-40s %5d toks mcp=%v", i+1, list[i].name, list[i].toks, list[i].mcp)
	}
	var mcpToks int
	for _, x := range list {
		if x.mcp {
			mcpToks += x.toks
		}
	}
	t.Logf("其中 MCP 动态工具 token=%d（经 ToolSearch 检索，不占每轮常驻 schema）", mcpToks)
}

// TestDeepAgentsRealQuestion 用真实股票问题复现生产耗时：逐事件输出时间线。
func TestDeepAgentsRealQuestion(t *testing.T) {
	db.Init("../../data/stock.db")

	question := "分析一下贵州茅台(600519)的最新行情，包括资金流向和最近的消息面"
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	aiConfigs := data.GetSettingConfig().AiConfigs
	if len(aiConfigs) == 0 {
		t.Skip("DB 中无 AI 配置")
	}
	aiCfg := *aiConfigs[0]

	t0 := time.Now()
	inst, err := GetStockAiAgent(&ctx, aiCfg, question, string(DeepAgents), "")
	if err != nil {
		t.Fatalf("GetStockAiAgent 失败: %v", err)
	}
	setup := time.Since(t0)
	t.Logf("Agent 构建: %v, tools=%d", setup.Round(time.Millisecond), len(inst.Tools))

	messages := []*schema.Message{
		{Role: schema.System, Content: "你是一个专业的股票分析助手。"},
		{Role: schema.User, Content: question},
	}

	if inst.AdkAgent == nil {
		t.Fatal("AdkAgent 为 nil")
	}
	start := time.Now()
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: inst.AdkAgent})
	iter := runner.Run(ctx, messages)

	var sb strings.Builder
	var lastLog time.Time
	turns := 0
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			t.Logf("[%6.1fs] 事件错误: %v", time.Since(start).Seconds(), event.Err)
			continue
		}
		mv := event.Output.MessageOutput
		if mv == nil || mv.Message == nil {
			continue
		}
		msg := mv.Message
		agent := event.AgentName
		if len(msg.ToolCalls) > 0 {
			names := make([]string, 0, len(msg.ToolCalls))
			for _, c := range msg.ToolCalls {
				names = append(names, c.Function.Name)
			}
			t.Logf("[%6.1fs] %s 发起工具调用: %v", time.Since(start).Seconds(), agent, names)
			turns++
		} else if msg.Content != "" {
			if time.Since(lastLog) > 500*time.Millisecond {
				t.Logf("[%6.1fs] %s 内容增量 %d 字（累计 %d 字）", time.Since(start).Seconds(), agent, len(msg.Content), sb.Len()+len(msg.Content))
				lastLog = time.Now()
			}
			sb.WriteString(msg.Content)
		}
	}
	total := time.Since(start)
	t.Logf("=== 总耗时 %v（不含构建 %v），LLM 轮次≈%d，回复 %d 字 ===", total.Round(time.Millisecond), setup.Round(time.Millisecond), turns, sb.Len())
	t.Logf("回复结尾: %s", truncForTest(strings.TrimSpace(sb.String()), 150))
}

// runDeepAgentsQuestion 构建 DeepAgents 并跑一个问题，返回分段时间线。
// reasoning 单独计量：thinking 模型的思维链先行流式输出，不计入内容 TTFT，
// 否则会把思维链耗时误判为平台慢。
func runDeepAgentsQuestion(t *testing.T, cfg data.AIConfig, sysPrompt, question string) (build, ttft, total time.Duration, turns int, reply string, reasoningChars int) {
	// 600s：生产运行预算 MaxDuration=0 不限时（agent_run.go），HTTP 客户端超时 6000s；
	// 测试脚手架只做兜底，避免把重任务（选股多轮工具链）的正常长耗时误判为截断。
	// 曾用 180s 导致两次误报 "context deadline exceeded"（见 2026-10-14 选股复测）。
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	t0 := time.Now()
	inst, err := GetStockAiAgent(&ctx, cfg, question, string(DeepAgents), "")
	if err != nil {
		t.Fatalf("GetStockAiAgent 失败: %v", err)
	}
	build = time.Since(t0)

	messages := []*schema.Message{
		{Role: schema.System, Content: sysPrompt},
		{Role: schema.User, Content: question},
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: inst.AdkAgent})
	iter := runner.Run(ctx, messages)

	start := time.Now()
	var sb strings.Builder
	lastEventAt := start
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		now := time.Now()
		if gap := now.Sub(lastEventAt); gap > 5*time.Second {
			t.Logf("[%5.1fs] ⚠️ 此前静默 %v（工具执行或模型卡顿）", time.Since(start).Seconds(), gap.Round(time.Millisecond))
		}
		lastEventAt = now
		if event.Err != nil {
			t.Logf("[%5.1fs] 事件错误: %v", time.Since(start).Seconds(), event.Err)
			continue
		}
		mv := event.Output.MessageOutput
		if mv == nil || mv.Message == nil {
			continue
		}
		msg := mv.Message
		if len(msg.ToolCalls) > 0 {
			names := make([]string, 0, len(msg.ToolCalls))
			for _, c := range msg.ToolCalls {
				names = append(names, c.Function.Name)
			}
			t.Logf("[%5.1fs] 🔧 工具调用: %v", time.Since(start).Seconds(), names)
			turns++
		}
		if msg.ReasoningContent != "" {
			if reasoningChars == 0 {
				t.Logf("[%5.1fs] 思维链开始输出", time.Since(start).Seconds())
			}
			reasoningChars += len([]rune(msg.ReasoningContent))
		}
		if msg.Content != "" {
			if ttft == 0 {
				ttft = time.Since(start)
			}
			sb.WriteString(msg.Content)
		}
	}
	return build, ttft, time.Since(start), turns, sb.String(), reasoningChars
}

// installedDB 安装版应用的数据目录（AI 模型配置以这里为准，与仓库内测试库不同）。
const installedDB = `D:/go-stock/data/stock.db`

// TestDumpAIConfigs 临时用：列出安装版库中全部 AI 配置（掩码 ApiKey），确认平台与模型。
func TestDumpAIConfigs(t *testing.T) {
	db.Init(installedDB)
	aiConfigs := data.GetSettingConfig().AiConfigs
	for i, c := range aiConfigs {
		key := ""
		if len(c.ApiKey) > 8 {
			key = c.ApiKey[:4] + "****" + c.ApiKey[len(c.ApiKey)-4:]
		}
		t.Logf("#%d name=%q base=%q model=%q key=%s maxTokens=%d ctx=%d thinking=%v timeout=%ds",
			i, c.Name, c.BaseUrl, c.ModelName, key, c.MaxTokens, c.ContextWindow, c.Thinking, c.TimeOut)
	}
}

// TestDeepAgentsModelSwap 实测推理更快的模型（glm-5.3-flash，安装版库中配置）。
// 只跑目标平台的配置，不动硅基流动；不落库，仅在测试内使用。
// 基线参照（硅基流动 zai-org/GLM-5.2，P1-P3 修复后实测）：
// 问候语 TTFT≈1.4s / 真实问题总耗时≈28s（其中大头为 GLM-5.2 生成 3-6k 字报告）。
func TestDeepAgentsModelSwap(t *testing.T) {
	db.Init(installedDB)

	aiConfigs := data.GetSettingConfig().AiConfigs
	if len(aiConfigs) == 0 {
		t.Skip("库中无 AI 配置")
	}

	// 按配置名精确定位（用户指定 "AI-z-ai/glm-5.3-flash"，TokenPlus 平台），
	// 避免误中其他平台同名模型（如火山引擎 Ark 的 glm-5.3-flash）。
	const wantName = "AI-z-ai/glm-5.3-flash"
	var target *data.AIConfig
	for _, c := range aiConfigs {
		if strings.EqualFold(c.Name, wantName) {
			target = c
			break
		}
	}
	if target == nil {
		// 兜底：按模型 ID 匹配 z-ai/glm-5.3-flash
		for _, c := range aiConfigs {
			if strings.Contains(strings.ToLower(c.ModelName), "z-ai/glm-5.3-flash") {
				target = c
				break
			}
		}
	}
	if target == nil {
		for i, c := range aiConfigs {
			t.Logf("#%d name=%q base=%q model=%q", i, c.Name, c.BaseUrl, c.ModelName)
		}
		t.Skip("未找到 model 含 5.3+flash 的配置，可用配置已列出")
	}
	t.Logf("目标配置: name=%q base=%q model=%q", target.Name, target.BaseUrl, target.ModelName)

	questions := []struct{ label, sys, user string }{
		{"问候语", "你是一个股票分析助手。", "你好，用一句话介绍你自己"},
		{"真实问题", "你是一个专业的股票分析助手。", "分析一下贵州茅台(600519)的最新行情，包括资金流向和最近的消息面"},
	}

	// 对照：thinking 开（用户当前配置）vs 关（量化思维链耗时占比）
	for _, thinking := range []bool{target.Thinking, !target.Thinking} {
		label := "thinking开"
		if !thinking {
			label = "thinking关"
		}
		t.Logf("---- %s ----", label)
		for _, q := range questions {
			cfg := *target
			cfg.Thinking = thinking
			build, ttft, total, turns, reply, reasoningChars := runDeepAgentsQuestion(t, cfg, q.sys, q.user)
			t.Logf("【%-6s】%-22s 构建=%8v 首token(内容)=%8v 总耗时=%8v 轮次=%d 思维链=%d字 回复=%d字",
				q.label, label, build.Round(time.Millisecond), ttft.Round(time.Millisecond), total.Round(time.Millisecond), turns, reasoningChars, len([]rune(reply)))
			if len(reply) > 0 {
				t.Logf("           回复开头: %s", truncForTest(strings.TrimSpace(reply), 80))
				t.Logf("           回复结尾: %s", truncForTestRight(strings.TrimSpace(reply), 80))
			}
		}
	}
}

// truncForTestRight 取字符串末尾 n 个 rune。
func truncForTestRight(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "..." + string(r[len(r)-n:])
}

// TestDeepAgentsScreeningQuestion 用真实选股问题复现前端「几分钟」的耗时构成。
// 逐事件时间线：工具调用时刻、静默间隔（>5s 预警）、思维链、总耗时。
// 默认 thinking=off；SCREENING_THINKING=on 时复现前端「深度思考」开启场景。
func TestDeepAgentsScreeningQuestion(t *testing.T) {
	db.Init(installedDB)

	question := "周线多头（收 > MA5w > MA10w）+ 本期上涨 + 年内位置 < 72% + 年内涨幅 < 55% + 近5日日均成交额 ≥ 0.8 亿。推荐几只潜力股给我"
	thinking := os.Getenv("SCREENING_THINKING") == "on"

	// 前端两个候选：#16 TokenPlus glm-5.3-flash（待验证的快模型）、
	// #0 硅基流动 DeepSeek-V3.2（配置列表第一个，前端默认概率高）
	targets := []string{"AI-z-ai/glm-5.3-flash", "硅基流动-deepseek-ai/DeepSeek-V3.2"}

	for _, name := range targets {
		cfg, ok := lo.Find(data.GetSettingConfig().AiConfigs, func(c *data.AIConfig) bool {
			return strings.EqualFold(c.Name, name)
		})
		if !ok {
			t.Logf("未找到配置 %q，跳过", name)
			continue
		}
		t.Logf("==== [%s thinking=%v] ====", cfg.Name, thinking)
		cfgCopy := *cfg
		cfgCopy.Thinking = thinking
		build, ttft, total, turns, reply, reasoningChars := runDeepAgentsQuestion(t, cfgCopy, "你是一个专业的股票分析助手。", question)
		t.Logf("【选股问题】%-40s 构建=%8v 首token(内容)=%8v 总耗时=%8v 轮次=%d 思维链=%d字 回复=%d字",
			cfg.ModelName, build.Round(time.Millisecond), ttft.Round(time.Millisecond), total.Round(time.Millisecond), turns, reasoningChars, len([]rune(reply)))
		if len(reply) > 0 {
			t.Logf("  回复开头: %s", truncForTest(strings.TrimSpace(reply), 100))
			t.Logf("  回复结尾: %s", truncForTestRight(strings.TrimSpace(reply), 100))
		}
	}
}
