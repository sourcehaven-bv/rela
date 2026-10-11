//go:build !windows

package kvpiles

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock takes an exclusive, non-blocking BSD lock and reports whether it
// got it. flock rather than fcntl: a POSIX record lock is dropped when the
// process closes ANY descriptor of the file, flock is tied to this one.
func tryLock(f *os.File) (bool, error) {
	//nolint:gosec // G115: a file descriptor is a small non-negative int by construction
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlockFile(f *os.File) error {
	//nolint:gosec // G115: a file descriptor is a small non-negative int by construction
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
