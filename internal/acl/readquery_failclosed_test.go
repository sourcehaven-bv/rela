package acl

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestReadQuery_FailsClosedOnEmptyMemberSet pins the fail-closed
// behavior of readQuery when the principal's expanded member set is
// empty.
//
// This guards a load-bearing interaction with store.GraphQuery: an
// EMPTY RelationPredicate.Endpoints means "ANY endpoint" — the widening
// that makes absence queries expressible — so composing the read gate
// from an empty member set would silently degrade it from "entities
// reachable from ME" to "entities reachable from ANYONE". That is a read
// bypass, not a narrowing.
//
// The test constructs a Request DIRECTLY rather than going through
// ForPrincipal, because ForPrincipal rejects the unstamped principal
// that produces an empty member set (walkMembers returns nil for
// User == ""). That upstream rejection is what keeps this unreachable
// in production today — and is exactly why the invariant needs pinning
// here: a future anonymous-read mode, or a system principal that skips
// ForPrincipal, would otherwise open every row with no test going red.
func TestReadQuery_FailsClosedOnEmptyMemberSet(t *testing.T) {
	t.Parallel()

	// A policy that grants read on `document` only via a role-relation:
	// the shape that composes a HasInbound predicate from the member
	// set, which is what the guard protects. A role granting read
	// globally would short-circuit to AllowAll and never reach it.
	p := &Policy{
		Roles: map[string]RoleDef{
			"editor": {Read: []string{"document"}},
		},
		RoleRelations: map[string]RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
	}
	d := newTestDeclarative(t, p, newFakeGraph())

	req := &Request{d: d, principal: principal.Principal{}} // unstamped → no members

	got := req.readQuery(context.Background(), "document")

	if !got.DenyAll {
		t.Errorf("readQuery with an empty member set: DenyAll = false, want true\n"+
			"  AllowAll=%v Query=%+v\n"+
			"  an empty Endpoints means ANY endpoint, so this would expose every "+
			"document reachable from anyone", got.AllowAll, got.Query)
	}
	if got.Query != nil {
		t.Errorf("readQuery with an empty member set returned a Query (%+v); "+
			"want DenyAll with no query", got.Query)
	}
}
