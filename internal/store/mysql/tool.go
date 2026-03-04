package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateTool(ctx context.Context, t *model.Tool) error {
	if t.UUID == "" {
		t.UUID = uuid.New().String()
	}
	funcDef, _ := json.Marshal(t.FunctionDef)
	handlerCfg, _ := json.Marshal(t.HandlerConfig)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO tools (uuid, name, description, function_def, handler_type, handler_config, enabled, timeout) VALUES (?,?,?,?,?,?,?,?)`,
		t.UUID, t.Name, t.Description, funcDef, t.HandlerType, handlerCfg, t.Enabled, t.Timeout,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

func (s *MySQLStore) GetTool(ctx context.Context, id int64) (*model.Tool, error) {
	var t model.Tool
	var funcDef, handlerCfg sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, name, description, function_def, handler_type, handler_config, enabled, timeout, created_at, updated_at FROM tools WHERE id = ?`, id,
	).Scan(&t.ID, &t.UUID, &t.Name, &t.Description, &funcDef, &t.HandlerType, &handlerCfg, &t.Enabled, &t.Timeout, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if funcDef.Valid {
		t.FunctionDef = []byte(funcDef.String)
	}
	if handlerCfg.Valid {
		t.HandlerConfig = []byte(handlerCfg.String)
	}
	return &t, nil
}

func (s *MySQLStore) ListTools(ctx context.Context, q model.ListQuery) ([]*model.Tool, int64, error) {
	var total int64
	args := []any{}
	where := ""
	if q.Keyword != "" {
		where = ` WHERE name LIKE ?`
		args = append(args, "%"+q.Keyword+"%")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM tools`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, name, description, function_def, handler_type, handler_config, enabled, timeout, created_at, updated_at FROM tools`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Tool
	for rows.Next() {
		var t model.Tool
		var funcDef, handlerCfg sql.NullString
		if err := rows.Scan(&t.ID, &t.UUID, &t.Name, &t.Description, &funcDef, &t.HandlerType, &handlerCfg, &t.Enabled, &t.Timeout, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if funcDef.Valid {
			t.FunctionDef = []byte(funcDef.String)
		}
		if handlerCfg.Valid {
			t.HandlerConfig = []byte(handlerCfg.String)
		}
		list = append(list, &t)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) UpdateTool(ctx context.Context, id int64, req model.UpdateToolReq) error {
	t, err := s.GetTool(ctx, id)
	if err != nil {
		return err
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.FunctionDef != nil {
		t.FunctionDef = req.FunctionDef
	}
	if req.HandlerType != nil {
		t.HandlerType = *req.HandlerType
	}
	if req.HandlerConfig != nil {
		t.HandlerConfig = req.HandlerConfig
	}
	if req.Enabled != nil {
		t.Enabled = *req.Enabled
	}
	if req.Timeout != nil {
		t.Timeout = *req.Timeout
	}
	funcDef, _ := json.Marshal(t.FunctionDef)
	handlerCfg, _ := json.Marshal(t.HandlerConfig)
	_, err = s.db.ExecContext(ctx,
		`UPDATE tools SET name=?, description=?, function_def=?, handler_type=?, handler_config=?, enabled=?, timeout=? WHERE id=?`,
		t.Name, t.Description, funcDef, t.HandlerType, handlerCfg, t.Enabled, t.Timeout, id,
	)
	return err
}

func (s *MySQLStore) DeleteTool(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_tools WHERE tool_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM skill_tools WHERE tool_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tools WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
