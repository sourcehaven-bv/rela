package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// gateFixture builds a store + policy where alice may read concepts globally
// and bob may not read them at all. Both are users; the policy is supplied by
// the caller so each test can vary one dimension.
func gateFixture(t *testing.T, p *Policy) (*Declarative, context.Context) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "alice", Type: "user"}, {ID: "bob", Type: "user"},
		{ID: "CON-1", Type: "concept"}, {ID: "TKT-1", Type: "ticket"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	d, err := NewDeclarative(p, NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d, ctx
}

func requestFor(t *testing.T, d *Declarative, user string) *Request {
	t.Helper()
	req, err := d.ForPrincipal(principal.Principal{User: user, Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	return req
}

// A principal who may not read the traversed-to type must not be able to
// traverse to it. This is the core of the inference channel: the traversal
// returns TICKETS, so nothing about the concept is ever serialized and
// response redaction never fires — only this gate stands between the caller
// and a filter that reveals a hidden entity's properties.
func TestGateTraversal_DeniedWhenTargetTypeUnreadable(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"bob": "reader"},
	})
	_, err := requestFor(t, d, "bob").GateTraversal(ctx, nil, TraversalHop{
		RelationTypes: []string{"caused-by"},
		EntityType:    "concept",
		Props: []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "secret", Scalar: true},
		},
	})
	if !errors.Is(err, ErrTraversalDenied) {
		t.Fatalf("traversal to an unreadable type must be denied, got err=%v", err)
	}
}

// The permitted case still has to WORK, or the gate is just a refusal.
func TestGateTraversal_AllowsReadableTargetAndKeepsTheFilter(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket", "concept"}}},
		Assignments: map[string]string{"alice": "reader"},
	})
	got, err := requestFor(t, d, "alice").GateTraversal(ctx, nil, TraversalHop{
		RelationTypes: []string{"caused-by"},
		EntityType:    "concept",
		Props: []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
		},
	})
	if err != nil {
		t.Fatalf("GateTraversal: %v", err)
	}
	if got.EndpointMatch == nil || got.EndpointMatch.EntityType != "concept" {
		t.Fatalf("endpoint match not composed: %+v", got)
	}
	if len(got.EndpointMatch.Props) != 1 || got.EndpointMatch.Props[0].Value != "open" {
		t.Fatalf("author's filter not preserved: %+v", got.EndpointMatch.Props)
	}
	// The gate must never emit the inheritance expansions: they are anchored
	// on the query's own entity type and re-anchoring them mid-traversal
	// changes who inherits from whom.
	if len(got.EndpointMatch.Props) > 0 && got.InheritThrough != nil {
		t.Fatalf("gate emitted an endpoint-side inheritance expansion: %+v", got)
	}
	if got.EntityInheritThrough != nil {
		t.Fatalf("gate emitted an entity-side inheritance expansion: %+v", got)
	}
}

// A conditional `visible:` grant means the VALUE is a secret even on a row the
// principal may read, so the property must not be filterable — by anyone. The
// refusal is policy-wide, not per-principal: a per-principal answer would make
// the privileged caller an oracle for the unprivileged one.
func TestGateTraversal_RefusesConditionallyVisibleProperty(t *testing.T) {
	p := &Policy{
		Roles: map[string]RoleDef{
			"reader": {
				Read: []string{"ticket", "concept"},
				Visible: map[string][]FieldGrant{
					"concept": {{Field: "salary", When: "has_role(current_user, entity, 'hr')"}},
				},
			},
		},
		Assignments: map[string]string{"alice": "reader"},
	}
	d, ctx := gateFixture(t, p)
	_, err := requestFor(t, d, "alice").GateTraversal(ctx, p, TraversalHop{
		EntityType: "concept",
		Props: []store.PropPredicate{
			{Property: "salary", Op: store.PropEqual, Value: "100000", Scalar: true},
		},
	})
	if err == nil || errors.Is(err, ErrTraversalDenied) {
		t.Fatalf("filtering a conditionally-visible property must be refused with a naming error, got %v", err)
	}

	// An UNCONDITIONALLY visible property on the same type stays filterable —
	// the rule targets conditional grants, not the `visible:` key itself.
	p.Roles["reader"].Visible["concept"] = append(
		p.Roles["reader"].Visible["concept"], FieldGrant{Field: "status"})
	if _, err := requestFor(t, d, "alice").GateTraversal(ctx, p, TraversalHop{
		EntityType: "concept",
		Props:      []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open"}},
	}); err != nil {
		t.Fatalf("unconditional visible property must stay filterable: %v", err)
	}
}

// A hop with no entity type cannot be authorized against anything, so it must
// be refused rather than composed with no read query.
func TestGateTraversal_RequiresAnEntityType(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket", "concept"}}},
		Assignments: map[string]string{"alice": "reader"},
	})
	if _, err := requestFor(t, d, "alice").GateTraversal(ctx, nil, TraversalHop{
		RelationTypes: []string{"caused-by"},
	}); err == nil {
		t.Fatal("a typeless hop must be refused")
	}
}

// Every hop of a chain is gated independently: a chain whose FIRST hop is
// readable and whose SECOND is not must fail closed, or the chain becomes the
// bypass for the gate on its own target.
func TestGateTraversal_GatesEveryHopOfAChain(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket", "concept"}}},
		Assignments: map[string]string{"alice": "reader"},
	})
	_, err := requestFor(t, d, "alice").GateTraversal(ctx, nil, TraversalHop{
		RelationTypes: []string{"caused-by"},
		EntityType:    "concept",
		Next: &TraversalHop{
			RelationTypes: []string{"owned-by"},
			EntityType:    "person", // not readable by this role
		},
	})
	if !errors.Is(err, ErrTraversalDenied) {
		t.Fatalf("an unreadable SECOND hop must deny the whole chain, got err=%v", err)
	}
}

