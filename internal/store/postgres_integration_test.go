//go:build integration

// Package store's Postgres coverage. These tests need a real database and are excluded
// from the default build, so `go test ./...` stays runnable with no infrastructure.
//
//	COMPOSER_TEST_DSN="postgres://composer:composer@localhost:5432/agent_composer?sslmode=disable" \
//	  go test -tags integration ./internal/store/
package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// newPostgres connects, migrates and truncates, so each run starts from a known state.
func newPostgres(t *testing.T) *store.Postgres {
	t.Helper()
	dsn := os.Getenv("COMPOSER_TEST_DSN")
	if dsn == "" {
		t.Skip("COMPOSER_TEST_DSN is not set; skipping Postgres integration tests")
	}
	ctx := context.Background()
	postgres, err := store.NewPostgres(ctx, dsn, 4, 10, true, nil)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	t.Cleanup(postgres.Close)
	if err := postgres.TruncateAll(ctx); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	return postgres
}

// TestPostgresMigrationsAreIdempotent covers the boot path: the server applies migrations
// on every start, so a second start must be a no-op.
func TestPostgresMigrationsAreIdempotent(t *testing.T) {
	postgres := newPostgres(t)
	if err := postgres.Migrate(context.Background()); err != nil {
		t.Fatalf("re-applying migrations failed: %v", err)
	}
}

