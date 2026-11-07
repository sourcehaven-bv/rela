package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// rollupNames is a [visibility.RollupNamer] over a fixed map.
type rollupNames map[string][]string

func (m rollupNames) Names(entityType string) []string { return m[entityType] }

// elementsPolicy: root reads and sees everything. alice reads every ticket
// but sees only the title, and sees the salary too on the tickets assigned
// to her. bob reads only the tickets assigned to him, and sees all of them.
func elementsPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"admin": {Read: []string{"*"}},
			"reader": {
				Read:    []string{"*"},
				Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
			},
			"assignee": {
				Read:    []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}, {Field: "salary"}}},
			},
			"member": {Read: []string{"person"}},
		},
		Assignments:   map[string]string{"root": "admin", "alice": "reader", "bob": "member"},
		RoleRelations: map[string]acl.RoleRelationDef{"assigned-to": {Confers: "assignee"}},
	}
}

func elementsMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"}, "salary": {Type: "string"}, "progress": {Type: "string"},
			}},
			"person": {},
		},
		Relations: map[string]metamodel.RelationDef{
			"assigned-to": {From: []string{"person"}, To: []string{"ticket"}},
		},
	}
}

type elementsFixture struct {
	t  *testing.T
	st store.Store
	d  *acl.Declarative
}

