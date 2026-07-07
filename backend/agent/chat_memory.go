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
				Content: m.Content,
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
