package agent

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"

	"go-stock/backend/agent/tools"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

var (
	answerNumericClaimPattern = regexp.MustCompile(`(?:[-+]?\d[\d,]*(?:\.\d+)?)\s*(?:%|％|元|块|亿|万|点|倍|PE|PB|ROE|EPS|市值|成交量|成交额)`)
	answerPlainPercentPattern = regexp.MustCompile(`(?:[-+]?\d[\d,]*(?:\.\d+)?)\s*(?:%|％)`)
)

var nonDataTools = map[string]bool{
	"GetCurrentTime":     true,
	"GetHolidayInfo":     true,
	"GetHolidayYear":     true,
	"GetHolidayBatch":    true,
	"IsTradingDay":       true,
	"GetNextTradingDay":  true,
	"QueryStockCodeInfo": true,
	"QueryBKDictInfo":    true,
	"ListMCPServers":     true,
	"GetMCPServerDetail": true,
	"plan":               true,
	"respond":            true,
}

func isDataFetchingTool(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return !nonDataTools[name]
}

type GuardVerdict struct {
	OK          bool
	Reason      string
	ShouldRetry bool
}

func questionRequiresFactualData(question string) bool {
	lower := strings.ToLower(question)

	if containsAnyKeyword(lower,
		"什么是", "是什么", "什么意思", "原理", "定义", "如何理解", "介绍一下", "科普",
	) {
		if !containsAnyKeyword(lower,
			"股价", "价格", "行情", "多少", "涨跌", "涨幅", "跌幅", "今天", "最新", "当前", "实时",
		) {
			return false
		}
	}

	switch tools.DetectQuestionIntent(question) {
	case tools.IntentQuoteLookup, tools.IntentCodeLookup, tools.IntentScreening,
		tools.IntentMarketOverview, tools.IntentNewsResearch, tools.IntentMoneyFlow,
		tools.IntentComprehensiveReport:
		return true
	default:
		return containsAnyKeyword(lower,
			"股价", "价格", "行情", "多少", "涨跌", "涨幅", "财务", "营收", "利润",
			"市盈率", "市净率", "资金", "流入", "流出", "涨停", "跌停", "指数",
		)
	}
}

