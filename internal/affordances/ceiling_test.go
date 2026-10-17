package affordances_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Client attenuation, field axis (TKT-IAC8TX).
//
// These tests pin the ORIGINATING use case: an MCP client connected by an HR
// user must not see person.salary, even though that user can. The distinguishing
// property — and the reason this is not just another `visible:` test — is that
// the SAME acting user gets different results depending on the client they came
// through.

// ceilingWorld wires a person type with a sensitive salary column, an `hr` role
// that can see everything, and whatever attenuation policy the test supplies.
type ceilingWorld struct {
	resolver *affordances.PolicyResolver
	decl     *acl.Declarative
	person   *entity.Entity
}

func newCeilingWorld(t *testing.T, policyYAML string) ceilingWorld {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"person": {
				Properties: map[string]metamodel.PropertyDef{
					"name":   {Type: metamodel.PropertyTypeString},
					"email":  {Type: metamodel.PropertyTypeString},
					"salary": {Type: metamodel.PropertyTypeString},
					"bsn":    {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	p := entity.New("PERS-1", "person")
	p.SetString("name", "Bob")
	p.SetString("email", "bob@example.com")
	p.SetString("salary", "100000")
	p.SetString("bsn", "123456789")
	mustCreate(t, ms, p)

	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, decl)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	return ceilingWorld{resolver: resolver, decl: decl, person: p}
}

// ctxFor builds a request context the way the dataentry middleware does:
// a principal carrying verified claims, PLUS the acl.Request attached to ctx.
// The ceiling is read from that Request — a context with only a principal has
// no ceiling, which is why the middleware ordering is load-bearing.
func (w ceilingWorld) ctxFor(t *testing.T, principalType string, scopes []string) context.Context {
	t.Helper()
	// The acting user is always alice: the whole point of these tests is that
	// the SAME user gets different results through different clients, so
	// varying the user would obscure what is being measured.
	p := principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{
		PrincipalType: principalType,
		Scopes:        scopes,
	})
	ctx := principal.With(context.Background(), p)
	req, err := w.decl.ForIdentity(acltest.IdentityFor(t.Context(), t, w.decl, p))
	if err != nil {
		t.Fatalf("ForIdentity: %v", err)
	}
	return acl.WithRequest(ctx, req)
}

func hidden(t *testing.T, v affordances.FieldVerdicts, field string) bool {
	t.Helper()
	val, present := v.Visible[field]
	return present && !val
}

const hrCanSeeEverything = `
roles:
  hr:
    read: [person]
    update: [person]
assignments:
  alice: hr
`

// TestCeiling_RedactHidesFieldsTheUserCanSee is the headline case. Alice's role
// declares no `visible:` block at all, so she sees every property. The MCP
// acting AS Alice must not see salary or bsn.
func TestCeiling_RedactHidesFieldsTheUserCanSee(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary, bsn]
`)

	// Alice at the browser: nothing hidden.
	direct := w.resolver.FieldVerdicts(w.ctxFor(t, "user", nil), w.person)
	for _, f := range []string{"name", "email", "salary", "bsn"} {
		if hidden(t, direct, f) {
			t.Errorf("interactive user: %s hidden, want visible", f)
		}
	}

	// The MCP acting as Alice: salary and bsn hidden, the rest untouched.
	viaApp := w.resolver.FieldVerdicts(w.ctxFor(t, "app", nil), w.person)
	for _, f := range []string{"salary", "bsn"} {
		if !hidden(t, viaApp, f) {
			t.Errorf("attenuated client: %s visible, want hidden", f)
		}
	}
	for _, f := range []string{"name", "email"} {
		if hidden(t, viaApp, f) {
			t.Errorf("attenuated client: %s hidden, want visible — redact over-reached", f)
		}
	}
}

// TestCeiling_VisibleIsClosedWorld pins AC4's fail-closed half: a `visible:`
// ceiling hides everything it does not name, INCLUDING a property added to the
// metamodel later. That is the property that makes the allowlist spelling worth
// having — nobody has to remember to redact a new sensitive column.
func TestCeiling_VisibleIsClosedWorld(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    visible:
      person: [name]
`)
	v := w.resolver.FieldVerdicts(w.ctxFor(t, "app", nil), w.person)

	if hidden(t, v, "name") {
		t.Error("name hidden, want visible — it is the one field the ceiling names")
	}
	// email is NOT named in the ceiling and NOT named in any redact list. Under
	// an open-world denylist it would leak; under closed-world it must be hidden.
	// This stands in for "a property added to the metamodel tomorrow".
	for _, f := range []string{"email", "salary", "bsn"} {
		if !hidden(t, v, f) {
			t.Errorf("%s visible, want hidden — `visible:` must be closed-world", f)
		}
	}
}

