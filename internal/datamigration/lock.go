package datamigration

import (
	"bytes"
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
// advisory lock), else a lock file under cacheDir, else a process-local
// lock (tests, cacheDir-less contexts). The cacheDir branch applies to the
// memory backend too, ON PURPOSE: what the lock guards is the marker/ledger
// in state.KV, and for every non-postgres backend that state lives under
// `.rela/` (FSKV) — shared across processes — so the lock must be a file
// beside it. Mirrors how the state.KV backend itself is chosen: backend
// capability wins, filesystem fallback.
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
// goroutines of one process only (tests, contexts with no cache dir). It is
// also embedded in the fs lock so two goroutines sharing one fsLock VALUE
// cannot both pass the pid-file check (their shared pid reads as "held by a
// live process — me" for both).
//
// Releases are generation-guarded, not just idempotent: a stale release
// func kept from an earlier acquisition can never unlock a later holder.
type ProcessLock struct {
	mu   sync.Mutex
	held bool
	gen  uint64
}

// NewProcessLock returns a process-local MigrationLock.
func NewProcessLock() *ProcessLock { return &ProcessLock{} }

// TryAcquire implements [MigrationLock].
func (l *ProcessLock) TryAcquire(context.Context) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return nil, ErrLockHeld
	}
	l.held = true
	l.gen++
	mine := l.gen
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.held && l.gen == mine {
			l.held = false
		}
	}, nil
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
// retried once. Nil: never returns a nil release on nil error.
func (l *fsLock) TryAcquire(ctx context.Context) (func(), error) {
	releaseLocal, err := l.local.TryAcquire(ctx)
	if err != nil {
		return nil, err
	}
	releaseFile, err := l.acquireFile(ctx)
	if err != nil {
		releaseLocal()
		return nil, err
	}
	return once(func() {
		releaseFile()
		releaseLocal()
	}), nil
}

func (l *fsLock) acquireFile(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return nil, fmt.Errorf("datamigration: create lock dir: %w", err)
	}
	// One stale-break retry, never more: a second failure means a live
	// holder raced us to re-create the file, which is contention, not
	// staleness.
	for range 2 {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("datamigration: acquire migration lock: %w", err)
		}
		// Serialize creation with stale inspection too. O_EXCL publishes the
		// pathname before Write fills its payload; without the break mutex a
		// contender can observe that empty window, call it unparseable/stale,
		// and delete a live holder's new file.
		bf, err := os.OpenFile(l.breakPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				l.clearAbandonedBreak()
				return nil, ErrLockHeld
			}
			return nil, fmt.Errorf("datamigration: create lock-break marker: %w", err)
		}
		_ = bf.Close()
		payload, _ := json.Marshal(lockFilePayload{PID: os.Getpid(), AcquiredAt: time.Now().UTC()})
		f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, werr := f.Write(payload)
			cerr := f.Close()
			if werr != nil || cerr != nil {
				l.removeIfOurs(payload)
				_ = os.Remove(l.breakPath())
				return nil, fmt.Errorf("datamigration: write lock file: %w", errors.Join(werr, cerr))
			}
			_ = os.Remove(l.breakPath())
			// Release removes the file ONLY while it still holds our own
			// payload: if an operator hand-removed the lock and another
			// process acquired meanwhile, an unconditional remove would
			// delete the new holder's lock.
			return func() { l.removeIfOurs(payload) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			_ = os.Remove(l.breakPath())
			return nil, fmt.Errorf("datamigration: create lock file: %w", err)
		}
		present, stale := l.staleState()
		if !stale {
			_ = os.Remove(l.breakPath())
			return nil, ErrLockHeld
		}
		if present {
			slog.Warn("datamigration: breaking stale migration lock", "path", l.path)
			_ = os.Remove(l.path)
		}
		_ = os.Remove(l.breakPath())
	}
	return nil, ErrLockHeld
}

// removeIfOurs deletes the lock file only when its content is exactly the
// payload this acquisition wrote.
func (l *fsLock) removeIfOurs(payload []byte) {
	current, err := os.ReadFile(l.path)
	if err == nil && bytes.Equal(current, payload) {
		_ = os.Remove(l.path)
	}
}

// breakPath is the break-mutex file: the exclusive right to REMOVE a stale
// lock file. Without it, two processes that both judged the file stale could
// interleave so that the second os.Remove deletes the first's freshly
// re-created lock — the classic read-decide-remove TOCTOU. Only the process
// that O_EXCL-creates the break file may remove the lock, and it re-verifies
// staleness while holding it, so a fresh lock can never be removed.
func (l *fsLock) breakPath() string { return l.path + ".break" }

// breakStaleWindow is how old an abandoned break file must be before it is
// itself removed. Legitimate holds last microseconds; a breaker that crashed
// mid-break is the only way one persists.
const breakStaleWindow = 30 * time.Second

// A file naming a LIVE pid is always honored — staleness can only be
// concluded from a dead or unattributable holder, never from age alone (a
// long migration is not a crash). KNOWN LIMIT: a crashed holder's pid recycled
// by an unrelated process reads as alive and wedges the lock; the operator
// remedy is removing .rela/migration.lock by hand (documented in the guide).
// staleState reports whether the lock file is present, and whether its holder
// is stale. A missing file is reported as stale-but-absent, which lets a caller
// distinguish "nothing to break, just retry" from "a dead holder's file is
// sitting there and must be removed".
func (l *fsLock) staleState() (present, stale bool) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return false, errors.Is(err, os.ErrNotExist)
	}
	var p lockFilePayload
	if err := json.Unmarshal(data, &p); err != nil || p.PID <= 0 {
		return true, true
	}
	return true, !pidAlive(p.PID)
}

// clearAbandonedBreak removes a break file left behind by a crashed breaker.
func (l *fsLock) clearAbandonedBreak() {
	info, err := os.Stat(l.breakPath())
	if err == nil && time.Since(info.ModTime()) > breakStaleWindow {
		slog.Warn("datamigration: removing abandoned lock-break marker", "path", l.breakPath())
		_ = os.Remove(l.breakPath())
	}
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
