package agent

// mcp_injection_gate_test.go — MCP 注入「廉价预判」与「加载器」服务器集合一致性的守护测试。
//
// 背景：预判阶段若只看「enable=true AND status=available」，而加载器 activeMCPServers
// 还会补上 skill 显式依赖的服务器（即使该服务器 Enable=false），两处集合不一致就会出现：
// 预判把整台服务器判成「无关」→ React/PlanExecute 下它的工具凭空消失，而 DeepAgents
// （alwaysIncludeMCP=true）却仍能检索到同一台服务器的工具。此文件把该不一致变成可见失败。

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go-stock/backend/agent/tools"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/components/tool"
)

// initMCPInjectionTestDB 把 MCP 服务器 / skill 表迁移到独立的内存库并清空数据。
// DSN 必须自带 _pragma=，否则 db.sqliteDSN 会追加第二个 "?" 导致参数错乱。
func initMCPInjectionTestDB(t *testing.T) {
	t.Helper()
	db.Init("file:mcp_injection_gate_test?mode=memory&cache=shared&_pragma=busy_timeout(10000)")
	if err := db.Dao.AutoMigrate(&models.MCPServer{}, &models.MCPServerTool{}, &models.Skill{}); err != nil {
		t.Fatalf("迁移 mcp_servers/mcp_server_tools/skills 表失败: %v", err)
	}
	// 共享内存库在进程内复用，逐测试清空避免互相污染
	if err := db.Dao.Exec("DELETE FROM mcp_servers").Error; err != nil {
		t.Fatalf("清空 mcp_servers 失败: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM mcp_server_tools").Error; err != nil {
		t.Fatalf("清空 mcp_server_tools 失败: %v", err)
	}
	if err := db.Dao.Exec("DELETE FROM skills").Error; err != nil {
		t.Fatalf("清空 skills 失败: %v", err)
	}
	resetMCPToolsCacheForTest()
}

// seedMCPServer 写入一台 MCP 服务器并返回落库后的记录。
//
// 陷阱一：Enable 带 gorm default:true，Create 时空值会被默认值覆盖，且 GORM 会把
// 默认值**回写进传入的 struct**——必须在 Create 之前记下期望值，否则后续判断全错。
// 陷阱二：因此 enable=false 必须用原始 SQL 显式落库，并回读校验（生产侧
// EnableServer 也是显式 Update，故「enable=false + status=available」是可达成状态）。
func seedMCPServer(t *testing.T, srv models.MCPServer) models.MCPServer {
	t.Helper()
	wantEnable := srv.Enable
	if err := db.Dao.Create(&srv).Error; err != nil {
		t.Fatalf("写入 MCP 服务器失败: %v", err)
	}
	enable := 0
	if wantEnable {
		enable = 1
	}
	if err := db.Dao.Exec("UPDATE mcp_servers SET enable = ? WHERE id = ?", enable, srv.ID).Error; err != nil {
		t.Fatalf("显式设置 enable 失败: %v", err)
	}
	var stored models.MCPServer
	if err := db.Dao.First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("回读 MCP 服务器失败: %v", err)
	}
	if stored.Enable != wantEnable || stored.Status != srv.Status {
		t.Fatalf("服务器落库状态与预期不符: enable=%v/%v status=%q/%q",
			stored.Enable, wantEnable, stored.Status, srv.Status)
	}
	return stored
}

// withServerToolsLoader 用假加载器替换真实建连：按服务器名返回固定工具，
// 并通过 calls 记录被初始化（建连）的服务器次数。
func withServerToolsLoader(t *testing.T, byServer map[string][]tool.BaseTool, calls *int32) {
	t.Helper()
	withFakeLoader(t, func(_ context.Context, server *models.MCPServer) ([]tool.BaseTool, closer) {
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		return byServer[server.Name], nil
	})
}

func mcpGroupNames(groups []mcpServerTools) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.serverName)
	}
	return out
}

