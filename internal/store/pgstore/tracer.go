package pgstore

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// queryTracer is the [pgx.QueryTracer] attached to every pool Open builds.
// It does two independent things per statement, each opt-in by the caller's
// context or logger rather than by pool configuration:
//
//   - Accounting: when the query's context carries a [store.QueryStats]
//     (a request middleware installs one), the statement's count and
//     duration are recorded there. This is how "how many queries did this
//     request issue?" is answered without touching the ~60 call sites that
//     run SQL.
//   - Logging: when slog's default handler is enabled at Debug, one
//     record per statement with SQL text, arguments, duration and row count.
//     Debug rather than Info is deliberate: query traffic is chatty, and
//     operators flip to Debug only when diagnosing a plan or an unexpected
//     query count.
//
// When neither applies — production with Debug off and no stats on the
// context — TraceQueryStart returns the context unchanged, so the per-query
// overhead is one context lookup and one level check, no allocation. That
// is why the tracer can be attached unconditionally: the earlier design
// attached it only when Debug was on at Open time, which made stats
// impossible and froze the logging decision at startup.
//
// Transactions are covered too: a [Store.Tx] view runs on a pgx.Tx from the
// same pool, and pgx traces BEGIN/COMMIT and every statement inside through
// the same connection-level tracer.
type queryTracer struct{}

type tracerCtxKey struct{}

type tracerCtxVal struct {
	start time.Time
	stats *store.QueryStats // nil when the context carries none
	debug bool
	sql   string
	args  []any
}

// TraceQueryStart decides what, if anything, this statement will report and
// parks that decision on the context for TraceQueryEnd. Nothing is logged
// here — one record per statement, emitted once with its timing.
func (queryTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	stats := store.QueryStatsFrom(ctx)
	debug := slog.Default().Enabled(ctx, slog.LevelDebug)
	if stats == nil && !debug {
		return ctx
	}
	v := &tracerCtxVal{start: time.Now(), stats: stats, debug: debug}
	if debug {
		v.sql = data.SQL
		v.args = data.Args
	}
	return context.WithValue(ctx, tracerCtxKey{}, v)
}

// TraceQueryEnd records the statement against the context's stats and, at
// Debug, emits one slog record. The `duration_us` field is a microseconds
// integer (jq-friendly, avoids the floating-point printing variance of
// time.Duration's String).
func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	v, ok := ctx.Value(tracerCtxKey{}).(*tracerCtxVal)
	if !ok {
		return
	}
	elapsed := time.Since(v.start)
	if v.stats != nil {
		v.stats.Record(elapsed)
	}
	if !v.debug {
		return
	}
	attrs := []any{
		"sql", v.sql,
		"args", v.args,
		"duration_us", elapsed.Microseconds(),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err.Error())
	} else {
		// CommandTag is "<verb> <rows>" or just a verb — surface the
		// row-count when present so operators can see how big a
		// query was without re-running EXPLAIN.
		if rows := data.CommandTag.RowsAffected(); rows >= 0 {
			attrs = append(attrs, "rows", rows)
		}
	}
	slog.Debug("pgstore: query", attrs...)
}
