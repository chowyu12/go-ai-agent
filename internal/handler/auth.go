package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

type AuthHandler struct {
	store       store.Store
	jwtSecret   []byte
	expireHours int
}

func NewAuthHandler(s store.Store, secret string, expireHours int) *AuthHandler {
	return &AuthHandler{store: s, jwtSecret: []byte(secret), expireHours: expireHours}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/setup-check", h.SetupCheck)
	mux.HandleFunc("POST /api/v1/auth/setup", h.Setup)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)
	mux.HandleFunc("GET /api/v1/users", h.ListUsers)
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.DeleteUser)
}

func (h *AuthHandler) SetupCheck(w http.ResponseWriter, r *http.Request) {
	has, err := h.store.HasAdmin(r.Context())
	if err != nil {
		httputil.InternalError(w, "check failed")
		return
	}
	httputil.OK(w, map[string]bool{"initialized": has})
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	has, err := h.store.HasAdmin(r.Context())
	if err != nil {
		httputil.InternalError(w, "check failed")
		return
	}
	if has {
		httputil.BadRequest(w, "系统已初始化，无法重复创建超管")
		return
	}

	var req model.LoginReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.BadRequest(w, "username and password required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.InternalError(w, "hash password failed")
		return
	}

	u := &model.User{
		Username: req.Username,
		Password: string(hash),
		Role:     model.RoleAdmin,
		Enabled:  true,
	}
	if err := h.store.CreateUser(r.Context(), u); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	token, err := h.generateToken(u)
	if err != nil {
		httputil.InternalError(w, "generate token failed")
		return
	}

	httputil.OK(w, model.LoginResp{
		Token: token,
		User:  model.UserInfo{ID: u.ID, Username: u.Username, Role: u.Role},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.BadRequest(w, "username and password required")
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		httputil.InternalError(w, "internal error")
		return
	}
	if user == nil {
		httputil.Unauthorized(w, "invalid credentials")
		return
	}
	if !user.Enabled {
		httputil.Forbidden(w, "account disabled")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		httputil.Unauthorized(w, "invalid credentials")
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		httputil.InternalError(w, "generate token failed")
		return
	}

	httputil.OK(w, model.LoginResp{
		Token: token,
		User:  model.UserInfo{ID: user.ID, Username: user.Username, Role: user.Role},
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		httputil.Unauthorized(w, "unauthorized")
		return
	}
	httputil.OK(w, model.UserInfo{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     model.Role(claims.Role),
	})
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.BadRequest(w, "username and password required")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleGuest
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.InternalError(w, "hash password failed")
		return
	}

	u := &model.User{
		Username: req.Username,
		Password: string(hash),
		Role:     req.Role,
		Enabled:  true,
	}
	if err := h.store.CreateUser(r.Context(), u); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	u.Password = ""
	httputil.OK(w, u)
}

func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	var req model.UpdateUserReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request")
		return
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			httputil.InternalError(w, "hash password failed")
			return
		}
		hashed := string(hash)
		req.Password = &hashed
	}
	if err := h.store.UpdateUser(r.Context(), id, req); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := parseListQuery(r)
	list, total, err := h.store.ListUsers(r.Context(), q)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OKList(w, list, total)
}

func (h *AuthHandler) generateToken(u *model.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     string(u.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(h.expireHours) * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
