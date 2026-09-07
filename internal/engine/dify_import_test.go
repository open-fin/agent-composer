package engine_test

import (
	"context"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
)

// TestImportDifyDSLFansOutOneAppIntoSixCapabilities pins the extraction contract: a
// single Dify application is not one capability, and the split is what makes its parts
// reusable in a different composition later.
func TestImportDifyDSLFansOutOneAppIntoSixCapabilities(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	result, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "customer-insight-agent.dsl.yaml"),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	want := map[string]domain.CapabilityType{
		"customer-insight-agent":    domain.CapabilityTypeAgent,
		"customer-insight-workflow": domain.CapabilityTypeWorkflow,
		"crm-query":                 domain.CapabilityTypeTool,
		"customer-profile-kb":       domain.CapabilityTypeKnowledgeData,
		"customer-insight-prompt":   domain.CapabilityTypePromptTemplate,
		"customer-insight":          domain.CapabilityTypeSkill,
	}
	if len(result.Candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(result.Candidates), len(want), candidateIDs(result.Candidates))
	}
	for _, candidate := range result.Candidates {
		expected, ok := want[candidate.ExternalID]
		if !ok {
			t.Errorf("unexpected candidate %q", candidate.ExternalID)
			continue
		}
		if candidate.CandidateType != expected {
			t.Errorf("%s: got type %s, want %s", candidate.ExternalID, candidate.CandidateType, expected)
		}
	}
}

// TestInferredSkillIsMarkedInferred checks that provenance is recorded. The skill is
// the one capability the DSL never states; it must not claim to have been extracted.
func TestInferredSkillIsMarkedInferred(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	result, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "customer-insight-agent.dsl.yaml"),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	for _, candidate := range result.Candidates {
		if candidate.ExternalID != "customer-insight" {
			continue
		}
		if candidate.Status != domain.CandidateStatusInferred {
			t.Fatalf("skill status = %s, want %s", candidate.Status, domain.CandidateStatusInferred)
		}
		if candidate.RawPayload["inferred_from"] != "llm node" {
			t.Fatalf("skill should record what it was inferred from, got %v", candidate.RawPayload)
		}
		return
	}
	t.Fatal("skill candidate customer-insight was not produced")
}

// TestWorkflowModeAppYieldsNoAgent verifies that a non-conversational Dify app does not
// invent an agent that does not exist.
func TestWorkflowModeAppYieldsNoAgent(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	result, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "product-recommendation-workflow.dsl.yaml"),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	for _, candidate := range result.Candidates {
		if candidate.CandidateType == domain.CapabilityTypeAgent {
			t.Fatalf("workflow-mode app produced an agent candidate: %s", candidate.ExternalID)
		}
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a warning explaining why no agent was extracted")
	}
}

// TestReimportIsIdempotent covers the natural key on (source_id, external_id): a second
// upload of the same file must refresh candidates, not double them.
func TestReimportIsIdempotent(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	payload := readExample(t, "dify", "customer-insight-agent.dsl.yaml")

	first, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{SourceName: "customer-dify", Payload: payload})
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	second, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{SourceName: "customer-dify", Payload: payload})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(first.Candidates) != len(second.Candidates) {
		t.Fatalf("re-import changed candidate count: %d then %d", len(first.Candidates), len(second.Candidates))
	}
	if first.Source.ID != second.Source.ID {
		t.Fatalf("re-import created a second source: %s then %s", first.Source.ID, second.Source.ID)
	}
	for i := range first.Candidates {
		if first.Candidates[i].ID != second.Candidates[i].ID {
			t.Fatalf("candidate %s changed id on re-import", first.Candidates[i].ExternalID)
		}
	}
}

// TestParseRejectsNonDifyDocument keeps a wrong file from silently importing nothing.
func TestParseRejectsNonDifyDocument(t *testing.T) {
	e, _ := newTestEngine(t)
	_, err := e.ImportDifyDSL(context.Background(), engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    []byte("name: not a dify export\ntype: tool\n"),
	})
	if err == nil {
		t.Fatal("expected an error for a document that is not a Dify export")
	}
}

func candidateIDs(candidates []domain.CapabilityCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.ExternalID)
	}
	return out
}
