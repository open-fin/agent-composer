package domain

// BusinessGoal is the user's request: a plain-language goal plus a target harness.
type BusinessGoal struct {
	Name        string      `json:"name"`
	Goal        string      `json:"goal"`
	Description string      `json:"description,omitempty"`
	HarnessType HarnessType `json:"harness_type"`
}

// RequiredRole is one step the goal decomposer says the business agent must be able to
// perform, together with the kind of capability that can satisfy it.
//
// Decomposing the goal into roles *before* matching is what makes missing capabilities
// derivable at all: matching alone can only ever return capabilities that exist.
type RequiredRole struct {
	Step           string         `json:"step"`
	CapabilityType CapabilityType `json:"capability_type"`
	Description    string         `json:"description,omitempty"`
	Keywords       []string       `json:"keywords,omitempty"`
	Intents        []string       `json:"intents,omitempty"`
}

// CapabilityRef is a capability as referenced from a composition.
type CapabilityRef struct {
	ID    string         `json:"id"`
	Slug  string         `json:"slug"`
	Name  string         `json:"name"`
	Type  CapabilityType `json:"type"`
	Role  string         `json:"role,omitempty"`
	Score float64        `json:"score,omitempty"`
	// Reason explains why the capability is in the plan: a matched role, or the
	// capability whose dependency pulled it in.
	Reason string `json:"reason,omitempty"`
}

// MissingCapability is a required role that nothing in the registry satisfies.
type MissingCapability struct {
	Step         string         `json:"step"`
	RequiredType CapabilityType `json:"required_type"`
	Reason       string         `json:"reason"`
	Suggestion   string         `json:"suggestion,omitempty"`
	Keywords     []string       `json:"keywords,omitempty"`
}

// Warning severities.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Warning is a non-fatal finding produced while building or validating a plan.
type Warning struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	CapabilitySlug string `json:"capability_slug,omitempty"`
	Step           string `json:"step,omitempty"`
}

// CompositionPlan is the recommendation returned for a business goal.
type CompositionPlan struct {
	Goal  BusinessGoal   `json:"goal"`
	Roles []RequiredRole `json:"roles"`

	RecommendedAgents        []CapabilityRef `json:"recommended_agents"`
	RecommendedWorkflows     []CapabilityRef `json:"recommended_workflows"`
	RecommendedSkills        []CapabilityRef `json:"recommended_skills"`
	RecommendedTools         []CapabilityRef `json:"recommended_tools"`
	RecommendedKnowledgeData []CapabilityRef `json:"recommended_knowledge_data"`
	RecommendedPrompts       []CapabilityRef `json:"recommended_prompts"`
	RecommendedPolicies      []CapabilityRef `json:"recommended_policies"`

	// WorkflowSteps is the ordered list of satisfied role steps.
	WorkflowSteps       []string            `json:"workflow_steps"`
	MissingCapabilities []MissingCapability `json:"missing_capabilities"`
	Warnings            []Warning           `json:"warnings"`
	Governance          GovernanceConfig    `json:"governance"`
}

// ByRole returns the recommendation bucket for a composition role.
func (p *CompositionPlan) ByRole(role ComponentRole) []CapabilityRef {
	switch role {
	case RoleAgent:
		return p.RecommendedAgents
	case RoleWorkflow:
		return p.RecommendedWorkflows
	case RoleSkill:
		return p.RecommendedSkills
	case RoleTool:
		return p.RecommendedTools
	case RoleKnowledgeData:
		return p.RecommendedKnowledgeData
	case RolePrompt:
		return p.RecommendedPrompts
	case RolePolicy:
		return p.RecommendedPolicies
	}
	return nil
}

// Add appends a reference into the bucket matching its capability type.
func (p *CompositionPlan) Add(ref CapabilityRef) {
	switch RoleForType(ref.Type) {
	case RoleAgent:
		p.RecommendedAgents = append(p.RecommendedAgents, ref)
	case RoleWorkflow:
		p.RecommendedWorkflows = append(p.RecommendedWorkflows, ref)
	case RoleSkill:
		p.RecommendedSkills = append(p.RecommendedSkills, ref)
	case RoleTool:
		p.RecommendedTools = append(p.RecommendedTools, ref)
	case RoleKnowledgeData:
		p.RecommendedKnowledgeData = append(p.RecommendedKnowledgeData, ref)
	case RolePrompt:
		p.RecommendedPrompts = append(p.RecommendedPrompts, ref)
	default:
		p.RecommendedPolicies = append(p.RecommendedPolicies, ref)
	}
}

// AllRefs flattens every bucket in fixed section order.
func (p *CompositionPlan) AllRefs() []CapabilityRef {
	var out []CapabilityRef
	for _, role := range AllComponentRoles {
		out = append(out, p.ByRole(role)...)
	}
	return out
}

// Has reports whether a capability id is already present in any bucket.
func (p *CompositionPlan) Has(capabilityID string) bool {
	for _, ref := range p.AllRefs() {
		if ref.ID == capabilityID {
			return true
		}
	}
	return false
}

// GovernanceConfig is derived from the selected capabilities, never hand-entered.
type GovernanceConfig struct {
	RiskLevel        RiskLevel `json:"risk_level"`
	ApprovalRequired []string  `json:"approval_required"`
	Permissions      []string  `json:"permissions"`
}

// CompositionValidationResult is returned by the validate endpoint.
type CompositionValidationResult struct {
	Valid    bool      `json:"valid"`
	Errors   []Warning `json:"errors"`
	Warnings []Warning `json:"warnings"`
}
