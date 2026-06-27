//go:build postgres

package pgstore_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestDebugQueryTracer_FromPoolEmits proves the env-driven path:
// when slog Debug is enabled BEFORE pgstore.Open, the pool gets the
// tracer attached and queries emit log records. Without this, the
// debugEnabled() check could be wired wrong and we'd ship a tracer
// that nobody ever sees.
func TestDebugQueryTracer_FromPoolEmits(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	st, _, closer, err := pgstore.Open(context.Background(), testDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })

	// Drive one query through the store. The exact query doesn't
	// matter; we only need a pgx round-trip so the tracer fires.
	_, _ = st.GetEntity(context.Background(), "nonexistent")

	require.Contains(t, buf.String(), "pgstore: query",
		"Open with Debug enabled should attach the tracer and queries should emit")
}
