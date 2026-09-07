package engine_test

import (
	"context"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
)

// TestValidateRejectsUnprovisionedDependency covers the check that keeps a draft
// deployable: a skill's tool cannot be silently dropped from the composition.
func TestValidateRejectsUnprovisionedDependency(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}

	// Drop the tool that the matched skill binds.
	var kept []domain.CapabilityRef
	for _, ref := range plan.RecommendedTools {
		if ref.Slug != "product-catalog-query" {
			kept = append(kept, ref)
		}
	}
	plan.RecommendedTools = kept

	result, err := e.ValidateComposition(ctx, *plan)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatal("a plan missing a bound tool must not validate")
	}
	found := false
	for _, issue := range result.Errors {
		if issue.Code == "dependency_not_composed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a dependency_not_composed error, got %+v", result.Errors)
	}
}

// TestValidateRejectsEmptyPlan covers the degenerate case.
func TestValidateRejectsEmptyPlan(t *testing.T) {
	e, _ := newTestEngine(t)
	result, err := e.ValidateComposition(context.Background(), domain.CompositionPlan{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatal("an empty plan must not validate")
	}
	codes := map[string]bool{}
	for _, issue := range result.Errors {
		codes[issue.Code] = true
	}
	for _, expected := range []string{"empty_composition", "no_workflow_steps", "nothing_executable"} {
		if !codes[expected] {
			t.Errorf("missing expected error %s, got %v", expected, codes)
		}
	}
}

// TestValidateAcceptsTheDemoPlan is the positive case.
func TestValidateAcceptsTheDemoPlan(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	result, err := e.ValidateComposition(ctx, *plan)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("the demo plan must validate, errors: %+v", result.Errors)
	}
	// A gap is a warning, not a blocking error.
	if len(result.Warnings) == 0 {
		t.Error("expected the outreach gap to appear as a warning")
	}
}
