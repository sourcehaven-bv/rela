package visibility_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const capACLMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      held:
        type: boolean
        rollup:
          exists: true
          along: [depends_on]
          max_depth: 4
          condition: {project: "entity.status == 'on_hold'"}
      waiting:
        type: boolean
        rollup:
          exists: true
          siblings_before: child_of
          where: {status: open}
          when_parent: {ordering: sequential}
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      status: {type: enum, values: [active, on_hold], default: active}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
      has_free:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition: {action: "entity.held == false"}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
relations:
  child_of:
    from: [action]
    to: [project]
    max_outgoing: 1
    orderable: incoming
    inverse: parent_of
  depends_on:
    from: [action]
    to: [action, project]
  shared-with:
    from: [person]
    to: [action]
`

// capACLPolicy is the acl.yaml of the cap tests. PERS-A reads everything.
// PERS-B reads the actions shared with it only, and `_order_in` only on
// the child_of edges of those actions: a relation `visible:` grant on the
// conferred role hides it on the edges of the others.
const capACLPolicy = `roles:
  all:
    read: ["*"]
    relations:
      action:
        - relation: child_of
          visible: [{field: _order_in}]
  base:
    read: [project, person]
  shared:
    read: [action]
    relations:
      action:
        - relation: child_of
          visible: [{field: _order_in}]
assignments:
  PERS-A: all
  PERS-B: base
role_relations:
  shared-with: {confers: shared}
