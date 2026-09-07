// Package engine orchestrates ingestion, extraction, recommendation and draft
// generation. It depends on the store and adapter interfaces only.
package engine

import (
	"regexp"
	"sort"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

var (
	slugShaped = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	uuidShaped = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// SlugFor picks the stable identifier a capability is referenced by in composition YAML.
//
// A source system's own identifier is preferred when it is already readable, because
// that is what operators recognise (`crm-query`, `product-policy-kb`). Opaque ids —
// Dify's dataset UUIDs, for example — are discarded in favour of the display name.
func SlugFor(externalID, name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(externalID))
	if trimmed != "" && slugShaped.MatchString(trimmed) && !uuidShaped.MatchString(trimmed) {
		return trimmed
	}
	return domain.Slugify(name)
}

// NormalizeFields cleans a resolved candidate's fields before they become a capability:
// trims text, de-duplicates and orders vocabulary, and applies safe defaults.
func NormalizeFields(fields domain.CapabilityFields) domain.CapabilityFields {
	fields.Name = strings.TrimSpace(fields.Name)
	fields.Description = strings.TrimSpace(fields.Description)
	fields.Subtype = strings.TrimSpace(fields.Subtype)
	fields.Owner = strings.TrimSpace(fields.Owner)
	fields.BusinessDomain = strings.ToLower(strings.TrimSpace(fields.BusinessDomain))

	fields.Intents = normalizeVocabulary(fields.Intents)
	fields.Tags = normalizeVocabulary(fields.Tags)
	fields.Permissions = normalizeVocabulary(fields.Permissions)

	if !fields.RiskLevel.Valid() {
		fields.RiskLevel = domain.RiskLevelLow
	}
	if fields.InputSchema == nil {
		fields.InputSchema = map[string]any{}
	}
	if fields.OutputSchema == nil {
		fields.OutputSchema = map[string]any{}
	}
	if fields.Metadata == nil {
		fields.Metadata = map[string]any{}
	}
	return fields
}

// normalizeVocabulary lower-cases, de-duplicates and sorts a matchable vocabulary list
// so recommendation scoring is order-independent and stable across imports.
func normalizeVocabulary(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// UnionSorted merges string sets into one sorted, de-duplicated list. Governance
// permissions and approval gates are built this way.
func UnionSorted(lists ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, list := range lists {
		for _, value := range list {
			v := strings.TrimSpace(value)
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
