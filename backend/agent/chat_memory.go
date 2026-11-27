package agent

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

type ChatMemoryService struct {
	sessionID string
	maxMemory int
}

func NewChatMemoryService(sessionID string, maxMemory int) *ChatMemoryService {
	if maxMemory <= 0 {
		maxMemory = 20
	}
	return &ChatMemoryService{
		sessionID: sessionID,
		maxMemory: maxMemory * 2,
	}
}

func (s *ChatMemoryService) AddUserMessage(content string) error {
	memory := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      "user",
		Content:   content,
	}
	return memory.Save()
}

func (s *ChatMemoryService) AddAssistantMessage(content string) error {
	memory := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      "assistant",
		Content:   content,
	}
	return memory.Save()
}

// assistant 历史回复在入库时为全文分析报告（数千字/条），下一轮随上下文全量重发，
// 多轮会话历史 token 随轮次线性膨胀。加载侧截断为「头 400 + 尾 200 字」：
// 保留结论开头与收尾摘要，中间数据细节省略；配合系统提示词既有规则
// 「历史数字不得直接引用，涉及具体数值必须重新调用工具」，语义风险可控。
const (
	assistantHistoryHeadChars = 400
	assistantHistoryTailChars = 200
)

// truncateAssistantHistoryContent 裁剪 assistant 历史回复（按 rune 计，中文安全）。
// 短于阈值的回复原样返回；超长时保留头尾并以省略标记衔接。
func truncateAssistantHistoryContent(content string) string {
	runes := []rune(content)
	if len(runes) <= assistantHistoryHeadChars+assistantHistoryTailChars {
		return content
	}
	head := string(runes[:assistantHistoryHeadChars])
	tail := string(runes[len(runes)-assistantHistoryTailChars:])
	return head + "\n\n...(中间数据细节已省略；引用其中的任何数据前必须重新调用工具核实)..." + tail
}

func (s *ChatMemoryService) GetHistoryMessages() ([]*schema.Message, error) {
	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxMemory)
	if err != nil {
		logger.SugaredLogger.Errorf("GetChatMemoryList failed: %v", err)
		return nil, err
	}

	var messages []*schema.Message
	for _, m := range memories {
		if m.Role == "user" {
			messages = append(messages, &schema.Message{
				Role:    schema.User,
				Content: m.Content,
			})
		} else if m.Role == "assistant" {
			messages = append(messages, &schema.Message{
				Role:    schema.Assistant,
				Content: truncateAssistantHistoryContent(m.Content),
			})
		}
	}
	return messages, nil
}

func (s *ChatMemoryService) GetHistory() ([]string, error) {
	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxMemory)
	if err != nil {
		logger.SugaredLogger.Errorf("GetChatMemoryList failed: %v", err)
		return nil, err
	}

	var history []string
	for _, m := range memories {
		role := m.Role
		if role == "user" {
			history = append(history, "用户: "+m.Content)
		} else if role == "assistant" {
			history = append(history, "助手: "+m.Content)
		}
	}
	return history, nil
}

func (s *ChatMemoryService) GetFormattedHistory() (string, error) {
	history, err := s.GetHistory()
	if err != nil {
		return "", err
	}
	if len(history) == 0 {
		return "", nil
	}
	result := "【对话历史】\n"
	for _, h := range history {
		result += h + "\n"
	}
	return result, nil
}

func (s *ChatMemoryService) Clear() error {
	return db.ClearChatMemory(s.sessionID)
}
