package domain

import "time"

// Capability is a registered, standardized unit of reusable business capability.
//
// ID is the storage key (readable slug plus a short unique suffix). Slug is the stable
// human-facing identifier that composition YAML references, and is unique per type.
type Capability struct {
	ID             string           `json:"id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Type           CapabilityType   `json:"type"`
	Subtype        string           `json:"subtype,omitempty"`
	Description    string           `json:"description"`
	SourceSystem   string           `json:"source_system"`
	ExternalID     string           `json:"external_id,omitempty"`
	BusinessDomain string           `json:"business_domain,omitempty"`
	Intents        []string         `json:"intents"`
	Tags           []string         `json:"tags"`
	InputSchema    map[string]any   `json:"input_schema"`
	OutputSchema   map[string]any   `json:"output_schema"`
	Owner          string           `json:"owner,omitempty"`
	Permissions    []string         `json:"permissions"`
	RiskLevel      RiskLevel        `json:"risk_level"`
	Reusable       bool             `json:"reusable"`
	Status         CapabilityStatus `json:"status"`
	Metadata       map[string]any   `json:"metadata"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`

	// Dependencies is hydrated on read; it is stored in capability_dependencies.
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

// ApprovalRequired reads the approval gates a capability declares in its metadata.
func (c Capability) ApprovalRequired() []string {
	return stringsFromMeta(c.Metadata, "approval_required")
}

func stringsFromMeta(meta map[string]any, key string) []string {
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	}
	return nil
}
