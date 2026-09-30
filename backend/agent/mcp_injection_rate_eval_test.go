package agent

// mcp_injection_rate_eval_test.go — 基于真实 MCP 配置的「工具注入率」评估。
//
// 数据源：backend/agent/testdata/mcp_eval_servers.json，由 D:\go-stock\data\stock.db
// 中「enable=1 AND status='available'」的服务器与其 mcp_server_tools 落库工具导出
// （17 台服务器 / 205 个工具，快照时间 2026-09-30）。
//
// 评估口径：调用生产同款注入函数 maybeGetMCPTools(question, mcpInjectContext{}, false)
//（空注入线索，等价 React / PlanExecute / auto 路径下的「无提示词点名、无技能声明」场景），
// 统计每个问题「注入了哪些服务器、注入了多少个工具」，
// 再按「正样本是否命中期望服务器 / 负样本是否误召」汇总。
// （DeepAgents 走 alwaysIncludeMCP=true 恒注入全部，不做召回统计，只统计 token 成本。）
//
// 运行：go test ./backend/agent/ -run TestMCPInjectionRateEval -v -count=1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"go-stock/backend/models"

	"github.com/cloudwego/eino/components/tool"
)

type evalTool struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type evalServer struct {
	SrcID  int        `json:"srcId"`
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Enable bool       `json:"enable"`
	Desc   string     `json:"desc"`
	Tools  []evalTool `json:"tools"`
}

type evalFixture struct {
	Source  string       `json:"source"`
	Servers []evalServer `json:"servers"`
}

// evalCase 一条评估用例。expect 为空表示负样本（期望不注入任何 MCP 服务器）。
type evalCase struct {
	name     string
	question string
	expect   []string
}

func loadEvalFixture(t *testing.T) evalFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mcp_eval_servers.json"))
	if err != nil {
		t.Skipf("评估数据缺失（%v），跳过 MCP 注入率评估", err)
	}
	var fx evalFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("解析评估数据失败: %v", err)
	}
	if len(fx.Servers) == 0 {
		t.Fatal("评估数据为空")
	}
	return fx
}

// setupEvalEnv 把 fixture 灌入内存库并替换加载器（不发网络请求）。
func setupEvalEnv(t *testing.T, fx evalFixture) {
	t.Helper()
	initMCPInjectionTestDB(t)

	byServer := make(map[string][]tool.BaseTool, len(fx.Servers))
	for _, s := range fx.Servers {
		stored := seedMCPServer(t, models.MCPServer{
			Name: s.Name, Description: s.Desc, URL: "http://127.0.0.1:9/mcp",
			Type: "streamable-http", Enable: true, Status: s.Status,
		})
		ts := make([]tool.BaseTool, 0, len(s.Tools))
		for _, tl := range s.Tools {
			// 同步落库，供「廉价预判」阶段的 mcp_server_tools 匹配使用
			seedMCPServerTool(t, stored.ID, tl.Name, tl.Desc)
			ts = append(ts, fakeMCPTool{name: tl.Name, desc: tl.Desc})
		}
		byServer[s.Name] = ts
	}
	withServerToolsLoader(t, byServer, nil)
}

// injectedServerNames 由注入结果反查命中的服务器名（整台注入，任一工具出现即命中）。
func injectedServerNames(fx evalFixture, toolNames []string) []string {
	have := make(map[string]bool, len(toolNames))
	for _, n := range toolNames {
		have[n] = true
	}
	var hit []string
	for _, s := range fx.Servers {
		for _, tl := range s.Tools {
			if have[tl.Name] {
				hit = append(hit, s.Name)
				break
			}
		}
	}
	return hit
}

func expectHit(expect, got []string) bool {
	for _, e := range expect {
		for _, g := range got {
			if e == g {
				return true
			}
		}
	}
	return false
}

func evalMean(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0
	for _, x := range xs {
		s += x
	}
	return float64(s) / float64(len(xs))
}

func evalMedian(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	return cp[len(cp)/2]
}

