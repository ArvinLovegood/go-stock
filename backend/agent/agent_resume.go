package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildAgentResumePrompt 将运行快照转换为受控恢复提示。
// 快照只提供历史事实和约束，模型仍需重新验证实时数据，不能把摘要当作最新行情。
func BuildAgentResumePrompt(snapshot AgentRunSnapshot) string {
	var b strings.Builder
	b.WriteString("\n\n## Agent 恢复上下文\n")
	b.WriteString("这是一次未完成任务的恢复执行。请继续完成原任务，不要把以下摘要当作最新数据。\n")
	b.WriteString("恢复原则：已成功完成的工具调用不要无意义重复；涉及实时行情、价格、涨跌幅和财务指标时，仍必须根据需要重新调用工具验证。\n")
	fmt.Fprintf(&b, "原运行 ID：%s；上次阶段：%s；上次更新时间：%s。\n", snapshot.ID, snapshot.Phase, snapshot.UpdatedAt.Format("2006-01-02 15:04:05"))

	limit := len(snapshot.Events)
	if limit > 30 {
		limit = 30
	}
	for _, event := range snapshot.Events[len(snapshot.Events)-limit:] {
		switch event.Type {
		case "phase":
			fmt.Fprintf(&b, "- 阶段：%s\n", event.Name)
		case "tool":
			line := fmt.Sprintf("- 工具 %s：%s", event.Name, event.Status)
			if event.ArgPreview != "" {
				line += "；参数=" + event.ArgPreview
			}
			if event.ResultPreview != "" {
				line += "；摘要=" + event.ResultPreview
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("请从上次未完成阶段继续，最终明确说明哪些数据已验证、哪些步骤仍未完成。\n")
	return b.String()
}

// ResumeAgentRun 从本地快照启动一个新的恢复批次，旧快照保留用于审计。
// 新批次会使用原问题、原会话和快照上下文，但拥有新的运行 ID。
func (receiver StockAiAgent) ResumeAgentRun(ctx context.Context, runID string, aiConfigID int, sysPromptID *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string) chan *schema.Message {
	store := NewAgentRunCheckpointStore(deepAgentRootDir())
	snapshot, err := store.Load(runID)
	if err != nil {
		return failedAgentMessageChannel(fmt.Sprintf("❌ 无法加载 Agent 运行快照：%v", err))
	}
	if snapshot.State == AgentRunComplete {
		return failedAgentMessageChannel("❌ 该 Agent 运行已经完成，无需恢复")
	}
	if strings.TrimSpace(snapshot.Question) == "" {
		return failedAgentMessageChannel("❌ 运行快照缺少原始问题，无法恢复")
	}
	if aiConfigID <= 0 {
		aiConfigID = snapshot.AIConfigID
	}
	if aiConfigID <= 0 {
		return failedAgentMessageChannel("❌ 运行快照缺少 AI 配置，请显式指定 aiConfigId")
	}
	if agentMode == "" {
		agentMode = string(snapshot.Mode)
	}
	return receiver.ChatWithContext(
		ctx,
		snapshot.Question,
		aiConfigID,
		sysPromptID,
		memoryMode,
		memoryCount,
		thinkingMode,
		agentMode,
		"",
		snapshot.SessionID,
		BuildAgentResumePrompt(snapshot),
	)
}

func failedAgentMessageChannel(content string) chan *schema.Message {
	ch := make(chan *schema.Message, 1)
	ch <- &schema.Message{Role: schema.Assistant, Content: content}
	close(ch)
	return ch
}
