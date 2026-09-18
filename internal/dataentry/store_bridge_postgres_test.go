//go:build postgres

package dataentry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

const bridgeTestDBEnv = "RELA_TEST_DATABASE_URL"

var bridgeSchemaCounter atomic.Int64

// TestStoreEventBridgeCrossProcessSSE (AC4): a write committed by writer A
// (a separate pgstore process) reaches the SSE feed of an App whose store is a
// SECOND pgstore on the same database — proving cross-process live-reload
// end-to-end (the headline of TKT-WZYWM9 scope b).
func TestStoreEventBridgeCrossProcessSSE(t *testing.T) {
	base := os.Getenv(bridgeTestDBEnv)
	if base == "" {
		t.Skipf("%s not set; skipping cross-process SSE test", bridgeTestDBEnv)
	}

	schema := fmt.Sprintf("relabridge_%d_%d", os.Getpid(), bridgeSchemaCounter.Add(1))
	dsn := dsnWithSchema(t, base, schema)
	ctx := context.Background()

	// Create + migrate the isolated schema.
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+quoteIdent(schema))
	require.NoError(t, err)
	require.NoError(t, pgstore.Migrate(ctx, pool))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
		pool.Close()
	})

	// Writer A: a standalone pgstore on the schema. Each side gets its OWN pool
	// (pgstore.Open takes an injected handle, TKT-OGTVJW), which is what makes
	// these two stand in for two rela-server processes.
	aPool, aPoolCloser, err := pgstore.NewPool(ctx, dsn)
	require.NoError(t, err)
	a, _, err := pgstore.Open(ctx, aPool, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close(); _ = aPoolCloser.Close() })

	// The "server B" side: a second pgstore wired into a dataentry App's bridge.
	bPool, bPoolCloser, err := pgstore.NewPool(ctx, dsn)
	require.NoError(t, err)
	bStore, _, err := pgstore.Open(ctx, bPool, dsn)
	require.NoError(t, err)
	app := &App{store: bStore, broker: newEventBroker()}
	app.startStoreEventBridge()
	t.Cleanup(func() {
		app.StopWatching()
		_ = bStore.Close()
		_ = bPoolCloser.Close()
	})

	sse := app.broker.subscribe()
	defer app.broker.unsubscribe(sse)

	// A writes; B's SSE feed must surface a cross-process entity change
	// for the feature type (no id on the wire — TKT-POT9GQ).
	require.NoError(t, a.CreateEntity(ctx, entity.New("FEAT-1", "feature")))
	ev := waitForEntityChange(t, sse, "feature")
	require.Empty(t, ev.Data, "cross-process entity change must carry no id")
}

// dsnWithSchema returns base re-serialized as a key/value DSN with search_path
// pinned to schema, mirroring the production helpers in `tenant`, `docscapture`
// and `backendtest`.
//
// The URL form this used to build — `?options=-c search_path=...` through
// url.Values.Encode — cannot survive the round trip. Encode writes a space as
// "+", but a URI query means a literal plus there, so libpq (and pgx from
// v5.11.0 on) reads the parameter name as "+search_path" and the server rejects
// the connection. Percent-encoding the space instead breaks the "=" inside the
// value. A key/value DSN has no such escaping to get wrong.
func dsnWithSchema(t *testing.T, base, schema string) string {
	t.Helper()
	cfg, err := pgx.ParseConfig(base)
	// pgx redacts the password in parse errors, so this is safe to surface.
	require.NoError(t, err)
	kv := []string{
		"host=" + cfg.Host,
		fmt.Sprintf("port=%d", cfg.Port),
		"user=" + cfg.User,
		"dbname=" + cfg.Database,
		"search_path=" + schema + ",public",
	}
	if cfg.Password != "" {
		kv = append(kv, "password="+cfg.Password)
	}
	// TLSConfig is nil exactly when the DSN disabled it; preserve that rather
	// than defaulting to on, which a plaintext CI container would refuse.
	if cfg.TLSConfig == nil {
		kv = append(kv, "sslmode=disable")
	}
	return strings.Join(kv, " ")
}

func quoteIdent(s string) string { return `"` + s + `"` }
