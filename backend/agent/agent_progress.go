package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// progressChCtxKey 用于在 context 中传递前端进度反馈 channel。
type progressChCtxKey struct{}

// WithProgressChannel 将前端消息 channel 注入 context。
// 中间件（如 errorRecoveryMiddleware）可通过 ProgressChannelFromCtx 取出，
// 在工具调用前后发送 ReasoningContent 形式的进度消息（不影响最终回答内容）。
func WithProgressChannel(ctx context.Context, ch chan *schema.Message) context.Context {
	if ch == nil {
		return ctx
	}
	return context.WithValue(ctx, progressChCtxKey{}, ch)
}

// ProgressChannelFromCtx 从 context 提取进度 channel，不存在返回 nil。
func ProgressChannelFromCtx(ctx context.Context) chan *schema.Message {
	if ctx == nil {
		return nil
	}
	ch, _ := ctx.Value(progressChCtxKey{}).(chan *schema.Message)
	return ch
}

// sendToolProgress 通过 channel 发送工具进度消息（ReasoningContent）。
// channel 不存在或已满时静默跳过，不影响主流程。
func sendToolProgress(ctx context.Context, msg string) {
	if msg == "" {
		return
	}
	ch := ProgressChannelFromCtx(ctx)
	if ch == nil {
		return
	}
	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: msg,
	})
}

// sendTurnStats 发送本轮 token 消耗统计到前端（通过 ReasoningContent）。
// 在各 run* 函数关闭 ch 之前调用，确保统计消息能送达。
func sendTurnStats(ctx context.Context, ch chan *schema.Message) {
	if ch == nil {
		return
	}
	trace := AgentTurnTraceFromContext(ctx)
	if trace == nil {
		return
	}
	stats := trace.StatsMessage()
	if stats == "" {
		return
	}
	safeSend(ch, &schema.Message{
		Role:             schema.Assistant,
		Content:          "",
		ReasoningContent: stats,
	})
}

// toolFriendlyDesc 将工具名转换为面向用户的中文动作描述。
// 例如：GetStockLatestFinance → "查询最新财务主要数据"。
// 未配置时返回空字符串，调用方自行降级。
func toolFriendlyDesc(toolName string) string {
	switch toolName {
	case "GetStockLatestFinance":
		return "查询最新财务主要数据"
	case "GetHKStockLatestFinance":
		return "查询港股最新财务主要指标"
	case "GetStockQtrMainFinance":
		return "查询季度主要财务指标"
	case "GetStockOrgPredict":
		return "查询机构预测明细"
	case "GetStockPredictSummary":
		return "查询机构预测汇总"
	case "GetStockValuationPercentile":
		return "查询估值百分位"
	case "GetStockMarginTrading":
		return "查询融资融券数据"
	case "GetStockBlockTrade":
		return "查询大宗交易数据"
	case "GetStockHolderTrend":
		return "查询户均持股趋势"
	case "GetStockBillboard":
		return "查询龙虎榜数据"
	case "GetStockOperationDeptTrade":
		return "查询营业部买卖明细"
	case "GetStockOrgBasicInfo":
		return "查询公司基础资料"
	case "GetStockQuote":
		return "查询实时行情"
	case "GetStockListByIndustry":
		return "查询行业股票列表"
	case "GetStockNewsByCode":
		return "查询个股新闻"
	case "GetMarketNews":
		return "查询市场资讯"
	case "GetMacroData":
		return "查询宏观经济数据"
	case "GetIndexQuote":
		return "查询指数行情"
	case "FinanceSearch":
		return "搜索金融资讯"
	case "FinanceDataQuery":
		return "查询金融数据"
	case "ComparableCompanyAnalysis":
		return "可比公司分析"
	case "QueryBKDictInfo":
		return "查询板块字典"
	case "CreateAiRecommendStocks":
		return "生成 AI 推荐股票"
	case "BatchCreateAiRecommendStocks":
		return "批量生成 AI 推荐股票"
	default:
		return ""
	}
}

// extractStockCodeFromArgs 从工具参数 JSON 中提取 stockCode 字段值（去引号）。
func extractStockCodeFromArgs(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	// 简易提取，避免引入 gjson 依赖到 agent 包
	idx := strings.Index(argsJSON, "\"stockCode\"")
	if idx < 0 {
		return ""
	}
	rest := argsJSON[idx+len("\"stockCode\""):]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = rest[colon+1:]
	rest = strings.TrimLeft(rest, " \t\r\n")
	if !strings.HasPrefix(rest, "\"") {
		// 非字符串，取到下一个逗号/}
		end := strings.IndexAny(rest, ",}")
		if end < 0 {
			end = len(rest)
		}
		return strings.TrimSpace(rest[:end])
	}
	rest = rest[1:]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// buildToolPreflightMsg 构造工具调用前预告消息。
// 格式："🔧 正在查询<动作描述>（<股票代码>）..."
func buildToolPreflightMsg(toolName, argsJSON string) string {
	desc := toolFriendlyDesc(toolName)
	if desc == "" {
		// 未配置友好描述的工具不发送预告，避免噪音
		return ""
	}
	code := extractStockCodeFromArgs(argsJSON)
	if code != "" {
		return fmt.Sprintf("🔧 正在%s（%s）...\n", desc, code)
	}
	return fmt.Sprintf("🔧 正在%s...\n", desc)
}

// buildToolResultSummaryMsg 构造工具调用后结果摘要消息。
// 格式："✅ <动作描述>完成（耗时 Xs，返回 N 字）：<前 80 字摘要>"
func buildToolResultSummaryMsg(toolName, result string, elapsed time.Duration) string {
	desc := toolFriendlyDesc(toolName)
	if desc == "" {
		desc = "工具调用"
	}
	preview := strings.TrimSpace(result)
	// 去除元数据前缀
	if meta, body := splitToolMetadataPrefix(preview); len(meta) > 0 {
		preview = strings.TrimSpace(body)
	}
	// 压缩到单行并截断
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "|", " ")
	preview = strings.TrimSpace(preview)
	if len([]rune(preview)) > 80 {
		preview = string([]rune(preview)[:80]) + "..."
	}
	if preview == "" {
		return fmt.Sprintf("✅ %s完成（耗时 %s）\n", desc, elapsed.Round(time.Millisecond))
	}
	return fmt.Sprintf("✅ %s完成（耗时 %s，%d 字）：%s\n",
		desc, elapsed.Round(time.Millisecond), len([]rune(result)), preview)
}
