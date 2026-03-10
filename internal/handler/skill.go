package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/skill"
	"github.com/chowyu12/go-ai-agent/internal/skill/clawhub"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/internal/workspace"
	"github.com/chowyu12/go-ai-agent/pkg/httputil"
)

type SkillHandler struct {
	store      store.Store
	clawClient *clawhub.Client
}

func NewSkillHandler(s store.Store) *SkillHandler {
	return &SkillHandler{
		store:      s,
		clawClient: clawhub.NewClient(),
	}
}

func (h *SkillHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/skills", h.Create)
	mux.HandleFunc("GET /api/v1/skills", h.List)
	mux.HandleFunc("GET /api/v1/skills/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/skills/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", h.Delete)
	mux.HandleFunc("POST /api/v1/skills/install", h.Install)
	mux.HandleFunc("POST /api/v1/skills/sync", h.Sync)
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateSkillReq
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	s := &model.Skill{
		Name:        req.Name,
		Description: req.Description,
		Instruction: req.Instruction,
		Source:      req.Source,
		Slug:        req.Slug,
		Version:     req.Version,
		Author:      req.Author,
		DirName:     req.DirName,
		MainFile:    req.MainFile,
		Config:      req.Config,
		Permissions: req.Permissions,
		ToolDefs:    req.ToolDefs,
		Enabled:     enabled,
	}
	if s.Source == "" {
		s.Source = model.SkillSourceCustom
	}

	if s.DirName != "" && s.Source == model.SkillSourceCustom {
		h.createSkillDir(s)
	}

	ctx := r.Context()
	if err := h.store.CreateSkill(ctx, s); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	if len(req.ToolIDs) > 0 {
		h.store.SetSkillTools(ctx, s.ID, req.ToolIDs)
	}
	httputil.OK(w, s)
}

func (h *SkillHandler) createSkillDir(s *model.Skill) {
	skillsDir := workspace.Skills()
	if skillsDir == "" {
		return
	}
	dirPath := filepath.Join(skillsDir, s.DirName)
	os.MkdirAll(dirPath, 0o755)

	manifest := model.SkillManifest{
		Name:        s.Name,
		Version:     s.Version,
		Description: s.Description,
		Author:      s.Author,
		Main:        s.MainFile,
	}
	if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		os.WriteFile(filepath.Join(dirPath, "manifest.json"), data, 0o644)
	}
	if s.Instruction != "" {
		os.WriteFile(filepath.Join(dirPath, "SKILL.md"), []byte(s.Instruction), 0o644)
	}
}

func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.BadRequest(w, "invalid id")
		return
	}
	ctx := r.Context()
	s, err := h.store.GetSkill(ctx, id)
	if err != nil {
		httputil.NotFound(w, "skill not found")
		return
	}
	s.Tools, _ = h.store.GetSkillTools(ctx, s.ID)
	httputil.OK(w, s)
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
	ctx := r.Context()
	if err := h.store.UpdateSkill(ctx, id, req); err != nil {
		httputil.InternalError(w, err.Error())
		return
	}
	if req.ToolIDs != nil {
		h.store.SetSkillTools(ctx, id, req.ToolIDs)
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

type installReq struct {
	Slug string `json:"slug"`
}

func (h *SkillHandler) Install(w http.ResponseWriter, r *http.Request) {
	var req installReq
	if err := httputil.BindJSON(r, &req); err != nil || req.Slug == "" {
		httputil.BadRequest(w, "slug is required, e.g. himalaya")
		return
	}

	skillsDir := workspace.Skills()
	if skillsDir == "" {
		httputil.InternalError(w, "workspace not initialized")
		return
	}

	ctx := r.Context()
	destDir, err := h.clawClient.Download(ctx, req.Slug, skillsDir)
	if err != nil {
		log.WithError(err).WithField("slug", req.Slug).Error("[Skill] ClawHub download failed")
		httputil.InternalError(w, "download failed: "+err.Error())
		return
	}

	info, err := skill.ParseSkillDir(destDir)
	if err != nil {
		log.WithError(err).WithField("dir", destDir).Error("[Skill] parse skill dir failed")
		httputil.InternalError(w, "parse skill failed: "+err.Error())
		return
	}

	s := skill.InfoToSkill(*info, model.SkillSourceClawHub, req.Slug)

	existing, err := h.store.GetSkillByDirName(ctx, s.DirName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httputil.InternalError(w, err.Error())
		return
	}
	if existing != nil {
		updateReq := model.UpdateSkillReq{
			Name:        &s.Name,
			Description: &s.Description,
			Instruction: &s.Instruction,
			Version:     &s.Version,
			Author:      &s.Author,
			MainFile:    &s.MainFile,
			Config:      s.Config,
			Permissions: s.Permissions,
			ToolDefs:    s.ToolDefs,
		}
		src := model.SkillSourceClawHub
		updateReq.Source = &src
		slug := req.Slug
		updateReq.Slug = &slug
		if err := h.store.UpdateSkill(ctx, existing.ID, updateReq); err != nil {
			httputil.InternalError(w, err.Error())
			return
		}
		s.ID = existing.ID
	} else {
		if err := h.store.CreateSkill(ctx, s); err != nil {
			httputil.InternalError(w, err.Error())
			return
		}
	}

	httputil.OK(w, s)
}

func (h *SkillHandler) Sync(w http.ResponseWriter, r *http.Request) {
	skillsDir := workspace.Skills()
	if skillsDir == "" {
		httputil.InternalError(w, "workspace not initialized")
		return
	}

	infos, err := skill.ScanAll(skillsDir)
	if err != nil {
		log.WithError(err).Error("[Skill] scan skills dir failed")
		httputil.InternalError(w, "scan failed: "+err.Error())
		return
	}

	ctx := r.Context()
	var synced int
	for _, info := range infos {
		s := skill.InfoToSkill(info, model.SkillSourceLocal, "")

		existing, err := h.store.GetSkillByDirName(ctx, s.DirName)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if existing != nil {
			updateReq := model.UpdateSkillReq{
				Name:        &s.Name,
				Description: &s.Description,
				Instruction: &s.Instruction,
				Version:     &s.Version,
				Author:      &s.Author,
				MainFile:    &s.MainFile,
				Config:      s.Config,
				Permissions: s.Permissions,
				ToolDefs:    s.ToolDefs,
			}
			if existing.Source == model.SkillSourceClawHub {
				src := existing.Source
				updateReq.Source = &src
			} else {
				src := model.SkillSourceLocal
				updateReq.Source = &src
			}
			h.store.UpdateSkill(ctx, existing.ID, updateReq)
		} else {
			h.store.CreateSkill(ctx, s)
		}
		synced++
	}

	log.WithField("count", synced).Info("[Skill] sync completed")
	httputil.OK(w, map[string]int{"synced": synced})
}
