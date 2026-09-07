package api

import (
	"net/http"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

func (s *Server) listCandidates(w http.ResponseWriter, r *http.Request) {
	candidates, page, err := s.engine.Candidates().List(r.Context(), store.CandidateFilter{
		Status:   domain.NormalizeStatusFilter(queryString(r, "status")),
		Type:     queryString(r, "type"),
		SourceID: queryString(r, "source_id"),
		Query:    queryString(r, "q"),
		Limit:    queryInt(r, "limit", store.DefaultLimit),
		Offset:   queryInt(r, "offset", 0),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, candidates, page)
}

func (s *Server) getCandidate(w http.ResponseWriter, r *http.Request) {
	candidate, err := s.engine.Candidates().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	// The review panel edits the resolved view, so it is served alongside the layers.
	writeData(w, http.StatusOK, map[string]any{
		"candidate": candidate,
		"resolved":  candidate.Resolved(),
	})
}

// reviewRequest is the body of POST /api/v1/candidates/{id}/review.
type reviewRequest struct {
	Fields domain.CapabilityFields `json:"fields"`
	Note   string                  `json:"note"`
}

func (s *Server) reviewCandidate(w http.ResponseWriter, r *http.Request) {
	var req reviewRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	candidate, err := s.engine.ReviewCandidate(r.Context(), r.PathValue("id"), domain.ReviewInput{
		Fields: req.Fields,
		Note:   req.Note,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"candidate": candidate,
		"resolved":  candidate.Resolved(),
	})
}

func (s *Server) registerCandidate(w http.ResponseWriter, r *http.Request) {
	capability, err := s.engine.RegisterCandidate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, capability)
}

// rejectRequest is the body of POST /api/v1/candidates/{id}/reject.
type rejectRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) rejectCandidate(w http.ResponseWriter, r *http.Request) {
	var req rejectRequest
	// A reject with no body is valid; only a malformed one is an error.
	if r.ContentLength > 0 && !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	candidate, err := s.engine.RejectCandidate(r.Context(), r.PathValue("id"), req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, candidate)
}
