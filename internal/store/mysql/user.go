package mysql

import (
	"context"
	"database/sql"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (s *MySQLStore) CreateUser(ctx context.Context, u *model.User) error {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password, role, enabled) VALUES (?, ?, ?, ?)`,
		u.Username, u.Password, u.Role, u.Enabled,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	u.ID = id
	return nil
}

func (s *MySQLStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password, role, enabled, created_at, updated_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (s *MySQLStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password, role, enabled, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (s *MySQLStore) ListUsers(ctx context.Context, q model.ListQuery) ([]*model.User, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM users`
	if q.Keyword != "" {
		countQuery += ` WHERE username LIKE ?`
		s.db.QueryRowContext(ctx, countQuery, "%"+q.Keyword+"%").Scan(&total)
	} else {
		s.db.QueryRowContext(ctx, countQuery).Scan(&total)
	}

	query := `SELECT id, username, password, role, enabled, created_at, updated_at FROM users`
	var args []any
	if q.Keyword != "" {
		query += ` WHERE username LIKE ?`
		args = append(args, "%"+q.Keyword+"%")
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		u.Password = ""
		list = append(list, &u)
	}
	return list, total, nil
}

func (s *MySQLStore) UpdateUser(ctx context.Context, id int64, req model.UpdateUserReq) error {
	sets, args := buildUserUpdateSets(req)
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	query := "UPDATE users SET " + joinSets(sets) + " WHERE id = ?"
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func buildUserUpdateSets(req model.UpdateUserReq) ([]string, []any) {
	var sets []string
	var args []any
	if req.Password != nil {
		sets = append(sets, "password = ?")
		args = append(args, *req.Password)
	}
	if req.Role != nil {
		sets = append(sets, "role = ?")
		args = append(args, *req.Role)
	}
	if req.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, *req.Enabled)
	}
	return sets, args
}

func joinSets(sets []string) string {
	result := sets[0]
	for _, s := range sets[1:] {
		result += ", " + s
	}
	return result
}

func (s *MySQLStore) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *MySQLStore) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count > 0, err
}
