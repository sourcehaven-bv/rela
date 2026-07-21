package acl_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Asserted-role granting (TKT-RP3X3Q): a role claim from a verified identity
// assertion, mapped to a declared role through the operator's allowlist.
//
// These tests build a Declarative directly rather than through World, because
// World's principalFor deliberately constructs a plain (non-assertion)
// Principal — the whole point of principal.Verified is that assertion claims
// cannot be set any other way.

const assertedPolicy = `
roles:
  editor:
    read: [ticket]
    create: [ticket]
    update: [ticket]
  auditor:
    read: [ticket]
asserted_role_assignments:
  admin: editor
  compliance: [editor, auditor]
`

// assertedWorld builds a Declarative over an empty store with the given policy.
// The store is empty on purpose: an asserted role must grant without the
// principal existing as an entity, which is exactly the SSO-provisioned-user
// case (see TKT-0C3II2 for the provisioning follow-up).
func assertedWorld(t *testing.T, policyYAML string) *acl.Declarative {
	t.Helper()
	p, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(st)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// verified builds a Principal carrying the given asserted role claims.
func verified(sub string, roles ...string) principal.Principal {
	return principal.Verified(sub, principal.ToolDataEntry, "org_acme", "acme", roles)
}

// globalRoles returns the effective global role names for p, sorted-insensitive
// membership testing via the returned map.
func globalRoles(t *testing.T, d *acl.Declarative, p principal.Principal) map[string]acl.Source {
	t.Helper()
	req, err := d.ForPrincipal(p)
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	out := map[string]acl.Source{}
	for _, a := range req.Globals(context.Background()).Attributions {
		out[a.Role] = a.Source
	}
	return out
}

func TestAssertedRoles_ClaimGrantsMappedRole(t *testing.T) {
	t.Parallel()
	d := assertedWorld(t, assertedPolicy)

	roles := globalRoles(t, d, verified("usr_1", "admin"))

	src, ok := roles["editor"]
	if !ok {
		t.Fatalf("claim %q did not grant role %q; got %v", "admin", "editor", roles)
	}
	if src.Kind != acl.SourceAsserted {
		t.Errorf("Source.Kind = %v, want SourceAsserted", src.Kind)
	}
	if src.Claim != "admin" {
		t.Errorf("Source.Claim = %q, want %q", src.Claim, "admin")
	}
}

func TestAssertedRoles_ClaimGrantsSeveralRoles(t *testing.T) {
	t.Parallel()
	// The list form: one claim, many roles. This is why the policy value is a
	// RoleList rather than a bare string.
	d := assertedWorld(t, assertedPolicy)

	roles := globalRoles(t, d, verified("usr_1", "compliance"))

	for _, want := range []string{"editor", "auditor"} {
		if _, ok := roles[want]; !ok {
			t.Errorf("claim %q did not grant role %q; got %v", "compliance", want, roles)
		}
	}
}

func TestAssertedRoles_MultipleClaimsAccumulate(t *testing.T) {
	t.Parallel()
	// A Pratique user routinely holds several roles; each maps independently.
	d := assertedWorld(t, `
roles:
  editor:
    read: [ticket]
  auditor:
    read: [ticket]
asserted_role_assignments:
  admin: editor
  billing: auditor
`)

	roles := globalRoles(t, d, verified("usr_1", "admin", "billing"))

	if len(roles) != 2 {
		t.Errorf("got %d roles, want 2: %v", len(roles), roles)
	}
}

func TestAssertedRoles_GrantsWriteAccess(t *testing.T) {
	t.Parallel()
	// End-to-end through the actual authorization decision, not just the
	// attribution set — a role that resolves but doesn't authorize is useless.
	d := assertedWorld(t, assertedPolicy)
	req, err := d.ForPrincipal(verified("usr_1", "admin"))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	got := req.AuthorizeWrite(context.Background(), acl.WriteRequest{
		Op:      acl.OpCreate,
		Subject: acl.EntitySubject{Type: "ticket", ID: "TKT-1"},
	})
	if !got.Allow {
		t.Errorf("asserted editor denied create on ticket: %+v", got)
	}
}

func TestAssertedRoles_UndeclaredRoleDroppedSilently(t *testing.T) {
	t.Parallel()
	// Mirrors the Assignments guard: a mapping to a role the policy never
	// declared is dropped at resolution rather than erroring, so a stale
	// mapping cannot brick every request.
	d := assertedWorld(t, `
roles:
  editor:
    read: [ticket]
asserted_role_assignments:
  admin: no-such-role
`)

	roles := globalRoles(t, d, verified("usr_1", "admin"))

	if len(roles) != 0 {
		t.Errorf("undeclared role was granted: %v", roles)
	}
}

func TestAssertedRoles_UnmappedClaimGrantsNothing(t *testing.T) {
	t.Parallel()
	d := assertedWorld(t, assertedPolicy)

	roles := globalRoles(t, d, verified("usr_1", "some-claim-we-never-mapped"))

	if len(roles) != 0 {
		t.Errorf("unmapped claim granted roles: %v", roles)
	}
}

func TestAssertedRoles_NoClaimsGrantsNothing(t *testing.T) {
	t.Parallel()
	// The header/loopback shape: a valid principal with no assertion at all
	// must behave exactly as before this feature existed.
	d := assertedWorld(t, assertedPolicy)

	for _, tc := range []struct {
		name string
		p    principal.Principal
	}{
		{"verified with empty roles", verified("usr_1")},
		{"plain principal", principal.Principal{User: "alice", Tool: principal.ToolDataEntry}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if roles := globalRoles(t, d, tc.p); len(roles) != 0 {
				t.Errorf("got roles %v, want none", roles)
			}
		})
	}
}

