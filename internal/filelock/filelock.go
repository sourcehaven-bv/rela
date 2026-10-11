// Package filelock is a cross-process lock on a lock file: an exclusive
// advisory lock (flock on unix, LockFileEx on windows) that every process
// opening the same file agrees on.
//
// The single-process storage tiers keep small JSON documents in the
// cache-directory state KV (piles, leases). Several processes can open one
// project at once (the desktop app, `rela mcp`, CLI calls, `rela scheduler`),
// and a mutex only excludes callers inside one process. A [Locker] on a lock
// file next to the document excludes the other processes too.
//
// # The lock file
//
// The lock file holds no data. It is opened without truncation, so taking
// the lock never changes the file, and it is never removed: removing it would
// let two processes lock two different files of the same name. It is created
// readable and writable by the owner only, and on unix it is never opened
// through a symlink, so a planted link cannot make rela create or lock a file
// elsewhere.
package filelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// lockWait bounds how long [Locker.Lock] waits for another process. A
// caller holds the lock for one small file rewrite, so a wait this long means
// a stuck holder, and an error beats hanging the caller.
const lockWait = 10 * time.Second

const (
	// maxPoll caps the backoff between lock attempts.
	maxPoll = 50 * time.Millisecond
	// dirPerm and filePerm match the cache directory's other files: private
	// to the user.
	dirPerm  = 0o750
	filePerm = 0o600
)

// Locker is an exclusive lock on one lock file. It is safe for concurrent
// use: every [Locker.Lock] opens the file afresh, and the OS lock belongs to
// that open file.
type Locker struct {
	path string
}

// New returns a locker on the file at path. The file and its directory are
// created on first use.
func New(path string) (*Locker, error) {
	if path == "" {
		return nil, errors.New("filelock: lock file path must be non-empty")
	}
	return &Locker{path: path}, nil
}

// Lock opens the lock file and polls for the lock until it is held, ctx
// ends, or [lockWait] passes. It returns the release func. The OS lock
// belongs to this open file, so two Lockers in one process exclude each other
// just as two processes do.
func (l *Locker) Lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	// No O_TRUNC: the lock file is never written, so opening it must not
	// change it either.
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE|oNoFollow, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	delay := time.Millisecond
	for {
		held, err := tryLock(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock file: %w", err)
		}
		if held {
			return func() {
				_ = unlockFile(f)
				_ = f.Close()
			}, nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, fmt.Errorf("lock file held by another process: %w", ctx.Err())
		case <-timer.C:
		}
		delay = min(2*delay, maxPoll)
	}
}
