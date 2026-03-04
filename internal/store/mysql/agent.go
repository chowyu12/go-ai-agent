package mysql

import (
	"context"
	"database/sql"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateAgent(ctx context.Context, a *model.Agent) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (uuid, name, description, system_prompt, provider_id, model_name, temperature, max_tokens, timeout) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.UUID, a.Name, a.Description, a.SystemPrompt, a.ProviderID, a.ModelName, a.Temperature, a.MaxTokens, a.Timeout,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	a.ID = id
	return nil
}

func (s *MySQLStore) GetAgent(ctx context.Context, id int64) (*model.Agent, error) {
	var a model.Agent
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, name, description, system_prompt, provider_id, model_name, temperature, max_tokens, timeout, created_at, updated_at FROM agents WHERE id = ?`, id,
	).Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &a.SystemPrompt, &a.ProviderID, &a.ModelName, &a.Temperature, &a.MaxTokens, &a.Timeout, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *MySQLStore) GetAgentByUUID(ctx context.Context, uid string) (*model.Agent, error) {
	var a model.Agent
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, name, description, system_prompt, provider_id, model_name, temperature, max_tokens, timeout, created_at, updated_at FROM agents WHERE uuid = ?`, uid,
	).Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &a.SystemPrompt, &a.ProviderID, &a.ModelName, &a.Temperature, &a.MaxTokens, &a.Timeout, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *MySQLStore) ListAgents(ctx context.Context, q model.ListQuery) ([]*model.Agent, int64, error) {
	var total int64
	args := []any{}
	where := ""
	if q.Keyword != "" {
		where = ` WHERE name LIKE ?`
		args = append(args, "%"+q.Keyword+"%")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agents`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, name, description, system_prompt, provider_id, model_name, temperature, max_tokens, timeout, created_at, updated_at FROM agents`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Agent
	for rows.Next() {
		var a model.Agent
		if err := rows.Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &a.SystemPrompt, &a.ProviderID, &a.ModelName, &a.Temperature, &a.MaxTokens, &a.Timeout, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &a)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) UpdateAgent(ctx context.Context, id int64, req model.UpdateAgentReq) error {
	a, err := s.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.SystemPrompt != nil {
		a.SystemPrompt = *req.SystemPrompt
	}
	if req.ProviderID != nil {
		a.ProviderID = *req.ProviderID
	}
	if req.ModelName != nil {
		a.ModelName = *req.ModelName
	}
	if req.Temperature != nil {
		a.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		a.MaxTokens = *req.MaxTokens
	}
	if req.Timeout != nil {
		a.Timeout = *req.Timeout
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE agents SET name=?, description=?, system_prompt=?, provider_id=?, model_name=?, temperature=?, max_tokens=?, timeout=? WHERE id=?`,
		a.Name, a.Description, a.SystemPrompt, a.ProviderID, a.ModelName, a.Temperature, a.MaxTokens, a.Timeout, id,
	)
	return err
}

func (s *MySQLStore) DeleteAgent(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, tbl := range []string{"agent_tools", "agent_skills", "agent_children"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+tbl+` WHERE agent_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_children WHERE child_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) SetAgentTools(ctx context.Context, agentID int64, toolIDs []int64) error {
	return s.setRelation(ctx, "agent_tools", "agent_id", "tool_id", agentID, toolIDs)
}

func (s *MySQLStore) GetAgentTools(ctx context.Context, agentID int64) ([]model.Tool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.uuid, t.name, t.description, t.function_def, t.handler_type, t.handler_config, t.enabled, t.timeout, t.created_at, t.updated_at
		 FROM tools t INNER JOIN agent_tools at2 ON t.id = at2.tool_id WHERE at2.agent_id = ?`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTools(rows)
}

func (s *MySQLStore) SetAgentSkills(ctx context.Context, agentID int64, skillIDs []int64) error {
	return s.setRelation(ctx, "agent_skills", "agent_id", "skill_id", agentID, skillIDs)
}

func (s *MySQLStore) GetAgentSkills(ctx context.Context, agentID int64) ([]model.Skill, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.uuid, s.name, s.description, s.instruction, s.created_at, s.updated_at
		 FROM skills s INNER JOIN agent_skills as2 ON s.id = as2.skill_id WHERE as2.agent_id = ?`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSkills(rows)
}

func (s *MySQLStore) SetAgentChildren(ctx context.Context, agentID int64, childIDs []int64) error {
	return s.setRelation(ctx, "agent_children", "parent_id", "child_id", agentID, childIDs)
}

func (s *MySQLStore) GetAgentChildren(ctx context.Context, agentID int64) ([]model.Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.uuid, a.name, a.description, a.system_prompt, a.provider_id, a.model_name, a.temperature, a.max_tokens, a.timeout, a.created_at, a.updated_at
		 FROM agents a INNER JOIN agent_children ac ON a.id = ac.child_id WHERE ac.parent_id = ?`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Agent
	for rows.Next() {
		var a model.Agent
		if err := rows.Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &a.SystemPrompt, &a.ProviderID, &a.ModelName, &a.Temperature, &a.MaxTokens, &a.Timeout, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *MySQLStore) setRelation(ctx context.Context, table, col1, col2 string, id int64, relIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+col1+` = ?`, id); err != nil {
		return err
	}
	for _, relID := range relIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+col1+`, `+col2+`) VALUES (?, ?)`, id, relID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanTools(rows *sql.Rows) ([]model.Tool, error) {
	var list []model.Tool
	for rows.Next() {
		var t model.Tool
		var funcDef, handlerCfg sql.NullString
		if err := rows.Scan(&t.ID, &t.UUID, &t.Name, &t.Description, &funcDef, &t.HandlerType, &handlerCfg, &t.Enabled, &t.Timeout, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if funcDef.Valid {
			t.FunctionDef = []byte(funcDef.String)
		}
		if handlerCfg.Valid {
			t.HandlerConfig = []byte(handlerCfg.String)
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func scanSkills(rows *sql.Rows) ([]model.Skill, error) {
	var list []model.Skill
	for rows.Next() {
		var sk model.Skill
		if err := rows.Scan(&sk.ID, &sk.UUID, &sk.Name, &sk.Description, &sk.Instruction, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, sk)
	}
	return list, rows.Err()
}
