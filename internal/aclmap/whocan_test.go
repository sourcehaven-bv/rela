package aclmap_test

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// groundingPolicy is the ISMS-style acl.yaml from RES-8TX9KF, exercised
// by every who-can test. It has a security group, an incident type, two
// role-conferring relations (editor-of, responds-to), and belongs-to
// inheritance — enough to produce every SourceKind.
const groundingPolicy = `
membership_relation: member-of
roles:
  everyone:  { read: [project] }
  reader:    { read: [ticket, incident] }
  editor:    { read: [ticket, incident], create: [ticket], update: [ticket], delete: [ticket] }
  responder: { read: [incident], update: [incident] }
  security:  { read: [incident, ticket], create: [incident], update: [incident], delete: [incident] }
assignments:
  PERS-ALICE:    editor
  ROLE-SECURITY: security
role_relations:
  editor-of:   { confers: editor }
  responds-to: { confers: responder }
inherit_roles_through:
  - belongs-to
`

// world is a materialized test fixture: a memstore populated with
// entities and relations, plus the Declarative built over it.
type world struct {
	store *memstore.MemStore
	decl  *acl.Declarative
	eng   *aclmap.Engine
}

type ent struct{ id, typ string }
type rel struct{ from, typ, to string }

func buildWorld(t *testing.T, policyYAML string, ents []ent, rels []rel) *world {
	t.Helper()
	ctx := context.Background()

	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	ms := memstore.New()
	for _, e := range ents {
		if cErr := ms.CreateEntity(ctx, entity.New(e.id, e.typ)); cErr != nil {
			t.Fatalf("create entity %s: %v", e.id, cErr)
		}
	}
	for _, r := range rels {
		if _, cErr := ms.CreateRelation(ctx, r.from, r.typ, r.to, nil); cErr != nil {
			t.Fatalf("create relation %s--%s-->%s: %v", r.from, r.typ, r.to, cErr)
		}
	}
	decl, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	eng, err := aclmap.New(ms, decl)
	if err != nil {
		t.Fatalf("aclmap.New: %v", err)
	}
	return &world{store: ms, decl: decl, eng: eng}
}

// resolvableEnt is an entity with an optional email property, used by
// the principal_property resolution test.
type resolvableEnt struct{ id, typ, email string }

// whoCanResolutionMeta declares a person type with a unique email
// property so principal_property validation passes and the store lookup
// can resolve a raw email to the person entity.
func whoCanResolutionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"person": {Label: "Person", Properties: map[string]metamodel.PropertyDef{
				"email": {Type: "string", Unique: true},
			}},
			"incident": {Label: "Incident", Properties: map[string]metamodel.PropertyDef{}},
			"folder":   {Label: "Folder", Properties: map[string]metamodel.PropertyDef{}},
		},
	}
}

// buildWorldWithMeta is buildWorld plus a metamodel: it validates the
// policy against the metamodel (the boot gate the server applies) and
// wires the store-backed principal lookup so principal_property
// resolution works.
func buildWorldWithMeta(t *testing.T, meta *metamodel.Metamodel, policyYAML string, ents []resolvableEnt, rels []rel) *world {
	t.Helper()
	ctx := context.Background()

	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if vErr := policy.ValidateAgainstMetamodel(metaView{meta}); vErr != nil {
		t.Fatalf("validate against metamodel: %v", vErr)
	}
	ms := memstore.New()
	for _, e := range ents {
		ent := entity.New(e.id, e.typ)
		if e.email != "" {
			ent.SetString("email", e.email)
		}
		if cErr := ms.CreateEntity(ctx, ent); cErr != nil {
			t.Fatalf("create entity %s: %v", e.id, cErr)
		}
	}
	for _, r := range rels {
		if _, cErr := ms.CreateRelation(ctx, r.from, r.typ, r.to, nil); cErr != nil {
			t.Fatalf("create relation %s--%s-->%s: %v", r.from, r.typ, r.to, cErr)
		}
	}
	decl, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(ms)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	eng, err := aclmap.New(ms, decl)
	if err != nil {
		t.Fatalf("aclmap.New: %v", err)
	}
	return &world{store: ms, decl: decl, eng: eng}
}

