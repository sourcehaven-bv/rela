package attachment

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
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
	extraReadOnly []string
}

// CmdRunnerOption configures a [CmdRunner].
type CmdRunnerOption func(*cmdRunnerConfig)

// WithScannerSockets binds extra host paths (a clamd socket in a non-standard
// location) read-only into the scan command's sandbox, on top of the well-known
// defaults ([cmdexec.DefaultScannerSockets]). A unix socket bound this way is
// reachable without opening network egress.
func WithScannerSockets(paths ...string) CmdRunnerOption {
	return func(c *cmdRunnerConfig) { c.extraReadOnly = append(c.extraReadOnly, paths...) }
}

// NewCmdRunner builds a runner. timeout bounds each command; maxBytes bounds
// transform output. Both must be positive.
func NewCmdRunner(timeout time.Duration, maxBytes int64, opts ...CmdRunnerOption) (*CmdRunner, error) {
	var cfg cmdRunnerConfig
	for _, o := range opts {
		o(&cfg)
	}
	// A scan command needs two things from outside the sandbox's mount view: the
	// daemon's unix socket, and the config file naming that socket (clamdscan
	// parses clamd.conf before it connects, so the socket alone is not enough).
	// Bind the well-known locations of both, plus any operator override, so a
	// stock install works without configuration and without network access.
	binds := append([]string{}, cmdexec.DefaultScannerSockets...)
	binds = append(binds, cmdexec.DefaultScannerConfigs...)
	binds = append(binds, cfg.extraReadOnly...)
	r, err := cmdexec.New(timeout, maxBytes, cmdexec.WithExtraReadOnly(binds...))
	if err != nil {
		return nil, fmt.Errorf("attachment: %w", err)
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

// SandboxErr reports why commands run by this runner will FAIL, or nil when they
// will run.
//
// Nil: does NOT mean "confined" — see [cmdexec.Runner.SandboxErr], which this
// delegates to. The composition root uses it to warn at startup that configured
// scans will all fail closed. Never skip a scan because it is non-nil: that
// turns a fail-closed rejection into an unscanned upload.
func (c *CmdRunner) SandboxErr() error { return c.exec.SandboxErr() }

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