// newElementsFixture seeds n tickets, each with a title and a salary,
// alice and bob, and assigns TKT-000 to both.
func newElementsFixture(t *testing.T, st store.Store, n int, policy *acl.Policy) *elementsFixture {
	t.Helper()
	ctx := context.Background()
	f := &elementsFixture{t: t, st: st}
	f.create(&entity.Entity{ID: "alice", Type: "person"})
	f.create(&entity.Entity{ID: "bob", Type: "person"})
	for i := range n {
		id := fmt.Sprintf("TKT-%03d", i)
		f.create(&entity.Entity{
			ID: id, Type: "ticket", Content: "Body of " + id,
			Properties: map[string]any{"title": "T " + id, "salary": "s" + id},
		})
	}
	for _, who := range []string{"alice", "bob"} {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: who, Type: "assigned-to", To: "TKT-000"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	f.d = d
	return f
}

func (f *elementsFixture) create(e *entity.Entity) {
	f.t.Helper()
	if err := f.st.CreateEntity(context.Background(), e); err != nil {
		f.t.Fatal(err)
	}
}

// set sets prop on id's row to v, or removes it when v is nil: a key that is
// present with a nil value still counts as set.
func (f *elementsFixture) set(id, prop string, v any) {
	f.t.Helper()
	ctx := context.Background()
	e, err := f.st.GetEntity(ctx, entity.Ref{ID: id})
	if err != nil {
		f.t.Fatal(err)
	}
	c := *e
	c.Properties = maps.Clone(e.Properties)
	if v == nil {
		delete(c.Properties, prop)
	} else {
		c.Properties[prop] = v
	}
	if err := f.st.UpdateEntity(ctx, &c); err != nil {
		f.t.Fatal(err)
	}
}

// elements builds the read over ranges (the store's own reader when nil).
func (f *elementsFixture) elements(ranges visibility.ElementRangeReader, rollups rollupNames, maxRows int) *visibility.Elements {
	f.t.Helper()
	if ranges == nil {
		var err error
		if ranges, err = elementnaive.New(f.st); err != nil {
			f.t.Fatal(err)
		}
	}
	resolver, err := affordances.New(elementsMeta(), edgeLookup{f.st}, f.d)
	if err != nil {
		f.t.Fatal(err)
	}
	gate, err := visibility.NewDeclarativeGate(f.d, store.TrivialScope())
	if err != nil {
		f.t.Fatal(err)
	}
	policy, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		f.t.Fatal(err)
	}
	if rollups == nil {
		rollups = rollupNames{}
	}
	x, err := visibility.NewElements(visibility.ElementDeps{
		Ranges: ranges, Rows: f.st, Reads: gate, Policy: policy, Rollups: rollups, MaxRows: maxRows,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return x
}

func (f *elementsFixture) as(user string) context.Context {
	f.t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	return acltest.Bind(ctx, f.t, f.d)
}

var (
	ticketTypes = visibility.ElementTypes{Entities: []string{"person", "ticket"}}
	allEntities = store.ElementRange{Unbounded: true}
)

func fingerprint(ctx context.Context, t *testing.T, x *visibility.Elements, types visibility.ElementTypes,
	r store.ElementRange,
) reconcile.Range {
	t.Helper()
	got, err := x.Fingerprint(ctx, types, r)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	return got
}

// A change to a property the reader cannot see leaves the reader's
// fingerprint alone; a change to one the reader sees moves it.
func TestElements_HiddenFieldChangeKeepsFingerprint(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 5, elementsPolicy())
	x := f.elements(nil, nil, 0)
	alice, root := f.as("alice"), f.as("root")

	aliceBefore := fingerprint(alice, t, x, ticketTypes, allEntities)
	rootBefore := fingerprint(root, t, x, ticketTypes, allEntities)
	if aliceBefore == rootBefore {
		t.Fatal("alice and root got the same fingerprint, though alice sees fewer fields")
	}
	if aliceBefore.Count != rootBefore.Count {
		t.Fatalf("counts differ: alice %d, root %d", aliceBefore.Count, rootBefore.Count)
	}

	f.set("TKT-002", "salary", "changed") // hidden from alice on TKT-002
	if got := fingerprint(alice, t, x, ticketTypes, allEntities); got != aliceBefore {
		t.Error("a hidden salary change moved alice's fingerprint")
	}
	rootAfter := fingerprint(root, t, x, ticketTypes, allEntities)
	if rootAfter == rootBefore {
		t.Error("root's fingerprint did not move on a salary change")
	}

	f.set("TKT-002", "salary", nil) // removing a hidden value is invisible too
	if got := fingerprint(alice, t, x, ticketTypes, allEntities); got != aliceBefore {
		t.Error("removing a hidden salary moved alice's fingerprint")
	}

	f.set("TKT-000", "salary", "changed") // alice is the assignee here and sees it
	aliceAssigned := fingerprint(alice, t, x, ticketTypes, allEntities)
	if aliceAssigned == aliceBefore {
		t.Error("a salary change alice can see did not move her fingerprint")
	}

	f.set("TKT-003", "title", "renamed")
	if got := fingerprint(alice, t, x, ticketTypes, allEntities); got == aliceAssigned {
		t.Error("a title change did not move alice's fingerprint")
	}
}

// Each element is the reader's view hash: the redacted row, hashed as the
// ETag hashes it. List and Fingerprint agree.
func TestElements_ElementsAreRedactedViews(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 4, elementsPolicy())
	x := f.elements(nil, nil, 0)
	ctx := context.Background()
	alice := f.as("alice")

	page, err := x.List(alice, ticketTypes, allEntities, 100)
	if err != nil {
		t.Fatal(err)
	}
	els := page.Elements
	if page.More || page.Last != els[len(els)-1].Address {
		t.Errorf("page More=%v Last=%q, want the last element and no more", page.More, page.Last)
	}
	want := map[string]reconcile.Element{}
	for _, id := range []string{"alice", "bob", "TKT-000", "TKT-001", "TKT-002", "TKT-003"} {
		e, err := f.st.GetEntity(ctx, entity.Ref{ID: id})
		if err != nil {
			t.Fatal(err)
		}
		view := *e
		if id != "TKT-000" && e.Type == "ticket" {
			view.Properties = map[string]any{"title": e.Properties["title"]}
		}
		el := reconcile.EntityElement(view)
		want[el.Address] = el
	}
	if len(els) != len(want) {
		t.Fatalf("listed %d elements, want %d", len(els), len(want))
	}
	var sum reconcile.Range
	for i, el := range els {
		if i > 0 && els[i-1].Address >= el.Address {
			t.Errorf("not in address order: %q then %q", els[i-1].Address, el.Address)
		}
		if want[el.Address] != el {
			t.Errorf("%s: element is not the redacted view", el.Address)
		}
		sum = sum.Add(el)
	}
	if got := fingerprint(alice, t, x, ticketTypes, allEntities); got != sum {
		t.Errorf("Fingerprint %v != sum of List %v", got, sum)
	}
}

