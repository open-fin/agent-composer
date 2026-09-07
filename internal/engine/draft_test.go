package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/pkg/spec"
)

// generateDemoDraft replays the full walkthrough and returns the resulting draft.
func generateDemoDraft(t *testing.T, e *engine.Engine) domain.BusinessAgentDraft {
	t.Helper()
	ctx := context.Background()

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	draft, err := e.GenerateDraft(ctx, engine.GenerateDraftRequest{
		Name:        "RM Campaign Agent",
		Goal:        RMGoal,
		Description: "Generates customer campaign suggestions for relationship managers.",
		HarnessType: domain.HarnessJiuwenSwarm,
		Plan:        *plan,
	})
	if err != nil {
		t.Fatalf("generate draft: %v", err)
	}
	return *draft
}

// TestGeneratedDraftMatchesGoldenYAML is the end-of-demo assertion: what the system
// prints on screen is byte-for-byte what the repository documents it produces.
func TestGeneratedDraftMatchesGoldenYAML(t *testing.T) {
	e, _ := newTestEngine(t)
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	goldenPath := filepath.Join(examplesDir, "drafts", "rm-campaign-agent.yaml")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden draft: %v", err)
	}
	if draft.YAMLText != string(golden) {
		t.Errorf("generated draft does not match %s\n--- got ---\n%s\n--- want ---\n%s",
			goldenPath, draft.YAMLText, golden)
	}
}

// TestGeneratedYAMLParsesBack keeps the export honest: a document that cannot be read
// back is not a deliverable.
func TestGeneratedYAMLParsesBack(t *testing.T) {
	e, _ := newTestEngine(t)
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	document, err := spec.ParseBusinessAgent([]byte(draft.YAMLText))
	if err != nil {
		t.Fatalf("parse generated yaml: %v", err)
	}
	agent := document.BusinessAgent
	if agent.ID != "rm-campaign-agent" {
		t.Errorf("id = %s, want rm-campaign-agent", agent.ID)
	}
	if agent.Harness.Type != string(domain.HarnessJiuwenSwarm) {
		t.Errorf("harness type = %s", agent.Harness.Type)
	}
	if agent.Harness.Mode != "bounded_dynamic_team" {
		t.Errorf("harness mode = %s, want bounded_dynamic_team", agent.Harness.Mode)
	}
	if len(agent.Workflow) != 5 {
		t.Errorf("workflow has %d steps, want 5", len(agent.Workflow))
	}
}

// TestDraftWithGapsNeedsReview checks the status reflects reality. A composition that
// cannot deliver its own message is not ready for evaluation.
func TestDraftWithGapsNeedsReview(t *testing.T) {
	e, _ := newTestEngine(t)
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	if draft.Status != domain.DraftStatusNeedsReview {
		t.Errorf("status = %s, want needs_review while outreach_send is unsatisfied", draft.Status)
	}
	if !strings.Contains(draft.YAMLText, "missing_capabilities") {
		t.Error("the gap should be visible in the exported YAML, not hidden in the database")
	}
}

// TestDraftReferencesSlugsNotIDs pins the identifier contract: YAML carries the stable
// readable slug, never the storage id with its uniqueness suffix.
func TestDraftReferencesSlugsNotIDs(t *testing.T) {
	e, _ := newTestEngine(t)
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	for _, component := range draft.Components {
		if strings.Contains(draft.YAMLText, component.CapabilityID) {
			t.Errorf("storage id %s leaked into the exported YAML", component.CapabilityID)
		}
		if !strings.Contains(draft.YAMLText, component.CapabilitySlug) {
			t.Errorf("slug %s is missing from the exported YAML", component.CapabilitySlug)
		}
	}
}

// TestComponentsAreTheAuthoritativeComposition verifies the relational rows agree with
// the rendered document.
func TestComponentsAreTheAuthoritativeComposition(t *testing.T) {
	e, _ := newTestEngine(t)
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	stored, err := e.Drafts().Get(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	if len(stored.Components) != len(draft.Composition.AllRefs()) {
		t.Fatalf("stored %d components for %d planned capabilities",
			len(stored.Components), len(draft.Composition.AllRefs()))
	}
	for i, component := range stored.Components {
		if component.OrderIndex != i {
			t.Errorf("component %s has order %d, want %d", component.CapabilitySlug, component.OrderIndex, i)
		}
	}
}

// TestRenderDraftYAMLReflectsCurrentState checks that export re-renders rather than
// returning a stale snapshot.
func TestRenderDraftYAMLReflectsCurrentState(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)
	draft := generateDemoDraft(t, e)

	newName := "Renamed Campaign Agent"
	if _, err := e.Drafts().Update(ctx, draft.ID, domain.DraftUpdate{Name: &newName}); err != nil {
		t.Fatalf("rename draft: %v", err)
	}
	rendered, err := e.RenderDraftYAML(ctx, draft.ID)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(rendered, newName) {
		t.Error("exported YAML did not pick up the renamed draft")
	}
}

// TestGenerateDraftRejectsInvalidComposition ensures a broken plan cannot become a
// draft that looks legitimate.
func TestGenerateDraftRejectsInvalidComposition(t *testing.T) {
	e, _ := newTestEngine(t)
	_, err := e.GenerateDraft(context.Background(), engine.GenerateDraftRequest{
		Name:        "Empty Agent",
		HarnessType: domain.HarnessManual,
		Plan:        domain.CompositionPlan{},
	})
	if err == nil {
		t.Fatal("expected an error when generating a draft from an empty plan")
	}
}
