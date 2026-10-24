package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The realtime worker is the one host for realtime background work: the
// automatic close and archive and a poll for due purges every minute, full
// retention every hour, a purge poll as soon as an entity delete commits in
// this process, and the triggers (a round every [realtime.TriggerTick]
// while entities changed, a reconcile every [realtime.TriggerReconcile]),
// and the session timers (a round at the due times it learns, at least
// once a minute; see [realtime.TimerSchedule]).
//
// With several rela-server processes on one database, one leads: it holds
// the worker lock for as long as it runs, and does all of this work. The
// others try to take the lock every realtimeTakeoverInterval and do
// nothing else, so when the leader stops another takes over. A delete in
// another process is purged by the leader's next minute poll. Every
// process sees every entity change (the fsstore watcher, pg NOTIFY); only
// the leader notes them for the triggers.
//
// On postgres the lock is a session lock on one pinned connection, which
// the database frees when it drops the session. The leader checks the
// session every realtimeTakeoverInterval and stops leading when the check
// fails, so two leaders overlap for at most that long. In that window a
// trigger may post twice: posts are at least once.
const (
	realtimeLifecycleInterval = time.Minute
	realtimeRetentionInterval = time.Hour
	// realtimeTakeoverInterval is how often a process that does not lead
	// tries to take the worker lock.
	realtimeTakeoverInterval = 10 * time.Second
	// realtimeWorkerLockFile is the worker lock of the file backend, in the
	// project's cache directory.
	realtimeWorkerLockFile = "realtime-worker.lock"
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

// leaderLock takes the worker lock if it is free and returns at once. It
// reports false, with a zero heldLock, when another process holds it.
type leaderLock func(ctx context.Context) (lock heldLock, ok bool, err error)

// heldLock is a worker lock this process took.
type heldLock struct {
	release func()
	// check reports whether the lock is still held. Nil for a lock that
	// holds as long as the process lives (a lock file, the memory lock).
	check func(ctx context.Context) error
}

// heldFor wraps a release func that comes with no check.
func heldFor(release func(), ok bool, err error) (heldLock, bool, error) {
	return heldLock{release: release}, ok, err
}

// realtimeLeaderLock returns the worker lock for st: the database's own lock
// on a backend that runs several processes (see storeLeaderLockFor), a lock
// file in the cache directory on disk, and a lock that is always free in
// memory.
func realtimeLeaderLock(fs storage.FS, cacheDir string, st store.Store) (leaderLock, error) {
	if l := storeLeaderLockFor(st); l != nil {
		return l, nil
	}
	if _, mem := fs.(*storage.MemFS); mem || fs == nil || cacheDir == "" {
		return func(context.Context) (heldLock, bool, error) { return heldLock{release: func() {}}, true, nil }, nil
	}
	l, err := filelock.New(filepath.Join(cacheDir, realtimeWorkerLockFile))
	if err != nil {
		return nil, fmt.Errorf("realtime worker: %w", err)
	}
	return func(context.Context) (heldLock, bool, error) { return heldFor(l.TryLock()) }, nil
}

// realtimeWorker runs the [realtime.Sweeper], the [realtime.Triggers] and
// the [realtime.Timers] while this process holds the worker lock. A host starts it with
// [RunRealtimeWorker] and stops it by canceling the context it passed;
// [Services.Close] stops it too.
type realtimeWorker struct {
	sweeper  *realtime.Sweeper
	triggers *realtime.Triggers
	timers   *realtime.Timers
	dueWake  *realtime.DueWake
	changes  store.Watcher
	lock     leaderLock
	wake     <-chan struct{}
	takeover time.Duration
	// leading is set while this process leads; the event drain notes
	// changes only then.
	leading atomic.Bool

	mu      sync.Mutex
	running chan struct{}
	closed  chan struct{}
	once    sync.Once
}

// realtimeWorkerParts are the collaborators of a [realtimeWorker].
type realtimeWorkerParts struct {
	sweeper  *realtime.Sweeper
	triggers *realtime.Triggers
	timers   *realtime.Timers
	// dueWake reports the due times writes store, so a timer round can run
	// before its planned time.
	dueWake *realtime.DueWake
	// changes reports the entity changes the triggers look at.
	changes store.Watcher
	lock    leaderLock
	wake    <-chan struct{}
}

// newRealtimeWorker returns the worker over p.
//
// Nil: every field of p is rejected.
func newRealtimeWorker(p realtimeWorkerParts) (*realtimeWorker, error) {
	if p.sweeper == nil || p.triggers == nil || p.timers == nil || p.dueWake == nil || p.changes == nil ||
		p.lock == nil || p.wake == nil {

		return nil, errors.New("realtime worker: sweeper, triggers, timers, dueWake, changes, lock and wake are required")
	}
	return &realtimeWorker{
		sweeper: p.sweeper, triggers: p.triggers, timers: p.timers, dueWake: p.dueWake, changes: p.changes,
		lock: p.lock, wake: p.wake,
		takeover: realtimeTakeoverInterval, closed: make(chan struct{}),
	}, nil
}

// buildRealtimeWorker builds the worker of an assembled Services, or
// returns nil when the process is not long-running or has no channels.
//
// Nil: refs.channels and refs.channelTombs are accepted (no worker).
func buildRealtimeWorker(
	base *SharedBase, st store.Store, kv state.KV, refs refHolders,
) (*realtimeWorker, error) {
	if !base.opts.backgroundAutomationJobs || refs.channels == nil || refs.channelTombs == nil ||
		refs.realtimeLocal == nil {

		return nil, nil //nolint:nilnil // not a long-running process: no worker
	}
	cfg := realtimeConfigSource{h: base.realtime}
	sweeper, err := realtime.NewSweeper(realtime.SweepDeps{
		Store: refs.channels, Config: cfg,
		Anchors: anchorExistence{st: st}, Marker: refs.realtimeLocal, Clock: time.Now, DueWake: refs.dueWake,
	})
	if err != nil {
		return nil, err
	}
	rows, err := newEntitySource(st, base.worlds.DefaultWorld())
	if err != nil {
		return nil, err
	}
	triggers, err := realtime.NewTriggers(realtime.TriggerDeps{
		Store: refs.channels, Config: cfg, Local: refs.realtimeLocal, Meta: base.meta,
		Rows: rows, Seen: kv, DueWake: refs.dueWake,
	})
	if err != nil {
		return nil, err
	}
	timers, err := realtime.NewTimers(realtime.TimerDeps{
		Store: refs.channels, Config: cfg, Marker: refs.realtimeLocal, Clock: time.Now,
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
	return newRealtimeWorker(realtimeWorkerParts{
		sweeper: sweeper, triggers: triggers, timers: timers, dueWake: refs.dueWake, changes: st, lock: l,
		wake: refs.channelTombs.wake,
	})
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

// run tries to take the worker lock until it gets it or ctx ends, and
// then leads until ctx ends or the lock is lost, when it goes back to
// trying. Store events are drained on a goroutine of
// their own, so a long round never fills the subscription.
func (w *realtimeWorker) run(ctx context.Context) {
	events, cancel := w.changes.Subscribe(triggerEventBuffer)
	defer cancel()
	var drained sync.WaitGroup
	drained.Go(func() { w.drain(ctx, events) })
	defer drained.Wait()
	takeover := time.NewTicker(w.takeover)
	defer takeover.Stop()
	for {
		held, ok, err := w.lock(ctx)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				slog.Warn("realtime worker: cannot take the worker lock; trying again", "error", err)
			}
		case ok:
			w.lead(ctx, held.check)
			held.release()
			if ctx.Err() != nil {
				return
			}
		default:
			slog.Debug("realtime worker: another process leads")
		}
		select {
		case <-ctx.Done():
			return
		case <-takeover.C:
		case <-w.wake: // the leader purges
		}
	}
}

// drain notes the entity changes for the triggers while this process
// leads, and drops them otherwise.
func (w *realtimeWorker) drain(ctx context.Context, events <-chan store.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				slog.Warn("realtime worker: the store stopped reporting changes; triggers rely on the reconcile")
				return
			}
			if c, isEntity := changeOf(ev); isEntity && w.leading.Load() {
				w.triggers.Note(c)
			}
		}
	}
}

