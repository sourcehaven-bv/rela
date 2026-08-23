package sqlitestore

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// processLock is the single-writer guard: an exclusive advisory lock on a
// sidecar file next to the database.
//
// # Why a sidecar and not the database file
//
// SQLite already holds POSIX advisory locks on the database file's inode, and
// on Linux/macOS closing ANY file descriptor to a file drops all of that
// process's POSIX locks on it. Taking our own lock on the same inode risks
// silently clobbering SQLite's. The sidecar keeps the two lock regimes on
// different files, where they cannot interfere.
//
// # Why this exists at all
//
// rela enforces `unique:` with an untransacted scan in entitymanager
// (ListEntities, then write). With a single process the in-process write mutex
// keeps that window narrow; with two processes over one database file there is
// no backstop at all, and the violation is silent. pgstore closes this with a
// partial unique index synthesized from the metamodel; this backend does not
// have that, so it refuses the second process instead (DEC-LFSYNY).
type processLock struct {
	path string
	file *os.File
}

// lockHolder is what a lock file contains. Advisory only: a PID can be reused,
// so this is for a human reading an error message, never for a correctness
// decision.
type lockHolder struct {
	PID      int       `json:"pid"`
	Hostname string    `json:"hostname"`
	Since    time.Time `json:"since"`
}

// acquireProcessLock takes the exclusive lock for dbPath, or reports who holds
// it.
//
// The holder identity is written BEFORE the lock is taken, so a waiting process
// can read it: after the lock is held the file is ours to describe, but a
// process that fails to acquire needs the previous holder's details, which only
// exist if they were written first.
func acquireProcessLock(dbPath string) (*processLock, error) {
	path := dbPath + ".lock"

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: open lock file %s: %w", path, err)
	}

	if err := lockFile(file); err != nil {
		holder := readHolder(path)
		_ = file.Close()
		return nil, fmt.Errorf(
			"sqlitestore: another process is using %s%s; "+
				"this backend is single-process by design (see DEC-LFSYNY). "+
				"Stop the other process, or use the PostgreSQL build for a "+
				"multi-process deployment: %w",
			dbPath, holder, err)
	}

	if err := writeHolder(file); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, err
	}
	return &processLock{path: path, file: file}, nil
}

// release drops the lock and closes the file. The lock file itself is left in
// place: removing it races another process that has already opened it but not
// yet locked it, which would let two processes hold "the lock" on two different
// inodes. An empty stale file is harmless.
//
// Nil: safe to call on a nil *processLock, so Close paths need no guard.
func (l *processLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	// Truncate rather than delete, so a later reader does not report a holder
	// that has already exited.
	_ = l.file.Truncate(0)
	if err := unlockFile(l.file); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("sqlitestore: release lock: %w", err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("sqlitestore: close lock file: %w", err)
	}
	l.file = nil
	return nil
}

func writeHolder(f *os.File) error {
	host, _ := os.Hostname() // best effort; an empty hostname is not fatal
	b, err := json.Marshal(lockHolder{PID: os.Getpid(), Hostname: host, Since: time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("sqlitestore: encode lock holder: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("sqlitestore: truncate lock file: %w", err)
	}
	if _, err := f.WriteAt(b, 0); err != nil {
		return fmt.Errorf("sqlitestore: write lock holder: %w", err)
	}
	return f.Sync()
}

// readHolder renders the recorded holder for an error message, or "" when the
// file is empty, unreadable or malformed — all of which are normal races, and
// none of which should replace the real "already locked" error.
func readHolder(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	var h lockHolder
	if err := json.Unmarshal(b, &h); err != nil {
		return ""
	}
	return fmt.Sprintf(" (held by pid %d on %s since %s)",
		h.PID, h.Hostname, h.Since.Format(time.RFC3339))
}
