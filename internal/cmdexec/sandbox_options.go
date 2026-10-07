package cmdexec

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
)

// This file holds sandbox POLICY — the operator-facing knobs and diagnostics
// that decide HOW a command is confined. The mechanism (the Sandbox interface
// and its per-platform backends) lives in sandbox*.go; the core "run a command
// safely" logic lives in cmdexec.go. Keeping policy separate keeps each file to
// one concern.

// Purpose says what a runner's commands are for. It picks which of the
// operator's read-path lists their sandbox gets, so a path one kind of command
// needs is not exposed to the others.
type Purpose int

const (
	// PurposeTransform is for converters fed untrusted content: export
	// transforms, attachment transform steps and document commands. The
	// zero value, so a runner that names no purpose gets this list.
	PurposeTransform Purpose = iota
	// PurposeScan is for attachment scanners. Their list holds what only a
	// scanner may touch, such as the clamd socket: a converter that could
	// connect to it could stop the daemon or probe files it can read.
	PurposeScan

	purposeCount
)

// String names the purpose for logs.
func (p Purpose) String() string {
	if p == PurposeScan {
		return "scan"
	}
	return "transform"
}

// EnvSandboxTransformReadPaths and EnvSandboxScanReadPaths name the environment
// variables holding the operator's read-only sandbox paths per [Purpose]: each
// a list separated like PATH (":" on Unix). Every binary that runs commands
// applies them (rela-server via its --sandbox-*-read-paths flags, which default
// to them; the CLI and rela-desktop via [ApplyHostEnv]), so a command is
// confined the same way whichever binary runs it.
const (
	EnvSandboxTransformReadPaths = "RELA_SANDBOX_TRANSFORM_READ_PATHS"
	EnvSandboxScanReadPaths      = "RELA_SANDBOX_SCAN_READ_PATHS"
)

// EnvVar names the environment variable holding this purpose's read paths.
func (p Purpose) EnvVar() string {
	if p == PurposeScan {
		return EnvSandboxScanReadPaths
	}
	return EnvSandboxTransformReadPaths
}

// readPathsSetting names where the operator sets p's read paths, for warnings:
// the server takes the flag, every binary takes the variable.
func (p Purpose) readPathsSetting() string {
	return p.EnvVar() + " / --sandbox-" + p.String() + "-read-paths"
}

// EnvUnconfinedCommands names the environment variable whose value "1" is the
// operator's host-level opt-out from confinement ([SetUnconfinedByDefault]).
const EnvUnconfinedCommands = "RELA_UNCONFINED_COMMANDS"

// hostReadOnly holds the operator's read-only paths per [Purpose], set once at
// startup by [SetHostReadOnly] and copied into every runner [New] builds for
// that purpose. Atomic because it is written once at startup and read in New.
var hostReadOnly [purposeCount]atomic.Pointer[[]string]

// ApplyHostEnv applies every host-level command setting from the environment:
// [EnvUnconfinedCommands] and the read paths of each [Purpose]. It is the single
// entry point for composition roots that take these settings from the
// environment only (the CLI and rela-desktop), so a new root cannot apply one
// and forget another. Call it once at startup, after logging is configured and
// before any runner is built.
func ApplyHostEnv() {
	SetUnconfinedByDefault(os.Getenv(EnvUnconfinedCommands) == "1")
	for p := range purposeCount {
		SetHostReadOnly(p, ParseReadPaths(os.Getenv(p.EnvVar())))
	}
}

// SetHostReadOnly records the host paths that sandboxed commands of purpose p
// may read, on top of the system binary and library directories. Call it once
// per purpose at startup, before constructing any runner.
//
// rela ships no defaults beyond those system directories. What a converter or
// scanner needs (TeX Live config, libpaper's /etc/paperspecs, a clamd socket
// and its clamd.conf) depends on the distro and on the operator's choice of
// tools, so the operator lists it.
//
// Every path listed is exposed to every command of that purpose. For
// [PurposeTransform] that means converters rendering untrusted content. Files
// under a path are readable: a raw LaTeX block can embed one into an export.
// Unix sockets under it are connectable: a read-only bind does not stop
// connect(), and on macOS each entry is an allowed socket connect target. List
// single files, and a socket file rather than its directory (never /run or
// /var/run, which hold the database and docker sockets).
//
// Rejected with a warning: non-absolute paths, and paths that would undo the
// sandbox or expose what it hides (see [unsafeReadPath]). Duplicates are
// dropped. A path that does not exist yet is kept: a daemon socket may appear
// after rela starts, and the sandbox skips missing paths. Callers that want to
// report missing paths do so themselves.
//
// Returns the accepted paths so the caller can log them.
func SetHostReadOnly(p Purpose, paths []string) []string {
	var accepted []string
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			slog.Warn("cmdexec: ignoring non-absolute sandbox read path",
				"path", path, "setting", p.readPathsSetting())
			continue
		}
		path = filepath.Clean(path)
		if reason := unsafeReadPath(path); reason != "" {
			slog.Warn("cmdexec: ignoring sandbox read path",
				"path", path, "reason", reason, "setting", p.readPathsSetting())
			continue
		}
		if !slices.Contains(accepted, path) {
			accepted = append(accepted, path)
		}
	}
	hostReadOnly[p].Store(&accepted)
	return accepted
}