// lead runs the sweeps and the triggers until ctx ends or check fails,
// which it calls every takeover interval: everything once
// at start, then the lifecycle sweep and the purge poll every minute, full
// retention every hour, the purge poll on every wake, a trigger round on
// every tick that has work, a trigger reconcile on its timer, and a timer
// round when the [realtime.TimerSchedule] says. The trigger engine and the timer loop
// start and end with nothing in memory, so they never act on what they
// held while another process led.
func (w *realtimeWorker) lead(ctx context.Context, check func(context.Context) error) {
	ctx = principal.With(ctx, principal.Principal{User: principal.UserRealtime, Tool: principal.ToolRealtime})
	w.leading.Store(true)
	w.triggers.Reset()
	w.timers.Reset()
	defer func() {
		w.leading.Store(false)
		w.triggers.Reset()
		w.timers.Reset()
	}()
	slog.Info("realtime worker: this process leads")
	w.retention(ctx)
	w.lifecycle(ctx)
	w.reconcile(ctx)
	w.dueWake.Take() // the first round below covers what was noted before
	sched := w.timerRound(ctx)
	timer := time.NewTimer(sched.Delay(time.Now()))
	defer timer.Stop()
	minute := time.NewTicker(realtimeLifecycleInterval)
	defer minute.Stop()
	hour := time.NewTicker(realtimeRetentionInterval)
	defer hour.Stop()
	tick := time.NewTicker(realtime.TriggerTick)
	defer tick.Stop()
	reconcile := time.NewTimer(reconcileDelay())
	defer reconcile.Stop()
	verify := time.NewTicker(w.takeover)
	defer verify.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-verify.C:
			if check == nil {
				continue
			}
			if err := check(ctx); err != nil {
				if ctx.Err() == nil {
					slog.Warn("realtime worker: lost the worker lock; this process stops leading", "error", err)
				}
				return
			}
		case <-tick.C:
			if w.triggers.Pending() {
				rep, err := w.triggers.RunRound(ctx)
				w.triggerReport(ctx, "round", rep, err)
			}
		case <-reconcile.C:
			w.reconcile(ctx)
			reconcile.Reset(reconcileDelay())
		case <-minute.C:
			w.lifecycle(ctx)
			w.purge(ctx)
		case <-hour.C:
			w.retention(ctx)
		case <-w.wake:
			w.purge(ctx)
		case <-timer.C:
			sched = w.timerRound(ctx)
			timer.Reset(sched.Delay(time.Now()))
		case <-w.dueWake.C():
			if sched.Wake(w.dueWake.Take()) {
				timer.Reset(sched.Delay(time.Now()))
			}
		}
	}
}

