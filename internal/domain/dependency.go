package domain

import "time"

// Dependency is a stored edge between two registered capabilities.
type Dependency struct {
	ID                    string         `json:"id"`
	CapabilityID          string         `json:"capability_id"`
	DependsOnCapabilityID string         `json:"depends_on_capability_id"`
	DependencyType        DependencyType `json:"dependency_type"`
	Config                map[string]any `json:"config"`
	CreatedAt             time.Time      `json:"created_at"`

	// DependsOnSlug and DependsOnType are hydrated on read for display convenience.
	DependsOnSlug string         `json:"depends_on_slug,omitempty"`
	DependsOnType CapabilityType `json:"depends_on_type,omitempty"`
	DependsOnName string         `json:"depends_on_name,omitempty"`
}

// DependencyRef is an unresolved edge carried on a candidate. It points at an external
// id because the target may not be registered yet at extraction time.
type DependencyRef struct {
	ExternalID     string         `json:"external_id"`
	TargetType     CapabilityType `json:"target_type"`
	DependencyType DependencyType `json:"dependency_type"`
	Config         map[string]any `json:"config,omitempty"`
}
