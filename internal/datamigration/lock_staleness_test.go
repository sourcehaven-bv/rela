package datamigration

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// writeLockPayload seeds the lock file with a holder record naming pid.
func writeLockPayload(t *testing.T, l *fsLock, pid int) {
	t.Helper()
	data, err := json.Marshal(lockFilePayload{PID: pid, AcquiredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// staleState must distinguish "no lock file" from "a stale holder's file is
// sitting there", because only the latter may be REMOVED.
//
// The two were conflated: isStale reports a missing file as stale, which is
// right for the pre-check (whose only question is whether a retry is
// worthwhile) but wrong as a license to remove. Under the break mutex the old
// code re-read the path, saw "missing" during the gap between another breaker
// removing the stale file and the winner creating its own, and then ran
// os.Remove anyway — deleting the winner's LIVE lock if the create landed
// first. Both callers then believed they held the lock, which is exactly the
// read-decide-remove TOCTOU the break mutex exists to close.
//
// Pins the distinction rather than the race: the window is a few syscalls
// wide, so a timing test observes it only under heavy parallel load (it
// surfaced in `just coverage-check`, roughly one run in three) and cannot be
// staged deterministically without a hook inside breakIfStale.
func TestStaleState_DistinguishesMissingFromStalePresent(t *testing.T) {
	t.Run("missing is retry-worthy but not present", func(t *testing.T) {
		l := newFSLock(t.TempDir())
		present, stale := l.staleState()
		if present {
			t.Errorf("missing lock reported present; it must never be removed")
		}
		if !stale {
			t.Errorf("missing lock must still be retry-worthy")
		}
	})

	t.Run("a dead holder's file is present and stale", func(t *testing.T) {
		l := newFSLock(t.TempDir())
		writeLockPayload(t, l, deadPID(t))
		present, stale := l.staleState()
		if !present || !stale {
			t.Errorf("dead-holder lock: present=%v stale=%v, want true/true", present, stale)
		}
	})

	t.Run("a live holder's file is present and not stale", func(t *testing.T) {
		l := newFSLock(t.TempDir())
		writeLockPayload(t, l, os.Getpid())
		present, stale := l.staleState()
		if !present || stale {
			t.Errorf("live-holder lock: present=%v stale=%v, want true/false", present, stale)
		}
	})
}
