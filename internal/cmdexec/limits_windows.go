package cmdexec

import "os/exec"

// platformApplyLimits is a no-op on Windows. Job objects could bound memory and
// process count, but wiring them needs hand-rolled syscall work, and Windows
// already has no sandbox mechanism here ([newPlatformSandbox] returns
// unavailable), so a command only runs at all when the operator has explicitly
// disabled confinement. Adding partial resource bounds behind that opt-out would
// suggest a protection that is not there.
func platformApplyLimits(*exec.Cmd, Limits) {}

// platformKillProcessGroup is a no-op: exec.CommandContext already terminates
// the child, and Windows process-tree kill needs job objects (see above).
func platformKillProcessGroup(*exec.Cmd) {}

// applyRlimits is a no-op on Windows.
func applyRlimits(int, Limits) error { return nil }

// rlimitsSupported reports that this platform cannot enforce resource ceilings.
func rlimitsSupported() bool { return false }
