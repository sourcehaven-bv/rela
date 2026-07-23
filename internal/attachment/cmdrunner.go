package attachment

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
)

// templateIn / templateOut mirror the cmdexec placeholders; kept as package
// aliases so attachment tests reference them without importing cmdexec.
const (
	templateIn  = cmdexec.TemplateIn
	templateOut = cmdexec.TemplateOut
)

// CmdRunner is the concrete [CommandRunner]: it drives external scan/transform
// binaries safely. The safe-exec core (array args, {in}/{out} templating,
// timeout, output cap) lives in [cmdexec]; CmdRunner adds the attachment-specific
// scan/transform policy semantics (fail-closed scan, [ErrRejected] mapping).
//
// A nil *CmdRunner is not valid; use [NewCmdRunner].
type CmdRunner struct {
	exec *cmdexec.Runner
}

// cmdRunnerConfig accumulates option settings before the cmdexec runner is built.
type cmdRunnerConfig struct {
	tempDir string
}

// CmdRunnerOption configures a [CmdRunner].
type CmdRunnerOption func(*cmdRunnerConfig)

// WithTempDir sets the directory for {in}/{out} temp files.
func WithTempDir(dir string) CmdRunnerOption {
	return func(c *cmdRunnerConfig) { c.tempDir = dir }
}

// NewCmdRunner builds a runner. timeout bounds each command; maxBytes bounds
// transform output. Both must be positive.
func NewCmdRunner(timeout time.Duration, maxBytes int64, opts ...CmdRunnerOption) (*CmdRunner, error) {
	var cfg cmdRunnerConfig
	for _, o := range opts {
		o(&cfg)
	}
	var execOpts []cmdexec.Option
	if cfg.tempDir != "" {
		execOpts = append(execOpts, cmdexec.WithTempDir(cfg.tempDir))
	}
	r, err := cmdexec.New(timeout, maxBytes, execOpts...)
	if err != nil {
		// Preserve the historical "attachment: ..." error namespace.
		return nil, errors.New("attachment: " + strings.TrimPrefix(err.Error(), "cmdexec: "))
	}
	return &CmdRunner{exec: r}, nil
}

// Probe reports whether the command's binary is resolvable on PATH. The
// composition root calls this at startup for every configured command so a
// missing tool surfaces as a warning rather than a per-upload failure.
func (c *CmdRunner) Probe(cmd []string) error { return c.exec.Probe(cmd) }

// Describe returns a one-line summary of how scan/transform commands are
// confined, for the startup log. Diagnostic only — never branch on this string;
// callers just run the command and handle the error.
func (c *CmdRunner) Describe() string { return c.exec.Describe() }

// Scan runs cmd over data as a virus/policy scan. A nil error means clean; a
// non-zero exit is mapped to a rejection wrapping [ErrRejected]. The bytes are
// offered via the {in} temp file when the command references it, else on stdin.
func (c *CmdRunner) Scan(ctx context.Context, cmd []string, data []byte) error {
	if len(cmd) == 0 {
		return Rejectedf("scan command is empty")
	}
	_, _, err := c.exec.Run(ctx, cmd, data, false)
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// A non-zero exit is the scanner's "not clean" signal.
		return Rejectedf("scan failed: %s", strings.TrimSpace(err.Error()))
	}
	// Couldn't even run the scanner — fail closed for a required scan.
	return Rejectedf("scan could not run: %v", err)
}

// Transform runs cmd over data and returns the rewritten bytes. The new file
// name is empty unless the command implies an extension change (not inferred
// here; transforms keep the name unless a future option sets it).
func (c *CmdRunner) Transform(
	ctx context.Context, cmd []string, _ ProcessContext, data []byte,
) (out []byte, newName string, err error) {
	if len(cmd) == 0 {
		return nil, "", errors.New("transform command is empty")
	}
	out, usedOutFile, err := c.exec.Run(ctx, cmd, data, true)
	if err != nil {
		return nil, "", err
	}
	if len(out) == 0 && !usedOutFile {
		return nil, "", errors.New("transform produced no output")
	}
	// newName stays empty: transforms keep the file name (a future option may
	// set it for extension-changing converts).
	return out, "", nil
}
