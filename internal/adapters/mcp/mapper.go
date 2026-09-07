// Package mcp imports tools exposed by an MCP server or an enterprise tool gateway.
package mcp

import (
	"github.com/open-fin/agent-composer/internal/domain"
)

// ToolDescriptor is the subset of an MCP tool listing that maps onto a capability.
// It matches the shape of an MCP `tools/list` entry, so a real gateway response can be
// decoded straight into it.
type ToolDescriptor struct {
	Name        string         `json:"name" yaml:"name"`
	Title       string         `json:"title" yaml:"title"`
	Description string         `json:"description" yaml:"description"`
	InputSchema map[string]any `json:"inputSchema" yaml:"inputSchema"`
	Annotations map[string]any `json:"annotations" yaml:"annotations"`
}

// Map converts an MCP tool listing into tool candidates.
func Map(server string, tools []ToolDescriptor, sourceID, sourceSystem string) []domain.CapabilityCandidate {
	out := make([]domain.CapabilityCandidate, 0, len(tools))
	for _, tool := range tools {
		name := tool.Title
		if name == "" {
			name = tool.Name
		}
		permissions := stringSlice(tool.Annotations["permissions"])
		out = append(out, domain.CapabilityCandidate{
			SourceID:      sourceID,
			SourceSystem:  sourceSystem,
			ExternalID:    tool.Name,
			CandidateType: domain.CapabilityTypeTool,
			Name:          name,
			Description:   tool.Description,
			RawPayload: map[string]any{
				"server":      server,
				"tool":        tool.Name,
				"annotations": tool.Annotations,
			},
			Extracted: domain.CapabilityFields{
				Name:         name,
				Description:  tool.Description,
				Subtype:      "mcp_tool",
				InputSchema:  tool.InputSchema,
				OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"result": map[string]any{"type": "object"}}},
				Permissions:  permissions,
				Metadata:     map[string]any{"mcp_server": server},
			},
			Status: domain.CandidateStatusExtracted,
		})
	}
	return out
}

func stringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
