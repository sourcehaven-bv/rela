package pgstore

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SetCatchUpIntervalForTest shortens the listener's safety-net catch-up poll so
// tests don't wait the production 30s, restoring it on cleanup. Test-only.
func SetCatchUpIntervalForTest(t *testing.T, d time.Duration) {
	t.Helper()
	prev := catchUpInterval
	catchUpInterval = d
	t.Cleanup(func() { catchUpInterval = prev })
}

// FeedPayloadForTest builds a NOTIFY payload for an entity event with the given
// origin and id, exactly as the producer would. Test-only.
func FeedPayloadForTest(origin string, op store.EventOp, id string) string {
	return notifyPayload(origin, store.Event{Op: op, EntityID: id})
}

// NotificationEmitsForTest runs the listener's handleNotification with a
// listener bound to selfOrigin and the given payload, and reports whether an
// event was emitted to a subscriber. This isolates the origin-filter decision
// from feed/DB timing. Test-only.
func NotificationEmitsForTest(t *testing.T, selfOrigin, payload string) bool {
	t.Helper()
	s := &Store{subscribers: make(map[int]chan store.Event)}
	ch, cancel := s.Subscribe(1)
	defer cancel()

	l := &listener{store: s, originID: selfOrigin}
	l.handleNotification(&pgconn.Notification{Payload: payload})

	select {
	case <-ch:
		return true
	default:
		return false
	}
}
