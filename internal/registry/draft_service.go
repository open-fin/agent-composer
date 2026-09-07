package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// DraftService owns business agent drafts.
type DraftService struct {
	repo store.DraftRepo
}

// NewDraftService builds the draft service.
func NewDraftService(repo store.DraftRepo) *DraftService { return &DraftService{repo: repo} }

// Create stores a draft together with its authoritative composition components.
func (s *DraftService) Create(ctx context.Context, draft domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	if strings.TrimSpace(draft.Name) == "" {
		return domain.BusinessAgentDraft{}, fmt.Errorf("%w: draft name is required", ErrValidation)
	}
	if !draft.HarnessType.Valid() {
		return domain.BusinessAgentDraft{}, fmt.Errorf("%w: unknown harness type %q", ErrValidation, draft.HarnessType)
	}
	if draft.Status == "" {
		draft.Status = domain.DraftStatusDraft
	}
	if !draft.Status.Valid() {
		return domain.BusinessAgentDraft{}, fmt.Errorf("%w: unknown draft status %q", ErrValidation, draft.Status)
	}
	return s.repo.Create(ctx, draft, components)
}

// Get returns one draft with its components hydrated.
func (s *DraftService) Get(ctx context.Context, id string) (domain.BusinessAgentDraft, error) {
	return s.repo.Get(ctx, id)
}

// List returns a filtered page of drafts.
func (s *DraftService) List(ctx context.Context, f store.DraftFilter) ([]domain.BusinessAgentDraft, store.Page, error) {
	return s.repo.List(ctx, f)
}

// Update applies the mutable subset of a draft. Composition changes go through the
// engine, which regenerates the components, the snapshot and the YAML together.
func (s *DraftService) Update(ctx context.Context, id string, update domain.DraftUpdate) (domain.BusinessAgentDraft, error) {
	draft, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	if update.Name != nil {
		if strings.TrimSpace(*update.Name) == "" {
			return domain.BusinessAgentDraft{}, fmt.Errorf("%w: draft name must not be empty", ErrValidation)
		}
		draft.Name = *update.Name
	}
	if update.Description != nil {
		draft.Description = *update.Description
	}
	if update.Goal != nil {
		draft.Goal = *update.Goal
	}
	if update.HarnessType != nil {
		if !update.HarnessType.Valid() {
			return domain.BusinessAgentDraft{}, fmt.Errorf("%w: unknown harness type %q", ErrValidation, *update.HarnessType)
		}
		draft.HarnessType = *update.HarnessType
	}
	if update.Status != nil {
		if !update.Status.Valid() {
			return domain.BusinessAgentDraft{}, fmt.Errorf("%w: unknown draft status %q", ErrValidation, *update.Status)
		}
		draft.Status = *update.Status
	}
	// A nil component slice tells the repository to leave the composition alone.
	return s.repo.Update(ctx, draft, nil)
}

// Save persists a fully regenerated draft, components included.
func (s *DraftService) Save(ctx context.Context, draft domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	return s.repo.Update(ctx, draft, components)
}
