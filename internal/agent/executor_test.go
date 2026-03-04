package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
)

// ==================== Mock Store ====================

type mockStore struct {
	mu        sync.RWMutex
	nextIDVal atomic.Int64

	providers     map[int64]*model.Provider
	agents        map[int64]*model.Agent
	agentsByUUID  map[string]*model.Agent
	toolItems     map[int64]*model.Tool
	skillItems    map[int64]*model.Skill
	conversations map[int64]*model.Conversation
	convByUUID    map[string]*model.Conversation
	messages      map[int64][]model.Message
	execSteps     map[int64][]model.ExecutionStep

	agentToolIDs  map[int64][]int64
	agentSkillIDs map[int64][]int64
	agentChildIDs map[int64][]int64
	skillToolIDs  map[int64][]int64
}

func newMockStore() *mockStore {
	return &mockStore{
		providers:     make(map[int64]*model.Provider),
		agents:        make(map[int64]*model.Agent),
		agentsByUUID:  make(map[string]*model.Agent),
		toolItems:     make(map[int64]*model.Tool),
		skillItems:    make(map[int64]*model.Skill),
		conversations: make(map[int64]*model.Conversation),
		convByUUID:    make(map[string]*model.Conversation),
		messages:      make(map[int64][]model.Message),
		execSteps:     make(map[int64][]model.ExecutionStep),
		agentToolIDs:  make(map[int64][]int64),
		agentSkillIDs: make(map[int64][]int64),
		agentChildIDs: make(map[int64][]int64),
		skillToolIDs:  make(map[int64][]int64),
	}
}

func (s *mockStore) nextID() int64 { return s.nextIDVal.Add(1) }
func (s *mockStore) Close() error  { return nil }

