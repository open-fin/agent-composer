package llm

import (
	"context"
	"sort"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

// MockEnricher is the deterministic enrichment path. It is the default when no LLM is
// configured and the fallback whenever a configured LLM misbehaves, so identical input
// always produces identical output and the demo is reproducible.
type MockEnricher struct{}

// NewMockEnricher builds the rule-based enricher.
func NewMockEnricher() *MockEnricher { return &MockEnricher{} }

func (m *MockEnricher) Name() string { return "mock" }

// intentRule maps vocabulary found in a capability's name and description onto a
// canonical intent. all terms must be present for the rule to fire.
type intentRule struct {
	intent string
	all    []string
	any    []string
}

var intentRules = []intentRule{
	{intent: "customer_profile_query", any: []string{"crm"}},
	// Both "customer" and "profile" are required: a product catalog lookup that merely
	// mentions the customer must not be classified as a profile query.
	{intent: "customer_profile_query", all: []string{"customer", "profile"}, any: []string{"query", "lookup", "fetch"}},
	{intent: "customer_profile_data", all: []string{"profile"}, any: []string{"knowledge", "kb", "document", "dataset"}},
	{intent: "customer_insight", any: []string{"insight", "analyz", "analys", "segment"}},
	{intent: "product_catalog_query", all: []string{"product"}, any: []string{"catalog", "query", "lookup"}},
	{intent: "product_policy_lookup", all: []string{"policy"}, any: []string{"knowledge", "kb", "document", "suitability"}},
	{intent: "product_recommendation", any: []string{"recommend", "cross-sell", "upsell"}},
	{intent: "campaign_message", any: []string{"campaign", "outreach message", "marketing message"}},
	{intent: "compliance_check", any: []string{"compliance", "regulat"}},
	{intent: "customer_outreach", any: []string{"outreach", "notify", "dispatch"}},
}

// domainRule assigns a business domain from the same vocabulary.
//
// Order matters: the first rule that fires wins. Product is checked before compliance
// so that a product policy knowledge base lands in the product domain rather than the
// compliance one, which is what governs the access permission it needs.
var domainRules = []struct {
	domain string
	any    []string
}{
	{domain: "product", any: []string{"product", "catalog"}},
	{domain: "marketing", any: []string{"campaign", "outreach", "message"}},
	{domain: "compliance", any: []string{"compliance", "regulat", "policy"}},
	{domain: "customer", any: []string{"customer", "crm", "profile", "insight"}},
}

// domainPermissions is the access a capability needs to read from its domain. Only
// tools and datasets reach external systems, so only they carry permissions; an agent
// or skill inherits whatever its bindings require.
var domainPermissions = map[string]string{
	"customer": "customer_profile_read",
	"product":  "product_catalog_read",
}

// Risk vocabulary. High risk is reserved for irreversible or money-moving actions.
var (
	highRiskTerms   = []string{"payment", "transfer", "wire", "delete", "irreversible", "disburse"}
	mediumRiskTerms = []string{"outreach", "external", "compliance", "send", "publish", "customer message"}
)

var tagStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "for": true, "of": true,
	"to": true, "with": true, "that": true, "this": true, "into": true, "from": true,
	"tool": true, "agent": true, "skill": true, "workflow": true, "prompt": true,
	"template": true, "base": true, "node": true,
}

// EnrichCandidate infers the fields a source system does not carry. Every value is
// derived from the candidate's own text, so results are stable across runs.
func (m *MockEnricher) EnrichCandidate(_ context.Context, in CandidateContext) (domain.CapabilityFields, error) {
	haystack := strings.ToLower(in.Name + " " + in.Description + " " + strings.Join(in.Hints, " "))

	businessDomain := inferDomain(haystack)
	fields := domain.CapabilityFields{
		Intents:        inferIntents(haystack),
		Tags:           inferTags(in.Name, in.Type),
		BusinessDomain: businessDomain,
		RiskLevel:      inferRisk(haystack),
		Permissions:    inferPermissions(businessDomain, in.Type),
	}
	if fields.Description == "" && in.Description == "" {
		fields.Description = defaultDescription(in)
	}
	return fields, nil
}

