package store

import (
	"context"
	"sync"
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
// tracing facility. Beyond the two counters it keeps one count per distinct
// statement text ([QueryStats.RecordStatement]), capped at
// [maxStatementShapes], so the memory cost does not grow with the number of
// statements a request runs.
//
// Safe for concurrent use: a request that fans out to goroutines sharing its
// context accumulates into the same counters.
type QueryStats struct {
	queries atomic.Int64
	nanos   atomic.Int64

	mu     sync.Mutex
	shapes map[string]int64
}

// maxStatementShapes caps the distinct statement texts one QueryStats
// counts. A backend issues parameterised SQL from a fixed set of call
// sites, so a request rarely needs this many; the cap bounds memory for one
// that renders literals into its SQL. Past it, a new text replaces the
// least counted one and inherits its count plus one (the Space-Saving
// algorithm), so a statement repeated hundreds of times is still found
// even when it starts after the table filled. Counts past the cap may
// overstate by at most the replaced count.
const maxStatementShapes = 256

// Record adds one statement of the given duration.
func (s *QueryStats) Record(d time.Duration) {
	s.queries.Add(1)
	s.nanos.Add(int64(d))
}

// RecordStatement is [QueryStats.Record] for a backend that knows the
// statement text. sql must be the parameterised text, never one with bound
// values inlined: [QueryStats.MostRepeated] reports it in a log line.
func (s *QueryStats) RecordStatement(sql string, d time.Duration) {
	s.Record(d)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shapes == nil {
		s.shapes = map[string]int64{}
	}
	if _, ok := s.shapes[sql]; !ok && len(s.shapes) >= maxStatementShapes {
		var minSQL string
		minN := int64(-1)
		for text, c := range s.shapes {
			if minN < 0 || c < minN {
				minSQL, minN = text, c
			}
		}
		delete(s.shapes, minSQL)
		s.shapes[sql] = minN
	}
	s.shapes[sql]++
}

// MostRepeated returns the statement text recorded most often and its count.
// One text run hundreds of times in one request is the signature of a
// per-row lookup (N+1). Ties go to the lexically smaller text, so the answer
// is deterministic. It returns "", 0 when no text was recorded.
func (s *QueryStats) MostRepeated() (sql string, count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var top string
	var n int64
	for text, c := range s.shapes {
		if c > n || (c == n && text < top) {
			top, n = text, c
		}
	}
	return top, n
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
