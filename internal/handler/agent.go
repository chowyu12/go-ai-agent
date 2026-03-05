package handler

import (
	"net/http"
	"strconv"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
	"github.com/google/uuid"
)

type AgentHandler struct {
	store store.Store
}

func NewAgentHandler(s store.Store) *AgentHandler {
	return &AgentHandler{store: s}
}

func (h *AgentHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/agents", h.Create)
	mux.HandleFunc("GET /api/v1/agents", h.List)
	mux.HandleFunc("GET /api/v1/agents/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/agents/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", h.Delete)
	mux.HandleFunc("POST /api/v1/agents/{id}/reset-token", h.ResetToken)
}

func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAgentReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	a := &model.Agent{
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		ProviderID:   req.ProviderID,
		ModelName:    req.ModelName,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
	}
	if a.Temperature == 0 {
		a.Temperature = 0.7
	}
	if a.MaxTokens == 0 {
		a.MaxTokens = 2048
	}
	if err := h.store.CreateAgent(r.Context(), a); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	ctx := r.Context()
	if len(req.ToolIDs) > 0 {
		h.store.SetAgentTools(ctx, a.ID, req.ToolIDs)
	}
	if len(req.SkillIDs) > 0 {
		h.store.SetAgentSkills(ctx, a.ID, req.SkillIDs)
	}
	if len(req.ChildIDs) > 0 {
		h.store.SetAgentChildren(ctx, a.ID, req.ChildIDs)
	}
	httputil.OK(w, a)
}

func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	ctx := r.Context()
	a, err := h.store.GetAgent(ctx, id)
	if err != nil {
		httputil.NotFound(w, "agent not found")
		return
	}
	a.Tools, _ = h.store.GetAgentTools(ctx, a.ID)
	a.Skills, _ = h.store.GetAgentSkills(ctx, a.ID)
	a.Children, _ = h.store.GetAgentChildren(ctx, a.ID)
	httputil.OK(w, a)
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	q := parseListQuery(r)
	list, total, err := h.store.ListAgents(r.Context(), q)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OKList(w, list, total)
}

func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	var req model.UpdateAgentReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	ctx := r.Context()
	if err := h.store.UpdateAgent(ctx, id, req); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	if req.ToolIDs != nil {
		h.store.SetAgentTools(ctx, id, req.ToolIDs)
	}
	if req.SkillIDs != nil {
		h.store.SetAgentSkills(ctx, id, req.SkillIDs)
	}
	if req.ChildIDs != nil {
		h.store.SetAgentChildren(ctx, id, req.ChildIDs)
	}
	httputil.OK(w, nil)
}

func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	if err := h.store.DeleteAgent(r.Context(), id); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}

func (h *AgentHandler) ResetToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	newToken := "ag-" + uuid.New().String()
	if err := h.store.UpdateAgentToken(r.Context(), id, newToken); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, map[string]string{"token": newToken})
}
