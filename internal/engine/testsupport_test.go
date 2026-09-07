package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

// examplesDir is the bundled demo asset directory, relative to this package.
const examplesDir = "../../examples"

// RMGoal is the business goal used throughout the demo walkthrough and the golden test.
const RMGoal = "Create an RM campaign agent that analyzes customer profile, recommends " +
	"suitable products, drafts a campaign message, and checks compliance before outreach."

func newTestEngine(t *testing.T) (*engine.Engine, *store.Memory) {
	t.Helper()
	memory := store.NewMemory()
	return engine.New(engine.Options{Store: memory}), memory
}

func readExample(t *testing.T, parts ...string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(append([]string{examplesDir}, parts...)...))
	if err != nil {
		t.Fatalf("read example %v: %v", parts, err)
	}
	return payload
}

// seedDemoRegistry replays the demo walkthrough: import the two Dify apps and the two
// manually registered capabilities, then register every candidate they produce.
func seedDemoRegistry(t *testing.T, e *engine.Engine) {
	t.Helper()
	ctx := context.Background()

	for _, file := range []string{"customer-insight-agent.dsl.yaml", "product-recommendation-workflow.dsl.yaml"} {
		if _, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
			SourceName: "customer-dify",
			Payload:    readExample(t, "dify", file),
		}); err != nil {
			t.Fatalf("import %s: %v", file, err)
		}
	}
	for _, file := range []string{"compliance-check-skill.yaml", "campaign-message-template.yaml"} {
		if _, err := e.ImportManualYAML(ctx, engine.ImportManualYAMLRequest{
			SourceName: "manual-registration",
			Payload:    readExample(t, "capabilities", file),
		}); err != nil {
			t.Fatalf("import %s: %v", file, err)
		}
	}
	registerAllPending(t, e)
}

// registerAllPending promotes every reviewable candidate into the registry. Tools and
// datasets go first so that the agents and skills depending on them resolve their
// bindings immediately rather than through the relink pass.
func registerAllPending(t *testing.T, e *engine.Engine) {
	t.Helper()
	ctx := context.Background()

	order := []domain.CapabilityType{
		domain.CapabilityTypeTool,
		domain.CapabilityTypeKnowledgeData,
		domain.CapabilityTypePromptTemplate,
		domain.CapabilityTypeSkill,
		domain.CapabilityTypeWorkflow,
		domain.CapabilityTypeAgent,
	}
	for _, capType := range order {
		candidates, _, err := e.Candidates().List(ctx, store.CandidateFilter{
			Type:  string(capType),
			Limit: store.MaxLimit,
		})
		if err != nil {
			t.Fatalf("list candidates: %v", err)
		}
		for _, candidate := range candidates {
			if !candidate.Status.Pending() {
				continue
			}
			if _, err := e.RegisterCandidate(ctx, candidate.ID); err != nil {
				t.Fatalf("register %s (%s): %v", candidate.Name, candidate.CandidateType, err)
			}
		}
	}
}

func slugsIn(refs []domain.CapabilityRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Slug)
	}
	return out
}
