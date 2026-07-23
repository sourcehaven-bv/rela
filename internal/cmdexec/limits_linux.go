package cmdexec

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// platformApplyLimits places the child in its own process group. The rlimits
// themselves cannot be set here: Go's syscall.SysProcAttr exposes no Rlimits
// field on Linux (that is a FreeBSD-only field), and there is no hook to run
// code between fork and exec. They are applied by [applyRlimits] immediately
// after Start instead.
func platformApplyLimits(cmd *exec.Cmd, _ Limits) { setPgid(cmd) }

// applyRlimits sets the child's resource ceilings via prlimit(2) once it has a
// pid. Called immediately after Start.
//
// This is a post-start application, so there is a microseconds-wide window
// before the limits land. That is acceptable here: the window closes long before
// the converter has read, parsed, or acted on any attacker-influenced input, and
// the alternative (a re-exec trampoline that sets its own rlimits pre-exec) buys
// no practical safety for materially more machinery. Errors are returned so the
// caller can fail closed rather than run an unbounded process.
func applyRlimits(pid int, l Limits) error {
	set := func(resource int, v uint64) error {
		if v == 0 {
			return nil // unset dimension
		}
		rl := unix.Rlimit{Cur: v, Max: v}
		return unix.Prlimit(pid, resource, &rl, nil)
	}
	if err := set(unix.RLIMIT_AS, l.MaxAddressSpace); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_NPROC, l.MaxProcesses); err != nil {
		return err
	}
	if err := set(unix.RLIMIT_FSIZE, l.MaxFileSize); err != nil {
		return err
	}
	return set(unix.RLIMIT_CPU, l.MaxCPUSeconds)
}

// rlimitsSupported reports that this platform can enforce resource ceilings.
func rlimitsSupported() bool { return true }
