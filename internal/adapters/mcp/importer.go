package mcp

import (
	"context"
	"encoding/json"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/domain"
)

// MockImporter stands in for a live MCP server or tool gateway. It returns a fixed
// inventory of enterprise tools so the tool-import path can be demonstrated without a
// gateway to connect to. A caller may also POST a real `tools/list` payload, which is
// decoded and mapped by the same code that a live gateway would use.
type MockImporter struct{}

// NewMockImporter builds the MCP mock importer.
func NewMockImporter() *MockImporter { return &MockImporter{} }

func (i *MockImporter) Name() string { return "mcp-mock" }

// DefaultServer is the server name reported for generated fixtures.
const DefaultServer = "enterprise-tool-gateway"

// mockTools is a small, deliberately non-overlapping inventory: it covers core banking
// lookups only. In particular it exposes no customer outreach channel, which is what
// leaves the RM campaign demo with a genuine capability gap to report.
var mockTools = []ToolDescriptor{
	{
		Name:        "account-balance-query",
		Title:       "Account Balance Query Tool",
		Description: "Returns the current and available balance for an account number.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"account_number": map[string]any{"type": "string"}},
			"required":   []any{"account_number"},
		},
		Annotations: map[string]any{"permissions": []any{"account_read"}},
	},
	{
		Name:        "fx-rate-lookup",
		Title:       "FX Rate Lookup Tool",
		Description: "Returns the indicative exchange rate for a currency pair.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"pair": map[string]any{"type": "string"}},
			"required":   []any{"pair"},
		},
		Annotations: map[string]any{"permissions": []any{"market_data_read"}},
	},
	{
		Name:        "document-ocr",
		Title:       "Document OCR Tool",
		Description: "Extracts structured text from a scanned document image.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"document_url": map[string]any{"type": "string"}},
			"required":   []any{"document_url"},
		},
		Annotations: map[string]any{"permissions": []any{"document_read"}},
	},
}

// Import returns the mock inventory, or maps a supplied MCP tools/list payload.
func (i *MockImporter) Import(_ context.Context, in adapters.ImportInput) ([]domain.CapabilityCandidate, error) {
	server := DefaultServer
	if named, ok := in.Options["server"].(string); ok && named != "" {
		server = named
	}

	tools := mockTools
	if len(in.Payload) > 0 {
		var listing struct {
			Tools []ToolDescriptor `json:"tools"`
		}
		if err := json.Unmarshal(in.Payload, &listing); err != nil {
			return nil, err
		}
		if len(listing.Tools) > 0 {
			tools = listing.Tools
		}
	}
	return Map(server, tools, in.SourceID, in.SourceSystem), nil
}

var _ adapters.Importer = (*MockImporter)(nil)
