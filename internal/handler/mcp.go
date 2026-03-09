package handler

import (
	"net/http"
	"strconv"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

type MCPHandler struct {
	store store.Store
}

func NewMCPHandler(s store.Store) *MCPHandler {
	return &MCPHandler{store: s}
}

func (h *MCPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/mcp-servers", h.Create)
	mux.HandleFunc("GET /api/v1/mcp-servers", h.List)
	mux.HandleFunc("GET /api/v1/mcp-servers/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/mcp-servers/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/mcp-servers/{id}", h.Delete)
}

func (h *MCPHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateMCPServerReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	s := &model.MCPServer{
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		Endpoint:    req.Endpoint,
		Args:        req.Args,
		Env:         req.Env,
		Headers:     req.Headers,
		Enabled:     true,
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if err := h.store.CreateMCPServer(r.Context(), s); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, s)
}

func (h *MCPHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	s, err := h.store.GetMCPServer(r.Context(), id)
	if err != nil {
		httputil.NotFound(w, "mcp server not found")
		return
	}
	httputil.OK(w, s)
}

func (h *MCPHandler) List(w http.ResponseWriter, r *http.Request) {
	q := parseListQuery(r)
	list, total, err := h.store.ListMCPServers(r.Context(), q)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OKList(w, list, total)
}

func (h *MCPHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	var req model.UpdateMCPServerReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if err := h.store.UpdateMCPServer(r.Context(), id, req); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}

func (h *MCPHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	if err := h.store.DeleteMCPServer(r.Context(), id); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}