func TestAssertedRoles_ClaimIsNotAnEntityID(t *testing.T) {
	t.Parallel()
	// The security property behind keeping asserted_role_assignments separate
	// from assignments: a claim value that happens to equal an entity ID in
	// `assignments` must NOT pick up that entity's role. Colliding namespaces
	// would be a silent cross-dimension grant.
	d := assertedWorld(t, `
roles:
  editor:
    read: [ticket]
  superuser:
    read: ["*"]
assignments:
  PERS-alice: superuser
asserted_role_assignments:
  admin: editor
`)

	// A token claiming the literal entity ID must not become that entity.
	roles := globalRoles(t, d, verified("usr_1", "PERS-alice"))

	if _, leaked := roles["superuser"]; leaked {
		t.Error("claim value matched an assignments key — namespaces collided, " +
			"which is a privilege-escalation path")
	}
	if len(roles) != 0 {
		t.Errorf("got roles %v, want none", roles)
	}
}

func TestAssertedRoles_ClaimMatchingIsExactAfterTrim(t *testing.T) {
	t.Parallel()
	d := assertedWorld(t, assertedPolicy)

	for _, tc := range []struct {
		name, claim string
		wantGrant   bool
	}{
		{"exact", "admin", true},
		{"leading and trailing space trimmed", "  admin  ", true},
		{"different case does not match", "Admin", false},
		{"substring does not match", "admi", false},
		{"superstring does not match", "administrator", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			roles := globalRoles(t, d, verified("usr_1", tc.claim))
			_, granted := roles["editor"]
			if granted != tc.wantGrant {
				t.Errorf("claim %q granted=%v, want %v", tc.claim, granted, tc.wantGrant)
			}
		})
	}
}

func TestAssertedRoles_UnmatchedPrincipalKeepsAssertedRoles(t *testing.T) {
	t.Parallel()
	// AC10. A verified principal whose subject resolves to no user entity keeps
	// its asserted grants — this is the SSO-provisioned user's first request.
	//
	// It is ALSO a regression guard on isUnstamped: that gate keys on User/Tool
	// and must stay untouched by this feature. Relaxing it would weaken the
	// fail-closed check for every entry point, not just this one.
	d := assertedWorld(t, `
user_entity_type: person
principal_property: email
roles:
  editor:
    read: [ticket]
asserted_role_assignments:
  admin: editor
`)

	// Empty store: no person entity has this email.
	roles := globalRoles(t, d, verified("nobody@example.com", "admin"))

	if _, ok := roles["editor"]; !ok {
		t.Errorf("unmatched verified principal lost its asserted roles: %v", roles)
	}
}

