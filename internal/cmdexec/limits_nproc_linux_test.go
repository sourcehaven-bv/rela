package cmdexec

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// applyRlimits must leave RLIMIT_NPROC alone (BUG-EA0E8M).
//
// cmdexec used to set RLIMIT_NPROC=256 on every command, reading it as "this
// command may fork 256 times". It is not: the kernel checks RLIMIT_NPROC in
// fork(2) against a count of every process and thread owned by the REAL UID. On
// a runner whose UID was already past 256 — a Playwright suite holding a few
// hundred browser threads — bwrap could not fork, and every `command:` document
// render failed with
//
//	bwrap: Creating new namespace failed: Resource temporarily unavailable
//
// The assertion is made by READING the child's limit after applyRlimits has run,
// rather than by lowering the limit and watching a command fail. Lowering it is
// the obvious approach and does not work: setrlimit applies to the calling
// process, which then has to fork to launch anything, so the test harness's own
// exec is refused before any code under test is reached — with or without the
// fix, which would make the test pin nothing. (Verified on Debian 12: after
// dropping RLIMIT_NPROC to 1, a plain exec.Command("/bin/true") with no rela
// code involved fails with "resource temporarily unavailable".)
//
// Reading the value instead states the property directly: whatever the UID is
// doing, rela does not narrow this child's process ceiling. Restoring
// `set(unix.RLIMIT_NPROC, ...)` to applyRlimits fails this test.
func TestApplyRlimitsLeavesNPROCAlone(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep unavailable")
	}

	// The inherited limit is the baseline: a child starts with the parent's
	// ceiling, so "unchanged" means "still this".
	var want unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NPROC, &want); err != nil {
		t.Fatalf("read this process's RLIMIT_NPROC: %v", err)
	}

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// The call under test, with the same Limits production uses.
	if err := applyRlimits(cmd.Process.Pid, DefaultLimits()); err != nil {
		t.Fatalf("applyRlimits: %v", err)
	}

	var got unix.Rlimit
	if err := unix.Prlimit(cmd.Process.Pid, unix.RLIMIT_NPROC, nil, &got); err != nil {
		t.Fatalf("read the child's RLIMIT_NPROC: %v", err)
	}

	if got.Cur != want.Cur || got.Max != want.Max {
		t.Errorf("applyRlimits changed the child's RLIMIT_NPROC to {cur=%d max=%d}, want the "+
			"inherited {cur=%d max=%d}.\nRLIMIT_NPROC is per-UID, not per-command: a ceiling set "+
			"here is charged against every process the user owns, so it denies this command's "+
			"fork for load that has nothing to do with it.", got.Cur, got.Max, want.Cur, want.Max)
	}
}

// The dimensions that ARE per-process must still be applied — otherwise the
// removal above could be "fixed" by disabling applyRlimits altogether and this
// package would lose its memory, file-size and CPU ceilings silently.
func TestApplyRlimitsStillAppliesPerProcessCeilings(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep unavailable")
	}

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	lim := DefaultLimits()
	if err := applyRlimits(cmd.Process.Pid, lim); err != nil {
		t.Fatalf("applyRlimits: %v", err)
	}

	for _, tc := range []struct {
		name     string
		resource int
		want     uint64
	}{
		{"RLIMIT_AS", unix.RLIMIT_AS, lim.MaxAddressSpace},
		{"RLIMIT_FSIZE", unix.RLIMIT_FSIZE, lim.MaxFileSize},
		{"RLIMIT_CPU", unix.RLIMIT_CPU, lim.MaxCPUSeconds},
	} {
		var got unix.Rlimit
		if err := unix.Prlimit(cmd.Process.Pid, tc.resource, nil, &got); err != nil {
			t.Errorf("%s: read child limit: %v", tc.name, err)
			continue
		}
		if got.Cur != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, got.Cur, tc.want)
		}
	}
}
