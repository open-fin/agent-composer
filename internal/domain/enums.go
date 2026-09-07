package domain

import "strings"

// CapabilityType is the first-class taxonomy of things the registry can hold.
type CapabilityType string

const (
	CapabilityTypeAgent          CapabilityType = "agent"
	CapabilityTypeWorkflow       CapabilityType = "workflow"
	CapabilityTypeSkill          CapabilityType = "skill"
	CapabilityTypeTool           CapabilityType = "tool"
	CapabilityTypeKnowledgeData  CapabilityType = "knowledge_data"
	CapabilityTypePromptTemplate CapabilityType = "prompt_template"
	CapabilityTypePolicy         CapabilityType = "policy"
)

// AllCapabilityTypes is the catalog order used by the UI filter and by YAML rendering.
var AllCapabilityTypes = []CapabilityType{
	CapabilityTypeAgent,
	CapabilityTypeWorkflow,
	CapabilityTypeSkill,
	CapabilityTypeTool,
	CapabilityTypeKnowledgeData,
	CapabilityTypePromptTemplate,
	CapabilityTypePolicy,
}

func (t CapabilityType) Valid() bool {
	for _, known := range AllCapabilityTypes {
		if t == known {
			return true
		}
	}
	return false
}

// CapabilityStatus is the lifecycle of a registered capability.
type CapabilityStatus string

const (
	CapabilityStatusActive     CapabilityStatus = "active"
	CapabilityStatusDeprecated CapabilityStatus = "deprecated"
)

// CandidateStatus carries both the provenance of a pre-registration candidate
// (extracted / inferred / needs_review) and its terminal outcome.
//
//	extracted    - every core field came straight from the source payload.
//	inferred     - one or more core fields were heuristic or LLM derived.
//	needs_review - validation found a required field missing.
//	registered   - promoted into the capability registry (terminal).
//	rejected     - dismissed by a reviewer (terminal).
type CandidateStatus string

const (
	CandidateStatusExtracted   CandidateStatus = "extracted"
	CandidateStatusInferred    CandidateStatus = "inferred"
	CandidateStatusNeedsReview CandidateStatus = "needs_review"
	CandidateStatusRegistered  CandidateStatus = "registered"
	CandidateStatusRejected    CandidateStatus = "rejected"
)

// Pending reports whether the candidate is still awaiting a review decision.
func (s CandidateStatus) Pending() bool {
	return s == CandidateStatusExtracted || s == CandidateStatusInferred || s == CandidateStatusNeedsReview
}

func (s CandidateStatus) Valid() bool {
	switch s {
	case CandidateStatusExtracted, CandidateStatusInferred, CandidateStatusNeedsReview,
		CandidateStatusRegistered, CandidateStatusRejected:
		return true
	}
	return false
}

// RiskLevel is ordered; governance takes the maximum across selected capabilities.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

var riskRank = map[RiskLevel]int{RiskLevelLow: 1, RiskLevelMedium: 2, RiskLevelHigh: 3}

func (r RiskLevel) Rank() int { return riskRank[r] }

func (r RiskLevel) Valid() bool { _, ok := riskRank[r]; return ok }

// MaxRisk returns the highest risk level in the set, defaulting to low.
func MaxRisk(levels ...RiskLevel) RiskLevel {
	out := RiskLevelLow
	for _, l := range levels {
		if l.Rank() > out.Rank() {
			out = l
		}
	}
	return out
}

// DependencyType describes how one capability binds to another.
type DependencyType string

const (
	DependencyUsesTool      DependencyType = "uses_tool"
	DependencyUsesKnowledge DependencyType = "uses_knowledge"
	DependencyUsesPrompt    DependencyType = "uses_prompt"
	DependencyCallsSkill    DependencyType = "calls_skill"
	DependencyContainsStep  DependencyType = "contains_step"
)

