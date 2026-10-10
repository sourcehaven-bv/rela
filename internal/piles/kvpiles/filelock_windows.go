//go:build windows

package kvpiles

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes an exclusive, immediately-failing lock on the whole file and
// reports whether it got it.
func tryLock(f *os.File) (bool, error) {
	// A fresh zero Overlapped is safe only because the call is synchronous
	// and fails immediately.
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, ^uint32(0), ^uint32(0), ol)
	// A contended lock surfaces as ERROR_LOCK_VIOLATION on some paths and
	// ERROR_IO_PENDING on others, because LockFileEx is an overlapped call.
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return false, nil
	}
	return err == nil, err
}

func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), ol)
}
