package manual

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/domain"
)

// YAMLImporter registers capabilities from a hand-written YAML document.
type YAMLImporter struct {
	LastWarnings []string
}

// NewYAMLImporter builds the manual importer.
func NewYAMLImporter() *YAMLImporter { return &YAMLImporter{} }

func (i *YAMLImporter) Name() string { return "manual-yaml" }

// Import parses and maps a manual capability document.
func (i *YAMLImporter) Import(_ context.Context, in adapters.ImportInput) ([]domain.CapabilityCandidate, error) {
	if len(strings.TrimSpace(string(in.Payload))) == 0 {
		return nil, fmt.Errorf("empty yaml document")
	}
	var doc Document
	if err := yaml.Unmarshal(in.Payload, &doc); err != nil {
		return nil, fmt.Errorf("parse manual yaml: %w", err)
	}
	entries := doc.Entries()
	if len(entries) == 0 {
		return nil, fmt.Errorf("document contains no capabilities; expected a `capabilities:` list or a single capability")
	}
	candidates, warnings, err := Map(entries, in.SourceID, in.SourceSystem)
	if err != nil {
		return nil, err
	}
	i.LastWarnings = warnings
	return candidates, nil
}

var _ adapters.Importer = (*YAMLImporter)(nil)
