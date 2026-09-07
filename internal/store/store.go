// Package store defines the persistence interfaces used by the registry services,
// together with a Postgres implementation and an in-memory implementation. Every
// service depends on the interfaces only, so the whole system runs and is tested
// without a database.
package store

import (
	"context"
	"errors"

	"github.com/open-fin/agent-composer/internal/domain"
)

// Sentinel errors the API layer maps onto HTTP status codes.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Page is the pagination envelope shared by every list endpoint.
type Page struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// DefaultLimit and MaxLimit bound list responses.
const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Normalize clamps caller-supplied paging into the supported range.
func (p *Page) Normalize() {
	if p.Limit <= 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
}

// CandidateFilter narrows a candidate listing.
type CandidateFilter struct {
	Status   string
	Type     string
	SourceID string
	Query    string
	Limit    int
	Offset   int
}

// CapabilityFilter narrows a capability listing.
type CapabilityFilter struct {
	Type   string
	Status string
	Domain string
	Query  string
	Limit  int
	Offset int
}

// DraftFilter narrows a draft listing.
type DraftFilter struct {
	Status string
	Query  string
	Limit  int
	Offset int
}

// SourceRepo persists configured external systems.
type SourceRepo interface {
	Create(ctx context.Context, src domain.ExternalSource) (domain.ExternalSource, error)
	Get(ctx context.Context, id string) (domain.ExternalSource, error)
	List(ctx context.Context) ([]domain.ExternalSource, error)
	// EnsureByName returns the existing source with this name and type, creating it if
	// absent. Imports call it so an upload never needs a source to be configured first.
	EnsureByName(ctx context.Context, src domain.ExternalSource) (domain.ExternalSource, error)
}

// CandidateRepo persists extracted, pre-registration capability candidates.
type CandidateRepo interface {
	// Upsert writes a candidate keyed on (source_id, external_id), so re-importing the
	// same DSL refreshes candidates instead of duplicating them.
	Upsert(ctx context.Context, c domain.CapabilityCandidate) (domain.CapabilityCandidate, error)
	Get(ctx context.Context, id string) (domain.CapabilityCandidate, error)
	List(ctx context.Context, f CandidateFilter) ([]domain.CapabilityCandidate, Page, error)
	Update(ctx context.Context, c domain.CapabilityCandidate) (domain.CapabilityCandidate, error)
}

// CapabilityRepo persists the registry itself.
type CapabilityRepo interface {
	Create(ctx context.Context, c domain.Capability) (domain.Capability, error)
	Get(ctx context.Context, id string) (domain.Capability, error)
	// GetBySlug resolves the stable identifier used by composition YAML.
	GetBySlug(ctx context.Context, capType domain.CapabilityType, slug string) (domain.Capability, error)
	// FindByExternalID resolves a dependency reference emitted during extraction.
	FindByExternalID(ctx context.Context, sourceSystem, externalID string) (domain.Capability, error)
	List(ctx context.Context, f CapabilityFilter) ([]domain.Capability, Page, error)
	Update(ctx context.Context, c domain.Capability) (domain.Capability, error)
}

// DependencyRepo persists capability-to-capability edges.
type DependencyRepo interface {
	Replace(ctx context.Context, capabilityID string, deps []domain.Dependency) error
	ListFor(ctx context.Context, capabilityID string) ([]domain.Dependency, error)
}

// DraftRepo persists business agent drafts and their authoritative components.
type DraftRepo interface {
	Create(ctx context.Context, d domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error)
	Get(ctx context.Context, id string) (domain.BusinessAgentDraft, error)
	List(ctx context.Context, f DraftFilter) ([]domain.BusinessAgentDraft, Page, error)
	Update(ctx context.Context, d domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error)
}

// Store aggregates every repository behind one handle.
type Store interface {
	Sources() SourceRepo
	Candidates() CandidateRepo
	Capabilities() CapabilityRepo
	Dependencies() DependencyRepo
	Drafts() DraftRepo
	Ping(ctx context.Context) error
	Close()
}
