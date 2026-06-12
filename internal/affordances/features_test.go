package affordances_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Feature tests for the affordance verdict layer.
//
// These pin design properties that compose the acl.Declarative
// resolver (groups, containment, typed-Source attribution) with the
// affordance grants from acl.yaml. They live in this package (not
// internal/acl) because the verdicts are an affordances-layer concern:
// the ACL provides role attribution, the affordances package turns
// roles + grants into field-level verdicts.
//
// The companion file in internal/acl/features_test.go documents the
// split — UC10 and UC11 explicitly redirect readers here.
//
// IMPORTANT: each test wires its policy to depend on group expansion
// (alice in a `viewers` team that holds the role). This pins that the
// member-of walk through acl.Declarative is the source of role
// attribution, not just the affordance grant lookup — if the resolver
// stops walking member-of, these tests fail.

// TestFeature_UC10_PropertyRedaction pins that closed-world `visible:`
// grants strip undeclared properties from the affordance verdict, and
// that the granted properties stay visible.
//
// Scenario. A ticket has a title, a status, and an internal_notes
// field. The `viewer` role can only see the title and status — the
// internal_notes is operationally sensitive and should be hidden.
//
// The `viewer` role is assigned to the `viewers` group (NOT directly
// to alice). Alice is a member of `viewers` via `member-of`. The role
// flows to her only via group expansion through acl.Declarative —
// this test fails if the member-of walk regresses or the affordances
// resolver stops consulting the declarative role attribution.
func TestFeature_UC10_PropertyRedaction(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: metamodel.PropertyTypeString},
					"status":         {Type: metamodel.PropertyTypeString},
					"internal_notes": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}

	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
assignments:
  viewers: viewer
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	// Real store with alice in the viewers group. The resolver's
	// member-of walk picks up the role through group expansion.
	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	mustCreateEntity(t, ms, "viewers", "team")
	mustCreateRelation(t, ms, "alice", "member-of", "viewers")
	tkt := entity.New("TKT-001", "ticket")
	tkt.SetString("title", "broken login")
	tkt.SetString("status", "open")
	tkt.SetString("internal_notes", "customer threatening to leave")
	mustCreate(t, ms, tkt)

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta,
		storeRelationLookup{ms}, declarative)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}

	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	verdict := resolver.FieldVerdicts(ctx, tkt)

	// internal_notes is closed-world denied; title and status are
	// visible by default (sparse map; absence == visible).
	if v, present := verdict.Visible["internal_notes"]; !present || v {
		t.Errorf("Visible[internal_notes] = (%v, present=%v); want (false, present=true)", v, present)
	}
	for _, granted := range []string{"title", "status"} {
		if _, present := verdict.Visible[granted]; present {
			t.Errorf("Visible[%s] present in sparse map; want absent (visible-by-default)", granted)
		}
	}
}

// TestFeature_UC11_ReadAndVisibleCompose pins the composition between
// entity-level read grants and property-level visible: grants. They
// apply independently — Read controls whether the entity appears at
// all, visible: controls which properties are stripped from a visible
// entity.
//
// Scenario. The `viewer` role grants read on tickets but lists no
// fields under visible: (closed-world deny on every property). The
// effect is that Alice can see THAT tickets exist (id and type appear
// on the wire) but every property is stripped.
//
// Same group-mediated wiring as UC10: alice in the viewers group.
func TestFeature_UC11_ReadAndVisibleCompose(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"status": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}

	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  viewer:
    read: [ticket]
    visible:
      ticket: []
assignments:
  viewers: viewer
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	mustCreateEntity(t, ms, "viewers", "team")
	mustCreateRelation(t, ms, "alice", "member-of", "viewers")
	tkt := entity.New("TKT-001", "ticket")
	tkt.SetString("title", "broken login")
	tkt.SetString("status", "open")
	mustCreate(t, ms, tkt)

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta,
		storeRelationLookup{ms}, declarative)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}

	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	verdict := resolver.FieldVerdicts(ctx, tkt)

	for _, name := range []string{"title", "status"} {
		v, present := verdict.Visible[name]
		if !present || v {
			t.Errorf("Visible[%s] = (%v, present=%v); want (false, present=true) — closed-world visible:[] strips every declared prop",
				name, v, present)
		}
	}
}