func (s *mockStore) CreateProvider(_ context.Context, p *model.Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = s.nextID()
	s.providers[p.ID] = p
	return nil
}
func (s *mockStore) GetProvider(_ context.Context, id int64) (*model.Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.providers[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("provider %d not found", id)
}
func (s *mockStore) ListProviders(_ context.Context, _ model.ListQuery) ([]*model.Provider, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*model.Provider
	for _, p := range s.providers {
		list = append(list, p)
	}
	return list, int64(len(list)), nil
}
func (s *mockStore) UpdateProvider(_ context.Context, _ int64, _ model.UpdateProviderReq) error {
	return nil
}
func (s *mockStore) DeleteProvider(_ context.Context, _ int64) error { return nil }

func (s *mockStore) CreateAgent(_ context.Context, a *model.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.ID = s.nextID()
	if a.UUID == "" {
		a.UUID = fmt.Sprintf("ag-%d", a.ID)
	}
	s.agents[a.ID] = a
	s.agentsByUUID[a.UUID] = a
	return nil
}
func (s *mockStore) GetAgent(_ context.Context, id int64) (*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.agents[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent %d not found", id)
}
func (s *mockStore) GetAgentByUUID(_ context.Context, uuid string) (*model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.agentsByUUID[uuid]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent %s not found", uuid)
}
func (s *mockStore) ListAgents(_ context.Context, _ model.ListQuery) ([]*model.Agent, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*model.Agent
	for _, a := range s.agents {
		list = append(list, a)
	}
	return list, int64(len(list)), nil
}
func (s *mockStore) UpdateAgent(_ context.Context, _ int64, _ model.UpdateAgentReq) error {
	return nil
}
func (s *mockStore) DeleteAgent(_ context.Context, _ int64) error { return nil }

func (s *mockStore) SetAgentTools(_ context.Context, agentID int64, toolIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentToolIDs[agentID] = toolIDs
	return nil
}
func (s *mockStore) GetAgentTools(_ context.Context, agentID int64) ([]model.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.Tool
	for _, id := range s.agentToolIDs[agentID] {
		if t, ok := s.toolItems[id]; ok {
			result = append(result, *t)
		}
	}
	return result, nil
}
func (s *mockStore) SetAgentSkills(_ context.Context, agentID int64, skillIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentSkillIDs[agentID] = skillIDs
	return nil
}
func (s *mockStore) GetAgentSkills(_ context.Context, agentID int64) ([]model.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.Skill
	for _, id := range s.agentSkillIDs[agentID] {
		if sk, ok := s.skillItems[id]; ok {
			result = append(result, *sk)
		}
	}
	return result, nil
}
func (s *mockStore) SetAgentChildren(_ context.Context, agentID int64, childIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentChildIDs[agentID] = childIDs
	return nil
}
func (s *mockStore) GetAgentChildren(_ context.Context, agentID int64) ([]model.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.Agent
	for _, id := range s.agentChildIDs[agentID] {
		if a, ok := s.agents[id]; ok {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (s *mockStore) CreateTool(_ context.Context, t *model.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = s.nextID()
	s.toolItems[t.ID] = t
	return nil
}
func (s *mockStore) GetTool(_ context.Context, id int64) (*model.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.toolItems[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("tool %d not found", id)
}
func (s *mockStore) ListTools(_ context.Context, _ model.ListQuery) ([]*model.Tool, int64, error) {
	return nil, 0, nil
}
func (s *mockStore) UpdateTool(_ context.Context, _ int64, _ model.UpdateToolReq) error { return nil }
func (s *mockStore) DeleteTool(_ context.Context, _ int64) error                        { return nil }

func (s *mockStore) CreateSkill(_ context.Context, sk *model.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sk.ID = s.nextID()
	s.skillItems[sk.ID] = sk
	return nil
}
func (s *mockStore) GetSkill(_ context.Context, id int64) (*model.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sk, ok := s.skillItems[id]; ok {
		return sk, nil
	}
	return nil, fmt.Errorf("skill %d not found", id)
}
func (s *mockStore) ListSkills(_ context.Context, _ model.ListQuery) ([]*model.Skill, int64, error) {
	return nil, 0, nil
}
func (s *mockStore) UpdateSkill(_ context.Context, _ int64, _ model.UpdateSkillReq) error {
	return nil
}
func (s *mockStore) DeleteSkill(_ context.Context, _ int64) error { return nil }
func (s *mockStore) SetSkillTools(_ context.Context, skillID int64, toolIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillToolIDs[skillID] = toolIDs
	return nil
}
func (s *mockStore) GetSkillTools(_ context.Context, skillID int64) ([]model.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.Tool
	for _, id := range s.skillToolIDs[skillID] {
		if t, ok := s.toolItems[id]; ok {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (s *mockStore) CreateConversation(_ context.Context, c *model.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = s.nextID()
	if c.UUID == "" {
		c.UUID = fmt.Sprintf("conv-%d", c.ID)
	}
	s.conversations[c.ID] = c
	s.convByUUID[c.UUID] = c
	return nil
}
func (s *mockStore) GetConversation(_ context.Context, id int64) (*model.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.conversations[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("conversation %d not found", id)
}
func (s *mockStore) GetConversationByUUID(_ context.Context, uuid string) (*model.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.convByUUID[uuid]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("conversation %s not found", uuid)
}
func (s *mockStore) ListConversations(_ context.Context, _ int64, _ string, _ model.ListQuery) ([]*model.Conversation, int64, error) {
	return nil, 0, nil
}
func (s *mockStore) DeleteConversation(_ context.Context, _ int64) error { return nil }
func (s *mockStore) CreateMessage(_ context.Context, m *model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ID = s.nextID()
	s.messages[m.ConversationID] = append(s.messages[m.ConversationID], *m)
	return nil
}
func (s *mockStore) ListMessages(_ context.Context, conversationID int64, limit int) ([]model.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.messages[conversationID]
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	result := make([]model.Message, len(msgs))
	copy(result, msgs)
	return result, nil
}
func (s *mockStore) CreateExecutionStep(_ context.Context, step *model.ExecutionStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	step.ID = s.nextID()
	s.execSteps[step.ConversationID] = append(s.execSteps[step.ConversationID], *step)
	return nil
}
func (s *mockStore) UpdateStepsMessageID(_ context.Context, conversationID, messageID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := s.execSteps[conversationID]
	for i := range steps {
		if steps[i].MessageID == 0 {
			steps[i].MessageID = messageID
		}
	}
	s.execSteps[conversationID] = steps
	return nil
}
func (s *mockStore) ListExecutionSteps(_ context.Context, messageID int64) ([]model.ExecutionStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.ExecutionStep
	for _, steps := range s.execSteps {
		for _, step := range steps {
			if step.MessageID == messageID {
				result = append(result, step)
			}
		}
	}
	return result, nil
}
func (s *mockStore) ListExecutionStepsByConversation(_ context.Context, convID int64) ([]model.ExecutionStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.execSteps[convID], nil
}

// ==================== Mock UserStore (no-op) ====================

func (s *mockStore) CreateUser(_ context.Context, _ *model.User) error          { return nil }
func (s *mockStore) GetUserByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, nil
}
func (s *mockStore) GetUser(_ context.Context, _ int64) (*model.User, error)    { return nil, nil }
func (s *mockStore) ListUsers(_ context.Context, _ model.ListQuery) ([]*model.User, int64, error) {
	return nil, 0, nil
}
func (s *mockStore) UpdateUser(_ context.Context, _ int64, _ model.UpdateUserReq) error { return nil }
func (s *mockStore) DeleteUser(_ context.Context, _ int64) error                        { return nil }
func (s *mockStore) HasAdmin(_ context.Context) (bool, error)                           { return false, nil }

// ==================== Mock LLM Provider ====================

type mockLLMProvider struct {
	mu            sync.Mutex
	responses     []*llms.ContentResponse
	errors        []error
	callIdx       int
	calls         [][]llms.MessageContent
	streamContent string
	streamErr     error
}

var _ provider.LLMProvider = (*mockLLMProvider)(nil)

func (m *mockLLMProvider) GetModel() llms.Model { return nil }

func (m *mockLLMProvider) GenerateContent(_ context.Context, messages []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, messages)
	idx := m.callIdx
	m.callIdx++
	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: ""}}}, nil
}

func (m *mockLLMProvider) StreamContent(ctx context.Context, messages []llms.MessageContent, handler func(ctx context.Context, chunk []byte) error, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, messages)
	content := m.streamContent
	streamErr := m.streamErr
	m.mu.Unlock()

	if streamErr != nil {
		return nil, streamErr
	}
	const chunkSize = 10
	for i := 0; i < len(content); i += chunkSize {
		end := min(i+chunkSize, len(content))
		if err := handler(ctx, []byte(content[i:end])); err != nil {
			return nil, err
		}
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: content}}}, nil
}

func (m *mockLLMProvider) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// ==================== Test Helpers ====================

func testJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func seedAgent(t *testing.T, s *mockStore) (*model.Agent, *model.Provider) {
	t.Helper()
	ctx := t.Context()
	p := &model.Provider{Name: "test-prov", Type: model.ProviderOpenAI, APIKey: "k", Enabled: true}
	if err := s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	a := &model.Agent{
		UUID: "test-agent", Name: "TestBot", ProviderID: p.ID,
		ModelName: "gpt-test", Temperature: 0.5, MaxTokens: 512,
		SystemPrompt: "你是一个测试助手",
	}
	if err := s.CreateAgent(ctx, a); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func seedToolForAgent(t *testing.T, s *mockStore, agentID int64, name, desc string) *model.Tool {
	t.Helper()
	ctx := t.Context()
	tool := &model.Tool{
		Name: name, Description: desc, HandlerType: model.HandlerBuiltin, Enabled: true,
		FunctionDef: testJSON(map[string]any{
			"name": name, "description": desc,
			"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
		}),
	}
	if err := s.CreateTool(ctx, tool); err != nil {
		t.Fatal(err)
	}
	existing := s.agentToolIDs[agentID]
	s.SetAgentTools(ctx, agentID, append(existing, tool.ID))
	return tool
}

func newTestExecutor(s *mockStore, registry *ToolRegistry, mockLLM *mockLLMProvider) *Executor {
	return NewExecutor(s, registry, WithProviderFactory(
		func(_ *model.Provider, _ string) (provider.LLMProvider, error) {
			return mockLLM, nil
		},
	))
}

func textResp(content string) *llms.ContentResponse {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: content}},
	}
}

