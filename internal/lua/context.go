package lua

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// LoadContextOptions loads AI provider and secrets from the .rela directory
// and returns them as runtime options. This is the single entry point for
// all Lua callers (CLI, MCP, automation, actions) to load project-level
// context into a runtime.
//
// scriptPath is the script being executed (used to resolve per-script
// secrets). Pass "" for inline code (skips secrets loading).
//
// Returns ai.ErrConfigNotFound (via errors.Is) when AI is not configured —
// callers that want to silently ignore missing AI can check for it.
func LoadContextOptions(cacheDir, scriptPath string) ([]Option, error) {
	var opts []Option

	provider, err := ai.LoadProvider(cacheDir)
	switch {
	case errors.Is(err, ai.ErrConfigNotFound):
		// no AI configured
	case err != nil:
		return nil, fmt.Errorf("ai: %w", err)
	default:
		opts = append(opts, WithAIProvider(provider))
	}

	if scriptPath != "" {
		sec, secErr := secrets.Load(cacheDir, scriptPath)
		switch {
		case errors.Is(secErr, secrets.ErrNotFound):
			// no secrets configured
		case secErr != nil:
			return nil, fmt.Errorf("secrets: %w", secErr)
		default:
			if len(sec) > 0 {
				opts = append(opts, WithSecrets(sec))
			}
		}
	}

	return opts, nil
}