func containsAnyKeyword(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if kw != "" && strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func responseHasNumericFactualClaims(content string) bool {
	cleaned := stripNonAnswerLines(content)
	if strings.TrimSpace(cleaned) == "" {
		return false
	}
	if answerNumericClaimPattern.MatchString(cleaned) {
		return true
	}
	if answerPlainPercentPattern.MatchString(cleaned) {
		return true
	}
	return false
}

func responseAdmitsDataUnavailable(content string) bool {
	markers := []string{
		"未能获取", "未获取到", "无法获取", "暂无数据", "没有获取到",
		"工具返回 status=empty", "status=error", "status=empty",
	}
	lower := strings.ToLower(content)
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func stripNonAnswerLines(content string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "[STEP]") || strings.HasPrefix(t, "[FALLBACK]") {
			continue
		}
		if strings.HasPrefix(t, "⚠️") && strings.Contains(t, "检测到") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func evaluateResponseGuard(question, answer string, trace *AgentTurnTrace) GuardVerdict {
	if strings.TrimSpace(answer) == "" {
		return GuardVerdict{OK: true, Reason: "empty_answer"}
	}
	if !questionRequiresFactualData(question) {
		return GuardVerdict{OK: true, Reason: "no_factual_requirement"}
	}
	if !responseHasNumericFactualClaims(answer) {
		return GuardVerdict{OK: true, Reason: "no_numeric_claims"}
	}
	if trace != nil && trace.HasSuccessfulDataToolCall() {
		return GuardVerdict{OK: true, Reason: "verified_tool_data"}
	}
	if responseAdmitsDataUnavailable(answer) {
		return GuardVerdict{OK: true, Reason: "explicit_data_unavailable"}
	}

	shouldRetry := trace == nil || !trace.HasAnyDataToolCall()
	return GuardVerdict{
		OK:          false,
		Reason:      "numeric_claims_without_verified_tool_data",
		ShouldRetry: shouldRetry,
	}
}

func buildGuardRetryPrompt(question string, verdict GuardVerdict) string {
	_ = verdict
	return "你上一轮回答含有具体数值，但未通过数据工具验证。请立即调用相关工具（如 GetStockInfo、GetMarketData、GetStockFinancialInfo 等）重新查询后再回答。" +
		"若工具返回 status=empty 或 status=error，只能明确说明未能获取数据，不得编造数字。原问题：" + question
}

func renderGuardDisclaimer(verdict GuardVerdict) string {
	return "\n\n---\n⚠️ **数据提示**：本次回答中的部分数值可能未通过工具验证（原因：" + verdict.Reason + "），请以工具查询结果为准。"
}

func enforceResponseGuard(
	ctx context.Context,
	mode string,
	question string,
	answer string,
	reactAgent *react.Agent,
	messages []*schema.Message,
	agentOptions []agent.AgentOption,
	ch chan *schema.Message,
	stockAiAgent *StockAiAgent,
	thinkingMode bool,
) string {
	trace := AgentTurnTraceFromContext(ctx)
	verdict := evaluateResponseGuard(question, answer, trace)
	if trace != nil {
		trace.GuardNote = verdict.Reason
	}
	if verdict.OK {
		return answer
	}

	logger.SugaredLogger.Warnf("response guard triggered: mode=%s reason=%s tools=[%s]", mode, verdict.Reason, strings.Join(traceToolNames(trace), ", "))

	if !verdict.ShouldRetry || reactAgent == nil {
		disclaimer := renderGuardDisclaimer(verdict)
		if ch != nil {
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: disclaimer,
			})
		}
		return answer + disclaimer
	}

	if len(agentOptions) == 0 {
		msgFutureOpt, _ := react.WithMessageFuture()
		opts := agent.GetComposeOptions(msgFutureOpt)
		agentOptions = []agent.AgentOption{agent.WithComposeOptions(opts...)}
	}

	if trace != nil {
		trace.ResetToolCalls()
		trace.GuardNote = verdict.Reason + "; retrying"
	}
	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: "[STEP]⚠️ 检测到未验证数值，正在重新调用工具查询...\n",
	})

	retryMessages := append([]*schema.Message{}, messages...)
	retryMessages = append(retryMessages,
		&schema.Message{Role: schema.Assistant, Content: answer},
		&schema.Message{Role: schema.User, Content: buildGuardRetryPrompt(question, verdict)},
	)
	retryMessages = validateAndFixMessages(retryMessages)

	retryAnswer := streamReactFinalContent(ctx, reactAgent, retryMessages, agentOptions, ch)
	retryVerdict := evaluateResponseGuard(question, retryAnswer, trace)
	if trace != nil {
		trace.GuardNote = retryVerdict.Reason
	}
	if retryVerdict.OK {
		return retryAnswer
	}

	disclaimer := renderGuardDisclaimer(retryVerdict)
	safeSend(ch, &schema.Message{Role: schema.Assistant, Content: disclaimer})
	return retryAnswer + disclaimer
}

func streamReactFinalContent(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, agentOptions []agent.AgentOption, ch chan *schema.Message) string {
	if reactAgent == nil {
		return ""
	}
	sr, err := reactAgent.Stream(ctx, messages, agentOptions...)
	if err != nil {
		logger.SugaredLogger.Warnf("response guard retry stream error: %v", err)
		return ""
	}
	defer sr.Close()

	var b strings.Builder
	for {
		msg, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			logger.SugaredLogger.Warnf("response guard retry recv error: %v", err)
			break
		}
		if msg == nil || msg.Content == "" {
			continue
		}
		b.WriteString(msg.Content)
		if ch != nil {
			safeSend(ch, &schema.Message{Role: schema.Assistant, Content: msg.Content})
		}
	}
	return b.String()
}

func traceToolNames(trace *AgentTurnTrace) []string {
	if trace == nil {
		return nil
	}
	return trace.ToolNames()
}

func resolveReactAgentForGuard(stockAiAgent *StockAiAgent, thinkingMode bool, ctx context.Context) *react.Agent {
	if stockAiAgent == nil || stockAiAgent.instance == nil {
		return nil
	}
	if stockAiAgent.instance.ReactAgent != nil {
		return stockAiAgent.instance.ReactAgent
	}
	return createFallbackReactAgent(ctx, stockAiAgent, thinkingMode)
}
