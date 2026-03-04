package model

import "time"

type Skill struct {
	ID          int64     `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Instruction string    `json:"instruction"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Tools []Tool `json:"tools,omitzero"`
}

type CreateSkillReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Instruction string  `json:"instruction"`
	ToolIDs     []int64 `json:"tool_ids,omitzero"`
}

type UpdateSkillReq struct {
	Name        *string `json:"name,omitzero"`
	Description *string `json:"description,omitzero"`
	Instruction *string `json:"instruction,omitzero"`
	ToolIDs     []int64 `json:"tool_ids,omitzero"`
}