func toolCallResp(toolName, args string) *llms.ContentResponse {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			ToolCalls: []llms.ToolCall{{
				ID: "tc_" + toolName, Type: "function",
				FunctionCall: &llms.FunctionCall{Name: toolName, Arguments: args},
			}},
		}},
	}
}

// ==================== Pure Function Tests ====================

func TestBuildSystemPrompt(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ag := &model.Agent{}
		result := buildSystemPrompt(ag, nil, nil)
		if result != "" {
			t.Errorf("expected empty, got %q", result)
		}
	})

	t.Run("with_prompt", func(t *testing.T) {
		ag := &model.Agent{SystemPrompt: "你是助手"}
		result := buildSystemPrompt(ag, nil, nil)
		if result != "你是助手" {
			t.Errorf("expected '你是助手', got %q", result)
		}
	})

	t.Run("with_skills", func(t *testing.T) {
		ag := &model.Agent{SystemPrompt: "base"}
		skills := []model.Skill{{Name: "翻译", Instruction: "翻译指令"}}
		result := buildSystemPrompt(ag, skills, nil)
		if !strings.Contains(result, "Skill: 翻译") || !strings.Contains(result, "翻译指令") {
			t.Errorf("skill not included: %q", result)
		}
	})

	t.Run("with_tools", func(t *testing.T) {
		ag := &model.Agent{}
		result := buildSystemPrompt(ag, nil, []string{"current_time", "calculator"})
		if !strings.Contains(result, "current_time") || !strings.Contains(result, "calculator") {
			t.Errorf("tool names not included: %q", result)
		}
		if !strings.Contains(result, "工具使用策略") {
			t.Errorf("missing tool strategy section: %q", result)
		}
	})

	t.Run("full", func(t *testing.T) {
		ag := &model.Agent{SystemPrompt: "base prompt"}
		skills := []model.Skill{{Name: "代码审查", Instruction: "审查代码"}}
		toolNames := []string{"test_tool"}
		result := buildSystemPrompt(ag, skills, toolNames)
		if !strings.Contains(result, "base prompt") {
			t.Error("missing base prompt")
		}
		if !strings.Contains(result, "代码审查") {
			t.Error("missing skill")
		}
		if !strings.Contains(result, "test_tool") {
			t.Error("missing tool name")
		}
	})
}

