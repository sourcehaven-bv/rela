package transform

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
)

// Default execution bounds for a transform command. A transform runs
// synchronously inside a request, so the timeout is modest; slow converters
// (LaTeX/libreoffice) and an async job model are explicit v2 scope.
const (
	defaultTimeout  = 30 * time.Second
	defaultMaxBytes = 64 << 20 // 64 MiB — a generous cap for a single exported document
	// defaultMaxConcurrent bounds simultaneous conversions. Each one is already
	// memory- and CPU-capped, but without an aggregate bound N concurrent export
	// requests multiply that N-fold — the cheapest way to exhaust the server.
	// Exports past the bound queue rather than pile on.
	defaultMaxConcurrent = 4
)

// Engine runs a registered transform over a [Renderer]'s markdown. Construct it
// with [NewEngine]; the zero value is not usable.
type Engine struct {
	registry Registry
	runner   *cmdexec.Runner
}

// EngineOption configures an [Engine].
type EngineOption func(*engineConfig)

type engineConfig struct {
	timeout          time.Duration
	maxBytes         int64
	tempDir          string
	sandboxOptOut    bool
	sandboxOptOutSet bool
}

// WithTimeout overrides the per-command timeout.
func WithTimeout(d time.Duration) EngineOption {
	return func(c *engineConfig) { c.timeout = d }
}

// WithMaxBytes overrides the output-size cap.
func WithMaxBytes(n int64) EngineOption {
	return func(c *engineConfig) { c.maxBytes = n }
}

// WithTempDir sets the directory for the runner's {in}/{out} temp files.
func WithTempDir(dir string) EngineOption {
	return func(c *engineConfig) { c.tempDir = dir }
}

// WithSandboxDisabled runs converters UNCONFINED — the operator's explicit
// acceptance of the risk, for hosts where no sandbox mechanism is available
// (Windows/BSD, a kernel without unprivileged user namespaces, a container that
// blocks them) or where isolation is provided at another layer. Without it, a
// host that cannot sandbox refuses to run any transform. The composition root
// must surface this in a startup warning.
func WithSandboxDisabled(disabled bool) EngineOption {
	return func(c *engineConfig) { c.sandboxOptOut, c.sandboxOptOutSet = disabled, true }
}

// NewEngine builds an engine over the given registry. It returns an error if the
// runner cannot be constructed (non-positive bounds).
func NewEngine(registry Registry, opts ...EngineOption) (*Engine, error) {
	if registry == nil {
		return nil, errors.New("transform: nil registry")
	}
	cfg := engineConfig{timeout: defaultTimeout, maxBytes: defaultMaxBytes}
	for _, o := range opts {
		o(&cfg)
	}
	runOpts := []cmdexec.Option{cmdexec.WithMaxConcurrent(defaultMaxConcurrent)}
	if cfg.tempDir != "" {
		runOpts = append(runOpts, cmdexec.WithTempDir(cfg.tempDir))
	}
	if cfg.sandboxOptOut {
		runOpts = append(runOpts, cmdexec.WithSandboxDisabled())
	}
	runner, err := cmdexec.New(cfg.timeout, cfg.maxBytes, runOpts...)
	if err != nil {
		return nil, fmt.Errorf("transform: %w", err)
	}
	return &Engine{registry: registry, runner: runner}, nil
}

// Result is the output of a successful transform.
type Result struct {
	Data     []byte // the converted bytes
	Produces string // the transform's content-type
}

// Run renders markdown via r, then converts it with the named transform.
//
// Errors:
//   - [UnknownTransformError] if name is not registered (map to 4xx — caller input).
//   - a wrapped render error if the [Renderer] fails.
//   - a wrapped exec error (missing binary, non-zero exit, timeout, over-cap) if
//     the transform command fails.
func (e *Engine) Run(ctx context.Context, name string, r Renderer) (Result, error) {
	def, ok := e.registry[name]
	if !ok {
		return Result{}, UnknownTransformError{Name: name}
	}
	if def.From != FormatMarkdown {
		// Defensive: the metamodel validator rejects non-markdown From at load,
		// so this should be unreachable, but a stale/hand-built registry must not
		// silently feed markdown to a non-markdown command.
		return Result{}, fmt.Errorf("transform: %q expects input format %q, only %q is supported",
			name, def.From, FormatMarkdown)
	}

	md, err := r.Render(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("transform: render for %q: %w", name, err)
	}

	out, _, err := e.runner.Run(ctx, def.Command, md, true)
	if err != nil {
		return Result{}, fmt.Errorf("transform %q: %w", name, err)
	}
	return Result{Data: out, Produces: def.Produces}, nil
}

// Probe reports, per registered transform, whether its command binary resolves
// on PATH. The composition root calls this at startup so a missing converter
// surfaces as a warning rather than a per-export failure. The returned map is
// keyed by transform name; a nil value means the binary was found.
func (e *Engine) Probe() map[string]error {
	out := make(map[string]error, len(e.registry))
	for name, def := range e.registry {
		out[name] = e.runner.Probe(def.Command)
	}
	return out
}
