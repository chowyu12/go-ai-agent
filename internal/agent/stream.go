package agent

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (e *Executor) ExecuteStream(ctx context.Context, req model.ChatRequest, chunkHandler func(chunk model.StreamChunk) error) error {
	ec, err := e.prepare(ctx, req)
	if err != nil {
		return err
	}
	defer ec.closeMCP()

	ec.l.WithField("user", req.UserID).Info("[Execute] >> start (stream)")

	ec.tracker.SetOnStep(func(step model.ExecutionStep) {
		_ = chunkHandler(model.StreamChunk{
			ConversationID: ec.conv.UUID,
			Step:           &step,
		})
	})

	return e.streamAgentic(ctx, ec, chunkHandler)
}