// For a reader with no hidden field, the Go path (forced by a roll-up name
// no row stores) and the stored path agree.
func TestElements_GoPathAgreesWithStoredPath(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 6, elementsPolicy())
	stored := f.elements(nil, nil, 0)
	inGo := f.elements(nil, rollupNames{"ticket": {"progress"}}, 0)
	root := f.as("root")
	for _, r := range []store.ElementRange{
		allEntities,
		{Lo: "ticket/TKT-001", Hi: "ticket/TKT-004"},
		{Lo: "person/", Hi: "ticket/TKT-002"},
	} {
		a := fingerprint(root, t, stored, ticketTypes, r)
		b := fingerprint(root, t, inGo, ticketTypes, r)
		if a != b || a.Count == 0 {
			t.Errorf("range %+v: stored path %v, Go path %v", r, a, b)
		}
	}
}

// A value stored under a roll-up name is never part of a view.
func TestElements_StoredRollupValueLeftOut(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 2, elementsPolicy())
	x := f.elements(nil, rollupNames{"ticket": {"progress"}}, 0)
	root := f.as("root")
	before := fingerprint(root, t, x, ticketTypes, allEntities)
	f.set("TKT-001", "progress", "50%")
	if got := fingerprint(root, t, x, ticketTypes, allEntities); got != before {
		t.Error("a value stored under a roll-up name moved the fingerprint")
	}
}

// The too-large refusal depends on the row count and the scope, never on
// whether a hidden value is set.
func TestElements_RowCap(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 5, elementsPolicy())
	x := f.elements(nil, nil, 3)
	types := visibility.ElementTypes{Entities: []string{"ticket"}}

	if _, err := x.Fingerprint(f.as("alice"), types, allEntities); !errors.Is(err, visibility.ErrRangeTooLarge) {
		t.Errorf("alice over the cap: err = %v, want ErrRangeTooLarge", err)
	}
	for i := range 5 {
		f.set(fmt.Sprintf("TKT-%03d", i), "salary", nil) // no hidden value is set anywhere now
	}
	if _, err := x.Fingerprint(f.as("alice"), types, allEntities); !errors.Is(err, visibility.ErrRangeTooLarge) {
		t.Errorf("alice over the cap with nothing hidden set: err = %v, want ErrRangeTooLarge", err)
	}
	if got := fingerprint(f.as("root"), t, x, types, allEntities); got.Count != 5 {
		t.Errorf("root (one aggregate) count = %d, want 5", got.Count)
	}
	if _, err := x.Fingerprint(f.as("alice"), types, store.ElementRange{Lo: "ticket/", Hi: "ticket/TKT-002"}); err != nil {
		t.Errorf("alice under the cap: %v", err)
	}
	for _, limit := range []int{0, 4} {
		if _, err := x.List(f.as("root"), types, allEntities, limit); !errors.Is(err, store.ErrInvalidQuery) {
			t.Errorf("List limit %d: err = %v, want ErrInvalidQuery", limit, err)
		}
	}
}

// bob reads TKT-000 only. A row that stops being readable between the
// listing and the load is left out and not counted.
func TestElements_RegatesLoadedRows(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 3, elementsPolicy())
	naive, err := elementnaive.New(f.st)
	if err != nil {
		t.Fatal(err)
	}
	bob := f.as("bob")
	types := visibility.ElementTypes{Entities: []string{"ticket"}}

	plain := f.elements(nil, rollupNames{"ticket": {"progress"}}, 0)
	if got := fingerprint(bob, t, plain, types, allEntities); got.Count != 1 {
		t.Fatalf("bob reads %d tickets, want 1", got.Count)
	}

	unassign := &afterList{ElementRangeReader: naive, after: func() {
		if err := f.st.DeleteRelation(context.Background(),
			entity.RelationKey{From: "bob", Type: "assigned-to", To: "TKT-000"}); err != nil {
			t.Fatal(err)
		}
	}}
	x := f.elements(unassign, rollupNames{"ticket": {"progress"}}, 0)
	if got := fingerprint(bob, t, x, types, allEntities); got != (reconcile.Range{}) {
		t.Errorf("a row bob lost between list and load was folded: %v", got)
	}
}

