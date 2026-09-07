package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/open-fin/agent-composer/internal/config"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

// Server holds everything the handlers need.
type Server struct {
	engine         *engine.Engine
	store          store.Store
	storeKind      string
	log            *slog.Logger
	metrics        *metrics
	maxUploadBytes int64
	corsOrigin     string
	started        time.Time
}

// ServerOptions configures a new API server.
type ServerOptions struct {
	Engine    *engine.Engine
	Store     store.Store
	StoreKind string
	Config    config.Config
	Logger    *slog.Logger
}

// NewServer builds the API server.
func NewServer(opts ServerOptions) *Server {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	maxUpload := opts.Config.HTTP.MaxUploadBytes
	if maxUpload <= 0 {
		maxUpload = 8 << 20
	}
	kind := opts.StoreKind
	if kind == "" {
		kind = "memory"
	}
	return &Server{
		engine:         opts.Engine,
		store:          opts.Store,
		storeKind:      kind,
		log:            log,
		metrics:        newMetrics(),
		maxUploadBytes: maxUpload,
		corsOrigin:     opts.Config.HTTP.CORSOrigin,
		started:        time.Now(),
	}
}

// Handler builds the routing table.
//
// Routing uses the Go 1.22 ServeMux patterns (method plus path wildcards) rather than a
// third-party router, which keeps the dependency list to three modules.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Operational endpoints. /metrics is the one route outside the JSON envelope.
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /metrics", s.metricsHandler)

	// Sources.
	mux.HandleFunc("POST /api/v1/sources", s.createSource)
	mux.HandleFunc("GET /api/v1/sources", s.listSources)
	mux.HandleFunc("GET /api/v1/sources/{id}", s.getSource)

	// Imports.
	mux.HandleFunc("POST /api/v1/import/dify/dsl", s.importDifyDSL)
	mux.HandleFunc("POST /api/v1/import/manual", s.importManual)
	mux.HandleFunc("POST /api/v1/import/mcp/mock", s.importMockMCP)
	mux.HandleFunc("POST /api/v1/import/data/mock", s.importMockData)

	// Candidates.
	mux.HandleFunc("GET /api/v1/candidates", s.listCandidates)
	mux.HandleFunc("GET /api/v1/candidates/{id}", s.getCandidate)
	mux.HandleFunc("POST /api/v1/candidates/{id}/review", s.reviewCandidate)
	mux.HandleFunc("POST /api/v1/candidates/{id}/register", s.registerCandidate)
	mux.HandleFunc("POST /api/v1/candidates/{id}/reject", s.rejectCandidate)

	// Capabilities.
	mux.HandleFunc("GET /api/v1/capabilities", s.listCapabilities)
	mux.HandleFunc("POST /api/v1/capabilities", s.createCapability)
	mux.HandleFunc("GET /api/v1/capabilities/{id}", s.getCapability)
	mux.HandleFunc("PUT /api/v1/capabilities/{id}", s.updateCapability)
	mux.HandleFunc("GET /api/v1/capabilities/{id}/dependencies", s.getCapabilityDependencies)

	// Compositions.
	mux.HandleFunc("POST /api/v1/compositions/recommend", s.recommendComposition)
	mux.HandleFunc("POST /api/v1/compositions/validate", s.validateComposition)

	// Drafts.
	mux.HandleFunc("POST /api/v1/drafts", s.createDraft)
	mux.HandleFunc("GET /api/v1/drafts", s.listDrafts)
	mux.HandleFunc("GET /api/v1/drafts/{id}", s.getDraft)
	mux.HandleFunc("PUT /api/v1/drafts/{id}", s.updateDraft)
	mux.HandleFunc("GET /api/v1/drafts/{id}/yaml", s.getDraftYAML)

	// An unrouted path still answers in the standard envelope.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, CodeNotFound, "no route for "+r.Method+" "+r.URL.Path, nil)
	})

	return cors(observability(mux, s.log, s.metrics), s.corsOrigin)
}
