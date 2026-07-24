package cmdexec

import (
	"log/slog"
	"path/filepath"
	"sync"
)

// This file holds sandbox POLICY — the operator-facing knobs and diagnostics
// that decide HOW a command is confined. The mechanism (the Sandbox interface
// and its per-platform backends) lives in sandbox*.go; the core "run a command
// safely" logic lives in cmdexec.go. Keeping policy separate keeps each file to
// one concern.

// DefaultScannerSockets are well-known unix-socket locations for scanner daemons
// (ClamAV's clamd), bound read-only into the sandbox so a scan command can reach
// them. Non-existent paths are skipped, so listing several distro defaults is
// harmless. An operator with a non-standard location extends this via
// [WithExtraReadOnly].
var DefaultScannerSockets = []string{
	"/var/run/clamav/clamd.ctl", // Debian/Ubuntu
	"/run/clamav/clamd.ctl",
	"/var/run/clamav/clamd.sock",
	"/run/clamav/clamd.sock",
	// A socket under /tmp is bound best-effort: the Linux backend mounts a fresh
	// tmpfs at /tmp and binds this single path on top, so only the socket inode
	// (not sibling lock/pid files) is visible, and a clamd restart that re-creates
	// the socket needs a fresh runner. Prefer a non-/tmp LocalSocket in production.
	"/tmp/clamd.socket", // some source builds / RHEL
}

// WithExtraReadOnly binds additional host paths read-only into every command's
// sandbox, on top of the standard allowlist. The motivating case is a scanner
// daemon's unix socket — a socket is a filesystem object, so binding it grants
// reachability WITHOUT opening network egress. Missing paths are skipped.
//
// Only ABSOLUTE paths are honored: a bind mount has no meaningful cwd-relative
// form, and an empty or relative entry (a typo'd `scan_sockets:` value) would be
// silently dropped by the backend's -try, leaving the operator to wonder why the
// socket is still unreachable. Such entries are warned about and skipped here so
// the misconfiguration is visible in the log rather than an invisible no-op.
func WithExtraReadOnly(paths ...string) Option {
	return func(r *Runner) {
		for _, p := range paths {
			if !filepath.IsAbs(p) {
				slog.Warn("cmdexec: ignoring non-absolute extra read-only bind path", "path", p)
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
// call site threading a bool. Guarded because it is read in New and written once
// at startup.
var (
	unconfinedMu        sync.RWMutex
	unconfinedByDefault bool
)

// SetUnconfinedByDefault records the host-level opt-out. Call it once at startup,
// before constructing any runner. When true, commands run UNCONFINED — the
// operator has accepted the risk (typically because the host cannot sandbox).
// Returns the value it set so the caller can log it.
func SetUnconfinedByDefault(v bool) bool {
	unconfinedMu.Lock()
	defer unconfinedMu.Unlock()
	unconfinedByDefault = v
	return v
}

func unconfinedDefault() bool {
	unconfinedMu.RLock()
	defer unconfinedMu.RUnlock()
	return unconfinedByDefault
}

// Describe returns a one-line summary of how commands are confined, for the
// startup log so an operator learns the posture before anything fails.
//
// This is the ONLY confinement accessor. There is deliberately no "can I run?"
// predicate: callers just call [Runner.Run] and handle its error, which reads
// the same whether the cause is a missing sandbox, a missing binary, or a
// crashing converter. A separate pre-check would be a second code path to keep
// in sync and a window for the state to change between check and use.
//
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
			limits = "memory/PID/file-size/CPU limits"
		}
		return "sandbox " + r.sandbox.Name() + " (no network, temp-dir-only writes) + " + limits
	}
}
