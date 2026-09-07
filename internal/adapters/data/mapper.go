// Package data imports knowledge bases and datasets from an enterprise data catalog.
package data

import (
	"github.com/open-fin/agent-composer/internal/domain"
)

// DatasetDescriptor is one entry of a data catalog listing.
type DatasetDescriptor struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Kind        string   `json:"kind" yaml:"kind"`
	Domain      string   `json:"domain" yaml:"domain"`
	Owner       string   `json:"owner" yaml:"owner"`
	Permissions []string `json:"permissions" yaml:"permissions"`
	Tags        []string `json:"tags" yaml:"tags"`
}

// Map converts a data catalog listing into knowledge_data candidates.
func Map(catalog string, datasets []DatasetDescriptor, sourceID, sourceSystem string) []domain.CapabilityCandidate {
	out := make([]domain.CapabilityCandidate, 0, len(datasets))
	for _, ds := range datasets {
		kind := ds.Kind
		if kind == "" {
			kind = "dataset"
		}
		out = append(out, domain.CapabilityCandidate{
			SourceID:      sourceID,
			SourceSystem:  sourceSystem,
			ExternalID:    ds.ID,
			CandidateType: domain.CapabilityTypeKnowledgeData,
			Name:          ds.Name,
			Description:   ds.Description,
			RawPayload:    map[string]any{"catalog": catalog, "dataset_id": ds.ID, "kind": kind},
			Extracted: domain.CapabilityFields{
				Name:           ds.Name,
				Description:    ds.Description,
				Subtype:        kind,
				BusinessDomain: ds.Domain,
				Tags:           ds.Tags,
				Owner:          ds.Owner,
				Permissions:    ds.Permissions,
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"query": map[string]any{"type": "string"}},
					"required":   []string{"query"},
				},
				OutputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"documents": map[string]any{"type": "array"}},
				},
				Metadata: map[string]any{"catalog": catalog},
			},
			Status: domain.CandidateStatusExtracted,
		})
	}
	return out
}
