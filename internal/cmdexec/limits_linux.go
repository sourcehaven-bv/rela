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
//
// RLIMIT_NPROC is deliberately NOT set, despite "bound a fork bomb" being a
// real goal (BUG-EA0E8M). It is the wrong mechanism for that goal at any value:
// the kernel checks it in fork(2) against a count of every process and thread
// owned by the real UID, not against this command's descendants. Setting it to
// 256 therefore did not grant the child 256 forks — it made the child's forks
// fail whenever the UID was already above 256, for reasons having nothing to do
// with the child. Worse, the ceiling applies to the shared counter while the
// child holds it, so rela was setting policy on a resource belonging to every
// other process running as the same user.
//
// That is not hypothetical: it took down every `command:` document render in a
// CI job, with `bwrap: Creating new namespace failed: Resource temporarily
// unavailable` (EAGAIN from fork), because a Playwright suite on the same UID
// held a few hundred browser threads. A server sharing its UID with anything
// busy hits the same wall.
//
// What bounds a runaway command instead, all of it correctly scoped:
//
//   - --unshare-pid (Sandbox.Wrap) puts each command in its own PID namespace,
//     so a fork bomb cannot outlive or escape that namespace;
//   - killProcessGroup reaps the whole tree when the timeout fires;
//   - RLIMIT_AS and RLIMIT_CPU below are genuinely per-process;
//   - Runner.WithMaxConcurrent bounds how many commands run at once, which is
//     the aggregate bound RLIMIT_NPROC was reached for.
//
// Operators wanting a hard per-service process ceiling should set it where it
// is cgroup-scoped rather than UID-scoped — `TasksMax=` in the rela-server
// systemd unit (see docs/attachment-security.md). That bounds the service and its
// children only, which is what this code wanted and could not express.
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
	if err := set(unix.RLIMIT_FSIZE, l.MaxFileSize); err != nil {
		return err
	}
	return set(unix.RLIMIT_CPU, l.MaxCPUSeconds)
}

// rlimitsSupported reports that this platform can enforce resource ceilings.
func rlimitsSupported() bool { return true }
