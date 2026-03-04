package agent

import (
	"context"

	"github.com/tmc/langchaingo/llms"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
)

type MemoryManager struct {
	store store.ConversationStore
}

func NewMemoryManager(s store.ConversationStore) *MemoryManager {
	return &MemoryManager{store: s}
}

func (m *MemoryManager) GetOrCreateConversation(ctx context.Context, conversationUUID string, agentID int64, userID string) (*model.Conversation, error) {
	if conversationUUID != "" {
		conv, err := m.store.GetConversationByUUID(ctx, conversationUUID)
		if err == nil {
			return conv, nil
		}
	}
	conv := &model.Conversation{
		AgentID: agentID,
		UserID:  userID,
		Title:   "New Conversation",
	}
	if err := m.store.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (m *MemoryManager) LoadHistory(ctx context.Context, conversationID int64, limit int) ([]llms.MessageContent, error) {
	msgs, err := m.store.ListMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}

	var result []llms.MessageContent
	for _, msg := range msgs {
		role := llms.ChatMessageTypeHuman
		switch msg.Role {
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "system":
			role = llms.ChatMessageTypeSystem
		case "tool":
			role = llms.ChatMessageTypeTool
		}
		result = append(result, llms.MessageContent{
			Role:  role,
			Parts: []llms.ContentPart{llms.TextContent{Text: msg.Content}},
		})
	}
	return result, nil
}

func (m *MemoryManager) SaveMessage(ctx context.Context, conversationID int64, role, content string, tokensUsed int) error {
	msg := &model.Message{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokensUsed:     tokensUsed,
	}
	return m.store.CreateMessage(ctx, msg)
}
