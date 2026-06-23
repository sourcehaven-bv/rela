package acl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// testPolicy is the fixture used across declarative tests. Mirrors the
// example operators see in docs/security.md: three real roles + the
// built-in everyone role, with one role-relation gated by a delegate
// permission.
func testPolicy() *acl.Policy {
	return &acl.Policy{
		UserEntityType: "person",
		Roles: map[string]acl.RoleDef{
			"admin": {
				Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"},
				Read:        []string{"*"},
				Permissions: []string{"delegate-admin", "delegate-contributor", "delegate-reviewer"},
			},
			"contributor": {
				Create: []string{"ticket", "concept"}, Update: []string{"ticket", "concept"}, Delete: []string{"ticket", "concept"},
				Read:        []string{"*"},
				Permissions: []string{"delegate-reviewer"},
			},
			"reviewer": {
				Create: []string{"review-response"}, Update: []string{"review-response"}, Delete: []string{"review-response"},
				Read: []string{"*"},
			},
			acl.EveryoneRole: {
				Read: []string{"*"},
			},
		},
		Assignments: map[string]string{
			"jeroen": "admin",
			"alice":  "contributor",
			"bob":    "reviewer",
		},
		RoleRelations: map[string]acl.RoleRelationDef{
			"ticket-owner": {
				Confers:            "contributor",
				RequiresPermission: "delegate-contributor",
			},
			"open-relation": {
				Confers: "contributor", // no requires_permission — delegate gate disabled
			},
		},
	}
}

// newACL builds a Declarative for tests that don't exercise group
// expansion (the resolver still needs a non-nil Graph; [acl.NullGraph]
// returns empty for every probe). Production wiring supplies a
// store-backed graph.
func newACL(t *testing.T, p *acl.Policy) *acl.Declarative {
	t.Helper()
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	return d
}

// ctxAs returns a context with the given principal user attached.
func ctxAs(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
}

// entityCreate builds the canonical WriteRequest for an entity create
// of the given type. Subject is required; tests for the type-level
// grant check fold the create case so the entity ID is empty.
func entityCreate(etype string) acl.WriteRequest {
	return acl.WriteRequest{Op: acl.OpCreate, Subject: acl.EntitySubject{Type: etype}}
}

// relCreate builds the canonical WriteRequest for a relation create
// from a fromType entity via relType. v1 evaluates only FromType; To*
// fields are unused by the resolver today (see RelationSubject doc).
func relCreate(fromType, relType string) acl.WriteRequest {
	return acl.WriteRequest{
		Op: acl.OpCreate,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: fromType,
		},
	}
}

// AC2.2: a role's `write` list grants creation of that type.
func TestAuthorizeWrite_RoleGrantsType_Allows(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	got := d.AuthorizeWrite(ctxAs("alice"), entityCreate("ticket"))
	if !got.Allow {
		t.Fatalf("Allow = false, want true. Decision = %+v", got)
	}
	if got.RuleKind != "role-grant" {
		t.Errorf("RuleKind = %q, want %q", got.RuleKind, "role-grant")
	}
	if got.RuleID != "contributor" {
		t.Errorf("RuleID = %q, want %q", got.RuleID, "contributor")
	}
}

// AC2.3: no role grants → structured deny.
func TestAuthorizeWrite_NoRoleGrants_Denies(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	got := d.AuthorizeWrite(ctxAs("bob"), entityCreate("ticket"))
	if got.Allow {
		t.Fatalf("Allow = true, want false (reviewer has no write on ticket). Decision = %+v", got)
	}
	if got.RuleKind != "role-grant" {
		t.Errorf("RuleKind = %q, want %q", got.RuleKind, "role-grant")
	}
	if got.RuleID != "-" {
		t.Errorf("RuleID = %q, want %q", got.RuleID, "-")
	}
	wantReason := `no role grants create on type "ticket"`
	if got.Reason != wantReason {
		t.Errorf("Reason = %q, want %q", got.Reason, wantReason)
	}
}

