package spec

// CompositionDocument is a recommendation rendered as YAML, used when a plan is
// exported for review before a draft is generated.
type CompositionDocument struct {
	Composition CompositionPlan `yaml:"composition"`
}

// CompositionPlan mirrors the recommendation response.
type CompositionPlan struct {
	Goal                string              `yaml:"goal"`
	Harness             string              `yaml:"harness"`
	Roles               []Role              `yaml:"roles"`
	Recommended         Composition         `yaml:"recommended"`
	Workflow            []string            `yaml:"workflow"`
	MissingCapabilities []MissingCapability `yaml:"missing_capabilities,omitempty"`
	Warnings            []Warning           `yaml:"warnings,omitempty"`
}

// Role is one decomposed step of the goal.
type Role struct {
	Step           string `yaml:"step"`
	CapabilityType string `yaml:"capability_type"`
	Satisfied      bool   `yaml:"satisfied"`
	MatchedSlug    string `yaml:"matched_slug,omitempty"`
}

// Warning is a finding attached to a plan.
type Warning struct {
	Code     string `yaml:"code"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}
