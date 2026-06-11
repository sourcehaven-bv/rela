package pgstore

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// debugQueryTracer is a [pgx.QueryTracer] that logs every SQL query
// at slog.Debug level with the SQL text, arguments, and execution
// duration. It is attached when slog's default handler is enabled at
// Debug level; at higher levels it is omitted, so production
// deployments pay no per-query overhead.
//
// Logging at Debug rather than Info is deliberate: query traffic is
// chatty (one log line per query, every query, for the lifetime of
// the process). Operators flip slog to Debug only when they need it
// — typically to diagnose a misbehaving query plan or unexpected
// query count from a higher layer.
type debugQueryTracer struct{}

type tracerCtxKey struct{}

type tracerCtxVal struct {
	start time.Time
	sql   string
	args  []any
}

// TraceQueryStart records the query parameters on the context for
// TraceQueryEnd to read. The data is intentionally NOT logged here
// — slog the whole event once with timing, not twice.
func (debugQueryTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(ctx, tracerCtxKey{}, &tracerCtxVal{
		start: time.Now(),
		sql:   data.SQL,
		args:  data.Args,
	})
}

// TraceQueryEnd emits one slog.Debug record per query. The
// `duration_us` field is a microseconds integer (jq-friendly,
// avoids the floating-point printing variance of time.Duration's
// String).
func (debugQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	v, ok := ctx.Value(tracerCtxKey{}).(*tracerCtxVal)
	if !ok {
		return
	}
	attrs := []any{
		"sql", v.sql,
		"args", v.args,
		"duration_us", time.Since(v.start).Microseconds(),
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

// debugEnabled reports whether slog's default logger will emit Debug
// records for ctx. Used by Open to decide whether to attach the
// tracer at all — the tracer's overhead is the context value alloc
// per query, trivial in isolation but multiplied by every query in
// the system.
func debugEnabled(ctx context.Context) bool {
	return slog.Default().Enabled(ctx, slog.LevelDebug)
}
