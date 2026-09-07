package data

import (
	"context"
	"encoding/json"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/domain"
)

// MockImporter stands in for an enterprise data or knowledge catalog. As with the MCP
// importer, a real catalog listing can be POSTed and is mapped by the same code path.
type MockImporter struct{}

// NewMockImporter builds the data mock importer.
func NewMockImporter() *MockImporter { return &MockImporter{} }

func (i *MockImporter) Name() string { return "data-mock" }

// DefaultCatalog is the catalog name reported for generated fixtures.
const DefaultCatalog = "enterprise-data-catalog"

var mockDatasets = []DatasetDescriptor{
	{
		ID:          "customer-segment-dataset",
		Name:        "Customer Segment Dataset",
		Description: "Quarterly customer segmentation attributes maintained by the analytics team.",
		Kind:        "dataset",
		Domain:      "customer",
		Owner:       "analytics-team",
		Permissions: []string{"customer_profile_read"},
		Tags:        []string{"customer", "segmentation"},
	},
	{
		ID:          "transaction-history-dataset",
		Name:        "Transaction History Dataset",
		Description: "Rolling twelve months of posted account transactions.",
		Kind:        "dataset",
		Domain:      "customer",
		Owner:       "core-banking",
		Permissions: []string{"transaction_read"},
		Tags:        []string{"transaction", "history"},
	},
}

// Import returns the mock catalog, or maps a supplied catalog listing.
func (i *MockImporter) Import(_ context.Context, in adapters.ImportInput) ([]domain.CapabilityCandidate, error) {
	catalog := DefaultCatalog
	if named, ok := in.Options["catalog"].(string); ok && named != "" {
		catalog = named
	}

	datasets := mockDatasets
	if len(in.Payload) > 0 {
		var listing struct {
			Datasets []DatasetDescriptor `json:"datasets"`
		}
		if err := json.Unmarshal(in.Payload, &listing); err != nil {
			return nil, err
		}
		if len(listing.Datasets) > 0 {
			datasets = listing.Datasets
		}
	}
	return Map(catalog, datasets, in.SourceID, in.SourceSystem), nil
}

var _ adapters.Importer = (*MockImporter)(nil)
