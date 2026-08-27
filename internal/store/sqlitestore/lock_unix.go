//go:build !windows

package sqlitestore

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// errLockHeld is what lockFile returns when another process holds the lock. It
// carries no holder detail — that comes from the lock file's contents, since
// the OS reports only THAT a lock is held, never by whom.
var errLockHeld = errors.New("lock is held by another process")

// lockFile takes an exclusive, non-blocking BSD lock.
//
// flock is used rather than fcntl/POSIX record locks deliberately: POSIX locks
// are dropped when ANY descriptor to the file is closed by the process, which
// makes them fragile in a library that cannot see what else the host program
// opens. flock is tied to the open file description instead.
func lockFile(f *os.File) error {
	//nolint:gosec // G115: a file descriptor is a small non-negative int by construction
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return errLockHeld
		}
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	//nolint:gosec // G115: a file descriptor is a small non-negative int by construction
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
