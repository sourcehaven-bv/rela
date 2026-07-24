package transform

import (
	"context"
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

// Engine converts a [Renderer]'s markdown via registered transforms. It owns
// the bounded worker pool that caps concurrent converter processes, so it MUST
// be built once and shared — a per-request engine would give every request its
// own pool and the concurrency cap would bound nothing.
//
// The engine holds no registry: [Engine.Run] takes one per call, so a config
// live-reload needs no engine rebuild (and cannot discard in-flight pool
// slots). Construct with [NewEngine]; the zero value is not usable.
type Engine struct {
	runner *cmdexec.Runner
}

// NewEngine builds an engine with the package's execution bounds.
func NewEngine() *Engine {
	runner, err := cmdexec.New(defaultTimeout, defaultMaxBytes,
		cmdexec.WithMaxConcurrent(defaultMaxConcurrent))
	if err != nil {
		// Unreachable: cmdexec.New fails only on non-positive bounds, and the
		// bounds are package constants.
		panic("transform: " + err.Error())
	}
	return &Engine{runner: runner}
}

// Result is the output of a successful transform.
type Result struct {
	Data     []byte // the converted bytes
	Produces string // the transform's content-type
}

// Run renders markdown via r, then converts it with the named transform from
// reg.
//
// Errors:
//   - [UnknownTransformError] if name is not registered (map to 4xx — caller input).
//   - a wrapped render error if the [Renderer] fails.
//   - a wrapped exec error (missing binary, non-zero exit, timeout, over-cap) if
//     the transform command fails.
func (e *Engine) Run(ctx context.Context, reg Registry, name string, r Renderer) (Result, error) {
	def, ok := reg[name]
	if !ok {
		return Result{}, UnknownTransformError{Name: name}
	}
	if def.From != FormatMarkdown {
		// Defensive: the metamodel validator canonicalizes and rejects non-markdown
		// From at load, so this should be unreachable, but a stale/hand-built
		// registry must not silently feed markdown to a non-markdown command.
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

// Probe reports, per transform in reg, whether its command binary resolves on
// PATH. The composition root calls this at startup so a missing converter
// surfaces as a warning rather than a per-export failure. The returned map is
// keyed by transform name; a nil value means the binary was found.
func (e *Engine) Probe(reg Registry) map[string]error {
	out := make(map[string]error, len(reg))
	for name, def := range reg {
		out[name] = e.runner.Probe(def.Command)
	}
	return out
}
