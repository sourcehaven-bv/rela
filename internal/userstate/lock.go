package userstate

import (
	"fmt"

	"github.com/rogpeppe/go-internal/lockedfile"
)

// lockPath takes an exclusive OS-level advisory lock on path,
// creating the lock file (not the path itself) if needed. The
// returned unlock function must be called exactly once. lockPath
// blocks until the lock is acquired or the OS returns an error.
//
// lockedfile uses flock(2) on Unix and LockFileEx on Windows; both
// are advisory-exclusive and safe to use across independent rela
// processes on the same machine. Cross-machine sync (NFS, CIFS) is
// not supported — the guarantees weaken under those filesystems,
// but rela's user-state directory isn't expected to live there.
func lockPath(path string) (unlock func() error, err error) {
	mu := lockedfile.MutexAt(path)
	unlocker, err := mu.Lock()
	if err != nil {
		return nil, fmt.Errorf("userstate: acquire lock %s: %w", path, err)
	}
	return func() error {
		unlocker()
		return nil
	}, nil
}
