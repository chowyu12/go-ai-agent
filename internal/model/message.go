package model

import (
	"encoding/json"
	"time"
)

type Conversation struct {
	ID        int64     `json:"id"`
	UUID      string    `json:"uuid"`
	AgentID   int64     `json:"agent_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             int64           `json:"id"`
	ConversationID int64           `json:"conversation_id"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	ToolCalls      json.RawMessage `json:"tool_calls,omitzero"`
	ToolCallID     string          `json:"tool_call_id,omitzero"`
	TokensUsed     int             `json:"tokens_used"`
	ParentStepID   int64           `json:"parent_step_id,omitzero"`
	CreatedAt      time.Time       `json:"created_at"`

	Steps []ExecutionStep `json:"steps,omitzero"`
}

type StepType string

const (
	StepLLMCall    StepType = "llm_call"
	StepToolCall   StepType = "tool_call"
	StepAgentCall  StepType = "agent_call"
	StepSkillMatch StepType = "skill_match"
)

type StepStatus string

const (
	StepSuccess StepStatus = "success"
	StepError   StepStatus = "error"
	StepPending StepStatus = "pending"
)

type ExecutionStep struct {
	ID             int64           `json:"id"`
	MessageID      int64           `json:"message_id"`
	ConversationID int64           `json:"conversation_id"`
	StepOrder      int             `json:"step_order"`
	StepType       StepType        `json:"step_type"`
	Name           string          `json:"name"`
	Input          string          `json:"input"`
	Output         string          `json:"output"`
	Status         StepStatus      `json:"status"`
	Error          string          `json:"error,omitzero"`
	DurationMs     int             `json:"duration_ms"`
	TokensUsed     int             `json:"tokens_used"`
	Metadata       json.RawMessage `json:"metadata,omitzero"`
	CreatedAt      time.Time       `json:"created_at"`
}

type StepMetadata struct {
	Provider    string   `json:"provider,omitzero"`
	Model       string   `json:"model,omitzero"`
	Temperature float64  `json:"temperature,omitzero"`
	ToolName    string   `json:"tool_name,omitzero"`
	SkillName   string   `json:"skill_name,omitzero"`
	SkillTools  []string `json:"skill_tools,omitzero"`
	AgentUUID   string   `json:"agent_uuid,omitzero"`
	AgentName   string   `json:"agent_name,omitzero"`
}

type ChatRequest struct {
	AgentID        string `json:"agent_id"`
	ConversationID string `json:"conversation_id,omitzero"`
	UserID         string `json:"user_id"`
	Message        string `json:"message"`
	Stream         bool   `json:"stream"`
}

type ChatResponse struct {
	ConversationID string          `json:"conversation_id"`
	Message        string          `json:"message"`
	TokensUsed     int             `json:"tokens_used"`
	Steps          []ExecutionStep `json:"steps,omitzero"`
}

type StreamChunk struct {
	ConversationID string          `json:"conversation_id,omitzero"`
	Delta          string          `json:"delta,omitzero"`
	Done           bool            `json:"done"`
	Step           *ExecutionStep  `json:"step,omitzero"`
	Steps          []ExecutionStep `json:"steps,omitzero"`
}

type ListQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword,omitzero"`
}
