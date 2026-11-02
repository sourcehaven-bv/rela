package pgstore_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// notifyRecorder records what a Listen registration receives.
type notifyRecorder struct {
	mu     sync.Mutex
	bodies []string
	gaps   int
}

func (r *notifyRecorder) deliver(b []byte) {
	r.mu.Lock()
	r.bodies = append(r.bodies, string(b))
	r.mu.Unlock()
}

func (r *notifyRecorder) gap() {
	r.mu.Lock()
	r.gaps++
	r.mu.Unlock()
}

func (r *notifyRecorder) snapshot() (bodies []string, gaps int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bodies...), r.gaps
}

// notifyUntil sends body from st until rec receives it. A listener LISTENs
// shortly after Open, so the first sends may find nobody listening yet.
func notifyUntil(t *testing.T, st store.Store, rec *notifyRecorder, body string) {
	t.Helper()
	require.Eventually(t, func() bool {
		require.NoError(t, pgstore.Notify(context.Background(), st, pgstore.ChannelHints, []byte(body)))
		time.Sleep(50 * time.Millisecond)
		bodies, _ := rec.snapshot()
		return slices.Contains(bodies, body)
	}, 10*time.Second, 10*time.Millisecond, "body %q never arrived", body)
}

func TestNotify_ReachesSameSchemaOnly(t *testing.T) {
	schema := freshFeedSchema(t)
	a, b := openWriter(t, schema), openWriter(t, schema)
	foreign := openWriter(t, freshFeedSchema(t))
	ctx := context.Background()

	var recB, recForeign notifyRecorder
	cancelB, err := pgstore.Listen(b, pgstore.ChannelHints, recB.deliver, recB.gap)
	require.NoError(t, err)
	defer cancelB()
	cancelF, err := pgstore.Listen(foreign, pgstore.ChannelHints, recForeign.deliver, recForeign.gap)
	require.NoError(t, err)
	defer cancelF()

	// The separator inside the body survives: only the first one splits.
	body := "hint\x1fwith-separator"
	notifyUntil(t, a, &recB, body)

	// A marker from the foreign schema's own writer proves its listener ran
	// past a's notification without delivering it.
	notifyUntil(t, foreign, &recForeign, "own")
	// Both listeners run now: a's next payload must not reach foreign.
	require.NoError(t, pgstore.Notify(ctx, a, pgstore.ChannelHints, []byte(body)))
	notifyUntil(t, foreign, &recForeign, "own2")
	bodies, _ := recForeign.snapshot()
	for _, got := range bodies {
		require.Contains(t, []string{"own", "own2"}, got, "a payload from another schema was delivered")
	}

	// The entity feed on the same connection is unaffected.
	events, cancel := b.Subscribe(16)
	defer cancel()
	require.NoError(t, a.CreateEntity(ctx, entity.New("NTF-1", "ticket")))
	waitForEntityEvent(t, events, "NTF-1", 10*time.Second)

	// After cancel, nothing more arrives.
	cancelB()
	require.NoError(t, pgstore.Notify(ctx, a, pgstore.ChannelHints, []byte("late")))
	require.NoError(t, pgstore.Notify(ctx, b, pgstore.ChannelHints, []byte("marker")))
	time.Sleep(200 * time.Millisecond)
	bodies, _ = recB.snapshot()
	require.NotContains(t, bodies, "late")
}

func TestNotify_Rejects(t *testing.T) {
	schema := freshFeedSchema(t)
	a := openWriter(t, schema)
	ctx := context.Background()
	require.Error(t, pgstore.Notify(ctx, a, "rela_changed", []byte("x")), "the entity feed is not a notify channel")
	_, err := pgstore.Listen(a, "other", func([]byte) {}, func() {})
	require.Error(t, err)
	_, err = pgstore.Listen(a, pgstore.ChannelHints, nil, func() {})
	require.Error(t, err)
	require.Error(t, pgstore.Notify(ctx, a, pgstore.ChannelHints, []byte(strings.Repeat("x", 8000))))
}

func TestNotify_ListenerUnavailable(t *testing.T) {
	schema := freshFeedSchema(t)
	dsn := dsnForSchema(t, schema)
	ctx := context.Background()
	pool, closer, err := pgstore.NewPool(ctx, dsn)
	require.NoError(t, err)
	defer closer.Close()

	// A store built with New has no listener.
	plain, err := pgstore.New(pool)
	require.NoError(t, err)
	require.ErrorIs(t, pgstore.Notify(ctx, plain, pgstore.ChannelHints, []byte("x")), pgstore.ErrListenerUnavailable)

	// A listener that cannot connect at Open leaves the store without one.
	st, _, err := pgstore.Open(ctx, pool, "host=127.0.0.1 port=1 user=nobody dbname=none sslmode=disable connect_timeout=1")
	require.NoError(t, err)
	defer st.Close()
	_, err = pgstore.Listen(st, pgstore.ChannelHints, func([]byte) {}, func() {})
	require.ErrorIs(t, err, pgstore.ErrListenerUnavailable)
	require.ErrorIs(t, pgstore.Notify(ctx, st, pgstore.ChannelHints, []byte("x")), pgstore.ErrListenerUnavailable)
}

func TestNotify_GapAfterReconnect(t *testing.T) {
	schema := freshFeedSchema(t)
	pgstore.SetCatchUpIntervalForTest(t, 300*time.Millisecond)
	a, b := openWriter(t, schema), openWriter(t, schema)
	ctx := context.Background()

	var rec notifyRecorder
	gapListened := make(chan bool, 4)
	gap := func() {
		rec.gap()
		// The gap must come after LISTEN runs again: a notification sent
		// from inside it is delivered.
		gapListened <- true
	}
	cancel, err := pgstore.Listen(b, pgstore.ChannelHints, rec.deliver, gap)
	require.NoError(t, err)
	defer cancel()
	notifyUntil(t, a, &rec, "before")
	_, gaps := rec.snapshot()
	require.Zero(t, gaps)

	admin := adminConn(t)
	_, err = admin.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		 WHERE query LIKE 'LISTEN %' AND pid <> pg_backend_pid()`)
	require.NoError(t, err)

	select {
	case <-gapListened:
	case <-time.After(15 * time.Second):
		t.Fatal("no gap after the listener reconnected")
	}
	require.NoError(t, pgstore.Notify(ctx, a, pgstore.ChannelHints, []byte("after")))
	notifyUntil(t, a, &rec, "after")
}
