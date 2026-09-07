package api

import (
	"net/http"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

func (s *Server) listCapabilities(w http.ResponseWriter, r *http.Request) {
	capabilities, page, err := s.engine.Capabilities().List(r.Context(), store.CapabilityFilter{
		Type:   queryString(r, "type"),
		Status: domain.NormalizeStatusFilter(queryString(r, "status")),
		Domain: queryString(r, "business_domain"),
		Query:  queryString(r, "q"),
		Limit:  queryInt(r, "limit", store.DefaultLimit),
		Offset: queryInt(r, "offset", 0),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, capabilities, page)
}

func (s *Server) getCapability(w http.ResponseWriter, r *http.Request) {
	capability, err := s.engine.Capabilities().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, capability)
}

func (s *Server) getCapabilityDependencies(w http.ResponseWriter, r *http.Request) {
	dependencies, err := s.engine.Capabilities().Dependencies(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, dependencies)
}

// capabilityRequest is the body of POST and PUT /api/v1/capabilities.
type capabilityRequest struct {
	Slug           string                  `json:"slug"`
	Name           string                  `json:"name"`
	Type           domain.CapabilityType   `json:"type"`
	Subtype        string                  `json:"subtype"`
	Description    string                  `json:"description"`
	SourceSystem   string                  `json:"source_system"`
	ExternalID     string                  `json:"external_id"`
	BusinessDomain string                  `json:"business_domain"`
	Intents        []string                `json:"intents"`
	Tags           []string                `json:"tags"`
	InputSchema    map[string]any          `json:"input_schema"`
	OutputSchema   map[string]any          `json:"output_schema"`
	Owner          string                  `json:"owner"`
	Permissions    []string                `json:"permissions"`
	RiskLevel      domain.RiskLevel        `json:"risk_level"`
	Reusable       *bool                   `json:"reusable"`
	Status         domain.CapabilityStatus `json:"status"`
	Metadata       map[string]any          `json:"metadata"`
	DependsOn      []domain.DependencyRef  `json:"depends_on"`
}

func (req capabilityRequest) toCapability() domain.Capability {
	fields := engine.NormalizeFields(domain.CapabilityFields{
		Name:           req.Name,
		Description:    req.Description,
		Subtype:        req.Subtype,
		BusinessDomain: req.BusinessDomain,
		Intents:        req.Intents,
		Tags:           req.Tags,
		InputSchema:    req.InputSchema,
		OutputSchema:   req.OutputSchema,
		Owner:          req.Owner,
		Permissions:    req.Permissions,
		RiskLevel:      req.RiskLevel,
		Metadata:       req.Metadata,
	})
	reusable := true
	if req.Reusable != nil {
		reusable = *req.Reusable
	}
	slug := req.Slug
	if slug == "" {
		slug = engine.SlugFor(req.ExternalID, fields.Name)
	}
	status := req.Status
	if status == "" {
		status = domain.CapabilityStatusActive
	}
	return domain.Capability{
		Slug:           slug,
		Name:           fields.Name,
		Type:           req.Type,
		Subtype:        fields.Subtype,
		Description:    fields.Description,
		SourceSystem:   req.SourceSystem,
		ExternalID:     req.ExternalID,
		BusinessDomain: fields.BusinessDomain,
		Intents:        fields.Intents,
		Tags:           fields.Tags,
		InputSchema:    fields.InputSchema,
		OutputSchema:   fields.OutputSchema,
		Owner:          fields.Owner,
		Permissions:    fields.Permissions,
		RiskLevel:      fields.RiskLevel,
		Reusable:       reusable,
		Status:         status,
		Metadata:       fields.Metadata,
	}
}

func (s *Server) createCapability(w http.ResponseWriter, r *http.Request) {
	var req capabilityRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	capability := req.toCapability()
	if capability.SourceSystem == "" {
		capability.SourceSystem = "api"
	}
	capability.ID = domain.NewID(capability.Slug)

	created, err := s.engine.Capabilities().Create(r.Context(), capability, req.DependsOn)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, created)
}

func (s *Server) updateCapability(w http.ResponseWriter, r *http.Request) {
	var req capabilityRequest
	if !decodeJSON(w, r, &req, s.maxUploadBytes) {
		return
	}
	existing, err := s.engine.Capabilities().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	updated := req.toCapability()
	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	if updated.SourceSystem == "" {
		updated.SourceSystem = existing.SourceSystem
	}
	if updated.ExternalID == "" {
		updated.ExternalID = existing.ExternalID
	}

	saved, err := s.engine.Capabilities().Update(r.Context(), updated)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, saved)
}