func TestBuildLLMToolDefs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		defs := buildLLMToolDefs(nil, nil)
		if len(defs) != 0 {
			t.Errorf("expected 0, got %d", len(defs))
		}
	})

	t.Run("disabled_tools_skipped", func(t *testing.T) {
		modelTools := []model.Tool{
			{Name: "a", Description: "A", Enabled: false},
			{Name: "b", Description: "B", Enabled: true},
		}
		defs := buildLLMToolDefs(modelTools, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1, got %d", len(defs))
		}
		if defs[0].Function.Name != "b" {
			t.Errorf("expected tool 'b', got %q", defs[0].Function.Name)
		}
	})

	t.Run("with_function_def", func(t *testing.T) {
		funcDef := testJSON(map[string]any{
			"description": "custom desc",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		})
		modelTools := []model.Tool{
			{Name: "weather", Description: "orig", Enabled: true, FunctionDef: funcDef},
		}
		defs := buildLLMToolDefs(modelTools, nil)
		if len(defs) != 1 {
			t.Fatalf("expected 1, got %d", len(defs))
		}
		if defs[0].Function.Description != "custom desc" {
			t.Errorf("expected 'custom desc', got %q", defs[0].Function.Description)
		}
		params, ok := defs[0].Function.Parameters.(map[string]any)
		if !ok {
			t.Fatal("parameters should be map")
		}
		if _, hasProps := params["properties"]; !hasProps {
			t.Error("missing properties in parameters")
		}
	})

	t.Run("with_sub_agent_tools", func(t *testing.T) {
		subTools := []tools.Tool{&dynamicTool{
			toolName: "delegate_child",
			toolDesc: "delegate to child",
		}}
		defs := buildLLMToolDefs(nil, subTools)
		if len(defs) != 1 {
			t.Fatalf("expected 1, got %d", len(defs))
		}
		if defs[0].Function.Name != "delegate_child" {
			t.Errorf("expected 'delegate_child', got %q", defs[0].Function.Name)
		}
	})

	t.Run("no_parameters_adds_default", func(t *testing.T) {
		modelTools := []model.Tool{
			{Name: "simple", Description: "simple tool", Enabled: true},
		}
		defs := buildLLMToolDefs(modelTools, nil)
		if defs[0].Function.Parameters == nil {
			t.Error("expected default parameters, got nil")
		}
	})
}

func TestExtractContent(t *testing.T) {
	t.Run("nil_response", func(t *testing.T) {
		if got := extractContent(nil); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("empty_choices", func(t *testing.T) {
		if got := extractContent(&llms.ContentResponse{}); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("normal", func(t *testing.T) {
		resp := textResp("hello world")
		if got := extractContent(resp); got != "hello world" {
			t.Errorf("expected 'hello world', got %q", got)
		}
	})
}

func TestTruncateLog(t *testing.T) {
	t.Run("short_string", func(t *testing.T) {
		if got := truncateLog("abc", 10); got != "abc" {
			t.Errorf("expected 'abc', got %q", got)
		}
	})
	t.Run("long_string", func(t *testing.T) {
		got := truncateLog("abcdefghij", 5)
		if got != "abcde..." {
			t.Errorf("expected 'abcde...', got %q", got)
		}
	})
	t.Run("replaces_newlines", func(t *testing.T) {
		got := truncateLog("a\nb\nc", 100)
		if strings.Contains(got, "\n") {
			t.Errorf("should not contain newlines: %q", got)
		}
	})
}

func TestSanitizeToolName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"Hello World", "hello_world"},
		{"test-tool!", "test_tool_"},
		{"", "agent"},
		{"测试工具", "____"},
		{"tool_123", "tool_123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeToolName(tt.input); got != tt.want {
				t.Errorf("sanitizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== Executor Integration Tests ====================

func TestExecute_Simple(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	mockLLM := &mockLLMProvider{responses: []*llms.ContentResponse{textResp("你好世界")}}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	result, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "你好世界" {
		t.Errorf("expected '你好世界', got %q", result.Content)
	}
	if result.ConversationID == "" {
		t.Error("conversation ID should not be empty")
	}
	if mockLLM.callCount() != 1 {
		t.Errorf("expected 1 LLM call, got %d", mockLLM.callCount())
	}
	if len(result.Steps) == 0 {
		t.Error("expected at least 1 execution step")
	}
}

func TestExecute_AgentNotFound(t *testing.T) {
	s := newMockStore()
	mockLLM := &mockLLMProvider{}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	_, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: "nonexistent", UserID: "u1", Message: "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "agent not found") {
		t.Errorf("expected 'agent not found' error, got: %v", err)
	}
}

func TestExecute_ProviderNotFound(t *testing.T) {
	s := newMockStore()
	ctx := t.Context()
	a := &model.Agent{UUID: "orphan", Name: "Orphan", ProviderID: 9999}
	s.CreateAgent(ctx, a)
	mockLLM := &mockLLMProvider{}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	_, err := exec.Execute(ctx, model.ChatRequest{
		AgentID: "orphan", UserID: "u1", Message: "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "provider not found") {
		t.Errorf("expected 'provider not found' error, got: %v", err)
	}
}

func TestExecute_LLMError(t *testing.T) {
	s := newMockStore()
	seedAgent(t, s)
	mockLLM := &mockLLMProvider{
		errors: []error{errors.New("rate limit exceeded")},
	}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	_, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: "test-agent", UserID: "u1", Message: "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected 'rate limit exceeded', got: %v", err)
	}
}

func TestExecute_WithToolCall(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)

	registry := NewToolRegistry()
	registry.RegisterBuiltin("test_echo", func(_ context.Context, args string) (string, error) {
		return "ECHO:" + args, nil
	})
	seedToolForAgent(t, s, agent.ID, "test_echo", "echo tool for test")

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			toolCallResp("test_echo", `{"text":"ping"}`),
			textResp("工具返回了 ECHO:{\"text\":\"ping\"}"),
		},
	}
	exec := newTestExecutor(s, registry, mockLLM)

	result, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "测试工具",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "ECHO") {
		t.Errorf("expected content to reference ECHO, got %q", result.Content)
	}
	if mockLLM.callCount() != 2 {
		t.Errorf("expected 2 LLM calls (tool request + final), got %d", mockLLM.callCount())
	}

	hasToolStep := false
	for _, step := range result.Steps {
		if step.StepType == model.StepToolCall && step.Name == "test_echo" {
			hasToolStep = true
			if !strings.Contains(step.Output, "ECHO:") {
				t.Errorf("tool step output should contain ECHO, got %q", step.Output)
			}
			if step.MessageID == 0 {
				t.Error("tool step message_id should not be 0 after SetMessageID")
			}
		}
	}
	if !hasToolStep {
		t.Error("expected a tool_call execution step for test_echo")
	}

	// Verify tool steps are queryable by messageID through store
	for _, step := range result.Steps {
		if step.StepType == model.StepToolCall {
			dbSteps, err := s.ListExecutionSteps(t.Context(), step.MessageID)
			if err != nil {
				t.Fatalf("ListExecutionSteps: %v", err)
			}
			found := false
			for _, ds := range dbSteps {
				if ds.Name == "test_echo" && ds.StepType == model.StepToolCall {
					found = true
				}
			}
			if !found {
				t.Error("tool step should be queryable by messageID from store")
			}
			break
		}
	}
}

