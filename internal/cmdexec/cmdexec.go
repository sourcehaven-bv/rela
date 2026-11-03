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
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// TemplateIn / TemplateOut are the placeholders substituted with runner-owned
// temp file paths in command argument arrays. A command that references neither
// receives its input on stdin and returns output on stdout.
const (
	TemplateIn  = "{in}"
	TemplateOut = "{out}"

	tempFilePerm = 0o600
	tempDirPerm  = 0o700
)

// Runner drives external commands. A nil *Runner is not valid; use [New].
type Runner struct {
	timeout  time.Duration
	maxBytes int64
	tempDir  string // where {in}/{out} temp files are created ("" → os.TempDir)

	// sandbox confines each command (no network, only the run's temp dir
	// writable). Exactly one of three implementations is chosen at construction:
	// the platform backend when it works, [disabledSandbox] when the operator
	// explicitly opted out, or [unavailableSandbox] when no mechanism is usable —
	// whose Wrap ERRORS, which is what makes an unsandboxable host fail closed.
	//
	// Because the choice is made once, Run always calls Wrap unconditionally;
	// there is no "should I sandbox?" branch left to forget.
	sandbox Sandbox
	// sandboxErr caches the availability probe so a broken host is diagnosed once
	// at construction rather than per invocation. Non-nil does NOT by itself mean
	// commands fail — an operator opt-out records the reason here while still
	// running (see SandboxEnforced).
	sandboxErr error
	// sandboxOptOut records that the operator explicitly accepted unconfined
	// execution, so diagnostics can distinguish "you turned it off" from "this
	// host cannot do it".
	sandboxOptOut bool

	// limits bound what the command may CONSUME (memory, file size, CPU). The
	// sandbox bounds what it may REACH; without limits a confined converter can
	// still exhaust the host. Process COUNT is deliberately not among them — see
	// applyRlimits (limits_linux.go) for why RLIMIT_NPROC cannot express it.
	limits Limits

	// slots bounds how many commands may run CONCURRENTLY. Per-command limits
	// cap one process; without a concurrency bound, N simultaneous requests
	// multiply that N-fold, which is the cheapest way to exhaust the host. nil
	// means unbounded.
	slots chan struct{}

	// purpose picks the operator's read-path list (SetHostReadOnly).
	purpose Purpose

	// extraReadOnly are host paths bound read-only into every command's sandbox,
	// on top of the system binary/library directories: the operator's
	// SetHostReadOnly list for purpose, then any WithExtraReadOnly paths.
	extraReadOnly []string
}

// Option configures a [Runner].
type Option func(*Runner)

// WithPurpose sets what the runner's commands are for, which picks the
// operator's read paths their sandbox gets. Without it a runner is
// [PurposeTransform].
func WithPurpose(p Purpose) Option { return func(r *Runner) { r.purpose = p } }

// WithTempDir sets the directory for {in}/{out} temp files.
func WithTempDir(dir string) Option { return func(r *Runner) { r.tempDir = dir } }

// WithMaxConcurrent bounds how many commands may run at once. Per-command
// limits cap a single process; this caps the aggregate, so N simultaneous
// export/upload requests cannot multiply resource use N-fold. Callers past the
// bound block until a slot frees or their context is done.
//
// n <= 0 leaves concurrency unbounded.
func WithMaxConcurrent(n int) Option {
	return func(r *Runner) {
		if n > 0 {
			r.slots = make(chan struct{}, n)
		}
	}
}

// hostSandboxAvailable caches the platform availability probe process-wide.
// The probe is a real subprocess ([Sandbox.Available] execs bwrap/sandbox-exec
// with a 10s budget) and the host's posture cannot change within the process,
// so every Runner after the first reuses the result instead of re-forking.
var hostSandboxAvailable = sync.OnceValue(func() error {
	return NewSandbox().Available()
})