// A row deleted between the listing and the load is left out.
func TestElements_DropsRowGoneAtLoad(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 3, elementsPolicy())
	naive, err := elementnaive.New(f.st)
	if err != nil {
		t.Fatal(err)
	}
	gone := &afterList{ElementRangeReader: naive, after: func() {
		if _, derr := f.st.DeleteFamily(context.Background(), "TKT-001", true); derr != nil {
			t.Fatal(derr)
		}
	}}
	x := f.elements(gone, nil, 0)
	page, err := x.List(f.as("alice"), visibility.ElementTypes{Entities: []string{"ticket"}}, allEntities, 2)
	if err != nil {
		t.Fatal(err)
	}
	// The store listed TKT-000 and TKT-001; TKT-001 was gone at load. The
	// page still says where the listing stopped, so the caller moves on.
	if len(page.Elements) != 1 || !page.More || page.Last != "ticket/TKT-001" {
		t.Errorf("page = %d elements, More=%v, Last=%q; want 1, true, ticket/TKT-001",
			len(page.Elements), page.More, page.Last)
	}
}

// afterList runs after once, after the first ListRange.
type afterList struct {
	visibility.ElementRangeReader
	after func()
	done  bool
}

func (a *afterList) ListRange(
	ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	out, err := a.ElementRangeReader.ListRange(ctx, scope, r, l)
	if !a.done {
		a.done = true
		a.after()
	}
	return out, err
}

func TestElements_Relations(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 3, elementsPolicy())
	x := f.elements(nil, nil, 0)
	types := visibility.ElementTypes{Entities: []string{"person", "ticket"}, Relations: []string{"assigned-to"}}
	rels := store.ElementRange{Relations: true, Unbounded: true}

	if got := fingerprint(f.as("root"), t, x, types, rels); got.Count != 2 {
		t.Errorf("root: %d relations, want 2", got.Count)
	}
	// bob reads both persons and TKT-000, so both edges.
	if got := fingerprint(f.as("bob"), t, x, types, rels); got.Count != 2 {
		t.Errorf("bob: %d relations, want 2", got.Count)
	}
	page, err := x.List(f.as("root"), types, rels, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, el := range page.Elements {
		if el.Kind != reconcile.KindRelation {
			t.Errorf("%s: kind %q", el.Address, el.Kind)
		}
	}
}

func TestElements_RefusesMetaGatedRelation(t *testing.T) {
	p := elementsPolicy()
	admin := p.Roles["admin"]
	admin.Relations = map[string][]acl.RelationGrant{
		"person": {{Relation: "assigned-to", Visible: []acl.FieldGrant{{Field: "note"}}}},
	}
	p.Roles["admin"] = admin
	f := newElementsFixture(t, memstore.New(), 1, p)
	x := f.elements(nil, nil, 0)
	types := visibility.ElementTypes{Entities: []string{"person", "ticket"}, Relations: []string{"assigned-to"}}
	_, err := x.Fingerprint(f.as("bob"), types, store.ElementRange{Relations: true, Unbounded: true})
	if !errors.Is(err, visibility.ErrRelationMetaGated) {
		t.Errorf("err = %v, want ErrRelationMetaGated", err)
	}
}

