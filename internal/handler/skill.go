package handler

import (
	"net/http"
	"strconv"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

type SkillHandler struct {
	store store.Store
}

func NewSkillHandler(s store.Store) *SkillHandler {
	return &SkillHandler{store: s}
}

func (h *SkillHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/skills", h.Create)
	mux.HandleFunc("GET /api/v1/skills", h.List)
	mux.HandleFunc("GET /api/v1/skills/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/skills/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", h.Delete)
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateSkillReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	sk := &model.Skill{
		Name:        req.Name,
		Description: req.Description,
		Instruction: req.Instruction,
	}
	if err := h.store.CreateSkill(r.Context(), sk); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	if len(req.ToolIDs) > 0 {
		h.store.SetSkillTools(r.Context(), sk.ID, req.ToolIDs)
	}
	httputil.OK(w, sk)
}

func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	ctx := r.Context()
	sk, err := h.store.GetSkill(ctx, id)
	if err != nil {
		httputil.NotFound(w, "skill not found")
		return
	}
	sk.Tools, _ = h.store.GetSkillTools(ctx, sk.ID)
	httputil.OK(w, sk)
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	q := parseListQuery(r)
	list, total, err := h.store.ListSkills(r.Context(), q)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OKList(w, list, total)
}

func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	var req model.UpdateSkillReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if err := h.store.UpdateSkill(r.Context(), id, req); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	if req.ToolIDs != nil {
		h.store.SetSkillTools(r.Context(), id, req.ToolIDs)
	}
	httputil.OK(w, nil)
}

func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	if err := h.store.DeleteSkill(r.Context(), id); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	httputil.OK(w, nil)
}
