package acl

import (
	"slices"
	"testing"
)

// These tests are IN-PACKAGE on purpose. The compiler is the one step in client
// attenuation that can fail OPEN — every other layer, if buggy, denies too much
// (annoying, visible, safe). A wrong intersection here permits more than the
// operator wrote, silently. So it is tested directly rather than only through
// the end-to-end behavior, where an over-permissive result could be masked by
// the user's own grants happening to be narrow.

func mustPolicy(t *testing.T, body string) *Policy {
	t.Helper()
	p, err := LoadPolicyBytes([]byte(body))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	return p
}

// TestCeilingFor_NoBaselineIsInactive pins AC3 at the compiler. An unrecognized
// or absent principal_type must yield an INACTIVE ceiling, not an empty one:
// "no baseline matched" (unrestricted) and "a baseline permitting nothing" are
// opposite outcomes, and conflating them would either lock out every
// interactive user or silently unrestrict every client.
func TestCeilingFor_NoBaselineIsInactive(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
`)
	for _, pt := range []string{"", "user", "unknown-from-another-idp"} {
		c := p.ceilingFor(pt, nil)
		if c.active {
			t.Errorf("principal_type %q produced an ACTIVE ceiling; want unrestricted", pt)
		}
		// An inactive ceiling must be transparent to every predicate.
		transparent := c.permitsRead("person") &&
			c.permitsVerb(OpUpdate, "person") &&
			c.permitsPermission("history:read")
		if !transparent {
			t.Errorf("principal_type %q: inactive ceiling denied something", pt)
		}
		role := RoleDef{Read: []string{"*"}, Update: []string{"person"}}
		if got := c.clamp(role); !slices.Equal(got.Update, role.Update) {
			t.Errorf("principal_type %q: inactive ceiling clamped a role: %+v", pt, got)
		}
	}
}

// TestClamp_NeverGrants is the core safety property (AC7): a ceiling may only
// ever REMOVE. Whatever the baseline or scopes say, the clamped role can never
// hold a type the original role did not.
func TestClamp_NeverGrants(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket, person, audit-record]
    update: [ticket, person]
    permissions: [history:read, export:bulk]
`)
	c := p.ceilingFor("app", nil)

	// A deliberately narrow role: the ceiling names far more than it holds.
	role := RoleDef{Read: []string{"ticket"}, Update: nil, Permissions: nil}
	got := c.clamp(role)

	if !slices.Equal(got.Read, []string{"ticket"}) {
		t.Errorf("Read = %v, want [ticket] — the ceiling must not add person/audit-record", got.Read)
	}
	if len(got.Update) != 0 {
		t.Errorf("Update = %v, want empty — the ceiling must not grant a verb the role lacks", got.Update)
	}
	if len(got.Permissions) != 0 {
		t.Errorf("Permissions = %v, want empty — the ceiling must not grant permissions", got.Permissions)
	}
}

