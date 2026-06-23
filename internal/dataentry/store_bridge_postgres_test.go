//go:build postgres

package dataentry

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

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

	// Writer A: a standalone pgstore on the schema.
	a, _, aCloser, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close(); _ = aCloser.Close() })

	// The "server B" side: a second pgstore wired into a dataentry App's bridge.
	bStore, _, bCloser, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	app := &App{store: bStore, broker: newEventBroker()}
	app.startStoreEventBridge()
	t.Cleanup(func() {
		app.StopWatching()
		_ = bStore.Close()
		_ = bCloser.Close()
	})

	sse := app.broker.subscribe()
	defer app.broker.unsubscribe(sse)

	// A writes; B's SSE feed must surface a cross-process entity change
	// for the feature type (no id on the wire — TKT-POT9GQ).
	require.NoError(t, a.CreateEntity(ctx, entity.New("FEAT-1", "feature")))
	ev := waitForEntityChange(t, sse, "feature")
	require.Empty(t, ev.Data, "cross-process entity change must carry no id")
}

func dsnWithSchema(t *testing.T, base, schema string) string {
	t.Helper()
	u, err := url.Parse(base)
	require.NoError(t, err)
	q := u.Query()
	q.Set("options", fmt.Sprintf("-c search_path=%s,public", schema))
	u.RawQuery = q.Encode()
	return u.String()
}

func quoteIdent(s string) string { return `"` + s + `"` }
