package mysql

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateSkill(ctx context.Context, sk *model.Skill) error {
	if sk.UUID == "" {
		sk.UUID = uuid.New().String()
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO skills (uuid, name, description, instruction) VALUES (?,?,?,?)`,
		sk.UUID, sk.Name, sk.Description, sk.Instruction,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	sk.ID = id
	return nil
}

func (s *MySQLStore) GetSkill(ctx context.Context, id int64) (*model.Skill, error) {
	var sk model.Skill
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, name, description, instruction, created_at, updated_at FROM skills WHERE id = ?`, id,
	).Scan(&sk.ID, &sk.UUID, &sk.Name, &sk.Description, &sk.Instruction, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *MySQLStore) ListSkills(ctx context.Context, q model.ListQuery) ([]*model.Skill, int64, error) {
	var total int64
	args := []any{}
	where := ""
	if q.Keyword != "" {
		where = ` WHERE name LIKE ?`
		args = append(args, "%"+q.Keyword+"%")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM skills`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset, limit := paginate(q)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, name, description, instruction, created_at, updated_at FROM skills`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Skill
	for rows.Next() {
		var sk model.Skill
		if err := rows.Scan(&sk.ID, &sk.UUID, &sk.Name, &sk.Description, &sk.Instruction, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &sk)
	}
	return list, total, rows.Err()
}

func (s *MySQLStore) UpdateSkill(ctx context.Context, id int64, req model.UpdateSkillReq) error {
	sk, err := s.GetSkill(ctx, id)
	if err != nil {
		return err
	}
	if req.Name != nil {
		sk.Name = *req.Name
	}
	if req.Description != nil {
		sk.Description = *req.Description
	}
	if req.Instruction != nil {
		sk.Instruction = *req.Instruction
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE skills SET name=?, description=?, instruction=? WHERE id=?`,
		sk.Name, sk.Description, sk.Instruction, id,
	)
	return err
}

func (s *MySQLStore) DeleteSkill(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_skills WHERE skill_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM skill_tools WHERE skill_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM skills WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) SetSkillTools(ctx context.Context, skillID int64, toolIDs []int64) error {
	return s.setRelation(ctx, "skill_tools", "skill_id", "tool_id", skillID, toolIDs)
}

func (s *MySQLStore) GetSkillTools(ctx context.Context, skillID int64) ([]model.Tool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.uuid, t.name, t.description, t.function_def, t.handler_type, t.handler_config, t.enabled, t.created_at, t.updated_at
		 FROM tools t INNER JOIN skill_tools st ON t.id = st.tool_id WHERE st.skill_id = ?`, skillID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTools(rows)
}