// AC2.4: wildcard `*` grants any type.
func TestAuthorizeWrite_WildcardRole_Allows(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	for _, etype := range []string{"ticket", "concept", "person", "made-up-type"} {
		got := d.AuthorizeWrite(ctxAs("jeroen"), entityCreate(etype))
		if !got.Allow {
			t.Errorf("admin denied write on %q: %+v", etype, got)
		}
		if got.RuleID != "admin" {
			t.Errorf("RuleID = %q, want %q (wildcard match should attribute to admin)", got.RuleID, "admin")
		}
	}
}

// AC2.5: writing a role-relation that requires a permission the
// principal doesn't hold → delegate-permission deny.
func TestAuthorizeWrite_RoleRelation_DelegatePermissionMissing_Denies(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	// alice (contributor) holds delegate-reviewer but not
	// delegate-contributor, so she cannot grant the contributor role
	// via the `ticket-owner` relation.
	got := d.AuthorizeWrite(ctxAs("alice"), relCreate("ticket", "ticket-owner"))
	if got.Allow {
		t.Fatalf("Allow = true, want false. Decision = %+v", got)
	}
	if got.RuleKind != "delegate-permission" {
		t.Errorf("RuleKind = %q, want %q", got.RuleKind, "delegate-permission")
	}
	if got.RuleID != "delegate-contributor" {
		t.Errorf("RuleID = %q, want %q", got.RuleID, "delegate-contributor")
	}
}

// AC2.5: writing the same role-relation with the required permission
// proceeds to the type-level write check and (here) succeeds.
func TestAuthorizeWrite_RoleRelation_DelegatePermissionHeld_Allows(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	// jeroen (admin) holds delegate-contributor AND can write any
	// entity type via the `*` wildcard, so this allows.
	got := d.AuthorizeWrite(ctxAs("jeroen"), relCreate("ticket", "ticket-owner"))
	if !got.Allow {
		t.Fatalf("Allow = false, want true (admin holds delegate-contributor). Decision = %+v", got)
	}
}

// AC2.5: a role-relation declared *without* requires_permission skips
// the delegate gate — any principal who can write the source entity
// can write the relation.
func TestAuthorizeWrite_RoleRelation_NoDelegateRequired_Allows(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	got := d.AuthorizeWrite(ctxAs("alice"), relCreate("ticket", "open-relation"))
	if !got.Allow {
		t.Fatalf("Allow = false, want true (open-relation has no requires_permission). Decision = %+v", got)
	}
}

// AC2.6: a principal with no Assignments entry inherits the `everyone`
// role's capabilities.
func TestAuthorizeWrite_UnknownPrincipal_GetsDefaultRole(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	// `everyone` has no write entries — every write is denied, but the
	// deny RuleKind is role-grant (not "no roles at all"), proving the
	// everyone role was consulted.
	got := d.AuthorizeWrite(ctxAs("carol"), entityCreate("ticket"))
	if got.Allow {
		t.Fatalf("Allow = true, want false (default has no writes). Decision = %+v", got)
	}
	if got.RuleKind != "role-grant" {
		t.Errorf("RuleKind = %q, want %q", got.RuleKind, "role-grant")
	}
}

// AC2.6 + AC2.7: when the everyone role grants writes, an unknown
// principal gets them via it. RuleID surfaces "everyone".
func TestAuthorizeWrite_DefaultRoleGrantsWrites(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	policy.Roles[acl.EveryoneRole] = acl.RoleDef{
		Create: []string{"comment"}, Update: []string{"comment"}, Delete: []string{"comment"},
		Read: []string{"*"},
	}
	d := newACL(t, policy)

	got := d.AuthorizeWrite(ctxAs("carol"), entityCreate("comment"))
	if !got.Allow {
		t.Fatalf("Allow = false, want true. Decision = %+v", got)
	}
	if got.RuleID != acl.EveryoneRole {
		t.Errorf("RuleID = %q, want %q", got.RuleID, acl.EveryoneRole)
	}
}

