package engine_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
)

// TestRecommendRMCampaignComposition is the golden recommendation: the exact set of
// capabilities the demo goal must produce from the demo registry.
func TestRecommendRMCampaignComposition(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name:        "RM Campaign Agent",
		Goal:        RMGoal,
		HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}

	for _, tc := range []struct {
		section string
		got     []string
		want    []string
	}{
		{"agents", slugsIn(plan.RecommendedAgents), []string{"customer-insight-agent"}},
		{"workflows", slugsIn(plan.RecommendedWorkflows), []string{}},
		{"skills", slugsIn(plan.RecommendedSkills), []string{"product-recommendation", "compliance-check"}},
		{"tools", slugsIn(plan.RecommendedTools), []string{"crm-query", "product-catalog-query"}},
		{"knowledge_data", slugsIn(plan.RecommendedKnowledgeData), []string{"customer-profile-kb", "product-policy-kb"}},
		{"prompts", slugsIn(plan.RecommendedPrompts), []string{"campaign-message-template"}},
		{"policies", slugsIn(plan.RecommendedPolicies), []string{}},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.section, tc.got, tc.want)
		}
	}

	wantSteps := []string{
		"query_customer_profile", "generate_customer_insight", "recommend_product",
		"draft_campaign_message", "compliance_check",
	}
	if !reflect.DeepEqual(plan.WorkflowSteps, wantSteps) {
		t.Errorf("workflow steps = %v, want %v", plan.WorkflowSteps, wantSteps)
	}
}

// TestMissingCapabilityIsReported is the point of decomposing the goal before matching.
// The registry has no outreach channel, and the plan has to say so rather than quietly
// return a composition that cannot deliver the message it drafts.
func TestMissingCapabilityIsReported(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	if len(plan.MissingCapabilities) != 1 {
		t.Fatalf("missing = %+v, want exactly one gap", plan.MissingCapabilities)
	}
	gap := plan.MissingCapabilities[0]
	if gap.Step != "outreach_send" {
		t.Errorf("gap step = %s, want outreach_send", gap.Step)
	}
	if gap.RequiredType != domain.CapabilityTypeTool {
		t.Errorf("gap type = %s, want tool", gap.RequiredType)
	}
	if gap.Suggestion == "" {
		t.Error("a gap should say what to register to close it")
	}

	// The unsatisfied step must not appear in the workflow.
	for _, step := range plan.WorkflowSteps {
		if step == "outreach_send" {
			t.Error("an unsatisfied step leaked into the workflow")
		}
	}
}

// TestGovernanceIsDerivedNotEntered checks that risk, permissions and approval gates all
// come from the composed capabilities.
func TestGovernanceIsDerivedNotEntered(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	if plan.Governance.RiskLevel != domain.RiskLevelMedium {
		t.Errorf("risk = %s, want medium (the highest risk among the selected parts)", plan.Governance.RiskLevel)
	}
	wantPermissions := []string{"customer_profile_read", "product_catalog_read"}
	if !reflect.DeepEqual(plan.Governance.Permissions, wantPermissions) {
		t.Errorf("permissions = %v, want %v", plan.Governance.Permissions, wantPermissions)
	}
	wantApprovals := []string{"external_customer_outreach"}
	if !reflect.DeepEqual(plan.Governance.ApprovalRequired, wantApprovals) {
		t.Errorf("approvals = %v, want %v", plan.Governance.ApprovalRequired, wantApprovals)
	}
}

// TestDependencyClosurePullsInBindings covers why the plan contains capabilities no
// step matched: a skill's tool and dataset must be provisioned with it, while its
// prompt stays encapsulated.
func TestDependencyClosurePullsInBindings(t *testing.T) {
	e, _ := newTestEngine(t)
	ctx := context.Background()
	seedDemoRegistry(t, e)

	plan, err := e.RecommendComposition(ctx, domain.BusinessGoal{
		Name: "RM Campaign Agent", Goal: RMGoal, HarnessType: domain.HarnessJiuwenSwarm,
	})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}

	byReason := map[string]string{}
	for _, ref := range plan.AllRefs() {
		byReason[ref.Slug] = ref.Reason
	}
	if reason := byReason["product-catalog-query"]; reason == "" || reason[:8] != "required" {
		t.Errorf("product-catalog-query should be present as a binding, reason = %q", reason)
	}
	// The prompts behind the matched skills are implementation detail and must not be
	// dragged into the composition.
	for _, unwanted := range []string{"customer-insight-prompt", "product-recommendation-prompt"} {
		if _, present := byReason[unwanted]; present {
			t.Errorf("%s should stay inside its owning capability, not appear in the composition", unwanted)
		}
	}
}

// TestUnmatchableGoalStillReturnsAPlan checks the empty-registry case degrades into a
// list of gaps rather than an error.
func TestUnmatchableGoalStillReturnsAPlan(t *testing.T) {
	e, _ := newTestEngine(t)
	plan, err := e.RecommendComposition(context.Background(), domain.BusinessGoal{
		Name: "Empty", Goal: RMGoal, HarnessType: domain.HarnessManual,
	})
	if err != nil {
		t.Fatalf("recommend against an empty registry: %v", err)
	}
	if len(plan.AllRefs()) != 0 {
		t.Errorf("expected no recommendations from an empty registry, got %v", slugsIn(plan.AllRefs()))
	}
	if len(plan.MissingCapabilities) != len(plan.Roles) {
		t.Errorf("every role should be reported missing: %d gaps for %d roles",
			len(plan.MissingCapabilities), len(plan.Roles))
	}
}

// TestEmptyGoalIsRejected keeps a blank form submission from producing a plan.
func TestEmptyGoalIsRejected(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.RecommendComposition(context.Background(), domain.BusinessGoal{
		HarnessType: domain.HarnessManual,
	}); err == nil {
		t.Fatal("expected an error for an empty goal")
	}
}