// TestFeature_UnstampedEveryone pins `everyone` role semantics: an
// unstamped principal still picks up `everyone` if the policy declares
// it. acl.Declarative.ForPrincipal errors on unstamped, but the
// resolveViaDeclarative fallback adds `everyone` explicitly so
// anonymous-readable workspaces work.
//
// Scenario. The policy declares the `everyone` role with `read:
// [ticket]` and `visible:` granting only the `title` field. A request
// with no principal stamp (User=""/"unknown") arrives — perhaps from
// a misconfigured proxy or a public anonymous-readable workspace.
// The affordance verdict should still apply the `everyone` role's
// visibility rules. Without this, anonymous-readable workspaces
// (which the design doc calls out as a supported case) silently
// degrade to "no role at all".
func TestFeature_UnstampedEveryone(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: metamodel.PropertyTypeString},
					"internal_notes": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}

	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  everyone:
    read: [ticket]
    visible:
      ticket:
        - field: title
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	ms := memstore.New()
	tkt := entity.New("TKT-001", "ticket")
	tkt.SetString("title", "public update")
	tkt.SetString("internal_notes", "hidden")
	mustCreate(t, ms, tkt)

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta,
		storeRelationLookup{ms}, declarative)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}

	// Unstamped: User="" → ForPrincipal errors, but everyone should
	// still apply.
	ctx := context.Background()
	verdict := resolver.FieldVerdicts(ctx, tkt)

	// internal_notes should be hidden (closed-world); title visible.
	if v, present := verdict.Visible["internal_notes"]; !present || v {
		t.Errorf("Visible[internal_notes] = (%v, present=%v); want (false, present=true) — everyone applies to unstamped", v, present)
	}
	if _, present := verdict.Visible["title"]; present {
		t.Errorf("Visible[title] should be absent (visible-by-default); got entry in map")
	}
}

// TestFeature_AC8_WriteAffordanceParity pins that the write path and
// the affordance path agree on the role attribution for the same
// (principal, policy, store, entity) tuple. They share a single
// *acl.Declarative — the wiring guarantee from TKT-DVOU's collapse —
// so the assertion is "no rogue resolver was injected and no
// transformation drift sneaks in between the layers."
//
// The test fails if a future change causes the affordance path to use
// a different attribution source than the write path (e.g. an
// independent walker, a stale snapshot, a filtered subset).
//
// Scenario (RR-Y6Y9 — discriminating shape). Alice is editor-of
// TKT-001 via a per-entity edge (NOT a global assignment, NOT a
// group). Two pieces of evidence:
//
//  1. Write path: authorize OpUpdate on TKT-001. Allow with RuleID
//     "editor" — only the editor role grants write on ticket, and
//     editor flows in only via the per-entity edge.
//  2. Affordance path: editor role declares a `visible:` grant on
//     internal_notes ONLY when has_role(..., "editor"). Under the
//     correct wiring, the affordance resolver picks up editor for
//     Alice on TKT-001, the predicate passes, and the field is
//     visible (absent from the deny-map under closed-world).
//
// A regression where affordances.New ignored its injected Declarative
// would fail (2): the resolver would not see editor for Alice on
// TKT-001 (no global, no group, no fallback resolver knows about the
// per-entity grant) and internal_notes would land in the deny-map.
//
// This is the discriminating test the reviewer asked for: the
// affordance path's behavior observably depends on whether the
// injected Declarative is consulted.
func TestFeature_AC8_WriteAffordanceParity(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: metamodel.PropertyTypeString},
					"internal_notes": {Type: metamodel.PropertyTypeString},
				},
			},
			"person": {},
		},
	}

	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor:
    write: [ticket]
    read: [ticket]
    visible:
      ticket:
        - field: internal_notes
          when: 'has_role(current_user, entity, "editor")'
role_relations:
  editor-of:
    confers: editor
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	tkt := entity.New("TKT-001", "ticket")
	tkt.SetString("title", "x")
	tkt.SetString("internal_notes", "secret")
	mustCreate(t, ms, tkt)
	// Per-entity grant: alice editor-of TKT-001. No group, no global
	// assignment — this is the ONLY path that confers editor.
	mustCreateRelation(t, ms, "alice", "editor-of", "TKT-001")

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, declarative)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}

	aliceCtx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	// (1) Write path: deny without the per-entity edge would mean
	// editor doesn't apply; allow with RuleID=editor confirms the
	// edge confers editor on TKT-001.
	writeDecision := declarative.AuthorizeWrite(aliceCtx, acl.WriteRequest{
		Op:      acl.OpUpdate,
		Subject: acl.EntitySubject{Type: "ticket", ID: "TKT-001"},
	})
	if !writeDecision.Allow {
		t.Fatalf("write path: expected allow (editor-of edge should confer editor); got %+v", writeDecision)
	}
	if writeDecision.RuleID != "editor" {
		t.Errorf("write path: RuleID = %q, want %q", writeDecision.RuleID, "editor")
	}

	// (2) Affordance path: internal_notes should be VISIBLE because
	// the predicate has_role(current_user, entity, "editor") passes —
	// the resolver attributes editor to Alice on TKT-001 via the
	// per-entity edge. Pre-fix or under a regression that ignored
	// the injected Declarative, internal_notes would be hidden
	// (present in the deny-map with false), because no other path
	// confers editor.
	verdict := resolver.FieldVerdicts(aliceCtx, tkt)
	if _, hidden := verdict.Visible["internal_notes"]; hidden {
		t.Errorf("affordance path: internal_notes is hidden (Visible=%+v); want visible — affordance resolver may not be consulting the shared Declarative",
			verdict.Visible)
	}
}

