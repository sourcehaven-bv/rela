package acl

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// membershipPolicy grants `editor` to the group `team`; alice reaches it
// through one member-of hop, so every fresh Request costs a walk.
func membershipPolicy() *Policy {
	return &Policy{
		Roles: map[string]RoleDef{
			"editor": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}},
		},
		Assignments: map[string]string{"team": "editor"},
	}
}

func membershipGraph() *fakeGraph {
	g := newFakeGraph()
	g.add("alice", "member-of", "team")
	return g
}

func writeTicket(op Op) WriteRequest {
	return WriteRequest{Op: op, Subject: EntitySubject{Type: "ticket", ID: "T-1"}}
}

// A ctx that carries the operation's Request must not pay the membership
// walk again per AuthorizeWrite: three verbs on one row are one walk, not
// three (TKT-1U8XYN — this was six membership queries per list row).
func TestAuthorizeWrite_ReusesContextRequest(t *testing.T) {
	t.Parallel()
	g := membershipGraph()
	d := newTestDeclarative(t, membershipPolicy(), g)
	p := aliceDataEntry()
	req, err := d.ForPrincipal(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithRequest(principal.With(context.Background(), p), req)

	for _, verb := range []Op{OpCreate, OpUpdate, OpDelete} {
		if got := d.AuthorizeWrite(ctx, writeTicket(verb)); !got.Allow {
			t.Fatalf("%v: Allow = false, want true: %+v", verb, got)
		}
	}
	if g.outgoingCalls != 2 {
		t.Errorf("membership walk ran %d OutgoingRelations calls, want 2 (one walk: alice, team)", g.outgoingCalls)
	}
}

// Without a bound Request the old behavior stands: each call opens its own
// scope. This is the fallback single-entity handlers and tests rely on.
func TestAuthorizeWrite_NoContextRequestOpensFresh(t *testing.T) {
	t.Parallel()
	g := membershipGraph()
	d := newTestDeclarative(t, membershipPolicy(), g)
	ctx := principal.With(context.Background(), aliceDataEntry())
	d.AuthorizeWrite(ctx, writeTicket(OpCreate))
	d.AuthorizeWrite(ctx, writeTicket(OpUpdate))
	if g.outgoingCalls != 4 {
		t.Errorf("OutgoingRelations calls = %d, want 4 (two fresh walks)", g.outgoingCalls)
	}
}

// A Request bound to a different principal than ctx's (the provisioning
// seam re-stamps ctx) or to a different Declarative (another tenant's
// policy) must NOT be reused — that would evaluate the wrong identity or
// the wrong policy.
func TestAuthorizeWrite_IgnoresForeignContextRequest(t *testing.T) {
	t.Parallel()
	t.Run("different principal", func(t *testing.T) {
		g := membershipGraph()
		d := newTestDeclarative(t, membershipPolicy(), g)
		bobReq, err := d.ForPrincipal(principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithRequest(principal.With(context.Background(), aliceDataEntry()), bobReq)
		if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); !got.Allow {
			t.Fatalf("alice must be evaluated as alice (allowed), got %+v", got)
		}
	})
	t.Run("different declarative", func(t *testing.T) {
		g := membershipGraph()
		d := newTestDeclarative(t, membershipPolicy(), g)
		other := newTestDeclarative(t, &Policy{Roles: map[string]RoleDef{"nobody": {}}}, newFakeGraph())
		otherReq, err := other.ForPrincipal(aliceDataEntry())
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithRequest(principal.With(context.Background(), aliceDataEntry()), otherReq)
		if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); !got.Allow {
			t.Fatalf("d's own policy must decide (allowed), got %+v", got)
		}
		if g.outgoingCalls == 0 {
			t.Error("a foreign Request must not be reused; d had to open its own scope")
		}
	})
}
