package ai

import (
	"errors"
	"log/slog"
)

// LoadProvider is a convenience for entry points: load .rela/ai.yaml
// from the given directory and build a Provider if it exists. Returns
// nil when no AI config is present, so callers can pass the returned
// Provider directly to lua.WithAIProvider — the Lua bindings will
// surface a typed not_configured error at call time.
//
// On config-load errors (malformed YAML, invalid fields), this logs a
// warning and returns nil. The rationale: a misconfigured ai.yaml
// should not prevent unrelated rela commands from running. The user
// will see the warning the next time they invoke an AI script and the
// not_configured error message will direct them to fix the config.
func LoadProvider(relaDir string) Provider {
	cfg, err := LoadConfig(relaDir)
	if err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			slog.Warn("ai: failed to load config", "rela_dir", relaDir, "error", err)
		}
		return nil
	}
	provider, err := NewOpenAICompatProvider(cfg)
	if err != nil {
		slog.Warn("ai: failed to build provider", "error", err)
		return nil
	}
	return provider
}
