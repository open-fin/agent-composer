package engine

import (
	"context"
	"log/slog"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/llm"
	"github.com/open-fin/agent-composer/internal/registry"
	"github.com/open-fin/agent-composer/internal/store"
)

// ImportDifyDSLRequest imports an uploaded Dify DSL document.
type ImportDifyDSLRequest struct {
	// SourceID targets an already-configured source. When empty, SourceName is used to
	// find or create one, so an upload works with no prior configuration.
	SourceID   string
	SourceName string
	Payload    []byte
}

// ImportManualYAMLRequest imports hand-written capability YAML.
type ImportManualYAMLRequest struct {
	SourceID   string
	SourceName string
	Payload    []byte
}

// ImportMockRequest imports from the MCP or data mock adapters. A payload is optional:
// when supplied it is treated as a real gateway or catalog listing.
type ImportMockRequest struct {
	SourceID   string
	SourceName string
	Payload    []byte
	Options    map[string]any
}

// ImportResult is what an import produces.
//
// The candidate list alone would hide the non-fatal findings that matter during a real
// import — nodes skipped, fields missing — so warnings travel with it.
type ImportResult struct {
	Source     domain.ExternalSource        `json:"source"`
	Candidates []domain.CapabilityCandidate `json:"candidates"`
	Warnings   []string                     `json:"warnings"`
}

// GenerateDraftRequest turns a composition plan into a business agent draft.
type GenerateDraftRequest struct {
	Name        string
	Goal        string
	Description string
	HarnessType domain.HarnessType
	Plan        domain.CompositionPlan
}

// CompositionEngine is the whole phase 1 surface: ingest, review, register, recommend,
// validate, generate.
type CompositionEngine interface {
	ImportDifyDSL(ctx context.Context, req ImportDifyDSLRequest) (*ImportResult, error)
	ImportManualYAML(ctx context.Context, req ImportManualYAMLRequest) (*ImportResult, error)
	ImportMockMCP(ctx context.Context, req ImportMockRequest) (*ImportResult, error)
	ImportMockData(ctx context.Context, req ImportMockRequest) (*ImportResult, error)

	ReviewCandidate(ctx context.Context, candidateID string, review domain.ReviewInput) (*domain.CapabilityCandidate, error)
	RegisterCandidate(ctx context.Context, candidateID string) (*domain.Capability, error)
	RejectCandidate(ctx context.Context, candidateID, reason string) (*domain.CapabilityCandidate, error)

	RecommendComposition(ctx context.Context, goal domain.BusinessGoal) (*domain.CompositionPlan, error)
	ValidateComposition(ctx context.Context, plan domain.CompositionPlan) (*domain.CompositionValidationResult, error)
	GenerateDraft(ctx context.Context, req GenerateDraftRequest) (*domain.BusinessAgentDraft, error)
	RenderDraftYAML(ctx context.Context, draftID string) (string, error)
}

// Engine is the concrete CompositionEngine.
type Engine struct {
	sources      *registry.SourceService
	candidates   *registry.CandidateService
	capabilities *registry.CapabilityService
	dependencies *registry.DependencyService
	drafts       *registry.DraftService
	enricher     llm.Enricher
	log          *slog.Logger
}

// Options configures a new engine.
type Options struct {
	Store    store.Store
	Enricher llm.Enricher
	Logger   *slog.Logger
}

// New wires an engine over a store.
func New(opts Options) *Engine {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	enricher := opts.Enricher
	if enricher == nil {
		enricher = llm.NewMockEnricher()
	}
	deps := registry.NewDependencyService(opts.Store.Dependencies(), opts.Store.Capabilities())
	return &Engine{
		sources:      registry.NewSourceService(opts.Store.Sources()),
		candidates:   registry.NewCandidateService(opts.Store.Candidates()),
		capabilities: registry.NewCapabilityService(opts.Store.Capabilities(), deps),
		dependencies: deps,
		drafts:       registry.NewDraftService(opts.Store.Drafts()),
		enricher:     enricher,
		log:          log,
	}
}

// Sources exposes the source service to the API layer.
func (e *Engine) Sources() *registry.SourceService { return e.sources }

// Candidates exposes the candidate service to the API layer.
func (e *Engine) Candidates() *registry.CandidateService { return e.candidates }

// Capabilities exposes the capability service to the API layer.
func (e *Engine) Capabilities() *registry.CapabilityService { return e.capabilities }

// Drafts exposes the draft service to the API layer.
func (e *Engine) Drafts() *registry.DraftService { return e.drafts }

// EnricherName reports which enrichment path is active, so the UI can show whether an
// LLM is in play or the deterministic mock is.
func (e *Engine) EnricherName() string { return e.enricher.Name() }

var _ CompositionEngine = (*Engine)(nil)
