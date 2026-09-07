package spec

// CapabilityDocument is the manual registration format, and the format capabilities
// are exported in. It round-trips with the manual YAML importer.
type CapabilityDocument struct {
	Capabilities []CapabilityEntry `yaml:"capabilities"`
}

// CapabilityEntry is one capability in registration form.
type CapabilityEntry struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Type           string         `yaml:"type"`
	Subtype        string         `yaml:"subtype,omitempty"`
	Description    string         `yaml:"description"`
	BusinessDomain string         `yaml:"business_domain,omitempty"`
	Intents        []string       `yaml:"intents,omitempty"`
	Tags           []string       `yaml:"tags,omitempty"`
	Owner          string         `yaml:"owner,omitempty"`
	Permissions    []string       `yaml:"permissions,omitempty"`
	RiskLevel      string         `yaml:"risk_level"`
	Reusable       *bool          `yaml:"reusable,omitempty"`
	InputSchema    map[string]any `yaml:"input_schema,omitempty"`
	OutputSchema   map[string]any `yaml:"output_schema,omitempty"`
	Metadata       map[string]any `yaml:"metadata,omitempty"`
	DependsOn      []DependsOn    `yaml:"depends_on,omitempty"`
}

// DependsOn is an edge to another capability, by external id.
type DependsOn struct {
	ExternalID     string `yaml:"external_id"`
	TargetType     string `yaml:"target_type,omitempty"`
	DependencyType string `yaml:"dependency_type"`
}
