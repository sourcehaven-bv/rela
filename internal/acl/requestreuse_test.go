package acl

import (
	"context"
	"errors"
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
	return WriteRequest{Op: op, Subject: NewFacelessEntitySubject("ticket", "T-1")}
}

// A ctx that carries the operation's Request must not pay the membership
// walk again per AuthorizeWrite: three verbs on one row are one walk, not
// three (TKT-1U8XYN — this was six membership queries per list row).
func TestAuthorizeWrite_ReusesContextRequest(t *testing.T) {
	t.Parallel()
	g := membershipGraph()
	d := newTestDeclarative(t, membershipPolicy(), g)
	p := aliceDataEntry()
	req, err := d.ForIdentity(identityFor(t.Context(), t, d, p))
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

// Without a bound Request there is no identity to evaluate. AuthorizeWrite
// denies and does not resolve the principal itself: resolution belongs to
// the request edge.
func TestAuthorizeWrite_NoContextRequestFailsClosed(t *testing.T) {
	t.Parallel()
	g := membershipGraph()
	d := newTestDeclarative(t, membershipPolicy(), g)
	ctx := principal.With(context.Background(), aliceDataEntry())
	if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); got.Allow {
		t.Fatalf("AuthorizeWrite without a bound Request allowed: %+v", got)
	}
	if g.outgoingCalls != 0 {
		t.Errorf("OutgoingRelations calls = %d, want 0 (no lazy resolution)", g.outgoingCalls)
	}
	if _, err := d.RequestFrom(ctx); !errors.Is(err, ErrNoIdentity) {
		t.Errorf("RequestFrom error = %v, want ErrNoIdentity", err)
	}
}

// A Request bound to a different principal than ctx's (the provisioning
// seam re-stamps ctx) or to a different Declarative (another tenant's
// policy) must NOT be used — that would evaluate the wrong identity or
// the wrong policy. The write is denied instead.
func TestAuthorizeWrite_RefusesForeignContextRequest(t *testing.T) {
	t.Parallel()
	t.Run("different principal", func(t *testing.T) {
		t.Parallel()
		d := newTestDeclarative(t, membershipPolicy(), membershipGraph())
		bobReq, err := d.ForIdentity(identityFor(t.Context(), t, d, principal.Principal{User: "bob", Tool: principal.ToolDataEntry}))
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithRequest(principal.With(context.Background(), aliceDataEntry()), bobReq)
		if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); got.Allow {
			t.Fatalf("bob's Request was used for alice: %+v", got)
		}
	})
	t.Run("same user, different verified claims", func(t *testing.T) {
		t.Parallel()
		// Same User and Tool, but the ctx principal carries verified claims
		// (a principal type and scopes — the inputs of a client ceiling) the
		// bound Request was not built from. Using it would evaluate the
		// wrong ceiling.
		d := newTestDeclarative(t, membershipPolicy(), membershipGraph())
		plain := aliceDataEntry()
		wideReq, err := d.ForIdentity(identityFor(t.Context(), t, d, plain))
		if err != nil {
			t.Fatal(err)
		}
		attenuated := principal.VerifiedFrom(plain.User, plain.Tool, principal.Claims{
			PrincipalType: "rela.client", Scopes: []string{"read"},
		})
		ctx := WithRequest(principal.With(context.Background(), attenuated), wideReq)
		if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); got.Allow {
			t.Fatalf("a Request bound to different claims was used: %+v", got)
		}
	})
	t.Run("different declarative", func(t *testing.T) {
		t.Parallel()
		d := newTestDeclarative(t, membershipPolicy(), membershipGraph())
		other := newTestDeclarative(t, &Policy{Roles: map[string]RoleDef{"nobody": {}}}, newFakeGraph())
		otherReq, err := other.ForIdentity(identityFor(t.Context(), t, other, aliceDataEntry()))
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithRequest(principal.With(context.Background(), aliceDataEntry()), otherReq)
		if got := d.AuthorizeWrite(ctx, writeTicket(OpCreate)); got.Allow {
			t.Fatalf("a foreign Request was used: %+v", got)
		}
		if _, err := d.ForIdentity(identityFor(t.Context(), t, other, aliceDataEntry())); !errors.Is(err, ErrForeignIdentity) {
			t.Errorf("ForIdentity(foreign identity) error = %v, want ErrForeignIdentity", err)
		}
		if _, err := d.ForIdentity(Identity{}); !errors.Is(err, ErrNoIdentity) {
			t.Errorf("ForIdentity(zero Identity) error = %v, want ErrNoIdentity", err)
		}
	})
}
