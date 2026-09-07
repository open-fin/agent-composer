package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// CapabilityService owns the capability registry.
type CapabilityService struct {
	repo store.CapabilityRepo
	deps *DependencyService
}

// NewCapabilityService builds the capability service.
func NewCapabilityService(repo store.CapabilityRepo, deps *DependencyService) *CapabilityService {
	return &CapabilityService{repo: repo, deps: deps}
}

// Create registers a capability directly, without going through a candidate.
func (s *CapabilityService) Create(ctx context.Context, capability domain.Capability, refs []domain.DependencyRef) (domain.Capability, error) {
	if err := validateCapability(capability); err != nil {
		return domain.Capability{}, err
	}
	if capability.Status == "" {
		capability.Status = domain.CapabilityStatusActive
	}

	// The (type, slug) uniqueness check is what stops the same CRM tool being
	// registered twice when it arrives from a Dify DSL and again from a manual YAML.
	if existing, err := s.repo.GetBySlug(ctx, capability.Type, capability.Slug); err == nil {
		return domain.Capability{}, fmt.Errorf("%w: %s/%s is already registered as %s",
			ErrConflict, capability.Type, capability.Slug, existing.ID)
	} else if !errors.Is(err, store.ErrNotFound) {
		return domain.Capability{}, err
	}

	created, err := s.repo.Create(ctx, capability)
	if err != nil {
		return domain.Capability{}, err
	}
	if err := s.applyDependencies(ctx, &created, refs); err != nil {
		return domain.Capability{}, err
	}
	return created, nil
}

// applyDependencies resolves and stores a capability's edges, recording anything that
// could not be resolved on the capability itself so validation can warn about it.
func (s *CapabilityService) applyDependencies(ctx context.Context, capability *domain.Capability, refs []domain.DependencyRef) error {
	if s.deps == nil || len(refs) == 0 {
		return nil
	}
	resolved, err := s.deps.Resolve(ctx, capability.ID, capability.SourceSystem, refs)
	if err != nil {
		return err
	}
	if err := s.deps.Store(ctx, capability.ID, resolved.Resolved); err != nil {
		return err
	}
	capability.Dependencies = resolved.Resolved

	if len(resolved.Unresolved) > 0 {
		if capability.Metadata == nil {
			capability.Metadata = map[string]any{}
		}
		pending := make([]any, 0, len(resolved.Unresolved))
		for _, ref := range resolved.Unresolved {
			pending = append(pending, map[string]any{
				"external_id":     ref.ExternalID,
				"target_type":     string(ref.TargetType),
				"dependency_type": string(ref.DependencyType),
			})
		}
		capability.Metadata["unresolved_dependencies"] = pending
		updated, err := s.repo.Update(ctx, *capability)
		if err != nil {
			return err
		}
		updated.Dependencies = resolved.Resolved
		*capability = updated
	}
	return nil
}

// RelinkPending retries the unresolved references recorded on every capability. It is
// called after each registration, so a Dify app registered before its tools ends up
// correctly linked once those tools arrive.
func (s *CapabilityService) RelinkPending(ctx context.Context) error {
	all, _, err := s.repo.List(ctx, store.CapabilityFilter{Limit: store.MaxLimit})
	if err != nil {
		return err
	}
	for _, capability := range all {
		pending, ok := capability.Metadata["unresolved_dependencies"]
		if !ok {
			continue
		}
		refs := decodeRefs(pending)
		if len(refs) == 0 {
			continue
		}
		resolved, err := s.deps.Resolve(ctx, capability.ID, capability.SourceSystem, refs)
		if err != nil {
			return err
		}
		if len(resolved.Resolved) == 0 {
			continue
		}
		existing, err := s.deps.ListFor(ctx, capability.ID)
		if err != nil {
			return err
		}
		if err := s.deps.Store(ctx, capability.ID, append(existing, resolved.Resolved...)); err != nil {
			return err
		}
		if len(resolved.Unresolved) == 0 {
			delete(capability.Metadata, "unresolved_dependencies")
		} else {
			pendingOut := make([]any, 0, len(resolved.Unresolved))
			for _, ref := range resolved.Unresolved {
				pendingOut = append(pendingOut, map[string]any{
					"external_id":     ref.ExternalID,
					"target_type":     string(ref.TargetType),
					"dependency_type": string(ref.DependencyType),
				})
			}
			capability.Metadata["unresolved_dependencies"] = pendingOut
		}
		if _, err := s.repo.Update(ctx, capability); err != nil {
			return err
		}
	}
	return nil
}

func decodeRefs(raw any) []domain.DependencyRef {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]domain.DependencyRef, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		externalID, _ := entry["external_id"].(string)
		if externalID == "" {
			continue
		}
		targetType, _ := entry["target_type"].(string)
		depType, _ := entry["dependency_type"].(string)
		out = append(out, domain.DependencyRef{
			ExternalID:     externalID,
			TargetType:     domain.CapabilityType(targetType),
			DependencyType: domain.DependencyType(depType),
		})
	}
	return out
}

// Get returns one capability with its dependencies hydrated.
func (s *CapabilityService) Get(ctx context.Context, id string) (domain.Capability, error) {
	capability, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Capability{}, err
	}
	if s.deps != nil {
		capability.Dependencies, err = s.deps.ListFor(ctx, id)
		if err != nil {
			return domain.Capability{}, err
		}
	}
	return capability, nil
}

// List returns a filtered page of capabilities.
func (s *CapabilityService) List(ctx context.Context, f store.CapabilityFilter) ([]domain.Capability, store.Page, error) {
	return s.repo.List(ctx, f)
}

// Update replaces a capability's mutable fields.
func (s *CapabilityService) Update(ctx context.Context, capability domain.Capability) (domain.Capability, error) {
	if err := validateCapability(capability); err != nil {
		return domain.Capability{}, err
	}
	return s.repo.Update(ctx, capability)
}

// Dependencies returns a capability's edges.
func (s *CapabilityService) Dependencies(ctx context.Context, id string) ([]domain.Dependency, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.deps.ListFor(ctx, id)
}

func validateCapability(capability domain.Capability) error {
	if capability.Name == "" {
		return fmt.Errorf("%w: capability name is required", ErrValidation)
	}
	if capability.Slug == "" {
		return fmt.Errorf("%w: capability slug is required", ErrValidation)
	}
	if !capability.Type.Valid() {
		return fmt.Errorf("%w: unknown capability type %q", ErrValidation, capability.Type)
	}
	if !capability.RiskLevel.Valid() {
		return fmt.Errorf("%w: unknown risk level %q", ErrValidation, capability.RiskLevel)
	}
	return nil
}
