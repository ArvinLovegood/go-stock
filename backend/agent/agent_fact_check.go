package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

var financialFactRe = regexp.MustCompile(`(?:[630]\d{5}(?:\.(?:SH|SZ|BJ))?|\d+(?:\.\d+)?%|\d+(?:\.\d+)?元|市盈率|市净率|股价|涨跌幅|成交量|目标价|换手率)`)

// VerifyFinancialAnswer 对最终回答做轻量事实门禁。
// 这是“提醒层”而不是 LLM 评判器：只判断回答是否包含金融事实表达、
// 本轮是否拿到成功工具结果，不擅自判断投资结论是否正确。
func VerifyFinancialAnswer(answer string, trace *AgentTurnTrace) string {
	answer = strings.TrimSpace(answer)
	if answer == "" || !financialFactRe.MatchString(answer) {
		return ""
	}
	if trace != nil && trace.SuccessfulToolCallCount() > 0 {
		return ""
	}
	return "回答包含股票或行情事实，但本轮没有成功获取可验证的工具数据；请将相关数字改为‘未获取到’或补充数据来源和时间。"
}

// SendFinancialFactCheck 将校验结果作为独立的进度/提醒事件发送，
// 不修改已经流式发送给用户的正文。
func SendFinancialFactCheck(ctx context.Context, ch chan *schema.Message, answer string) {
	warning := VerifyFinancialAnswer(answer, AgentTurnTraceFromContext(ctx))
	if warning == "" || ch == nil {
		return
	}
	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: "[FACT_CHECK]⚠️ " + warning + "\n",
	})
}