// A face-restricted read cannot be expressed by an EndpointPredicate, so the
// gate refuses rather than traversing through states the principal may not
// read.
func TestGateTraversal_DeniedWhenReadIsFaceRestricted(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles: map[string]RoleDef{
			"reader": {Read: []string{"ticket", "concept@published"}},
		},
		Assignments: map[string]string{"alice": "reader"},
	})
	if _, err := requestFor(t, d, "alice").GateTraversal(ctx, nil, TraversalHop{
		EntityType: "concept",
	}); !errors.Is(err, ErrTraversalDenied) {
		t.Fatalf("a face-restricted read must deny traversal, got err=%v", err)
	}
}

// ConditionallyVisible is the policy-wide question the field gate asks. A nil
// policy answers false so a caller with no policy is not silently refused
// every property.
func TestConditionallyVisible(t *testing.T) {
	var nilPolicy *Policy
	if nilPolicy.ConditionallyVisible("concept", "salary") {
		t.Fatal("nil policy must report false")
	}

	// `visible:` is a CLOSED WORLD per role: a role declaring it hides every
	// field it does not list, with no `when:` anywhere to find. Both that and
	// the conditional case must refuse filtering.
	p := &Policy{Roles: map[string]RoleDef{
		"staff": {Visible: map[string][]FieldGrant{
			"person": {{Field: "name"}, {Field: "secret", When: "x == 1"}},
		}},
		// Declares nothing for person, so it hides nothing there.
		"other": {Visible: map[string][]FieldGrant{"concept": {{Field: "title"}}}},
	}}

	for _, tc := range []struct {
		name, entityType, property string
		want                       bool
	}{
		{"unconditional grant stays filterable", "person", "name", false},
		{"conditional grant is refused", "person", "secret", true},
		{"closed world: unlisted field is refused", "person", "salary", true},
		{"type nobody gates is filterable", "widget", "anything", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.ConditionallyVisible(tc.entityType, tc.property); got != tc.want {
				t.Fatalf("ConditionallyVisible(%q, %q) = %v, want %v",
					tc.entityType, tc.property, got, tc.want)
			}
		})
	}
}

// The gate must not write into the caller's Props backing array.
//
// Today readQuery never populates Query.Props (it constrains rows through
// HasInbound), so the fold appends nothing and the alias is harmless. That is
// exactly why this is pinned: the day a read query grows a property
// constraint, an aliasing append would write the ACL's predicate into the
// caller's spare capacity, and a TraversalHop reused across two gate calls
// would keep the WRONG principal's predicate — a cross-request authorization
// bleed that no existing test would catch.
//
// Asserted structurally (the gate returns a distinct backing array) rather
// than by observing a mutation, so it holds before the latent bug is
// reachable rather than after.
func TestGateTraversal_DoesNotAliasCallerProps(t *testing.T) {
	d, ctx := gateFixture(t, &Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket", "concept"}}},
		Assignments: map[string]string{"alice": "reader"},
	})

	props := make([]store.PropPredicate, 0, 8) // spare capacity: what makes append write in place
	props = append(props, store.PropPredicate{
		Property: "status", Op: store.PropEqual, Value: "open", Scalar: true,
	})
	hop := TraversalHop{RelationTypes: []string{"caused-by"}, EntityType: "concept", Props: props}

	got, err := requestFor(t, d, "alice").GateTraversal(ctx, nil, hop)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if len(got.EndpointMatch.Props) == 0 {
		t.Fatal("expected the author's filter to survive")
	}
	if &got.EndpointMatch.Props[0] == &hop.Props[0] {
		t.Fatal("gate aliased the caller's Props slice; a later fold would write into its spare capacity")
	}
	if len(hop.Props) != 1 || hop.Props[0].Value != "open" {
		t.Fatalf("gate mutated the caller's hop: %+v", hop.Props)
	}
}

// A client attenuated by `client_baselines` must not filter on a field the
// ceiling hides, even when the USER it acts as holds that field
// unconditionally. Without this the traversal reaches further than a plain
// read for the same principal — the ceiling only ever narrows, so refusing
// more for an attenuated client is the correct direction.
func TestGateTraversal_RefusesFieldHiddenByTheClientCeiling(t *testing.T) {
	p := &Policy{
		Roles: map[string]RoleDef{
			// Alice's own grant is UNCONDITIONAL, so the policy-wide field
			// check says "filterable". Only the ceiling knows better.
			"reader": {Read: []string{"ticket", "concept"}},
		},
		Assignments: map[string]string{"alice": "reader"},
		ClientBaselines: map[string]ClientBaseline{
			"app": {
				AppliesTo:   []string{"app"},
				Restriction: Restriction{Redact: map[string][]string{"concept": {"salary"}}},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, ctx := gateFixture(t, p)

	attenuated, err := d.ForPrincipal(principal.VerifiedFrom("alice", principal.ToolDataEntry,
		principal.Claims{PrincipalType: "app"}))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	hop := TraversalHop{
		RelationTypes: []string{"caused-by"},
		EntityType:    "concept",
		Props: []store.PropPredicate{
			{Property: "salary", Op: store.PropEqual, Value: "250000", Scalar: true},
		},
	}
	if _, err := attenuated.GateTraversal(ctx, p, hop); err == nil {
		t.Fatal("an attenuated client must not filter on a ceiling-redacted field")
	}

	// The same hop through the UNATTENUATED user is allowed: the ceiling is
	// what refuses, not the property itself.
	if _, err := requestFor(t, d, "alice").GateTraversal(ctx, p, hop); err != nil {
		t.Fatalf("the unattenuated user must still filter on it: %v", err)
	}
}