// seedSkillDependingOnServer 写入一个启用中的 skill，显式依赖指定 MCP 服务器。
func seedSkillDependingOnServer(t *testing.T, serverID uint) {
	t.Helper()
	if err := db.Dao.Create(&models.Skill{
		Name:         "同步文档技能",
		MCPServerIDs: fmt.Sprintf("%d", serverID),
		Enable:       true,
	}).Error; err != nil {
		t.Fatalf("写入 skill 失败: %v", err)
	}
	if ids := getSkillMCPServerIDs(); len(ids) != 1 || ids[0] != serverID {
		t.Fatalf("测试前提失效：skill 依赖的服务器 ID 未被识别，got %v", ids)
	}
}

// skill 显式依赖的服务器即使 Enable=false（曾测试连接成功、随后被停用），
// 也属于「会被加载」的集合——这是预判必须认它的前提。
func TestActiveMCPServersIncludesSkillDeclaredDisabledServer(t *testing.T) {
	initMCPInjectionTestDB(t)

	srv := seedMCPServer(t, models.MCPServer{
		Name:   "docs-sync-bridge",
		URL:    "http://127.0.0.1:9/mcp",
		Type:   "streamable-http",
		Enable: false, // 用户已停用
		Status: "available",
	})
	seedSkillDependingOnServer(t, srv.ID)

	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"docs-sync-bridge": {fakeMCPTool{name: "sync_docs", desc: "同步文档"}},
	}, nil)

	groups := loadMCPToolsGroupedByServer()
	if len(groups) != 1 || groups[0].serverName != "docs-sync-bridge" {
		t.Fatalf("skill 显式依赖的服务器应被加载，实际 %v", mcpGroupNames(groups))
	}
	if len(groups[0].tools) != 1 {
		t.Fatalf("应加载到 1 个工具，实际 %d", len(groups[0].tools))
	}
}

// 回归守卫：预判与加载必须用同一份服务器集合，否则 skill 显式依赖的服务器
// 会在 React/PlanExecute 下被预判阶段整体丢弃（工具凭空消失）。
func TestMaybeGetMCPToolsInjectsSkillDeclaredDisabledServer(t *testing.T) {
	initMCPInjectionTestDB(t)

	srv := seedMCPServer(t, models.MCPServer{
		Name:   "docs-sync-bridge",
		URL:    "http://127.0.0.1:9/mcp",
		Type:   "streamable-http",
		Enable: false,
		Status: "available",
	})
	seedSkillDependingOnServer(t, srv.ID)

	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"docs-sync-bridge": {fakeMCPTool{name: "sync_docs", desc: "同步文档"}},
	}, nil)

	question := "帮我用 docs-sync-bridge 同步一下文档"
	if mcpRelevantHint(question) {
		t.Fatalf("测试前提失效：问题 %q 命中 MCP 信号词，将绕过服务器名预判", question)
	}

	got := maybeGetMCPTools(question, mcpInjectContext{}, false)
	if len(got) != 1 {
		t.Fatalf("skill 显式依赖的服务器应可注入，实际注入 %v", mcpTestToolNames(got))
	}
	if names := mcpTestToolNames(got); names[0] != "sync_docs" {
		t.Fatalf("应注入 sync_docs，实际 %v", names)
	}
}

// 完全不相关的问题：预判直接拒绝，不应初始化任何 MCP 客户端（有网络成本）。
func TestMaybeGetMCPToolsUnrelatedQuestionSkipsLoading(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "feishu-hub", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})

	question := "今天大盘指数怎么样"
	if mcpRelevantHint(question) {
		t.Fatalf("测试前提失效：问题 %q 被判定含 MCP 信号词", question)
	}

	var calls int32
	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"feishu-hub": {fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"}},
	}, &calls)

	if got := maybeGetMCPTools(question, mcpInjectContext{}, false); len(got) != 0 {
		t.Fatalf("无关问题不应注入 MCP 工具，实际 %v", mcpTestToolNames(got))
	}
	if calls != 0 {
		t.Fatalf("无关问题不应初始化 MCP 客户端，实际建连 %d 次", calls)
	}
}