func evalCases() []evalCase {
	return []evalCase{
		// A. 问题里点名服务器名
		{"A1 点名-无前导空格", "用 westock-mcp 查一下贵州茅台的K线", []string{"westock-mcp"}},
		{"A2 点名-中文名", "帮我调用公募基金看看易方达蓝筹精选", []string{"公募基金"}},
		{"A3 点名-名含内部空格", "用 A 股标的宇宙 拉一下涨停梯队", []string{"A 股标的宇宙"}},
		{"A4 点名-两字短名", "用期货查螺纹钢主力合约", []string{"期货"}},
		{"A5 点名-名带前导空格(原样)", "用 加密货币 查比特币", []string{" 加密货币 "}},
		{"A6 点名-名带前导空格(用户自然写法)", "用加密货币查一下比特币价格", []string{" 加密货币 "}},

		// B. 纯内容命中：不含服务器名、不含 MCP 信号词
		{"B1 内容-龙虎榜", "今天的龙虎榜有哪些股票", []string{"A 股标的宇宙", "westock-mcp"}},
		{"B2 内容-基金净值", "易方达蓝筹精选的单位净值是多少", []string{"公募基金"}},
		{"B3 内容-北向持股", "北向资金最近持有哪些股票", []string{"westock-mcp"}},
		{"B4 内容-日K线", "贵州茅台的日K线数据", []string{"westock-mcp", "A 股标的宇宙"}},
		{"B5 内容-期货日K", "螺纹钢期货的日K线", []string{"期货"}},
		{"B6 内容-条件选股", "帮我筛选市盈率低于15的股票", []string{"妙想 MCP"}},
		{"B7 内容-网页转markdown", "把 https://example.com 这个网页转成 markdown", []string{"markitdown"}},
		{"B8 内容-资讯搜索", "搜索一下今天的财经新闻", []string{"Baidu Search", "妙想 MCP", "westock-mcp"}},
		{"B9 内容-指数行情", "沪深300指数现在多少点", []string{"A 股同花顺指数", "A 股标的宇宙"}},
		{"B10 内容-空描述服务器", "美元兑人民币的汇率是多少", []string{"货币与石油价格"}},
		{"B11 内容-英文服务器", "获取 Hacker News 首页热门故事", []string{" Hacker News "}},
		{"B12 内容-华尔街见闻", "华尔街见闻今天有哪些大涨股票", []string{"华尔街见闻MCP", "westock-mcp"}},
		// 工具描述全空、仅剩英文服务器描述可用的服务器：中文提问无从命中（数据缺口），
		// 英文提问靠「服务器描述兜底」可达。
		{"B13 内容-空描述兜底(英文提问)", "what is the USD to RUB exchange rate", []string{"货币与石油价格"}},

		// C. 同一意图的不同提示词写法
		{"C1 变体-点名", "用 westock-mcp 拉一下龙虎榜", []string{"westock-mcp"}},
		{"C2 变体-含信号词", "帮我用工具查一下龙虎榜", []string{"A 股标的宇宙", "westock-mcp"}},
		{"C3 变体-纯语义", "看看今天游资都买了什么", []string{"A 股标的宇宙", "westock-mcp"}},

		// D. 负样本：与任何 MCP 工具无关，期望不注入
		{"D1 负样本-写代码", "帮我写一个快速排序的 Python 脚本", nil},
		{"D2 负样本-常识", "1+1 等于几", nil},
		{"D3 负样本-寒暄", "你好，你是谁", nil},
		{"D4 负样本-讲笑话", "给我讲个笑话", nil},
		{"D5 负样本-荐书", "推荐几本适合睡前读的书", nil},
	}
}

func TestMCPInjectionRateEval(t *testing.T) {
	fx := loadEvalFixture(t)
	setupEvalEnv(t, fx)

	totalTools := 0
	for _, s := range fx.Servers {
		totalTools += len(s.Tools)
	}
	t.Logf("数据源: %s", fx.Source)
	t.Logf("启用且可用服务器: %d 台, 落库工具: %d 个", len(fx.Servers), totalTools)

	cases := evalCases()
	var posTotal, posHit, negTotal, negFP int
	var posTools, posServers []int
	posMiss := map[string]bool{}

	t.Logf("%-34s | %-4s | %-6s | %-8s | %s", "用例", "工具数", "服务器数", "判定", "注入的服务器")
	for _, c := range cases {
		names := mcpTestToolNames(maybeGetMCPTools(c.question, mcpInjectContext{}, false))
		hitServers := injectedServerNames(fx, names)
		sort.Strings(hitServers)

		verdict := ""
		if len(c.expect) == 0 {
			negTotal++
			if len(hitServers) > 0 {
				negFP++
				verdict = "误召"
			} else {
				verdict = "正确不注入"
			}
		} else {
			posTotal++
			posTools = append(posTools, len(names))
			posServers = append(posServers, len(hitServers))
			if expectHit(c.expect, hitServers) {
				posHit++
				verdict = "命中"
			} else {
				verdict = "漏召"
				posMiss[c.name] = true
			}
		}
		t.Logf("%-34s | %-6d | %-8d | %-8s | %v", c.name, len(names), len(hitServers), verdict, hitServers)
	}

	t.Logf("---- 汇总（React / PlanExecute / auto 路径）----")
	if posTotal > 0 {
		t.Logf("正样本召回: %d/%d = %.0f%%", posHit, posTotal, float64(posHit)/float64(posTotal)*100)
	}
	if negTotal > 0 {
		t.Logf("负样本误召: %d/%d = %.0f%%", negFP, negTotal, float64(negFP)/float64(negTotal)*100)
	}
	if len(posMiss) > 0 {
		miss := make([]string, 0, len(posMiss))
		for k := range posMiss {
			miss = append(miss, k)
		}
		sort.Strings(miss)
		t.Logf("漏召用例: %v", miss)
	}
	if len(posTools) > 0 {
		t.Logf("正样本注入成本: 平均 %.1f 个工具 / %.1f 台服务器（中位 %d / %d），占全部工具的 %.0f%%",
			evalMean(posTools), evalMean(posServers), evalMedian(posTools), evalMedian(posServers),
			evalMean(posTools)/float64(totalTools)*100)
		t.Logf("正样本注入工具数分布: %v", posTools)
	}
	t.Logf("DeepAgents 路径（alwaysIncludeMCP=true）恒注入: %d 台服务器 / %d 个工具", len(fx.Servers), totalTools)

	// 硬性回归：点名「名字不带前后空格」的服务器必须命中（A1/A2/A4）。
	for _, c := range cases {
		if c.name != "A1 点名-无前导空格" && c.name != "A2 点名-中文名" && c.name != "A4 点名-两字短名" {
			continue
		}
		names := mcpTestToolNames(maybeGetMCPTools(c.question, mcpInjectContext{}, false))
		if !expectHit(c.expect, injectedServerNames(fx, names)) {
			t.Errorf("按服务器名点名必须命中：用例 %q 期望 %v，实际注入 %v", c.name, c.expect, injectedServerNames(fx, names))
		}
	}
}
