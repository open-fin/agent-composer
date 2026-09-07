// Package manual imports capabilities that an enterprise registers by hand, as YAML.
package manual

import (
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

// Document is the manual registration format. A file may hold a single capability at
// the top level, or a list under `capabilities:`.
type Document struct {
	Capabilities []Entry `yaml:"capabilities"`

	// Inline single-capability form.
	Entry `yaml:",inline"`
}

// Entry is one hand-registered capability.
type Entry struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Type           string         `yaml:"type"`
	Subtype        string         `yaml:"subtype"`
	Description    string         `yaml:"description"`
	BusinessDomain string         `yaml:"business_domain"`
	Intents        []string       `yaml:"intents"`
	Tags           []string       `yaml:"tags"`
	Owner          string         `yaml:"owner"`
	Permissions    []string       `yaml:"permissions"`
	RiskLevel      string         `yaml:"risk_level"`
	Reusable       *bool          `yaml:"reusable"`
	InputSchema    map[string]any `yaml:"input_schema"`
	OutputSchema   map[string]any `yaml:"output_schema"`
	Metadata       map[string]any `yaml:"metadata"`
	DependsOn      []DependsOn    `yaml:"depends_on"`
}

// DependsOn is an unresolved edge to another capability, by its external id.
type DependsOn struct {
	ExternalID string         `yaml:"external_id"`
	TargetType string         `yaml:"target_type"`
	Type       string         `yaml:"dependency_type"`
	Config     map[string]any `yaml:"config"`
}

// Entries flattens the two accepted document shapes into one list.
func (d Document) Entries() []Entry {
	if len(d.Capabilities) > 0 {
		return d.Capabilities
	}
	if d.Entry.Name != "" || d.Entry.ID != "" {
		return []Entry{d.Entry}
	}
	return nil
}

// Map converts parsed entries into candidates.
//
// Manually registered capabilities arrive complete, so every field is `extracted` and
// nothing needs inferring. Entries missing a required field are marked needs_review
// rather than rejected, so a reviewer can finish them in the UI.
func Map(entries []Entry, sourceID, sourceSystem string) ([]domain.CapabilityCandidate, []string, error) {
	var (
		candidates []domain.CapabilityCandidate
		warnings   []string
	)
	for i, entry := range entries {
		capType := domain.CapabilityType(strings.TrimSpace(entry.Type))
		if !capType.Valid() {
			return nil, warnings, fmt.Errorf("capability %d (%s): unknown type %q", i+1, entry.Name, entry.Type)
		}
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			return nil, warnings, fmt.Errorf("capability %d: name is required", i+1)
		}
		externalID := strings.TrimSpace(entry.ID)
		if externalID == "" {
			externalID = domain.Slugify(name)
		}

		risk := domain.RiskLevel(strings.TrimSpace(entry.RiskLevel))
		status := domain.CandidateStatusExtracted
		if entry.Description == "" {
			warnings = append(warnings, fmt.Sprintf("%s has no description", name))
			status = domain.CandidateStatusNeedsReview
		}
		if risk != "" && !risk.Valid() {
			return nil, warnings, fmt.Errorf("capability %s: invalid risk_level %q", name, entry.RiskLevel)
		}

		deps, err := mapDependencies(name, entry.DependsOn)
		if err != nil {
			return nil, warnings, err
		}

		candidates = append(candidates, domain.CapabilityCandidate{
			SourceID:      sourceID,
			SourceSystem:  sourceSystem,
			ExternalID:    externalID,
			CandidateType: capType,
			Name:          name,
			Description:   entry.Description,
			RawPayload:    map[string]any{"format": "manual_yaml", "id": externalID},
			Extracted: domain.CapabilityFields{
				Name:           name,
				Description:    entry.Description,
				Subtype:        entry.Subtype,
				BusinessDomain: entry.BusinessDomain,
				Intents:        entry.Intents,
				Tags:           entry.Tags,
				Owner:          entry.Owner,
				Permissions:    entry.Permissions,
				RiskLevel:      risk,
				Reusable:       entry.Reusable,
				InputSchema:    entry.InputSchema,
				OutputSchema:   entry.OutputSchema,
				Metadata:       entry.Metadata,
				DependsOn:      deps,
			},
			Status: status,
		})
	}
	return candidates, warnings, nil
}

func mapDependencies(owner string, refs []DependsOn) ([]domain.DependencyRef, error) {
	var out []domain.DependencyRef
	for _, ref := range refs {
		depType := domain.DependencyType(strings.TrimSpace(ref.Type))
		if !depType.Valid() {
			return nil, fmt.Errorf("capability %s: invalid dependency_type %q", owner, ref.Type)
		}
		targetType := domain.CapabilityType(strings.TrimSpace(ref.TargetType))
		if ref.TargetType != "" && !targetType.Valid() {
			return nil, fmt.Errorf("capability %s: invalid dependency target_type %q", owner, ref.TargetType)
		}
		if strings.TrimSpace(ref.ExternalID) == "" {
			return nil, fmt.Errorf("capability %s: dependency external_id is required", owner)
		}
		out = append(out, domain.DependencyRef{
			ExternalID:     ref.ExternalID,
			TargetType:     targetType,
			DependencyType: depType,
			Config:         ref.Config,
		})
	}
	return out, nil
}
