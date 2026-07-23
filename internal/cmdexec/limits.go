package cmdexec

import (
	"os/exec"
)

// Limits bound what a single command may CONSUME, complementing [Sandbox],
// which bounds what it may REACH. Both are needed: namespaces isolate access,
// not resource usage — a sandboxed converter can still allocate gigabytes, fork
// a thousand processes, or fill the disk, any of which takes down the server.
//
// Every field is a hard ceiling applied to the child before exec. Zero means
// "unset" for that dimension.
type Limits struct {
	// MaxAddressSpace caps the child's virtual memory (RLIMIT_AS), in bytes.
	// Bounds the "allocate until the host OOMs" case; the converter dies with an
	// allocation failure instead.
	MaxAddressSpace uint64
	// MaxProcesses caps processes/threads for the child's uid (RLIMIT_NPROC).
	// Bounds fork bombs.
	MaxProcesses uint64
	// MaxFileSize caps any single file the child writes (RLIMIT_FSIZE), in
	// bytes. This is a PREVENTIVE disk bound: the write fails at the limit,
	// unlike the output cap, which only rejects an oversize result after it has
	// already been written to disk.
	MaxFileSize uint64
	// MaxCPUSeconds caps CPU time (RLIMIT_CPU). Complements the wall-clock
	// timeout: a process that spins on CPU is killed by the kernel even if the
	// wall-clock kill were to race or the process ignored signals.
	MaxCPUSeconds uint64
}

// DefaultLimits are the bounds applied when a caller does not choose its own.
// They are deliberately generous — a legitimate PDF/DOCX conversion of a large
// document must not trip them — while still being orders of magnitude below
// "takes down the host".
func DefaultLimits() Limits {
	return Limits{
		MaxAddressSpace: 2 << 30, // 2 GiB
		MaxProcesses:    256,
		MaxFileSize:     1 << 30, // 1 GiB
		MaxCPUSeconds:   120,
	}
}

// WithLimits overrides the per-command resource ceilings.
func WithLimits(l Limits) Option { return func(r *Runner) { r.limits = l } }

// applyLimits sets the platform's resource ceilings and process-group placement
// on cmd before it starts. Implemented per-GOOS: rlimits are Linux-only in Go's
// SysProcAttr, while process-group isolation (so a timeout kills the whole tree,
// not just the direct child) applies to every Unix.
//
// Callers must call [killProcessGroup] on timeout to make the group placement
// meaningful.
func applyLimits(cmd *exec.Cmd, l Limits) { platformApplyLimits(cmd, l) }

// killProcessGroup terminates the command's entire process group, so a converter
// that spawned helpers (pandoc → a PDF engine) cannot leave orphans running
// after the timeout. exec.CommandContext only signals the direct child, which a
// detached grandchild survives — verified: without this, a grandchild outlives
// the deadline.
func killProcessGroup(cmd *exec.Cmd) { platformKillProcessGroup(cmd) }