// AC2.7: a principal with explicit role X *plus* the everyone role
// gets the union of writes. The explicit role takes priority for
// RuleID when it covers the type.
func TestAuthorizeWrite_MultipleRoles_Unions(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	// Make everyone also grant writes on a type the explicit role
	// doesn't cover, so we can prove the union.
	policy.Roles[acl.EveryoneRole] = acl.RoleDef{
		Create: []string{"comment"}, Update: []string{"comment"}, Delete: []string{"comment"},
		Read: []string{"*"},
	}
	d := newACL(t, policy)

	// bob is reviewer (writes review-response). With everyone also
	// granting "comment", bob should be able to write both.
	for _, tc := range []struct {
		etype  string
		wantID string
	}{
		{"review-response", "reviewer"}, // explicit role wins
		{"comment", acl.EveryoneRole},   // only everyone covers this
	} {
		got := d.AuthorizeWrite(ctxAs("bob"), entityCreate(tc.etype))
		if !got.Allow {
			t.Errorf("bob denied on %q: %+v", tc.etype, got)
			continue
		}
		if got.RuleID != tc.wantID {
			t.Errorf("RuleID for %q = %q, want %q", tc.etype, got.RuleID, tc.wantID)
		}
	}
}

// Negative test: an Assignments entry referencing an undefined role
// is silently ignored — the principal falls through to default only.
func TestAuthorizeWrite_AssignmentToUndefinedRole_DropsToDefault(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	policy.Assignments["typo"] = "contribtuor" // misspelled role name
	d := newACL(t, policy)

	// the everyone role has no writes → deny on ticket. If the bad
	// assignment leaked through, RuleID would be "contribtuor" (or
	// the eval would Allow if we resolved the typo).
	got := d.AuthorizeWrite(ctxAs("typo"), entityCreate("ticket"))
	if got.Allow {
		t.Fatalf("Allow = true, want false. Decision = %+v", got)
	}
	if got.RuleKind != "role-grant" || got.RuleID != "-" {
		t.Errorf("Decision = %+v, want role-grant/- (undefined role should be dropped)", got)
	}
}

// RR-X1TE: a WriteRequest with nil Subject is a programmer error and
// must panic. The legacy "fall through to global-roles-only" path
// that briefly existed during the v0→v1 migration was removed because
// it bypassed the unstamped-principal check.
func TestAuthorizeWrite_NilSubject_Panics(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on nil Subject; got none")
		}
	}()
	_ = d.AuthorizeWrite(ctxAs("alice"), acl.WriteRequest{Op: acl.OpCreate})
}

// Returns ForbiddenError via Decision propagation: any deny can be
// wrapped at the manager boundary into the same *ForbiddenError shape
// PR 1 ships. Sanity-check the round trip.
func TestAuthorizeWrite_DenyConvertsToForbiddenError(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())

	dec := d.AuthorizeWrite(ctxAs("bob"), entityCreate("ticket"))
	if dec.Allow {
		t.Fatal("expected deny")
	}
	err := &acl.ForbiddenError{Decision: dec}
	if !errors.Is(err, acl.ErrForbidden) {
		t.Errorf("errors.Is(_, ErrForbidden) = false on wrapped deny Decision")
	}
}

// Unstamped principal (User="" or "unknown") → structured deny with
// reason naming the ErrUnstampedPrincipal sentinel. This is the
// hard-fail RR-X1TE protects: production code that forgets to stamp
// identity gets a clean deny instead of silently picking up
// Assignments["unknown"] or "everyone".
func TestAuthorizeWrite_UnstampedPrincipal_Denies(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())
	ctx := principal.With(context.Background(), principal.Principal{User: "unknown", Tool: principal.ToolCLI})

	got := d.AuthorizeWrite(ctx, entityCreate("ticket"))
	if got.Allow {
		t.Fatalf("Allow = true on unstamped principal; want deny. Decision = %+v", got)
	}
	if got.RuleKind != "role-grant" || got.RuleID != "-" {
		t.Errorf("Decision = %+v, want role-grant/-", got)
	}
	if got.Reason == "" {
		t.Errorf("Reason is empty; expected mention of ErrUnstampedPrincipal")
	}
}
