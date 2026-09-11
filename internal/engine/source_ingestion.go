package engine

import (
	"context"
	"fmt"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/adapters/data"
	"github.com/open-fin/agent-composer/internal/adapters/dify"
	"github.com/open-fin/agent-composer/internal/adapters/manual"
	"github.com/open-fin/agent-composer/internal/adapters/mcp"
	"github.com/open-fin/agent-composer/internal/domain"
)

// ImportDifyDSL imports an uploaded Dify application export.
func (e *Engine) ImportDifyDSL(ctx context.Context, req ImportDifyDSLRequest) (*ImportResult, error) {
	importer := dify.NewDSLImporter()
	return e.runImport(ctx, importFor{
		importer:    importer,
		sourceID:    req.SourceID,
		sourceName:  req.SourceName,
		defaultName: "dify-upload",
		sourceType:  domain.SourceTypeDify,
		payload:     req.Payload,
		warnings:    func() []string { return importer.LastWarnings },
	})
}

// ImportManualYAML imports hand-written capability YAML.
func (e *Engine) ImportManualYAML(ctx context.Context, req ImportManualYAMLRequest) (*ImportResult, error) {
	importer := manual.NewYAMLImporter()
	return e.runImport(ctx, importFor{
		importer:    importer,
		sourceID:    req.SourceID,
		sourceName:  req.SourceName,
		defaultName: "manual-registration",
		sourceType:  domain.SourceTypeManual,
		payload:     req.Payload,
		warnings:    func() []string { return importer.LastWarnings },
	})
}

// ImportMockMCP imports the mock MCP / tool gateway inventory.
func (e *Engine) ImportMockMCP(ctx context.Context, req ImportMockRequest) (*ImportResult, error) {
	return e.runImport(ctx, importFor{
		importer:    mcp.NewMockImporter(),
		sourceID:    req.SourceID,
		sourceName:  req.SourceName,
		defaultName: mcp.DefaultServer,
		sourceType:  domain.SourceTypeMCP,
		payload:     req.Payload,
		options:     req.Options,
	})
}

// ImportMockData imports the mock knowledge / data catalog.
func (e *Engine) ImportMockData(ctx context.Context, req ImportMockRequest) (*ImportResult, error) {
	return e.runImport(ctx, importFor{
		importer:    data.NewMockImporter(),
		sourceID:    req.SourceID,
		sourceName:  req.SourceName,
		defaultName: data.DefaultCatalog,
		sourceType:  domain.SourceTypeData,
		payload:     req.Payload,
		options:     req.Options,
	})
}

// importFor is the shared shape of every import: resolve a source, run the adapter,
// enrich, persist. Adding a source system means adding an adapter, nothing else.
type importFor struct {
	importer    adapters.Importer
	sourceID    string
	sourceName  string
	defaultName string
	sourceType  domain.SourceType
	payload     []byte
	options     map[string]any
	warnings    func() []string
}

func (e *Engine) runImport(ctx context.Context, spec importFor) (*ImportResult, error) {
	source, err := e.resolveSource(ctx, spec.sourceID, spec.sourceName, spec.defaultName, spec.sourceType)
	if err != nil {
		return nil, err
	}

	candidates, err := spec.importer.Import(ctx, adapters.ImportInput{
		SourceID:     source.ID,
		SourceSystem: string(source.Type),
		Payload:      spec.payload,
		Options:      spec.options,
	})
	if err != nil {
		return nil, err
	}

	result := &ImportResult{Source: source.Redacted(), Candidates: []domain.CapabilityCandidate{}}
	if spec.warnings != nil {
		result.Warnings = spec.warnings()
	}
	if len(candidates) == 0 {
		result.Warnings = append(result.Warnings, "import produced no capability candidates")
		return result, nil
	}

	for _, candidate := range e.enrichCandidates(ctx, candidates) {
		stored, err := e.candidates.Upsert(ctx, candidate)
		if err != nil {
			return nil, fmt.Errorf("store candidate %s: %w", candidate.Name, err)
		}
		if stored.Status == domain.CandidateStatusRegistered {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%s (%s) is already in the registry; the existing entry was kept",
					stored.Name, stored.ExternalID))
		}
		result.Candidates = append(result.Candidates, stored)
	}
	return result, nil
}

// resolveSource finds the source an import belongs to, creating it on first use.
func (e *Engine) resolveSource(ctx context.Context, sourceID, sourceName, defaultName string, sourceType domain.SourceType) (domain.ExternalSource, error) {
	if sourceID != "" {
		source, err := e.sources.Get(ctx, sourceID)
		if err != nil {
			return domain.ExternalSource{}, err
		}
		if source.Type != sourceType {
			return domain.ExternalSource{}, fmt.Errorf("source %s is of type %s, not %s", sourceID, source.Type, sourceType)
		}
		return source, nil
	}
	name := sourceName
	if name == "" {
		name = defaultName
	}
	return e.sources.Ensure(ctx, name, sourceType, nil)
}

// ReviewCandidate applies a reviewer's edits to a candidate.
func (e *Engine) ReviewCandidate(ctx context.Context, candidateID string, review domain.ReviewInput) (*domain.CapabilityCandidate, error) {
	candidate, err := e.candidates.Review(ctx, candidateID, review)
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

// RejectCandidate dismisses a candidate.
func (e *Engine) RejectCandidate(ctx context.Context, candidateID, reason string) (*domain.CapabilityCandidate, error) {
	candidate, err := e.candidates.Reject(ctx, candidateID, reason)
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

// RegisterCandidate promotes a reviewed candidate into the capability registry.
func (e *Engine) RegisterCandidate(ctx context.Context, candidateID string) (*domain.Capability, error) {
	candidate, err := e.candidates.Get(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if !candidate.Status.Pending() {
		return nil, fmt.Errorf("%w: candidate %s is already %s", ErrConflict, candidateID, candidate.Status)
	}

	fields := NormalizeFields(candidate.Resolved())
	name := fields.Name
	if name == "" {
		name = candidate.Name
	}
	reusable := true
	if fields.Reusable != nil {
		reusable = *fields.Reusable
	}

	capability := domain.Capability{
		Slug:           SlugFor(candidate.ExternalID, name),
		Name:           name,
		Type:           candidate.CandidateType,
		Subtype:        fields.Subtype,
		Description:    fields.Description,
		SourceSystem:   candidate.SourceSystem,
		ExternalID:     candidate.ExternalID,
		BusinessDomain: fields.BusinessDomain,
		Intents:        fields.Intents,
		Tags:           fields.Tags,
		InputSchema:    fields.InputSchema,
		OutputSchema:   fields.OutputSchema,
		Owner:          fields.Owner,
		Permissions:    fields.Permissions,
		RiskLevel:      fields.RiskLevel,
		Reusable:       reusable,
		Status:         domain.CapabilityStatusActive,
		Metadata:       fields.Metadata,
	}
	capability.ID = domain.NewID(capability.Slug)

	created, err := e.capabilities.Create(ctx, capability, fields.DependsOn)
	if err != nil {
		return nil, err
	}
	if _, err := e.candidates.MarkRegistered(ctx, candidate, created.ID); err != nil {
		return nil, err
	}
	// Anything registered earlier may have been waiting on this capability.
	if err := e.capabilities.RelinkPending(ctx); err != nil {
		e.log.Warn("relinking pending dependencies failed", "error", err)
	}
	return &created, nil
}
