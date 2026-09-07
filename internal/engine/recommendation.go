package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/store"
)

// Scoring weights. Type is a hard filter as well as a score contribution: a role can
// only ever be satisfied by the kind of capability it asks for.
const (
	weightType    = 3.0
	weightIntent  = 3.0
	weightTag     = 2.0
	weightKeyword = 1.0
	weightDomain  = 0.5

	// minScore keeps weak matches out. It sits above weightType on purpose, so that
	// "it happens to be a tool" can never on its own satisfy a role.
	minScore = 4.5
)

// RecommendComposition turns a business goal into a composition plan.
//
// It runs in two stages, and the order matters. First the goal is decomposed into the
// roles a solution must cover — independently of what happens to be in the registry.
// Only then is each role matched against registered capabilities. Roles that nothing
// satisfies are exactly the missing capabilities; a matcher on its own could never
// produce that list, because matching can only return things that exist.
func (e *Engine) RecommendComposition(ctx context.Context, goal domain.BusinessGoal) (*domain.CompositionPlan, error) {
	if strings.TrimSpace(goal.Goal) == "" {
		return nil, fmt.Errorf("%w: business goal text is required", ErrValidation)
	}
	if goal.HarnessType == "" {
		goal.HarnessType = domain.HarnessJiuwenSwarm
	}
	if !goal.HarnessType.Valid() {
		return nil, fmt.Errorf("%w: unknown harness type %q", ErrValidation, goal.HarnessType)
	}

	roles, err := e.enricher.DecomposeGoal(ctx, goal)
	if err != nil {
		return nil, fmt.Errorf("decompose goal: %w", err)
	}

	catalog, _, err := e.capabilities.List(ctx, store.CapabilityFilter{
		Status: string(domain.CapabilityStatusActive),
		Limit:  store.MaxLimit,
	})
	if err != nil {
		return nil, err
	}

	plan := &domain.CompositionPlan{
		Goal:                goal,
		Roles:               roles,
		WorkflowSteps:       []string{},
		MissingCapabilities: []domain.MissingCapability{},
		Warnings:            []domain.Warning{},
	}

	selected := map[string]domain.Capability{}
	var matchedOrder []domain.Capability

	for _, role := range roles {
		best, score, ok := bestMatch(role, catalog)
		if !ok {
			plan.MissingCapabilities = append(plan.MissingCapabilities, domain.MissingCapability{
				Step:         role.Step,
				RequiredType: role.CapabilityType,
				Reason: fmt.Sprintf("no registered %s capability matches this step",
					role.CapabilityType),
				Suggestion: fmt.Sprintf("register a %s capability covering: %s",
					role.CapabilityType, strings.Join(role.Keywords, ", ")),
				Keywords: role.Keywords,
			})
			continue
		}
		plan.WorkflowSteps = append(plan.WorkflowSteps, role.Step)
		if _, already := selected[best.ID]; already {
			// One capability satisfying two steps is fine; do not list it twice.
			continue
		}
		selected[best.ID] = best
		matchedOrder = append(matchedOrder, best)
		plan.Add(domain.CapabilityRef{
			ID: best.ID, Slug: best.Slug, Name: best.Name, Type: best.Type,
			Role: role.Step, Score: round2(score),
			Reason: fmt.Sprintf("matched step %s", role.Step),
		})
	}

	// Pull in the tools and datasets the matched capabilities are bound to. Those are
	// externally provisioned and separately permissioned, so a draft that omitted them
	// would not be deployable; prompts and nested steps stay inside their owner.
	if err := e.addDependencyClosure(ctx, plan, matchedOrder, selected); err != nil {
		return nil, err
	}

	plan.Governance = deriveGovernance(plan, selected)
	plan.Warnings = append(plan.Warnings, buildWarnings(plan, selected)...)
	return plan, nil
}

// addDependencyClosure walks provisioned dependency edges breadth-first from every
// matched capability and adds what it finds to the plan.
func (e *Engine) addDependencyClosure(ctx context.Context, plan *domain.CompositionPlan, matched []domain.Capability, selected map[string]domain.Capability) error {
	queue := append([]domain.Capability{}, matched...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		deps, err := e.dependencies.ListFor(ctx, current.ID)
		if err != nil {
			return err
		}
		for _, dep := range deps {
			if !dep.DependencyType.ProvisionedAtComposition() {
				continue
			}
			if _, already := selected[dep.DependsOnCapabilityID]; already {
				continue
			}
			target, err := e.capabilities.Get(ctx, dep.DependsOnCapabilityID)
			if err != nil {
				// A dangling edge is reported, not fatal.
				plan.Warnings = append(plan.Warnings, domain.Warning{
					Code:           "dependency_unresolved",
					Severity:       domain.SeverityWarning,
					Message:        fmt.Sprintf("%s depends on a capability that is no longer registered", current.Slug),
					CapabilitySlug: current.Slug,
				})
				continue
			}
			selected[target.ID] = target
			plan.Add(domain.CapabilityRef{
				ID: target.ID, Slug: target.Slug, Name: target.Name, Type: target.Type,
				Reason: fmt.Sprintf("required by %s (%s)", current.Slug, dep.DependencyType),
			})
			queue = append(queue, target)
		}
	}
	return nil
}

// bestMatch scores every capability of the role's type and returns the strongest.
func bestMatch(role domain.RequiredRole, catalog []domain.Capability) (domain.Capability, float64, bool) {
	var (
		best      domain.Capability
		bestScore float64
		found     bool
	)
	for _, capability := range catalog {
		score, ok := scoreCapability(role, capability)
		if !ok {
			continue
		}
		if !found || score > bestScore {
			best, bestScore, found = capability, score, true
		}
	}
	return best, bestScore, found
}

