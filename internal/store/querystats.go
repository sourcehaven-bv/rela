package store

import (
	"context"
	"sync/atomic"
	"time"
)

// QueryStats accumulates the storage work performed on behalf of one
// operation — typically one HTTP request. A backend that talks to an
// external database records every statement it issues against the stats
// found on the context; the fs and memory backends record nothing, which is
// honest: they have no per-statement cost to report.
//
// The value is meant to answer "how many round-trips did this request cost,
// and how long did the database spend on them?", the two numbers that make
// an N+1 pattern visible before it becomes a latency complaint. It is NOT a
// tracing facility — no per-statement detail is kept, so the memory cost is
// two counters regardless of how many statements run.
//
// Safe for concurrent use: a request that fans out to goroutines sharing its
// context accumulates into the same counters.
type QueryStats struct {
	queries atomic.Int64
	nanos   atomic.Int64
}

// Record adds one statement of the given duration.
func (s *QueryStats) Record(d time.Duration) {
	s.queries.Add(1)
	s.nanos.Add(int64(d))
}

// Queries returns the number of statements recorded so far.
func (s *QueryStats) Queries() int64 { return s.queries.Load() }

// Duration returns the summed statement durations recorded so far. It is
// database time, not wall time: statements issued concurrently overlap, and
// time spent in Go between statements is not included.
func (s *QueryStats) Duration() time.Duration { return time.Duration(s.nanos.Load()) }

type queryStatsKey struct{}

// WithQueryStats attaches a fresh QueryStats to ctx and returns both. The
// caller that starts an operation (a request middleware, a job runner) owns
// the returned stats and reads them when the operation ends.
func WithQueryStats(ctx context.Context) (context.Context, *QueryStats) {
	s := &QueryStats{}
	return context.WithValue(ctx, queryStatsKey{}, s), s
}

// QueryStatsFrom returns the QueryStats attached to ctx.
//
// Nil: returned when the context carries none — a backend records against it
// only when non-nil, so an operation that never asked for stats costs nothing
// beyond this lookup.
func QueryStatsFrom(ctx context.Context) *QueryStats {
	s, _ := ctx.Value(queryStatsKey{}).(*QueryStats)
	return s
}