func TestExecute_WithMultipleToolCalls(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)

	registry := NewToolRegistry()
	registry.RegisterBuiltin("tool_a", func(_ context.Context, _ string) (string, error) {
		return "result_a", nil
	})
	registry.RegisterBuiltin("tool_b", func(_ context.Context, _ string) (string, error) {
		return "result_b", nil
	})
	seedToolForAgent(t, s, agent.ID, "tool_a", "tool A")
	seedToolForAgent(t, s, agent.ID, "tool_b", "tool B")

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			{Choices: []*llms.ContentChoice{{
				ToolCalls: []llms.ToolCall{
					{ID: "c1", Type: "function", FunctionCall: &llms.FunctionCall{Name: "tool_a", Arguments: "{}"}},
					{ID: "c2", Type: "function", FunctionCall: &llms.FunctionCall{Name: "tool_b", Arguments: "{}"}},
				},
			}}},
			textResp("综合结果: result_a 和 result_b"),
		},
	}
	exec := newTestExecutor(s, registry, mockLLM)

	result, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "调用两个工具",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "result_a") || !strings.Contains(result.Content, "result_b") {
		t.Errorf("expected both results, got %q", result.Content)
	}

	toolStepCount := 0
	for _, step := range result.Steps {
		if step.StepType == model.StepToolCall {
			toolStepCount++
		}
	}
	if toolStepCount != 2 {
		t.Errorf("expected 2 tool steps, got %d", toolStepCount)
	}
}

func TestExecute_ToolCallError(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)

	registry := NewToolRegistry()
	registry.RegisterBuiltin("failing_tool", func(_ context.Context, _ string) (string, error) {
		return "", errors.New("tool internal error")
	})
	seedToolForAgent(t, s, agent.ID, "failing_tool", "tool that fails")

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			toolCallResp("failing_tool", "{}"),
			textResp("工具调用失败了，让我直接回答"),
		},
	}
	exec := newTestExecutor(s, registry, mockLLM)

	result, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "试试工具",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty content even after tool error")
	}

	hasErrorStep := false
	for _, step := range result.Steps {
		if step.StepType == model.StepToolCall && step.Status == model.StepError {
			hasErrorStep = true
		}
	}
	if !hasErrorStep {
		t.Error("expected an error tool step")
	}
}

