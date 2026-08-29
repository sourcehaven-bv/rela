package lua

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// MailSenderLoader builds the mail transport for a project's .rela directory.
//
// A function supplied BY THE CALLER rather than a direct internal/mail import,
// because internal/mail depends on this package (transport: script runs a Lua
// runtime) and importing it back would be a cycle. The wiring site — which
// already knows about both — provides the one-line adapter.
//
// It returns (nil, nil) when the project has no mail configured. That is not
// an error: mail is off in the overwhelmingly common case, every other Lua
// binding must still work, and mail.send reports the absence as a typed
// not_configured error rather than by being missing.
type MailSenderLoader func(cacheDir string) (MailSender, error)

// LoadContextOptions loads AI provider, secrets and the mail transport from
// the .rela directory and returns them as runtime options. This is the single
// entry point for all Lua callers (CLI, MCP, automation, actions) to load
// project-level context into a runtime.
//
// scriptPath is the script being executed (used to resolve per-script
// secrets). Pass "" for inline code (skips secrets loading).
//
// mailLoader may be nil, in which case no mail transport is wired and
// mail.send returns not_configured. It is a PARAMETER rather than a second
// exported LoadXxx function on purpose: the ticket that added mail.send
// (TKT-DS1CR6) inherited the rule internal/ai already states — one load point,
// no parallel call sites — and adding a `mail.LoadSender` that callers had to
// remember to also call is exactly how the AI provider ended up needing that
// rule written down.
//
// Returns ai.ErrConfigNotFound (via errors.Is) when AI is not configured —
// callers that want to silently ignore missing AI can check for it.
func LoadContextOptions(cacheDir, scriptPath string, mailLoader MailSenderLoader) ([]Option, error) {
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

	if mailLoader != nil {
		sender, mailErr := mailLoader(cacheDir)
		if mailErr != nil {
			// A broken mail.yaml is reported, not swallowed. "Configured but
			// invalid" and "not configured" look identical to a script
			// otherwise, and the operator debugging a typo gets no signal at
			// all. The absent case is (nil, nil) — see MailSenderLoader.
			return nil, fmt.Errorf("mail: %w", mailErr)
		}
		if sender != nil {
			opts = append(opts, WithMailSender(sender))
		}
	}

	return opts, nil
}
