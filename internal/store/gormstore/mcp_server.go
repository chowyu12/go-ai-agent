package gormstore

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (s *GormStore) CreateMCPServer(ctx context.Context, m *model.MCPServer) error {
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	return s.db.WithContext(ctx).Create(m).Error
}

func (s *GormStore) GetMCPServer(ctx context.Context, id int64) (*model.MCPServer, error) {
	var m model.MCPServer
	if err := s.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (s *GormStore) ListMCPServers(ctx context.Context, q model.ListQuery) ([]*model.MCPServer, int64, error) {
	var items []*model.MCPServer
	var total int64

	db := s.db.WithContext(ctx).Model(&model.MCPServer{})
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

func (s *GormStore) UpdateMCPServer(ctx context.Context, id int64, req model.UpdateMCPServerReq) error {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Transport != nil {
		updates["transport"] = *req.Transport
	}
	if req.Endpoint != nil {
		updates["endpoint"] = *req.Endpoint
	}
	if req.Args != nil {
		updates["args"] = req.Args
	}
	if req.Env != nil {
		updates["env"] = req.Env
	}
	if req.Headers != nil {
		updates["headers"] = req.Headers
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.MCPServer{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GormStore) DeleteMCPServer(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Where("mcp_server_id = ?", id).Delete(&model.AgentMCPServer{})
		return tx.Delete(&model.MCPServer{}, id).Error
	})
}
