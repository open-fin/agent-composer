package api

import (
	"net/http"
	"time"
)

// healthz reports that the process is up. It never touches the database, so it stays
// truthful while the database is still starting.
func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"service":  "composer-server",
		"uptime_s": int(time.Since(s.started).Seconds()),
	})
}

// readyz reports whether the server can serve traffic, which means the store answers.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, CodeUpstream, "store is not reachable: "+err.Error(), nil)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"store":    s.storeKind,
		"enricher": s.engine.EnricherName(),
	})
}

// metricsHandler serves Prometheus text format. It is deliberately outside the JSON
// envelope: Prometheus cannot parse one.
func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(s.metrics.render()))
}