// unsafeReadPath reports why binding p read-only would undo the sandbox or
// expose what it exists to hide, or "" when p is acceptable.
//
// Operator binds are applied after the sandbox's own /proc, /dev, /tmp and
// writable-dir mounts, so a path at or above one of those replaces it, and a
// path inside /proc or /dev swaps a namespaced view for the host's. A path at or
// above /etc, /run, /var/lib, /home or /root exposes host configuration, every
// daemon socket (database, docker, D-Bus) or data directories that typically
// hold projects, so only paths inside those are allowed.
//
// The check runs on the path as written AND on its symlink-resolved form: bwrap
// follows a symlinked source and the macOS backend resolves paths before
// writing its profile, so /opt/x -> / or /private/tmp (macOS's /tmp) must not
// slip through. On macOS paths compare case-insensitively, as its default
// filesystem does.
func unsafeReadPath(p string) string {
	for _, cand := range withResolved(p) {
		for _, mount := range []string{"/proc", "/dev"} {
			if within(cand, mount) {
				return "replaces the sandbox's own " + mount
			}
		}
		for _, dir := range protectedDirs() {
			if within(dir, cand) {
				return "contains " + dir + "; list single files or sockets inside it"
			}
		}
	}
	return ""
}

// protectedDirs are the directories no operator path may contain (see
// [unsafeReadPath]), each also in its symlink-resolved form.
func protectedDirs() []string {
	dirs := []string{
		"/proc", "/dev", "/tmp", os.TempDir(),
		"/etc", "/run", "/var/run", "/var/lib", "/home", "/root",
	}
	out := make([]string, 0, 2*len(dirs))
	for _, d := range dirs {
		out = append(out, withResolved(filepath.Clean(d))...)
	}
	return out
}

