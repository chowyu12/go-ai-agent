package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type nopConvStore struct{}

func (nopConvStore) CreateConversation(_ context.Context, _ *model.Conversation) error { return nil }
func (nopConvStore) GetConversation(_ context.Context, _ int64) (*model.Conversation, error) {
	return nil, nil
}
func (nopConvStore) GetConversationByUUID(_ context.Context, _ string) (*model.Conversation, error) {
	return nil, nil
}
func (nopConvStore) ListConversations(_ context.Context, _ int64, _ string, _ model.ListQuery) ([]*model.Conversation, int64, error) {
	return nil, 0, nil
}
func (nopConvStore) UpdateConversationTitle(_ context.Context, _ int64, _ string) error { return nil }
func (nopConvStore) DeleteConversation(_ context.Context, _ int64) error                { return nil }
func (nopConvStore) CreateMessage(_ context.Context, _ *model.Message) error            { return nil }
func (nopConvStore) ListMessages(_ context.Context, _ int64, _ int) ([]model.Message, error) {
	return nil, nil
}
func (nopConvStore) CreateExecutionStep(_ context.Context, _ *model.ExecutionStep) error { return nil }
func (nopConvStore) UpdateStepsMessageID(_ context.Context, _, _ int64) error            { return nil }
func (nopConvStore) ListExecutionSteps(_ context.Context, _ int64) ([]model.ExecutionStep, error) {
	return nil, nil
}
func (nopConvStore) ListExecutionStepsByConversation(_ context.Context, _ int64) ([]model.ExecutionStep, error) {
	return nil, nil
}

func TestRegistry_BuildTrackedTools(t *testing.T) {
	registry := NewRegistry()
	registry.LoadDefaults()

	t.Run("builtin_tool", func(t *testing.T) {
		tracker := NewStepTracker(nopConvStore{}, 1)
		toolDefs := []model.Tool{
			{Name: "current_time", Description: "get time", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker, nil)
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
		tracker := NewStepTracker(nopConvStore{}, 1)
		toolDefs := []model.Tool{
			{Name: "current_time", HandlerType: model.HandlerBuiltin, Enabled: false},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker, nil)
		if len(tools) != 0 {
			t.Errorf("expected 0 tools, got %d", len(tools))
		}
	})

	t.Run("unknown_builtin_skipped", func(t *testing.T) {
		tracker := NewStepTracker(nopConvStore{}, 1)
		toolDefs := []model.Tool{
			{Name: "nonexistent_builtin", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker, nil)
		if len(tools) != 0 {
			t.Errorf("expected 0 tools, got %d", len(tools))
		}
	})

	t.Run("tracked_tool_records_step", func(t *testing.T) {
		tracker := NewStepTracker(nopConvStore{}, 100)
		toolDefs := []model.Tool{
			{Name: "uuid_generator", Description: "gen uuid", HandlerType: model.HandlerBuiltin, Enabled: true},
		}
		tools := registry.BuildTrackedTools(toolDefs, tracker, nil)
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
	registry := NewRegistry()
	registry.LoadDefaults()
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
