package gormstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "ag-" + hex.EncodeToString(b)
}

func (s *GormStore) CreateAgent(ctx context.Context, a *model.Agent) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	if a.Token == "" {
		a.Token = generateToken()
	}
	return s.db.WithContext(ctx).Create(a).Error
}

func (s *GormStore) GetAgent(ctx context.Context, id int64) (*model.Agent, error) {
	var a model.Agent
	if err := s.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (s *GormStore) GetAgentByUUID(ctx context.Context, uid string) (*model.Agent, error) {
	var a model.Agent
	if err := s.db.WithContext(ctx).Where("uuid = ?", uid).First(&a).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (s *GormStore) GetAgentByToken(ctx context.Context, token string) (*model.Agent, error) {
	var a model.Agent
	if err := s.db.WithContext(ctx).Where("token = ? AND token != ''", token).First(&a).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (s *GormStore) ListAgents(ctx context.Context, q model.ListQuery) ([]*model.Agent, int64, error) {
	var items []*model.Agent
	var total int64

	db := s.db.WithContext(ctx).Model(&model.Agent{})
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

func (s *GormStore) UpdateAgent(ctx context.Context, id int64, req model.UpdateAgentReq) error {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if req.ProviderID != nil {
		updates["provider_id"] = *req.ProviderID
	}
	if req.ModelName != nil {
		updates["model_name"] = *req.ModelName
	}
	if req.Temperature != nil {
		updates["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		updates["max_tokens"] = *req.MaxTokens
	}
	if req.Timeout != nil {
		updates["timeout"] = *req.Timeout
	}
	if req.MaxHistory != nil {
		updates["max_history"] = *req.MaxHistory
	}
	if req.MaxIterations != nil {
		updates["max_iterations"] = *req.MaxIterations
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GormStore) UpdateAgentToken(ctx context.Context, id int64, token string) error {
	return s.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ?", id).Update("token", token).Error
}

func (s *GormStore) DeleteAgent(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Where("agent_id = ?", id).Delete(&model.AgentTool{})
		tx.Where("agent_id = ?", id).Delete(&model.AgentSkill{})
		tx.Where("agent_id = ?", id).Delete(&model.AgentMCPServer{})

		var convIDs []int64
		tx.Model(&model.Conversation{}).Where("agent_id = ?", id).Pluck("id", &convIDs)
		if len(convIDs) > 0 {
			tx.Where("conversation_id IN ?", convIDs).Delete(&model.File{})
			tx.Where("conversation_id IN ?", convIDs).Delete(&model.ExecutionStep{})
			tx.Where("conversation_id IN ?", convIDs).Delete(&model.Message{})
			tx.Where("agent_id = ?", id).Delete(&model.Conversation{})
		}

		return tx.Delete(&model.Agent{}, id).Error
	})
}

func (s *GormStore) SetAgentTools(ctx context.Context, agentID int64, toolIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return setRelation(tx, "agent_tools", "agent_id", "tool_id", agentID, toolIDs)
	})
}

func (s *GormStore) GetAgentTools(ctx context.Context, agentID int64) ([]model.Tool, error) {
	var tools []model.Tool
	err := s.db.WithContext(ctx).
		Joins("INNER JOIN agent_tools ON tools.id = agent_tools.tool_id").
		Where("agent_tools.agent_id = ?", agentID).
		Find(&tools).Error
	return tools, err
}

func (s *GormStore) SetAgentSkills(ctx context.Context, agentID int64, skillIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return setRelation(tx, "agent_skills", "agent_id", "skill_id", agentID, skillIDs)
	})
}

func (s *GormStore) GetAgentSkills(ctx context.Context, agentID int64) ([]model.Skill, error) {
	var skills []model.Skill
	err := s.db.WithContext(ctx).
		Joins("INNER JOIN agent_skills ON skills.id = agent_skills.skill_id").
		Where("agent_skills.agent_id = ?", agentID).
		Find(&skills).Error
	return skills, err
}

func (s *GormStore) SetAgentMCPServers(ctx context.Context, agentID int64, mcpServerIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return setRelation(tx, "agent_mcp_servers", "agent_id", "mcp_server_id", agentID, mcpServerIDs)
	})
}

func (s *GormStore) GetAgentMCPServers(ctx context.Context, agentID int64) ([]model.MCPServer, error) {
	var servers []model.MCPServer
	err := s.db.WithContext(ctx).
		Joins("INNER JOIN agent_mcp_servers ON mcp_servers.id = agent_mcp_servers.mcp_server_id").
		Where("agent_mcp_servers.agent_id = ? AND mcp_servers.enabled = ?", agentID, true).
		Find(&servers).Error
	return servers, err
}