func TestExecute_ToolNotFoundByLLM(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	seedToolForAgent(t, s, agent.ID, "real_tool", "a real tool")

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			toolCallResp("nonexistent_tool", "{}"),
			textResp("我没法使用那个工具"),
		},
	}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	result, err := exec.Execute(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestExecute_WithSkills(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	ctx := t.Context()

	sk := &model.Skill{Name: "翻译助手", Instruction: "翻译指令内容"}
	s.CreateSkill(ctx, sk)
	s.SetAgentSkills(ctx, agent.ID, []int64{sk.ID})

	mockLLM := &mockLLMProvider{responses: []*llms.ContentResponse{textResp("translated content")}}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	result, err := exec.Execute(ctx, model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "translate this",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "translated content" {
		t.Errorf("unexpected content: %q", result.Content)
	}

	llmMsgs := mockLLM.calls[0]
	systemMsg := ""
	for _, msg := range llmMsgs {
		if msg.Role == llms.ChatMessageTypeSystem {
			for _, part := range msg.Parts {
				if tc, ok := part.(llms.TextContent); ok {
					systemMsg += tc.Text
				}
			}
		}
	}
	if !strings.Contains(systemMsg, "翻译指令内容") {
		t.Errorf("system prompt should include skill instruction, got %q", systemMsg)
	}
}

func TestExecute_ConversationReuse(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			textResp("first response"),
			textResp("second response"),
		},
	}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)
	ctx := t.Context()

	r1, err := exec.Execute(ctx, model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "第一条消息",
	})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	convID := r1.ConversationID

	r2, err := exec.Execute(ctx, model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "第二条消息",
		ConversationID: convID,
	})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if r2.ConversationID != convID {
		t.Errorf("expected same conversation %q, got %q", convID, r2.ConversationID)
	}
	if r2.Content != "second response" {
		t.Errorf("expected 'second response', got %q", r2.Content)
	}

	secondCallMsgs := mockLLM.calls[1]
	historyCount := 0
	for _, msg := range secondCallMsgs {
		if msg.Role == llms.ChatMessageTypeHuman || msg.Role == llms.ChatMessageTypeAI {
			historyCount++
		}
	}
	if historyCount < 2 {
		t.Errorf("expected at least 2 history messages (prev user+assistant), got %d", historyCount)
	}
}

func TestExecute_WithSubAgents(t *testing.T) {
	s := newMockStore()
	ctx := t.Context()

	p := &model.Provider{Name: "prov", Type: model.ProviderOpenAI, APIKey: "k", Enabled: true}
	s.CreateProvider(ctx, p)

	child := &model.Agent{UUID: "child-agent", Name: "ChildBot", ProviderID: p.ID, ModelName: "gpt-test", Temperature: 0.5, MaxTokens: 256}
	s.CreateAgent(ctx, child)

	parent := &model.Agent{UUID: "parent-agent", Name: "ParentBot", ProviderID: p.ID, ModelName: "gpt-test", Temperature: 0.5, MaxTokens: 512}
	s.CreateAgent(ctx, parent)
	s.SetAgentChildren(ctx, parent.ID, []int64{child.ID})

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			toolCallResp("delegate_childbot", `{"input":"子任务"}`),
			textResp("子代理完成了任务"),
			textResp("最终结果：子代理完成了任务"),
		},
	}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	result, err := exec.Execute(ctx, model.ChatRequest{
		AgentID: parent.UUID, UserID: "u1", Message: "委托子代理",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty content")
	}
	if mockLLM.callCount() < 2 {
		t.Errorf("expected at least 2 LLM calls (parent + child), got %d", mockLLM.callCount())
	}
}

func TestExecuteStream_Simple(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	mockLLM := &mockLLMProvider{streamContent: "这是流式响应内容"}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	var chunks []model.StreamChunk
	err := exec.ExecuteStream(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "hello",
	}, func(chunk model.StreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.Done {
		t.Error("last chunk should have Done=true")
	}

	var content strings.Builder
	for _, c := range chunks {
		content.WriteString(c.Delta)
	}
	if !strings.Contains(content.String(), "这是流式响应内容") {
		t.Errorf("content mismatch: %q", content.String())
	}
}

func TestExecuteStream_LLMError(t *testing.T) {
	s := newMockStore()
	seedAgent(t, s)
	mockLLM := &mockLLMProvider{streamErr: errors.New("stream broken")}
	exec := newTestExecutor(s, NewToolRegistry(), mockLLM)

	err := exec.ExecuteStream(t.Context(), model.ChatRequest{
		AgentID: "test-agent", UserID: "u1", Message: "hello",
	}, func(_ model.StreamChunk) error { return nil })
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "stream broken") {
		t.Errorf("expected 'stream broken', got: %v", err)
	}
}

