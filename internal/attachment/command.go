package attachment

import (
	"context"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// CommandRunner drives an external command (scan or transform) over an
// attachment's bytes. It is a consumer-side seam: the data-entry/CLI wiring
// supplies a concrete runner (the cmd: harness, Phase 2); when nil the
// [PolicyProcessor] performs native MIME validation only.
//
// Implementations MUST invoke commands safely: array args (never a shell
// string), templated {in}/{out} paths owned by the runner, a timeout, and an
// output-size cap. See the attachment-security guide.
type CommandRunner interface {
	// Scan runs the field's virus-scan command over data. A nil error means
	// clean; an error wrapping [ErrRejected] means a positive/blocked result or
	// (fail-closed) that the scan could not run for a `required` field.
	Scan(ctx context.Context, cmd []string, data []byte) error

	// Transform runs a transform command and returns the rewritten bytes and an
	// optional new file name.
	Transform(ctx context.Context, cmd []string, in ProcessContext, data []byte) ([]byte, string, error)
}

// applyCommands runs the scan (when policy requires it) then the configured
// transforms for one property. Phase 2 supplies the runner; with a nil runner
// this is never called (see PolicyProcessor.Process).
func (p *PolicyProcessor) applyCommands(
	ctx context.Context, pc ProcessContext, prop metamodel.PropertyDef, r io.Reader, info ProcessInfo,
) (io.Reader, ProcessInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, ProcessInfo{}, err
	}

	// Scan first — a reject here stops the write before any transform cost.
	if policy := p.meta.EffectiveScanPolicy(prop); policy == metamodel.ScanRequired {
		if scanCmd := p.scanCommand(prop); len(scanCmd) > 0 {
			if err := p.runner.Scan(ctx, scanCmd, data); err != nil {
				return nil, ProcessInfo{}, err
			}
		} else {
			// `scan: required` but no command configured — fail closed.
			return nil, ProcessInfo{}, Rejectedf(
				"scan required for property %q but no scan command is configured", pc.Property)
		}
	}

	// Transforms in declared order; each rewrites the bytes.
	for _, cmd := range p.transformCommands(prop) {
		out, newName, terr := p.runner.Transform(ctx, cmd, pc, data)
		if terr != nil {
			return nil, ProcessInfo{}, terr
		}
		data = out
		if newName != "" {
			info.FileName = newName
		}
	}

	return readerFor(data), info, nil
}
