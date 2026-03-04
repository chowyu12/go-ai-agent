package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type contextKey string

const claimsKey contextKey = "claims"

func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func Auth(secret string) func(http.Handler) http.Handler {
	jwtSecret := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if !strings.HasPrefix(path, "/api/v1/") ||
				path == "/api/v1/auth/login" ||
				strings.HasPrefix(path, "/api/v1/auth/setup") {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tokenStr == "" {
				httputil.Unauthorized(w, "missing token")
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
				return jwtSecret, nil
			})
			if err != nil || !token.Valid {
				httputil.Unauthorized(w, "invalid token")
				return
			}

			if r.Method != http.MethodGet &&
				!strings.HasPrefix(path, "/api/v1/chat/") &&
				!strings.HasPrefix(path, "/api/v1/auth/") {
				if model.Role(claims.Role) != model.RoleAdmin {
					httputil.Forbidden(w, "admin access required")
					return
				}
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		if !strings.HasPrefix(path, "/api/") {
			return
		}

		dur := time.Since(start)
		entry := log.WithField("duration", dur.String())
		if sw.status >= 400 {
			entry.Warnf("[HTTP] %s %s %d", r.Method, path, sw.status)
		} else {
			entry.Infof("[HTTP] %s %s %d", r.Method, path, sw.status)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
