package pgstore_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// dsnForSchema returns the test DSN with search_path pinned to schema, so both
// the store's pool and its listener's standalone connection resolve the same
// schema-scoped NOTIFY channel.
func dsnForSchema(t *testing.T, schema string) string {
	t.Helper()
	base := os.Getenv(testDBEnv)
	if base == "" {
		t.Skipf("%s not set; skipping multi-writer tests", testDBEnv)
	}
	u, err := url.Parse(base)
	require.NoError(t, err)
	q := u.Query()
	// libpq options: set search_path for every connection from this DSN.
	q.Set("options", fmt.Sprintf("-c search_path=%s,public", schema))
	u.RawQuery = q.Encode()
	return u.String()
}

// openWriter opens a full pgstore (store + search + listener) against schema,
// the way pgstore.Open does in production but pinned to an isolated test schema.
func openWriter(t *testing.T, schema string) store.Store {
	t.Helper()
	dsn := dsnForSchema(t, schema)
	st, _, closer, err := pgstore.Open(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = st.Close()     // stops the listener
		_ = closer.Close() // closes the pool
	})
	return st
}

// freshFeedSchema creates an isolated, migrated schema for a multi-writer test
// (dropped on cleanup). Unlike newScopedPool it returns the schema name so two
// writers can share it.
func freshFeedSchema(t *testing.T) string {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()
	schema := fmt.Sprintf("relafeed_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+pgQuoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgQuoteIdent(schema)+" CASCADE")
	})

	// Migrate the schema via a short-lived pool pinned to it.
	cfg, err := pgxpool.ParseConfig(dsnForSchema(t, schema))
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pgstore.Migrate(ctx, pool))
	return schema
}

// waitForEvent reads from ch until it finds an event with the given entity ID,
// or fails after timeout.
func waitForEntityEvent(t *testing.T, ch <-chan store.Event, id string, timeout time.Duration) store.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if ev.EntityID == id {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event for entity %q", id)
			return store.Event{}
		}
	}
}

// TestCrossProcessPropagation (AC1): a write committed by writer A is delivered
// to writer B's Subscribe channel, via the LISTEN/NOTIFY feed.
//
// The op assertion accepts either EventEntityCreated (live NOTIFY path) or
// EventEntityUpdated (initial-catch-up path). When b's listener is mid-startup
// (LISTEN issued, first catchUp running) at the moment a.CreateEntity commits,
// the catchUp scan sees FEAT-1 and emits Updated before the live NOTIFY can
// deliver Created. Both paths satisfy the contract — consumers re-snapshot on
// any event — and this test cares about propagation, not which path delivered
// it. See catchUpEvent for the "Updated for re-snapshot" semantics rationale.
func TestCrossProcessPropagation(t *testing.T) {
	schema := freshFeedSchema(t)
	a := openWriter(t, schema)
	b := openWriter(t, schema)

	ch, cancel := b.Subscribe(16)
	defer cancel()

	ctx := context.Background()
	require.NoError(t, a.CreateEntity(ctx, entity.New("FEAT-1", "feature")))

	ev := waitForEntityEvent(t, ch, "FEAT-1", 5*time.Second)
	require.Contains(t,
		[]store.EventOp{store.EventEntityCreated, store.EventEntityUpdated},
		ev.Op,
		"expected Created (live NOTIFY) or Updated (initial catch-up); both satisfy the propagation contract")
	require.Equal(t, "FEAT-1", ev.EntityID)
}

