package visibility_test

import (
	"context"
	"errors"
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
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// hiddenFieldTickets is the number of tickets in a hidden-field fixture.
const hiddenFieldTickets = 24

// hiddenFieldFixture is a ScriptReader whose policy shows some fields on
// some rows only (TKT-9BKVQ9I):
//
//   - alice (staff, global) sees secret and prio only on open tickets;
//   - bob is assignee on T-00..T-05, which shows every field, and watcher
//     on T-06..T-11, which shows title and status only;
//   - erin is watcher on T-00..T-03 and assignee on T-02..T-05, so the two
//     local roles overlap on T-02 and T-03.
//
// A faced type, doc, is read by dora through the global role drafter at its
// draft face only, which has no `visible:` block, and through the role
// editor of her group at every face, which shows no field. So title shows to her
// on the draft rows only: a field grant limited to one face.
type hiddenFieldFixture struct {
	sr *visibility.ScriptReader
	d  *acl.Declarative
}

func newHiddenFieldFixture(t *testing.T, base store.Store) hiddenFieldFixture {
	t.Helper()
	ctx := context.Background()
	mk := func(e *entity.Entity) {
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	rel := func(from, typ, to string) {
		if _, err := base.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"alice", "bob", "erin", "dora", "editors"} {
		mk(&entity.Entity{ID: u, Type: "person"})
	}
	for i := range hiddenFieldTickets {
		id := fmt.Sprintf("T-%02d", i)
		status := "open"
		if i%2 == 1 {
			status = "closed"
		}
		secret := "s"
		if i%3 == 0 {
			secret = "t"
		}
		props := map[string]any{"title": id, "status": status, "secret": secret, "prio": i % 7}
		if i%5 == 4 {
			delete(props, "secret")
		}
		mk(&entity.Entity{ID: id, Type: "ticket", Properties: props})
		switch {
		case i < 6:
			rel("bob", "assigned-to", id)
		case i < 12:
			rel("bob", "watches", id)
		}
		if i < 4 {
			rel("erin", "watches", id)
		}
		if i >= 2 && i < 6 {
			rel("erin", "assigned-to", id)
		}
	}
	for _, r := range []entity.Ref{
		{ID: "D-1", Face: "draft"}, {ID: "D-1", Face: "final"}, {ID: "D-2", Face: "draft"}, {ID: "D-2", Face: "final"},
	} {
		mk(&entity.Entity{ID: r.ID, Type: "doc", Face: r.Face, Properties: map[string]any{"title": r.ID}})
	}
	rel("dora", "member-of", "editors")
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"}, "status": {Type: "string"}, "secret": {Type: "string"},
				"prio": {Type: "integer"},
			}},
			"doc": {
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
				Faces:      map[string]metamodel.FaceDef{"draft": {}, "final": {}},
			},
			"person": {},
		},
		Relations: map[string]metamodel.RelationDef{
			"assigned-to": {From: []string{"person"}, To: []string{"ticket"}},
			"watches":     {From: []string{"person"}, To: []string{"ticket"}},
			"member-of":   {From: []string{"person"}, To: []string{"person"}},
		},
	}
	open := "entity.status == 'open'"
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"staff": {
				Read: []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"}, {Field: "status"}, {Field: "secret", When: open}, {Field: "prio", When: open},
				}},
			},
			"assignee": {
				Read: []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"}, {Field: "status"}, {Field: "secret"}, {Field: "prio"},
				}},
			},
			"watcher": {
				Read:    []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}, {Field: "status"}}},
			},
			"drafter": {Read: []string{"doc@draft"}},
			"editor": {
				Read:    []string{"doc"},
				Visible: map[string][]acl.FieldGrant{"doc": {}},
			},
		},
		Assignments: map[string]string{"alice": "staff", "dora": "drafter", "editors": "editor"},
		RoleRelations: map[string]acl.RoleRelationDef{
			"assigned-to": {Confers: "assignee"},
			"watches":     {Confers: "watcher"},
		},
	}, aclstore.MustNewGraph(base), base)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(meta, edgeLookup{base}, d)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	redact, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := visibility.NewPolicyReader(gate, redact, base)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(reader, base, gate)
	if err != nil {
		t.Fatal(err)
	}
	return hiddenFieldFixture{sr: sr, d: d}
}

func (f hiddenFieldFixture) as(t *testing.T, user string) context.Context {
	t.Helper()
	return acltest.Bind(principal.With(context.Background(), principal.Principal{
		User: user, Tool: principal.ToolMCP,
	}), t, f.d)
}

