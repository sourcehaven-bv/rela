package pgstore

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SetCatchUpIntervalForTest shortens the listener's safety-net catch-up poll so
// tests don't wait the production 30s, restoring it on cleanup. A non-positive
// d disables the poll entirely (the listener blocks on the live notification
// alone) — used to isolate the live feed. Test-only. catchUpInterval is an
// atomic, so this is safe vs the listener goroutines that read it concurrently.
func SetCatchUpIntervalForTest(t *testing.T, d time.Duration) {
	t.Helper()
	prev := catchUpInterval.Load()
	catchUpInterval.Store(int64(d))
	t.Cleanup(func() { catchUpInterval.Store(prev) })
}

// SetNotifyDisabledForTest suppresses the producer's pg_notify for the test's
// duration (restored on cleanup), so an ordinary write commits without a live
// notification and the seq-watermark catch-up is the only delivery channel.
// Test-only. notifyDisabled is an atomic; the listener tests that use it run
// sequentially (no t.Parallel).
func SetNotifyDisabledForTest(t *testing.T, disabled bool) {
	t.Helper()
	prev := notifyDisabled.Swap(disabled)
	t.Cleanup(func() { notifyDisabled.Store(prev) })
}

// FeedPayloadForTest builds a NOTIFY payload for an entity event with the given
// origin, schema and id, exactly as the producer would. Test-only.
func FeedPayloadForTest(origin, schema string, op store.EventOp, id string) string {
	return notifyPayload(origin, schema, store.Event{Op: op, EntityID: id})
}

// NotificationEmitsForTest runs the listener's handleNotification with a
// listener bound to (selfOrigin, selfSchema) and the given payload, and reports
// whether an event was emitted to a subscriber. This isolates the filter
// decisions from feed/DB timing. Test-only.
//
// selfSchema is an explicit parameter rather than defaulted: with a shared
// channel, "" == "" would make a caller that forgot it pass vacuously, which is
// the exact failure mode the schema filter exists to prevent.
func NotificationEmitsForTest(t *testing.T, selfOrigin, selfSchema, payload string) bool {
	t.Helper()
	s := &Store{subscribers: make(map[int]chan store.Event)}
	ch, cancel := s.Subscribe(1)
	defer cancel()

	l := &listener{store: s, originID: selfOrigin, schema: selfSchema}
	l.handleNotification(&pgconn.Notification{Payload: payload})

	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// WriteAdvisoryLockKeyForTest exposes the Tx write-serialization advisory
// key so the stress tests can assert it never leaks in pg_locks. Test-only.
const WriteAdvisoryLockKeyForTest = writeAdvisoryLockKey

// MaxStateValueBytesForTest exposes the state-value ceiling so a test can build
// an over-limit payload without restating the constant. Test-only.
const MaxStateValueBytesForTest = maxStateValueBytes

// FeedChannelForTest exposes the shared NOTIFY channel so a test can emit a
// raw notification onto the same channel the listener LISTENs on. Referencing
// the constant (rather than rebuilding the name) means a rename cannot leave a
// test quietly notifying a channel nobody hears. Test-only.
const FeedChannelForTest = feedChannel

// BuildGraphQuerySQLForTest exposes the internal SQL builder so
// explain-plan tests can render the SQL without going through a pgx
// round-trip. Keeps the production surface narrow while letting
// tests pin query shape and verify index usage. Test-only.
func BuildGraphQuerySQLForTest(q store.GraphQuery, countOnly bool) (sqlText string, args []any) {
	return buildGraphQuerySQL(q, countOnly)
}
