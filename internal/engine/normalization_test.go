package engine_test

import (
	"reflect"
	"testing"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
)

// TestSlugForPrefersReadableExternalIDs pins the identifier rule that keeps generated
// YAML legible: use the source system's own id when a human would recognise it, and
// fall back to the display name when the id is opaque.
func TestSlugForPrefersReadableExternalIDs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		externalID string
		display    string
		want       string
	}{
		{"readable source id wins", "crm-query", "CRM Query Tool", "crm-query"},
		{"uuid is discarded", "3f2a1b4c-1111-2222-3333-444455556666", "Customer Profile KB", "customer-profile-kb"},
		{"empty id falls back to the name", "", "Compliance Check Skill", "compliance-check-skill"},
		{"mixed case id is normalized", "CRM-Query", "CRM Query Tool", "crm-query"},
		{"id with spaces is not slug shaped", "crm query", "CRM Query Tool", "crm-query-tool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := engine.SlugFor(tc.externalID, tc.display); got != tc.want {
				t.Errorf("SlugFor(%q, %q) = %q, want %q", tc.externalID, tc.display, got, tc.want)
			}
		})
	}
}

// TestNormalizeFieldsStabilizesVocabulary checks that matching is not sensitive to the
// order or casing a source system happens to use.
func TestNormalizeFieldsStabilizesVocabulary(t *testing.T) {
	got := engine.NormalizeFields(domain.CapabilityFields{
		Name:        "  CRM Query Tool  ",
		Description: " Queries the CRM. ",
		Intents:     []string{"Customer_Profile_Query", "customer_profile_query", " "},
		Tags:        []string{"CRM", "crm", "Customer"},
		Permissions: []string{"customer_profile_read", "customer_profile_read"},
	})

	if got.Name != "CRM Query Tool" {
		t.Errorf("name = %q, want trimmed", got.Name)
	}
	if !reflect.DeepEqual(got.Intents, []string{"customer_profile_query"}) {
		t.Errorf("intents = %v, want a single lowercased value", got.Intents)
	}
	if !reflect.DeepEqual(got.Tags, []string{"crm", "customer"}) {
		t.Errorf("tags = %v, want deduplicated and sorted", got.Tags)
	}
	if !reflect.DeepEqual(got.Permissions, []string{"customer_profile_read"}) {
		t.Errorf("permissions = %v, want deduplicated", got.Permissions)
	}
	if got.RiskLevel != domain.RiskLevelLow {
		t.Errorf("risk = %q, want the low default", got.RiskLevel)
	}
	if got.InputSchema == nil || got.OutputSchema == nil || got.Metadata == nil {
		t.Error("schema and metadata maps should be initialized, not nil")
	}
}

// TestUnionSorted covers the helper governance permissions are built with.
func TestUnionSorted(t *testing.T) {
	got := engine.UnionSorted(
		[]string{"product_catalog_read", "customer_profile_read"},
		[]string{"customer_profile_read", ""},
		nil,
	)
	want := []string{"customer_profile_read", "product_catalog_read"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnionSorted = %v, want %v", got, want)
	}
}

// TestMaxRisk pins the ordering governance relies on.
func TestMaxRisk(t *testing.T) {
	if got := domain.MaxRisk(domain.RiskLevelLow, domain.RiskLevelMedium, domain.RiskLevelLow); got != domain.RiskLevelMedium {
		t.Errorf("MaxRisk = %s, want medium", got)
	}
	if got := domain.MaxRisk(); got != domain.RiskLevelLow {
		t.Errorf("MaxRisk of nothing = %s, want low", got)
	}
}