// TestCatchUpRecoversMissedEvents (AC2): a committed write whose live
// notification was MISSED (simulated by inserting a row directly, bypassing the
// Go producer's pg_notify) is still recovered by the seq-watermark catch-up that
// runs on the safety ticker. Uses a short ticker via the test hook.
func TestCatchUpRecoversMissedEvents(t *testing.T) {
	schema := freshFeedSchema(t)
	pgstore.SetCatchUpIntervalForTest(t, 200*time.Millisecond)

	b := openWriter(t, schema)
	ch, cancel := b.Subscribe(16)
	defer cancel()

	// Insert a row DIRECTLY (no pg_notify) so the only way B learns of it is the
	// catch-up query — this isolates the recovery path from the live feed.
	admin := adminConn(t)
	_, err := admin.Exec(context.Background(),
		"INSERT INTO "+pgQuoteIdent(schema)+".entities (id, type) VALUES ('REQ-9', 'requirement')")
	require.NoError(t, err)

	// The ticker catch-up (200ms) should find and emit REQ-9 within a second or two.
	ev := waitForEntityEvent(t, ch, "REQ-9", 5*time.Second)
	require.Equal(t, "REQ-9", ev.EntityID)
}

// TestSelfNotificationFiltered (AC5): the LISTEN/NOTIFY path must not re-deliver
// a writer's OWN writes (those were already emitted in-process), but MUST
// deliver another origin's writes. The origin filter lives in
// handleNotification; this exercises it directly so the assertion is
// deterministic regardless of feed timing. (The seq catch-up is a separate,
// deliberately origin-agnostic re-snapshot path — see TestCatchUp* — and is
// allowed to re-emit recent rows; that's why the guarantee is on the
// notification path, not "exactly once ever".)
func TestSelfNotificationFiltered(t *testing.T) {
	selfPayload := pgstore.FeedPayloadForTest("origin-self", store.EventEntityCreated, "DEC-1")
	require.False(t, pgstore.NotificationEmitsForTest(t, "origin-self", selfPayload),
		"self-origin notification must be filtered (already emitted in-process)")

	remotePayload := pgstore.FeedPayloadForTest("origin-other", store.EventEntityCreated, "FEAT-2")
	require.True(t, pgstore.NotificationEmitsForTest(t, "origin-self", remotePayload),
		"remote-origin notification must be emitted")
}

// TestChannelIsolationAcrossSchemas (AC7): two writers on DIFFERENT schemas in
// the same database do NOT see each other's notifications (schema-scoped
// channel). This is what keeps parallel tests from cross-talking.
func TestChannelIsolationAcrossSchemas(t *testing.T) {
	schemaX := freshFeedSchema(t)
	schemaY := freshFeedSchema(t)
	x := openWriter(t, schemaX)
	y := openWriter(t, schemaY)

	chX, cancelX := x.Subscribe(16)
	defer cancelX()

	ctx := context.Background()
	// Write on Y; X must NOT receive it (different schema => different channel).
	require.NoError(t, y.CreateEntity(ctx, entity.New("FEAT-Y", "feature")))

	select {
	case ev := <-chX:
		if ev.EntityID == "FEAT-Y" {
			t.Fatalf("schema X received schema Y's notification — channel not isolated: %+v", ev)
		}
	case <-time.After(1500 * time.Millisecond):
		// good — isolated
	}
}

// TestInterleavedWritesAllDelivered (AC3): many concurrent writes from A are all
// observed by B (overlap-window catch-up tolerates commit-order skew; duplicates
// are allowed, misses are not).
func TestInterleavedWritesAllDelivered(t *testing.T) {
	schema := freshFeedSchema(t)
	a := openWriter(t, schema)
	b := openWriter(t, schema)

	ch, cancel := b.Subscribe(256)
	defer cancel()

	ctx := context.Background()
	const n = 30
	for i := range n {
		go func(i int) {
			_ = a.CreateEntity(ctx, entity.New(fmt.Sprintf("E-%02d", i), "ticket"))
		}(i)
	}

	seen := make(map[string]bool)
	deadline := time.After(10 * time.Second)
	for len(seen) < n {
		select {
		case ev := <-ch:
			if ev.EntityID != "" {
				seen[ev.EntityID] = true
			}
		case <-deadline:
			t.Fatalf("only saw %d/%d entities before timeout; missing some", len(seen), n)
		}
	}
	require.Len(t, seen, n)
}