// metaView adapts a *metamodel.Metamodel to acl.MetamodelView for the
// resolution test's boot-gate validation.
type metaView struct{ m *metamodel.Metamodel }

func (v metaView) HasEntityType(t string) bool {
	_, ok := v.m.Entities[t]
	return ok
}

func (v metaView) PropertyInfo(entityType, property string) acl.PropertyInfo {
	def, ok := v.m.Entities[entityType]
	if !ok {
		return acl.PropertyInfo{}
	}
	pd, ok := def.Properties[property]
	if !ok {
		return acl.PropertyInfo{}
	}
	return acl.PropertyInfo{Exists: true, Unique: pd.Unique, List: pd.List}
}

// groundingWorld wires the canonical scenario:
//   - PERS-ALICE: global editor.
//   - PERS-BOB: member of ROLE-SECURITY (group → security).
//   - PERS-CAROL: responds-to INC-042 (local → responder).
//   - PERS-DAVE: editor-of FOLDER-Q3, which contains INC-042 via
//     belongs-to (local-via-ancestor → editor).
//   - PERS-EVE: member of ROLE-IR, which is editor-of FOLDER-Q3
//     (local-via-group-and-ancestor → editor).
//   - ROLE-IR is editor-of INC-999 directly (local-via-group → editor
//     for its members).
func groundingWorld(t *testing.T) *world {
	t.Helper()
	return buildWorld(t, groundingPolicy,
		[]ent{
			{"PERS-ALICE", "person"}, {"PERS-BOB", "person"}, {"PERS-CAROL", "person"},
			{"PERS-DAVE", "person"}, {"PERS-EVE", "person"},
			{"ROLE-SECURITY", "team"}, {"ROLE-IR", "team"},
			{"FOLDER-Q3", "folder"},
			{"INC-042", "incident"}, {"INC-999", "incident"}, {"TKT-1", "ticket"},
			{"PROJ", "project"},
		},
		[]rel{
			{"PERS-BOB", "member-of", "ROLE-SECURITY"},
			{"PERS-EVE", "member-of", "ROLE-IR"},
			{"PERS-CAROL", "responds-to", "INC-042"},
			{"PERS-DAVE", "editor-of", "FOLDER-Q3"},
			{"ROLE-IR", "editor-of", "FOLDER-Q3"},
			{"ROLE-IR", "editor-of", "INC-999"},
			{"INC-042", "belongs-to", "FOLDER-Q3"},
		},
	)
}

// principals returns the sorted principal IDs in a WhoCanResult.
func principals(r *aclmap.WhoCanResult) []string {
	out := make([]string, 0, len(r.Principals))
	for _, p := range r.Principals {
		out = append(out, p.Principal)
	}
	sort.Strings(out)
	return out
}