func TestAssertedRoles_UnstampedPrincipalStillRejected(t *testing.T) {
	t.Parallel()
	// The other side of the AC10 guard: asserted roles must NOT be a way to
	// smuggle an identity-less principal past the fail-closed gate.
	d := assertedWorld(t, assertedPolicy)

	for _, tc := range []struct {
		name string
		p    principal.Principal
	}{
		{"blank tool", principal.Verified("usr_1", "", "org", "slug", []string{"admin"})},
		{"blank user", principal.Verified("", principal.ToolDataEntry, "org", "slug", []string{"admin"})},
		{"unknown user", principal.Verified("unknown", principal.ToolDataEntry, "org", "slug", []string{"admin"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := d.ForPrincipal(tc.p); err == nil {
				t.Error("ForPrincipal accepted an unstamped principal carrying asserted roles")
			}
		})
	}
}

func TestAssertedRoles_OrgIsNotEvaluated(t *testing.T) {
	t.Parallel()
	// Pins the ABSENCE of org enforcement so a later reader cannot mistake the
	// omission for an oversight — and so this fails loudly the day someone adds
	// half an org check. Org is carried for audit attribution ONLY; a principal
	// in org A holding a role sees every entity that role grants, in every org.
	// Enforcement is a separate ticket by explicit decision.
	d := assertedWorld(t, assertedPolicy)

	orgA := principal.Verified("usr_1", principal.ToolDataEntry, "org_a", "a", []string{"admin"})
	orgB := principal.Verified("usr_1", principal.ToolDataEntry, "org_b", "b", []string{"admin"})

	rolesA, rolesB := globalRoles(t, d, orgA), globalRoles(t, d, orgB)

	if len(rolesA) != len(rolesB) {
		t.Fatalf("org changed the role set: %v vs %v", rolesA, rolesB)
	}
	for role, srcA := range rolesA {
		srcB, ok := rolesB[role]
		if !ok {
			t.Errorf("role %q granted in org_a but not org_b", role)
			continue
		}
		if srcA != srcB {
			t.Errorf("role %q source differs by org: %v vs %v", role, srcA, srcB)
		}
	}
}

// ---- Policy validation --------------------------------------------------

func TestAssertedRoles_PolicyAcceptsScalarAndList(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor: {read: [ticket]}
  auditor: {read: [ticket]}
asserted_role_assignments:
  scalar: editor
  list: [editor, auditor]
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	if got := p.AssertedRoles["scalar"]; len(got) != 1 || got[0] != "editor" {
		t.Errorf("scalar form = %v, want [editor]", got)
	}
	if got := p.AssertedRoles["list"]; len(got) != 2 {
		t.Errorf("list form = %v, want 2 entries", got)
	}
}

func TestAssertedRoles_PolicyRejectsBlankClaimKey(t *testing.T) {
	t.Parallel()
	// A blank key can never match (matching is exact after trim), so the
	// mapping would be silently inert — the failure an operator is least
	// likely to notice.
	for _, tc := range []struct{ name, key string }{
		{"empty", `"": editor`},
		{"whitespace", `"   ": editor`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := acl.LoadPolicyBytes([]byte(
				"roles:\n  editor: {read: [ticket]}\nasserted_role_assignments:\n  " + tc.key + "\n"))
			if err == nil {
				t.Fatal("blank claim key accepted")
			}
			if !strings.Contains(err.Error(), "asserted_role_assignments") {
				t.Errorf("error does not name the offending key: %v", err)
			}
		})
	}
}

func TestAssertedRoles_PolicyRejectsEveryoneAsTarget(t *testing.T) {
	t.Parallel()
	// everyone already applies to every principal via SourceGlobal. Granting it
	// again from a claim would add a second attribution for the same role under
	// a different Source, double-reporting it downstream and buying nothing.
	_, err := acl.LoadPolicyBytes([]byte(`
roles:
  everyone: {read: [ticket]}
asserted_role_assignments:
  admin: everyone
`))
	if err == nil {
		t.Fatal("everyone accepted as an asserted-role target")
	}
	if !strings.Contains(err.Error(), "everyone") {
		t.Errorf("error does not explain the everyone conflict: %v", err)
	}
}

func TestAssertedRoles_AbsentKeyIsByteIdenticalToBefore(t *testing.T) {
	t.Parallel()
	// The no-op default the docs idiom requires: a policy that never mentions
	// the key behaves exactly as it did before this feature.
	d := assertedWorld(t, `
roles:
  editor:
    read: [ticket]
assignments:
  alice: editor
`)

	roles := globalRoles(t, d, verified("alice", "admin"))

	if _, ok := roles["editor"]; !ok {
		t.Error("assignments-based grant broke")
	}
	if src := roles["editor"]; src.Kind != acl.SourceGlobal {
		t.Errorf("Source.Kind = %v, want SourceGlobal", src.Kind)
	}
}
