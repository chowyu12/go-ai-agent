package model

import (
	"encoding/json"
	"time"
)

type HandlerType string

const (
	HandlerBuiltin HandlerType = "builtin"
	HandlerHTTP    HandlerType = "http"
	HandlerScript  HandlerType = "script"
	HandlerCommand HandlerType = "command"
)

type Tool struct {
	ID            int64           `json:"id"`
	UUID          string          `json:"uuid"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	FunctionDef   json.RawMessage `json:"function_def,omitzero"`
	HandlerType   HandlerType     `json:"handler_type"`
	HandlerConfig json.RawMessage `json:"handler_config,omitzero"`
	Enabled       bool            `json:"enabled"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CreateToolReq struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	FunctionDef   json.RawMessage `json:"function_def,omitzero"`
	HandlerType   HandlerType     `json:"handler_type"`
	HandlerConfig json.RawMessage `json:"handler_config,omitzero"`
	Enabled       *bool           `json:"enabled,omitzero"`
}

type UpdateToolReq struct {
	Name          *string         `json:"name,omitzero"`
	Description   *string         `json:"description,omitzero"`
	FunctionDef   json.RawMessage `json:"function_def,omitzero"`
	HandlerType   *HandlerType    `json:"handler_type,omitzero"`
	HandlerConfig json.RawMessage `json:"handler_config,omitzero"`
	Enabled       *bool           `json:"enabled,omitzero"`
}

type HTTPHandlerConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitzero"`
	Body    string            `json:"body,omitzero"`
}

type CommandHandlerConfig struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitzero"`
	Timeout    int    `json:"timeout,omitzero"`
	Shell      string `json:"shell,omitzero"`
}