func TestExecuteStream_WithTools(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)

	registry := NewToolRegistry()
	registry.RegisterBuiltin("stream_echo", func(_ context.Context, args string) (string, error) {
		return "STREAM_ECHO:" + args, nil
	})
	seedToolForAgent(t, s, agent.ID, "stream_echo", "echo for stream test")

	mockLLM := &mockLLMProvider{
		responses: []*llms.ContentResponse{
			toolCallResp("stream_echo", `{"msg":"hi"}`),
			textResp("流式工具结果已处理"),
		},
	}
	exec := newTestExecutor(s, registry, mockLLM)

	var chunks []model.StreamChunk
	err := exec.ExecuteStream(t.Context(), model.ChatRequest{
		AgentID: agent.UUID, UserID: "u1", Message: "stream with tool",
	}, func(chunk model.StreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasDone := false
	var content strings.Builder
	for _, c := range chunks {
		content.WriteString(c.Delta)
		if c.Done {
			hasDone = true
		}
	}
	if !hasDone {
		t.Error("expected a Done chunk")
	}
	if !strings.Contains(content.String(), "流式工具结果已处理") {
		t.Errorf("content mismatch: %q", content.String())
	}
}

func TestCollectTools(t *testing.T) {
	s := newMockStore()
	agent, _ := seedAgent(t, s)
	ctx := t.Context()

	directTool := seedToolForAgent(t, s, agent.ID, "direct_tool", "direct")

	sk := &model.Skill{Name: "testskill"}
	s.CreateSkill(ctx, sk)
	s.SetAgentSkills(ctx, agent.ID, []int64{sk.ID})

	skillTool := &model.Tool{Name: "skill_tool", Description: "from skill", HandlerType: model.HandlerBuiltin, Enabled: true}
	s.CreateTool(ctx, skillTool)
	s.SetSkillTools(ctx, sk.ID, []int64{skillTool.ID})

	exec := newTestExecutor(s, NewToolRegistry(), &mockLLMProvider{})
	tools, err := exec.collectTools(ctx, agent.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names[directTool.Name] {
		t.Errorf("missing direct tool %q", directTool.Name)
	}
	if !names["skill_tool"] {
		t.Error("missing skill_tool")
	}
}

func TestBuildMessages(t *testing.T) {
	exec := &Executor{}
	ag := &model.Agent{SystemPrompt: "you are a bot"}
	skills := []model.Skill{{Name: "sk1", Instruction: "do stuff"}}
	history := []llms.MessageContent{
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "prev question"}}},
		{Role: llms.ChatMessageTypeAI, Parts: []llms.ContentPart{llms.TextContent{Text: "prev answer"}}},
	}
	toolNames := []string{"tool1"}

	msgs := exec.buildMessages(ag, skills, history, "new question", toolNames)

	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 messages (system + 2 history + user), got %d", len(msgs))
	}
	if msgs[0].Role != llms.ChatMessageTypeSystem {
		t.Errorf("first message should be system, got %s", msgs[0].Role)
	}
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != llms.ChatMessageTypeHuman {
		t.Errorf("last message should be human, got %s", lastMsg.Role)
	}
	lastText := ""
	for _, part := range lastMsg.Parts {
		if tc, ok := part.(llms.TextContent); ok {
			lastText = tc.Text
		}
	}
	if lastText != "new question" {
		t.Errorf("last message content should be 'new question', got %q", lastText)
	}
}

// ==================== Registry Tests ====================