// 命中一台服务器名 → 只注入该服务器的全部工具，其它服务器不参与。
func TestMaybeGetMCPToolsInjectsOnlyMatchedServer(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "feishu-hub", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	seedMCPServer(t, models.MCPServer{
		Name: "jira-board", URL: "http://127.0.0.1:8/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})

	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"feishu-hub": {
			fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"},
			fakeMCPTool{name: "list_chats", desc: "列出飞书会话"},
		},
		"jira-board": {fakeMCPTool{name: "create_issue", desc: "创建JIRA工单"}},
	}, nil)

	got := mcpTestToolNames(maybeGetMCPTools("用 feishu-hub 同步一下", mcpInjectContext{}, false))
	if len(got) != 2 || !containsName(got, "send_message") || !containsName(got, "list_chats") {
		t.Fatalf("应只注入 feishu-hub 的 2 个工具，实际 %v", got)
	}
	if containsName(got, "create_issue") {
		t.Fatalf("不应注入未命中服务器的工具，实际 %v", got)
	}
}

// 命中服务器名但该服务器未暴露任何工具（连接失败/空工具列表）：返回 nil，不 panic。
func TestMaybeGetMCPToolsServerMatchedButNoTools(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "broken-mcp", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})

	withServerToolsLoader(t, map[string][]tool.BaseTool{}, nil)

	if got := maybeGetMCPTools("用 broken-mcp 发个通知", mcpInjectContext{}, false); len(got) != 0 {
		t.Fatalf("服务器无工具时不应注入任何工具，实际 %v", mcpTestToolNames(got))
	}
}

// DeepAgents（alwaysIncludeMCP=true）跳过预判，恒加载全部服务器的工具，
// 交由 ToolSearch 动态检索——与上面「无关问题跳过加载」形成对照。
func TestMaybeGetMCPToolsAlwaysIncludeLoadsAllServers(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "feishu-hub", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	seedMCPServer(t, models.MCPServer{
		Name: "jira-board", URL: "http://127.0.0.1:8/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})

	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"feishu-hub": {fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"}},
		"jira-board": {fakeMCPTool{name: "create_issue", desc: "创建JIRA工单"}},
	}, nil)

	got := mcpTestToolNames(maybeGetMCPTools("今天大盘指数怎么样", mcpInjectContext{}, true))
	if len(got) != 2 || !containsName(got, "send_message") || !containsName(got, "create_issue") {
		t.Fatalf("alwaysIncludeMCP 应加载全部服务器工具，实际 %v", got)
	}
}

// seedMCPServerTool 写入一条 mcp_server_tools 记录（模拟「测试连接」的落库结果）。
func seedMCPServerTool(t *testing.T, serverID uint, name, desc string) {
	t.Helper()
	if err := db.Dao.Create(&models.MCPServerTool{
		MCPServerID: serverID,
		ToolName:    name,
		Description: desc,
	}).Error; err != nil {
		t.Fatalf("写入 mcp_server_tools 失败: %v", err)
	}
}

// 预判扩展：问题既不含服务器名、也不含 MCP 信号词，但命中 mcp_server_tools 中已落库
// 的工具描述 → 仍应放行并注入。修复前该场景会在预判阶段被整体跳过（工具用不上）。
func TestMaybeGetMCPToolsInjectsByPersistedToolDescription(t *testing.T) {
	initMCPInjectionTestDB(t)

	srv := seedMCPServer(t, models.MCPServer{
		Name: "sky-bridge", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	seedMCPServerTool(t, srv.ID, "get_forecast", "查询未来天气")

	question := "明天北京天气怎么样"
	if mcpRelevantHint(question) {
		t.Fatalf("测试前提失效：问题 %q 命中 MCP 信号词，将绕过落库工具预判", question)
	}
	if mcpServerNamesMatchQuestion(question, []string{srv.Name}) {
		t.Fatalf("测试前提失效：问题 %q 命中服务器名", question)
	}

	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"sky-bridge": {fakeMCPTool{name: "get_forecast", desc: "查询未来天气"}},
	}, nil)

	got := mcpTestToolNames(maybeGetMCPTools(question, mcpInjectContext{}, false))
	if len(got) != 1 || got[0] != "get_forecast" {
		t.Fatalf("命中落库工具描述时应注入 get_forecast，实际 %v", got)
	}
}

// 系统提示词里点名服务器 → 即使问题本身完全无关，也应加载并整台注入该服务器。
func TestMaybeGetMCPToolsInjectsByPromptServerName(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "feishu-hub", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"feishu-hub": {
			fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"},
			fakeMCPTool{name: "list_chats", desc: "列出飞书会话"},
		},
	}, nil)

	question := "帮我写一段总结"
	if mcpRelevantHint(question) {
		t.Fatalf("测试前提失效：问题 %q 命中 MCP 信号词", question)
	}

	// 用户把服务器名写进了提示词模板，即为明确指定。
	ctx := mcpInjectContext{promptHint: "你是助理，所有通知都通过 feishu-hub 推送。"}
	got := mcpTestToolNames(maybeGetMCPTools(question, ctx, false))
	if len(got) != 2 || !containsName(got, "send_message") || !containsName(got, "list_chats") {
		t.Fatalf("提示词点名服务器应整台注入，实际 %v", got)
	}
}

