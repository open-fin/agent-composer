package llm_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/llm"
)

// TestMockEnricherIsDeterministic is what makes the demo reproducible: the same input
// must classify identically on every run.
func TestMockEnricherIsDeterministic(t *testing.T) {
	enricher := llm.NewMockEnricher()
	in := llm.CandidateContext{
		Type:        domain.CapabilityTypeTool,
		Name:        "CRM Query Tool",
		Description: "Queries the core banking CRM for a customer profile record.",
	}
	first, err := enricher.EnrichCandidate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := enricher.EnrichCandidate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("enrichment is not deterministic:\n%+v\n%+v", first, second)
	}
	if !reflect.DeepEqual(first.Intents, []string{"customer_profile_query"}) {
		t.Errorf("intents = %v", first.Intents)
	}
	if first.BusinessDomain != "customer" {
		t.Errorf("domain = %s, want customer", first.BusinessDomain)
	}
	if !reflect.DeepEqual(first.Permissions, []string{"customer_profile_read"}) {
		t.Errorf("permissions = %v, want customer_profile_read", first.Permissions)
	}
}

// TestOnlyToolsAndDatasetsGetPermissions checks the rule that an agent inherits access
// through its bindings rather than holding it directly.
func TestOnlyToolsAndDatasetsGetPermissions(t *testing.T) {
	enricher := llm.NewMockEnricher()
	fields, err := enricher.EnrichCandidate(context.Background(), llm.CandidateContext{
		Type:        domain.CapabilityTypeAgent,
		Name:        "Customer Insight Agent",
		Description: "Analyzes a customer profile.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields.Permissions) != 0 {
		t.Errorf("an agent should carry no permissions of its own, got %v", fields.Permissions)
	}
}

// TestDecomposeGoalEmitsUnsatisfiableSteps is the property the whole missing-capability
// feature rests on: decomposition describes what the goal needs, not what exists.
func TestDecomposeGoalEmitsUnsatisfiableSteps(t *testing.T) {
	roles, err := llm.NewMockEnricher().DecomposeGoal(context.Background(), domain.BusinessGoal{
		Goal: "Create an RM campaign agent that analyzes customer profile, recommends " +
			"suitable products, drafts a campaign message, and checks compliance before outreach.",
	})
	if err != nil {
		t.Fatal(err)
	}

	var steps []string
	for _, role := range roles {
		steps = append(steps, role.Step)
	}
	want := []string{
		"query_customer_profile", "generate_customer_insight", "recommend_product",
		"draft_campaign_message", "compliance_check", "outreach_send",
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	for _, role := range roles {
		if !role.CapabilityType.Valid() {
			t.Errorf("role %s has invalid capability type %q", role.Step, role.CapabilityType)
		}
	}
}

// TestDecomposeGoalAlwaysReturnsAtLeastOneRole keeps an unrecognised goal from
// producing an empty plan with nothing to report.
func TestDecomposeGoalAlwaysReturnsAtLeastOneRole(t *testing.T) {
	roles, err := llm.NewMockEnricher().DecomposeGoal(context.Background(), domain.BusinessGoal{
		Goal: "Reconcile the nostro ledger every morning.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) == 0 {
		t.Fatal("expected a fallback role for an unrecognised goal")
	}
}

// TestClientEnricherFallsBackToMock is the guarantee that a misconfigured or unreachable
// LLM degrades the output instead of breaking the demo.
func TestClientEnricherFallsBackToMock(t *testing.T) {
	enricher := llm.NewClientEnricher(&llm.MockClient{Err: errors.New("connection refused")}, nil)

	fields, err := enricher.EnrichCandidate(context.Background(), llm.CandidateContext{
		Type:        domain.CapabilityTypeTool,
		Name:        "CRM Query Tool",
		Description: "Queries the core banking CRM for a customer profile record.",
	})
	if err != nil {
		t.Fatalf("enrichment should not fail when the model is unreachable: %v", err)
	}
	if len(fields.Intents) == 0 {
		t.Error("fallback enrichment produced nothing")
	}

	roles, err := enricher.DecomposeGoal(context.Background(), domain.BusinessGoal{Goal: "check compliance"})
	if err != nil {
		t.Fatalf("decomposition should not fail when the model is unreachable: %v", err)
	}
	if len(roles) == 0 {
		t.Error("fallback decomposition produced nothing")
	}
}

// TestClientEnricherUsesModelOutputWhenUsable covers the happy path, including the
// markdown fences models habitually wrap JSON in.
func TestClientEnricherUsesModelOutputWhenUsable(t *testing.T) {
	enricher := llm.NewClientEnricher(&llm.MockClient{
		Response: "```json\n{\"business_domain\":\"wealth\",\"intents\":[\"portfolio_review\"]}\n```",
	}, nil)

	fields, err := enricher.EnrichCandidate(context.Background(), llm.CandidateContext{
		Type: domain.CapabilityTypeSkill,
		Name: "Portfolio Review Skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fields.BusinessDomain != "wealth" {
		t.Errorf("domain = %s, want the model's answer", fields.BusinessDomain)
	}
	if !reflect.DeepEqual(fields.Intents, []string{"portfolio_review"}) {
		t.Errorf("intents = %v, want the model's answer", fields.Intents)
	}
}

// TestClientEnricherRejectsUnusableModelOutput checks that malformed roles are dropped
// rather than propagated into a plan.
func TestClientEnricherRejectsUnusableModelOutput(t *testing.T) {
	enricher := llm.NewClientEnricher(&llm.MockClient{
		Response: `{"roles":[{"step":"","capability_type":"nonsense"}]}`,
	}, nil)

	roles, err := enricher.DecomposeGoal(context.Background(), domain.BusinessGoal{
		Goal: "checks compliance before outreach",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if role.Step == "" || !role.CapabilityType.Valid() {
			t.Fatalf("an invalid role survived: %+v", role)
		}
	}
}