// RR-JRPZ: has_role on an ancestor-conferred role must return true
// even though no direct principal --role-relation--> entity edge
// exists. Pre-fix, hasRole only consulted (globals ∪ direct-edge),
// so the verdict layer (which DOES inherit through belongs-to) and
// the predicate-layer hasRole disagreed on what "principal holds
// role on entity" meant.
//
// Scenario. Alice is `editor-of` folder F-eng. D-secret belongs-to
// F-eng via the policy's inherit_roles_through. Two grants on
// document.internal_notes test the discriminator:
//
//   - "editor" predicate: should match → field granted → absent from
//     the Visible deny-map (visible-by-default).
//   - "nonexistent" predicate: should NOT match → field denied →
//     Visible[internal_notes] = false (closed-world hidden).
//
// Without the fix, the editor predicate returns false (no direct
// editor-of edge from alice to D-secret) and internal_notes is
// indistinguishable from the bogus case.
func TestFeature_HasRole_AncestorConferred(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document": {
				Properties: map[string]metamodel.PropertyDef{
					"internal_notes": {Type: metamodel.PropertyTypeString},
				},
			},
			"folder": {Properties: map[string]metamodel.PropertyDef{}},
			"person": {},
		},
	}

	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	mustCreate(t, ms, entity.New("F-eng", "folder"))
	dsecret := entity.New("D-secret", "document")
	dsecret.SetString("internal_notes", "headcount changes")
	mustCreate(t, ms, dsecret)
	// Alice is editor-of the folder, not the document.
	mustCreateRelation(t, ms, "alice", "editor-of", "F-eng")
	// Document belongs-to the folder → editor on F-eng inherits to D-secret.
	mustCreateRelation(t, ms, "D-secret", "belongs-to", "F-eng")

	cases := []struct {
		name        string
		when        string
		wantVisible bool // expect Visible[internal_notes]=false to be PRESENT in map
	}{
		{
			name:        "matches_ancestor_conferred",
			when:        `has_role(current_user, entity, "editor")`,
			wantVisible: false, // grant fires → field visible → absent from deny-map
		},
		{
			name:        "matches_nothing",
			when:        `has_role(current_user, entity, "nonexistent")`,
			wantVisible: true, // grant denied → present with false (closed-world)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor:
    read: [document]
    visible:
      document:
        - field: internal_notes
          when: '` + tc.when + `'
role_relations:
  editor-of:
    confers: editor
inherit_roles_through:
  - belongs-to
`))
			if err != nil {
				t.Fatalf("LoadPolicyBytes: %v", err)
			}
			d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
			if err != nil {
				t.Fatalf("NewDeclarative: %v", err)
			}
			resolver, err := affordances.New(meta, storeRelationLookup{ms}, d)
			if err != nil {
				t.Fatalf("affordances.New: %v", err)
			}
			ctx := principal.With(context.Background(),
				principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
			verdict := resolver.FieldVerdicts(ctx, dsecret)

			_, denied := verdict.Visible["internal_notes"]
			if denied != tc.wantVisible {
				t.Errorf("Visible[internal_notes] presence = %v; want %v (verdict=%+v)",
					denied, tc.wantVisible, verdict)
			}
		})
	}
}

// ---- Internal helpers ---------------------------------------------------

// storeRelationLookup adapts a store.Store to the
// affordances.RelationLookup interface. Same shape as the production
// adapter in internal/dataentry; duplicated for tests since the
// dataentry adapter is package-private. A follow-up will consolidate
// RelationLookup and acl.Graph (only OutgoingCounts differs).
type storeRelationLookup struct{ st store.Store }

func (l storeRelationLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		EntityID: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		counts[rel.Type]++
	}
	return counts
}

func (l storeRelationLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		EntityID: fromID, Type: relType, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		if rel.To == toID {
			return true
		}
	}
	return false
}

func mustCreateEntity(t *testing.T, ms *memstore.MemStore, id, typ string) {
	t.Helper()
	if err := ms.CreateEntity(context.Background(), entity.New(id, typ)); err != nil {
		t.Fatalf("create %s/%s: %v", typ, id, err)
	}
}

func mustCreate(t *testing.T, ms *memstore.MemStore, e *entity.Entity) {
	t.Helper()
	if err := ms.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("create %s/%s: %v", e.Type, e.ID, err)
	}
}

func mustCreateRelation(t *testing.T, ms *memstore.MemStore, from, typ, to string) {
	t.Helper()
	if _, err := ms.CreateRelation(context.Background(), from, typ, to, nil); err != nil {
		t.Fatalf("create relation %s --%s--> %s: %v", from, typ, to, err)
	}
}
