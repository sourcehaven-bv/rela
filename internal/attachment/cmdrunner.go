package attachment

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

// CmdRunner is the concrete [CommandRunner]: it drives external scan/transform
// binaries safely. Safety is the whole point of this type, so every invocation:
//
//   - uses array args (never a shell string) → no shell injection;
//   - templates {in}/{out} to runner-owned temp file paths → the operator never
//     builds a path from the (attacker-influenced) file name;
//   - bounds wall-clock time with a timeout;
//   - bounds the output size with a cap.
//
// A nil *CmdRunner is not valid; use [NewCmdRunner].
type CmdRunner struct {
	timeout  time.Duration
	maxBytes int64
	tempDir  string // where {in}/{out} temp files are created ("" → os.TempDir)
}

// CmdRunnerOption configures a [CmdRunner].
type CmdRunnerOption func(*CmdRunner)

// WithTempDir sets the directory for {in}/{out} temp files.
func WithTempDir(dir string) CmdRunnerOption { return func(c *CmdRunner) { c.tempDir = dir } }

// NewCmdRunner builds a runner. timeout bounds each command; maxBytes bounds
// transform output. Both must be positive.
func NewCmdRunner(timeout time.Duration, maxBytes int64, opts ...CmdRunnerOption) (*CmdRunner, error) {
	if timeout <= 0 {
		return nil, errors.New("attachment: cmd runner timeout must be positive")
	}
	if maxBytes <= 0 {
		return nil, errors.New("attachment: cmd runner maxBytes must be positive")
	}
	c := &CmdRunner{timeout: timeout, maxBytes: maxBytes}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// templateIn / templateOut are the placeholders substituted with runner-owned
// temp file paths in command argument arrays. tempFilePerm is the mode for the
// input temp file (owner read/write only).
const (
	templateIn   = "{in}"
	templateOut  = "{out}"
	tempFilePerm = 0o600
)

// Probe reports whether the command's binary is resolvable on PATH. The
// composition root calls this at startup for every configured command so a
// missing tool surfaces as a warning rather than a per-upload failure.
func (c *CmdRunner) Probe(cmd []string) error {
	if len(cmd) == 0 {
		return errors.New("empty command")
	}
	if _, err := exec.LookPath(cmd[0]); err != nil {
		return fmt.Errorf("binary %q not found on PATH: %w", cmd[0], err)
	}
	return nil
}

// Scan runs cmd over data as a virus/policy scan. A nil error means clean; a
// non-zero exit is mapped to a rejection wrapping [ErrRejected]. The bytes are
// offered via the {in} temp file when the command references it, else on stdin.
func (c *CmdRunner) Scan(ctx context.Context, cmd []string, data []byte) error {
	if len(cmd) == 0 {
		return Rejectedf("scan command is empty")
	}
	_, _, err := c.run(ctx, cmd, data, false)
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
	out, usedOutFile, err := c.run(ctx, cmd, data, true)
	if err != nil {
		return nil, "", fmt.Errorf("transform failed: %w", err)
	}
	if len(out) == 0 && !usedOutFile {
		return nil, "", errors.New("transform produced no output")
	}
	// newName stays empty: transforms keep the file name (a future option may
	// set it for extension-changing converts).
	return out, newName, nil
}

// run executes cmd with {in}/{out} templated to temp files, a timeout, and an
// output-size cap. When wantOutput is true the rewritten bytes are returned —
// from the {out} file if the command uses it, else from stdout. The returned
// bool reports whether an {out} file was used.
func (c *CmdRunner) run(
	ctx context.Context, cmd []string, data []byte, wantOutput bool,
) (output []byte, usedOutFile bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	dir, err := os.MkdirTemp(c.tempDir, "rela-attach-cmd-")
	if err != nil {
		return nil, false, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	args := slices.Clone(cmd)
	usesIn := slices.Contains(args, templateIn)
	usesOut := slices.Contains(args, templateOut)

	inPath := filepath.Join(dir, "in")
	outPath := filepath.Join(dir, "out")
	if usesIn {
		if werr := os.WriteFile(inPath, data, tempFilePerm); werr != nil {
			return nil, false, fmt.Errorf("write input temp: %w", werr)
		}
	}
	for i, a := range args {
		switch a {
		case templateIn:
			args[i] = inPath
		case templateOut:
			args[i] = outPath
		}
	}

	// args[0] is operator-configured (metamodel), not user input; the array
	// form guarantees no shell interpretation of attacker-controlled bytes.
	ec := exec.CommandContext(ctx, args[0], args[1:]...)
	if !usesIn {
		ec.Stdin = bytes.NewReader(data)
	}
	var stdout, stderr bytes.Buffer
	// Cap stdout so a runaway transform can't exhaust memory; +1 to detect over.
	ec.Stdout = &cappedWriter{w: &stdout, remaining: c.maxBytes + 1}
	ec.Stderr = &stderr

	runErr := ec.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, usesOut, fmt.Errorf("command timed out after %s", c.timeout)
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
		out, rerr := os.ReadFile(outPath)
		if rerr != nil {
			return nil, true, fmt.Errorf("read output temp: %w", rerr)
		}
		if int64(len(out)) > c.maxBytes {
			return nil, true, fmt.Errorf("transform output exceeds cap (%d bytes)", c.maxBytes)
		}
		return out, true, nil
	}
	if int64(stdout.Len()) > c.maxBytes {
		return nil, false, fmt.Errorf("transform output exceeds cap (%d bytes)", c.maxBytes)
	}
	return stdout.Bytes(), false, nil
}

// cappedWriter fails the write once more than `remaining` bytes are seen,
// turning an oversize transform into an error instead of OOM.
type cappedWriter struct {
	w         *bytes.Buffer
	remaining int64
}

func (cw *cappedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > cw.remaining {
		// Write what fits so the cap check downstream still trips, then error.
		cw.w.Write(p[:cw.remaining])
		cw.remaining = 0
		return len(p), errors.New("output size cap exceeded")
	}
	cw.remaining -= int64(len(p))
	return cw.w.Write(p)
}
