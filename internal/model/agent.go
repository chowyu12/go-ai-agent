package model

import "time"

type Agent struct {
	ID           int64     `json:"id"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt"`
	ProviderID   int64     `json:"provider_id"`
	ModelName    string    `json:"model_name"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"max_tokens"`
	Timeout      int       `json:"timeout"`
	MaxHistory   int       `json:"max_history"`
	MaxIterations int      `json:"max_iterations"`
	Token        string    `json:"token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Tools    []Tool  `json:"tools,omitzero"`
	Skills   []Skill `json:"skills,omitzero"`
	Children []Agent `json:"children,omitzero"`
}

const (
	DefaultAgentTimeout       = 120
	DefaultAgentMaxHistory    = 50
	DefaultAgentMaxIterations = 10
)

func (a *Agent) TimeoutSeconds() int {
	if a.Timeout > 0 {
		return a.Timeout
	}
	return DefaultAgentTimeout
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
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	SystemPrompt string  `json:"system_prompt"`
	ProviderID   int64   `json:"provider_id"`
	ModelName    string  `json:"model_name"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	Timeout      int     `json:"timeout"`
	MaxHistory    int     `json:"max_history"`
	MaxIterations int     `json:"max_iterations"`
	ToolIDs       []int64 `json:"tool_ids,omitzero"`
	SkillIDs      []int64 `json:"skill_ids,omitzero"`
	ChildIDs      []int64 `json:"child_ids,omitzero"`
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
	ChildIDs      []int64  `json:"child_ids,omitzero"`
}
