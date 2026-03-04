package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (s *MySQLStore) CreateProvider(ctx context.Context, p *model.Provider) error {
	models, _ := json.Marshal(p.Models)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO providers (name, type, base_url, api_key, models, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Name, p.Type, p.BaseURL, p.APIKey, models, p.Enabled,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	p.ID = id
	return nil
}

func (s *MySQLStore) GetProvider(ctx context.Context, id int64) (*model.Provider, error) {
	var p model.Provider
	var modelsRaw sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, base_url, api_key, models, enabled, created_at, updated_at FROM providers WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &modelsRaw, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if modelsRaw.Valid {
		p.Models = json.RawMessage(modelsRaw.String)
	}
	return &p, nil
}

func (s *MySQLStore) ListProviders(ctx context.Context, q model.ListQuery) ([]*model.Provider, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(1) FROM providers`
	args := []any{}
	where := ""
	if q.Keyword != "" {
		where = ` WHERE name LIKE ?`
		args = append(args, "%"+q.Keyword+"%")
	}
	if err := s.db.QueryRowContext(ctx, countQuery+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, base_url, api_key, models, enabled, created_at, updated_at FROM providers`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Provider
	for rows.Next() {
		var p model.Provider
		var modelsRaw sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &modelsRaw, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if modelsRaw.Valid {
			p.Models = json.RawMessage(modelsRaw.String)
		}
		list = append(list, &p)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) UpdateProvider(ctx context.Context, id int64, req model.UpdateProviderReq) error {
	p, err := s.GetProvider(ctx, id)
	if err != nil {
		return err
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Type != nil {
		p.Type = *req.Type
	}
	if req.BaseURL != nil {
		p.BaseURL = *req.BaseURL
	}
	if req.APIKey != nil {
		p.APIKey = *req.APIKey
	}
	if req.Models != nil {
		p.Models = req.Models
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	models, _ := json.Marshal(p.Models)
	_, err = s.db.ExecContext(ctx,
		`UPDATE providers SET name=?, type=?, base_url=?, api_key=?, models=?, enabled=? WHERE id=?`,
		p.Name, p.Type, p.BaseURL, p.APIKey, models, p.Enabled, id,
	)
	return err
}

func (s *MySQLStore) DeleteProvider(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id)
	return err
}

func paginate(q model.ListQuery) (offset, limit int) {
	limit = q.PageSize
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	offset = (page - 1) * limit
	return
}
