package api

import (
	"net/http"

	"github.com/open-fin/agent-composer/internal/domain"
)

// recommendRequest is the body of POST /api/v1/compositions/recommend.
type recommendRequest struct {
	Name        string             `json:"name"`
	Goal        string             `json:"goal"`
	Description string             `json:"description"`
	HarnessType domain.HarnessType `json:"harness_type"`
}

func (s *Server) recommendComposition(w http.ResponseWriter, r *http.Request) {
	var req recommendRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	plan, err := s.engine.RecommendComposition(r.Context(), domain.BusinessGoal{
		Name:        req.Name,
		Goal:        req.Goal,
		Description: req.Description,
		HarnessType: req.HarnessType,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	// The plan is returned rather than stored: a reviewer may edit it in the UI before
	// asking for a draft, and POST /drafts takes the edited plan back.
	writeData(w, http.StatusOK, plan)
}

// validateRequest is the body of POST /api/v1/compositions/validate.
type validateRequest struct {
	Plan domain.CompositionPlan `json:"plan"`
}

func (s *Server) validateComposition(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	result, err := s.engine.ValidateComposition(r.Context(), req.Plan)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}
