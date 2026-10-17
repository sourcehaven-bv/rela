package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The realtime worker is the one host for realtime background work: the
// automatic close and archive every minute, retention every hour, and a
// retention run as soon as an entity delete commits. Only the process that
// holds the worker lock runs it, so several rela-server processes against
// one database do the work once.
const (
	realtimeLifecycleInterval = time.Minute
	realtimeRetentionInterval = time.Hour
	// realtimeLeaderRetry is how long a process that could not take the
	// worker lock waits before it tries again.
	realtimeLeaderRetry = 30 * time.Second
	// realtimeWorkerLockFile is the worker lock of the file backend, in the
	// project's cache directory.
	realtimeWorkerLockFile = "realtime-worker.lock"
	// realtimeWorkerLockKey names the worker lock of a database backend.
	realtimeWorkerLockKey = "realtime-worker"
)

// tombstoneConfirmer connects the channel guard to the post-commit delete
// hooks. The guard tombstones an entity's channels before the delete's
// transaction and files the batch as pending, so retention cannot erase it
// while the delete may still fail. Once the delete commits, the hook
// confirms the batch and wakes the worker, which erases it when due.
//
// A confirmation lost to a crash is settled by retention: a pending batch
// whose entity is gone is confirmed, and one whose entity still exists
// after [realtimelimits.OrphanGrace] is put back.
type tombstoneConfirmer struct {
	store channels.Store
	wake  chan struct{}

	mu      sync.Mutex
	pending map[string][]channels.TombstoneHandle
}

// newTombstoneConfirmer returns the confirmer over cs.
//
// Nil: cs is rejected; a confirmer without a store could confirm nothing.
func newTombstoneConfirmer(cs channels.Store) (*tombstoneConfirmer, error) {
	if cs == nil {
		return nil, errors.New("channels: tombstone confirmer needs a channel store")
	}
	return &tombstoneConfirmer{
		store: cs, wake: make(chan struct{}, 1), pending: map[string][]channels.TombstoneHandle{},
	}, nil
}

// add remembers h until its delete commits or is undone.
func (c *tombstoneConfirmer) add(h channels.TombstoneHandle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := h.Anchor().ID
	c.pending[id] = append(c.pending[id], h)
}

// drop forgets h: its delete failed and the batch went back.
func (c *tombstoneConfirmer) drop(h channels.TombstoneHandle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := h.Anchor().ID
	rest := c.pending[id][:0]
	for _, p := range c.pending[id] {
		if p != h {
			rest = append(rest, p)
		}
	}
	if len(rest) == 0 {
		delete(c.pending, id)
		return
	}
	c.pending[id] = rest
}

// confirm confirms the pending batches of id whose delete committed: every
// one after a whole-entity delete (whole), only the face's batches after a
// face delete. It wakes the worker when it confirmed any.
func (c *tombstoneConfirmer) confirm(ctx context.Context, id string, face entity.Face, whole bool) error {
	c.mu.Lock()
	var mine, rest []channels.TombstoneHandle
	for _, h := range c.pending[id] {
		if whole || h.Anchor().Face == face {
			mine = append(mine, h)
		} else {
			rest = append(rest, h)
		}
	}
	if len(rest) == 0 {
		delete(c.pending, id)
	} else {
		c.pending[id] = rest
	}
	c.mu.Unlock()
	if len(mine) == 0 {
		return nil
	}
	var errs []error
	for _, h := range mine {
		if err := c.store.ConfirmTombstone(ctx, h); err != nil && !errors.Is(err, channels.ErrNotFound) {
			errs = append(errs, fmt.Errorf("channels: confirm %s: %w", h, err))
		}
	}
	c.ping()
	return errors.Join(errs...)
}

// ping wakes the worker without blocking; a wake already queued covers it.
func (c *tombstoneConfirmer) ping() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// anchorExistence is the [realtime.AnchorExistence] over the raw store. It
// reads headers, never bodies, with no ACL: the sweep acts for the system
// and shows nothing it reads. A soft-deleted family counts as existing, so
// its channels survive until the soft delete is purged.
type anchorExistence struct {
	st store.Store
}

// Faces implements [realtime.AnchorExistence] with one header read for ids
// and, on a store that soft-deletes, one read of the marked families.
func (a anchorExistence) Faces(ctx context.Context, ids []string) (map[string][]entity.Face, error) {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := map[string][]entity.Face{}
	q := store.EntityQuery{IDs: ids, Faces: store.AllFaces()}
	for h, err := range store.ListEntityHeaders(ctx, a.st, q) {
		if err != nil {
			return nil, err
		}
		if want[h.ID] {
			out[h.ID] = append(out[h.ID], h.Face)
		}
	}
	sd, ok := a.st.(store.SoftDeleteProvider)
	if !ok {
		return out, nil
	}
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range marked {
		if !want[m.ID] {
			continue
		}
		for _, e := range m.Entities {
			out[m.ID] = append(out[m.ID], e.Face)
		}
	}
	return out, nil
}

// leaderLock takes the worker lock, blocking until it is held or ctx ends,
// and returns its release.
type leaderLock func(ctx context.Context) (func(), error)

// realtimeLeaderLock returns the worker lock for st: the database's own lock
// on a backend that runs several processes, a lock file in the cache
// directory on disk, and a lock that is always free in memory.
func realtimeLeaderLock(fs storage.FS, cacheDir string, st store.Store) (leaderLock, error) {
	if _, ok := st.(lock.KeyedLockBackend); ok {
		l := lock.For(st)
		return func(ctx context.Context) (func(), error) { return l.Acquire(ctx, realtimeWorkerLockKey) }, nil
	}
	if _, mem := fs.(*storage.MemFS); mem || fs == nil || cacheDir == "" {
		return filelock.ProcessPrivate{}.Lock, nil
	}
	l, err := filelock.New(filepath.Join(cacheDir, realtimeWorkerLockFile))
	if err != nil {
		return nil, fmt.Errorf("realtime worker: %w", err)
	}
	return l.Lock, nil
}