func (d DependencyType) Valid() bool {
	switch d {
	case DependencyUsesTool, DependencyUsesKnowledge, DependencyUsesPrompt,
		DependencyCallsSkill, DependencyContainsStep:
		return true
	}
	return false
}

// ProvisionedAtComposition reports whether a dependency must be pulled into a
// composition alongside the capability that declares it. Tools and knowledge are
// externally provisioned and permissioned, so they surface in the draft; prompts and
// nested steps stay encapsulated inside their owning capability.
func (d DependencyType) ProvisionedAtComposition() bool {
	return d == DependencyUsesTool || d == DependencyUsesKnowledge
}

// HarnessType is the runtime a draft is aimed at. No harness is executed in phase 1.
type HarnessType string

const (
	HarnessDifyWorkflow HarnessType = "dify_workflow"
	HarnessJiuwenSwarm  HarnessType = "jiuwen_swarm"
	HarnessHTTPAgent    HarnessType = "http_agent"
	HarnessManual       HarnessType = "manual"
)

func (h HarnessType) Valid() bool {
	switch h {
	case HarnessDifyWorkflow, HarnessJiuwenSwarm, HarnessHTTPAgent, HarnessManual:
		return true
	}
	return false
}

// Mode is the harness-specific execution mode recorded on the draft.
func (h HarnessType) Mode() string {
	switch h {
	case HarnessJiuwenSwarm:
		return "bounded_dynamic_team"
	case HarnessDifyWorkflow:
		return "sequential_graph"
	case HarnessHTTPAgent:
		return "single_agent"
	default:
		return "manual"
	}
}

// DraftStatus is the lifecycle of a business agent draft. Phase 1 never publishes.
type DraftStatus string

const (
	DraftStatusDraft        DraftStatus = "draft"
	DraftStatusNeedsReview  DraftStatus = "needs_review"
	DraftStatusReadyForEval DraftStatus = "ready_for_eval"
)

func (s DraftStatus) Valid() bool {
	switch s {
	case DraftStatusDraft, DraftStatusNeedsReview, DraftStatusReadyForEval:
		return true
	}
	return false
}

// SourceType is the kind of external system a source points at.
type SourceType string

const (
	SourceTypeDify   SourceType = "dify"
	SourceTypeMCP    SourceType = "mcp"
	SourceTypeData   SourceType = "data"
	SourceTypeManual SourceType = "manual"
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceTypeDify, SourceTypeMCP, SourceTypeData, SourceTypeManual:
		return true
	}
	return false
}

// ComponentRole is the role a capability plays inside a composition, and doubles as
// the YAML section name it renders under.
type ComponentRole string

const (
	RoleAgent         ComponentRole = "agents"
	RoleWorkflow      ComponentRole = "workflows"
	RoleSkill         ComponentRole = "skills"
	RoleTool          ComponentRole = "tools"
	RoleKnowledgeData ComponentRole = "knowledge_data"
	RolePrompt        ComponentRole = "prompts"
	RolePolicy        ComponentRole = "policies"
)

// RoleForType maps a capability type onto its composition section.
func RoleForType(t CapabilityType) ComponentRole {
	switch t {
	case CapabilityTypeAgent:
		return RoleAgent
	case CapabilityTypeWorkflow:
		return RoleWorkflow
	case CapabilityTypeSkill:
		return RoleSkill
	case CapabilityTypeTool:
		return RoleTool
	case CapabilityTypeKnowledgeData:
		return RoleKnowledgeData
	case CapabilityTypePromptTemplate:
		return RolePrompt
	default:
		return RolePolicy
	}
}

// AllComponentRoles is the fixed YAML section order of a composition block.
var AllComponentRoles = []ComponentRole{
	RoleAgent, RoleWorkflow, RoleSkill, RoleTool, RoleKnowledgeData, RolePrompt, RolePolicy,
}

// NormalizeStatusFilter trims and lowercases a caller-supplied filter value.
func NormalizeStatusFilter(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
