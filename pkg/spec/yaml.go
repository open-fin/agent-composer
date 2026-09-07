package spec

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Indent is the indentation used for every rendered document. yaml.v3 defaults to
// four spaces, which does not match the house style of the example files.
const Indent = 2

// Render marshals a document to YAML.
func Render(document any) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(Indent)
	if err := encoder.Encode(document); err != nil {
		return "", fmt.Errorf("render yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close yaml encoder: %w", err)
	}
	return buf.String(), nil
}

// ParseBusinessAgent reads a rendered draft back in, which is what makes the generated
// YAML verifiable rather than merely printable.
func ParseBusinessAgent(payload []byte) (BusinessAgentDocument, error) {
	var doc BusinessAgentDocument
	if err := yaml.Unmarshal(payload, &doc); err != nil {
		return doc, fmt.Errorf("parse business agent yaml: %w", err)
	}
	if doc.BusinessAgent.ID == "" {
		return doc, fmt.Errorf("business_agent.id is missing")
	}
	return doc, nil
}
