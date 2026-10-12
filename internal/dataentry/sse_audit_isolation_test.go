package dataentry

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestSSE_DoesNotFlowAuditEvents pins the audit-isolation invariant
// documented on [startStoreEventBridge]: a write rejected by ACL must
// not produce any SSE event. The SSE broker is for live-reload of
// committed entity state — never for audit records. Leaking the
// principal-to-entity topology of denies to every subscriber would
// be a serious confidentiality regression, so this test exists to
// fail loudly if a future change wires an audit-sink subscriber into
// the broker.
//
// The test subscribes to the broker, attempts a write that
// [acl.ReadOnlyACL] will deny, and asserts NO event of any kind
// fires during a quiet-window grace period. A denied write must not
// reach the store, so the store-event subscription doesn't fire
// today; this test is the structural guard that keeps it that way.
func TestSSE_DoesNotFlowAuditEvents(t *testing.T) {
	app := buildAppWithACLAndAudit(t, acl.ReadOnlyACL{}, nil)

	// Subscribe to the broker and the change feed BEFORE attempting the
	// write so we see any event that fires as a consequence. The feed is
	// started on the real store, as StartWatching does.
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	app.startStoreEventBridge()
	t.Cleanup(app.StopWatching)
	sub := app.feed.Subscribe("")
	defer sub.Close()

	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice",
		Tool: principal.ToolDataEntry,
	})

	// Attempt a write — ReadOnlyACL denies every write, so the
	// EntityManager returns *acl.ForbiddenError. The audit sink
	// records this internally, but no SSE event must surface.
	_, err := app.entityManager.CreateEntity(ctx, &entity.Entity{
		ID:         "TKT-DENIED",
		Type:       "ticket",
		Properties: map[string]any{"title": "should never broadcast"},
	}, entity.CreateOptions{})
	if err == nil {
		t.Fatal("expected denied write, got success")
	}

	// Grace window: if any future code path wires audit → broker,
	// the event would land here. 100ms is generous for in-process
	// channel sends.
	select {
	case ev := <-ch:
		// Found an event. Even a benign one (e.g. a "refresh")
		// triggered by the deny would be a regression — denies
		// must be invisible to SSE subscribers.
		t.Fatalf("unexpected SSE event after denied write: %+v", ev)
	case <-sub.Wake():
		t.Fatalf("change feed woke after denied write: %+v", sub.Take())
	case <-time.After(100 * time.Millisecond):
		// Expected: zero events.
	}
}

// TestSSE_FeedCarriesTypeOnly pins the shape of an entity change as it
// leaves the store: the change feed reports the entity TYPE and nothing
// else — no id, no audit attribution. Per-type granularity is the
// security boundary (TKT-POT9GQ): the wire never carries an entity id,
// so the feed cannot be a per-entity existence oracle. The handler
// (runSSELoop) gates the type per-connection before rendering it.
func TestSSE_FeedCarriesTypeOnly(t *testing.T) {
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, nil)
	app.startStoreEventBridge()
	t.Cleanup(app.StopWatching)
	sub := app.feed.Subscribe("")
	defer sub.Close()

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if _, err := app.entityManager.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-FEED", Type: "ticket", Properties: map[string]any{"title": "t"},
	}, entity.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-sub.Wake():
	case <-time.After(time.Second):
		t.Fatal("change feed did not wake within 1s")
	}
	got := sub.Take()
	if len(got.Types) != 1 || got.Types[0] != "ticket" {
		t.Fatalf("Take = %+v, want exactly [ticket]", got)
	}
}
