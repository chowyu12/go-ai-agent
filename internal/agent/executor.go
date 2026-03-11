package agent

import (
	"context"
	"encoding/json"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (e *Executor) Execute(ctx context.Context, req model.ChatRequest) (*ExecuteResult, error) {
	ec, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	defer ec.closeMCP()

	ec.l.WithField("user", req.UserID).Info("[Execute] >> start")
	if body, err := json.Marshal(req); err == nil {
		ec.l.WithField("body", string(body)).Debug("[Execute]    request body")
	}

	return e.executeAgentic(ctx, ec)
}