// TestCeiling_RedactIsOpenWorld is the counterpart: the denylist spelling hides
// ONLY what it names, so a new property stays visible. Cheaper to write, weaker
// guarantee — an operator choosing between the two needs both behaviors pinned.
func TestCeiling_RedactIsOpenWorld(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	v := w.resolver.FieldVerdicts(w.ctxFor(t, "app", nil), w.person)
	if !hidden(t, v, "salary") {
		t.Error("salary visible, want hidden")
	}
	if hidden(t, v, "bsn") {
		t.Error("bsn hidden — redact must hide only what it names")
	}
}

// TestCeiling_NeverReGrantsWhatARoleHid is the safety property at the field
// axis. A role that hides a field via its own `visible:` block must keep it
// hidden even when a ceiling names it: a ceiling only ever REMOVES.
//
// This is the bug the dimension.restrictTo intersection exists to prevent —
// implementing it as `allow()` in a loop would union, clearing the role's
// denial and handing the client MORE than its user has.
func TestCeiling_NeverReGrantsWhatARoleHid(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, `
roles:
  hr:
    read: [person]
    visible:
      person:
        - field: name
assignments:
  alice: hr
client_baselines:
  apps:
    applies_to: [app]
    visible:
      person: [name, salary]
`)
	// Alice's role hides salary (closed-world: only name is visible).
	direct := w.resolver.FieldVerdicts(w.ctxFor(t, "user", nil), w.person)
	if !hidden(t, direct, "salary") {
		t.Fatal("precondition: the hr role should already hide salary")
	}

	// The ceiling names salary — but it must not re-grant it.
	viaApp := w.resolver.FieldVerdicts(w.ctxFor(t, "app", nil), w.person)
	if !hidden(t, viaApp, "salary") {
		t.Error("the ceiling RE-GRANTED a field the role hid — a ceiling may only remove")
	}
	if hidden(t, viaApp, "name") {
		t.Error("name hidden; both the role and the ceiling permit it")
	}
}

// TestCeiling_ScopeReopensFields: a scope widens the field ceiling, bounded by
// what the role allows.
func TestCeiling_ScopeReopensFields(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary, bsn]
scope_grants:
  rela.payroll:
    visible:
      person: [salary]
`)
	v := w.resolver.FieldVerdicts(
		w.ctxFor(t, "app", []string{"rela.payroll"}), w.person)

	if hidden(t, v, "salary") {
		t.Error("salary hidden; the rela.payroll scope should re-open it")
	}
	if !hidden(t, v, "bsn") {
		t.Error("bsn visible; the scope named only salary")
	}
}

// TestCeiling_UnmatchedPrincipalTypeIsUnrestricted pins AC3 at the field axis:
// a principal type no baseline covers — including one from a different IdP —
// keeps exactly its user's visibility.
func TestCeiling_UnmatchedPrincipalTypeIsUnrestricted(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	for _, pt := range []string{"", "user", "some-other-idp-type"} {
		v := w.resolver.FieldVerdicts(w.ctxFor(t, pt, nil), w.person)
		if hidden(t, v, "salary") {
			t.Errorf("principal_type %q: salary hidden, want visible (no baseline covers it)", pt)
		}
	}
}

// TestCeiling_NoRequestOnContextFailsClosed is a regression test for a
// fail-open found in review.
//
// applyClientCeiling originally returned early when ctx carried no
// acl.Request, so a principal stamped WITHOUT one kept its user's full field
// visibility: the ceiling silently did not apply. The originating use case is
// an MCP acting as an HR user, which must not see person.salary.
//
// Identity is now resolved only at the request edge. A ctx with a principal
// but no bound Request gets no verdict from the resolver at all: every field
// is hidden, salary and name alike. The guarantee rests on structure, not on
// upstream wiring (visibility.PolicyRedactor.HiddenProperties forwards
// whatever ctx it is handed and binds nothing itself).
func TestCeiling_NoRequestOnContextFailsClosed(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	// A principal, but deliberately NO acl.Request: the shape every other
	// test in this file avoids by going through ctxFor.
	ctx := principal.With(context.Background(),
		principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{
			PrincipalType: "app",
		}))
	v := w.resolver.FieldVerdicts(ctx, w.person)
	for _, field := range []string{"salary", "name"} {
		if !hidden(t, v, field) {
			t.Errorf("%s VISIBLE with no acl.Request on ctx; a missing identity must fail closed", field)
		}
	}

	// No principal at all fails closed the same way.
	v = w.resolver.FieldVerdicts(context.Background(), w.person)
	if !hidden(t, v, "salary") {
		t.Error("salary VISIBLE for an unstamped ctx; a missing identity must fail closed")
	}
}
