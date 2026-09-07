package dify

import (
	"context"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/domain"
)

// DSLImporter imports an uploaded Dify DSL document.
type DSLImporter struct {
	// LastWarnings holds the non-fatal findings of the most recent import, so the API
	// can surface "3 nodes were skipped" without failing the upload.
	LastWarnings []string
}

// NewDSLImporter builds the DSL importer.
func NewDSLImporter() *DSLImporter { return &DSLImporter{} }

func (i *DSLImporter) Name() string { return "dify-dsl" }

// Import parses the DSL and fans it out into candidates.
func (i *DSLImporter) Import(_ context.Context, in adapters.ImportInput) ([]domain.CapabilityCandidate, error) {
	parsed, err := Parse(in.Payload)
	if err != nil {
		return nil, err
	}
	mapped := Map(parsed.DSL, in.SourceID, in.SourceSystem)
	i.LastWarnings = append(append([]string{}, parsed.Warnings...), mapped.Warnings...)
	return mapped.Candidates, nil
}

var _ adapters.Importer = (*DSLImporter)(nil)
