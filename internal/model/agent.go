package model

import "time"

type Agent struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UUID          string    `json:"uuid" gorm:"uniqueIndex;size:36;not null"`
	Name          string    `json:"name" gorm:"size:200;not null"`
	Description   string    `json:"description" gorm:"type:text"`
	SystemPrompt  string    `json:"system_prompt" gorm:"type:text"`
	ProviderID    int64     `json:"provider_id" gorm:"index;not null"`
	ModelName     string    `json:"model_name" gorm:"size:100;not null"`
	Temperature   float64   `json:"temperature" gorm:"default:0.7"`
	MaxTokens     int       `json:"max_tokens" gorm:"default:4096"`
	Timeout       int       `json:"timeout" gorm:"default:120"`
	MaxHistory    int       `json:"max_history" gorm:"default:50"`
	MaxIterations int       `json:"max_iterations" gorm:"default:10"`
	Token         string    `json:"token" gorm:"uniqueIndex;size:64"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Tools      []Tool      `json:"tools,omitzero" gorm:"-"`
	Skills     []Skill     `json:"skills,omitzero" gorm:"-"`
	MCPServers []MCPServer `json:"mcp_servers,omitzero" gorm:"-"`
}

const (
	DefaultAgentMaxHistory    = 50
	DefaultAgentMaxIterations = 10
)

func (a *Agent) TimeoutSeconds() int {
	return a.Timeout
}

func (a *Agent) HistoryLimit() int {
	if a.MaxHistory > 0 {
		return a.MaxHistory
	}
	return DefaultAgentMaxHistory
}

func (a *Agent) IterationLimit() int {
	if a.MaxIterations > 0 {
		return a.MaxIterations
	}
	return DefaultAgentMaxIterations
}

type CreateAgentReq struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	SystemPrompt  string  `json:"system_prompt"`
	ProviderID    int64   `json:"provider_id"`
	ModelName     string  `json:"model_name"`
	Temperature   float64 `json:"temperature"`
	MaxTokens     int     `json:"max_tokens"`
	Timeout       int     `json:"timeout"`
	MaxHistory    int     `json:"max_history"`
	MaxIterations int     `json:"max_iterations"`
	ToolIDs       []int64 `json:"tool_ids,omitzero"`
	SkillIDs      []int64 `json:"skill_ids,omitzero"`
	MCPServerIDs  []int64 `json:"mcp_server_ids,omitzero"`
}

type UpdateAgentReq struct {
	Name          *string  `json:"name,omitzero"`
	Description   *string  `json:"description,omitzero"`
	SystemPrompt  *string  `json:"system_prompt,omitzero"`
	ProviderID    *int64   `json:"provider_id,omitzero"`
	ModelName     *string  `json:"model_name,omitzero"`
	Temperature   *float64 `json:"temperature,omitzero"`
	MaxTokens     *int     `json:"max_tokens,omitzero"`
	Timeout       *int     `json:"timeout,omitzero"`
	MaxHistory    *int     `json:"max_history,omitzero"`
	MaxIterations *int     `json:"max_iterations,omitzero"`
	ToolIDs       []int64  `json:"tool_ids,omitzero"`
	SkillIDs      []int64  `json:"skill_ids,omitzero"`
	MCPServerIDs  []int64  `json:"mcp_server_ids,omitzero"`
}