func inferIntents(haystack string) []string {
	seen := map[string]bool{}
	var out []string
	for _, rule := range intentRules {
		matched := true
		for _, term := range rule.all {
			if !strings.Contains(haystack, term) {
				matched = false
				break
			}
		}
		if matched && len(rule.any) > 0 {
			matched = false
			for _, term := range rule.any {
				if strings.Contains(haystack, term) {
					matched = true
					break
				}
			}
		}
		if matched && !seen[rule.intent] {
			seen[rule.intent] = true
			out = append(out, rule.intent)
		}
	}
	return out
}

func inferDomain(haystack string) string {
	for _, rule := range domainRules {
		for _, term := range rule.any {
			if strings.Contains(haystack, term) {
				return rule.domain
			}
		}
	}
	return "general"
}

// inferPermissions grants a tool or dataset read access to its own domain.
func inferPermissions(businessDomain string, capType domain.CapabilityType) []string {
	if capType != domain.CapabilityTypeTool && capType != domain.CapabilityTypeKnowledgeData {
		return nil
	}
	if permission, ok := domainPermissions[businessDomain]; ok {
		return []string{permission}
	}
	return nil
}

func inferRisk(haystack string) domain.RiskLevel {
	for _, term := range highRiskTerms {
		if strings.Contains(haystack, term) {
			return domain.RiskLevelHigh
		}
	}
	for _, term := range mediumRiskTerms {
		if strings.Contains(haystack, term) {
			return domain.RiskLevelMedium
		}
	}
	return domain.RiskLevelLow
}

