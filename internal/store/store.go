package store

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type Store interface {
	ProviderStore
	AgentStore
	ToolStore
	SkillStore
	ConversationStore
	UserStore
	FileStore
	Close() error
}

type ProviderStore interface {
	CreateProvider(ctx context.Context, p *model.Provider) error
	GetProvider(ctx context.Context, id int64) (*model.Provider, error)
	ListProviders(ctx context.Context, q model.ListQuery) ([]*model.Provider, int64, error)
	UpdateProvider(ctx context.Context, id int64, req model.UpdateProviderReq) error
	DeleteProvider(ctx context.Context, id int64) error
}

type AgentStore interface {
	CreateAgent(ctx context.Context, a *model.Agent) error
	GetAgent(ctx context.Context, id int64) (*model.Agent, error)
	GetAgentByUUID(ctx context.Context, uuid string) (*model.Agent, error)
	GetAgentByToken(ctx context.Context, token string) (*model.Agent, error)
	ListAgents(ctx context.Context, q model.ListQuery) ([]*model.Agent, int64, error)
	UpdateAgent(ctx context.Context, id int64, req model.UpdateAgentReq) error
	UpdateAgentToken(ctx context.Context, id int64, token string) error
	DeleteAgent(ctx context.Context, id int64) error

	SetAgentTools(ctx context.Context, agentID int64, toolIDs []int64) error
	GetAgentTools(ctx context.Context, agentID int64) ([]model.Tool, error)
	SetAgentSkills(ctx context.Context, agentID int64, skillIDs []int64) error
	GetAgentSkills(ctx context.Context, agentID int64) ([]model.Skill, error)
	SetAgentChildren(ctx context.Context, agentID int64, childIDs []int64) error
	GetAgentChildren(ctx context.Context, agentID int64) ([]model.Agent, error)
}

type ToolStore interface {
	CreateTool(ctx context.Context, t *model.Tool) error
	GetTool(ctx context.Context, id int64) (*model.Tool, error)
	ListTools(ctx context.Context, q model.ListQuery) ([]*model.Tool, int64, error)
	UpdateTool(ctx context.Context, id int64, req model.UpdateToolReq) error
	DeleteTool(ctx context.Context, id int64) error
}

type SkillStore interface {
	CreateSkill(ctx context.Context, s *model.Skill) error
	GetSkill(ctx context.Context, id int64) (*model.Skill, error)
	ListSkills(ctx context.Context, q model.ListQuery) ([]*model.Skill, int64, error)
	UpdateSkill(ctx context.Context, id int64, req model.UpdateSkillReq) error
	DeleteSkill(ctx context.Context, id int64) error

	SetSkillTools(ctx context.Context, skillID int64, toolIDs []int64) error
	GetSkillTools(ctx context.Context, skillID int64) ([]model.Tool, error)
}

type ConversationStore interface {
	CreateConversation(ctx context.Context, c *model.Conversation) error
	GetConversation(ctx context.Context, id int64) (*model.Conversation, error)
	GetConversationByUUID(ctx context.Context, uuid string) (*model.Conversation, error)
	ListConversations(ctx context.Context, agentID int64, userID string, q model.ListQuery) ([]*model.Conversation, int64, error)
	UpdateConversationTitle(ctx context.Context, id int64, title string) error
	DeleteConversation(ctx context.Context, id int64) error

	CreateMessage(ctx context.Context, m *model.Message) error
	ListMessages(ctx context.Context, conversationID int64, limit int) ([]model.Message, error)

	CreateExecutionStep(ctx context.Context, step *model.ExecutionStep) error
	UpdateStepsMessageID(ctx context.Context, conversationID, messageID int64) error
	ListExecutionSteps(ctx context.Context, messageID int64) ([]model.ExecutionStep, error)
	ListExecutionStepsByConversation(ctx context.Context, conversationID int64) ([]model.ExecutionStep, error)
}

type FileStore interface {
	CreateFile(ctx context.Context, f *model.File) error
	GetFileByUUID(ctx context.Context, uuid string) (*model.File, error)
	ListFilesByConversation(ctx context.Context, conversationID int64) ([]*model.File, error)
	ListFilesByMessage(ctx context.Context, messageID int64) ([]*model.File, error)
	UpdateFileMessageID(ctx context.Context, fileID, messageID int64) error
	DeleteFile(ctx context.Context, id int64) error
}

type UserStore interface {
	CreateUser(ctx context.Context, u *model.User) error
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUser(ctx context.Context, id int64) (*model.User, error)
	ListUsers(ctx context.Context, q model.ListQuery) ([]*model.User, int64, error)
	UpdateUser(ctx context.Context, id int64, req model.UpdateUserReq) error
	DeleteUser(ctx context.Context, id int64) error
	HasAdmin(ctx context.Context) (bool, error)
}
