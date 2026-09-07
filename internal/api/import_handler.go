package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/open-fin/agent-composer/internal/engine"
)

// importRequest is the JSON form of an import. A payload may also arrive as a raw file
// body or as a multipart upload, which is what the web UI sends.
type importRequest struct {
	SourceID   string         `json:"source_id"`
	SourceName string         `json:"source_name"`
	Content    string         `json:"content"`
	Options    map[string]any `json:"options"`
}

// readImportRequest accepts the three shapes a client might send: multipart form upload,
// a JSON envelope with the document inline, or the raw document as the request body.
func (s *Server) readImportRequest(w http.ResponseWriter, r *http.Request) (importRequest, bool) {
	var req importRequest
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
			writeAPIError(w, http.StatusBadRequest, CodeParse, "invalid multipart upload: "+err.Error(), nil)
			return req, false
		}
		req.SourceID = r.FormValue("source_id")
		req.SourceName = r.FormValue("source_name")

		file, _, err := r.FormFile("file")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, CodeValidation, "a file field is required", nil)
			return req, false
		}
		defer file.Close()
		payload, err := io.ReadAll(io.LimitReader(file, s.maxUploadBytes))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, CodeParse, "cannot read uploaded file: "+err.Error(), nil)
			return req, false
		}
		req.Content = string(payload)

	case strings.HasPrefix(contentType, "application/json"):
		if !decodeJSON(w, r, &req, s.maxUploadBytes) {
			return req, false
		}

	default:
		payload, err := io.ReadAll(io.LimitReader(r.Body, s.maxUploadBytes))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, CodeParse, "cannot read request body: "+err.Error(), nil)
			return req, false
		}
		req.Content = string(payload)
		req.SourceName = queryString(r, "source_name")
		req.SourceID = queryString(r, "source_id")
	}
	return req, true
}

func (s *Server) importDifyDSL(w http.ResponseWriter, r *http.Request) {
	req, ok := s.readImportRequest(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeAPIError(w, http.StatusBadRequest, CodeValidation, "a Dify DSL document is required", nil)
		return
	}
	result, err := s.engine.ImportDifyDSL(r.Context(), engine.ImportDifyDSLRequest{
		SourceID:   req.SourceID,
		SourceName: req.SourceName,
		Payload:    []byte(req.Content),
	})
	s.writeImportResult(w, result, err)
}

func (s *Server) importManual(w http.ResponseWriter, r *http.Request) {
	req, ok := s.readImportRequest(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeAPIError(w, http.StatusBadRequest, CodeValidation, "a capability YAML document is required", nil)
		return
	}
	result, err := s.engine.ImportManualYAML(r.Context(), engine.ImportManualYAMLRequest{
		SourceID:   req.SourceID,
		SourceName: req.SourceName,
		Payload:    []byte(req.Content),
	})
	s.writeImportResult(w, result, err)
}

func (s *Server) importMockMCP(w http.ResponseWriter, r *http.Request) {
	req, ok := s.readImportRequest(w, r)
	if !ok {
		return
	}
	result, err := s.engine.ImportMockMCP(r.Context(), engine.ImportMockRequest{
		SourceID:   req.SourceID,
		SourceName: req.SourceName,
		Payload:    []byte(req.Content),
		Options:    req.Options,
	})
	s.writeImportResult(w, result, err)
}

func (s *Server) importMockData(w http.ResponseWriter, r *http.Request) {
	req, ok := s.readImportRequest(w, r)
	if !ok {
		return
	}
	result, err := s.engine.ImportMockData(r.Context(), engine.ImportMockRequest{
		SourceID:   req.SourceID,
		SourceName: req.SourceName,
		Payload:    []byte(req.Content),
		Options:    req.Options,
	})
	s.writeImportResult(w, result, err)
}

// writeImportResult reports a parse failure as a client error rather than a server one:
// a malformed upload is the caller's problem to fix.
func (s *Server) writeImportResult(w http.ResponseWriter, result *engine.ImportResult, err error) {
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, CodeParse, err.Error(), nil)
		return
	}
	writeData(w, http.StatusOK, result)
}
