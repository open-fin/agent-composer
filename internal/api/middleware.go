// Package api exposes the composition engine over REST.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/registry"
	"github.com/open-fin/agent-composer/internal/store"
)

// Envelope is the response shape of every JSON endpoint. Exactly one of data or error
// is populated; /metrics is the sole exemption, because Prometheus is not JSON.
type Envelope struct {
	Success bool      `json:"success"`
	Data    any       `json:"data"`
	Error   *APIError `json:"error"`
}

// APIError is the error half of the envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Error codes. The set is closed so clients can branch on it.
const (
	CodeValidation = "VALIDATION_ERROR"
	CodeNotFound   = "NOT_FOUND"
	CodeConflict   = "CONFLICT"
	CodeParse      = "PARSE_ERROR"
	CodeUpstream   = "UPSTREAM_ERROR"
	CodeInternal   = "INTERNAL"
)

// ListResponse wraps a page of results. Paging lives beside the items rather than in
// headers so the whole response survives being logged or replayed.
type ListResponse struct {
	Items  any `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func writeJSON(w http.ResponseWriter, status int, envelope Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope); err != nil {
		slog.Default().Error("write response", "error", err)
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, Envelope{Success: true, Data: data})
}

func writeList(w http.ResponseWriter, items any, page store.Page) {
	writeData(w, http.StatusOK, ListResponse{
		Items: items, Total: page.Total, Limit: page.Limit, Offset: page.Offset,
	})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, Envelope{
		Success: false,
		Error:   &APIError{Code: code, Message: message, Details: details},
	})
}

// writeError maps a domain error onto a status code. Handlers call this rather than
// choosing status codes themselves, so the mapping stays in one place.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, CodeNotFound, err.Error(), nil)
	case errors.Is(err, store.ErrConflict), errors.Is(err, registry.ErrConflict), errors.Is(err, engine.ErrConflict):
		writeAPIError(w, http.StatusConflict, CodeConflict, err.Error(), nil)
	case errors.Is(err, registry.ErrValidation), errors.Is(err, engine.ErrValidation):
		writeAPIError(w, http.StatusBadRequest, CodeValidation, err.Error(), nil)
	default:
		slog.Default().Error("unhandled request error", "error", err)
		writeAPIError(w, http.StatusInternalServerError, CodeInternal, err.Error(), nil)
	}
}

// decodeJSON reads a JSON request body, refusing unknown fields so a typo in a client
// payload is reported instead of silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeAPIError(w, http.StatusBadRequest, CodeParse, "invalid JSON body: "+err.Error(), nil)
		return false
	}
	return true
}

// queryInt reads a bounded integer query parameter.
func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryString(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// metrics is a minimal in-process counter set. A demo does not need a metrics library,
// but it does need /metrics to answer with something real.
type metrics struct {
	mu             sync.Mutex
	requestsTotal  map[string]int64
	requestsByCode map[int]int64
	started        time.Time
}

func newMetrics() *metrics {
	return &metrics{
		requestsTotal:  map[string]int64{},
		requestsByCode: map[int]int64{},
		started:        time.Now(),
	}
}

func (m *metrics) observe(route string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestsTotal[route]++
	m.requestsByCode[status]++
}

func (m *metrics) render() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString("# HELP composer_uptime_seconds Time since the server started.\n")
	b.WriteString("# TYPE composer_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "composer_uptime_seconds %.0f\n", time.Since(m.started).Seconds())

	b.WriteString("# HELP composer_requests_total Requests handled, by route.\n")
	b.WriteString("# TYPE composer_requests_total counter\n")
	for route, count := range m.requestsTotal {
		fmt.Fprintf(&b, "composer_requests_total{route=%q} %d\n", route, count)
	}

	b.WriteString("# HELP composer_responses_total Responses sent, by status code.\n")
	b.WriteString("# TYPE composer_responses_total counter\n")
	for code, count := range m.requestsByCode {
		fmt.Fprintf(&b, "composer_responses_total{code=\"%d\"} %d\n", code, count)
	}
	return b.String()
}

// statusRecorder captures the status code for logging and metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// observability wraps a handler with panic recovery, request logging and metrics. A
// panic must return a well-formed envelope rather than an empty connection.
func observability(next http.Handler, log *slog.Logger, m *metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w}
		start := time.Now()

		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic handling request", "method", r.Method, "path", r.URL.Path, "panic", recovered)
				if recorder.status == 0 {
					writeAPIError(recorder, http.StatusInternalServerError, CodeInternal, "internal error", nil)
				}
			}
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			m.observe(r.Method+" "+r.URL.Path, status)
			log.Info("request",
				"method", r.Method, "path", r.URL.Path,
				"status", status, "duration_ms", time.Since(start).Milliseconds())
		}()

		next.ServeHTTP(recorder, r)
	})
}

// cors allows the Vite dev server to call the API directly during development.
func cors(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
