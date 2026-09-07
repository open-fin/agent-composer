package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/open-fin/agent-composer/internal/domain"
)

// CandidateContext is what the enricher sees about an extracted candidate.
type CandidateContext struct {
	Type         domain.CapabilityType `json:"type"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	SourceSystem string                `json:"source_system"`
	ExternalID   string                `json:"external_id"`
	Hints        []string              `json:"hints,omitempty"`
}

// Enricher fills in the fields a source system does not carry (business domain,
// intents, tags, risk) and decomposes a business goal into the roles a composition
// must cover.
//
// Goal decomposition lives here rather than in the matcher on purpose: matching can
// only return capabilities that exist, so the set of *required* roles has to be
// produced independently for missing capabilities to be derivable at all.
type Enricher interface {
	Name() string
	EnrichCandidate(ctx context.Context, in CandidateContext) (domain.CapabilityFields, error)
	DecomposeGoal(ctx context.Context, goal domain.BusinessGoal) ([]domain.RequiredRole, error)
}

// ClientEnricher asks a real model, and silently falls back to the deterministic mock
// whenever the model is unreachable, slow, or returns something unparseable. A broken
// endpoint degrades the output; it never breaks the demo.
type ClientEnricher struct {
	client   Client
	fallback Enricher
	log      *slog.Logger
}

// NewClientEnricher wraps an LLM client with mock fallback.
func NewClientEnricher(client Client, log *slog.Logger) *ClientEnricher {
	if log == nil {
		log = slog.Default()
	}
	return &ClientEnricher{client: client, fallback: NewMockEnricher(), log: log}
}

func (e *ClientEnricher) Name() string { return e.client.Name() }

func (e *ClientEnricher) EnrichCandidate(ctx context.Context, in CandidateContext) (domain.CapabilityFields, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return e.fallback.EnrichCandidate(ctx, in)
	}
	raw, err := e.client.Complete(ctx, Request{
		System:     enrichCandidateSystemPrompt,
		User:       fmt.Sprintf(enrichCandidateUserPrompt, string(payload)),
		JSONObject: true,
		MaxTokens:  800,
	})
	if err != nil {
		e.log.Warn("llm enrichment failed, using mock", "error", err, "candidate", in.Name)
		return e.fallback.EnrichCandidate(ctx, in)
	}
	object, err := extractJSONObject(raw)
	if err != nil {
		e.log.Warn("llm enrichment returned no json, using mock", "error", err, "candidate", in.Name)
		return e.fallback.EnrichCandidate(ctx, in)
	}
	var fields domain.CapabilityFields
	if err := json.Unmarshal([]byte(object), &fields); err != nil {
		e.log.Warn("llm enrichment json invalid, using mock", "error", err, "candidate", in.Name)
		return e.fallback.EnrichCandidate(ctx, in)
	}
	// The mock supplies any field the model left blank, so enrichment is never a
	// downgrade on completeness.
	base, err := e.fallback.EnrichCandidate(ctx, in)
	if err != nil {
		return fields, nil
	}
	return base.Merge(fields), nil
}

func (e *ClientEnricher) DecomposeGoal(ctx context.Context, goal domain.BusinessGoal) ([]domain.RequiredRole, error) {
	raw, err := e.client.Complete(ctx, Request{
		System:     decomposeGoalSystemPrompt,
		User:       fmt.Sprintf(decomposeGoalUserPrompt, goal.Goal, capabilityTypeList()),
		JSONObject: true,
		MaxTokens:  900,
	})
	if err != nil {
		e.log.Warn("llm goal decomposition failed, using mock", "error", err)
		return e.fallback.DecomposeGoal(ctx, goal)
	}
	object, err := extractJSONObject(raw)
	if err != nil {
		e.log.Warn("llm goal decomposition returned no json, using mock", "error", err)
		return e.fallback.DecomposeGoal(ctx, goal)
	}
	var decoded struct {
		Roles []domain.RequiredRole `json:"roles"`
	}
	if err := json.Unmarshal([]byte(object), &decoded); err != nil || len(decoded.Roles) == 0 {
		e.log.Warn("llm goal decomposition unusable, using mock", "error", err)
		return e.fallback.DecomposeGoal(ctx, goal)
	}
	valid := make([]domain.RequiredRole, 0, len(decoded.Roles))
	for _, role := range decoded.Roles {
		if role.Step == "" || !role.CapabilityType.Valid() {
			continue
		}
		valid = append(valid, role)
	}
	if len(valid) == 0 {
		return e.fallback.DecomposeGoal(ctx, goal)
	}
	return valid, nil
}
