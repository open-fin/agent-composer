package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-fin/agent-composer/internal/api"
	"github.com/open-fin/agent-composer/internal/config"
	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/store"
)

const examplesDir = "../../examples"

const rmGoal = "Create an RM campaign agent that analyzes customer profile, recommends " +
	"suitable products, drafts a campaign message, and checks compliance before outreach."

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	memory := store.NewMemory()
	composer := engine.New(engine.Options{Store: memory})
	server := api.NewServer(api.ServerOptions{
		Engine: composer, Store: memory, StoreKind: "memory", Config: config.Default(),
	})
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	return httpServer
}

// envelope mirrors the documented response shape so the tests assert on it directly.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func do(t *testing.T, srv *httptest.Server, method, path string, body any) (int, envelope) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("%s %s: decode envelope: %v", method, path, err)
	}
	return resp.StatusCode, env
}

func mustData(t *testing.T, env envelope, dst any) {
	t.Helper()
	if !env.Success {
		t.Fatalf("request failed: %+v", env.Error)
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func readExample(t *testing.T, parts ...string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(append([]string{examplesDir}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

// TestEveryResponseUsesTheEnvelope covers the API contract that clients depend on.
func TestEveryResponseUsesTheEnvelope(t *testing.T) {
	srv := newTestServer(t)

	for _, tc := range []struct {
		method string
		path   string
		body   any
		status int
	}{
		{http.MethodGet, "/healthz", nil, http.StatusOK},
		{http.MethodGet, "/readyz", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/sources", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/candidates", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/capabilities", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/drafts", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/capabilities/does-not-exist", nil, http.StatusNotFound},
		{http.MethodGet, "/api/v1/nope", nil, http.StatusNotFound},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			status, env := do(t, srv, tc.method, tc.path, tc.body)
			if status != tc.status {
				t.Errorf("status = %d, want %d (%+v)", status, tc.status, env.Error)
			}
			if env.Success != (tc.status < 400) {
				t.Errorf("success = %v for status %d", env.Success, status)
			}
			if env.Success && env.Error != nil {
				t.Error("a successful response must carry no error")
			}
			if !env.Success && env.Error == nil {
				t.Error("a failed response must carry an error")
			}
		})
	}
}

// TestMetricsIsExemptFromTheEnvelope documents the one deliberate exception: Prometheus
// cannot parse JSON.
func TestMetricsIsExemptFromTheEnvelope(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.Client().Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("content type = %s, want Prometheus text format", contentType)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "composer_requests_total") {
		t.Errorf("metrics output is missing the request counter:\n%s", buf.String())
	}
}

// TestNotFoundAndConflictMapToStatusCodes pins the error taxonomy.
func TestNotFoundAndConflictMapToStatusCodes(t *testing.T) {
	srv := newTestServer(t)

	status, env := do(t, srv, http.MethodGet, "/api/v1/drafts/missing", nil)
	if status != http.StatusNotFound || env.Error.Code != api.CodeNotFound {
		t.Errorf("missing draft: status %d code %v", status, env.Error)
	}

	capability := map[string]any{
		"name": "CRM Query Tool", "type": "tool", "slug": "crm-query",
		"description": "Queries the CRM.", "risk_level": "low",
	}
	if status, env := do(t, srv, http.MethodPost, "/api/v1/capabilities", capability); status != http.StatusCreated {
		t.Fatalf("create capability: status %d %+v", status, env.Error)
	}
	status, env = do(t, srv, http.MethodPost, "/api/v1/capabilities", capability)
	if status != http.StatusConflict || env.Error.Code != api.CodeConflict {
		t.Errorf("duplicate capability: status %d code %v", status, env.Error)
	}

	status, env = do(t, srv, http.MethodPost, "/api/v1/capabilities", map[string]any{"type": "tool"})
	if status != http.StatusBadRequest || env.Error.Code != api.CodeValidation {
		t.Errorf("invalid capability: status %d code %v", status, env.Error)
	}
}

// TestFullDemoOverHTTP walks the entire demo through the REST API, which is what the
// web UI does.
func TestFullDemoOverHTTP(t *testing.T) {
	srv := newTestServer(t)

	// Import both Dify apps and both manual capabilities.
	for _, file := range []string{"customer-insight-agent.dsl.yaml", "product-recommendation-workflow.dsl.yaml"} {
		status, env := do(t, srv, http.MethodPost, "/api/v1/import/dify/dsl", map[string]any{
			"source_name": "customer-dify",
			"content":     readExample(t, "dify", file),
		})
		if status != http.StatusOK {
			t.Fatalf("import %s: status %d %+v", file, status, env.Error)
		}
	}
	for _, file := range []string{"compliance-check-skill.yaml", "campaign-message-template.yaml"} {
		status, env := do(t, srv, http.MethodPost, "/api/v1/import/manual", map[string]any{
			"source_name": "manual-registration",
			"content":     readExample(t, "capabilities", file),
		})
		if status != http.StatusOK {
			t.Fatalf("import %s: status %d %+v", file, status, env.Error)
		}
	}

	// Register everything, bindings first.
	for _, capType := range []string{"tool", "knowledge_data", "prompt_template", "skill", "workflow", "agent"} {
		_, env := do(t, srv, http.MethodGet, "/api/v1/candidates?limit=500&type="+capType, nil)
		var list struct {
			Items []domain.CapabilityCandidate `json:"items"`
		}
		mustData(t, env, &list)
		for _, candidate := range list.Items {
			if !candidate.Status.Pending() {
				continue
			}
			status, env := do(t, srv, http.MethodPost, "/api/v1/candidates/"+candidate.ID+"/register", nil)
			if status != http.StatusCreated {
				t.Fatalf("register %s: status %d %+v", candidate.Name, status, env.Error)
			}
		}
	}

	// The catalog filter the UI uses must work.
	_, env := do(t, srv, http.MethodGet, "/api/v1/capabilities?type=tool", nil)
	var tools struct {
		Items []domain.Capability `json:"items"`
		Total int                 `json:"total"`
	}
	mustData(t, env, &tools)
	if tools.Total != 2 {
		t.Errorf("tool filter returned %d capabilities, want 2", tools.Total)
	}

	// Recommend.
	status, env := do(t, srv, http.MethodPost, "/api/v1/compositions/recommend", map[string]any{
		"name": "RM Campaign Agent", "goal": rmGoal, "harness_type": "jiuwen_swarm",
	})
	if status != http.StatusOK {
		t.Fatalf("recommend: status %d %+v", status, env.Error)
	}
	var plan domain.CompositionPlan
	mustData(t, env, &plan)
	if len(plan.MissingCapabilities) != 1 {
		t.Errorf("missing capabilities = %+v, want one gap", plan.MissingCapabilities)
	}

	// Validate.
	status, env = do(t, srv, http.MethodPost, "/api/v1/compositions/validate", map[string]any{"plan": plan})
	if status != http.StatusOK {
		t.Fatalf("validate: status %d %+v", status, env.Error)
	}
	var validation domain.CompositionValidationResult
	mustData(t, env, &validation)
	if !validation.Valid {
		t.Fatalf("plan should validate: %+v", validation.Errors)
	}

	// Generate the draft.
	status, env = do(t, srv, http.MethodPost, "/api/v1/drafts", map[string]any{
		"name": "RM Campaign Agent", "goal": rmGoal, "harness_type": "jiuwen_swarm", "plan": plan,
	})
	if status != http.StatusCreated {
		t.Fatalf("create draft: status %d %+v", status, env.Error)
	}
	var draft domain.BusinessAgentDraft
	mustData(t, env, &draft)
	if draft.Slug != "rm-campaign-agent" {
		t.Errorf("draft slug = %s", draft.Slug)
	}

	// Export the YAML, both enveloped and as a download.
	_, env = do(t, srv, http.MethodGet, "/api/v1/drafts/"+draft.ID+"/yaml", nil)
	var exported struct {
		YAML     string `json:"yaml"`
		Filename string `json:"filename"`
	}
	mustData(t, env, &exported)
	if !strings.Contains(exported.YAML, "id: rm-campaign-agent") {
		t.Errorf("exported YAML looks wrong:\n%s", exported.YAML)
	}
	if exported.Filename != "rm-campaign-agent.yaml" {
		t.Errorf("filename = %s", exported.Filename)
	}

	resp, err := srv.Client().Get(srv.URL + "/api/v1/drafts/" + draft.ID + "/yaml?download=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if disposition := resp.Header.Get("Content-Disposition"); !strings.Contains(disposition, "rm-campaign-agent.yaml") {
		t.Errorf("download disposition = %q", disposition)
	}
}

// TestMultipartUploadIsAccepted covers the path the web UI's file picker uses.
func TestMultipartUploadIsAccepted(t *testing.T) {
	srv := newTestServer(t)

	body := new(bytes.Buffer)
	boundary := "testboundary"
	fmt.Fprintf(body, "--%s\r\n", boundary)
	fmt.Fprint(body, "Content-Disposition: form-data; name=\"source_name\"\r\n\r\ncustomer-dify\r\n")
	fmt.Fprintf(body, "--%s\r\n", boundary)
	fmt.Fprint(body, "Content-Disposition: form-data; name=\"file\"; filename=\"agent.dsl.yaml\"\r\n")
	fmt.Fprint(body, "Content-Type: application/x-yaml\r\n\r\n")
	body.WriteString(readExample(t, "dify", "customer-insight-agent.dsl.yaml"))
	fmt.Fprintf(body, "\r\n--%s--\r\n", boundary)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/import/dify/dsl", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("multipart upload: status %d %+v", resp.StatusCode, env.Error)
	}
	var result engine.ImportResult
	mustData(t, env, &result)
	if len(result.Candidates) != 6 {
		t.Errorf("multipart upload produced %d candidates, want 6", len(result.Candidates))
	}
}

// TestSourceAPIRedactsSecrets keeps a configured Dify key from being read back.
func TestSourceAPIRedactsSecrets(t *testing.T) {
	srv := newTestServer(t)

	status, env := do(t, srv, http.MethodPost, "/api/v1/sources", map[string]any{
		"name": "customer-dify", "type": "dify", "base_url": "https://dify.internal",
		"auth_type": "bearer", "config": map[string]any{"api_key": "app-secret", "workspace": "retail"},
	})
	if status != http.StatusCreated {
		t.Fatalf("create source: status %d %+v", status, env.Error)
	}
	var created domain.ExternalSource
	mustData(t, env, &created)
	if created.Config["api_key"] == "app-secret" {
		t.Error("the API echoed the secret back on create")
	}

	_, env = do(t, srv, http.MethodGet, "/api/v1/sources/"+created.ID, nil)
	var fetched domain.ExternalSource
	mustData(t, env, &fetched)
	if fetched.Config["api_key"] == "app-secret" {
		t.Error("the API returned the secret on read")
	}
	if fetched.Config["workspace"] != "retail" {
		t.Error("redaction removed a non-secret value")
	}
}

// TestReviewThenRegisterOverHTTP covers the review panel workflow.
func TestReviewThenRegisterOverHTTP(t *testing.T) {
	srv := newTestServer(t)

	status, env := do(t, srv, http.MethodPost, "/api/v1/import/manual", map[string]any{
		"source_name": "manual-registration",
		"content":     readExample(t, "capabilities", "compliance-check-skill.yaml"),
	})
	if status != http.StatusOK {
		t.Fatalf("import: status %d %+v", status, env.Error)
	}
	var result engine.ImportResult
	mustData(t, env, &result)
	id := result.Candidates[0].ID

	status, env = do(t, srv, http.MethodPost, "/api/v1/candidates/"+id+"/review", map[string]any{
		"fields": map[string]any{"owner": "risk-office", "risk_level": "high"},
		"note":   "escalated after the audit",
	})
	if status != http.StatusOK {
		t.Fatalf("review: status %d %+v", status, env.Error)
	}
	var reviewed struct {
		Resolved domain.CapabilityFields `json:"resolved"`
	}
	mustData(t, env, &reviewed)
	if reviewed.Resolved.Owner != "risk-office" || reviewed.Resolved.RiskLevel != domain.RiskLevelHigh {
		t.Fatalf("review did not take effect: %+v", reviewed.Resolved)
	}

	status, env = do(t, srv, http.MethodPost, "/api/v1/candidates/"+id+"/register", nil)
	if status != http.StatusCreated {
		t.Fatalf("register: status %d %+v", status, env.Error)
	}
	var capability domain.Capability
	mustData(t, env, &capability)
	if capability.Owner != "risk-office" || capability.RiskLevel != domain.RiskLevelHigh {
		t.Errorf("reviewer edits did not reach the registry: %+v", capability)
	}

	// The candidate is now terminal.
	status, _ = do(t, srv, http.MethodPost, "/api/v1/candidates/"+id+"/register", nil)
	if status != http.StatusConflict {
		t.Errorf("re-registering should conflict, got status %d", status)
	}
}

// TestRejectCandidateOverHTTP covers the reject button, including the empty body case.
func TestRejectCandidateOverHTTP(t *testing.T) {
	srv := newTestServer(t)

	_, env := do(t, srv, http.MethodPost, "/api/v1/import/manual", map[string]any{
		"source_name": "manual-registration",
		"content":     readExample(t, "capabilities", "campaign-message-template.yaml"),
	})
	var result engine.ImportResult
	mustData(t, env, &result)
	id := result.Candidates[0].ID

	status, env := do(t, srv, http.MethodPost, "/api/v1/candidates/"+id+"/reject", nil)
	if status != http.StatusOK {
		t.Fatalf("reject with no body: status %d %+v", status, env.Error)
	}
	var candidate domain.CapabilityCandidate
	mustData(t, env, &candidate)
	if candidate.Status != domain.CandidateStatusRejected {
		t.Errorf("status = %s, want rejected", candidate.Status)
	}
}

// TestUnknownFieldsAreRejected keeps a client typo from being silently discarded.
func TestUnknownFieldsAreRejected(t *testing.T) {
	srv := newTestServer(t)
	status, env := do(t, srv, http.MethodPost, "/api/v1/compositions/recommend", map[string]any{
		"goal": rmGoal, "harnessType": "jiuwen_swarm",
	})
	if status != http.StatusBadRequest || env.Error.Code != api.CodeParse {
		t.Errorf("status = %d, error = %+v; want a parse error naming the unknown field", status, env.Error)
	}
}
