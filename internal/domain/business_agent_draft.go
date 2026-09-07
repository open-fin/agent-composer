package domain

import "time"

// CompositionComponent is the authoritative, relational record of one capability's
// participation in a draft. business_agent_drafts.composition_json is a regenerated
// snapshot of the plan; these rows are what the draft actually consists of.
type CompositionComponent struct {
	ID           string         `json:"id"`
	DraftID      string         `json:"draft_id"`
	CapabilityID string         `json:"capability_id"`
	Role         ComponentRole  `json:"component_role"`
	OrderIndex   int            `json:"order_index"`
	Config       map[string]any `json:"config"`
	CreatedAt    time.Time      `json:"created_at"`

	// Hydrated on read for display.
	CapabilitySlug string         `json:"capability_slug,omitempty"`
	CapabilityName string         `json:"capability_name,omitempty"`
	CapabilityType CapabilityType `json:"capability_type,omitempty"`
}

// BusinessAgentDraft is the phase 1 output artifact. It is never published or executed.
type BusinessAgentDraft struct {
	ID          string           `json:"id"`
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Goal        string           `json:"goal"`
	Description string           `json:"description,omitempty"`
	HarnessType HarnessType      `json:"harness_type"`
	Status      DraftStatus      `json:"status"`
	Composition CompositionPlan  `json:"composition"`
	Governance  GovernanceConfig `json:"governance"`
	YAMLText    string           `json:"yaml_text"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`

	// Components is hydrated from composition_components on read.
	Components []CompositionComponent `json:"components,omitempty"`
}

// DraftUpdate is the mutable subset of a draft exposed by PUT /drafts/{id}.
type DraftUpdate struct {
	Name        *string      `json:"name,omitempty"`
	Description *string      `json:"description,omitempty"`
	Goal        *string      `json:"goal,omitempty"`
	HarnessType *HarnessType `json:"harness_type,omitempty"`
	Status      *DraftStatus `json:"status,omitempty"`
}
