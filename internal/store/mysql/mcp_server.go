package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateMCPServer(ctx context.Context, m *model.MCPServer) error {
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	args, _ := json.Marshal(m.Args)
	env, _ := json.Marshal(m.Env)
	headers, _ := json.Marshal(m.Headers)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_servers (uuid, name, description, transport, endpoint, args, env, headers, enabled) VALUES (?,?,?,?,?,?,?,?,?)`,
		m.UUID, m.Name, m.Description, m.Transport, m.Endpoint, args, env, headers, m.Enabled,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	m.ID = id
	return nil
}

func (s *MySQLStore) GetMCPServer(ctx context.Context, id int64) (*model.MCPServer, error) {
	var m model.MCPServer
	var args, env, headers sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, name, description, transport, endpoint, args, env, headers, enabled, created_at, updated_at FROM mcp_servers WHERE id = ?`, id,
	).Scan(&m.ID, &m.UUID, &m.Name, &m.Description, &m.Transport, &m.Endpoint, &args, &env, &headers, &m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if args.Valid {
		m.Args = json.RawMessage(args.String)
	}
	if env.Valid {
		m.Env = json.RawMessage(env.String)
	}
	if headers.Valid {
		m.Headers = json.RawMessage(headers.String)
	}
	return &m, nil
}

func (s *MySQLStore) ListMCPServers(ctx context.Context, q model.ListQuery) ([]*model.MCPServer, int64, error) {
	var total int64
	where := ""
	qArgs := []any{}
	if q.Keyword != "" {
		where = ` WHERE name LIKE ?`
		qArgs = append(qArgs, "%"+q.Keyword+"%")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM mcp_servers`+where, qArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, name, description, transport, endpoint, args, env, headers, enabled, created_at, updated_at FROM mcp_servers`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(qArgs, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.MCPServer
	for rows.Next() {
		var m model.MCPServer
		var args, env, headers sql.NullString
		if err := rows.Scan(&m.ID, &m.UUID, &m.Name, &m.Description, &m.Transport, &m.Endpoint, &args, &env, &headers, &m.Enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if args.Valid {
			m.Args = json.RawMessage(args.String)
		}
		if env.Valid {
			m.Env = json.RawMessage(env.String)
		}
		if headers.Valid {
			m.Headers = json.RawMessage(headers.String)
		}
		list = append(list, &m)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) UpdateMCPServer(ctx context.Context, id int64, req model.UpdateMCPServerReq) error {
	m, err := s.GetMCPServer(ctx, id)
	if err != nil {
		return err
	}
	if req.Name != nil {
		m.Name = *req.Name
	}
	if req.Description != nil {
		m.Description = *req.Description
	}
	if req.Transport != nil {
		m.Transport = *req.Transport
	}
	if req.Endpoint != nil {
		m.Endpoint = *req.Endpoint
	}
	if req.Args != nil {
		m.Args = req.Args
	}
	if req.Env != nil {
		m.Env = req.Env
	}
	if req.Headers != nil {
		m.Headers = req.Headers
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	args, _ := json.Marshal(m.Args)
	env, _ := json.Marshal(m.Env)
	headers, _ := json.Marshal(m.Headers)
	_, err = s.db.ExecContext(ctx,
		`UPDATE mcp_servers SET name=?, description=?, transport=?, endpoint=?, args=?, env=?, headers=?, enabled=? WHERE id=?`,
		m.Name, m.Description, m.Transport, m.Endpoint, args, env, headers, m.Enabled, id,
	)
	return err
}

func (s *MySQLStore) DeleteMCPServer(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_mcp_servers WHERE mcp_server_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) SetAgentMCPServers(ctx context.Context, agentID int64, mcpServerIDs []int64) error {
	return s.setRelation(ctx, "agent_mcp_servers", "agent_id", "mcp_server_id", agentID, mcpServerIDs)
}

func (s *MySQLStore) GetAgentMCPServers(ctx context.Context, agentID int64) ([]model.MCPServer, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.uuid, m.name, m.description, m.transport, m.endpoint, m.args, m.env, m.headers, m.enabled, m.created_at, m.updated_at
		 FROM mcp_servers m INNER JOIN agent_mcp_servers am ON m.id = am.mcp_server_id WHERE am.agent_id = ? AND m.enabled = 1`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.MCPServer
	for rows.Next() {
		var m model.MCPServer
		var args, env, headers sql.NullString
		if err := rows.Scan(&m.ID, &m.UUID, &m.Name, &m.Description, &m.Transport, &m.Endpoint, &args, &env, &headers, &m.Enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if args.Valid {
			m.Args = json.RawMessage(args.String)
		}
		if env.Valid {
			m.Env = json.RawMessage(env.String)
		}
		if headers.Valid {
			m.Headers = json.RawMessage(headers.String)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}
