//go:build postgres

package pgstore

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// TestDebugQueryTracer_LogsAtDebugLevel exercises the tracer
// directly (without a real pgx connection) to confirm:
//
//   - At slog.Debug it emits one record per (Start, End) pair.
//   - At slog.Info it emits nothing — the per-query overhead is
//     limited to the (cheap) context value alloc.
//   - The emitted record carries the SQL text and a non-negative
//     duration_us.
//
// The full integration (pool → query → tracer) is covered by
// TestDebugQueryTracer_FromPoolEmits in tracer_pool_test.go.
func TestDebugQueryTracer_LogsAtDebugLevel(t *testing.T) {
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

			tr := debugQueryTracer{}
			ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
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
			}
		})
	}
}

// swapDefaultLogger replaces slog.Default and returns a closer that
// restores the previous default. Lets each test isolate its slog
// output without polluting siblings.
func swapDefaultLogger(l *slog.Logger) func() {
	prev := slog.Default()
	slog.SetDefault(l)
	return func() { slog.SetDefault(prev) }
}
