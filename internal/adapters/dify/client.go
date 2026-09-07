package dify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/open-fin/agent-composer/internal/adapters"
	"github.com/open-fin/agent-composer/internal/domain"
)

// Client is the placeholder adapter for pulling apps directly from a running Dify
// instance rather than from an uploaded file.
//
// Phase 1 ships the HTTP surface and the wiring but not a full Dify API integration:
// Dify's app-export endpoints differ across versions and self-hosted deployments, and
// guessing at them would produce an adapter that fails in the field. FetchDSL is the
// single seam a deployment needs to fill in; everything downstream of it — parsing,
// mapping, review, registration — is already exercised by the file upload path.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient builds a Dify API client with a bounded timeout.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return "dify-api" }

// ErrNotImplemented marks the parts of the Dify API adapter that a deployment must
// supply. It is returned rather than a fake success so nobody mistakes a stub for a
// working integration.
var ErrNotImplemented = fmt.Errorf("dify api import is not implemented in phase 1; export the app DSL and use POST /api/v1/import/dify/dsl")

// Ping checks that the configured base URL is reachable. This part is real, so a
// misconfigured source is caught when it is created rather than at import time.
func (c *Client) Ping(ctx context.Context) error {
	if c.BaseURL == "" {
		return fmt.Errorf("dify base_url is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("reach dify at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("dify at %s returned %s", c.BaseURL, resp.Status)
	}
	return nil
}

// FetchDSL would retrieve an application's DSL export by id.
func (c *Client) FetchDSL(_ context.Context, appID string) ([]byte, error) {
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	return nil, ErrNotImplemented
}

// Import satisfies adapters.Importer. When a caller supplies a payload it behaves
// exactly like the DSL upload path; otherwise it reports that the remote fetch is not
// available yet.
func (c *Client) Import(ctx context.Context, in adapters.ImportInput) ([]domain.CapabilityCandidate, error) {
	if len(in.Payload) > 0 {
		return NewDSLImporter().Import(ctx, in)
	}
	appID, _ := in.Options["app_id"].(string)
	payload, err := c.FetchDSL(ctx, appID)
	if err != nil {
		return nil, err
	}
	in.Payload = payload
	return NewDSLImporter().Import(ctx, in)
}

// DescribeConfig renders the client configuration for storage on an external source,
// with the API key deliberately left out; secrets are held on the source record and
// redacted on read.
func (c *Client) DescribeConfig() map[string]any {
	return map[string]any{"base_url": c.BaseURL, "adapter": c.Name()}
}

// MarshalJSON keeps the API key out of any accidental serialization of the client.
func (c *Client) MarshalJSON() ([]byte, error) { return json.Marshal(c.DescribeConfig()) }

var _ adapters.Importer = (*Client)(nil)
