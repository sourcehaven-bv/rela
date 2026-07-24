//go:build unix && !linux

package cmdexec

import "os/exec"

// platformApplyLimits places the child in its own process group so a timeout
// kills the whole tree (the orphaned-grandchild case). Resource ceilings are not
// applied: prlimit(2) is Linux-specific, and setrlimit on the parent would bound
// the rela server itself rather than the converter.
//
// The practical consequence is that on macOS/BSD a converter is confined
// (sandbox-exec blocks network and writes) but NOT resource-bounded. Those are
// development tiers; production guidance is Linux, where both hold.
func platformApplyLimits(cmd *exec.Cmd, _ Limits) { setPgid(cmd) }

// applyRlimits is a no-op here — see platformApplyLimits.
func applyRlimits(int, Limits) error { return nil }

// rlimitsSupported reports that this platform cannot enforce resource ceilings,
// so callers can surface the gap rather than assume protection.
func rlimitsSupported() bool { return false }
