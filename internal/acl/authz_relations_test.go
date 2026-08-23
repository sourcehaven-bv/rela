package acl

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// relationGrantsPolicy is the motivating outage's acl.yaml, modernized: the
// scheduler may create `taak` entities and update `terugkerend` ones, but the
// edge it must write runs FROM a terugkerend — which the old gate resolved as
// "create on terugkerend", i.e. entity-creation authority it was never meant
// to have. relation_grants: is how that is now expressible.
const relationGrantsPolicy = `
roles:
  scheduler-system:
    read: ["*"]
    create: [taak]
    update: [taak, terugkerend]
    permissions: [create-spawnt]
assignments:
  system:scheduler: scheduler-system
relation_grants:
  spawnt:
    create: create-spawnt
`

// mustDeclarative loads a policy body and builds a resolver over an empty
// graph — these tests exercise global roles and policy config, never
// graph-conferred local roles.
func mustDeclarative(t *testing.T, body string) *Declarative {
	t.Helper()
	d, err := NewDeclarative(mustPolicy(t, body), NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// mustRequest resolves an unattenuated principal (no principal_type, so no
// ceiling applies). Tests that need a ceiling use verifiedClient instead.
func mustRequest(t *testing.T, d *Declarative, user string) *Request {
	t.Helper()
	r, err := d.ForPrincipal(principal.Principal{User: user, Tool: principal.ToolCLI})
	if err != nil {
		t.Fatalf("ForPrincipal(%q): %v", user, err)
	}
	return r
}

func spawnt(fromType string) RelationSubject {
	return RelationSubject{Type: "spawnt", FromType: fromType, FromID: "TERUG-1"}
}

// TestRelationGrant_AllowsEdgeWithoutEntityCreate is the regression test for
// the outage: the scheduler creates the edge, and STILL cannot create entities
// of the source type. Least privilege is the whole point — an assertion that
// only checked the allow would pass for a policy that simply granted
// `create: [terugkerend]`.
func TestRelationGrant_AllowsEdgeWithoutEntityCreate(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, relationGrantsPolicy)
	ctx := context.Background()
	r := mustRequest(t, d, "system:scheduler")

	if got := r.authorizeRelationWrite(ctx, OpCreate, spawnt("terugkerend")); !got.Allow {
		t.Fatalf("relation create denied: %s", got.Reason)
	} else if got.RuleKind != "relation-grant" || got.RuleID != "create-spawnt" {
		t.Errorf("allow attributed to %q/%q, want relation-grant/create-spawnt — "+
			"the audit log cannot otherwise tell this apart from a source-type grant",
			got.RuleKind, got.RuleID)
	}

	if got := r.authorizeEntityWrite(ctx, OpCreate, EntitySubject{Type: "terugkerend"}); got.Allow {
		t.Error("granting the edge also granted entity-create on the source type; " +
			"that is the over-broad authority this feature exists to avoid")
	}
}

// TestRelationGrant_DoesNotLeakToOtherVerbsOrTypes pins that a create grant is
// exactly a create grant.
func TestRelationGrant_DoesNotLeakToOtherVerbsOrTypes(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, relationGrantsPolicy)
	ctx := context.Background()
	r := mustRequest(t, d, "system:scheduler")

	if got := r.authorizeRelationWrite(ctx, OpDelete, spawnt("terugkerend")); got.Allow {
		t.Error("create-spawnt satisfied a DELETE")
	}
	other := RelationSubject{Type: "hoort-bij", FromType: "terugkerend", FromID: "TERUG-1"}
	if got := r.authorizeRelationWrite(ctx, OpCreate, other); got.Allow {
		t.Error("create-spawnt satisfied a different relation type")
	}
}

// TestRelationGrant_RequiresResolvedSourceType is the DR-4 guard.
//
// Four of the five RelationSubject call sites leave FromType empty when the
// source entity is missing or unreadable. That fails closed today only because
// no role lists "". The relation grant keys on the caller-supplied relation
// type, which is ALWAYS populated — so honoring it on an empty FromType would
// silently convert "source unresolvable ⇒ deny" into "⇒ allow", with nothing
// else left to check.
func TestRelationGrant_RequiresResolvedSourceType(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, relationGrantsPolicy)
	ctx := context.Background()
	r := mustRequest(t, d, "system:scheduler")

	// Precondition: with the type resolved, this same principal IS allowed.
	if got := r.authorizeRelationWrite(ctx, OpCreate, spawnt("terugkerend")); !got.Allow {
		t.Fatalf("precondition: resolved source should be allowed: %s", got.Reason)
	}

	for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
		got := r.authorizeRelationWrite(ctx, op, spawnt(""))
		if got.Allow {
			t.Errorf("%s allowed with an unresolved source type — an empty FromType "+
				"is a fail-closed sentinel, not a wildcard", op)
		}
	}
}

