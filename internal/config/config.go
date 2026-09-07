// Package config loads server configuration from a YAML file with COMPOSER_* env
// overrides. Every field has a working default so the demo runs with no config at all.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the full server configuration.
type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	LLM      LLMConfig      `yaml:"llm"`
	Examples ExamplesConfig `yaml:"examples"`
}

// HTTPConfig controls the API listener.
type HTTPConfig struct {
	// Addr defaults to :8088 rather than :8080, which is commonly already in use.
	Addr           string        `yaml:"addr"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	MaxUploadBytes int64         `yaml:"max_upload_bytes"`
	CORSOrigin     string        `yaml:"cors_origin"`
}

// DatabaseConfig selects the store. An empty DSN runs against the in-memory store,
// which is what unit tests and `agentctl` use.
type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxConns        int32         `yaml:"max_conns"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	MigrateOnStart  bool          `yaml:"migrate_on_start"`
	StartupAttempts int           `yaml:"startup_attempts"`
}

// LLMConfig configures optional capability enrichment. Disabled by default; when
// enabled but unreachable the client falls back to the deterministic mock.
type LLMConfig struct {
	Enabled bool          `yaml:"enabled"`
	BaseURL string        `yaml:"base_url"`
	APIKey  string        `yaml:"api_key"`
	Model   string        `yaml:"model"`
	Timeout time.Duration `yaml:"timeout"`
}

// ExamplesConfig points at the bundled demo assets used by `agentctl seed`.
type ExamplesConfig struct {
	Dir string `yaml:"dir"`
}

// Default returns the configuration used when no file and no env vars are present.
func Default() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:           ":8088",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   60 * time.Second,
			MaxUploadBytes: 8 << 20,
			CORSOrigin:     "*",
		},
		Database: DatabaseConfig{
			MaxConns:        8,
			ConnectTimeout:  10 * time.Second,
			MigrateOnStart:  true,
			StartupAttempts: 30,
		},
		LLM: LLMConfig{
			Enabled: false,
			Model:   "gpt-4o-mini",
			Timeout: 20 * time.Second,
		},
		Examples: ExamplesConfig{Dir: "examples"},
	}
}

// Load reads path if it exists, then applies COMPOSER_* env overrides. A missing file is
// not an error: the defaults plus environment are a complete configuration.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		raw, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config %s: %w", path, err)
			}
		case !os.IsNotExist(err):
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	envString("COMPOSER_HTTP_ADDR", &cfg.HTTP.Addr)
	envString("COMPOSER_CORS_ORIGIN", &cfg.HTTP.CORSOrigin)
	envString("COMPOSER_DB_DSN", &cfg.Database.DSN)
	envBool("COMPOSER_DB_MIGRATE_ON_START", &cfg.Database.MigrateOnStart)
	envBool("COMPOSER_LLM_ENABLED", &cfg.LLM.Enabled)
	envString("COMPOSER_LLM_BASE_URL", &cfg.LLM.BaseURL)
	envString("COMPOSER_LLM_API_KEY", &cfg.LLM.APIKey)
	envString("COMPOSER_LLM_MODEL", &cfg.LLM.Model)
	envDuration("COMPOSER_LLM_TIMEOUT", &cfg.LLM.Timeout)
	envString("COMPOSER_EXAMPLES_DIR", &cfg.Examples.Dir)
}

func envString(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}

func envBool(key string, dst *bool) {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			*dst = parsed
		}
	}
}

func envDuration(key string, dst *time.Duration) {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			*dst = parsed
		}
	}
}

// Validate rejects configurations that would fail confusingly at runtime.
func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http.addr must not be empty")
	}
	if c.LLM.Enabled && c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.enabled requires llm.base_url (or unset COMPOSER_LLM_ENABLED to use the mock enricher)")
	}
	return nil
}

// UsesPostgres reports whether a real database is configured.
func (c Config) UsesPostgres() bool { return strings.TrimSpace(c.Database.DSN) != "" }
