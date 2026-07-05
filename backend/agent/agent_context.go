package agent

import (
	"fmt"
	"go-stock/backend/data"
	"strings"
	"time"
)

// buildAgentTimeContext 注入当前时间与市场状态，避免模型依赖训练数据中的过时时间/行情。
func buildAgentTimeContext() string {
	now := time.Now()
	weekday := data.WeekdayCN(now.Weekday())

	var sb strings.Builder
	sb.WriteString("\n\n【当前环境】\n")
	sb.WriteString(fmt.Sprintf("- 本地时间：%s %s\n", now.Format("2006-01-02 15:04:05"), weekday))

	if status := strings.TrimSpace(data.NewMarketNewsApi().GlobalStockIndexesReadable(30)); status != "" {
		sb.WriteString("- 全球市场状态：\n")
		for _, line := range strings.Split(status, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	}

	sb.WriteString("- A股是否交易日请以 IsTradingDay 工具查询为准（含法定节假日与调休）。\n")
	sb.WriteString("- 对话历史中的股价、财务等数字可能已过时；涉及具体数值必须重新调用工具，不得直接引用历史数字或训练记忆。\n")
	sb.WriteString("- 工具返回以 [as_of=...] [tool=...] [status=...] 元数据为准；status=empty/error 时不得编造数据。\n")

	return sb.String()
}
