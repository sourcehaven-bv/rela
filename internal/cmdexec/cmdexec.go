// Package cmdexec runs external commands safely for the "bytes in → bytes out"
// pattern shared by attachment processing and view export.
//
// Safety is the whole point of this package, so every invocation:
//
//   - uses array args (never a shell string) → no shell injection;
//   - templates {in}/{out} to runner-owned temp file paths → the caller never
//     builds a path from (attacker-influenced) input;
//   - bounds wall-clock time with a timeout;
//   - bounds the output size with a cap.
//
// It was extracted from internal/attachment's CmdRunner so a second consumer
// (view export) can reuse the reviewed core without importing attachment.
package cmdexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// TemplateIn / TemplateOut are the placeholders substituted with runner-owned
// temp file paths in command argument arrays. A command that references neither
// receives its input on stdin and returns output on stdout.
const (
	TemplateIn  = "{in}"
	TemplateOut = "{out}"

	tempFilePerm = 0o600
)

// Runner drives external commands. A nil *Runner is not valid; use [New].
type Runner struct {
	timeout  time.Duration
	maxBytes int64
	tempDir  string // where {in}/{out} temp files are created ("" → os.TempDir)
}

// Option configures a [Runner].
type Option func(*Runner)

// WithTempDir sets the directory for {in}/{out} temp files.
func WithTempDir(dir string) Option { return func(r *Runner) { r.tempDir = dir } }

// New builds a runner. timeout bounds each command; maxBytes bounds output.
// Both must be positive.
func New(timeout time.Duration, maxBytes int64, opts ...Option) (*Runner, error) {
	if timeout <= 0 {
		return nil, errors.New("cmdexec: timeout must be positive")
	}
	if maxBytes <= 0 {
		return nil, errors.New("cmdexec: maxBytes must be positive")
	}
	r := &Runner{timeout: timeout, maxBytes: maxBytes}
	for _, o := range opts {
		o(r)
	}
	return r, nil
}

// Probe reports whether the command's binary is resolvable on PATH. Callers
// probe at startup for every configured command so a missing tool surfaces as a
// clear warning rather than a per-request failure.
func (r *Runner) Probe(cmd []string) error {
	if len(cmd) == 0 {
		return errors.New("cmdexec: empty command")
	}
	if _, err := exec.LookPath(cmd[0]); err != nil {
		return fmt.Errorf("cmdexec: binary %q not found on PATH: %w", cmd[0], err)
	}
	return nil
}

// Run executes cmd over data with {in}/{out} templated to temp files, a timeout,
// and an output-size cap. When wantOutput is true the rewritten bytes are
// returned — from the {out} file if the command uses it, else from stdout. The
// returned bool reports whether an {out} file was used (so callers can
// distinguish "ran, produced no bytes" from "ran, wrote empty out file").
//
// A non-nil error from the command itself wraps *exec.ExitError, so callers that
// treat a non-zero exit as a domain signal can errors.As it.
func (r *Runner) Run(
	ctx context.Context, cmd []string, data []byte, wantOutput bool,
) (output []byte, usedOutFile bool, err error) {
	if len(cmd) == 0 {
		return nil, false, errors.New("cmdexec: empty command")
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	dir, err := os.MkdirTemp(r.tempDir, "rela-cmdexec-")
	if err != nil {
		return nil, false, fmt.Errorf("cmdexec: create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	args := slices.Clone(cmd)
	usesIn := slices.Contains(args, TemplateIn)
	usesOut := slices.Contains(args, TemplateOut)

	inPath := filepath.Join(dir, "in")
	outPath := filepath.Join(dir, "out")
	if usesIn {
		if werr := os.WriteFile(inPath, data, tempFilePerm); werr != nil {
			return nil, false, fmt.Errorf("cmdexec: write input temp: %w", werr)
		}
	}
	for i, a := range args {
		switch a {
		case TemplateIn:
			args[i] = inPath
		case TemplateOut:
			args[i] = outPath
		}
	}

	// args[0] is caller-configured (project config), not request input; the array
	// form guarantees no shell interpretation of attacker-controlled bytes.
	ec := exec.CommandContext(ctx, args[0], args[1:]...)
	if !usesIn {
		ec.Stdin = bytes.NewReader(data)
	}
	var stdout, stderr bytes.Buffer
	// Cap stdout so a runaway command can't exhaust memory; +1 to detect over.
	ec.Stdout = &cappedWriter{w: &stdout, remaining: r.maxBytes + 1}
	ec.Stderr = &stderr

	runErr := ec.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, usesOut, fmt.Errorf("cmdexec: command timed out after %s", r.timeout)
	}
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, usesOut, fmt.Errorf("%w: %s", runErr, msg)
	}

	if !wantOutput {
		return nil, usesOut, nil
	}

	if usesOut {
		out, rerr := os.ReadFile(outPath) // outPath is runner-owned temp, not caller input
		if rerr != nil {
			return nil, true, fmt.Errorf("cmdexec: read output temp: %w", rerr)
		}
		if int64(len(out)) > r.maxBytes {
			return nil, true, fmt.Errorf("cmdexec: output exceeds cap (%d bytes)", r.maxBytes)
		}
		return out, true, nil
	}
	if int64(stdout.Len()) > r.maxBytes {
		return nil, false, fmt.Errorf("cmdexec: output exceeds cap (%d bytes)", r.maxBytes)
	}
	return stdout.Bytes(), false, nil
}

// cappedWriter fails the write once more than `remaining` bytes are seen,
// turning an oversize command into an error instead of OOM.
type cappedWriter struct {
	w         *bytes.Buffer
	remaining int64
}

func (cw *cappedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > cw.remaining {
		// Write what fits so the cap check downstream still trips, then error.
		cw.w.Write(p[:cw.remaining])
		cw.remaining = 0
		return len(p), errors.New("cmdexec: output size cap exceeded")
	}
	cw.remaining -= int64(len(p))
	return cw.w.Write(p)
}
