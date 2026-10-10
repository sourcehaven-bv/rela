package kvpiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// lockWait bounds how long [FileLocker.Lock] waits for another process. A
// pile operation holds the lock for one small file rewrite, so a wait this
// long means a stuck holder, and an error beats hanging the caller.
const lockWait = 10 * time.Second

const (
	// maxPoll caps the backoff between lock attempts.
	maxPoll = 50 * time.Millisecond
	// dirPerm and filePerm match the cache directory's other files: private
	// to the user.
	dirPerm  = 0o750
	filePerm = 0o600
)

// FileLocker is the [Locker] for an on-disk KV: an exclusive advisory lock
// (flock on unix, LockFileEx on windows) on a lock file that every process
// opening the project agrees on. The lock file holds no data and is never
// removed, because removing it would let two processes lock two different
// files of the same name.
type FileLocker struct {
	path string
}

// NewFileLocker returns a locker on the file at path, created on first use.
func NewFileLocker(path string) (*FileLocker, error) {
	if path == "" {
		return nil, errors.New("kvpiles: lock file path must be non-empty")
	}
	return &FileLocker{path: path}, nil
}

// Lock opens the lock file and polls for the lock until it is held, ctx
// ends, or [lockWait] passes. The OS lock belongs to this open file, so two
// FileLockers in one process exclude each other just as two processes do.
func (l *FileLocker) Lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE, filePerm)
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
