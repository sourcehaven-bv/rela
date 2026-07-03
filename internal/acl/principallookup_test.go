package acl

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// principalLookupPolicyYAML is the acceptance-test policy from the spec:
// resolve the raw principal against persoon.email, walk heeft_rol for
// group membership, and assign role `md` to the ROLE-MD group.
const principalLookupPolicyYAML = `
user_entity_type: persoon
principal_property: email
membership_relation: heeft_rol
assignments:
  ROLE-MD: md
roles:
  everyone:
    read: ["*"]
  md:
    read: ["*"]
    update: ["*"]
`

type personRow struct {
	id    string
	email string
}

// buildPrincipalLookupWorld constructs a memstore-backed Declarative with
// the principal-property lookup enabled. persons carry an `email`
// property; relations is a list of (from, type, to) triples.
func buildPrincipalLookupWorld(
	ctx context.Context, t *testing.T, policyYAML string, persons []personRow, relations [][3]string,
) *Declarative {
	t.Helper()
	ms := memstore.New()
	for _, p := range persons {
		e := entity.New(p.id, "persoon")
		if p.email != "" {
			e.SetString("email", p.email)
		}
		if err := ms.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create persoon %s: %v", p.id, err)
		}
	}
	// Ensure any role entities referenced by relations exist.
	seen := map[string]bool{}
	for _, p := range persons {
		seen[p.id] = true
	}
	for _, r := range relations {
		for _, id := range []string{r[0], r[2]} {
			if seen[id] {
				continue
			}
			seen[id] = true
			typ := "rol"
			if strings.HasPrefix(id, "PERS-") {
				typ = "persoon"
			}
			if err := ms.CreateEntity(ctx, entity.New(id, typ)); err != nil {
				t.Fatalf("create %s: %v", id, err)
			}
		}
	}
	for _, r := range relations {
		if _, err := ms.CreateRelation(ctx, r[0], r[1], r[2], nil); err != nil {
			t.Fatalf("create relation %s--%s-->%s: %v", r[0], r[1], r[2], err)
		}
	}

	policy, err := LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	d, err := NewDeclarative(policy, NewStoreGraph(ms), ms, WithPrincipalLookup(NewStorePrincipalLookup(ms)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// resolveAndAttribute resolves rawUser to an entity ID (falling back to
// rawUser on empty/error, mirroring the wiring layer) and returns the
// role names attributed to the resulting principal against entityID.
func resolveAndAttribute(ctx context.Context, t *testing.T, d *Declarative, rawUser, entityID string) map[string]bool {
	t.Helper()
	effective := rawUser
	if id, err := d.ResolvePrincipal(ctx, rawUser); err == nil && id != "" {
		effective = id
	}
	req, err := d.ForPrincipal(principal.Principal{User: effective, Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("ForPrincipal(%q): %v", effective, err)
	}
	roles := map[string]bool{}
	for _, a := range req.ForEntity(ctx, "", entityID) {
		roles[a.Role] = true
	}
	return roles
}

// ---- ResolvePrincipal unit behaviors -----------------------------------

func TestResolvePrincipal(t *testing.T) {
	ctx := context.Background()
	persons := []personRow{
		{id: "PERS-JV", email: "jvloothuis@sourcehaven.nl"},
		{id: "PERS-TS", email: "tschmits@sourcehaven.nl"},
	}
	d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, persons, nil)

	t.Run("single match returns id", func(t *testing.T) {
		id, err := d.ResolvePrincipal(ctx, "jvloothuis@sourcehaven.nl")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if id != "PERS-JV" {
			t.Fatalf("got %q, want PERS-JV", id)
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		id, err := d.ResolvePrincipal(ctx, "nobody@sourcehaven.nl")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if id != "" {
			t.Fatalf("got %q, want empty", id)
		}
	})

	t.Run("blank raw returns empty", func(t *testing.T) {
		id, err := d.ResolvePrincipal(ctx, "   ")
		if err != nil || id != "" {
			t.Fatalf("got (%q,%v), want (\"\",nil)", id, err)
		}
	})

	t.Run("multiple matches error", func(t *testing.T) {
		dupPersons := []personRow{
			{id: "PERS-A", email: "dup@sourcehaven.nl"},
			{id: "PERS-B", email: "dup@sourcehaven.nl"},
		}
		dd := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, dupPersons, nil)
		id, err := dd.ResolvePrincipal(ctx, "dup@sourcehaven.nl")
		if err == nil {
			t.Fatal("expected ambiguity error, got nil")
		}
		if id != "" {
			t.Fatalf("got id %q on ambiguity, want empty", id)
		}
	})

	t.Run("disabled policy returns empty", func(t *testing.T) {
		// Policy without principal_property → lookup disabled.
		noLookup := "user_entity_type: persoon\nroles:\n  everyone:\n    read: [\"*\"]\n"
		dd := buildPrincipalLookupWorld(ctx, t, noLookup, persons, nil)
		id, err := dd.ResolvePrincipal(ctx, "jvloothuis@sourcehaven.nl")
		if err != nil || id != "" {
			t.Fatalf("disabled lookup: got (%q,%v), want (\"\",nil)", id, err)
		}
	})
}

// ---- The 6 spec acceptance cases ---------------------------------------

func TestPrincipalProperty_AcceptanceCases(t *testing.T) {
	ctx := context.Background()
	persons := []personRow{
		{id: "PERS-JV", email: "jvloothuis@sourcehaven.nl"},
		{id: "PERS-TS", email: "tschmits@sourcehaven.nl"},
	}
	// PERS-JV --heeft_rol--> ROLE-MD ; assignments: ROLE-MD -> md.
	rels := [][3]string{{"PERS-JV", "heeft_rol", "ROLE-MD"}}

	t.Run("case1 happy path: email resolves and md via group", func(t *testing.T) {
		d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, persons, rels)
		roles := resolveAndAttribute(ctx, t, d, "jvloothuis@sourcehaven.nl", "PERS-TS")
		if !roles["md"] {
			t.Fatalf("expected md role via ROLE-MD group, got %v", roles)
		}
		if !roles["everyone"] {
			t.Fatalf("expected everyone role, got %v", roles)
		}
	})

	t.Run("case2 no graph match: everyone only", func(t *testing.T) {
		d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, persons, rels)
		roles := resolveAndAttribute(ctx, t, d, "nobody@sourcehaven.nl", "PERS-TS")
		if roles["md"] {
			t.Fatalf("unexpected md role for unknown principal, got %v", roles)
		}
		if !roles["everyone"] {
			t.Fatalf("expected everyone role, got %v", roles)
		}
	})

	t.Run("case3 multiple matches: fall back to raw, everyone only", func(t *testing.T) {
		dupPersons := append([]personRow{
			{id: "PERS-DUP1", email: "dup@sourcehaven.nl"},
			{id: "PERS-DUP2", email: "dup@sourcehaven.nl"},
		}, persons...)
		d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, dupPersons, rels)
		roles := resolveAndAttribute(ctx, t, d, "dup@sourcehaven.nl", "PERS-TS")
		if roles["md"] {
			t.Fatalf("ambiguous principal must not gain md, got %v", roles)
		}
		if !roles["everyone"] {
			t.Fatalf("expected everyone role, got %v", roles)
		}
	})

	t.Run("case4 raw-key assignment escape hatch", func(t *testing.T) {
		// No persoon with this email; assignment keyed on the raw UPN.
		rawPolicy := `
user_entity_type: persoon
principal_property: email
membership_relation: heeft_rol
assignments:
  jvloothuis@sourcehaven.nl: md
roles:
  everyone:
    read: ["*"]
  md:
    read: ["*"]
    update: ["*"]
`
		noEmailPersons := []personRow{{id: "PERS-TS", email: "tschmits@sourcehaven.nl"}}
		d := buildPrincipalLookupWorld(ctx, t, rawPolicy, noEmailPersons, nil)
		roles := resolveAndAttribute(ctx, t, d, "jvloothuis@sourcehaven.nl", "PERS-TS")
		if !roles["md"] {
			t.Fatalf("expected md via raw-key assignment, got %v", roles)
		}
	})

	t.Run("case6 missing principal_property: raw string used as-is", func(t *testing.T) {
		// Policy without principal_property; assignment on the raw UPN
		// must still fire (byte-for-byte pre-feature behavior).
		rawPolicy := `
user_entity_type: persoon
membership_relation: heeft_rol
assignments:
  jvloothuis@sourcehaven.nl: md
roles:
  everyone:
    read: ["*"]
  md:
    read: ["*"]
    update: ["*"]
`
		d := buildPrincipalLookupWorld(ctx, t, rawPolicy, persons, rels)
		roles := resolveAndAttribute(ctx, t, d, "jvloothuis@sourcehaven.nl", "PERS-TS")
		if !roles["md"] {
			t.Fatalf("expected md via raw-key assignment with lookup disabled, got %v", roles)
		}
	})
}

// case5: local role via role_relations, from the substituted entity ID.
func TestPrincipalProperty_LocalRoleFromSubstitutedID(t *testing.T) {
	ctx := context.Background()
	policyYAML := `
user_entity_type: persoon
principal_property: email
membership_relation: heeft_rol
role_relations:
  toegewezen_aan:
    confers: assignee
roles:
  everyone:
    read: ["*"]
  assignee:
    read: ["*"]
    update: ["*"]
`
	persons := []personRow{{id: "PERS-JV", email: "jvloothuis@sourcehaven.nl"}}
	// PERS-JV --toegewezen_aan--> ASSET-X (ASSET-X created as a rol-typed
	// placeholder; type does not matter for the edge probe).
	rels := [][3]string{{"PERS-JV", "toegewezen_aan", "ASSET-X"}}
	d := buildPrincipalLookupWorld(ctx, t, policyYAML, persons, rels)

	roles := resolveAndAttribute(ctx, t, d, "jvloothuis@sourcehaven.nl", "ASSET-X")
	if !roles["assignee"] {
		t.Fatalf("expected assignee local role from substituted PERS-JV, got %v", roles)
	}
}
