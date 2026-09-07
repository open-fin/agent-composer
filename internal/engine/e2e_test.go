package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

// TestDemoWalkthrough drives the entire README walkthrough through the engine: import,
// review, register, catalog, recommend, validate, generate, export. It is the
// acceptance test for criteria 2 through 8, and needs no HTTP server or database.
func TestDemoWalkthrough(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	// 1-2. Upload the Dify customer insight agent.
	insight, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "customer-insight-agent.dsl.yaml"),
	})
	if err != nil {
		t.Fatalf("step 2, import customer insight agent: %v", err)
	}
	if len(insight.Candidates) != 6 {
		t.Fatalf("step 3, expected 6 candidates, got %d", len(insight.Candidates))
	}

	// 4. Review one candidate the way an operator would, then register everything.
	target := insight.Candidates[0]
	owner := "core-banking"
	reviewed, err := e.ReviewCandidate(ctx, target.ID, domain.ReviewInput{
		Fields: domain.CapabilityFields{Owner: owner, Tags: []string{"reviewed"}},
		Note:   "confirmed with the core banking team",
	})
	if err != nil {
		t.Fatalf("step 4, review: %v", err)
	}
	if reviewed.Resolved().Owner != owner {
		t.Fatalf("step 4, review did not take effect: owner = %q", reviewed.Resolved().Owner)
	}
	// The reviewer's edit must not destroy what extraction found.
	if reviewed.Extracted.Name == "" {
		t.Fatal("step 4, reviewing overwrote the extracted fields")
	}

	// 5-6. Import the product recommendation workflow and the manual capabilities.
	if _, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "product-recommendation-workflow.dsl.yaml"),
	}); err != nil {
		t.Fatalf("step 5, import product recommendation workflow: %v", err)
	}
	for _, file := range []string{"compliance-check-skill.yaml", "campaign-message-template.yaml"} {
		if _, err := e.ImportManualYAML(ctx, engine.ImportManualYAMLRequest{
			SourceName: "manual-registration",
			Payload:    readExample(t, "capabilities", file),
		}); err != nil {
			t.Fatalf("step 6, import %s: %v", file, err)
		}
	}
	registerAllPending(t, e)

	// The catalog now holds every registered capability.
	catalog, page, err := e.Capabilities().List(ctx, store.CapabilityFilter{Limit: store.MaxLimit})
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if page.Total != 13 {
		t.Fatalf("catalog holds %d capabilities, want 13: %v", page.Total, capabilitySlugs(catalog))
	}

	// 7-9. Recommend a composition for the business goal.
	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("step 9, recommend: %v", err)
	}
	if len(plan.AllRefs()) != 8 {
		t.Fatalf("step 9, plan selects %d capabilities, want 8: %v", len(plan.AllRefs()), slugsIn(plan.AllRefs()))
	}

	validation, err := e.ValidateComposition(ctx, *plan)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("plan should validate, errors: %+v", validation.Errors)
	}

	// 10-11. Generate the draft and export its YAML.
	draft, err := e.GenerateDraft(ctx, engine.GenerateDraftRequest{
		Name:        "RM Campaign Agent",
		Goal:        RMGoal,
		Description: "Generates customer campaign suggestions for relationship managers.",
		HarnessType: domain.HarnessJiuwenSwarm,
		Plan:        *plan,
	})
	if err != nil {
		t.Fatalf("step 10, generate draft: %v", err)
	}
	exported, err := e.RenderDraftYAML(ctx, draft.ID)
	if err != nil {
		t.Fatalf("step 11, export yaml: %v", err)
	}
	for _, expected := range []string{
		"id: rm-campaign-agent", "type: jiuwen_swarm",
		"customer-insight-agent", "compliance-check", "campaign-message-template",
		"missing_capabilities",
	} {
		if !strings.Contains(exported, expected) {
			t.Errorf("step 11, exported YAML is missing %q", expected)
		}
	}
}

