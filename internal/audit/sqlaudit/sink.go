package sqlaudit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

const (
	// maxPending caps the records queued in memory. A record that arrives
	// when the queue is full is dropped and counted.
	maxPending = 100_000
	// sinkBatch is how many queued records one transaction appends.
	sinkBatch = 500
	// attemptTimeout bounds one append transaction, lock wait included.
	attemptTimeout = 10 * time.Second
	// firstBackoff and maxBackoff shape the retries of a failed batch:
	// doubling from firstBackoff, capped at maxBackoff.
	firstBackoff = 200 * time.Millisecond
	maxBackoff   = 5 * time.Second
	// appendAttempts bounds the tries per batch: with the backoff above,
	// about 20 seconds of database outage cost no records.
	appendAttempts = 8
	// closeTimeout bounds how long [Sink.Close] waits for the queue to drain.
	closeTimeout = 10 * time.Second
)

// Sink is the [audit.Audit] over a [Log]. Record queues the record and one
// goroutine appends the queue in order, a batch per transaction.
//
// Record does not write inline because it has no context and may be called
// while its caller holds the store's write transaction: an inline append on
// another connection would then wait for that transaction, which waits for
// Record. The queue never blocks the caller. A batch that still fails after
// [appendAttempts] tries is logged as audit.write_failed with its size and
// dropped, as the JSONL sink drops a record it cannot write.
type Sink struct {
	log *Log
	// cancel stops a retry wait or an append in flight; the goroutine owns
	// the context it cancels.
	cancel context.CancelFunc
	// tuning holds the retry and close timings; the constants above in
	// production, shorter in tests.
	tuning sinkTuning

	mu       sync.Mutex
	cond     *sync.Cond
	pending  []audit.Record
	inflight int
	dropped  int64
	closed   bool
	done     chan struct{}
}

type sinkTuning struct {
	firstBackoff time.Duration
	attempts     int
	closeTimeout time.Duration
}

var defaultTuning = sinkTuning{firstBackoff: firstBackoff, attempts: appendAttempts, closeTimeout: closeTimeout}

// NewSink starts the writer goroutine over log. The caller owns the sink
// and must [Sink.Close] it.
//
// Nil: log rejected.
func NewSink(log *Log) (*Sink, error) {
	if log == nil {
		return nil, errNilLog
	}
	return newSink(log, defaultTuning), nil
}

func newSink(log *Log, tuning sinkTuning) *Sink {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sink{log: log, cancel: cancel, tuning: tuning, done: make(chan struct{})}
	s.cond = sync.NewCond(&s.mu)
	go s.run(ctx)
	return s
}

// Log returns the log the sink appends to.
func (s *Sink) Log() *Log { return s.log }

// Record queues rec. After Close, or when the queue is full, it logs and
// drops rec.
func (s *Sink) Record(rec audit.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		slog.Error("audit.write_failed", "stage", "closed", "op", rec.Op)
	case len(s.pending) >= maxPending:
		s.dropped++
		if s.dropped == 1 || s.dropped%1000 == 0 {
			slog.Error("audit.write_failed", "stage", "queue_full", "op", rec.Op, "dropped", s.dropped)
		}
	default:
		s.pending = append(s.pending, rec)
		s.cond.Signal()
	}
}

// Close appends what is queued, waiting at most [closeTimeout], and stops
// the writer. It returns an error naming how many records were not written:
// those still queued when the wait ran out, and those dropped because the
// queue was full or their batch failed every attempt. Records queued after Close are dropped.
func (s *Sink) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cond.Signal()
	s.mu.Unlock()

	timer := time.NewTimer(s.tuning.closeTimeout)
	defer timer.Stop()
	select {
	case <-s.done:
	case <-timer.C:
		s.cancel()
		<-s.done
	}
	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	lost := int64(len(s.pending)+s.inflight) + s.dropped
	if lost > 0 {
		return fmt.Errorf("%s: %d audit records were not written", s.log.d.Name, lost)
	}
	return nil
}

func (s *Sink) run(ctx context.Context) {
	defer close(s.done)
	for {
		s.mu.Lock()
		for len(s.pending) == 0 && !s.closed {
			s.cond.Wait()
		}
		if len(s.pending) == 0 || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		n := min(len(s.pending), sinkBatch)
		batch := s.pending[:n:n]
		s.pending = s.pending[n:]
		s.inflight = n
		s.mu.Unlock()

		written := s.append(ctx, batch)

		s.mu.Lock()
		s.inflight = 0
		if !written {
			s.dropped += int64(n)
		}
		s.mu.Unlock()
	}
}

// append writes batch, retrying with backoff, and reports whether it did.
func (s *Sink) append(ctx context.Context, batch []audit.Record) bool {
	backoff := s.tuning.firstBackoff
	var err error
	for attempt := range s.tuning.attempts {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return false
			}
			backoff = min(2*backoff, maxBackoff)
		}
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err = s.log.appendAll(attemptCtx, batch)
		cancel()
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
	}
	slog.Error("audit.write_failed", "stage", "sql", "records", len(batch), "first_op", batch[0].Op, "error", err)
	return false
}
