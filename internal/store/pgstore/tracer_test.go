//go:build postgres

package pgstore

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestQueryTracer_LogsAtDebugLevel exercises the tracer directly (without
// a real pgx connection) to confirm:
//
//   - At slog.Debug it emits one record per (Start, End) pair.
//   - At slog.Info it emits nothing and, with no stats on the context,
//     returns the context UNCHANGED — the production no-alloc guarantee.
//   - The emitted record carries the SQL text and a non-negative
//     duration_us.
//
// The full integration (pool → query → tracer) is covered by
// TestQueryTracer_FromPoolEmits in tracer_pool_test.go.
func TestQueryTracer_LogsAtDebugLevel(t *testing.T) {
	for _, tc := range []struct {
		name     string
		level    slog.Level
		wantLogs bool
	}{
		{"debug emits", slog.LevelDebug, true},
		{"info silences", slog.LevelInfo, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: tc.level})
			t.Cleanup(swapDefaultLogger(slog.New(h)))

			tr := queryTracer{}
			base := context.Background()
			ctx := tr.TraceQueryStart(base, nil, pgx.TraceQueryStartData{
				SQL:  "SELECT 1",
				Args: []any{42},
			})
			tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
				CommandTag: pgconn.CommandTag{},
			})

			out := buf.String()
			if tc.wantLogs {
				require.Contains(t, out, "pgstore: query")
				require.Contains(t, out, `sql="SELECT 1"`)
				require.Contains(t, out, "duration_us=")
			} else {
				require.Empty(t, out, "Info level should not emit Debug traces")
				require.Equal(t, base, ctx,
					"with Debug off and no stats the tracer must not allocate a derived context")
			}
		})
	}
}

// TestQueryTracer_RecordsIntoContextStats: a context carrying
// store.QueryStats is accounted regardless of log level, and an errored
// statement still counts — a failed round-trip is still a round-trip.
func TestQueryTracer_RecordsIntoContextStats(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	t.Cleanup(swapDefaultLogger(slog.New(h)))

	tr := queryTracer{}
	ctx, stats := store.WithQueryStats(context.Background())

	for i, err := range []error{nil, context.DeadlineExceeded} {
		qctx := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
		time.Sleep(time.Millisecond)
		tr.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{Err: err})
		require.EqualValues(t, i+1, stats.Queries())
	}
	require.Greater(t, stats.Duration(), time.Duration(0))
	require.Empty(t, buf.String(), "accounting must not log at Info")
}

// swapDefaultLogger replaces slog.Default and returns a closer that
// restores the previous default. Lets each test isolate its slog
// output without polluting siblings.
func swapDefaultLogger(l *slog.Logger) func() {
	prev := slog.Default()
	slog.SetDefault(l)
	return func() { slog.SetDefault(prev) }
}
