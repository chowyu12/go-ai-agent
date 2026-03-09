package model

import (
	"encoding/json"
	"time"
)

type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportSSE   MCPTransport = "sse"
)

type MCPServer struct {
	ID          int64            `json:"id"`
	UUID        string           `json:"uuid"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Transport   MCPTransport     `json:"transport"`
	Endpoint    string           `json:"endpoint"`
	Args        json.RawMessage  `json:"args,omitzero"`
	Env         json.RawMessage  `json:"env,omitzero"`
	Headers     json.RawMessage  `json:"headers,omitzero"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func (s *MCPServer) GetArgs() []string {
	var args []string
	if len(s.Args) > 0 {
		_ = json.Unmarshal(s.Args, &args)
	}
	return args
}

func (s *MCPServer) GetEnv() map[string]string {
	m := make(map[string]string)
	if len(s.Env) > 0 {
		_ = json.Unmarshal(s.Env, &m)
	}
	return m
}

func (s *MCPServer) GetHeaders() map[string]string {
	m := make(map[string]string)
	if len(s.Headers) > 0 {
		_ = json.Unmarshal(s.Headers, &m)
	}
	return m
}

type CreateMCPServerReq struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Transport   MCPTransport    `json:"transport"`
	Endpoint    string          `json:"endpoint"`
	Args        json.RawMessage `json:"args,omitzero"`
	Env         json.RawMessage `json:"env,omitzero"`
	Headers     json.RawMessage `json:"headers,omitzero"`
	Enabled     *bool           `json:"enabled,omitzero"`
}

type UpdateMCPServerReq struct {
	Name        *string         `json:"name,omitzero"`
	Description *string         `json:"description,omitzero"`
	Transport   *MCPTransport   `json:"transport,omitzero"`
	Endpoint    *string         `json:"endpoint,omitzero"`
	Args        json.RawMessage `json:"args,omitzero"`
	Env         json.RawMessage `json:"env,omitzero"`
	Headers     json.RawMessage `json:"headers,omitzero"`
	Enabled     *bool           `json:"enabled,omitzero"`
}