// withResolved returns p and, when it differs, p with symlinks resolved. A path
// that does not exist yet is resolved through its longest existing ancestor, so
// a future socket under a symlinked directory is still checked where it lands.
func withResolved(p string) []string {
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			if resolved := filepath.Join(r, rest); resolved != p {
				return []string{p, resolved}
			}
			return []string{p}
		}
		if dir == filepath.Dir(dir) {
			return []string{p}
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// within reports whether p is dir or lies under it. Both must be clean. On
// macOS the comparison ignores case, matching its default filesystem.
func within(p, dir string) bool {
	if runtime.GOOS == "darwin" {
		p, dir = strings.ToLower(p), strings.ToLower(dir)
	}
	if p == dir {
		return true
	}
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// CheckProjectNotExposed returns an error when an operator read path is the
// project directory, lies inside it, or contains it. The sandbox exists to keep
// the project (entities of every user, .rela secrets) away from converters
// that render untrusted content; cmdexec cannot know where the project lives,
// so each composition root that has one calls this after [SetHostReadOnly].
// Symlinks are resolved on both sides.
func CheckProjectNotExposed(projectRoot string) error {
	if projectRoot == "" {
		return nil
	}
	roots := withResolved(filepath.Clean(projectRoot))
	for purpose := range purposeCount {
		for _, p := range HostReadOnly(purpose) {
			for _, cand := range withResolved(p) {
				for _, root := range roots {
					if within(cand, root) || within(root, cand) {
						return fmt.Errorf("sandbox read path %s exposes the project directory %s to every "+
							"sandboxed %s command; list paths outside the project (%s)",
							p, projectRoot, purpose, purpose.readPathsSetting())
					}
				}
			}
		}
	}
	return nil
}

// ParseReadPaths splits a PATH-style list (the format of [Purpose.EnvVar])
// into its entries, dropping empty ones so a trailing separator is harmless.
func ParseReadPaths(s string) []string {
	var out []string
	for _, p := range filepath.SplitList(s) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HostReadOnly returns the operator's read paths for purpose p currently in
// effect, as set by [SetHostReadOnly]. Diagnostic only, for startup warnings:
// runners copy the list at construction.
func HostReadOnly(p Purpose) []string {
	if p := hostReadOnly[p].Load(); p != nil {
		return slices.Clone(*p)
	}
	return nil
}

// WithExtraReadOnly binds additional host paths read-only into this runner's
// sandbox, on top of the system directories and the operator's
// [SetHostReadOnly] list for its purpose. A unix socket is a filesystem object, so binding one
// grants reachability WITHOUT opening network egress. Missing paths are skipped.
//
// Only ABSOLUTE paths are honored: a bind mount has no meaningful cwd-relative
// form, and an empty or relative entry would be silently dropped by the
// backend's -try, leaving the operator to wonder why the path is still
// unreachable. Such entries are warned about and skipped here so
// the misconfiguration is visible in the log rather than an invisible no-op.
// Paths refused by [SetHostReadOnly] are refused here too.
//
// No production runner uses it today: host paths come from the operator lists.
// It stays for a runner that needs a path no other command should see, and
// tests use it to bind a scratch socket.
func WithExtraReadOnly(paths ...string) Option {
	return func(r *Runner) {
		for _, p := range paths {
			if !filepath.IsAbs(p) {
				slog.Warn("cmdexec: ignoring non-absolute extra read-only bind path", "path", p)
				continue
			}
			p = filepath.Clean(p)
			if reason := unsafeReadPath(p); reason != "" {
				slog.Warn("cmdexec: ignoring extra read-only bind path", "path", p, "reason", reason)
				continue
			}
			r.extraReadOnly = append(r.extraReadOnly, p)
		}
	}
}

// WithSandboxDisabled turns confinement off. This is the operator's explicit
// "I accept running third-party parsers on untrusted input unconfined" escape
// hatch — for hosts where no mechanism exists (Windows/BSD, a kernel without
// unprivileged user namespaces) or where isolation is provided at a different
// layer (a locked-down container, a no-egress network policy).
//
// Callers that expose this MUST log a one-time startup warning naming the risk,
// matching how a disabled attachment scan is surfaced.
func WithSandboxDisabled() Option {
	return func(r *Runner) { r.sandboxOptOut = true }
}

// unconfinedByDefault, when true, makes every [New] runner opt out of
// confinement unless a WithSandboxDisabled option says otherwise. It is the
// single host-level knob the composition roots (server and CLI) set from the
// RELA_UNCONFINED_COMMANDS env var, so the CLI and server agree without each
// call site threading a bool. Atomic because it is read in New and written once
// at startup.
var unconfinedByDefault atomic.Bool

// SetUnconfinedByDefault records the host-level opt-out. Call it once at startup,
// before constructing any runner. When true, commands run UNCONFINED — the
// operator has accepted the risk (typically because the host cannot sandbox).
// Returns the value it set so the caller can log it.
func SetUnconfinedByDefault(v bool) bool {
	unconfinedByDefault.Store(v)
	return v
}

func unconfinedDefault() bool { return unconfinedByDefault.Load() }

// UnconfinedByDefault reports the host-level opt-out set by
// [SetUnconfinedByDefault], for startup diagnostics.
func UnconfinedByDefault() bool { return unconfinedDefault() }

// Describe returns a one-line summary of how commands are confined, for the
// startup log so an operator learns the posture before anything fails.
//
// This is the ONLY confinement accessor. There is deliberately no "can I run?"
// predicate: callers just call [Runner.Run] and handle its error, which reads
// the same whether the cause is a missing sandbox, a missing binary, or a
// crashing converter. A separate pre-check would be a second code path to keep
// in sync and a window for the state to change between check and use.
//
// ExtraReadOnly returns the host paths this runner binds read-only into every
// command's sandbox, on top of the system directories: the operator's
// [SetHostReadOnly] list plus any [WithExtraReadOnly] paths.
//
// Diagnostic and test-support only: it lets a caller assert it was WIRED with
// the binds it needs. Do not use it to decide whether a command can run — that
// remains [Runner.Run]'s error to report.
func (r *Runner) ExtraReadOnly() []string {
	out := make([]string, len(r.extraReadOnly))
	copy(out, r.extraReadOnly)
	return out
}

// SandboxErr reports why commands run by this runner will FAIL, or nil when they
// will run.
//
// Nil: does NOT mean "confined". It is also nil when the operator opted out via
// [WithSandboxDisabled], where commands run unconfined but do run. The question
// this answers is "will a command fail?", not "is it sandboxed?" — use
// [Runner.Describe] for the posture.
//
// It exists so a composition root can WARN AT STARTUP that configured
// scan/transform commands will all fail closed, instead of the operator learning
// it from a rejected upload.
//
// Never branch a SECURITY CONTROL on it. Skipping a scan because this is non-nil
// converts a fail-closed rejection into an unscanned upload — the one change
// that turns a broken sandbox from an outage into a vulnerability. It is equally
// not the "can I run?" predicate [Runner.Describe] documents as deliberately
// absent: call [Runner.Run] and handle its error, which stays the single
// execution path.
func (r *Runner) SandboxErr() error { return r.sandboxErr }

// Diagnostic only — never branch on this string.
func (r *Runner) Describe() string {
	switch {
	case r.sandboxOptOut:
		return "sandbox DISABLED by operator (commands run unconfined)"
	case r.sandboxErr != nil:
		return "sandbox unavailable (commands will refuse to run): " + r.sandboxErr.Error()
	default:
		limits := "no resource limits (non-Linux)"
		if rlimitsSupported() {
			limits = "memory/file-size/CPU limits"
		}
		listed := "none"
		if len(r.extraReadOnly) > 0 {
			listed = strings.Join(r.extraReadOnly, ":")
		}
		// Only the Linux backend confines reads; on macOS the operator list
		// selects the unix sockets a command may connect to.
		access := "reads: system directories only"
		if len(r.extraReadOnly) > 0 {
			access = "reads: system directories + " + listed
		}
		if runtime.GOOS != "linux" {
			access = "reads: unrestricted, unix-socket connects: " + listed
		}
		return "sandbox " + r.sandbox.Name() + " (no network, temp-dir-only writes, " + access + ") + " + limits
	}
}
