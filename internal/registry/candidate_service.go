package registry

import (
	"context"
	"fmt"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// CandidateService manages pre-registration capability candidates.
type CandidateService struct {
	repo store.CandidateRepo
}

// NewCandidateService builds the candidate service.
func NewCandidateService(repo store.CandidateRepo) *CandidateService {
	return &CandidateService{repo: repo}
}

// Upsert persists a candidate, keyed on (source_id, external_id).
func (s *CandidateService) Upsert(ctx context.Context, candidate domain.CapabilityCandidate) (domain.CapabilityCandidate, error) {
	if candidate.SourceID == "" {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate source_id is required", ErrValidation)
	}
	if candidate.ExternalID == "" {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate external_id is required", ErrValidation)
	}
	if !candidate.CandidateType.Valid() {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: unknown candidate type %q", ErrValidation, candidate.CandidateType)
	}
	if !candidate.Status.Valid() {
		candidate.Status = domain.CandidateStatusExtracted
	}
	return s.repo.Upsert(ctx, candidate)
}

// Get returns one candidate.
func (s *CandidateService) Get(ctx context.Context, id string) (domain.CapabilityCandidate, error) {
	return s.repo.Get(ctx, id)
}

// List returns a filtered page of candidates.
func (s *CandidateService) List(ctx context.Context, f store.CandidateFilter) ([]domain.CapabilityCandidate, store.Page, error) {
	return s.repo.List(ctx, f)
}

// Review applies a reviewer's edits. Edits are stored in their own field block rather
// than overwriting the extracted values, so the provenance of every field survives.
func (s *CandidateService) Review(ctx context.Context, id string, input domain.ReviewInput) (domain.CapabilityCandidate, error) {
	candidate, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	if !candidate.Status.Pending() {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate %s is already %s", ErrConflict, id, candidate.Status)
	}

	candidate.Review = candidate.Review.Merge(input.Fields)
	if input.Note != "" {
		if candidate.Review.Metadata == nil {
			candidate.Review.Metadata = map[string]any{}
		}
		candidate.Review.Metadata["review_note"] = input.Note
	}

	resolved := candidate.Resolved()
	if resolved.Name != "" {
		candidate.Name = resolved.Name
	}
	if resolved.Description != "" {
		candidate.Description = resolved.Description
	}
	// A reviewed candidate that is now complete stops asking for attention.
	if candidate.Status == domain.CandidateStatusNeedsReview &&
		resolved.Description != "" && len(resolved.Intents) > 0 {
		candidate.Status = domain.CandidateStatusInferred
	}
	return s.repo.Update(ctx, candidate)
}

// Reject marks a candidate as dismissed.
func (s *CandidateService) Reject(ctx context.Context, id, reason string) (domain.CapabilityCandidate, error) {
	candidate, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	if candidate.Status == domain.CandidateStatusRegistered {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate %s is already registered", ErrConflict, id)
	}
	if reason != "" {
		if candidate.Review.Metadata == nil {
			candidate.Review.Metadata = map[string]any{}
		}
		candidate.Review.Metadata["reject_reason"] = reason
	}
	candidate.Status = domain.CandidateStatusRejected
	return s.repo.Update(ctx, candidate)
}

// MarkRegistered records the capability a candidate was promoted into.
func (s *CandidateService) MarkRegistered(ctx context.Context, candidate domain.CapabilityCandidate, capabilityID string) (domain.CapabilityCandidate, error) {
	if candidate.Review.Metadata == nil {
		candidate.Review.Metadata = map[string]any{}
	}
	candidate.Review.Metadata["registered_capability_id"] = capabilityID
	candidate.Status = domain.CandidateStatusRegistered
	return s.repo.Update(ctx, candidate)
}
