package agent

// mcp_server_injection_test.go — MCP 按服务器粒度注入的守护测试。
//
// 目的：命中某台 MCP 服务器时只注入该服务器的工具（整台注入，保证同服务器内
// 多步调用链路完整），不再「命中一个工具就注入全部服务器」，避免无关 MCP schema
// 每轮固定占用 token。

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// fakeMCPTool 用于构造确定性的 MCP 工具（名称 + 描述），不发任何网络请求。
type fakeMCPTool struct {
	name string
	desc string
}

func (f fakeMCPTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: f.name, Desc: f.desc}, nil
}

func testMCPServerGroups() []mcpServerTools {
	return []mcpServerTools{
		{
			serverName: "feishu",
			tools: []tool.BaseTool{
				fakeMCPTool{name: "send_message", desc: "发送飞书消息给指定用户"},
				fakeMCPTool{name: "list_chats", desc: "列出飞书会话"},
			},
		},
		{
			serverName: "jira",
			tools: []tool.BaseTool{
				fakeMCPTool{name: "create_issue", desc: "创建JIRA工单"},
				fakeMCPTool{name: "search_issues", desc: "检索JIRA工单"},
			},
		},
		{
			serverName: "weather-mcp",
			tools: []tool.BaseTool{
				fakeMCPTool{name: "get_forecast", desc: "查询未来天气"},
			},
		},
	}
}

// selectMCPToolsByQuestionOnly 只按问题注入（无系统提示词、无技能声明线索），
// 等价于「未配置提示词模板 + 未命中技能」这一最常见场景。
func selectMCPToolsByQuestionOnly(question string, groups []mcpServerTools) (injected []tool.BaseTool, hitServers []string, total int) {
	return selectMCPToolsForQuestion(question, mcpInjectContext{}, groups)
}

func mcpTestToolNames(tl []tool.BaseTool) []string {
	out := make([]string, 0, len(tl))
	for _, t := range tl {
		if info, err := t.Info(context.Background()); err == nil && info != nil {
			out = append(out, info.Name)
		}
	}
	return out
}