// 技能声明只在触发词命中问题时才生效：未配触发词的技能不应让声明的服务器常驻注入。
func TestActiveSkillMCPServerIDsRequiresTriggerHit(t *testing.T) {
	initMCPInjectionTestDB(t)

	srv := seedMCPServer(t, models.MCPServer{
		Name: "docs-sync-bridge", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	if err := db.Dao.Create(&models.Skill{
		Name:            "文档同步技能",
		TriggerKeywords: "同步文档, 文档同步",
		MCPServerIDs:    fmt.Sprintf("%d", srv.ID),
		Enable:          true,
	}).Error; err != nil {
		t.Fatalf("写入 skill 失败: %v", err)
	}

	if ids := activeSkillMCPServerIDs("帮我同步文档一下"); len(ids) != 1 || ids[0] != srv.ID {
		t.Fatalf("触发词命中时应返回技能声明的服务器，实际 %v", ids)
	}
	if ids := activeSkillMCPServerIDs("今天大盘指数怎么样"); len(ids) != 0 {
		t.Fatalf("触发词未命中时不应返回技能声明的服务器，实际 %v", ids)
	}

	// 未配置触发词的技能不参与（否则其声明的服务器会对所有问题常驻注入）。
	if err := db.Dao.Model(&models.Skill{}).Where("name = ?", "文档同步技能").
		Update("trigger_keywords", "").Error; err != nil {
		t.Fatalf("清空触发词失败: %v", err)
	}
	if ids := activeSkillMCPServerIDs("帮我同步文档一下"); len(ids) != 0 {
		t.Fatalf("未配置触发词的技能不应参与注入，实际 %v", ids)
	}
}

// 注入的 MCP 工具必须被标记为动态工具（*mcpToolWrapper），DeepAgents 才能把它们
// 交给 ToolSearch 动态检索。修复前 MarkMCPTools 从未被调用，该标记恒为 false。
func TestInjectedMCPToolsAreMarkedDynamic(t *testing.T) {
	initMCPInjectionTestDB(t)

	seedMCPServer(t, models.MCPServer{
		Name: "feishu-hub", URL: "http://127.0.0.1:9/mcp", Type: "streamable-http",
		Enable: true, Status: "available",
	})
	withServerToolsLoader(t, map[string][]tool.BaseTool{
		"feishu-hub": {fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"}},
	}, nil)

	injected := maybeGetMCPTools("用 feishu-hub 发个消息", mcpInjectContext{}, false)
	if len(injected) != 1 {
		t.Fatalf("应注入 1 个工具，实际 %d", len(injected))
	}
	if !tools.IsDynamicTool(injected[0]) {
		t.Fatalf("注入的 MCP 工具应被标记为动态工具（IsDynamicTool=true）")
	}
}
