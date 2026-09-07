package registry

import (
	"context"
	"errors"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// DependencyService resolves extraction-time dependency references into stored edges.
type DependencyService struct {
	deps         store.DependencyRepo
	capabilities store.CapabilityRepo
}

// NewDependencyService builds the dependency service.
func NewDependencyService(deps store.DependencyRepo, capabilities store.CapabilityRepo) *DependencyService {
	return &DependencyService{deps: deps, capabilities: capabilities}
}

// ResolveResult reports which references became edges and which could not be resolved.
type ResolveResult struct {
	Resolved   []domain.Dependency
	Unresolved []domain.DependencyRef
}

// Resolve turns external-id references into edges between registered capabilities.
//
// A reference to something not yet registered is not an error: a Dify app is normally
// registered before the tool it binds. Unresolved references are returned so they can
// be recorded on the capability and retried the next time anything is registered.
func (s *DependencyService) Resolve(ctx context.Context, capabilityID, sourceSystem string, refs []domain.DependencyRef) (ResolveResult, error) {
	var result ResolveResult
	for _, ref := range refs {
		target, err := s.capabilities.FindByExternalID(ctx, sourceSystem, ref.ExternalID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				result.Unresolved = append(result.Unresolved, ref)
				continue
			}
			return result, err
		}
		if target.ID == capabilityID {
			continue
		}
		if ref.TargetType != "" && target.Type != ref.TargetType {
			// The external id resolved to a different kind of capability than the
			// source declared; treat it as unresolved rather than link the wrong thing.
			result.Unresolved = append(result.Unresolved, ref)
			continue
		}
		result.Resolved = append(result.Resolved, domain.Dependency{
			CapabilityID:          capabilityID,
			DependsOnCapabilityID: target.ID,
			DependencyType:        ref.DependencyType,
			Config:                ref.Config,
			DependsOnSlug:         target.Slug,
			DependsOnType:         target.Type,
			DependsOnName:         target.Name,
		})
	}
	return result, nil
}

// Store replaces a capability's outgoing edges.
func (s *DependencyService) Store(ctx context.Context, capabilityID string, deps []domain.Dependency) error {
	return s.deps.Replace(ctx, capabilityID, deps)
}

// ListFor returns a capability's hydrated dependencies.
func (s *DependencyService) ListFor(ctx context.Context, capabilityID string) ([]domain.Dependency, error) {
	return s.deps.ListFor(ctx, capabilityID)
}