// TestDuplicateRegistrationIsRefused covers the same asset arriving from two sources:
// the CRM tool comes from the Dify DSL and again from a manual YAML file.
func TestDuplicateRegistrationIsRefused(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	if _, err := e.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{
		SourceName: "customer-dify",
		Payload:    readExample(t, "dify", "customer-insight-agent.dsl.yaml"),
	}); err != nil {
		t.Fatalf("import dsl: %v", err)
	}
	registerAllPending(t, e)

	manual, err := e.ImportManualYAML(ctx, engine.ImportManualYAMLRequest{
		SourceName: "manual-registration",
		Payload:    readExample(t, "capabilities", "crm-query-tool.yaml"),
	})
	if err != nil {
		t.Fatalf("import manual crm tool: %v", err)
	}
	if len(manual.Candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(manual.Candidates))
	}
	// The candidate imports fine; registration is where the clash is caught.
	_, err = e.RegisterCandidate(ctx, manual.Candidates[0].ID)
	if err == nil {
		t.Fatal("expected registering a duplicate tool/crm-query to be refused")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("error should explain the duplicate, got: %v", err)
	}
}

// TestRejectedCandidateCannotBeRegistered checks the terminal states hold.
func TestRejectedCandidateCannotBeRegistered(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	result, err := e.ImportManualYAML(ctx, engine.ImportManualYAMLRequest{
		SourceName: "manual-registration",
		Payload:    readExample(t, "capabilities", "compliance-check-skill.yaml"),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	id := result.Candidates[0].ID

	rejected, err := e.RejectCandidate(ctx, id, "superseded by the new policy engine")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != domain.CandidateStatusRejected {
		t.Fatalf("status = %s, want rejected", rejected.Status)
	}
	if _, err := e.RegisterCandidate(ctx, id); !errors.Is(err, engine.ErrConflict) {
		t.Fatalf("registering a rejected candidate should conflict, got: %v", err)
	}
}

// TestMockImportersProduceCandidates covers the MCP and data adapter paths.
func TestMockImportersProduceCandidates(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	mcpResult, err := e.ImportMockMCP(ctx, engine.ImportMockRequest{})
	if err != nil {
		t.Fatalf("mcp import: %v", err)
	}
	if len(mcpResult.Candidates) == 0 {
		t.Fatal("mcp mock produced no candidates")
	}
	for _, candidate := range mcpResult.Candidates {
		if candidate.CandidateType != domain.CapabilityTypeTool {
			t.Errorf("mcp candidate %s has type %s, want tool", candidate.ExternalID, candidate.CandidateType)
		}
	}

	dataResult, err := e.ImportMockData(ctx, engine.ImportMockRequest{})
	if err != nil {
		t.Fatalf("data import: %v", err)
	}
	if len(dataResult.Candidates) == 0 {
		t.Fatal("data mock produced no candidates")
	}
	for _, candidate := range dataResult.Candidates {
		if candidate.CandidateType != domain.CapabilityTypeKnowledgeData {
			t.Errorf("data candidate %s has type %s, want knowledge_data", candidate.ExternalID, candidate.CandidateType)
		}
	}
}

// TestSourceSecretsAreRedacted keeps a configured API key from travelling back out.
func TestSourceSecretsAreRedacted(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()

	created, err := e.Sources().Create(ctx, domain.ExternalSource{
		Name:     "customer-dify",
		Type:     domain.SourceTypeDify,
		BaseURL:  "https://dify.internal",
		AuthType: "bearer",
		Config:   map[string]any{"api_key": "app-super-secret", "workspace": "retail"},
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	redacted := created.Redacted()
	if redacted.Config["api_key"] == "app-super-secret" {
		t.Error("api_key was not redacted")
	}
	if redacted.Config["workspace"] != "retail" {
		t.Error("redaction should only touch secrets")
	}
	// Redaction must not corrupt what is stored.
	stored, err := e.Sources().Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if stored.Config["api_key"] != "app-super-secret" {
		t.Error("redaction leaked into the stored record")
	}
}

func capabilitySlugs(capabilities []domain.Capability) []string {
	out := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, capability.Slug)
	}
	return out
}