`

// capACLHidden is the number of actions PERS-B cannot read.
const capACLHidden = 8

// capACLFixture is a store seeded for the cap tests, with the roll-up set
// and the compiled policy over it.
type capACLFixture struct {
	st     store.Store
	set    *rollup.Set
	d      *acl.Declarative
	gate   visibility.DeclarativeGate
	fields visibility.PolicyRedactor
}

// newCapACLFixture seeds st: PRJ-1 is sequential with ACT-1 (order 9),
// ACT-2 (order 1) and the hidden ACT-H* (order 0), all open. ACT-1 depends
// on the on-hold PRJ-H and on every hidden action. PERS-B is shared ACT-1
// and ACT-2 only.
func newCapACLFixture(t *testing.T, st store.Store) capACLFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(capACLMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	s := nestedSeed{t: t, st: st}
	child := func(id string, order int) {
		t.Helper()
		if _, cerr := st.CreateRelation(t.Context(), entity.RelationKey{From: id, Type: "child_of", To: "PRJ-1"},
			&store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: order}}); cerr != nil {
			t.Fatal(cerr)
		}
	}
	s.mk("PERS-A", "person", nil)
	s.mk("PERS-B", "person", nil)
	s.mk("PRJ-1", "project", map[string]any{"ordering": "sequential"})
	s.mk("PRJ-H", "project", map[string]any{"status": "on_hold"})
	s.mk("ACT-1", "action", nil)
	s.mk("ACT-2", "action", nil)
	child("ACT-1", 9)
	child("ACT-2", 1)
	s.rel("ACT-1", "depends_on", "PRJ-H")
	for i := range capACLHidden {
		id := fmt.Sprintf("ACT-H%d", i)
		s.mk(id, "action", nil)
		child(id, 0)
		s.rel("ACT-1", "depends_on", id)
	}
	s.rel("PERS-B", "shared-with", "ACT-1")
	s.rel("PERS-B", "shared-with", "ACT-2")
	policy, err := acl.ParsePolicy([]byte(capACLPolicy), "acl.yaml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(meta, edgeLookup{st}, d)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatal(err)
	}
	return capACLFixture{st: st, set: set, d: d, gate: gate, fields: fields}
}

// as binds the compiled policy for user on a fresh context.
func (f capACLFixture) as(t *testing.T, user string) context.Context {
	t.Helper()
	return acltest.Bind(principal.With(context.Background(), principal.Principal{
		User: user, Tool: principal.ToolMCP,
	}), t, f.d)
}

// rollups is the roll-up reader of the fixture with the row cap n.
func (f capACLFixture) rollups(t *testing.T, n int) *visibility.Rollups {
	t.Helper()
	r, err := visibility.NewRollups(f.set, f.st, f.gate, f.fields)
	if err != nil {
		t.Fatal(err)
	}
	return r.WithRowCap(n)
}

// absentLine renders prop on rows: the value, "-" when absent because it
// could not be computed, "." when it has no value.
func absentLine(values map[string]map[string]any, absent map[string]map[string]bool, rows []string, prop string) string {
	parts := make([]string, len(rows))
	for i, id := range rows {
		v, ok := values[id][prop]
		switch {
		case absent[id][prop]:
			parts[i] = "-"
		case !ok:
			parts[i] = "."
		default:
			parts[i] = fmt.Sprint(v)
		}
	}
	return strings.Join(parts, " ")
}

// assertCapACL pins, under a compiled acl.yaml, that rows a reader cannot
// read change none of its roll-up answers and none of its cap outcomes:
// along, siblings (with `_order_in` hidden on the hidden siblings' edges),
// nested and the store leaf. PERS-A, who reads the hidden actions, is over
// a cap of 3 on ACT-1's walk, its siblings and PRJ-1's nested charge;
// PERS-B holds at most 3 gated rows on each and keeps every answer, and
// at a cap of 2 is over it on the siblings and the nested charge only.
func assertCapACL(t *testing.T, st store.Store) {
	t.Helper()
	f := newCapACLFixture(t, st)
	acts := []string{"ACT-1", "ACT-2"}
	cases := []struct {
		user, held, waiting, free string
		cap                       int
	}{
		{"PERS-A", "true false", "true true", "true", 0},
		{"PERS-B", "true false", "true false", "true", 0},
		{"PERS-A", "- false", "- -", "-", 3},
		{"PERS-B", "true false", "true false", "true", 3},
		{"PERS-B", "true false", "- -", "-", 2},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s cap %d", tc.user, tc.cap), func(t *testing.T) {
			t.Parallel()
			ctx := f.as(t, tc.user)
			r := f.rollups(t, tc.cap)
			refs := []entity.Ref{{ID: "ACT-1"}, {ID: "ACT-2"}}
			v, a := r.ValuesOrAbsent(ctx, "action", refs, store.TrivialScope(), nil)
			if got := absentLine(v, a, acts, "held"); got != tc.held {
				t.Errorf("held = %q, want %q", got, tc.held)
			}
			if got := absentLine(v, a, acts, "waiting"); got != tc.waiting {
				t.Errorf("waiting = %q, want %q", got, tc.waiting)
			}
			v, a = r.ValuesOrAbsent(ctx, "project", []entity.Ref{{ID: "PRJ-1"}}, store.TrivialScope(), nil)
			if got := absentLine(v, a, []string{"PRJ-1"}, "has_free"); got != tc.free {
				t.Errorf("has_free = %q, want %q", got, tc.free)
			}
			assertLeafMatchesValues(ctx, t, f, r, "held", tc.held)
			assertLeafMatchesValues(ctx, t, f, r, "waiting", tc.waiting)
		})
	}
}

// assertLeafMatchesValues pins that the store leaf of prop agrees with its
// column on ACT-1 and ACT-2 under every polarity: a true value matches
// `== true` and `not (== false)`, a false one the reverse, and an absent
// one matches none of the four.
func assertLeafMatchesValues(
	ctx context.Context, t *testing.T, f capACLFixture, r *visibility.Rollups, prop, want string,
) {
	t.Helper()
	q, ok, err := r.LeafQuery(ctx, "action", prop, store.TrivialScope())
	if err != nil || !ok {
		t.Fatalf("LeafQuery(%s) = ok %v, err %v", prop, ok, err)
	}
	vals := strings.Fields(want)
	for _, form := range []struct {
		name  string
		where store.Cond
		holds string
	}{
		{"== true", store.RollupLeaf(q, true), "true"},
		{"== false", store.RollupLeaf(q, false), "false"},
		{"not == false", store.NotCond(store.RollupLeaf(q, false)), "true"},
		{"not == true", store.NotCond(store.RollupLeaf(q, true)), "false"},
	} {
		var wantIDs []string
		for i, id := range []string{"ACT-1", "ACT-2"} {
			if vals[i] == form.holds {
				wantIDs = append(wantIDs, id)
			}
		}
		var got []string
		for e, qerr := range f.st.GraphQuery(ctx, store.GraphQuery{EntityType: "action", Faces: store.AllFaces(), Where: &form.where}) {
			if qerr != nil {
				t.Fatalf("%s %s: %v", prop, form.name, qerr)
			}
			if e.ID == "ACT-1" || e.ID == "ACT-2" {
				got = append(got, e.ID)
			}
		}
		slices.Sort(got)
		if fmt.Sprint(got) != fmt.Sprint(wantIDs) {
			t.Errorf("%s %s = %v, want %v", prop, form.name, got, wantIDs)
		}
	}
}

// TestRollups_CapACL runs the cap conformance under a compiled acl.yaml on
// memstore; the sqlite and postgres twins run the same assertions.
func TestRollups_CapACL(t *testing.T) {
	t.Parallel()
	assertCapACL(t, memstore.New())
}
