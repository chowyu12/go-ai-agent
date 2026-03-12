package gormstore

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (s *GormStore) CreateSkill(ctx context.Context, sk *model.Skill) error {
	if sk.UUID == "" {
		sk.UUID = uuid.New().String()
	}
	return s.db.WithContext(ctx).Create(sk).Error
}

func (s *GormStore) GetSkill(ctx context.Context, id int64) (*model.Skill, error) {
	var sk model.Skill
	if err := s.db.WithContext(ctx).First(&sk, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &sk, nil
}

func (s *GormStore) GetSkillByDirName(ctx context.Context, dirName string) (*model.Skill, error) {
	var sk model.Skill
	if err := s.db.WithContext(ctx).Where("dir_name = ?", dirName).First(&sk).Error; err != nil {
		return nil, notFound(err)
	}
	return &sk, nil
}

func (s *GormStore) ListSkills(ctx context.Context, q model.ListQuery) ([]*model.Skill, int64, error) {
	var items []*model.Skill
	var total int64

	db := s.db.WithContext(ctx).Model(&model.Skill{})
	if q.Keyword != "" {
		db = db.Where("name LIKE ?", "%"+q.Keyword+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset, limit := paginate(q)
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *GormStore) UpdateSkill(ctx context.Context, id int64, req model.UpdateSkillReq) error {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Instruction != nil {
		updates["instruction"] = *req.Instruction
	}
	if req.Source != nil {
		updates["source"] = *req.Source
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.Version != nil {
		updates["version"] = *req.Version
	}
	if req.Author != nil {
		updates["author"] = *req.Author
	}
	if req.DirName != nil {
		updates["dir_name"] = *req.DirName
	}
	if req.MainFile != nil {
		updates["main_file"] = *req.MainFile
	}
	if req.Config != nil {
		updates["config"] = req.Config
	}
	if req.Permissions != nil {
		updates["permissions"] = req.Permissions
	}
	if req.ToolDefs != nil {
		updates["tool_defs"] = req.ToolDefs
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.Skill{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GormStore) DeleteSkill(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Where("skill_id = ?", id).Delete(&model.AgentSkill{})
		tx.Where("skill_id = ?", id).Delete(&model.SkillTool{})
		return tx.Delete(&model.Skill{}, id).Error
	})
}

func (s *GormStore) SetSkillTools(ctx context.Context, skillID int64, toolIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return setRelation(tx, "skill_tools", "skill_id", "tool_id", skillID, toolIDs)
	})
}

func (s *GormStore) GetSkillTools(ctx context.Context, skillID int64) ([]model.Tool, error) {
	var tools []model.Tool
	err := s.db.WithContext(ctx).
		Joins("INNER JOIN skill_tools ON tools.id = skill_tools.tool_id").
		Where("skill_tools.skill_id = ?", skillID).
		Find(&tools).Error
	return tools, err
}