// TestRelationGrant_CeilingStillDenies is the critical regression test.
//
// filterTypes deliberately PRESERVES a role's "*" under a pure denial, so the
// RoleDef that roleFor returns still looks permissive under `deny_write: ["*"]`
// — permitsVerb is what actually gates. Any allow source that skips the
// ceiling check escapes it entirely, which would make a ceiling GRANT.
//
// The unrestricted-principal precondition below is load-bearing: without it
// this test would pass vacuously against a principal for whom no ceiling was
// ever active, and would keep passing after a regression.
func TestRelationGrant_CeilingStillDenies(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
roles:
  scheduler-system:
    read: ["*"]
    update: ["*"]
    permissions: [create-spawnt]
assignments:
  alice: scheduler-system
client_baselines:
  readonly:
    applies_to: [readonly-client]
    deny_write: ["*"]
relation_grants:
  spawnt:
    create: create-spawnt
    update: create-spawnt
    delete: create-spawnt
`)
	d, err := NewDeclarative(p, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	ctx := context.Background()

	unrestricted, err := d.ForPrincipal(verifiedClient("alice", "user"))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
		if got := unrestricted.authorizeRelationWrite(ctx, op, spawnt("terugkerend")); !got.Allow {
			t.Fatalf("precondition: unattenuated alice should be allowed %s: %s", op, got.Reason)
		}
	}

	client, err := d.ForPrincipal(verifiedClient("alice", "readonly-client"))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
		got := client.authorizeRelationWrite(ctx, op, spawnt("terugkerend"))
		if got.Allow {
			t.Errorf("%s: relation grant escaped the client ceiling — a ceiling that "+
				"GRANTS violates the only-ever-narrows invariant", op)
		}
		if got.RuleKind != "client-ceiling" {
			t.Errorf("%s: denied by %q, want client-ceiling", op, got.RuleKind)
		}
	}
}

// TestRelationGrant_DelegateGateStillFires pins that a relation grant cannot
// satisfy the delegate-X gate. Validate rejects this policy shape, so this
// exercises the runtime ordering directly — belt and braces, because the two
// defenses fail independently.
func TestRelationGrant_DelegateGateStillFires(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
roles:
  editor:
    read: ["*"]
    create: ["*"]
    permissions: [link-owner]
assignments:
  alice: editor
role_relations:
  owner-of:
    confers: owner
    requires_permission: delegate-ownership
`)
	// Injected post-Validate: the loader rejects this overlap outright.
	p.RelationWriteGrants = map[string]RelationWriteGrant{
		"owner-of": {Create: "link-owner"},
	}
	d, err := NewDeclarative(p, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	r := mustRequest(t, d, "alice")

	got := r.authorizeRelationWrite(context.Background(), OpCreate,
		RelationSubject{Type: "owner-of", FromType: "persoon", FromID: "P-1"})
	if got.Allow {
		t.Fatal("relation grant satisfied the delegate-X gate — a principal could " +
			"hand itself a role-conferring edge without delegate-ownership (RR-7O6Q)")
	}
	if got.RuleKind != "delegate-permission" {
		t.Errorf("denied by %q, want delegate-permission", got.RuleKind)
	}
}

// TestRelationGrant_DenialNamesThePermission pins the observability fix. The
// incident's second root cause was a gate that knew why it said no and said
// something else.
func TestRelationGrant_DenialNamesThePermission(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, `
roles:
  worker:
    read: ["*"]
assignments:
  bob: worker
relation_grants:
  spawnt:
    create: create-spawnt
`)
	r := mustRequest(t, d, "bob")
	got := r.authorizeRelationWrite(context.Background(), OpCreate, spawnt("terugkerend"))
	if got.Allow {
		t.Fatal("bob holds neither the source-type grant nor the permission")
	}
	if !strings.Contains(got.Reason, "create-spawnt") {
		t.Errorf("denial reason %q does not name the permission that would have "+
			"satisfied it; an operator who configured relation_grants gets no hint "+
			"that the closer rule was even consulted", got.Reason)
	}
}

// TestRelationGrant_PermissionCheckRoutesThroughGrantsPermission is the
// companion the ceiling guard cannot provide.
//
// ceilingguard_test.go scans for `policy.Roles[` — the pattern that reaches a
// RoleDef without the clamp. The relation-grant path reads a DIFFERENT map
// (policy.RelationWriteGrants) which that regex cannot see, so the guard is
// blind to it. What makes the read safe is that it yields a permission NAME,
// never a capability: the actual check goes through grantsPermission, which
// applies permitsPermission and roleFor.
//
// This pins that property directly. If someone reworked the relation-grant
// path to decide from the policy map without grantsPermission, the guard would
// stay green and only this test would fail.
func TestRelationGrant_PermissionCheckRoutesThroughGrantsPermission(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
roles:
  writer:
    read: ["*"]
    permissions: [create-spawnt]
assignments:
  alice: writer
client_baselines:
  attenuated:
    applies_to: [limited]
    deny_permissions: [create-spawnt]
relation_grants:
  spawnt:
    create: create-spawnt
`)
	d, err := NewDeclarative(p, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	ctx := context.Background()

	// Precondition: unattenuated, the relation grant allows.
	full, err := d.ForPrincipal(verifiedClient("alice", "user"))
	if err != nil {
		t.Fatal(err)
	}
	if got := full.authorizeRelationWrite(ctx, OpCreate, spawnt("terugkerend")); !got.Allow {
		t.Fatalf("precondition: unattenuated alice should be allowed: %s", got.Reason)
	}

	// A ceiling withholding the PERMISSION (not the verb) must deny. This axis
	// is only reachable via grantsPermission — a direct map read would sail
	// past it, and filterPermissions preserves a "*" under a pure denial just
	// as filterTypes does.
	limited, err := d.ForPrincipal(verifiedClient("alice", "limited"))
	if err != nil {
		t.Fatal(err)
	}
	if got := limited.authorizeRelationWrite(ctx, OpCreate, spawnt("terugkerend")); got.Allow {
		t.Error("the relation grant honored a permission the client ceiling " +
			"withholds — the permission check is not routing through " +
			"grantsPermission")
	}
}