// TestClamp_WildcardMeetsAllowlist is the fail-open case that matters most. A
// role granting "*" under a closed allowlist must collapse to that allowlist —
// if the wildcard survived, the ceiling would be silently ignored and the
// client would keep everything.
func TestClamp_WildcardMeetsAllowlist(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    update: [ticket]
    permissions: [history:read]
`)
	c := p.ceilingFor("app", nil)
	got := c.clamp(RoleDef{
		Read:        []string{"*"},
		Update:      []string{"*"},
		Permissions: []string{"*"},
	})

	if slices.Contains(got.Read, "*") {
		t.Errorf("Read kept the wildcard (%v) — the ceiling was silently ignored", got.Read)
	}
	if !slices.Equal(got.Read, []string{"ticket"}) {
		t.Errorf("Read = %v, want [ticket]", got.Read)
	}
	if !slices.Equal(got.Update, []string{"ticket"}) {
		t.Errorf("Update = %v, want [ticket]", got.Update)
	}
	if !slices.Equal(got.Permissions, []string{"history:read"}) {
		t.Errorf("Permissions = %v, want [history:read]", got.Permissions)
	}
}

// TestClamp_WildcardMeetsDenial covers the case a plain list cannot express:
// "everything except person". The wildcard is deliberately PRESERVED in the
// clamped role (expanding it against the metamodel is the fail-open risk this
// design avoids), so the denial must be enforced by the match-time predicates.
// If both halves are not present the ceiling leaks.
func TestClamp_WildcardMeetsDenial(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [person]
    deny_update: [person]
`)
	c := p.ceilingFor("app", nil)
	got := c.clamp(RoleDef{Read: []string{"*"}, Update: []string{"*"}})

	// Half one: the list keeps the wildcard (nothing else is expressible).
	if !slices.Contains(got.Read, "*") {
		t.Errorf("Read = %v, want the wildcard preserved", got.Read)
	}
	// Half two: the predicates carry the denial.
	if c.permitsRead("person") {
		t.Error("permitsRead(person) = true; the deny_read is not enforced at match time")
	}
	if !c.permitsRead("ticket") {
		t.Error("permitsRead(ticket) = false; the denial over-reached")
	}
	if c.permitsVerb(OpUpdate, "person") {
		t.Error("permitsVerb(update, person) = true; the deny_update is not enforced")
	}
	if !c.permitsVerb(OpUpdate, "ticket") {
		t.Error("permitsVerb(update, ticket) = false; the denial over-reached")
	}
}

// TestClamp_ExplicitTypesMeetDenial checks the non-wildcard denial path: named
// types are filtered out of the list directly.
func TestClamp_ExplicitTypesMeetDenial(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [person]
`)
	c := p.ceilingFor("app", nil)
	got := c.clamp(RoleDef{Read: []string{"ticket", "person", "feature"}})
	if slices.Contains(got.Read, "person") {
		t.Errorf("Read = %v, still contains the denied type", got.Read)
	}
	if !slices.Equal(got.Read, []string{"ticket", "feature"}) {
		t.Errorf("Read = %v, want [ticket feature]", got.Read)
	}
}

// TestCeiling_OmittedAxisInherits pins the "omitted = inherited" rule that
// keeps a two-line baseline two lines long. A baseline that only hides a field
// must not accidentally restrict reads or writes.
func TestCeiling_OmittedAxisInherits(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	c := p.ceilingFor("app", nil)
	if !c.active {
		t.Fatal("baseline did not match")
	}
	role := RoleDef{
		Read:        []string{"ticket", "person"},
		Update:      []string{"ticket"},
		Permissions: []string{"history:read"},
	}
	got := c.clamp(role)
	if !slices.Equal(got.Read, role.Read) {
		t.Errorf("Read = %v, want unchanged %v — an omitted axis must inherit", got.Read, role.Read)
	}
	if !slices.Equal(got.Update, role.Update) {
		t.Errorf("Update = %v, want unchanged %v", got.Update, role.Update)
	}
	if !slices.Equal(got.Permissions, role.Permissions) {
		t.Errorf("Permissions = %v, want unchanged %v", got.Permissions, role.Permissions)
	}
}

// TestCeiling_ExplicitEmptyPermitsNothing is the counterpart: `read: []` is a
// non-nil empty list and must mean "nothing", not "omitted". YAML distinguishes
// these and so must the compiler, or an operator cannot express a hard lockout.
func TestCeiling_ExplicitEmptyPermitsNothing(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  locked:
    applies_to: [service]
    read: []
`)
	c := p.ceilingFor("service", nil)
	if !c.active {
		t.Fatal("baseline did not match")
	}
	if c.permitsRead("ticket") {
		t.Error("permitsRead(ticket) = true under `read: []`; want nothing permitted")
	}
	got := c.clamp(RoleDef{Read: []string{"ticket", "person"}})
	if len(got.Read) != 0 {
		t.Errorf("Read = %v, want empty", got.Read)
	}
}