func containsName(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// 命中服务器名 → 只注入该服务器的全部工具。
func TestSelectMCPToolsByServerName(t *testing.T) {
	injected, hit, total := selectMCPToolsByQuestionOnly("帮我在飞书发个消息", testMCPServerGroups())
	if total != 5 {
		t.Fatalf("候选工具总数应为 5，实际 %d", total)
	}
	if len(hit) != 1 || hit[0] != "feishu" {
		t.Fatalf("应只命中 feishu 服务器，实际 %v", hit)
	}
	got := mcpTestToolNames(injected)
	if len(got) != 2 || !containsName(got, "send_message") || !containsName(got, "list_chats") {
		t.Fatalf("feishu 应整台注入 2 个工具，实际 %v", got)
	}
	for _, n := range got {
		if n == "create_issue" || n == "search_issues" || n == "get_forecast" {
			t.Fatalf("不应注入其它服务器的工具: %v", got)
		}
	}
}

// 未命中服务器名、但命中某服务器工具名 → 只注入该服务器中命中的那个工具
// （工具级裁剪：命中一个工具不应带出整台 schema）。
func TestSelectMCPToolsByToolName(t *testing.T) {
	injected, hit, _ := selectMCPToolsByQuestionOnly("帮我 create_issue 一下", testMCPServerGroups())
	if len(hit) != 1 || hit[0] != "jira" {
		t.Fatalf("应只命中 jira 服务器，实际 %v", hit)
	}
	got := mcpTestToolNames(injected)
	if len(got) != 1 || got[0] != "create_issue" {
		t.Fatalf("jira 应只注入命中的 create_issue，实际 %v", got)
	}
}

// 大小写不敏感：服务器名命中。
func TestSelectMCPToolsServerNameCaseInsensitive(t *testing.T) {
	_, hit, _ := selectMCPToolsByQuestionOnly("我们FEISHU那里同步一下", testMCPServerGroups())
	if len(hit) != 1 || hit[0] != "feishu" {
		t.Fatalf("服务器名匹配应不区分大小写，实际 %v", hit)
	}
}

// 命中工具描述关键词 → 只注入对应服务器。
func TestSelectMCPToolsByDescription(t *testing.T) {
	_, hit, _ := selectMCPToolsByQuestionOnly("明天天气怎么样", testMCPServerGroups())
	if len(hit) != 1 || hit[0] != "weather-mcp" {
		t.Fatalf("应只命中 weather-mcp 服务器，实际 %v", hit)
	}
}

// 多台服务器同时相关 → 同时注入。
func TestSelectMCPToolsMultipleServers(t *testing.T) {
	_, hit, _ := selectMCPToolsByQuestionOnly("把 jira 工单结果发到飞书", testMCPServerGroups())
	if len(hit) != 2 || !containsName(hit, "feishu") || !containsName(hit, "jira") {
		t.Fatalf("应命中 feishu+jira，实际 %v", hit)
	}
}

// 完全不相关 → 不注入任何 MCP 工具（避免无谓 token 占用）。
func TestSelectMCPToolsNoMatch(t *testing.T) {
	injected, _, _ := selectMCPToolsByQuestionOnly("算了", testMCPServerGroups())
	if len(injected) != 0 {
		t.Fatalf("无关问题不应注入 MCP 工具，实际 %v", mcpTestToolNames(injected))
	}
}

// 工具自身描述为空时，用所属服务器的描述兜底匹配（部分 MCP 服务器只下发工具名）。
func TestSelectMCPToolsFallsBackToServerDesc(t *testing.T) {
	groups := []mcpServerTools{{
		serverName: "dex-bridge",
		serverDesc: "提供跨链实时加密货币和 DEX 数据，支持查询代币价格与流动性池状态",
		tools:      []tool.BaseTool{fakeMCPTool{name: "getPoolDetails", desc: ""}},
	}}

	injected, hit, _ := selectMCPToolsByQuestionOnly("查一下 DEX 流动性池状态", groups)
	if len(injected) != 1 || len(hit) != 1 || hit[0] != "dex-bridge" {
		t.Fatalf("工具描述为空时应按服务器描述兜底注入，实际 injected=%v hit=%v", mcpTestToolNames(injected), hit)
	}

	// 服务器描述也为空 → 无任何线索，不应注入。
	groups[0].serverDesc = ""
	if injected, _, _ = selectMCPToolsByQuestionOnly("查一下 DEX 流动性池状态", groups); len(injected) != 0 {
		t.Fatalf("无描述可依据时不应注入，实际 %v", mcpTestToolNames(injected))
	}
}

// 空服务器名不应误命中（否则 "" 会作为子串匹配一切，导致整包注入）。
func TestSelectMCPToolsEmptyServerName(t *testing.T) {
	groups := []mcpServerTools{
		{serverName: "", tools: []tool.BaseTool{fakeMCPTool{name: "xyzzy", desc: "无关工具"}}},
	}
	injected, hit, _ := selectMCPToolsByQuestionOnly("任意问题", groups)
	if len(injected) != 0 || len(hit) != 0 {
		t.Fatalf("空服务器名不应被注入，实际 injected=%v hit=%v", mcpTestToolNames(injected), hit)
	}
	// serverID=0（非持久化服务器）不应被技能声明误命中。
	if injected, _, _ = selectMCPToolsForQuestion("任意问题", mcpInjectContext{skillServerIDs: []uint{0}}, groups); len(injected) != 0 {
		t.Fatalf("serverID=0 不应被技能声明命中，实际 %v", mcpTestToolNames(injected))
	}
}

// 技能声明依赖的服务器 → 整台注入，即使问题本身与该服务器毫无关联
// （技能跑起来就是要用这些服务器，工具被裁掉会让技能失效）。
func TestSelectMCPToolsBySkillDeclaredServer(t *testing.T) {
	groups := []mcpServerTools{{
		serverID:   7,
		serverName: "docs-sync-bridge",
		tools: []tool.BaseTool{
			fakeMCPTool{name: "sync_docs", desc: "同步文档"},
			fakeMCPTool{name: "list_docs", desc: "列出文档"},
		},
	}}

	injected, hit, _ := selectMCPToolsForQuestion("随便聊聊", mcpInjectContext{skillServerIDs: []uint{7}}, groups)
	got := mcpTestToolNames(injected)
	if len(got) != 2 || !containsName(got, "sync_docs") || !containsName(got, "list_docs") {
		t.Fatalf("技能声明的服务器应整台注入，实际 injected=%v hit=%v", got, hit)
	}
}

// 系统提示词里出现服务器名 → 整台注入（用户在提示词模板中指定了要用哪台服务器）。
func TestSelectMCPToolsByPromptServerName(t *testing.T) {
	prompt := "你是运维助手。所有工单都必须用 jira 记录，完成后同步给相关同学。"
	injected, hit, _ := selectMCPToolsForQuestion("随便聊聊", mcpInjectContext{promptHint: prompt}, testMCPServerGroups())

	if len(hit) != 1 || hit[0] != "jira" {
		t.Fatalf("提示词点名服务器应整台注入 jira，实际 %v", hit)
	}
	got := mcpTestToolNames(injected)
	if len(got) != 2 || !containsName(got, "create_issue") || !containsName(got, "search_issues") {
		t.Fatalf("jira 应整台注入 2 个工具，实际 %v", got)
	}
}

// 系统提示词里出现工具全名 → 只注入该工具（单台封顶），不带出整台 schema。
func TestSelectMCPToolsByPromptToolName(t *testing.T) {
	prompt := "流程要求：收到需求后先调用 create_issue 建单，再回复用户。"
	injected, hit, _ := selectMCPToolsForQuestion("随便聊聊", mcpInjectContext{promptHint: prompt}, testMCPServerGroups())

	if len(hit) != 1 || hit[0] != "jira" {
		t.Fatalf("应按提示词命中 jira，实际 %v", hit)
	}
	got := mcpTestToolNames(injected)
	if len(got) != 1 || got[0] != "create_issue" {
		t.Fatalf("只应注入提示词点名的 create_issue，实际 %v", got)
	}
}

// 提示词侧只认强信号：长提示词与工具描述共享关键词窗口不算命中。
//
// 这是本轮的关键取舍——系统提示词动辄数千字，若允许窗口匹配，几乎必然命中所有工具，
// 退回「MCP schema 每轮固定占用 token」的老问题。此处断言只共享窗口时零注入。
func TestSelectMCPToolsPromptHintDoesNotWindowMatch(t *testing.T) {
	groups := []mcpServerTools{{
		serverName: "alert-hub",
		tools:      []tool.BaseTool{fakeMCPTool{name: "send_alert", desc: "风险控制提醒推送"}},
	}}

	prompt := "你是投资大师，必须始终把风险控制放在第一位，关注仓位管理。"
	if injected, _, _ := selectMCPToolsForQuestion("随便聊聊", mcpInjectContext{promptHint: prompt}, groups); len(injected) != 0 {
		t.Fatalf("提示词与工具描述共享关键词不应触发注入，实际 %v", mcpTestToolNames(injected))
	}
}

// 提示词点名大量工具时按 mcpMaxToolsPerServer 封顶。
func TestSelectMCPToolsByPromptToolNameCapped(t *testing.T) {
	tools := make([]tool.BaseTool, 0, mcpMaxToolsPerServer+5)
	var sb strings.Builder
	for i := 0; i < mcpMaxToolsPerServer+5; i++ {
		name := fmt.Sprintf("bulk_tool_%02d", i)
		tools = append(tools, fakeMCPTool{name: name})
		sb.WriteString("调用 " + name + "；")
	}
	groups := []mcpServerTools{{serverName: "bulk-hub", tools: tools}}

	injected, _, _ := selectMCPToolsForQuestion("随便聊聊", mcpInjectContext{promptHint: sb.String()}, groups)
	if len(injected) != mcpMaxToolsPerServer {
		t.Fatalf("提示词点名工具数应封顶 %d，实际 %d", mcpMaxToolsPerServer, len(injected))
	}
}
