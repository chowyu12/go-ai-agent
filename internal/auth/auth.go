package auth

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type Kind string

const (
	KindJWT        Kind = "jwt"
	KindAgentToken Kind = "agent_token"
)

type Identity struct {
	Kind      Kind
	UserID    int64
	Username  string
	Role      string
	AgentID   int64
	AgentUUID string
}

func (id *Identity) IsAdmin() bool {
	return model.Role(id.Role) == model.RoleAdmin
}

func (id *Identity) IsAgentToken() bool {
	return id.Kind == KindAgentToken
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	return id
}