// TestScopes_ReopenUnderAllowlist: a scope adds to a closed allowlist.
func TestScopes_ReopenUnderAllowlist(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
scope_grants:
  rela.people.read:
    read: [person]
`)
	base := p.ceilingFor("app", nil)
	if base.permitsRead("person") {
		t.Fatal("precondition: person readable without the scope")
	}
	withScope := p.ceilingFor("app", []string{"rela.people.read"})
	if !withScope.permitsRead("person") {
		t.Error("scope did not re-open person")
	}
	if !withScope.permitsRead("ticket") {
		t.Error("scope removed the baseline's own grant")
	}
}

// TestScopes_ReopenUnderDenial: a scope removes a type from a denial.
func TestScopes_ReopenUnderDenial(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [person, audit-record]
scope_grants:
  rela.people.read:
    read: [person]
`)
	c := p.ceilingFor("app", []string{"rela.people.read"})
	if !c.permitsRead("person") {
		t.Error("scope did not lift the denial on person")
	}
	if c.permitsRead("audit-record") {
		t.Error("scope lifted a denial it did not name")
	}
}

// TestScopes_ReopenUnderWildcardDenial is the "read-only client, except
// tickets" case — the shape most deployments will actually write, and one that
// list-subtraction cannot express: removing "ticket" from a deny list of ["*"]
// leaves ["*"], which still denies everything. Regression test for exactly that
// bug, caught by TestScopes_MoreScopesNeverNarrows during development.
func TestScopes_ReopenUnderWildcardDenial(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
scope_grants:
  rela.tickets.write:
    update: [ticket]
`)
	base := p.ceilingFor("app", nil)
	if base.permitsVerb(OpUpdate, "ticket") {
		t.Fatal("precondition: deny_write did not deny")
	}

	c := p.ceilingFor("app", []string{"rela.tickets.write"})
	if !c.permitsVerb(OpUpdate, "ticket") {
		t.Error("scope did not carve ticket out of a wildcard denial")
	}
	// The carve-out is surgical: it must not lift the wildcard generally, and
	// must not reach verbs the scope did not name.
	if c.permitsVerb(OpUpdate, "person") {
		t.Error("the exception widened beyond the type the scope named")
	}
	if c.permitsVerb(OpDelete, "ticket") {
		t.Error("an update scope also re-opened delete")
	}

	// And it survives the clamp: a role that may update tickets keeps that.
	got := c.clamp(RoleDef{Read: []string{"*"}, Update: []string{"ticket", "person"}})
	if !slices.Contains(got.Update, "ticket") {
		t.Errorf("Update = %v, want ticket preserved by the scope exception", got.Update)
	}
	if slices.Contains(got.Update, "person") {
		t.Errorf("Update = %v, person should stay denied", got.Update)
	}
}

// TestScopes_MoreScopesNeverNarrows is the monotonicity property that makes a
// scoped token safe to reason about: adding a scope can only widen (within the
// ceiling), never restrict. If this inverted, a client could be broken by
// GAINING a permission, which is deeply unintuitive.
func TestScopes_MoreScopesNeverNarrows(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    deny_update: ["*"]
scope_grants:
  a:
    read: [person]
  b:
    update: [ticket]
`)
	types := []string{"ticket", "person", "feature", "audit-record"}
	none := p.ceilingFor("app", nil)
	both := p.ceilingFor("app", []string{"a", "b"})

	for _, ty := range types {
		if none.permitsRead(ty) && !both.permitsRead(ty) {
			t.Errorf("read %q: permitted with no scopes but denied with two", ty)
		}
		if none.permitsVerb(OpUpdate, ty) && !both.permitsVerb(OpUpdate, ty) {
			t.Errorf("update %q: permitted with no scopes but denied with two", ty)
		}
	}
	// And the widening actually happened.
	if !both.permitsRead("person") || !both.permitsVerb(OpUpdate, "ticket") {
		t.Error("scopes did not widen as expected")
	}
}