// TestPostgresRoundTripsACapability exercises the jsonb columns and the scan path.
func TestPostgresRoundTripsACapability(t *testing.T) {
	postgres := newPostgres(t)
	ctx := context.Background()

	created, err := postgres.Capabilities().Create(ctx, domain.Capability{
		ID: "crm-query-test01", Slug: "crm-query", Name: "CRM Query Tool",
		Type: domain.CapabilityTypeTool, Subtype: "api",
		Description: "Queries the core banking CRM.", SourceSystem: "dify",
		ExternalID: "crm-query", BusinessDomain: "customer",
		Intents: []string{"customer_profile_query"}, Tags: []string{"crm", "customer"},
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
		Owner:        "core-banking", Permissions: []string{"customer_profile_read"},
		RiskLevel: domain.RiskLevelLow, Reusable: true,
		Status:   domain.CapabilityStatusActive,
		Metadata: map[string]any{"system_of_record": "core-banking-crm"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fetched, err := postgres.Capabilities().Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Slug != "crm-query" || fetched.Name != created.Name {
		t.Errorf("round trip changed identity: %+v", fetched)
	}
	if len(fetched.Intents) != 1 || fetched.Intents[0] != "customer_profile_query" {
		t.Errorf("intents did not round trip: %v", fetched.Intents)
	}
	if len(fetched.Tags) != 2 {
		t.Errorf("tags did not round trip: %v", fetched.Tags)
	}
	if fetched.Metadata["system_of_record"] != "core-banking-crm" {
		t.Errorf("metadata did not round trip: %v", fetched.Metadata)
	}

	bySlug, err := postgres.Capabilities().GetBySlug(ctx, domain.CapabilityTypeTool, "crm-query")
	if err != nil || bySlug.ID != created.ID {
		t.Errorf("GetBySlug: %v %+v", err, bySlug)
	}
}

// TestPostgresEnforcesSlugUniqueness proves the unique index, not just the Go check.
func TestPostgresEnforcesSlugUniqueness(t *testing.T) {
	postgres := newPostgres(t)
	ctx := context.Background()

	capability := domain.Capability{
		ID: "crm-query-a", Slug: "crm-query", Name: "CRM Query Tool",
		Type: domain.CapabilityTypeTool, RiskLevel: domain.RiskLevelLow,
		Status: domain.CapabilityStatusActive,
	}
	if _, err := postgres.Capabilities().Create(ctx, capability); err != nil {
		t.Fatalf("first create: %v", err)
	}
	capability.ID = "crm-query-b"
	if _, err := postgres.Capabilities().Create(ctx, capability); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate (type, slug) should conflict, got: %v", err)
	}
}

// TestPostgresCandidateUpsert covers the ON CONFLICT clause and its status guard.
func TestPostgresCandidateUpsert(t *testing.T) {
	postgres := newPostgres(t)
	ctx := context.Background()

	source, err := postgres.Sources().EnsureByName(ctx, domain.ExternalSource{
		Name: "customer-dify", Type: domain.SourceTypeDify, Config: map[string]any{},
	})
	if err != nil {
		t.Fatalf("ensure source: %v", err)
	}

	candidate := domain.CapabilityCandidate{
		SourceID: source.ID, SourceSystem: "dify", ExternalID: "crm-query",
		CandidateType: domain.CapabilityTypeTool, Name: "CRM Query Tool",
		Description: "v1", Status: domain.CandidateStatusExtracted,
		RawPayload: map[string]any{"node_id": "crm_query"},
	}
	first, err := postgres.Candidates().Upsert(ctx, candidate)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	candidate.Description = "v2"
	second, err := postgres.Candidates().Upsert(ctx, candidate)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("re-import created a new row: %s then %s", first.ID, second.ID)
	}
	if second.Description != "v2" {
		t.Errorf("re-import did not refresh the description: %q", second.Description)
	}

	// A registered candidate must survive a re-import untouched.
	second.Status = domain.CandidateStatusRegistered
	if _, err := postgres.Candidates().Update(ctx, second); err != nil {
		t.Fatalf("mark registered: %v", err)
	}
	candidate.Description = "v3"
	third, err := postgres.Candidates().Upsert(ctx, candidate)
	if err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if third.Description == "v3" {
		t.Error("re-import overwrote an already-registered candidate")
	}
}

// TestPostgresDependenciesAndDraft covers the two transactional write paths.
func TestPostgresDependenciesAndDraft(t *testing.T) {
	postgres := newPostgres(t)
	ctx := context.Background()

	newCapability := func(id, slug string, capType domain.CapabilityType) domain.Capability {
		return domain.Capability{
			ID: id, Slug: slug, Name: slug, Type: capType,
			RiskLevel: domain.RiskLevelLow, Status: domain.CapabilityStatusActive,
		}
	}
	agent, err := postgres.Capabilities().Create(ctx, newCapability("agent-1", "customer-insight-agent", domain.CapabilityTypeAgent))
	if err != nil {
		t.Fatal(err)
	}
	tool, err := postgres.Capabilities().Create(ctx, newCapability("tool-1", "crm-query", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}

	if err := postgres.Dependencies().Replace(ctx, agent.ID, []domain.Dependency{{
		CapabilityID: agent.ID, DependsOnCapabilityID: tool.ID,
		DependencyType: domain.DependencyUsesTool, Config: map[string]any{},
	}}); err != nil {
		t.Fatalf("replace dependencies: %v", err)
	}
	deps, err := postgres.Dependencies().ListFor(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list dependencies: %v", err)
	}
	if len(deps) != 1 || deps[0].DependsOnSlug != "crm-query" {
		t.Fatalf("dependencies did not hydrate: %+v", deps)
	}

	// Replace is a full rewrite, so an empty set clears the edges.
	if err := postgres.Dependencies().Replace(ctx, agent.ID, nil); err != nil {
		t.Fatalf("clear dependencies: %v", err)
	}
	if deps, err = postgres.Dependencies().ListFor(ctx, agent.ID); err != nil || len(deps) != 0 {
		t.Fatalf("dependencies were not cleared: %v %+v", err, deps)
	}

	draft, err := postgres.Drafts().Create(ctx, domain.BusinessAgentDraft{
		ID: "draft-1", Slug: "rm-campaign-agent", Name: "RM Campaign Agent",
		Goal: "test", HarnessType: domain.HarnessJiuwenSwarm, Status: domain.DraftStatusDraft,
		Composition: domain.CompositionPlan{WorkflowSteps: []string{"query_customer_profile"}},
		Governance:  domain.GovernanceConfig{RiskLevel: domain.RiskLevelMedium},
		YAMLText:    "business_agent:\n  id: rm-campaign-agent\n",
	}, []domain.CompositionComponent{
		{CapabilityID: agent.ID, Role: domain.RoleAgent},
		{CapabilityID: tool.ID, Role: domain.RoleTool},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if len(draft.Components) != 2 {
		t.Fatalf("draft has %d components, want 2", len(draft.Components))
	}
	if draft.Components[0].OrderIndex != 0 || draft.Components[1].OrderIndex != 1 {
		t.Errorf("component ordering was not preserved: %+v", draft.Components)
	}

	fetched, err := postgres.Drafts().Get(ctx, draft.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if len(fetched.Composition.WorkflowSteps) != 1 {
		t.Errorf("composition snapshot did not round trip: %+v", fetched.Composition)
	}
	if fetched.Governance.RiskLevel != domain.RiskLevelMedium {
		t.Errorf("governance did not round trip: %+v", fetched.Governance)
	}
}