func TestElements_FailsClosedWithoutRequest(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 1, elementsPolicy())
	x := f.elements(nil, nil, 0)
	unbound := principal.With(context.Background(), principal.Principal{User: "root", Tool: principal.ToolDataEntry})
	if _, err := x.Fingerprint(unbound, ticketTypes, allEntities); err == nil {
		t.Error("Fingerprint without a bound request succeeded")
	}
	if _, err := x.List(unbound, ticketTypes, allEntities, 10); err == nil {
		t.Error("List without a bound request succeeded")
	}
	// The check does not hang on a named entity type.
	for name, types := range map[string]visibility.ElementTypes{
		"none":           {},
		"relations only": {Relations: []string{"assigned-to"}},
	} {
		if _, err := x.Fingerprint(unbound, types, store.ElementRange{Relations: true, Unbounded: true}); err == nil {
			t.Errorf("%s: Fingerprint without a bound request succeeded", name)
		}
	}
}

func TestElements_EmptyAndUnnamed(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 2, elementsPolicy())
	x := f.elements(nil, nil, 0)
	root := f.as("root")
	if got := fingerprint(root, t, x, visibility.ElementTypes{}, allEntities); got != (reconcile.Range{}) {
		t.Errorf("no types: %v, want empty", got)
	}
	if got := fingerprint(root, t, x, ticketTypes, store.ElementRange{Lo: "b", Hi: "b"}); got != (reconcile.Range{}) {
		t.Errorf("empty range: %v, want empty", got)
	}
	page, err := x.List(root, visibility.ElementTypes{}, allEntities, 5)
	if err != nil || len(page.Elements) != 0 || page.More {
		t.Errorf("List with no types = %+v, %v", page, err)
	}
	if _, err := x.Fingerprint(root, ticketTypes, store.ElementRange{Lo: "z", Hi: "a"}); !errors.Is(err, store.ErrInvalidQuery) {
		t.Errorf("inverted range: err = %v, want ErrInvalidQuery", err)
	}
}

func TestNewElements_Rejects(t *testing.T) {
	st := memstore.New()
	naive, err := elementnaive.New(st)
	if err != nil {
		t.Fatal(err)
	}
	full := visibility.ElementDeps{
		Ranges: naive, Rows: st, Reads: visibility.AllowAllReadQueries{},
		Policy: visibility.NopRedactor{}, Rollups: rollupNames{},
	}
	if _, err := visibility.NewElements(full); err != nil {
		t.Fatalf("complete deps: %v", err)
	}
	for name, mut := range map[string]func(*visibility.ElementDeps){
		"ranges":   func(d *visibility.ElementDeps) { d.Ranges = nil },
		"rows":     func(d *visibility.ElementDeps) { d.Rows = nil },
		"reads":    func(d *visibility.ElementDeps) { d.Reads = nil },
		"policy":   func(d *visibility.ElementDeps) { d.Policy = nil },
		"rollups":  func(d *visibility.ElementDeps) { d.Rollups = nil },
		"max low":  func(d *visibility.ElementDeps) { d.MaxRows = -1 },
		"max high": func(d *visibility.ElementDeps) { d.MaxRows = store.MaxRangeList },
	} {
		d := full
		mut(&d)
		if _, err := visibility.NewElements(d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Without a policy every row folds its stored element: one aggregate.
func TestElements_NopPolicy(t *testing.T) {
	st := storetest.NewCounting(memstore.New())
	ctx := context.Background()
	for i := range 3 {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: fmt.Sprintf("N-%d", i), Type: "note"}); err != nil {
			t.Fatal(err)
		}
	}
	naive, err := elementnaive.New(st)
	if err != nil {
		t.Fatal(err)
	}
	x, err := visibility.NewElements(visibility.ElementDeps{
		Ranges: naive, Rows: st, Reads: visibility.AllowAllReadQueries{},
		Policy: visibility.NopRedactor{}, Rollups: rollupNames{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := x.Fingerprint(ctx, visibility.ElementTypes{Entities: []string{"note"}}, allEntities)
	if err != nil {
		t.Fatal(err)
	}
	var want reconcile.Range
	for i := range 3 {
		e, err := st.GetEntity(ctx, entity.Ref{ID: fmt.Sprintf("N-%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		want = want.Add(reconcile.EntityElement(*e))
	}
	if got != want {
		t.Errorf("Fingerprint = %v, want %v", got, want)
	}
}