// TestScopes_OrderIndependent: scopes union, so the order they arrive in (which
// is whatever the IdP put in the claim string) must not change the outcome.
func TestScopes_OrderIndependent(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
scope_grants:
  a:
    read: [person]
  b:
    read: [feature]
`)
	ab := p.ceilingFor("app", []string{"a", "b"})
	ba := p.ceilingFor("app", []string{"b", "a"})
	for _, ty := range []string{"ticket", "person", "feature", "other"} {
		if ab.permitsRead(ty) != ba.permitsRead(ty) {
			t.Errorf("read %q differs by scope order: ab=%v ba=%v",
				ty, ab.permitsRead(ty), ba.permitsRead(ty))
		}
	}
}

// TestScopes_UnknownContributesNothing: an IdP must not be able to invent
// capability by minting a scope name the operator never wrote.
func TestScopes_UnknownContributesNothing(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
`)
	c := p.ceilingFor("app", []string{"rela.admin", "*", "person"})
	for _, ty := range []string{"person", "audit-record", "anything"} {
		if c.permitsRead(ty) {
			t.Errorf("unknown scope granted read on %q", ty)
		}
	}
	if !c.permitsRead("ticket") {
		t.Error("unknown scopes disturbed the baseline")
	}
}

// TestScopes_DoNotMutatePolicy guards a sharing bug: reopen() edits slices that
// originate in the Policy, so a missing clone would let ONE request's scope set
// permanently widen the ceiling for every later request. That is a persistent
// privilege escalation from a single token.
func TestScopes_DoNotMutatePolicy(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    deny_update: [person]
scope_grants:
  wide:
    read: [person, feature, audit-record]
    update: [person]
`)
	// A request carrying the widening scope.
	if c := p.ceilingFor("app", []string{"wide"}); !c.permitsRead("person") {
		t.Fatal("precondition: scope did not widen")
	}
	// A later request with NO scopes must see the original ceiling.
	after := p.ceilingFor("app", nil)
	if after.permitsRead("person") {
		t.Error("a scoped request permanently widened the baseline's read allowlist")
	}
	if after.permitsVerb(OpUpdate, "person") {
		t.Error("a scoped request permanently lifted the baseline's deny_update")
	}
	if got := p.ClientBaselines["apps"].Read; !slices.Equal(got, []string{"ticket"}) {
		t.Errorf("policy read list mutated to %v", got)
	}
}

// TestClamp_DoesNotAliasRoleBackingArray: clamp takes a RoleDef by value but
// its slices are shared with the Policy. Writing through them would corrupt the
// role for every other principal.
func TestClamp_DoesNotAliasRoleBackingArray(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
`)
	c := p.ceilingFor("app", nil)
	original := []string{"*"}
	got := c.clamp(RoleDef{Read: original})
	if len(got.Read) > 0 {
		got.Read[0] = "mutated"
	}
	if original[0] != "*" {
		t.Errorf("clamp wrote through to the caller's slice: %v", original)
	}
	if base := p.ClientBaselines["apps"].Read; !slices.Equal(base, []string{"ticket"}) {
		t.Errorf("clamp mutated the policy's own list: %v", base)
	}
}

// TestPermissions_DenyWithholds pins AC8 at the compiler layer.
func TestPermissions_DenyWithholds(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_permissions: [history:read]
`)
	c := p.ceilingFor("app", nil)
	if c.permitsPermission("history:read") {
		t.Error("deny_permissions did not withhold history:read")
	}
	if !c.permitsPermission("export:bulk") {
		t.Error("deny_permissions withheld a permission it did not name")
	}
	got := c.clamp(RoleDef{Permissions: []string{"history:read", "export:bulk"}})
	if slices.Contains(got.Permissions, "history:read") {
		t.Errorf("Permissions = %v, still holds the withheld permission", got.Permissions)
	}
}

// TestCeiling_DisjointnessMakesSelectionDeterministic: load-time validation
// guarantees one baseline per principal_type, so selection cannot depend on map
// iteration order. Exercised repeatedly because a map-order bug is flaky by
// nature and would otherwise pass most runs.
func TestCeiling_DisjointnessMakesSelectionDeterministic(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
client_baselines:
  alpha:
    applies_to: [app]
    read: [ticket]
  beta:
    applies_to: [pat]
    read: [person]
  gamma:
    applies_to: [service]
    read: [feature]
`)
	for range 50 {
		if c := p.ceilingFor("app", nil); c.name != "alpha" {
			t.Fatalf("principal_type app selected %q, want alpha", c.name)
		}
		if c := p.ceilingFor("pat", nil); c.name != "beta" {
			t.Fatalf("principal_type pat selected %q, want beta", c.name)
		}
	}
}