// TestScriptReader_ListEntityPage_HiddenFields: a filter on a property the
// reader sees on some rows only is answered in the store, and keeps exactly
// the rows the filter keeps on the rows as served, where the hidden value
// is unset.
func TestScriptReader_ListEntityPage_HiddenFields(t *testing.T) {
	t.Parallel()
	assertHiddenFieldPages(t, newHiddenFieldFixture(t, memstore.New()))
}

// hiddenFieldFilters are the filters the hidden-field tests run, in the
// shapes queryplan.LowerCondition emits.
func hiddenFieldFilters() []struct {
	name  string
	props []store.PropPredicate
	where *store.Cond
} {
	eq := func(p, v string) store.PropPredicate {
		return store.PropPredicate{Property: p, Op: store.PropEqual, Value: v, Scalar: true}
	}
	ne := func(p, v string) store.PropPredicate {
		return store.PropPredicate{Property: p, Op: store.PropNotEqualOrEmpty, Value: v, Scalar: true}
	}
	prio := func(op store.PropOp, v string) store.PropPredicate {
		return store.PropPredicate{Property: "prio", Op: op, Value: v, Kind: store.OrderInteger}
	}
	cond := func(c store.Cond) *store.Cond { return &c }
	return []struct {
		name  string
		props []store.PropPredicate
		where *store.Cond
	}{
		{name: "equality", props: []store.PropPredicate{eq("secret", "s")}},
		{name: "not equal, true when unset", props: []store.PropPredicate{ne("secret", "s")}},
		{name: "integer equality is two comparisons", props: []store.PropPredicate{
			prio(store.PropGreaterEqual, "3"), prio(store.PropLessEqual, "3"),
		}},
		{name: "ordered", props: []store.PropPredicate{prio(store.PropLess, "4")}},
		{name: "with a visible field", props: []store.PropPredicate{eq("status", "closed"), ne("secret", "t")}},
		{name: "not", where: cond(store.NotCond(store.PropLeaf(eq("secret", "s"))))},
		{name: "or across fields", where: cond(store.OrCond(
			store.PropLeaf(eq("secret", "t")), store.PropLeaf(prio(store.PropGreater, "4"))))},
		{name: "integer not equal", where: cond(store.NotCond(store.AndCond(
			store.PropLeaf(prio(store.PropGreaterEqual, "2")),
			store.PropLeaf(prio(store.PropLessEqual, "2")))))},
	}
}