// routeKinds returns the sorted SourceKind strings across a principal's
// routes in the result.
func routeKindsFor(r *aclmap.WhoCanResult, who string) []string {
	var kinds []string
	for _, p := range r.Principals {
		if p.Principal != who {
			continue
		}
		for _, rt := range p.Routes {
			kinds = append(kinds, rt.Kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}

func TestWhoCan_AllSixRouteKinds(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	// read INC-042: who can read the incident, and by what route?
	res, err := w.eng.WhoCan(ctx, acl.VerbRead, "INC-042")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}

	// Expected readers of INC-042 and the route kind each takes:
	//   PERS-ALICE  global editor (read: incident)
	//   PERS-BOB    group → security
	//   PERS-CAROL  local (responds-to → responder)
	//   PERS-DAVE   local-via-ancestor (editor-of FOLDER-Q3 ⤳ INC-042)
	//   PERS-EVE    local-via-group-and-ancestor (member ROLE-IR, editor-of FOLDER-Q3)
	// The role/group entities themselves also resolve to a reader when
	// treated as a principal (ROLE-SECURITY is assigned security;
	// ROLE-IR is editor-of the containing folder) — reported truthfully,
	// not excluded by topology (see RR-C5Q743).
	wantPrincipals := []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE", "PERS-EVE",
		"ROLE-IR", "ROLE-SECURITY",
	}
	if got := principals(res); !equal(got, wantPrincipals) {
		t.Errorf("read INC-042 principals:\n got: %v\nwant: %v", got, wantPrincipals)
	}

	checks := map[string]string{
		"PERS-ALICE": "global",
		"PERS-BOB":   "group",
		"PERS-CAROL": "local",
		"PERS-DAVE":  "local-via-ancestor",
		"PERS-EVE":   "local-via-group-and-ancestor",
	}
	for who, kind := range checks {
		kinds := routeKindsFor(res, who)
		if !contains(kinds, kind) {
			t.Errorf("%s: want a %q route, got kinds %v", who, kind, kinds)
		}
	}

	// local-via-group: PERS-EVE reading INC-999 (ROLE-IR editor-of INC-999
	// directly; Eve is a member).
	res999, err := w.eng.WhoCan(ctx, acl.VerbRead, "INC-999")
	if err != nil {
		t.Fatalf("WhoCan INC-999: %v", err)
	}
	if kinds := routeKindsFor(res999, "PERS-EVE"); !contains(kinds, "local-via-group") {
		t.Errorf("PERS-EVE on INC-999: want a local-via-group route, got %v", kinds)
	}
}

// TestWhoCan_ReadMatchesRuntime is the anti-false-negative guard
// (RR-7UXWNA). For every enumerated principal and a spread of entities,
// who-can's read result must agree with the runtime PermitsRead
// decision — the map may never report a principal cannot read when the
// server would let them.
func TestWhoCan_ReadMatchesRuntime(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	// The candidate universe is EVERY entity the resolver could attribute
	// a role to — including the role/group entities, which are reported
	// truthfully. For each, the runtime read decision must agree with the
	// map (two-way: listed ⟺ runtime-permits).
	candidates := []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE", "PERS-EVE",
		"ROLE-SECURITY", "ROLE-IR",
	}
	entities := []struct{ id, typ string }{
		{"INC-042", "incident"}, {"INC-999", "incident"}, {"TKT-1", "ticket"},
	}

	for _, e := range entities {
		res, err := w.eng.WhoCan(ctx, acl.VerbRead, e.id)
		if err != nil {
			t.Fatalf("WhoCan read %s: %v", e.id, err)
		}
		reported := map[string]bool{}
		for _, p := range res.Principals {
			reported[p.Principal] = true
		}
		everyoneGrantsRead := res.Everyone.Granted

		for _, c := range candidates {
			req, err := w.decl.ForPrincipal(principal.Principal{User: c, Tool: principal.ToolCLI})
			if err != nil {
				t.Fatalf("ForPrincipal %s: %v", c, err)
			}
			runtime, err := req.PermitsRead(ctx, e.typ, e.id)
			if err != nil {
				t.Fatalf("PermitsRead %s %s: %v", c, e.id, err)
			}
			// Two-way agreement:
			//   false negative — runtime permits but who-can omitted (and
			//   everyone doesn't blanket-grant): the worst failure.
			if runtime && !reported[c] && !everyoneGrantsRead {
				t.Errorf("false negative: runtime permits %s to read %s, but who-can omitted it",
					c, e.id)
			}
			//   false positive — who-can lists a principal the runtime
			//   denies (only checkable when everyone isn't blanket-granting,
			//   since everyone would make runtime true for all).
			if !runtime && reported[c] && !everyoneGrantsRead {
				t.Errorf("false positive: who-can lists %s for read %s, but the runtime denies it",
					c, e.id)
			}
		}
	}
}

