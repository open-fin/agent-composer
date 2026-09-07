// Package spec defines the exported YAML documents: the business agent draft, the
// capability registration format, and the composition plan.
package spec

// BusinessAgentDocument is the top level of an exported draft.
type BusinessAgentDocument struct {
	BusinessAgent BusinessAgent `yaml:"business_agent"`
}

// BusinessAgent is the draft itself. Field order here is the rendered YAML order.
type BusinessAgent struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Goal        string `yaml:"goal"`
	Description string `yaml:"description,omitempty"`
	Status      string `yaml:"status"`

	Harness     Harness     `yaml:"harness"`
	Composition Composition `yaml:"composition"`
	Workflow    []string    `yaml:"workflow"`
	Governance  Governance  `yaml:"governance"`

	// MissingCapabilities is what the registry could not satisfy. It is part of the
	// draft on purpose: the gap is a deliverable, not an error to hide.
	MissingCapabilities []MissingCapability `yaml:"missing_capabilities,omitempty"`
}

// Harness names the runtime a draft targets. Phase 1 executes nothing.
type Harness struct {
	Type string `yaml:"type"`
	Mode string `yaml:"mode"`
}

// Composition lists the capability slugs in each section. Empty sections are omitted.
type Composition struct {
	Agents        []string `yaml:"agents,omitempty"`
	Workflows     []string `yaml:"workflows,omitempty"`
	Skills        []string `yaml:"skills,omitempty"`
	Tools         []string `yaml:"tools,omitempty"`
	KnowledgeData []string `yaml:"knowledge_data,omitempty"`
	Prompts       []string `yaml:"prompts,omitempty"`
	Policies      []string `yaml:"policies,omitempty"`
}

// Governance is derived from the composed capabilities, never entered by hand.
type Governance struct {
	RiskLevel        string   `yaml:"risk_level"`
	ApprovalRequired []string `yaml:"approval_required,omitempty"`
	Permissions      []string `yaml:"permissions,omitempty"`
}

// MissingCapability is a step the registry cannot satisfy today.
type MissingCapability struct {
	Step       string `yaml:"step"`
	Type       string `yaml:"type"`
	Suggestion string `yaml:"suggestion,omitempty"`
}
