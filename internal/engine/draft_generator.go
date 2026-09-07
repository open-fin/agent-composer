package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/pkg/spec"
)

// GenerateDraft turns a validated composition plan into a business agent draft.
//
// The draft is the phase 1 deliverable: it is never published, deployed or executed.
// Its status ends at ready_for_eval, which is the handoff point to the evaluation
// system rather than to a runtime.
func (e *Engine) GenerateDraft(ctx context.Context, req GenerateDraftRequest) (*domain.BusinessAgentDraft, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: draft name is required", ErrValidation)
	}
	harness := req.HarnessType
	if harness == "" {
		harness = req.Plan.Goal.HarnessType
	}
	if harness == "" {
		harness = domain.HarnessJiuwenSwarm
	}
	if !harness.Valid() {
		return nil, fmt.Errorf("%w: unknown harness type %q", ErrValidation, harness)
	}

	validation, err := e.ValidateComposition(ctx, req.Plan)
	if err != nil {
		return nil, err
	}
	if !validation.Valid {
		return nil, fmt.Errorf("%w: composition is not valid: %s", ErrValidation, summarize(validation.Errors))
	}

	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		goal = req.Plan.Goal.Goal
	}

	plan := req.Plan
	plan.Goal.Name = name
	plan.Goal.Goal = goal
	plan.Goal.HarnessType = harness

	draft := domain.BusinessAgentDraft{
		Slug:        domain.Slugify(name),
		Name:        name,
		Goal:        goal,
		Description: strings.TrimSpace(req.Description),
		HarnessType: harness,
		Status:      draftStatusFor(plan),
		Composition: plan,
		Governance:  plan.Governance,
	}
	draft.ID = domain.NewID(draft.Slug)

	yamlText, err := RenderDraft(draft)
	if err != nil {
		return nil, err
	}
	draft.YAMLText = yamlText

	created, err := e.drafts.Create(ctx, draft, componentsFor(plan))
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// draftStatusFor decides where a freshly generated draft lands. A draft with gaps or
// blocking findings is not ready for evaluation and should not pretend to be.
func draftStatusFor(plan domain.CompositionPlan) domain.DraftStatus {
	if len(plan.MissingCapabilities) > 0 {
		return domain.DraftStatusNeedsReview
	}
	for _, warning := range plan.Warnings {
		if warning.Severity == domain.SeverityError {
			return domain.DraftStatusNeedsReview
		}
	}
	return domain.DraftStatusDraft
}

// componentsFor flattens a plan into the authoritative component rows, in the fixed
// section order that YAML rendering also uses.
func componentsFor(plan domain.CompositionPlan) []domain.CompositionComponent {
	components := []domain.CompositionComponent{}
	index := 0
	for _, role := range domain.AllComponentRoles {
		for _, ref := range plan.ByRole(role) {
			config := map[string]any{}
			if ref.Role != "" {
				config["step"] = ref.Role
			}
			if ref.Reason != "" {
				config["reason"] = ref.Reason
			}
			components = append(components, domain.CompositionComponent{
				CapabilityID:   ref.ID,
				Role:           role,
				OrderIndex:     index,
				Config:         config,
				CapabilitySlug: ref.Slug,
				CapabilityName: ref.Name,
				CapabilityType: ref.Type,
			})
			index++
		}
	}
	return components
}

// RenderDraft renders a draft as the exportable YAML document.
func RenderDraft(draft domain.BusinessAgentDraft) (string, error) {
	plan := draft.Composition

	composition := spec.Composition{
		Agents:        slugsOf(plan.RecommendedAgents),
		Workflows:     slugsOf(plan.RecommendedWorkflows),
		Skills:        slugsOf(plan.RecommendedSkills),
		Tools:         slugsOf(plan.RecommendedTools),
		KnowledgeData: slugsOf(plan.RecommendedKnowledgeData),
		Prompts:       slugsOf(plan.RecommendedPrompts),
		Policies:      slugsOf(plan.RecommendedPolicies),
	}

	missing := make([]spec.MissingCapability, 0, len(plan.MissingCapabilities))
	for _, gap := range plan.MissingCapabilities {
		missing = append(missing, spec.MissingCapability{
			Step:       gap.Step,
			Type:       string(gap.RequiredType),
			Suggestion: gap.Suggestion,
		})
	}

	document := spec.BusinessAgentDocument{
		BusinessAgent: spec.BusinessAgent{
			ID:          draft.Slug,
			Name:        draft.Name,
			Goal:        draft.Goal,
			Description: draft.Description,
			Status:      string(draft.Status),
			Harness: spec.Harness{
				Type: string(draft.HarnessType),
				Mode: draft.HarnessType.Mode(),
			},
			Composition: composition,
			Workflow:    plan.WorkflowSteps,
			Governance: spec.Governance{
				RiskLevel:        string(draft.Governance.RiskLevel),
				ApprovalRequired: draft.Governance.ApprovalRequired,
				Permissions:      draft.Governance.Permissions,
			},
			MissingCapabilities: missing,
		},
	}
	return spec.Render(document)
}

// RenderDraftYAML re-renders a stored draft, so the exported YAML always reflects the
// draft's current state rather than a stale snapshot.
func (e *Engine) RenderDraftYAML(ctx context.Context, draftID string) (string, error) {
	draft, err := e.drafts.Get(ctx, draftID)
	if err != nil {
		return "", err
	}
	return RenderDraft(draft)
}

// slugsOf projects capability references onto the slugs YAML references them by. The
// draft document deliberately carries slugs, not storage ids: `crm-query` is what an
// operator recognises, and it stays stable across environments.
func slugsOf(refs []domain.CapabilityRef) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Slug)
	}
	return out
}

func summarize(warnings []domain.Warning) string {
	messages := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		messages = append(messages, warning.Message)
	}
	return strings.Join(messages, "; ")
}