// realtimeWorker runs the [realtime.Sweeper] while this process holds the
// worker lock. A host starts it with [RunRealtimeWorker] and stops it by
// cancelling the context it passed; [Services.Close] stops it too.
type realtimeWorker struct {
	sweeper *realtime.Sweeper
	lock    leaderLock
	wake    <-chan struct{}

	mu      sync.Mutex
	running chan struct{}
	closed  chan struct{}
	once    sync.Once
}

// newRealtimeWorker returns the worker over its parts.
//
// Nil: every argument is rejected.
func newRealtimeWorker(sweeper *realtime.Sweeper, l leaderLock, wake <-chan struct{}) (*realtimeWorker, error) {
	if sweeper == nil || l == nil || wake == nil {
		return nil, errors.New("realtime worker: sweeper, lock and wake are required")
	}
	return &realtimeWorker{sweeper: sweeper, lock: l, wake: wake, closed: make(chan struct{})}, nil
}

// buildRealtimeWorker builds the worker of an assembled Services, or
// returns nil when the process is not long-running or has no channels.
//
// Nil: refs.channels and refs.channelTombs are accepted (no worker).
func buildRealtimeWorker(base *SharedBase, st store.Store, refs refHolders) (*realtimeWorker, error) {
	if !base.opts.backgroundAutomationJobs || refs.channels == nil || refs.channelTombs == nil ||
		refs.realtimeLocal == nil {
		return nil, nil //nolint:nilnil // not a long-running process: no worker
	}
	sweeper, err := realtime.NewSweeper(realtime.SweepDeps{
		Store: refs.channels, Config: realtimeConfigSource{h: base.realtime},
		Anchors: anchorExistence{st: st}, Marker: refs.realtimeLocal, Clock: time.Now,
	})
	if err != nil {
		return nil, err
	}
	cacheDir := ""
	if base.cfg.Paths != nil {
		cacheDir = base.cfg.Paths.CacheDir
	}
	l, err := realtimeLeaderLock(base.cfg.FS, cacheDir, st)
	if err != nil {
		return nil, err
	}
	return newRealtimeWorker(sweeper, l, refs.channelTombs.wake)
}

// RunRealtimeWorker runs the realtime worker of s until ctx is cancelled or
// s closes, and returns a channel closed once it has stopped. Only
// long-running hosts call it, next to [WatchLeases]. A Services without a
// worker, or one whose worker already runs, returns a closed channel.
func RunRealtimeWorker(ctx context.Context, s *Services) <-chan struct{} {
	done := make(chan struct{})
	w := s.realtimeWorker
	if w == nil {
		close(done)
		return done
	}
	w.mu.Lock()
	if w.running != nil {
		w.mu.Unlock()
		slog.Warn("realtime worker: already running")
		close(done)
		return done
	}
	w.running = done
	w.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer close(done)
		defer cancel()
		go func() {
			select {
			case <-w.closed:
				cancel()
			case <-ctx.Done():
			}
		}()
		w.run(ctx)
		w.mu.Lock()
		w.running = nil
		w.mu.Unlock()
	}()
	return done
}

// stop ends a running worker and waits for it. Safe to call more than once.
func (w *realtimeWorker) stop() {
	w.once.Do(func() { close(w.closed) })
	w.mu.Lock()
	done := w.running
	w.mu.Unlock()
	if done != nil {
		<-done
	}
}

// run takes the worker lock and leads until ctx ends. A failed attempt is
// retried after realtimeLeaderRetry.
func (w *realtimeWorker) run(ctx context.Context) {
	for ctx.Err() == nil {
		release, err := w.lock(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.Debug("realtime worker: lock not taken; retrying", "error", err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(realtimeLeaderRetry):
			}
			continue
		}
		w.lead(ctx)
		release()
	}
}

// lead runs the sweeps on their intervals, and retention on every wake,
// until ctx ends.
func (w *realtimeWorker) lead(ctx context.Context) {
	w.retention(ctx)
	w.lifecycle(ctx)
	lifecycle := time.NewTicker(realtimeLifecycleInterval)
	defer lifecycle.Stop()
	retention := time.NewTicker(realtimeRetentionInterval)
	defer retention.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-lifecycle.C:
			w.lifecycle(ctx)
		case <-retention.C:
			w.retention(ctx)
		case <-w.wake:
			w.retention(ctx)
		}
	}
}

func (w *realtimeWorker) lifecycle(ctx context.Context) {
	rep, err := w.sweeper.RunLifecycle(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Warn("realtime: lifecycle sweep failed", "error", err)
	}
	if rep.Closed+rep.Archived > 0 {
		slog.Info("realtime: lifecycle sweep", "closed", rep.Closed, "archived", rep.Archived)
	}
	for _, p := range rep.StalePending {
		slog.Warn("realtime: action pending past the release time; a moderator can release it",
			"channel", p.Key.String(), "seq", p.Seq, "action", p.Name, "pending_at", p.PendingAt)
	}
}

func (w *realtimeWorker) retention(ctx context.Context) {
	rep, err := w.sweeper.RunRetention(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Warn("realtime: retention sweep failed", "error", err)
	}
	if rep != (realtime.RetentionReport{}) {
		slog.Info("realtime: retention sweep", "purged", rep.Purged, "confirmed", rep.Confirmed,
			"restored", rep.Restored, "detached", rep.Detached, "types_cleared", rep.TypesCleared)
	}
}
