package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
)

type MemoryManager struct {
	store store.ConversationStore
	files store.FileStore
}

func NewMemoryManager(convStore store.ConversationStore, fileStore store.FileStore) *MemoryManager {
	return &MemoryManager{store: convStore, files: fileStore}
}

func (m *MemoryManager) GetOrCreateConversation(ctx context.Context, conversationUUID string, agentID int64, userID string) (*model.Conversation, error) {
	if conversationUUID != "" {
		conv, err := m.store.GetConversationByUUID(ctx, conversationUUID)
		if err == nil {
			if conv.UserID != "" && conv.UserID != userID {
				return nil, fmt.Errorf("conversation %s does not belong to user %s", conversationUUID, userID)
			}
			return conv, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get conversation: %w", err)
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

func (m *MemoryManager) LoadHistory(ctx context.Context, conversationID int64, limit int) ([]openai.ChatCompletionMessage, error) {
	msgs, err := m.store.ListMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}

	var result []openai.ChatCompletionMessage
	for _, msg := range msgs {
		role := openai.ChatMessageRoleUser
		switch msg.Role {
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "system":
			role = openai.ChatMessageRoleSystem
		case "tool":
			role = openai.ChatMessageRoleTool
		}
		cm := openai.ChatCompletionMessage{
			Role:       role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			_ = json.Unmarshal(msg.ToolCalls, &cm.ToolCalls)
		}
		result = append(result, cm)
	}
	return result, nil
}

func (m *MemoryManager) SaveUserMessage(ctx context.Context, conversationID int64, content string, files []*model.File) (int64, error) {
	msgID, err := m.saveMessage(ctx, conversationID, "user", content, 0)
	if err != nil {
		return 0, err
	}
	m.linkFiles(ctx, files, conversationID, msgID)
	return msgID, nil
}

func (m *MemoryManager) SaveAssistantMessage(ctx context.Context, conversationID int64, content string, tokensUsed int) (int64, error) {
	return m.saveMessage(ctx, conversationID, "assistant", content, tokensUsed)
}

func (m *MemoryManager) saveMessage(ctx context.Context, conversationID int64, role, content string, tokensUsed int) (int64, error) {
	msg := &model.Message{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokensUsed:     tokensUsed,
	}
	if err := m.store.CreateMessage(ctx, msg); err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (m *MemoryManager) linkFiles(ctx context.Context, files []*model.File, conversationID, messageID int64) {
	for _, f := range files {
		if f.ID == 0 {
			continue
		}
		if err := m.files.LinkFileToMessage(ctx, f.ID, conversationID, messageID); err != nil {
			log.WithFields(log.Fields{"file": f.Filename, "msg_id": messageID}).WithError(err).Warn("[Memory] link file to message failed")
		}
	}
}

func (m *MemoryManager) AutoSetTitle(ctx context.Context, conversationID int64, userMessage string) {
	title := userMessage
	rs := []rune(title)
	if len(rs) > 50 {
		title = string(rs[:50]) + "..."
	}
	if err := m.store.UpdateConversationTitle(ctx, conversationID, title); err != nil {
		log.WithFields(log.Fields{"conv_id": conversationID, "title": title}).WithError(err).Warn("[Memory] auto set title failed")
	}
}
