// Package ai provides LLM access for rela via OpenAI-compatible providers.
//
// The package exposes a Provider interface, an OpenAI-compatible HTTP
// implementation, a typed error taxonomy, and a config loader for
// .rela/ai.yaml. It is designed to be the foundation for AI-powered
// features throughout rela: Lua bindings, validations, CLI commands,
// MCP tools, and the data entry UI.
//
// The Provider interface intentionally aggregates capabilities (Chat
// today, Embed in a future ticket) so that consumers — particularly the
// Lua runtime — only have one wiring point.
package ai

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the AI config file inside .rela/.
const ConfigFile = "ai.yaml"

// DefaultTimeoutSeconds is the default request timeout when none is set.
const DefaultTimeoutSeconds = 30

// ErrConfigNotFound indicates that .rela/ai.yaml does not exist. This
// is the "AI not configured" state and is returned by LoadConfig so
// callers can use errors.Is(err, ErrConfigNotFound) to handle it as a
// normal absence rather than a failure.
var ErrConfigNotFound = errors.New("ai: not configured (no .rela/ai.yaml)")

// Config is the contents of .rela/ai.yaml.
//
// APIKeyEnv is OPTIONAL. When empty, the provider sends no Authorization
// header at all (supports auth-free local providers like ollama, apfel,
// LM Studio). When non-empty, the named environment variable must be set
// to a non-empty value at Chat() call time.
type Config struct {
	Provider       string `yaml:"provider"`
	BaseURL        string `yaml:"base_url"`
	Model          string `yaml:"model"`
	APIKeyEnv      string `yaml:"api_key_env"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// LoadConfig reads .rela/ai.yaml from the given .rela directory.
//
// Returns ErrConfigNotFound (wrapped) if the file does not exist — this
// is the "AI not configured" state. Callers can use
// errors.Is(err, ErrConfigNotFound) to distinguish it from real errors.
//
// Returns other errors on read failure, parse failure, or invalid
// required fields.
func LoadConfig(relaDir string) (*Config, error) {
	path := filepath.Join(relaDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks required fields and rejects unsafe values.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("model is required")
	}

	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("base_url is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base_url must use http:// or https://, got %q", c.BaseURL)
	}
	if u.User != nil {
		return errors.New("base_url must not contain credentials (use api_key_env instead)")
	}
	if u.RawQuery != "" {
		// Some legacy providers (notably old Azure) carry the API key
		// in a query parameter like ?api_key=... or ?token=.... If we
		// allowed that the key would be embedded in every log line via
		// logRequestStart and never redacted (the redactKey helper only
		// knows about env-var-sourced keys). Force users to authenticate
		// via api_key_env so the key never lands in the URL.
		return errors.New("base_url must not contain a query string (use api_key_env for authentication)")
	}
	if u.Fragment != "" {
		return errors.New("base_url must not contain a fragment")
	}

	if c.Provider != "" && c.Provider != "openai-compatible" {
		return fmt.Errorf("provider %q is not supported (only openai-compatible)", c.Provider)
	}

	if c.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout_seconds must be >= 0, got %d", c.TimeoutSeconds)
	}

	return nil
}

// Timeout returns the request timeout in seconds, applying the default
// when TimeoutSeconds is zero.
func (c *Config) Timeout() int {
	if c.TimeoutSeconds <= 0 {
		return DefaultTimeoutSeconds
	}
	return c.TimeoutSeconds
}
