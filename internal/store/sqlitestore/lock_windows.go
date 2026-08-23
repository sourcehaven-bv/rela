//go:build windows

package sqlitestore

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// errLockHeld is what lockFile returns when another process holds the lock. It
// carries no holder detail — the OS reports only THAT a lock is held, never by
// whom; the identity comes from the lock file's contents.
var errLockHeld = errors.New("lock is held by another process")

// lockFile takes an exclusive, immediately-failing lock on the whole file.
//
// LOCKFILE_FAIL_IMMEDIATELY makes this non-blocking, matching the unix
// LOCK_NB path: Open must refuse a second process, not wait for the first to
// exit.
func lockFile(f *os.File) error {
	// A fresh zero Overlapped is safe ONLY because this call is synchronous and
	// fails immediately. If a blocking or retrying path is ever added, each
	// call needs its own properly-initialized Overlapped with an event handle.
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		// Lock the maximum range: the region is what is locked, and the file
		// grows as the holder record is written.
		^uint32(0), ^uint32(0),
		ol,
	)
	if err != nil {
		// A contended lock surfaces as ERROR_LOCK_VIOLATION on some paths and
		// ERROR_IO_PENDING on others, because LockFileEx is an overlapped-I/O
		// call. Handling only the first would leave errLockHeld dead code on
		// Windows and make "lock held" indistinguishable from "the lock
		// syscall is broken".
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return errLockHeld
		}
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), ol)
}