// TestListenerReconnects (RR-4GMZD4): kill the listener's backend connection and
// assert a subsequent write still propagates — proving the reconnect +
// re-LISTEN + catch-up path works, not just the happy path.
func TestListenerReconnects(t *testing.T) {
	schema := freshFeedSchema(t)
	pgstore.SetCatchUpIntervalForTest(t, 300*time.Millisecond)
	a := openWriter(t, schema)
	b := openWriter(t, schema)

	ch, cancel := b.Subscribe(16)
	defer cancel()

	// Confirm the feed works once.
	ctx := context.Background()
	require.NoError(t, a.CreateEntity(ctx, entity.New("PRE-1", "ticket")))
	waitForEntityEvent(t, ch, "PRE-1", 5*time.Second)

	// Terminate ALL of B's listening backends (the connections doing LISTEN on
	// B's channel). pg_terminate_backend forces B's listener to error out of
	// WaitForNotification and reconnect.
	admin := adminConn(t)
	_, err := admin.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		 WHERE query LIKE 'LISTEN %' AND pid <> pg_backend_pid()`)
	require.NoError(t, err)

	// After the kill, B must reconnect and still deliver A's next write (via the
	// re-LISTEN live path or the catch-up after reconnect).
	require.NoError(t, a.CreateEntity(ctx, entity.New("POST-1", "ticket")))
	ev := waitForEntityEvent(t, ch, "POST-1", 10*time.Second)
	require.Equal(t, "POST-1", ev.EntityID)
}

// TestMalformedNotificationTriggersCatchUp (RR-11KW9M): a garbage NOTIFY on the
// channel must not be trusted, and the change it (fails to) describe must still
// be recovered promptly via the immediate catch-up handleNotification triggers —
// not only after the 30s ticker.
func TestMalformedNotificationTriggersCatchUp(t *testing.T) {
	schema := freshFeedSchema(t)
	// Long ticker: if recovery happens it MUST be via the parse-failure
	// immediate catch-up, not the safety poll.
	pgstore.SetCatchUpIntervalForTest(t, time.Hour)
	b := openWriter(t, schema)

	ch, cancel := b.Subscribe(16)
	defer cancel()

	ctx := context.Background()
	admin := adminConn(t)

	// Insert a row directly (no valid NOTIFY for it), then fire a GARBAGE
	// notification on B's channel. Parsing fails -> immediate catch-up -> the
	// directly-inserted row is discovered and emitted.
	_, err := admin.Exec(ctx,
		"INSERT INTO "+pgQuoteIdent(schema)+".entities (id, type) VALUES ('GAR-1', 'ticket')")
	require.NoError(t, err)
	_, err = admin.Exec(ctx,
		"SELECT pg_notify('rela_changed_'||$1, 'not-a-valid-payload')", schema)
	require.NoError(t, err)

	ev := waitForEntityEvent(t, ch, "GAR-1", 5*time.Second)
	require.Equal(t, "GAR-1", ev.EntityID)
}

// TestListenerGoroutineExitsOnClose (RR-4GMZD4): opening and closing a store
// must not leak its listener goroutine. goleak.VerifyNone asserts no goroutine
// started by Open is still running after Close + pool close. pgx/pgxpool spin up
// their own background goroutines that outlive a single pool briefly; those are
// ignored by name so the check targets the listener.
func TestListenerGoroutineExitsOnClose(t *testing.T) {
	schema := freshFeedSchema(t)

	dsn := dsnForSchema(t, schema)
	st, _, closer, err := pgstore.Open(context.Background(), dsn)
	require.NoError(t, err)

	// Subscribe + a write so the listener goroutine is fully active before we
	// assert it exits.
	ch, cancel := st.Subscribe(4)
	require.NoError(t, st.CreateEntity(context.Background(), entity.New("TKT-1", "ticket")))
	waitForEntityEvent(t, ch, "TKT-1", 5*time.Second)
	cancel()

	require.NoError(t, st.Close())     // must stop + join the listener goroutine
	require.NoError(t, closer.Close()) // close the pool

	goleak.VerifyNone(t,
		// pgx/pgxpool background goroutines unrelated to our listener.
		goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
		goleak.IgnoreAnyFunction("github.com/jackc/puddle/v2.(*Pool[...]).backgroundHealthCheck"),
	)
}
