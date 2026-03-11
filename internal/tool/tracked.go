package tool

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type TrackedTool struct {
	BaseTool  Tool
	ToolName  string
	SkillName string
	Tracker   *StepTracker
}

func (t *TrackedTool) Name() string        { return t.BaseTool.Name() }
func (t *TrackedTool) Description() string { return t.BaseTool.Description() }

func (t *TrackedTool) Call(ctx context.Context, input string) (string, error) {
	l := log.WithField("tool", t.ToolName)
	if t.SkillName != "" {
		l = l.WithField("skill", t.SkillName)
	}
	l.WithField("input", TruncateLog(input, 200)).Debug("[Tool]    invoke args")

	start := time.Now()
	output, err := t.BaseTool.Call(ctx, input)
	duration := time.Since(start)

	status := model.StepSuccess
	errMsg := ""
	if err != nil {
		status = model.StepError
		errMsg = err.Error()
	}

	t.Tracker.RecordStep(ctx, model.StepToolCall, t.ToolName, input, output, status, errMsg, duration, 0, &model.StepMetadata{
		ToolName:  t.ToolName,
		SkillName: t.SkillName,
	})
	return output, err
}

var _ Tool = (*TrackedTool)(nil)

type DynamicTool struct {
	ToolName string
	ToolDesc string
	Params   any
	Handler  func(ctx context.Context, input string) (string, error)
}

func (t *DynamicTool) Name() string        { return t.ToolName }
func (t *DynamicTool) Description() string { return t.ToolDesc }
func (t *DynamicTool) Call(ctx context.Context, input string) (string, error) {
	return t.Handler(ctx, input)
}

var _ Tool = (*DynamicTool)(nil)
