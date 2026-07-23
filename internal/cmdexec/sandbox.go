package cmdexec

import (
	"errors"
	"fmt"
	"time"
)

// ErrSandboxUnavailable is returned by [Sandbox.Available] when this platform
// or host cannot provide a working sandbox — no supported mechanism is compiled
// in (Windows/BSD), the helper binary is missing (no bwrap), or the kernel
// refuses it (unprivileged user namespaces disabled). Callers that run untrusted
// input MUST fail closed on it rather than silently executing unsandboxed.
var ErrSandboxUnavailable = errors.New("cmdexec: no working sandbox available")

// probeTimeout bounds the startup availability check. The probe runs a trivial
// command; if it has not returned by now the mechanism is not usable anyway.
const probeTimeout = 10 * time.Second

// Spec describes the confinement a single command needs. The zero value is the
// safe default: no network, nothing writable.
//
// Filesystem: the command may read the system read-only, and may write ONLY
// inside WritableDir (the runner-owned temp dir holding {in}/{out}). Network:
// denied unless Network is true — no backend currently sets it, but the field
// makes "this command legitimately needs egress" an explicit, greppable choice
// rather than an accident.
type Spec struct {
	// WritableDir is the single directory the command may write to. Required;
	// Wrap returns an error when empty, because a sandbox that lets a command
	// write anywhere is not a sandbox.
	WritableDir string
	// Network allows egress when true. Default false — the whole point.
	Network bool
}

// Sandbox confines an external command. Implementations rewrite an argv into a
// wrapped argv (e.g. prefixing `bwrap …` or `sandbox-exec -f profile …`) rather
// than managing the process themselves, which keeps them pure and unit-testable
// and lets the caller keep using one exec path.
//
// A Sandbox is safe for concurrent use.
type Sandbox interface {
	// Available reports whether this sandbox actually works on this host. It
	// must perform a real check (running the helper, not merely finding it on
	// PATH) so a silently-broken sandbox cannot masquerade as a working one.
	// Returns nil when usable, else an error wrapping [ErrSandboxUnavailable].
	Available() error

	// Wrap returns argv rewritten to run confined per spec. It does not execute
	// anything. Implementations must not mutate the input argv.
	Wrap(argv []string, spec Spec) ([]string, error)

	// Name identifies the mechanism ("bubblewrap", "sandbox-exec", "none") for
	// logging and diagnostics.
	Name() string
}

// NewSandbox returns the platform's sandbox implementation. It always returns a
// non-nil Sandbox; call [Sandbox.Available] to learn whether it works here. On
// platforms with no supported mechanism this is [unavailableSandbox], whose
// Available always reports [ErrSandboxUnavailable].
func NewSandbox() Sandbox { return newPlatformSandbox() }

// unavailableSandbox is the fallback for platforms with no supported mechanism.
// It never pretends to confine anything: Available always fails, and Wrap
// refuses rather than returning the argv unchanged — returning it unwrapped
// would be the dangerous outcome (a caller that ignored Available would run
// untrusted input with no confinement and no signal).
type unavailableSandbox struct{ reason string }

func (u unavailableSandbox) Name() string { return "none" }

func (u unavailableSandbox) Available() error {
	return fmt.Errorf("%w: %s", ErrSandboxUnavailable, u.reason)
}

func (u unavailableSandbox) Wrap([]string, Spec) ([]string, error) {
	return nil, u.Available()
}

// disabledSandbox is the operator's explicit "run unconfined" choice: Wrap
// passes the argv through untouched.
//
// It is deliberately a SEPARATE type from [unavailableSandbox], and the only way
// to select it is [WithSandboxDisabled]. Collapsing the two would be a security
// bug: "no mechanism on this host" must fail CLOSED (refuse to run), while "the
// operator accepted the risk" runs open. One no-op type serving both would turn
// a missing sandbox into a silent unconfined execution.
//
// Available returns nil so the runner treats this as a working configuration —
// the risk was accepted at construction, and callers surface it via Name.
type disabledSandbox struct{}

func (disabledSandbox) Name() string { return "disabled" }

func (disabledSandbox) Available() error { return nil }

func (disabledSandbox) Wrap(argv []string, spec Spec) ([]string, error) {
	if err := validateSpec(argv, spec); err != nil {
		return nil, err
	}
	return argv, nil
}

// validateSpec is the shared precondition check for every backend.
func validateSpec(argv []string, spec Spec) error {
	if len(argv) == 0 {
		return errors.New("cmdexec: sandbox: empty command")
	}
	if spec.WritableDir == "" {
		return errors.New("cmdexec: sandbox: WritableDir is required")
	}
	return nil
}