func TestWhoCan_EveryoneReportedOnceGlobally(t *testing.T) {
	t.Parallel()
	// everyone: read: ["*"] — every principal (and unauthenticated) can read.
	policy := `
roles:
  everyone: { read: ["*"] }
  editor:   { read: [ticket], create: [ticket], update: [ticket], delete: [ticket] }
assignments:
  PERS-ALICE: editor
`
	w := buildWorld(t, policy,
		[]ent{{"PERS-ALICE", "person"}, {"INC-1", "incident"}},
		nil,
	)
	res, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	if !res.Everyone.Granted || !res.Everyone.Wildcard {
		t.Errorf("want everyone granted via wildcard, got %+v", res.Everyone)
	}
	// The everyone read must NOT be attributed to PERS-ALICE as a
	// per-principal row (Alice's only non-everyone routes are writes on
	// ticket, none on incident-read).
	for _, p := range res.Principals {
		if p.Principal == "PERS-ALICE" {
			t.Errorf("PERS-ALICE should not appear as a read-principal for INC-1 "+
				"(read is everyone-only); got routes %+v", p.Routes)
		}
	}
}

// TestWhoCan_ActorThatIsAlsoMembershipTarget is the RR-C5Q743
// regression: a principal who both holds a role (member of ROLE-EXEC)
// AND is a membership target (someone reports to them) must NOT be
// dropped. Excluding membership targets as "groups" silently loses this
// real grantee — a false negative.
func TestWhoCan_ActorThatIsAlsoMembershipTarget(t *testing.T) {
	t.Parallel()
	w := buildWorld(t, `
roles:
  exec:     { read: [incident] }
  everyone: { read: [folder] }
assignments:
  ROLE-EXEC: exec
`,
		[]ent{
			{"PERS-BOSS", "person"}, {"PERS-REPORT", "person"},
			{"ROLE-EXEC", "team"}, {"INC-1", "incident"},
		},
		[]rel{
			{"PERS-BOSS", "member-of", "ROLE-EXEC"},
			{"PERS-REPORT", "member-of", "PERS-BOSS"},
		},
	)
	res, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	// The bug was that PERS-BOSS (a membership target) was excluded even
	// though they hold exec via ROLE-EXEC. It must be present now.
	if !contains(principals(res), "PERS-BOSS") {
		t.Errorf("PERS-BOSS (real exec, also a membership target) dropped; principals=%v",
			principals(res))
	}
	// PERS-REPORT also reads: member-of is transitive, so
	// REPORT → BOSS → ROLE-EXEC confers exec. The runtime agrees (asserted
	// by the two-way conformance test); listing them is correct, not a
	// false positive.
	if !contains(principals(res), "PERS-REPORT") {
		t.Errorf("PERS-REPORT should read INC-1 via transitive member-of; principals=%v",
			principals(res))
	}
}

// TestWhoCan_PrincipalPropertyResolution exercises the principal_property
// path (RR-EFFDQL). The person entity carries an email; a role-conferring
// edge grants it read. The engine enumerates the person by ID and by the
// role-relation source, resolves consistently, and collapses to ONE row
// (RR-XC2NTO) — it must not double-report. The grant is via a graph edge
// (which keys on the entity ID, so it survives resolution), demonstrating
// the resolution path end-to-end without the raw-key/entity-key
// assignment mismatch (the documented data-entry-transport caveat).
func TestWhoCan_PrincipalPropertyResolution(t *testing.T) {
	t.Parallel()
	w := buildWorldWithMeta(t, whoCanResolutionMeta(), `
user_entity_type: person
principal_property: email
roles:
  responder: { read: [incident] }
  everyone:  { read: [folder] }
role_relations:
  responds-to: { confers: responder }
`,
		[]resolvableEnt{
			{"PERS-ALICE", "person", "alice@corp.example"},
			{"INC-1", "incident", ""},
		},
		[]rel{{"PERS-ALICE", "responds-to", "INC-1"}},
	)
	res, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	var alice []aclmap.PrincipalAccess
	for _, p := range res.Principals {
		if p.Principal == "PERS-ALICE" {
			alice = append(alice, p)
		}
	}
	if len(alice) != 1 {
		t.Fatalf("PERS-ALICE should appear exactly once (merged), got %d rows: %+v", len(alice), res.Principals)
	}
	if len(alice[0].Routes) == 0 || alice[0].Routes[0].Role != "responder" {
		t.Errorf("PERS-ALICE should read via responder route; got %+v", alice[0].Routes)
	}
}

