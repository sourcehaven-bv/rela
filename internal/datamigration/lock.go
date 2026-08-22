package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrLockHeld is returned by [MigrationLock.TryAcquire] when another
// migration or GC run is active against the same store. Callers fail fast
// (CLI) or skip the cycle (sweep/gate) — nothing in this package ever blocks
// waiting for the lock.
var ErrLockHeld = errors.New("datamigration: another migration or GC run is active")

// MigrationLock serializes the writers of the migration marker/ledger and
// the destructive GC path across processes sharing one store (TKT-CPCBR7).
// It is an operational mutual-exclusion primitive like the pgstore sweep's
// advisory lock — NOT a transaction seam (that stays store.Store.Tx,
// DEC-8UIL0) and not a general lock service.
//
// Consumer-side interface: implementations are selected per backend by
// [LockFor], the way state.KV backends are.
type MigrationLock interface {
	// TryAcquire returns a release func on success and ErrLockHeld when the
	// lock is taken. It never blocks beyond one round-trip; release is
	// idempotent.
	TryAcquire(ctx context.Context) (release func(), err error)
}

// storeLocker is the optional store capability behind the postgres
// implementation, discovered by type-assert like store.Formatter /
// HistoryReader. The signature matches (*pgstore.Store).TryMigrationLock
// exactly so no adapter package is needed; ok=false means held.
type storeLocker interface {
	TryMigrationLock(ctx context.Context) (release func(), ok bool, err error)
}

// LockFor selects the migration lock for a store: a store-provided
// cross-process lock when the backend has one (pgstore's schema-scoped
// advisory lock), else a lock file under cacheDir (fsstore — single-machine
// by nature), else a process-local lock (memory backend, tests). Mirrors how
// the state.KV backend is chosen: backend capability wins, filesystem
// fallback.
func LockFor(st store.Store, cacheDir string) MigrationLock {
	if sl, ok := st.(storeLocker); ok {
		return &storeLock{sl: sl}
	}
	if cacheDir != "" {
		return newFSLock(cacheDir)
	}
	return NewProcessLock()
}

// ---- store-backed (postgres) ----

type storeLock struct{ sl storeLocker }

func (l *storeLock) TryAcquire(ctx context.Context) (func(), error) {
	release, ok, err := l.sl.TryMigrationLock(ctx)
	if err != nil {
		return nil, fmt.Errorf("datamigration: acquire store migration lock: %w", err)
	}
	if !ok {
		return nil, ErrLockHeld
	}
	return once(release), nil
}

// ---- process-local ----

// ProcessLock is the in-process implementation: mutual exclusion between
// goroutines of one process only (memory backend, tests). It is also
// embedded in the fs lock so two goroutines in one process cannot both pass
// the pid-file check (their shared pid reads as "held by a live process —
// me" for both).
type ProcessLock struct {
	mu sync.Mutex
}

// NewProcessLock returns a process-local MigrationLock.
func NewProcessLock() *ProcessLock { return &ProcessLock{} }

// TryAcquire implements [MigrationLock].
func (l *ProcessLock) TryAcquire(context.Context) (func(), error) {
	if !l.mu.TryLock() {
		return nil, ErrLockHeld
	}
	return once(l.mu.Unlock), nil
}

// ---- filesystem lock file ----

// lockFileName is the lock file under the project cache dir (.rela/).
const lockFileName = "migration.lock"

// lockFilePayload is what the holder writes into the lock file: enough to
// decide staleness (pid liveness on this host) and to name the holder in
// logs. Parsed defensively — an unparseable file counts as stale, because a
// file we cannot attribute to a live process must not wedge migrations
// forever.
type lockFilePayload struct {
	PID        int       `json:"pid"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// fsLock is the filesystem implementation: an O_CREATE|O_EXCL lock file
// under `.rela/`, composed with a ProcessLock so in-process callers exclude
// each other too. Single-machine by design — matching fsstore, whose write
// serialization is likewise in-process; a project directory shared over a
// network filesystem gets no cross-host guarantee (and never did).
type fsLock struct {
	path  string
	local ProcessLock
}

func newFSLock(cacheDir string) *fsLock {
	return &fsLock{path: filepath.Join(cacheDir, lockFileName)}
}

// TryAcquire implements [MigrationLock]. A lock file whose recorded pid is
// no longer alive (crashed run) is broken with a warning and acquisition is
// retried once.
func (l *fsLock) TryAcquire(ctx context.Context) (func(), error) {
	if !l.local.mu.TryLock() {
		return nil, ErrLockHeld
	}
	releaseFile, err := l.acquireFile(ctx)
	if err != nil {
		l.local.mu.Unlock()
		return nil, err
	}
	return once(func() {
		releaseFile()
		l.local.mu.Unlock()
	}), nil
}

func (l *fsLock) acquireFile(ctx context.Context) (func(), error) {
	// One stale-break retry, never more: a second failure means a live
	// holder raced us to re-create the file, which is contention, not
	// staleness.
	for range 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
			return nil, fmt.Errorf("datamigration: create lock dir: %w", err)
		}
		payload, _ := json.Marshal(lockFilePayload{PID: os.Getpid(), AcquiredAt: time.Now().UTC()})
		f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, werr := f.Write(payload)
			cerr := f.Close()
			if werr != nil || cerr != nil {
				_ = os.Remove(l.path)
				return nil, fmt.Errorf("datamigration: write lock file: %w", errors.Join(werr, cerr))
			}
			return func() { _ = os.Remove(l.path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("datamigration: create lock file: %w", err)
		}
		if !l.breakIfStale() {
			return nil, ErrLockHeld
		}
	}
	return nil, ErrLockHeld
}

// breakIfStale removes the lock file when its holder is provably gone on
// this host, returning true when a retry is worthwhile. A file naming a
// LIVE pid is always honored — staleness can only be concluded from a dead
// or unattributable holder, never from age alone (a long migration is not a
// crash).
func (l *fsLock) breakIfStale() bool {
	data, err := os.ReadFile(l.path)
	if err != nil {
		// Raced with the holder's release: the file is already gone.
		return errors.Is(err, os.ErrNotExist)
	}
	var p lockFilePayload
	if err := json.Unmarshal(data, &p); err != nil || p.PID <= 0 {
		slog.Warn("datamigration: breaking unparseable migration lock file", "path", l.path)
		_ = os.Remove(l.path)
		return true
	}
	if pidAlive(p.PID) {
		return false
	}
	slog.Warn("datamigration: breaking stale migration lock from dead process",
		"path", l.path, "pid", p.PID, "acquired_at", p.AcquiredAt)
	_ = os.Remove(l.path)
	return true
}

// pidAlive reports whether a process with the given pid exists on this
// host (signal 0 probe; EPERM still means "exists").
func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// once wraps a release func so double-release is safe — every acquire site
// defers release, and error paths must be free to call it early too.
func once(release func()) func() {
	var o sync.Once
	return func() { o.Do(release) }
}
