package domain

import "time"

// CapabilityFields is the mutable payload of a candidate. The same shape is produced by
// extraction, by inference, and by a human reviewer, which lets the three be layered.
type CapabilityFields struct {
	Name           string          `json:"name,omitempty"`
	Description    string          `json:"description,omitempty"`
	Subtype        string          `json:"subtype,omitempty"`
	BusinessDomain string          `json:"business_domain,omitempty"`
	Intents        []string        `json:"intents,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	InputSchema    map[string]any  `json:"input_schema,omitempty"`
	OutputSchema   map[string]any  `json:"output_schema,omitempty"`
	Owner          string          `json:"owner,omitempty"`
	Permissions    []string        `json:"permissions,omitempty"`
	RiskLevel      RiskLevel       `json:"risk_level,omitempty"`
	Reusable       *bool           `json:"reusable,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	DependsOn      []DependencyRef `json:"depends_on,omitempty"`
}

// Merge layers other on top of f, with non-empty values in other winning. Layering order
// is extracted -> inferred -> review, so a reviewer always has the final say.
func (f CapabilityFields) Merge(other CapabilityFields) CapabilityFields {
	out := f
	if other.Name != "" {
		out.Name = other.Name
	}
	if other.Description != "" {
		out.Description = other.Description
	}
	if other.Subtype != "" {
		out.Subtype = other.Subtype
	}
	if other.BusinessDomain != "" {
		out.BusinessDomain = other.BusinessDomain
	}
	if len(other.Intents) > 0 {
		out.Intents = other.Intents
	}
	if len(other.Tags) > 0 {
		out.Tags = other.Tags
	}
	if len(other.InputSchema) > 0 {
		out.InputSchema = other.InputSchema
	}
	if len(other.OutputSchema) > 0 {
		out.OutputSchema = other.OutputSchema
	}
	if other.Owner != "" {
		out.Owner = other.Owner
	}
	if len(other.Permissions) > 0 {
		out.Permissions = other.Permissions
	}
	if other.RiskLevel != "" {
		out.RiskLevel = other.RiskLevel
	}
	if other.Reusable != nil {
		out.Reusable = other.Reusable
	}
	if len(other.Metadata) > 0 {
		merged := make(map[string]any, len(out.Metadata)+len(other.Metadata))
		for k, v := range out.Metadata {
			merged[k] = v
		}
		for k, v := range other.Metadata {
			merged[k] = v
		}
		out.Metadata = merged
	}
	if len(other.DependsOn) > 0 {
		out.DependsOn = other.DependsOn
	}
	return out
}

// CapabilityCandidate is an extracted, not-yet-registered capability awaiting review.
type CapabilityCandidate struct {
	ID            string           `json:"id"`
	SourceID      string           `json:"source_id"`
	SourceSystem  string           `json:"source_system"`
	ExternalID    string           `json:"external_id"`
	CandidateType CapabilityType   `json:"candidate_type"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	RawPayload    map[string]any   `json:"raw_payload"`
	Extracted     CapabilityFields `json:"extracted"`
	Inferred      CapabilityFields `json:"inferred"`
	Review        CapabilityFields `json:"review"`
	Status        CandidateStatus  `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// Resolved layers extracted, inferred and review fields into the values that would be
// registered. It is what the review panel previews and what registration writes.
func (c CapabilityCandidate) Resolved() CapabilityFields {
	return c.Extracted.Merge(c.Inferred).Merge(c.Review)
}

// ReviewInput is the reviewer's edit applied to a candidate.
type ReviewInput struct {
	Fields CapabilityFields `json:"fields"`
	Note   string           `json:"note,omitempty"`
}
