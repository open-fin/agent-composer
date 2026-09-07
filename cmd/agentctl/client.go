package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/llm"
	"github.com/open-fin/agent-composer/internal/store"
)

// composer is the subset of the engine agentctl drives. Implementing it twice lets the
// same commands run against a live server or entirely in process.
type composer interface {
	ImportDify(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error)
	ImportManual(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error)
	ImportMockMCP(ctx context.Context) (*engine.ImportResult, error)
	ImportMockData(ctx context.Context) (*engine.ImportResult, error)
	ListCandidates(ctx context.Context, status, capType string) ([]domain.CapabilityCandidate, error)
	Register(ctx context.Context, candidateID string) (*domain.Capability, error)
	ListCapabilities(ctx context.Context, capType string) ([]domain.Capability, error)
	Recommend(ctx context.Context, goal domain.BusinessGoal) (*domain.CompositionPlan, error)
	GenerateDraft(ctx context.Context, req engine.GenerateDraftRequest) (*domain.BusinessAgentDraft, error)
	Describe() string
}

// localComposer runs the engine in process against the in-memory store. Useful for a
// dry run of the demo without starting anything.
type localComposer struct{ engine *engine.Engine }

func newLocalComposer() *localComposer {
	return &localComposer{engine: engine.New(engine.Options{
		Store: store.NewMemory(), Enricher: llm.NewMockEnricher(),
	})}
}

func (c *localComposer) Describe() string { return "in-process engine (in-memory store)" }

func (c *localComposer) ImportDify(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error) {
	return c.engine.ImportDifyDSL(ctx, engine.ImportDifyDSLRequest{SourceName: sourceName, Payload: payload})
}

func (c *localComposer) ImportManual(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error) {
	return c.engine.ImportManualYAML(ctx, engine.ImportManualYAMLRequest{SourceName: sourceName, Payload: payload})
}

func (c *localComposer) ImportMockMCP(ctx context.Context) (*engine.ImportResult, error) {
	return c.engine.ImportMockMCP(ctx, engine.ImportMockRequest{})
}

func (c *localComposer) ImportMockData(ctx context.Context) (*engine.ImportResult, error) {
	return c.engine.ImportMockData(ctx, engine.ImportMockRequest{})
}

func (c *localComposer) ListCandidates(ctx context.Context, status, capType string) ([]domain.CapabilityCandidate, error) {
	candidates, _, err := c.engine.Candidates().List(ctx, store.CandidateFilter{
		Status: status, Type: capType, Limit: store.MaxLimit,
	})
	return candidates, err
}

func (c *localComposer) Register(ctx context.Context, candidateID string) (*domain.Capability, error) {
	return c.engine.RegisterCandidate(ctx, candidateID)
}

func (c *localComposer) ListCapabilities(ctx context.Context, capType string) ([]domain.Capability, error) {
	capabilities, _, err := c.engine.Capabilities().List(ctx, store.CapabilityFilter{
		Type: capType, Limit: store.MaxLimit,
	})
	return capabilities, err
}

func (c *localComposer) Recommend(ctx context.Context, goal domain.BusinessGoal) (*domain.CompositionPlan, error) {
	return c.engine.RecommendComposition(ctx, goal)
}

func (c *localComposer) GenerateDraft(ctx context.Context, req engine.GenerateDraftRequest) (*domain.BusinessAgentDraft, error) {
	return c.engine.GenerateDraft(ctx, req)
}

// httpComposer drives a running composer-server over its REST API.
type httpComposer struct {
	baseURL string
	client  *http.Client
}

func newHTTPComposer(baseURL string) *httpComposer {
	return &httpComposer{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *httpComposer) Describe() string { return "composer-server at " + c.baseURL }

// envelope mirrors the server's response shape.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *httpComposer) call(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode %s %s (status %s): %w", method, path, resp.Status, err)
	}
	if !env.Success {
		if env.Error != nil {
			return fmt.Errorf("%s: %s", env.Error.Code, env.Error.Message)
		}
		return fmt.Errorf("%s %s failed with %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

type importBody struct {
	SourceName string `json:"source_name"`
	Content    string `json:"content"`
}

func (c *httpComposer) importTo(ctx context.Context, path, sourceName string, payload []byte) (*engine.ImportResult, error) {
	var result engine.ImportResult
	err := c.call(ctx, http.MethodPost, path, importBody{SourceName: sourceName, Content: string(payload)}, &result)
	return &result, err
}

func (c *httpComposer) ImportDify(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error) {
	return c.importTo(ctx, "/api/v1/import/dify/dsl", sourceName, payload)
}

func (c *httpComposer) ImportManual(ctx context.Context, sourceName string, payload []byte) (*engine.ImportResult, error) {
	return c.importTo(ctx, "/api/v1/import/manual", sourceName, payload)
}

func (c *httpComposer) ImportMockMCP(ctx context.Context) (*engine.ImportResult, error) {
	return c.importTo(ctx, "/api/v1/import/mcp/mock", "", nil)
}

func (c *httpComposer) ImportMockData(ctx context.Context) (*engine.ImportResult, error) {
	return c.importTo(ctx, "/api/v1/import/data/mock", "", nil)
}

type candidateList struct {
	Items []domain.CapabilityCandidate `json:"items"`
}

func (c *httpComposer) ListCandidates(ctx context.Context, status, capType string) ([]domain.CapabilityCandidate, error) {
	var list candidateList
	path := fmt.Sprintf("/api/v1/candidates?limit=%d", store.MaxLimit)
	if status != "" {
		path += "&status=" + status
	}
	if capType != "" {
		path += "&type=" + capType
	}
	err := c.call(ctx, http.MethodGet, path, nil, &list)
	return list.Items, err
}

func (c *httpComposer) Register(ctx context.Context, candidateID string) (*domain.Capability, error) {
	var capability domain.Capability
	err := c.call(ctx, http.MethodPost, "/api/v1/candidates/"+candidateID+"/register", nil, &capability)
	return &capability, err
}

type capabilityList struct {
	Items []domain.Capability `json:"items"`
}

func (c *httpComposer) ListCapabilities(ctx context.Context, capType string) ([]domain.Capability, error) {
	var list capabilityList
	path := fmt.Sprintf("/api/v1/capabilities?limit=%d", store.MaxLimit)
	if capType != "" {
		path += "&type=" + capType
	}
	err := c.call(ctx, http.MethodGet, path, nil, &list)
	return list.Items, err
}

func (c *httpComposer) Recommend(ctx context.Context, goal domain.BusinessGoal) (*domain.CompositionPlan, error) {
	var plan domain.CompositionPlan
	err := c.call(ctx, http.MethodPost, "/api/v1/compositions/recommend", goal, &plan)
	return &plan, err
}

type draftBody struct {
	Name        string                 `json:"name"`
	Goal        string                 `json:"goal"`
	Description string                 `json:"description"`
	HarnessType domain.HarnessType     `json:"harness_type"`
	Plan        domain.CompositionPlan `json:"plan"`
}

func (c *httpComposer) GenerateDraft(ctx context.Context, req engine.GenerateDraftRequest) (*domain.BusinessAgentDraft, error) {
	var draft domain.BusinessAgentDraft
	err := c.call(ctx, http.MethodPost, "/api/v1/drafts", draftBody{
		Name: req.Name, Goal: req.Goal, Description: req.Description,
		HarnessType: req.HarnessType, Plan: req.Plan,
	}, &draft)
	return &draft, err
}
