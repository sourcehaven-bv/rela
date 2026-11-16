package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

// recordRetentionKind is the job that drops record rows past their type's
// retention.
var recordRetentionKind = jobs.NewKind("records", "retention")

// defaultRecordRetentionInterval is how often the retention job runs. A row
// is dropped between its retention and its retention plus this interval.
const defaultRecordRetentionInterval = time.Hour

// startRecordRetention registers the retention job on q and starts a ticker
// that enqueues it every interval, like [startSoftDeleteGC]: one fixed
// IdempotencyKey, so a pending run absorbs the next tick, and on the durable
// queue several servers run one sweep at a time.
//
// It never fails boot. Without a record type that declares a retention, or
// when the job cannot be registered, it starts nothing. The returned stop
// function is always safe to call.
func startRecordRetention(svc *records.Service, q jobs.Client, interval time.Duration) (stop func()) {
	if svc == nil || q == nil || !hasRetention(svc) {
		return func() {}
	}
	err := q.Register(recordRetentionKind, func(ctx context.Context, _ jobs.Job) error {
		n, err := svc.Expire(ctx)
		if n > 0 {
			slog.Info("records: dropped rows past their retention", "rows", n)
		}
		return err
	})
	if err != nil {
		slog.Warn("records: retention not started", "error", err)
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
				err := q.Enqueue(ctx, jobs.Job{Kind: recordRetentionKind, IdempotencyKey: recordRetentionKind.String()})
				switch {
				case err == nil, errors.Is(err, jobs.ErrDuplicateJob):
				case errors.Is(err, context.Canceled), errors.Is(err, jobs.ErrClosed):
				default:
					slog.Warn("records: retention enqueue failed", "error", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func hasRetention(svc *records.Service) bool {
	for _, t := range svc.Types() {
		if t.Retention != "" {
			return true
		}
	}
	return false
}
