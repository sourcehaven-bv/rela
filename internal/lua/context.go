package lua

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// HostConfig is what a runtime reads from the machine rather than from the
// project's config: secrets, and the AI and mail settings.
// hostconfig.Dir implements it over a .rela directory; the desktop supplies
// one backed by the OS keychain and the project's database.
type HostConfig interface {
	// File returns ai.yaml or mail.yaml; an error wrapping fs.ErrNotExist
	// when the project has none.
	File(name string) ([]byte, error)
	// Secrets returns the secrets visible to scriptPath ("" for the global
	// ones); secrets.ErrNotFound when the project has none.
	Secrets(scriptPath string) (map[string]string, error)
	// Path is the .rela directory, or "" when there is none on disk.
	Path() string
}

// MailSenderLoader builds the mail transport from a project's host config.
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
type MailSenderLoader func(host HostConfig) (MailSender, error)

// ErrHostConfigWithheld is what a [HostConfig] returns for ai.yaml or the
// secrets when the host has them but does not share them with this
// project. Tenant mode withholds the operator's own credentials from every
// tenant unless the operator opts in. Unlike an absent file, the runtime
// still starts: ai.* returns a typed not_configured error and an index of
// rela.secrets raises, both with the wrapping error's text, so the script
// author learns why.
var ErrHostConfigWithheld = errors.New("not shared with this project")

// LoadContextOptions loads the AI provider, secrets and the mail transport
// from host and returns them as runtime options. This is the single entry
// point for all Lua callers (CLI, MCP, automation, actions) to load
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
//
// Nil: host is rejected; a caller with no config passes hostconfig.Dir("").
func LoadContextOptions(host HostConfig, scriptPath string, mailLoader MailSenderLoader) ([]Option, error) {
	if host == nil {
		return nil, errors.New("lua: LoadContextOptions requires a host config")
	}
	var opts []Option

	provider, err := loadAIProvider(host)
	switch {
	case errors.Is(err, ai.ErrConfigNotFound):
		// no AI configured
	case errors.Is(err, ErrHostConfigWithheld):
		opts = append(opts, WithAIProvider(withheldAI{reason: err.Error()}))
	case err != nil:
		return nil, fmt.Errorf("ai: %w", err)
	default:
		opts = append(opts, WithAIProvider(provider))
	}

	if scriptPath != "" {
		sec, secErr := host.Secrets(scriptPath)
		switch {
		case errors.Is(secErr, secrets.ErrNotFound):
			// no secrets configured
		case errors.Is(secErr, ErrHostConfigWithheld):
			opts = append(opts, withSecretsWithheld(secErr.Error()))
		case secErr != nil:
			return nil, fmt.Errorf("secrets: %w", secErr)
		default:
			if len(sec) > 0 {
				opts = append(opts, WithSecrets(sec))
			}
		}
	}

	if mailLoader != nil {
		sender, mailErr := mailLoader(host)
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

// AIProvider is the provider ai.yaml configures on host, for the one caller
// that needs it outside a runtime: the declarative llm action
// (script.AIProvider). ai.ErrConfigNotFound when there is no ai.yaml.
func AIProvider(host HostConfig) (ai.Provider, error) { return loadAIProvider(host) }

// loadAIProvider builds the provider ai.yaml configures, reading its API key
// from the global secrets first. ai.ErrConfigNotFound when there is no
// ai.yaml.
func loadAIProvider(host HostConfig) (ai.Provider, error) {
	data, err := host.File(ai.ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ai.ErrConfigNotFound
	}
	if errors.Is(err, ErrHostConfigWithheld) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ai.ConfigFile, err)
	}
	cfg, err := ai.ParseConfig(data, ai.ConfigFile)
	if err != nil {
		return nil, err
	}
	cfg.WithSecrets(func() (map[string]string, error) { return host.Secrets("") })
	return ai.NewOpenAICompatProvider(cfg)
}

// withheldAI is the provider of a runtime whose host withholds ai.yaml.
// Every call fails with a not_configured [ai.Error] that says why.
type withheldAI struct{ reason string }

// Chat refuses the call.
func (p withheldAI) Chat(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return nil, &ai.Error{Kind: ai.ErrNotConfigured, Message: "AI is not available: " + p.reason}
}

// Embed refuses the call.
func (p withheldAI) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, &ai.Error{Kind: ai.ErrNotConfigured, Message: "AI is not available: " + p.reason}
}
