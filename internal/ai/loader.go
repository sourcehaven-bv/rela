package ai

// LoadProvider is a convenience for entry points: load .rela/ai.yaml
// from the given directory and build a Provider if it exists.
//
// Returns:
//   - (provider, nil) when the config loaded successfully
//   - (nil, ErrConfigNotFound) when no config exists (AI is "not
//     configured" — this is a normal state, not an error). Callers
//     check via errors.Is(err, ErrConfigNotFound).
//   - (nil, err) when the config exists but cannot be loaded or
//     validated, or when provider construction fails
//
// Callers decide policy: interactive entry points like `rela script`
// and `rela flow` should surface non-ErrConfigNotFound errors to the
// user (AI may be the whole point of the command). Background contexts
// (automation script executor, MCP tool handlers) typically log + ignore
// so a misconfigured ai.yaml doesn't break the host process.
func LoadProvider(relaDir string) (Provider, error) {
	cfg, err := LoadConfig(relaDir)
	if err != nil {
		// LoadConfig already returns ErrConfigNotFound (wrapped) for
		// the missing-file case, so we just propagate.
		return nil, err
	}
	return NewOpenAICompatProvider(cfg)
}
