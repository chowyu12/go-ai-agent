package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type subAgentTool struct {
	agentUUID   string
	agentName   string
	description string
	executor    *Executor
	tracker     *StepTracker
}

func (t *subAgentTool) Name() string {
	return "delegate_" + sanitizeToolName(t.agentName)
}

func (t *subAgentTool) Description() string {
	desc := t.description
	if desc == "" {
		desc = "A sub-agent"
	}
	return fmt.Sprintf("将任务委托给子Agent '%s': %s。输入你要委托的具体任务描述。", t.agentName, desc)
}

func (t *subAgentTool) Call(ctx context.Context, input string) (string, error) {
	start := time.Now()
	result, err := t.executor.Execute(ctx, model.ChatRequest{
		AgentID: t.agentUUID,
		UserID:  "system",
		Message: input,
	})
	duration := time.Since(start)

	status := model.StepSuccess
	errMsg := ""
	output := ""
	if err != nil {
		status = model.StepError
		errMsg = err.Error()
	} else {
		output = result.Content
	}

	t.tracker.RecordStep(ctx, model.StepAgentCall, t.agentName, input, output, status, errMsg, duration, 0, &model.StepMetadata{
		AgentUUID: t.agentUUID,
		AgentName: t.agentName,
	})

	if err != nil {
		return "", err
	}
	return result.Content, nil
}

var _ Tool = (*subAgentTool)(nil)

func sanitizeToolName(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	s := sb.String()
	if s == "" {
		return "agent"
	}
	return s
}

func (e *Executor) buildSubAgentTools(ctx context.Context, agentID int64, tracker *StepTracker) []Tool {
	children, err := e.store.GetAgentChildren(ctx, agentID)
	if err != nil || len(children) == 0 {
		return nil
	}
	result := make([]Tool, 0, len(children))
	for _, child := range children {
		result = append(result, &subAgentTool{
			agentUUID:   child.UUID,
			agentName:   child.Name,
			description: child.Description,
			executor:    e,
			tracker:     tracker,
		})
	}
	return result
}
