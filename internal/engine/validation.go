package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// ValidateComposition checks a plan against the current registry.
//
// A plan can be recommended and then sit in the UI while a reviewer edits it, so the
// capabilities it names must be re-checked rather than trusted. Errors block draft
// generation; warnings do not.
func (e *Engine) ValidateComposition(ctx context.Context, plan domain.CompositionPlan) (*domain.CompositionValidationResult, error) {
	result := &domain.CompositionValidationResult{
		Errors:   []domain.Warning{},
		Warnings: []domain.Warning{},
	}

	refs := plan.AllRefs()
	if len(refs) == 0 {
		result.Errors = append(result.Errors, domain.Warning{
			Code:     "empty_composition",
			Severity: domain.SeverityError,
			Message:  "the plan selects no capabilities",
		})
	}

	present := map[string]domain.Capability{}
	for _, ref := range refs {
		capability, err := e.capabilities.Get(ctx, ref.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				result.Errors = append(result.Errors, domain.Warning{
					Code:           "capability_missing",
					Severity:       domain.SeverityError,
					Message:        fmt.Sprintf("%s is no longer registered", ref.Slug),
					CapabilitySlug: ref.Slug,
				})
				continue
			}
			return nil, err
		}
		if capability.Status != domain.CapabilityStatusActive {
			result.Errors = append(result.Errors, domain.Warning{
				Code:           "capability_deprecated",
				Severity:       domain.SeverityError,
				Message:        fmt.Sprintf("%s is %s and cannot be composed", capability.Slug, capability.Status),
				CapabilitySlug: capability.Slug,
			})
		}
		present[capability.ID] = capability
	}

	// Every tool and dataset a selected capability binds must itself be in the plan,
	// otherwise the draft describes something that cannot run.
	for _, capability := range present {
		deps, err := e.dependencies.ListFor(ctx, capability.ID)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			if !dep.DependencyType.ProvisionedAtComposition() {
				continue
			}
			if _, ok := present[dep.DependsOnCapabilityID]; ok {
				continue
			}
			result.Errors = append(result.Errors, domain.Warning{
				Code:     "dependency_not_composed",
				Severity: domain.SeverityError,
				Message: fmt.Sprintf("%s requires %s (%s), which the plan does not include",
					capability.Slug, dep.DependsOnSlug, dep.DependencyType),
				CapabilitySlug: capability.Slug,
			})
		}
	}

	if len(plan.WorkflowSteps) == 0 {
		result.Errors = append(result.Errors, domain.Warning{
			Code:     "no_workflow_steps",
			Severity: domain.SeverityError,
			Message:  "no goal step could be satisfied, so the plan has no workflow",
		})
	}

	// Something has to actually do the work.
	if len(plan.RecommendedAgents) == 0 && len(plan.RecommendedWorkflows) == 0 && len(plan.RecommendedSkills) == 0 {
		result.Errors = append(result.Errors, domain.Warning{
			Code:     "nothing_executable",
			Severity: domain.SeverityError,
			Message:  "the plan contains no agent, workflow or skill to execute",
		})
	}

	result.Warnings = append(result.Warnings, buildWarnings(&plan, present)...)
	result.Valid = len(result.Errors) == 0
	return result, nil
}
