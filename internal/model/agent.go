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
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Tools    []Tool  `json:"tools,omitzero"`
	Skills   []Skill `json:"skills,omitzero"`
	Children []Agent `json:"children,omitzero"`
}

type CreateAgentReq struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	SystemPrompt string  `json:"system_prompt"`
	ProviderID   int64   `json:"provider_id"`
	ModelName    string  `json:"model_name"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	ToolIDs      []int64 `json:"tool_ids,omitzero"`
	SkillIDs     []int64 `json:"skill_ids,omitzero"`
	ChildIDs     []int64 `json:"child_ids,omitzero"`
}

type UpdateAgentReq struct {
	Name         *string  `json:"name,omitzero"`
	Description  *string  `json:"description,omitzero"`
	SystemPrompt *string  `json:"system_prompt,omitzero"`
	ProviderID   *int64   `json:"provider_id,omitzero"`
	ModelName    *string  `json:"model_name,omitzero"`
	Temperature  *float64 `json:"temperature,omitzero"`
	MaxTokens    *int     `json:"max_tokens,omitzero"`
	ToolIDs      []int64  `json:"tool_ids,omitzero"`
	SkillIDs     []int64  `json:"skill_ids,omitzero"`
	ChildIDs     []int64  `json:"child_ids,omitzero"`
}
