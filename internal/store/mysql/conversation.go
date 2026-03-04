package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateConversation(ctx context.Context, c *model.Conversation) error {
	if c.UUID == "" {
		c.UUID = uuid.New().String()
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO conversations (uuid, agent_id, user_id, title) VALUES (?,?,?,?)`,
		c.UUID, c.AgentID, c.UserID, c.Title,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	c.ID = id
	return nil
}

func (s *MySQLStore) GetConversation(ctx context.Context, id int64) (*model.Conversation, error) {
	var c model.Conversation
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, agent_id, user_id, title, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&c.ID, &c.UUID, &c.AgentID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *MySQLStore) GetConversationByUUID(ctx context.Context, uid string) (*model.Conversation, error) {
	var c model.Conversation
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, agent_id, user_id, title, created_at, updated_at FROM conversations WHERE uuid = ?`, uid,
	).Scan(&c.ID, &c.UUID, &c.AgentID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *MySQLStore) ListConversations(ctx context.Context, agentID int64, userID string, q model.ListQuery) ([]*model.Conversation, int64, error) {
	var total int64
	where := ` WHERE 1=1`
	args := []any{}
	if agentID > 0 {
		where += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if userID != "" {
		where += ` AND user_id = ?`
		args = append(args, userID)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM conversations`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, agent_id, user_id, title, created_at, updated_at FROM conversations`+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Conversation
	for rows.Next() {
		var c model.Conversation
		if err := rows.Scan(&c.ID, &c.UUID, &c.AgentID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &c)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) DeleteConversation(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM execution_steps WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) CreateMessage(ctx context.Context, m *model.Message) error {
	toolCalls, _ := json.Marshal(m.ToolCalls)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, tool_calls, tool_call_id, tokens_used, parent_step_id) VALUES (?,?,?,?,?,?,?)`,
		m.ConversationID, m.Role, m.Content, toolCalls, m.ToolCallID, m.TokensUsed, m.ParentStepID,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	m.ID = id
	return nil
}

func (s *MySQLStore) ListMessages(ctx context.Context, conversationID int64, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tokens_used, parent_step_id, created_at FROM messages WHERE conversation_id = ? ORDER BY id ASC LIMIT ?`,
		conversationID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Message
	for rows.Next() {
		var m model.Message
		var toolCalls sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &toolCalls, &m.ToolCallID, &m.TokensUsed, &m.ParentStepID, &m.CreatedAt); err != nil {
			return nil, err
		}
		if toolCalls.Valid {
			m.ToolCalls = json.RawMessage(toolCalls.String)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MySQLStore) CreateExecutionStep(ctx context.Context, step *model.ExecutionStep) error {
	meta, _ := json.Marshal(step.Metadata)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO execution_steps (message_id, conversation_id, step_order, step_type, name, input, output, status, error, duration_ms, tokens_used, metadata) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		step.MessageID, step.ConversationID, step.StepOrder, step.StepType, step.Name,
		step.Input, step.Output, step.Status, step.Error,
		step.DurationMs, step.TokensUsed, meta,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	step.ID = id
	return nil
}

func (s *MySQLStore) ListExecutionSteps(ctx context.Context, messageID int64) ([]model.ExecutionStep, error) {
	return s.querySteps(ctx,
		`SELECT id, message_id, conversation_id, step_order, step_type, name, input, output, status, error, duration_ms, tokens_used, metadata, created_at FROM execution_steps WHERE message_id = ? ORDER BY step_order ASC`,
		messageID,
	)
}

func (s *MySQLStore) ListExecutionStepsByConversation(ctx context.Context, conversationID int64) ([]model.ExecutionStep, error) {
	return s.querySteps(ctx,
		`SELECT id, message_id, conversation_id, step_order, step_type, name, input, output, status, error, duration_ms, tokens_used, metadata, created_at FROM execution_steps WHERE conversation_id = ? ORDER BY id ASC`,
		conversationID,
	)
}

func (s *MySQLStore) querySteps(ctx context.Context, query string, args ...any) ([]model.ExecutionStep, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.ExecutionStep
	for rows.Next() {
		var step model.ExecutionStep
		var errStr sql.NullString
		var meta sql.NullString
		if err := rows.Scan(
			&step.ID, &step.MessageID, &step.ConversationID, &step.StepOrder,
			&step.StepType, &step.Name, &step.Input, &step.Output,
			&step.Status, &errStr, &step.DurationMs, &step.TokensUsed,
			&meta, &step.CreatedAt,
		); err != nil {
			return nil, err
		}
		if errStr.Valid {
			step.Error = errStr.String
		}
		if meta.Valid {
			step.Metadata = json.RawMessage(meta.String)
		}
		list = append(list, step)
	}
	return list, rows.Err()
}
