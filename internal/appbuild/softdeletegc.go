package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// softDeleteGCKind is the job that purges soft-deleted entities whose undo
// window has passed.
var softDeleteGCKind = jobs.NewKind("soft-delete", "gc")

// Defaults for the soft-delete GC. The delay is the undo window: how long a
// soft-deleted entity can still be restored. The interval is how often the GC
// looks, so an entity is purged between delay and delay+interval after its
// delete.
const (
	defaultSoftDeleteDelay      = 60 * time.Second
	defaultSoftDeleteGCInterval = 15 * time.Second
)

// startSoftDeleteGC registers the soft-delete GC job on q and starts a ticker
// that enqueues it every interval. Entities soft-deleted more than delay ago
// are purged by [entitymanager.PurgeSoftDeleted].
//
// Each tick enqueues with one fixed IdempotencyKey, so a run that is still
// pending absorbs the next tick instead of stacking behind it. On the durable
// queue the key also holds across processes, so several servers sharing a
// database run one GC at a time.
//
// It never fails boot. With a store that cannot soft-delete, or when the job
// cannot be registered, it starts nothing. The returned stop function is
// always safe to call; it stops the ticker, and the queue's own Close drains a
// run in flight.
func startSoftDeleteGC(mgr *entitymanager.Manager, q jobs.Client, delay, interval time.Duration) (stop func()) {
	if mgr == nil || q == nil || !entitymanager.SupportsSoftDelete(mgr) {
		return func() {}
	}
	err := q.Register(softDeleteGCKind, func(ctx context.Context, _ jobs.Job) error {
		n, err := entitymanager.PurgeSoftDeleted(ctx, mgr, time.Now().Add(-delay))
		if n > 0 {
			slog.Info("soft delete: purged entities past the undo window", "entities", n)
		}
		return err
	})
	if err != nil {
		slog.Warn("soft delete: gc not started", "error", err)
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// RetryNever: the next tick is the retry.
				err := q.Enqueue(ctx, jobs.Job{Kind: softDeleteGCKind, IdempotencyKey: softDeleteGCKind.String()})
				switch {
				case err == nil, errors.Is(err, jobs.ErrDuplicateJob):
				case errors.Is(err, context.Canceled), errors.Is(err, jobs.ErrClosed):
					// Shutdown raced a tick.
				default:
					slog.Warn("soft delete: gc enqueue failed", "error", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