// New builds a runner. timeout bounds each command; maxBytes bounds output.
// Both must be positive.
//
// The sandbox availability is probed once per process (first New). When it is
// unavailable, New still SUCCEEDS — a missing sandbox must not stop a server
// from booting and serving everything that does not shell out. The failure
// surfaces two ways: [Runner.Describe] for the startup log, and an error from
// every [Runner.Run], which is what actually blocks execution.
func New(timeout time.Duration, maxBytes int64, opts ...Option) (*Runner, error) {
	if timeout <= 0 {
		return nil, errors.New("cmdexec: timeout must be positive")
	}
	if maxBytes <= 0 {
		return nil, errors.New("cmdexec: maxBytes must be positive")
	}
	r := &Runner{
		timeout:       timeout,
		maxBytes:      maxBytes,
		limits:        DefaultLimits(),
		sandboxOptOut: unconfinedDefault(), // host-level knob; an explicit option below overrides
	}
	for _, o := range opts {
		o(r)
	}
	// After the options, which set the purpose and may add their own paths.
	r.extraReadOnly = append(HostReadOnly(r.purpose), r.extraReadOnly...)

	// The three-way confinement decision, made ONCE. After this, Run always calls
	// sandbox.Wrap unconditionally — there is no per-invocation branch that could
	// be edited into silently skipping confinement.
	platform := NewSandbox()
	switch avail := hostSandboxAvailable(); {
	case r.sandboxOptOut:
		// Operator explicitly accepted unconfined execution. Record why the
		// platform sandbox was (or wasn't) usable so diagnostics stay honest.
		r.sandbox, r.sandboxErr = disabledSandbox{}, avail
	case avail != nil:
		// No usable mechanism and no opt-out: keep the unavailable sandbox, whose
		// Wrap errors. Commands refuse to run rather than run unconfined.
		r.sandbox, r.sandboxErr = platform, avail
	default:
		r.sandbox, r.sandboxErr = platform, nil
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
	res, err := r.run(ctx, cmd, data, wantOutput, RunSpec{}, false)
	return res.Output, res.UsedOutFile, err
}

// RunSpec holds per-call settings for [Runner.RunWith]. The zero value runs
// like [Runner.Run].
type RunSpec struct {
	// InputName is the {in} file's path relative to the run's temp dir (""
	// means "in"): a file name, or one directory and a file name separated by
	// "/". Some tools select behavior by path (a prose checker's per-type
	// rules), so the path is part of the input. Each segment must match
	// [inputNamePattern].
	InputName string
	// AcceptedExitCodes lists non-zero exit codes that still count as success:
	// the output is returned and [RunResult.ExitCode] reports the code. Linters
	// exit non-zero when they find something.
	AcceptedExitCodes []int
	// Timeout replaces the runner's timeout when positive. It may not exceed
	// the runner's timeout, so a caller can only shorten the bound.
	Timeout time.Duration
}

// RunResult is what [Runner.RunWith] returns.
type RunResult struct {
	Output      []byte
	UsedOutFile bool
	ExitCode    int
}

// inputNamePattern allowlists each segment of [RunSpec.InputName]: no leading
// dot, no glob or shell metacharacters.
var inputNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// writeInput writes data to name under the run's temp dir, creating the one
// directory a [RunSpec.InputName] may have.
func writeInput(dir, name string, data []byte) error {
	if sub := filepath.Dir(name); sub != "." {
		if err := os.Mkdir(filepath.Join(dir, sub), tempDirPerm); err != nil {
			return fmt.Errorf("cmdexec: create input dir: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, tempFilePerm); err != nil {
		return fmt.Errorf("cmdexec: write input temp: %w", err)
	}
	return nil
}

// validInputName reports whether name is one or two allowlisted segments
// that cannot collide with the {out} file. "out" is refused as either
// segment, without regard to case: a directory named out would take the
// {out} path, and macOS file systems ignore case.
func validInputName(name string) bool {
	segs := strings.Split(name, "/")
	if len(segs) > 2 {
		return false
	}
	for _, s := range segs {
		if !inputNamePattern.MatchString(s) || strings.Contains(s, "..") || strings.EqualFold(s, "out") {
			return false
		}
	}
	return true
}

// RunWith is [Runner.Run] with per-call settings, always returning output.
//
// Unlike Run, the command's working directory is the run's temp dir on every
// platform, not rela's (often the project root), so a tool that searches
// upward from its working directory for config finds none. The Linux sandbox
// already does this inside its namespace; Run keeps the inherited directory
// elsewhere because existing commands may name a program by relative path.
func (r *Runner) RunWith(ctx context.Context, cmd []string, data []byte, spec RunSpec) (RunResult, error) {
	return r.run(ctx, cmd, data, true, spec, true)
}

// templateArgs copies cmd with {in} and {out} replaced by paths in dir, and
// writes data to the {in} file when the command uses one.
func templateArgs(cmd []string, dir, inName string, data []byte) (args []string, usesIn, usesOut bool, err error) {
	args = slices.Clone(cmd)
	usesIn = slices.Contains(args, TemplateIn)
	usesOut = slices.Contains(args, TemplateOut)
	if usesIn {
		if err := writeInput(dir, inName, data); err != nil {
			return nil, false, false, err
		}
	}
	for i, a := range args {
		switch a {
		case TemplateIn:
			args[i] = filepath.Join(dir, inName)
		case TemplateOut:
			args[i] = filepath.Join(dir, "out")
		}
	}
	return args, usesIn, usesOut, nil
}

func (r *Runner) run(
	ctx context.Context, cmd []string, data []byte, wantOutput bool, spec RunSpec, inTempDir bool,
) (RunResult, error) {
	if len(cmd) == 0 {
		return RunResult{}, errors.New("cmdexec: empty command")
	}
	inName := "in"
	if spec.InputName != "" {
		if !validInputName(spec.InputName) {
			return RunResult{}, fmt.Errorf("cmdexec: invalid input name %q", spec.InputName)
		}
		inName = spec.InputName
	}
	timeout := r.timeout
	if spec.Timeout > 0 && spec.Timeout < timeout {
		timeout = spec.Timeout
	}

	// Bound aggregate concurrency before doing any work (including the timeout
	// clock, which should measure this command's own run, not its queue wait).
	if r.slots != nil {
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		case <-ctx.Done():
			return RunResult{}, fmt.Errorf("cmdexec: waiting for a run slot: %w", ctx.Err())
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dir, err := os.MkdirTemp(r.tempDir, "rela-cmdexec-")
	if err != nil {
		return RunResult{}, fmt.Errorf("cmdexec: create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	args, usesIn, usesOut, err := templateArgs(cmd, dir, inName, data)
	if err != nil {
		return RunResult{}, err
	}
	outPath := filepath.Join(dir, "out")

	args, err = r.confine(args, dir)
	if err != nil {
		return RunResult{UsedOutFile: usesOut}, err
	}

	var stdin io.Reader
	if !usesIn {
		stdin = bytes.NewReader(data)
	}
	workDir := ""
	if inTempDir {
		workDir = dir
	}
	stdout, exitCode, runErr := r.execute(ctx, args, stdin, workDir, timeout, spec.AcceptedExitCodes)
	if runErr != nil {
		return RunResult{UsedOutFile: usesOut}, runErr
	}

	res := RunResult{UsedOutFile: usesOut, ExitCode: exitCode}
	if !wantOutput {
		return res, nil
	}

	if usesOut {
		out, rerr := os.ReadFile(outPath) // outPath is runner-owned temp, not caller input
		if rerr != nil {
			return res, fmt.Errorf("cmdexec: read output temp: %w", rerr)
		}
		if int64(len(out)) > r.maxBytes {
			return res, fmt.Errorf("cmdexec: output exceeds cap (%d bytes)", r.maxBytes)
		}
		res.Output = out
		return res, nil
	}
	if int64(stdout.Len()) > r.maxBytes {
		return res, fmt.Errorf("cmdexec: output exceeds cap (%d bytes)", r.maxBytes)
	}
	res.Output = stdout.Bytes()
	return res, nil
}

// execute starts the (already-confined) command, in dir when it is set,
// applies resource ceilings, and waits for it — killing the whole process
// group if the deadline passes. Returns the captured stdout buffer and the
// exit code; a non-zero exit listed in accepted is not an error.
func (r *Runner) execute(
	ctx context.Context, args []string, stdin io.Reader, dir string, timeout time.Duration, accepted []int,
) (*bytes.Buffer, int, error) {
	// Every element of args is the sandbox wrapper, the caller-configured
	// command (project config), or a runner-owned temp path for {in}/{out};
	// request data arrives only on stdin or in the {in} file. The array form
	// means no shell interprets any of it.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	ec := exec.CommandContext(ctx, args[0], args[1:]...)
	ec.Env = Environ()
	ec.Dir = dir // "" inherits rela's; see RunWith
	ec.Stdin = stdin
	var stdout, stderr bytes.Buffer
	// Cap stdout so a runaway command can't exhaust memory; +1 to detect over.
	ec.Stdout = &cappedWriter{w: &stdout, remaining: r.maxBytes + 1}
	ec.Stderr = &stderr

	// Put the child in its own process group so a timeout can take down the WHOLE
	// tree: exec's default kill signals only the direct child, which a detached
	// grandchild (pandoc → PDF engine) outlives.
	applyLimits(ec, r.limits)

	// Tear the group down via Cmd.Cancel rather than a hand-rolled watchdog. The
	// runtime calls Cancel only between Start and Wait and never after Wait has
	// reaped, which is what makes this safe: signaling a raw pgid after the
	// leader is reaped could hit an UNRELATED group once the kernel recycles the
	// pid. WaitDelay then guarantees Wait returns even if a descendant holds the
	// output pipes open.
	ec.Cancel = func() error {
		killProcessGroup(ec)
		return os.ErrProcessDone // suppress the redundant single-process kill
	}
	ec.WaitDelay = waitDelay

	if startErr := ec.Start(); startErr != nil {
		return nil, 0, fmt.Errorf("cmdexec: start %q: %w", args[0], startErr)
	}
	// Resource ceilings, applied as soon as the child has a pid. Failing to set
	// them is fatal: an unbounded converter can exhaust the host, so kill the
	// process rather than let it run without limits.
	if limErr := applyRlimits(ec.Process.Pid, r.limits); limErr != nil {
		killProcessGroup(ec)
		_ = ec.Wait()
		return nil, 0, fmt.Errorf("cmdexec: apply resource limits: %w", limErr)
	}

	runErr := ec.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, 0, fmt.Errorf("cmdexec: command timed out after %s", timeout)
	}
	// WaitDelay fires when the child itself exited but a detached descendant kept
	// an output pipe open past waitDelay (a pandoc PDF-engine grandchild is the
	// canonical case). Wait then closes the pipes and returns ErrWaitDelay. This
	// is NOT a command failure: the child completed and, when it writes to {out},
	// the result is already on disk. Distinguish it from a real non-zero exit
	// (which surfaces as *exec.ExitError, not ErrWaitDelay) and treat it as
	// success — Wait has already joined the copier goroutine, so stdout is stable.
	if errors.Is(runErr, exec.ErrWaitDelay) {
		slog.Warn("cmdexec: command left a lingering child past wait delay; "+
			"output taken as-is", "cmd", args[0], "wait_delay", waitDelay)
		return &stdout, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() > 0 && slices.Contains(accepted, exitErr.ExitCode()) {
		return &stdout, exitErr.ExitCode(), nil
	}
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, 0, fmt.Errorf("%w: %s", runErr, msg)
	}
	return &stdout, 0, nil
}

// confine rewrites argv to run inside the sandbox: no network, and only this
// run's temp dir writable.
//
// The data being converted is attacker-influenceable (an entity body, an
// uploaded file), and converters fetch remote resources by design — an
// unconfined run is a server-side request forgery primitive, and a parser bug
// becomes a foothold. Fails CLOSED when confinement is required but unavailable;
// it never silently downgrades to an unconfined run.
func (r *Runner) confine(args []string, dir string) ([]string, error) {
	// Unconditional: the implementation chosen in New already encodes the policy.
	// A working backend wraps; disabledSandbox passes through (operator opted
	// out); unavailableSandbox errors, which is the fail-closed path.
	wrapped, err := r.sandbox.Wrap(args, Spec{WritableDir: dir, ExtraReadOnly: r.extraReadOnly})
	if err != nil {
		if errors.Is(err, ErrSandboxUnavailable) {
			return nil, fmt.Errorf("cmdexec: refusing to run %q unconfined: %w", args[0], err)
		}
		return nil, fmt.Errorf("cmdexec: sandbox: %w", err)
	}
	return wrapped, nil
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
