package api

import (
	"net/http"

	"github.com/open-fin/agent-composer/internal/domain"
)

// createSourceRequest is the body of POST /api/v1/sources.
type createSourceRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	BaseURL  string         `json:"base_url"`
	AuthType string         `json:"auth_type"`
	Config   map[string]any `json:"config"`
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var req createSourceRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	created, err := s.engine.Sources().Create(r.Context(), domain.ExternalSource{
		Name:     req.Name,
		Type:     domain.SourceType(req.Type),
		BaseURL:  req.BaseURL,
		AuthType: req.AuthType,
		Config:   req.Config,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	// Credentials never travel back out, even to the caller who just supplied them.
	writeData(w, http.StatusCreated, created.Redacted())
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.engine.Sources().List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	redacted := make([]domain.ExternalSource, 0, len(sources))
	for _, source := range sources {
		redacted = append(redacted, source.Redacted())
	}
	writeData(w, http.StatusOK, redacted)
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.engine.Sources().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, source.Redacted())
}
