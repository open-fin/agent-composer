package api

import (
	"net/http"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

// createDraftRequest is the body of POST /api/v1/drafts. It carries the whole plan
// because recommendations are not persisted: the client may have edited the plan in
// advanced mode between recommending and generating.
type createDraftRequest struct {
	Name        string                 `json:"name"`
	Goal        string                 `json:"goal"`
	Description string                 `json:"description"`
	HarnessType domain.HarnessType     `json:"harness_type"`
	Plan        domain.CompositionPlan `json:"plan"`
}

func (s *Server) createDraft(w http.ResponseWriter, r *http.Request) {
	var req createDraftRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	draft, err := s.engine.GenerateDraft(r.Context(), engine.GenerateDraftRequest{
		Name:        req.Name,
		Goal:        req.Goal,
		Description: req.Description,
		HarnessType: req.HarnessType,
		Plan:        req.Plan,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, draft)
}

func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	drafts, page, err := s.engine.Drafts().List(r.Context(), store.DraftFilter{
		Status: domain.NormalizeStatusFilter(queryString(r, "status")),
		Query:  queryString(r, "q"),
		Limit:  queryInt(r, "limit", store.DefaultLimit),
		Offset: queryInt(r, "offset", 0),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, drafts, page)
}

func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := s.engine.Drafts().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, draft)
}

func (s *Server) updateDraft(w http.ResponseWriter, r *http.Request) {
	var update domain.DraftUpdate
	if !decodeJSON(w, r, &update, s.maxUploadBytes) {
		return
	}
	draft, err := s.engine.Drafts().Update(r.Context(), r.PathValue("id"), update)
	if err != nil {
		writeError(w, err)
		return
	}
	// Renaming or retargeting a draft changes its document, so re-render and store it.
	yamlText, err := engine.RenderDraft(draft)
	if err != nil {
		writeError(w, err)
		return
	}
	draft.YAMLText = yamlText
	saved, err := s.engine.Drafts().Save(r.Context(), draft, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, saved)
}

// getDraftYAML serves the exported document. By default it answers inside the standard
// envelope so the YAML viewer can consume it like every other endpoint; `?download=1`
// switches to a raw attachment for the Export button.
func (s *Server) getDraftYAML(w http.ResponseWriter, r *http.Request) {
	draft, err := s.engine.Drafts().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	yamlText, err := engine.RenderDraft(draft)
	if err != nil {
		writeError(w, err)
		return
	}
	filename := draft.Slug + ".yaml"

	if download := queryString(r, "download"); download != "" && download != "0" && download != "false" {
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		_, _ = w.Write([]byte(yamlText))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"yaml": yamlText, "filename": filename})
}
