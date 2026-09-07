package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

func newCandidate(sourceID, externalID, name string, capType domain.CapabilityType) domain.CapabilityCandidate {
	return domain.CapabilityCandidate{
		SourceID:      sourceID,
		SourceSystem:  "dify",
		ExternalID:    externalID,
		CandidateType: capType,
		Name:          name,
		Description:   name + " description",
		Status:        domain.CandidateStatusExtracted,
	}
}

// TestCandidateUpsertKeysOnSourceAndExternalID covers re-import idempotency.
func TestCandidateUpsertKeysOnSourceAndExternalID(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()

	first, err := memory.Candidates().Upsert(ctx, newCandidate("src-1", "crm-query", "CRM Query Tool", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	second, err := memory.Candidates().Upsert(ctx, newCandidate("src-1", "crm-query", "CRM Query Tool v2", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("re-import created a new candidate: %s then %s", first.ID, second.ID)
	}
	if second.Name != "CRM Query Tool v2" {
		t.Errorf("re-import did not refresh the name: %s", second.Name)
	}

	// A different source holding the same external id is a separate candidate.
	other, err := memory.Candidates().Upsert(ctx, newCandidate("src-2", "crm-query", "CRM Query Tool", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == first.ID {
		t.Error("candidates from different sources were merged")
	}
}

// TestUpsertPreservesReviewAndTerminalState checks a re-import cannot undo human work.
func TestUpsertPreservesReviewAndTerminalState(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()

	created, err := memory.Candidates().Upsert(ctx, newCandidate("src-1", "crm-query", "CRM Query Tool", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	created.Review = domain.CapabilityFields{Owner: "core-banking"}
	if _, err := memory.Candidates().Update(ctx, created); err != nil {
		t.Fatal(err)
	}

	refreshed, err := memory.Candidates().Upsert(ctx, newCandidate("src-1", "crm-query", "CRM Query Tool", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Review.Owner != "core-banking" {
		t.Error("re-import discarded the reviewer's edit")
	}

	refreshed.Status = domain.CandidateStatusRegistered
	if _, err := memory.Candidates().Update(ctx, refreshed); err != nil {
		t.Fatal(err)
	}
	final, err := memory.Candidates().Upsert(ctx, newCandidate("src-1", "crm-query", "Renamed", domain.CapabilityTypeTool))
	if err != nil {
		t.Fatal(err)
	}
	if final.Name == "Renamed" {
		t.Error("re-import overwrote an already-registered candidate")
	}
}

// TestCapabilitySlugUniqueness is the duplicate guard the registry relies on.
func TestCapabilitySlugUniqueness(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()

	base := domain.Capability{
		Slug: "crm-query", Name: "CRM Query Tool", Type: domain.CapabilityTypeTool,
		RiskLevel: domain.RiskLevelLow, Status: domain.CapabilityStatusActive,
	}
	if _, err := memory.Capabilities().Create(ctx, base); err != nil {
		t.Fatal(err)
	}
	if _, err := memory.Capabilities().Create(ctx, base); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected a conflict on a duplicate (type, slug), got %v", err)
	}

	// The same slug under a different type is a different capability.
	base.Type = domain.CapabilityTypeSkill
	if _, err := memory.Capabilities().Create(ctx, base); err != nil {
		t.Fatalf("same slug under a different type should be allowed: %v", err)
	}
}

// TestListFiltersAndPaging covers what the catalog UI depends on.
func TestListFiltersAndPaging(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()

	for _, spec := range []struct {
		slug    string
		capType domain.CapabilityType
	}{
		{"crm-query", domain.CapabilityTypeTool},
		{"product-catalog-query", domain.CapabilityTypeTool},
		{"customer-profile-kb", domain.CapabilityTypeKnowledgeData},
	} {
		if _, err := memory.Capabilities().Create(ctx, domain.Capability{
			Slug: spec.slug, Name: spec.slug, Type: spec.capType,
			Description: spec.slug + " description",
			RiskLevel:   domain.RiskLevelLow, Status: domain.CapabilityStatusActive,
		}); err != nil {
			t.Fatal(err)
		}
	}

	tools, page, err := memory.Capabilities().List(ctx, store.CapabilityFilter{Type: "tool"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(tools) != 2 {
		t.Errorf("type filter returned %d of %d", len(tools), page.Total)
	}

	found, page, err := memory.Capabilities().List(ctx, store.CapabilityFilter{Query: "profile"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || found[0].Slug != "customer-profile-kb" {
		t.Errorf("search returned %v", found)
	}

	firstPage, page, err := memory.Capabilities().List(ctx, store.CapabilityFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 2 || page.Total != 3 {
		t.Errorf("paging returned %d items of %d total", len(firstPage), page.Total)
	}
	secondPage, _, err := memory.Capabilities().List(ctx, store.CapabilityFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 1 {
		t.Errorf("second page returned %d items, want 1", len(secondPage))
	}
}

// TestNotFoundIsDistinguishable lets the API map errors onto status codes.
func TestNotFoundIsDistinguishable(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()

	if _, err := memory.Capabilities().Get(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("capability: got %v, want ErrNotFound", err)
	}
	if _, err := memory.Candidates().Get(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("candidate: got %v, want ErrNotFound", err)
	}
	if _, err := memory.Drafts().Get(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("draft: got %v, want ErrNotFound", err)
	}
	if _, err := memory.Sources().Get(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("source: got %v, want ErrNotFound", err)
	}
}

// TestEnsureByNameIsIdempotent covers the auto-created upload source.
func TestEnsureByNameIsIdempotent(t *testing.T) {
	memory := store.NewMemory()
	ctx := context.Background()
	src := domain.ExternalSource{Name: "customer-dify", Type: domain.SourceTypeDify}

	first, err := memory.Sources().EnsureByName(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	second, err := memory.Sources().EnsureByName(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Errorf("EnsureByName created two sources: %s and %s", first.ID, second.ID)
	}
}
