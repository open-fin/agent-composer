package domain

import "time"

// ExternalSource is a configured system that capabilities can be imported from.
type ExternalSource struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      SourceType     `json:"type"`
	BaseURL   string         `json:"base_url,omitempty"`
	AuthType  string         `json:"auth_type,omitempty"`
	Config    map[string]any `json:"config"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// secretConfigKeys are redacted on every read path. Phase 1 has no auth and no secret
// store, so credentials must never travel back out of the API.
var secretConfigKeys = []string{"api_key", "apikey", "token", "secret", "password", "authorization"}

// Redacted returns a copy safe to serialize to a client.
func (s ExternalSource) Redacted() ExternalSource {
	if s.Config == nil {
		return s
	}
	clone := make(map[string]any, len(s.Config))
	for k, v := range s.Config {
		clone[k] = v
	}
	for _, key := range secretConfigKeys {
		if raw, ok := clone[key]; ok {
			if str, isStr := raw.(string); !isStr || str != "" {
				clone[key] = "***"
			}
		}
	}
	s.Config = clone
	return s
}