// timerRound runs one timer round and returns the schedule of the next.
func (w *realtimeWorker) timerRound(ctx context.Context) realtime.TimerSchedule {
	rep, err := w.timers.RunRound(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Warn("realtime: session timer round failed", "error", err)
	}
	if rep.Fired+rep.Rescheduled+rep.Failed > 0 {
		slog.Info("realtime: session timer round", "fired", rep.Fired, "rescheduled", rep.Rescheduled,
			"failed", rep.Failed, "stranded", rep.Stranded, "more", rep.More)
	}
	return realtime.NextTimerRound(rep, time.Now())
}

// reconcileDelay is [realtime.TriggerReconcile] with up to a tenth
// of jitter either way, so processes started together do not reconcile
// together.
func reconcileDelay() time.Duration {
	const tenth = realtime.TriggerReconcile / 10
	return realtime.TriggerReconcile - tenth + rand.N(2*tenth) //nolint:gosec // jitter, not a secret
}

func (w *realtimeWorker) reconcile(ctx context.Context) {
	rep, err := w.triggers.RunReconcile(ctx)
	w.triggerReport(ctx, "reconcile", rep, err)
}

func (w *realtimeWorker) triggerReport(ctx context.Context, what string, rep realtime.TriggerReport, err error) {
	if err != nil && ctx.Err() == nil {
		slog.Warn("realtime: trigger "+what+" failed", "error", err)
	}
	if rep != (realtime.TriggerReport{}) {
		slog.Info("realtime: trigger "+what, "posted", rep.Posted, "skipped", rep.Skipped, "failed", rep.Failed,
			"baselined", rep.Baselined)
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

func (w *realtimeWorker) purge(ctx context.Context) {
	rep, err := w.sweeper.RunPurge(ctx)
	w.report(ctx, "purge", rep, err)
}

func (w *realtimeWorker) retention(ctx context.Context) {
	rep, err := w.sweeper.RunRetention(ctx)
	w.report(ctx, "retention", rep, err)
}

func (w *realtimeWorker) report(ctx context.Context, what string, rep realtime.RetentionReport, err error) {
	if err != nil && ctx.Err() == nil {
		slog.Warn("realtime: "+what+" sweep failed", "error", err)
	}
	if rep != (realtime.RetentionReport{}) {
		slog.Info("realtime: "+what+" sweep", "purged", rep.Purged, "confirmed", rep.Confirmed,
			"restored", rep.Restored, "detached", rep.Detached, "types_cleared", rep.TypesCleared)
	}
}