// scoreCapability rates one capability against one role. It returns false when the
// capability is the wrong type, when nothing but the type matched, or when the total
// falls below the threshold.
func scoreCapability(role domain.RequiredRole, capability domain.Capability) (float64, bool) {
	if capability.Type != role.CapabilityType {
		return 0, false
	}

	vocabulary := UnionSorted(role.Keywords, role.Intents)
	haystack := strings.ToLower(strings.Join([]string{
		capability.Name, capability.Description, capability.Slug,
	}, " "))

	intentScore := overlapRatio(capability.Intents, role.Intents)
	tagScore := overlapRatio(capability.Tags, vocabulary)
	keywordScore := textHitRatio(haystack, role.Keywords)

	domainScore := 0.0
	if capability.BusinessDomain != "" && contains(vocabulary, capability.BusinessDomain) {
		domainScore = 1
	}

	// Type agreement alone is not evidence. Without at least one substantive signal the
	// capability is not a match at any score.
	if intentScore == 0 && tagScore == 0 && keywordScore == 0 {
		return 0, false
	}

	total := weightType +
		weightIntent*intentScore +
		weightTag*tagScore +
		weightKeyword*keywordScore +
		weightDomain*domainScore
	if total < minScore {
		return 0, false
	}
	return total, true
}

// overlapRatio is the share of `wanted` that `have` covers.
func overlapRatio(have, wanted []string) float64 {
	if len(wanted) == 0 {
		return 0
	}
	present := map[string]bool{}
	for _, value := range have {
		present[strings.ToLower(value)] = true
	}
	hits := 0
	for _, value := range wanted {
		if present[strings.ToLower(value)] {
			hits++
		}
	}
	return float64(hits) / float64(len(wanted))
}

// textHitRatio is the share of keywords appearing anywhere in the capability's text.
func textHitRatio(haystack string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	hits := 0
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(haystack, strings.ToLower(keyword)) {
			hits++
		}
	}
	return float64(hits) / float64(len(keywords))
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// deriveGovernance computes the draft's governance from the capabilities it selects.
// Nothing here is hand-entered: risk is the highest risk in the composition,
// permissions are the union of what the parts need, and approval gates are whatever
// the parts declare.
func deriveGovernance(plan *domain.CompositionPlan, selected map[string]domain.Capability) domain.GovernanceConfig {
	governance := domain.GovernanceConfig{
		RiskLevel:        domain.RiskLevelLow,
		ApprovalRequired: []string{},
		Permissions:      []string{},
	}
	var (
		permissions [][]string
		approvals   [][]string
		risks       []domain.RiskLevel
	)
	for _, ref := range plan.AllRefs() {
		capability, ok := selected[ref.ID]
		if !ok {
			continue
		}
		permissions = append(permissions, capability.Permissions)
		approvals = append(approvals, capability.ApprovalRequired())
		risks = append(risks, capability.RiskLevel)
	}
	governance.RiskLevel = domain.MaxRisk(risks...)
	governance.Permissions = UnionSorted(permissions...)
	governance.ApprovalRequired = UnionSorted(approvals...)
	return governance
}

// buildWarnings reports what a reviewer should look at before trusting the plan.
func buildWarnings(plan *domain.CompositionPlan, selected map[string]domain.Capability) []domain.Warning {
	warnings := []domain.Warning{}

	slugsByType := map[string][]string{}
	for _, ref := range plan.AllRefs() {
		capability, ok := selected[ref.ID]
		if !ok {
			continue
		}
		key := string(capability.Type) + "/" + capability.Slug
		slugsByType[key] = append(slugsByType[key], capability.ID)

		if capability.RiskLevel == domain.RiskLevelHigh {
			warnings = append(warnings, domain.Warning{
				Code:           "high_risk_capability",
				Severity:       domain.SeverityWarning,
				Message:        fmt.Sprintf("%s is classified high risk and needs sign-off before use", capability.Slug),
				CapabilitySlug: capability.Slug,
			})
		}
		if !capability.Reusable {
			warnings = append(warnings, domain.Warning{
				Code:           "not_reusable",
				Severity:       domain.SeverityWarning,
				Message:        fmt.Sprintf("%s is marked not reusable but is being composed into a new agent", capability.Slug),
				CapabilitySlug: capability.Slug,
			})
		}
		if len(capability.Intents) == 0 {
			warnings = append(warnings, domain.Warning{
				Code:           "no_intents",
				Severity:       domain.SeverityInfo,
				Message:        fmt.Sprintf("%s has no intents, so it can only be matched by keyword", capability.Slug),
				CapabilitySlug: capability.Slug,
			})
		}
	}

	for key, ids := range slugsByType {
		if len(ids) > 1 {
			warnings = append(warnings, domain.Warning{
				Code:     "duplicate_slug",
				Severity: domain.SeverityError,
				Message:  fmt.Sprintf("%d capabilities share the identifier %s", len(ids), key),
			})
		}
	}

	for _, missing := range plan.MissingCapabilities {
		warnings = append(warnings, domain.Warning{
			Code:     "missing_capability",
			Severity: domain.SeverityWarning,
			Message:  fmt.Sprintf("step %s has no registered %s capability", missing.Step, missing.RequiredType),
			Step:     missing.Step,
		})
	}

	sort.SliceStable(warnings, func(i, j int) bool { return warnings[i].Code < warnings[j].Code })
	return warnings
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
