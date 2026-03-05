package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

// TokenResolver resolves an agent token to an Agent.
// Kept minimal to avoid coupling to the full store interface.
type TokenResolver interface {
	GetAgentByToken(ctx context.Context, token string) (*model.Agent, error)
}

type Config struct {
	JWTSecret     []byte
	TokenResolver TokenResolver
}

// Middleware returns an HTTP middleware that performs authentication and authorization.
//
// Flow: extract token → authenticate (JWT / Agent Token) → authorize → next handler.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublic(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := extractBearer(r)
			if tokenStr == "" {
				httputil.Unauthorized(w, "missing token")
				return
			}

			id, err := authenticate(cfg, r.Context(), tokenStr)
			if err != nil {
				httputil.Unauthorized(w, "invalid token")
				return
			}

			if err := authorize(id, r.Method, r.URL.Path); err != nil {
				httputil.Forbidden(w, err.Error())
				return
			}

			ctx := WithIdentity(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// --- extract ---

func extractBearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if v == "" {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}

// --- authenticate ---

func authenticate(cfg Config, ctx context.Context, tokenStr string) (*Identity, error) {
	if strings.HasPrefix(tokenStr, "ag-") {
		return authenticateAgentToken(cfg.TokenResolver, ctx, tokenStr)
	}
	return parseJWT(cfg.JWTSecret, tokenStr)
}

func authenticateAgentToken(resolver TokenResolver, ctx context.Context, token string) (*Identity, error) {
	ag, err := resolver.GetAgentByToken(ctx, token)
	if err != nil {
		return nil, errors.New("invalid agent token")
	}
	return &Identity{
		Kind:      KindAgentToken,
		AgentID:   ag.ID,
		AgentUUID: ag.UUID,
	}, nil
}

// --- authorize ---

func authorize(id *Identity, method, path string) error {
	if id.IsAgentToken() {
		if !strings.HasPrefix(path, "/api/v1/chat/") {
			return errors.New("agent token can only access chat endpoints")
		}
		return nil
	}

	if method != http.MethodGet &&
		!strings.HasPrefix(path, "/api/v1/chat/") &&
		!strings.HasPrefix(path, "/api/v1/auth/") {
		if !id.IsAdmin() {
			return errors.New("admin access required")
		}
	}
	return nil
}

// --- public routes ---

func isPublic(path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") {
		return true
	}
	return path == "/api/v1/auth/login" ||
		strings.HasPrefix(path, "/api/v1/auth/setup")
}