func TestWhoCan_MissingEntityErrors(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-DOES-NOT-EXIST")
	if !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Fatalf("want ErrEntityNotFound, got %v", err)
	}
}

// TestWhoCan_UnknownVerbErrors guards fail-closed behavior: a garbage
// verb (bypassing the CLI enum) must error, never be silently treated
// as read.
func TestWhoCan_UnknownVerbErrors(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.WhoCan(context.Background(), acl.Verb("frobnicate"), "INC-042")
	if err == nil {
		t.Fatal("want an error for an unknown verb, got nil")
	}
}

func TestWhoCan_WriteVerb_Delete(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	// delete incident: only the security group may delete incidents.
	res, err := w.eng.WhoCan(context.Background(), acl.VerbDelete, "INC-042")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	// PERS-BOB (member of ROLE-SECURITY) can delete; ROLE-SECURITY itself
	// (the assignment key, resolved as a principal) also carries security
	// → delete and is reported truthfully. PERS-DAVE (editor via folder
	// inheritance) can also delete incidents. PERS-CAROL (responder,
	// update only) must NOT appear.
	got := principals(res)
	if !contains(got, "PERS-BOB") {
		t.Errorf("delete INC-042 must list PERS-BOB; got %v", got)
	}
	if contains(got, "PERS-CAROL") {
		t.Errorf("PERS-CAROL (responder, no delete) must not appear; got %v", got)
	}
	if res.Everyone.Granted {
		t.Errorf("everyone should not grant delete")
	}
}

func TestWhoCan_MultiRoute(t *testing.T) {
	t.Parallel()
	// PERS-DAVE reads INC-042 via BOTH a direct responds-to edge AND
	// folder inheritance (editor-of FOLDER-Q3 ⤳ INC-042). All routes
	// must be listed, not collapsed to one.
	w := buildWorld(t, groundingPolicy,
		[]ent{
			{"PERS-DAVE", "person"}, {"FOLDER-Q3", "folder"}, {"INC-042", "incident"},
		},
		[]rel{
			{"PERS-DAVE", "editor-of", "FOLDER-Q3"},
			{"PERS-DAVE", "responds-to", "INC-042"},
			{"INC-042", "belongs-to", "FOLDER-Q3"},
		},
	)
	res, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-042")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	kinds := routeKindsFor(res, "PERS-DAVE")
	if !contains(kinds, "local") || !contains(kinds, "local-via-ancestor") {
		t.Errorf("PERS-DAVE multi-route: want both local and local-via-ancestor, got %v", kinds)
	}
}

// TestWhoCan_JSONSchemaVersioned pins the versioned, deterministic JSON
// shape (schema_version present; routes are a list, not an enum).
func TestWhoCan_JSONSchemaVersioned(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	res, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-042")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	if res.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", res.SchemaVersion)
	}
	for _, p := range res.Principals {
		if p.Routes == nil {
			t.Errorf("%s: routes must be a (possibly empty) list, got nil", p.Principal)
		}
	}
}

// ---- helpers ----

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(xs []string, x string) bool {
	return slices.Contains(xs, x)
}

// ensure the store type satisfies the engine's narrow interface at
// compile time (defensive; the constructor already takes it).
var _ aclmap.EntitySource = store.Store(nil)
