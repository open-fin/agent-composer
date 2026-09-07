// Package adapters holds the source-specific importers. Every importer satisfies the
// same interface, so adding a source system never changes the engine.
package adapters

import (
	"context"

	"github.com/open-fin/agent-composer/internal/domain"
)

// ImportInput is what an importer receives: the raw payload plus the source it is
// being attributed to.
type ImportInput struct {
	SourceID     string
	SourceSystem string
	// Payload is the uploaded file for file-based importers, and may be empty for
	// importers that generate fixtures or call a remote API.
	Payload []byte
	Options map[string]any
}

// Importer turns an external payload into capability candidates. Candidates come back
// without ids or timestamps; the engine enriches and persists them.
type Importer interface {
	Name() string
	Import(ctx context.Context, in ImportInput) ([]domain.CapabilityCandidate, error)
}
