//go:build !linux && !darwin

package cmdexec

import "runtime"

// newPlatformSandbox returns the no-mechanism fallback for platforms where we
// have no way to confine a child process.
//
// Windows: AppContainer would need a hand-rolled restricted-token/capability-SID
// dance in unsafe syscall code, and job objects bound resources but NOT network
// or filesystem — they would not meet the requirement, so claiming support would
// be worse than admitting none. BSD: pledge/unveil (OpenBSD) and Capsicum
// (FreeBSD) are self-applied by the target program; they cannot be imposed on a
// third-party converter from outside.
//
// Callers fail closed on ErrSandboxUnavailable, so untrusted input is refused
// here rather than silently run unconfined.
func newPlatformSandbox() Sandbox {
	return unavailableSandbox{
		reason: "no supported sandbox mechanism on " + runtime.GOOS +
			" (Linux uses bubblewrap; macOS uses sandbox-exec)",
	}
}
