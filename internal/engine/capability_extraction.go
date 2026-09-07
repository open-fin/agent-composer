package engine

import (
	"context"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/llm"
)

// enrichCandidates fills the fields a source system does not carry.
//
// Enrichment only ever *adds*: anything the adapter extracted stays authoritative, and
// inferred values land in the candidate's separate `inferred` block so a reviewer can
// see exactly which values the system guessed at.
func (e *Engine) enrichCandidates(ctx context.Context, candidates []domain.CapabilityCandidate) []domain.CapabilityCandidate {
	out := make([]domain.CapabilityCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		inferred, err := e.enricher.EnrichCandidate(ctx, llm.CandidateContext{
			Type:         candidate.CandidateType,
			Name:         candidate.Name,
			Description:  candidate.Description,
			SourceSystem: candidate.SourceSystem,
			ExternalID:   candidate.ExternalID,
			Hints:        candidateHints(candidate),
		})
		if err != nil {
			// Enrichment is best effort; a candidate without inferred fields is still
			// reviewable and registrable.
			e.log.Warn("capability enrichment failed", "candidate", candidate.Name, "error", err)
			out = append(out, candidate)
			continue
		}

		// Drop anything extraction already established, so `inferred` shows only what
		// the system added.
		candidate.Inferred = subtractKnown(inferred, candidate.Extracted)
		candidate.Status = statusAfterEnrichment(candidate)
		out = append(out, candidate)
	}
	return out
}

// candidateHints gives the enricher a little extra vocabulary from the raw payload.
func candidateHints(candidate domain.CapabilityCandidate) []string {
	var hints []string
	if subtype := candidate.Extracted.Subtype; subtype != "" {
		hints = append(hints, subtype)
	}
	for _, key := range []string{"provider_name", "tool_name", "catalog", "server"} {
		if value, ok := candidate.RawPayload[key].(string); ok && value != "" {
			hints = append(hints, value)
		}
	}
	return hints
}

// subtractKnown removes values the adapter already extracted.
func subtractKnown(inferred, extracted domain.CapabilityFields) domain.CapabilityFields {
	if len(extracted.Intents) > 0 {
		inferred.Intents = nil
	}
	if len(extracted.Tags) > 0 {
		inferred.Tags = nil
	}
	if extracted.BusinessDomain != "" {
		inferred.BusinessDomain = ""
	}
	if extracted.RiskLevel != "" {
		inferred.RiskLevel = ""
	}
	if extracted.Description != "" {
		inferred.Description = ""
	}
	if len(extracted.Permissions) > 0 {
		inferred.Permissions = nil
	}
	// Name and dependencies are never inferred over.
	inferred.Name = ""
	inferred.DependsOn = nil
	return inferred
}

// statusAfterEnrichment records how a candidate came to be, and flags the ones a human
// must look at. Registration is blocked on nothing; the status is guidance for review.
func statusAfterEnrichment(candidate domain.CapabilityCandidate) domain.CandidateStatus {
	if !candidate.Status.Pending() {
		return candidate.Status
	}
	resolved := candidate.Resolved()
	if resolved.Description == "" || len(resolved.Intents) == 0 {
		return domain.CandidateStatusNeedsReview
	}
	if candidate.Status == domain.CandidateStatusInferred || hasInferredValues(candidate.Inferred) {
		return domain.CandidateStatusInferred
	}
	return domain.CandidateStatusExtracted
}

func hasInferredValues(fields domain.CapabilityFields) bool {
	return fields.BusinessDomain != "" || len(fields.Intents) > 0 || len(fields.Tags) > 0 ||
		fields.RiskLevel != "" || fields.Description != "" || len(fields.Permissions) > 0
}
