package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The flow worker drives the flow engine in a long-running host
// (TKT-FWQZGAM): every flowWorkerTick it runs [flowengine.Engine.DueRound],
// which fires due timers, settles runs left with a pending mark (a crash or
// a failed output) after their backoff, and tests joins that wait on a
// `when:`.
//
// With several processes on one project or tenant, one leads, as with the
// realtime worker: it holds the flow worker lock (see workerLeaderLock)
// and the others try to take it every realtimeTakeoverInterval. The
// postgres lock is keyed by the schema, so tenants never exclude each
// other. The leader checks the lock between ticks, so two leaders can
// overlap after the database drops a session: for up to a takeover
// interval plus one tick (flowWorkerRounds x flowWorkerRoundTimeout). The
// engine's compare-and-set keeps that safe, and a create output is
// claimed before it is written, so it is written at most once.
//
// One tick advances at most flowWorkerRounds rounds of
// [flowengine.MaxDuePerRound] runs; the rest wait for the next tick. Each
// round ends at flowWorkerRoundTimeout, and the next tick goes on after
// the run it cut off. That run keeps its pending mark and is settled on a
// later tick, as after a crash. The engine does not let the deadline cut a
// create output (see [flowengine.Engine.DueRound]).
//
// The worker acts as system:flow. It holds no capability of its own: the
// engine writes headers as system:flow and outputs as each flow's run_as,
// binding each identity at the write (see buildFlowEngine).
const (
	flowWorkerTick         = 15 * time.Second
	flowWorkerRounds       = 4
	flowWorkerRoundTimeout = time.Minute
	// flowWorkerLockFile is the flow worker lock of the file backend, in
	// the project's cache directory.
	flowWorkerLockFile = "flow-worker.lock"
	// flowWorkerLockKey names the flow worker lock in the database.
	flowWorkerLockKey = "flow-worker"
	// workerLocks is how many worker locks a long-running host holds: the
	// realtime worker's and the flow worker's.
	workerLocks = 2
)

// dueRounds is the part of [flowengine.Engine] the worker runs.
type dueRounds interface {
	DueRound(ctx context.Context, after flowrun.DueCursor) (flowengine.DueReport, error)
}

// flowWorker runs the flow engine's due rounds while this process holds
// the flow worker lock. A host starts it with [RunFlowWorker] and stops it
// by canceling the context it passed; [Services.Close] stops it too.
type flowWorker struct {
	due      dueRounds
	lock     leaderLock
	tick     time.Duration
	takeover time.Duration
	// after is where the next tick starts; only the leading goroutine
	// touches it.
	after flowrun.DueCursor

	life *workerLife
}

// newFlowWorker returns the worker over due, under lock.
//
// Nil: due and lock are rejected.
func newFlowWorker(due dueRounds, lock leaderLock) (*flowWorker, error) {
	if due == nil || lock == nil {
		return nil, errors.New("flow worker: engine and lock are required")
	}
	return &flowWorker{
		due: due, lock: lock, tick: flowWorkerTick, takeover: realtimeTakeoverInterval,
		life: newWorkerLife(),
	}, nil
}

// buildFlowWorker builds the worker of an assembled Services.
//
// Nil: eng is accepted (no flows). The worker is nil in a process that is
// not long-running or declares no flows.
func buildFlowWorker(base *SharedBase, st store.Store, eng *flowengine.Engine) (*flowWorker, error) {
	if !base.opts.backgroundAutomationJobs || eng == nil {
		return nil, nil //nolint:nilnil // not a long-running process, or no flows: no worker
	}
	// The flow worker's lock and the realtime worker's are held side by
	// side; a pool without room for both would leave one never leading.
	if err := storeLockRoom(st, workerLocks); err != nil {
		return nil, err
	}
	cacheDir := ""
	if base.cfg.Paths != nil {
		cacheDir = base.cfg.Paths.CacheDir
	}
	l, err := workerLeaderLock(base.cfg.FS, cacheDir, st, flowWorkerLockKey, flowWorkerLockFile)
	if err != nil {
		return nil, err
	}
	return newFlowWorker(eng, l)
}

// RunFlowWorker runs the flow worker of s as [RunRealtimeWorker] runs the
// realtime worker, and the hosts call the two side by side.
func RunFlowWorker(ctx context.Context, s *Services) <-chan struct{} {
	w := s.flowWorker
	if w == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return w.life.start(ctx, "flow worker", w.run)
}

// stop ends a running worker and waits for it. Safe to call more than once.
func (w *flowWorker) stop() { w.life.stop() }

// run tries to take the lock until it gets it or ctx ends, then leads
// until ctx ends or the lock is lost, when it goes back to trying.
func (w *flowWorker) run(ctx context.Context) {
	takeover := time.NewTicker(w.takeover)
	defer takeover.Stop()
	for {
		held, ok, err := w.lock(ctx)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				slog.Warn("flow worker: cannot take the worker lock; trying again", "error", err)
			}
		case ok:
			w.lead(ctx, held.check)
			held.release()
			if ctx.Err() != nil {
				return
			}
		default:
			slog.Debug("flow worker: another process leads")
		}
		select {
		case <-ctx.Done():
			return
		case <-takeover.C:
		}
	}
}

// lead runs a tick at once and then every tick interval, until ctx ends
// or check fails; it calls check every takeover interval.
func (w *flowWorker) lead(ctx context.Context, check func(context.Context) error) {
	if ctx.Err() != nil {
		// The lock came free because the host is stopping.
		return
	}
	ctx = principal.With(ctx, principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow})
	slog.Info("flow worker: this process leads")
	w.runTick(ctx)
	tick := time.NewTicker(w.tick)
	defer tick.Stop()
	// A lock without a check (a lock file) is never lost.
	var verify <-chan time.Time
	if check != nil {
		t := time.NewTicker(w.takeover)
		defer t.Stop()
		verify = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-verify:
			if err := check(ctx); err != nil {
				if ctx.Err() == nil {
					slog.Warn("flow worker: lost the worker lock; this process stops leading", "error", err)
				}
				return
			}
		case <-tick.C:
			w.runTick(ctx)
		}
	}
}

// runTick runs up to flowWorkerRounds due rounds, each from where the
// last one stopped. When the last round was full, or a round failed, the
// next tick goes on from there rather than from the front, so runs that
// keep failing or stalling at the front of the due order cannot starve the
// rest. It returns how many runs the tick advanced and how many failed.
//
// Only the leading goroutine calls it.
func (w *flowWorker) runTick(ctx context.Context) (advanced, failed int) {
	after := w.after
	w.after = flowrun.DueCursor{}
	for range flowWorkerRounds {
		rctx, cancel := context.WithTimeout(ctx, flowWorkerRoundTimeout)
		rep, err := w.due.DueRound(rctx, after)
		cancel()
		advanced += rep.Advanced
		failed += rep.Failed
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("flow worker: a due round failed", "error", err)
				w.after = rep.Next
			}
			break
		}
		if !rep.More {
			break
		}
		after = rep.Next
		w.after = after
	}
	if advanced+failed > 0 {
		slog.Info("flow worker: due runs", "advanced", advanced, "failed", failed)
	}
	return advanced, failed
}
