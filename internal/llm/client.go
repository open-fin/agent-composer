// Package llm provides optional capability enrichment. Everything here has a
// deterministic mock implementation, so the system runs identically with no API key.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Request is one chat completion.
type Request struct {
	System      string
	User        string
	Temperature float64
	MaxTokens   int
	// JSONObject asks the endpoint for a strict JSON object response.
	JSONObject bool
}

// Client is the minimal LLM surface the enricher needs.
type Client interface {
	Name() string
	Complete(ctx context.Context, req Request) (string, error)
}

// OpenAICompatible talks to any endpoint exposing /chat/completions, which covers
// OpenAI, vLLM, Ollama's compatibility layer, and most enterprise gateways.
type OpenAICompatible struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// NewOpenAICompatible builds a client with a bounded timeout. A hung endpoint must
// never stall an import or a recommendation.
func NewOpenAICompatible(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatible {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &OpenAICompatible{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func (c *OpenAICompatible) Name() string { return "openai-compatible:" + c.Model }

func (c *OpenAICompatible) Complete(ctx context.Context, req Request) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := map[string]any{
		"model": c.Model,
		"messages": []message{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.JSONObject {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call llm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm returned %s", resp.Status)
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// extractJSONObject pulls the first balanced JSON object out of a model response,
// tolerating the markdown fences models habitually add.
func extractJSONObject(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if fence := strings.Index(trimmed, "```"); fence >= 0 {
		rest := trimmed[fence+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		trimmed = strings.TrimSpace(rest)
	}
	start := strings.IndexByte(trimmed, '{')
	if start < 0 {
		return "", fmt.Errorf("no json object in response")
	}
	depth := 0
	for i := start; i < len(trimmed); i++ {
		switch trimmed[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return trimmed[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated json object in response")
}
