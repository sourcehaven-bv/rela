//go:build unix

package cmdexec

import (
	"os/exec"
	"syscall"
)

// platformKillProcessGroup signals the whole group. cmd.Process.Pid is the group
// leader because platformApplyLimits set Setpgid, so negating it targets every
// descendant — including a grandchild that detached from its parent.
func platformKillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Negative pid = "the process group with this id". Ignore the error: the
	// group is usually already gone (the normal case after a clean exit).
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// setPgid puts the child in its own process group so a timeout can take down the
// entire tree. Shared by every unix; the rlimit half is Linux-only.
func setPgid(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
