package dataentry

import (
	"context"
	"strings"
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

	// Subscribe to the broker BEFORE attempting the write so we see
	// any event that fires as a consequence.
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)

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
		Properties: map[string]interface{}{"title": "should never broadcast"},
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
		t.Fatalf("unexpected SSE event after denied write: type=%q data=%q",
			ev.Type, ev.Data)
	case <-time.After(100 * time.Millisecond):
		// Expected: zero events.
	}
}

// TestSSE_BroadcastEntityEvent_PayloadShape pins the wire format of
// the events the broker DOES emit: `{type, id}` only — no full entity
// payload, no audit attribution. A reader of this test can verify
// at a glance that the SSE wire shape carries nothing beyond the
// entity marker; ACL-protected content is fetched separately by the
// browser and gates on the entity-read path.
func TestSSE_BroadcastEntityEvent_PayloadShape(t *testing.T) {
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, nil)

	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)

	app.broker.broadcastEntityEvent("created", "ticket", "TKT-001")

	select {
	case ev := <-ch:
		if ev.Type != "entity:created" {
			t.Errorf("Type = %q, want entity:created", ev.Type)
		}
		// The data payload is exactly {type, id} — no Subject,
		// no FromID, no Principal, no role attribution.
		for _, leak := range []string{"subject", "principal", "role", "attribution", "user", "from_id", "to_id"} {
			if strings.Contains(strings.ToLower(ev.Data), leak) {
				t.Errorf("SSE data leaked field %q: %s", leak, ev.Data)
			}
		}
		if !strings.Contains(ev.Data, `"id":"TKT-001"`) || !strings.Contains(ev.Data, `"type":"ticket"`) {
			t.Errorf("SSE data missing expected marker fields: %s", ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no SSE event received within 1s")
	}
}