func assertHiddenFieldPages(t *testing.T, f hiddenFieldFixture) {
	t.Helper()
	for _, user := range []string{"alice", "bob", "erin"} {
		for _, tc := range hiddenFieldFilters() {
			t.Run(user+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				ctx := f.as(t, user)
				want := servedMatches(ctx, t, f.sr, tc.props, tc.where)
				page, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{
					Type: "ticket", Faces: store.AllFaces(), Props: tc.props, Where: pageCond(tc.where),
					Limit: hiddenFieldTickets,
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := pageIDs(page); !slices.Equal(got, want) || page.Total != len(want) {
					t.Fatalf("got %v (total %d), want %v", got, page.Total, want)
				}
				if len(want) < 2 {
					return
				}
				// Paging cuts the matched rows and keeps their total.
				small, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{
					Type: "ticket", Faces: store.AllFaces(), Props: tc.props, Where: pageCond(tc.where),
					Limit: 1, Offset: 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := pageIDs(small); !slices.Equal(got, want[1:2]) || small.Total != len(want) {
					t.Fatalf("paged: got %v (total %d), want %v of %d", got, small.Total, want[1:2], len(want))
				}
			})
		}
	}
}

// servedMatches is the reference: the ids of the rows the reader is served,
// unfiltered and redacted, whose served properties satisfy the filter with
// the store's comparison semantics.
func servedMatches(
	ctx context.Context, t *testing.T, sr *visibility.ScriptReader, props []store.PropPredicate, where *store.Cond,
) []string {
	t.Helper()
	all, err := sr.ListEntityPage(ctx, visibility.PageQuery{
		Type: "ticket", Faces: store.AllFaces(), Limit: hiddenFieldTickets,
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all.Rows {
		ok, err := graphquerynaive.MatchProps(e.Properties, props)
		if err != nil {
			t.Fatal(err)
		}
		if ok && where != nil {
			ok = evalPropCond(t, e.Properties, *where)
		}
		if ok {
			out = append(out, e.ID)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func evalPropCond(t *testing.T, props map[string]any, c store.Cond) bool {
	t.Helper()
	switch c.Op {
	case store.CondProp:
		ok, err := graphquerynaive.MatchProps(props, []store.PropPredicate{c.Prop})
		if err != nil {
			t.Fatal(err)
		}
		return ok
	case store.CondNot:
		return !evalPropCond(t, props, c.Args[0])
	case store.CondAnd:
		for _, a := range c.Args {
			if !evalPropCond(t, props, a) {
				return false
			}
		}
		return true
	case store.CondOr:
		for _, a := range c.Args {
			if evalPropCond(t, props, a) {
				return true
			}
		}
		return false
	default:
		t.Fatalf("unexpected op %d", c.Op)
		return false
	}
}

// pageCond is c as a page filter: it has property leaves only.
func pageCond(c *store.Cond) *visibility.PageCond {
	if c == nil {
		return nil
	}
	var conv func(store.Cond) visibility.PageCond
	conv = func(c store.Cond) visibility.PageCond {
		out := visibility.PageCond{Op: c.Op, Prop: c.Prop}
		for _, a := range c.Args {
			out.Args = append(out.Args, conv(a))
		}
		return out
	}
	pc := conv(*c)
	return &pc
}

// A field grant limited to some faces is decided against the faces a
// returned row can have: dora sees title at the draft face only. A read of
// the draft face is answered; a read of every face, where the grant covers
// the faces in part, is refused, never answered approximately.
func TestScriptReader_ListEntityPage_HiddenFieldFaces(t *testing.T) {
	t.Parallel()
	f := newHiddenFieldFixture(t, memstore.New())
	ctx := f.as(t, "dora")
	title := func(v string) []store.PropPredicate {
		return []store.PropPredicate{{Property: "title", Op: store.PropEqual, Value: v, Scalar: true}}
	}
	all, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{Type: "doc", Faces: store.AllFaces(), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageIDs(all); !slices.Equal(got, []string{"D-1@draft", "D-1@final", "D-2@draft", "D-2@final"}) {
		t.Fatalf("dora reads %v", got)
	}
	for _, r := range all.Rows {
		if _, shown := r.Properties["title"]; shown != (r.Face == "draft") {
			t.Fatalf("%s@%s: title shown = %v", r.ID, r.Face, shown)
		}
	}
	page, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{
		Type: "doc", Faces: store.AtFaces("draft"), Props: title("D-1"), Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageIDs(page); !slices.Equal(got, []string{"D-1@draft"}) || page.Total != 1 {
		t.Fatalf("draft face: got %v (total %d)", got, page.Total)
	}
	_, err = f.sr.ListEntityPage(ctx, visibility.PageQuery{
		Type: "doc", Faces: store.AllFaces(), Props: title("D-1"), Limit: 5,
	})
	if !errors.Is(err, visibility.ErrPropsNotPushable) {
		t.Fatalf("every face: err = %v, want ErrPropsNotPushable", err)
	}
	_, err = f.sr.ListEntityPage(f.as(t, "alice"), visibility.PageQuery{
		Type: "ticket", Faces: store.AllFaces(), Limit: 5,
		Props: []store.PropPredicate{{Property: "secret", Op: store.PropEqual, Value: "", Scalar: true}},
	})
	if !errors.Is(err, visibility.ErrPropsNotPushable) {
		t.Fatalf("an empty-valued equality on a hidden field: err = %v, want ErrPropsNotPushable", err)
	}
}

// Each guarded subtree carries its own copy of the grant condition, so a
// filter with many of them can outgrow the store's tree limits once field
// visibility is applied. It is refused, not answered in part.
func TestScriptReader_ListEntityPage_HiddenFieldTooLarge(t *testing.T) {
	t.Parallel()
	f := newHiddenFieldFixture(t, memstore.New())
	var arms []visibility.PageCond
	for i := range 12 {
		arms = append(arms, visibility.PageCond{Op: store.CondAnd, Args: []visibility.PageCond{
			{Op: store.CondProp, Prop: store.PropPredicate{
				Property: "secret", Op: store.PropEqual, Value: fmt.Sprint("v", i), Scalar: true,
			}},
			{Op: store.CondProp, Prop: store.PropPredicate{
				Property: "status", Op: store.PropEqual, Value: "open", Scalar: true,
			}},
		}})
	}
	_, err := f.sr.ListEntityPage(f.as(t, "erin"), visibility.PageQuery{
		Type: "ticket", Faces: store.AllFaces(), Limit: 5, Where: &visibility.PageCond{Op: store.CondOr, Args: arms},
	})
	if !errors.Is(err, visibility.ErrPropsNotPushable) || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v, want ErrPropsNotPushable for a filter too large", err)
	}
}
