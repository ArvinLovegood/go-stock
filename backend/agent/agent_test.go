package agent

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/fileutil"
)

func TestGetStockAiAgent(t *testing.T) {
	ctx := context.Background()
	db.Init("../../data/stock.db")
	config := data.GetSettingConfig()
	agentInstance := GetStockAiAgent(&ctx, *config.AiConfigs[0], "分析当前市场情绪和热点", "")

	if agentInstance == nil {
		t.Fatal("agent instance is nil")
	}

	t.Logf("Agent mode: %s", agentInstance.Mode)

	switch agentInstance.Mode {
	case AgentModePlanExecute:
		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent: agentInstance.AdkAgent,
		})
		messages := []*schema.Message{
			{Role: schema.System, Content: config.Settings.Prompt + ""},
			{Role: schema.User, Content: "结合以上提供的宏观经济数据/市场指数行情/国内外市场资讯/电报/会议/事件/投资者关注的问题，\n结合宏观经济，事件驱动，政策支持，投资者关注的问题，分析当前市场情绪和热点 找出有潜力/优质的板块/行业/概念/标的/主题，\n多因子深度分析计算上涨或下跌的逻辑和概率，\n最后按风险和投资周期给出具体推荐标的操作建议"},
		}
		iter := runner.Run(ctx, messages)
		md := strings.Builder{}
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event == nil || event.Err != nil {
				continue
			}
			if event.Output != nil && event.Output.MessageOutput != nil {
				mv := event.Output.MessageOutput
				if mv.Message != nil {
					if mv.Message.ReasoningContent != "" {
						md.WriteString(mv.Message.ReasoningContent)
					}
					if mv.Message.Content != "" {
						md.WriteString(mv.Message.Content)
					}
				}
			}
		}
		logger.SugaredLogger.Info(md.String())

	default:
		md := strings.Builder{}
		ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
		for message := range ch {
			logger.SugaredLogger.Infof("res:%s", message.String())
			md.WriteString(message.String())
		}
		logger.SugaredLogger.Info(md.String())
	}
}

func TestCollapseLongMarkdownTableRunsIsolation(t *testing.T) {
	run := make([]string, 0, 22)
	for i := 0; i < 22; i++ {
		run = append(run, fmt.Sprintf("| col | %02d |", i))
	}
	out := collapseLongMarkdownTableRuns(run)
	if !strings.Contains(strings.Join(out, "\n"), "省略") {
		t.Fatalf("isolated collapse failed: %q", strings.Join(out, "\n"))
	}
}

func TestCollapseLongMarkdownTableRuns(t *testing.T) {
	var b strings.Builder
	_, _ = b.WriteString("前言\n")
	for i := 0; i < 22; i++ {
		_, _ = fmt.Fprintf(&b, "| col | %02d |\n", i)
	}
	_, _ = b.WriteString("后记\n")
	content := b.String()
	norm := normalizeCompressLines(content)
	if !strings.Contains(strings.Join(norm, "\n"), "省略") {
		t.Fatalf("normalize should collapse table: lines=%d body=%q", len(norm), strings.Join(norm, "\n"))
	}
	out := smartContentCompress(content, 900)
	if !strings.Contains(out, "省略") || !strings.Contains(out, "markdown") {
		t.Fatalf("expected long markdown table collapsed: %q", out)
	}
	if !strings.Contains(out, "| col | 21 |") {
		t.Fatalf("expected tail table rows preserved: %q", out)
	}
}

func TestSupplementaryFactLineKeepsStockDigits(t *testing.T) {
	var parts []string
	for i := 0; i < 18; i++ {
		parts = append(parts, "叙述性占位句子不含百分号。")
	}
	parts = append(parts, "重点观测标的代码 600519 仅供单元测试")
	for i := 0; i < 18; i++ {
		parts = append(parts, "叙述性占位句子不含百分号。")
	}
	content := strings.Join(parts, "\n")
	out := smartContentCompress(content, 420)
	if !strings.Contains(out, "600519") {
		t.Fatalf("expected supplementary fact line retained under tight budget: %q", out)
	}
}

func TestSmartContentCompressDedupeAndTail(t *testing.T) {
	repeat := strings.Repeat("重复行\n", 24)
	tail := "尾部结论：综上所述建议持有\n"
	data := "成交额 123.45 亿元\n"
	content := repeat + data + tail
	out := smartContentCompress(content, 120)
	if strings.Count(out, "重复行") > 1 {
		t.Fatalf("expected consecutive duplicate lines collapsed: %q", out)
	}
	if !strings.Contains(out, "综上所述") {
		t.Fatalf("expected tail summary kept: %q", out)
	}
}

func TestCompressExecutedStepResult(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		_, _ = fmt.Fprintf(&b, "第%03d段 ROE=%.1f%% 营收同比说明文字占位\n", i, float64(i%13)+0.5)
	}
	line := b.String()
	old := compressExecutedStepResult(line, 0, 8, false)
	recent := compressExecutedStepResult(line, 7, 8, false)
	if len(old) >= len(recent) {
		t.Fatalf("expected older step (smaller byte budget) more compressed: old=%d recent=%d", len(old), len(recent))
	}
	if len(recent) < 200 {
		t.Fatalf("expected recent step to retain meaningful content: len=%d", len(recent))
	}
}

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		question string
		expected AgentMode
	}{
		{"今天茅台股价多少", AgentModeReact},
		{"查询一下平安银行的代码", AgentModeReact},
		{"全面分析贵州茅台的投资价值", AgentModePlanExecute},
		{"综合分析当前市场热点和投资机会", AgentModePlanExecute},
		{"帮我查一下今天大盘行情", AgentModeReact},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", AgentModePlanExecute},
	}

	for _, tt := range tests {
		result := classifyComplexity(tt.question)
		if result != tt.expected {
			t.Errorf("question=%q expected=%s got=%s", tt.question, tt.expected, result)
		}
	}
}

func TestAgent(t *testing.T) {
	db.Init("../../data/stock.db")

	md := strings.Builder{}
	ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
	for message := range ch {
		logger.SugaredLogger.Infof("res:%s", message.String())
		md.WriteString(message.String())
	}
	logger.SugaredLogger.Info(md.String())
	fileutil.WriteStringToFile("../../data/result.md", md.String(), false)
}