// inferTags reduces a display name to its distinguishing words, plus the type itself.
func inferTags(name string, capType domain.CapabilityType) []string {
	words := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	out := []string{}
	for _, word := range words {
		if len(word) < 3 || tagStopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	if typeTag := string(capType); !seen[typeTag] {
		out = append(out, typeTag)
	}
	return out
}

func defaultDescription(in CandidateContext) string {
	return "Imported " + string(in.Type) + " " + in.Name + " from " + in.SourceSystem + "."
}

// roleTemplate is one entry in the goal decomposition catalog. A template contributes a
// required role when any of its triggers appears in the goal text.
type roleTemplate struct {
	step        string
	capType     domain.CapabilityType
	description string
	triggers    []string
	keywords    []string
	intents     []string
}

// roleCatalog is ordered: the emitted roles follow this sequence, which is also the
// order the workflow steps are rendered in.
var roleCatalog = []roleTemplate{
	{
		step:        "query_customer_profile",
		capType:     domain.CapabilityTypeTool,
		description: "Retrieve the customer's profile record from the system of record.",
		triggers:    []string{"customer profile", "customer data", "crm", "client profile"},
		keywords:    []string{"crm", "customer", "profile", "query", "lookup"},
		intents:     []string{"customer_profile_query"},
	},
	{
		step:        "retrieve_reference_knowledge",
		capType:     domain.CapabilityTypeKnowledgeData,
		description: "Retrieve supporting documents or reference data.",
		triggers:    []string{"knowledge base", "reference document", "retrieve document", "look up documentation"},
		keywords:    []string{"knowledge", "document", "reference", "retrieval"},
		intents:     []string{"customer_profile_data", "product_policy_lookup"},
	},
	{
		step:        "generate_customer_insight",
		capType:     domain.CapabilityTypeAgent,
		description: "Analyze the customer and produce an insight summary.",
		triggers:    []string{"analyz", "analys", "insight", "understand the customer", "segment"},
		keywords:    []string{"insight", "analysis", "customer", "profile"},
		intents:     []string{"customer_insight"},
	},
	{
		step:        "recommend_product",
		capType:     domain.CapabilityTypeSkill,
		description: "Select products or offers suited to the customer.",
		triggers:    []string{"recommend", "suitable product", "cross-sell", "upsell", "offer"},
		keywords:    []string{"recommend", "product", "suitable", "offer"},
		intents:     []string{"product_recommendation"},
	},
	{
		step:        "draft_campaign_message",
		capType:     domain.CapabilityTypePromptTemplate,
		description: "Draft the customer-facing message.",
		triggers:    []string{"draft", "campaign message", "write a message", "compose", "generate a message"},
		keywords:    []string{"campaign", "message", "draft", "template"},
		intents:     []string{"campaign_message"},
	},
	{
		step:        "compliance_check",
		capType:     domain.CapabilityTypeSkill,
		description: "Validate the output against policy and regulation before it leaves the bank.",
		triggers:    []string{"complian", "regulat", "policy check", "approval check"},
		keywords:    []string{"compliance", "check", "policy", "regulat"},
		intents:     []string{"compliance_check"},
	},
	{
		step:        "orchestrate_workflow",
		capType:     domain.CapabilityTypeWorkflow,
		description: "Sequence the steps end to end.",
		triggers:    []string{"workflow", "orchestrat", "pipeline", "multi-step"},
		keywords:    []string{"workflow", "orchestration", "pipeline"},
		intents:     []string{},
	},
	{
		step:        "enforce_governance_policy",
		capType:     domain.CapabilityTypePolicy,
		description: "Apply the governance policy that gates the action.",
		triggers:    []string{"governance", "guardrail", "approval workflow", "four-eyes"},
		keywords:    []string{"governance", "policy", "approval", "guardrail"},
		intents:     []string{},
	},
	{
		step:        "outreach_send",
		capType:     domain.CapabilityTypeTool,
		description: "Deliver the approved message to the customer over an outreach channel.",
		triggers:    []string{"outreach", "send", "notify", "contact the customer", "deliver"},
		keywords:    []string{"outreach", "send", "channel", "notify", "delivery"},
		intents:     []string{"customer_outreach"},
	},
}

// DecomposeGoal turns the goal text into the ordered roles a composition must cover.
// Roles are emitted whether or not the registry can satisfy them; that is precisely
// what lets the recommender report missing capabilities.
func (m *MockEnricher) DecomposeGoal(_ context.Context, goal domain.BusinessGoal) ([]domain.RequiredRole, error) {
	haystack := strings.ToLower(goal.Goal + " " + goal.Description + " " + goal.Name)

	var roles []domain.RequiredRole
	for _, template := range roleCatalog {
		fired := false
		for _, trigger := range template.triggers {
			if strings.Contains(haystack, trigger) {
				fired = true
				break
			}
		}
		if !fired {
			continue
		}
		roles = append(roles, domain.RequiredRole{
			Step:           template.step,
			CapabilityType: template.capType,
			Description:    template.description,
			Keywords:       append([]string(nil), template.keywords...),
			Intents:        append([]string(nil), template.intents...),
		})
	}

	if len(roles) == 0 {
		// A goal that matches nothing in the catalog still needs one role, otherwise the
		// recommender has nothing to report against.
		roles = append(roles, domain.RequiredRole{
			Step:           "fulfill_goal",
			CapabilityType: domain.CapabilityTypeAgent,
			Description:    "Fulfill the stated business goal end to end.",
			Keywords:       significantWords(goal.Goal),
		})
	}
	return roles, nil
}

// significantWords extracts the content words of a free-text goal, used as match
// keywords when the catalog has nothing specific to offer.
func significantWords(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	out := []string{}
	for _, word := range words {
		if len(word) < 4 || tagStopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	sort.Strings(out)
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// MockClient is a Client stub used to exercise the ClientEnricher path in tests.
type MockClient struct {
	Response string
	Err      error
}

func (c *MockClient) Name() string { return "mock-client" }

func (c *MockClient) Complete(context.Context, Request) (string, error) {
	return c.Response, c.Err
}