func TestToolRegistry_BuildTrackedTools(t *testing.T) {
	registry := NewToolRegistry()

	t.Run("builtin_tool", func(t *testing.T) {
		tracker := NewStepTracker(newMockStore(), 1)
		toolDefs := []model.Tool{
			{Name: "current_time", Description: "get time", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker)
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].Name() != "current_time" {
			t.Errorf("expected 'current_time', got %q", tools[0].Name())
		}
		output, err := tools[0].Call(t.Context(), "{}")
		if err != nil {
			t.Fatalf("tool call error: %v", err)
		}
		if output == "" {
			t.Error("expected non-empty output from current_time")
		}
	})

	t.Run("disabled_tool_skipped", func(t *testing.T) {
		tracker := NewStepTracker(newMockStore(), 1)
		toolDefs := []model.Tool{
			{Name: "current_time", HandlerType: model.HandlerBuiltin, Enabled: false},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker)
		if len(tools) != 0 {
			t.Errorf("expected 0 tools, got %d", len(tools))
		}
	})

	t.Run("unknown_builtin_skipped", func(t *testing.T) {
		tracker := NewStepTracker(newMockStore(), 1)
		toolDefs := []model.Tool{
			{Name: "nonexistent_builtin", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker)
		if len(tools) != 0 {
			t.Errorf("expected 0 tools, got %d", len(tools))
		}
	})

	t.Run("tracked_tool_records_step", func(t *testing.T) {
		ms := newMockStore()
		tracker := NewStepTracker(ms, 100)
		toolDefs := []model.Tool{
			{Name: "uuid_generator", Description: "gen uuid", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker)
		if len(tools) != 1 {
			t.Fatal("expected 1 tool")
		}
		_, err := tools[0].Call(t.Context(), "{}")
		if err != nil {
			t.Fatal(err)
		}
		steps := tracker.Steps()
		if len(steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(steps))
		}
		if steps[0].StepType != model.StepToolCall {
			t.Errorf("expected tool_call step, got %s", steps[0].StepType)
		}
		if steps[0].Status != model.StepSuccess {
			t.Errorf("expected success status, got %s", steps[0].Status)
		}
	})
}

func TestBuiltinHandlers(t *testing.T) {
	registry := NewToolRegistry()
	ctx := t.Context()

	t.Run("base64_encode", func(t *testing.T) {
		handler := registry.builtins["base64_encode"]
		result, err := handler(ctx, `{"text":"hello"}`)
		if err != nil {
			t.Fatal(err)
		}
		if result != "aGVsbG8=" {
			t.Errorf("expected 'aGVsbG8=', got %q", result)
		}
	})

	t.Run("base64_decode", func(t *testing.T) {
		handler := registry.builtins["base64_decode"]
		result, err := handler(ctx, `{"text":"aGVsbG8="}`)
		if err != nil {
			t.Fatal(err)
		}
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	t.Run("hash_text_sha256", func(t *testing.T) {
		handler := registry.builtins["hash_text"]
		result, err := handler(ctx, `{"text":"test","algorithm":"sha256"}`)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 64 {
			t.Errorf("expected 64 char sha256 hex, got len=%d", len(result))
		}
	})

	t.Run("json_formatter", func(t *testing.T) {
		handler := registry.builtins["json_formatter"]
		result, err := handler(ctx, `{"json_string":"{\"a\":1}"}`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, "\"a\"") {
			t.Errorf("expected formatted JSON, got %q", result)
		}
	})

	t.Run("random_number", func(t *testing.T) {
		handler := registry.builtins["random_number"]
		result, err := handler(ctx, `{"min":1,"max":1}`)
		if err != nil {
			t.Fatal(err)
		}
		if result != "1" {
			t.Errorf("expected '1', got %q", result)
		}
	})
}

// ==================== StepTracker Tests ====================

func TestStepTracker(t *testing.T) {
	ms := newMockStore()
	tracker := NewStepTracker(ms, 42)

	if steps := tracker.Steps(); len(steps) != 0 {
		t.Errorf("new tracker should have 0 steps, got %d", len(steps))
	}

	tracker.SetMessageID(10)
	ctx := t.Context()
	step := tracker.RecordStep(ctx, model.StepToolCall, "my_tool", "input", "output", model.StepSuccess, "", 100, 0, nil)

	if step.StepOrder != 1 {
		t.Errorf("expected step order 1, got %d", step.StepOrder)
	}
	if step.MessageID != 10 {
		t.Errorf("expected message ID 10, got %d", step.MessageID)
	}
	if step.ConversationID != 42 {
		t.Errorf("expected conversation ID 42, got %d", step.ConversationID)
	}

	steps := tracker.Steps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	tracker.RecordStep(ctx, model.StepLLMCall, "gpt", "q", "a", model.StepSuccess, "", 200, 0, nil)
	if len(tracker.Steps()) != 2 {
		t.Errorf("expected 2 steps after second record")
	}
}

// ==================== Memory Manager Tests ====================

func TestMemoryManager_GetOrCreateConversation(t *testing.T) {
	ms := newMockStore()
	mm := NewMemoryManager(ms)
	ctx := t.Context()

	conv1, err := mm.GetOrCreateConversation(ctx, "", 1, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if conv1.UUID == "" {
		t.Error("new conversation should have UUID")
	}
	if conv1.AgentID != 1 {
		t.Errorf("expected agent ID 1, got %d", conv1.AgentID)
	}

	conv2, err := mm.GetOrCreateConversation(ctx, conv1.UUID, 1, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if conv2.ID != conv1.ID {
		t.Errorf("expected same conversation ID %d, got %d", conv1.ID, conv2.ID)
	}

	conv3, err := mm.GetOrCreateConversation(ctx, "nonexistent", 1, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if conv3.ID == conv1.ID {
		t.Error("nonexistent UUID should create new conversation")
	}
}

func TestMemoryManager_SaveAndLoadHistory(t *testing.T) {
	ms := newMockStore()
	mm := NewMemoryManager(ms)
	ctx := t.Context()

	conv, _ := mm.GetOrCreateConversation(ctx, "", 1, "user1")

	mm.SaveMessage(ctx, conv.ID, "user", "你好", 0)
	mm.SaveMessage(ctx, conv.ID, "assistant", "你好！", 0)
	mm.SaveMessage(ctx, conv.ID, "user", "再见", 0)

	history, err := mm.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history messages, got %d", len(history))
	}
	if history[0].Role != llms.ChatMessageTypeHuman {
		t.Errorf("first message should be human, got %s", history[0].Role)
	}
	if history[1].Role != llms.ChatMessageTypeAI {
		t.Errorf("second message should be AI, got %s", history[1].Role)
	}
}
