// Package registry holds the service layer over the store: the business rules that
// govern sources, candidates, capabilities, dependencies and drafts.
package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// SourceService manages configured external systems.
type SourceService struct {
	repo store.SourceRepo
}

// NewSourceService builds the source service.
func NewSourceService(repo store.SourceRepo) *SourceService { return &SourceService{repo: repo} }

// Create registers a new external source.
func (s *SourceService) Create(ctx context.Context, src domain.ExternalSource) (domain.ExternalSource, error) {
	src.Name = strings.TrimSpace(src.Name)
	if src.Name == "" {
		return domain.ExternalSource{}, fmt.Errorf("%w: source name is required", ErrValidation)
	}
	if !src.Type.Valid() {
		return domain.ExternalSource{}, fmt.Errorf("%w: unknown source type %q", ErrValidation, src.Type)
	}
	if src.Config == nil {
		src.Config = map[string]any{}
	}
	if src.Status == "" {
		src.Status = "active"
	}
	return s.repo.Create(ctx, src)
}

// Get returns one source.
func (s *SourceService) Get(ctx context.Context, id string) (domain.ExternalSource, error) {
	return s.repo.Get(ctx, id)
}

// List returns every configured source.
func (s *SourceService) List(ctx context.Context) ([]domain.ExternalSource, error) {
	return s.repo.List(ctx)
}

// Ensure resolves the source an import should be attributed to, creating it on first
// use. Uploads therefore never require a source to be configured up front.
func (s *SourceService) Ensure(ctx context.Context, name string, sourceType domain.SourceType, config map[string]any) (domain.ExternalSource, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = string(sourceType) + "-upload"
	}
	if config == nil {
		config = map[string]any{}
	}
	return s.repo.EnsureByName(ctx, domain.ExternalSource{
		Name:   name,
		Type:   sourceType,
		Config: config,
		Status: "active",
	})
}
