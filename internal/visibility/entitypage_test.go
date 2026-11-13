package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// pageAssigned is how many tickets bob is assigned to in a page fixture.
const pageAssigned = 12

// pageFixture is a ScriptReader over a counting store of n tickets, and the
// policy that gates it: alice reads every ticket and every field, bob reads
// the tickets assigned to him and only their title, carol reads nothing.
// Two policies are faced: POL-1 at draft and published, POL-2 at draft.
// dave reads only the draft face of a policy.
type pageFixture struct {
	st *storetest.Counting
	sr *visibility.ScriptReader
	d  *acl.Declarative
}

func newPageFixture(t *testing.T, n int) pageFixture {
	t.Helper()
	return newPageFixtureOn(t, memstore.New(), n)
}

// newPageFixtureOn is newPageFixture over base, which must be empty.
func newPageFixtureOn(t *testing.T, base store.Store, n int) pageFixture {
	t.Helper()
	ctx := context.Background()
	mk := func(e *entity.Entity) {
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"alice", "bob", "carol", "dave"} {
		mk(&entity.Entity{ID: u, Type: "person"})
	}
	for _, r := range []entity.Ref{{ID: "POL-1", Face: "draft"}, {ID: "POL-1", Face: "published"}, {ID: "POL-2", Face: "draft"}} {
		mk(&entity.Entity{ID: r.ID, Type: "policy", Face: r.Face, Properties: map[string]any{"title": r.ID}})
	}
	for i := range n {
		id := fmt.Sprintf("TKT-%04d", i)
		status := "open"
		if i%2 == 1 {
			status = "closed"
		}
		mk(&entity.Entity{ID: id, Type: "ticket", Content: "body",
			Properties: map[string]any{"title": id, "status": status, "secret": "s"}})
		if i < pageAssigned {
			if _, err := base.CreateRelation(ctx, entity.RelationKey{From: "bob", Type: "assigned-to", To: id}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"}, "status": {Type: "string"}, "secret": {Type: "string"},
			}},
			"policy": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
			"person": {},
		},
		Relations: map[string]metamodel.RelationDef{
			"assigned-to": {From: []string{"person"}, To: []string{"ticket"}},
		},
	}
	st := storetest.NewCounting(base)
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader": {Read: []string{"*"}},
			"assignee": {
				Read:    []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
			},
			"drafter": {Read: []string{"policy@draft"}},
		},
		Assignments:   map[string]string{"alice": "reader", "dave": "drafter"},
		RoleRelations: map[string]acl.RoleRelationDef{"assigned-to": {Confers: "assignee"}},
	}, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(meta, edgeLookup{st}, d)
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
	reader, err := visibility.NewPolicyReader(gate, redact, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(reader, st, gate)
	if err != nil {
		t.Fatal(err)
	}
	return pageFixture{st: st, sr: sr, d: d}
}

func (f pageFixture) as(t *testing.T, user string) context.Context {
	t.Helper()
	return acltest.Bind(principal.With(context.Background(), principal.Principal{
		User: user, Tool: principal.ToolMCP,
	}), t, f.d)
}

// pageIDs names the page's rows, as ID@face for a faced row.
func pageIDs(p visibility.EntityPage) []string {
	ids := make([]string, len(p.Rows))
	for i, e := range p.Rows {
		ids[i] = e.ID
		if e.Face != "" {
			ids[i] += "@" + e.Face.String()
		}
	}
	return ids
}

var openStatus = []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}}

// ListEntityPage serves the page and the total for the caller: the row gate
// and the filter run in the store, the field gate on the page, and the total
// counts only the rows the caller may read (TKT-D9UO9Y4).
func TestScriptReader_ListEntityPage(t *testing.T) {
	t.Parallel()
	assertEntityPages(t, newPageFixture(t, 40))
}

// assertEntityPages checks the pages of a 40-ticket fixture for each reader.
func assertEntityPages(t *testing.T, f pageFixture) {
	t.Helper()
	world := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	}))
	for _, tc := range []struct {
		name      string
		user      string
		q         visibility.PageQuery
		wantIDs   []string
		wantTotal int
		wantErr   error
	}{
		{
			name: "global reader pages the type", user: "alice",
			q:       visibility.PageQuery{Limit: 3, Offset: 2},
			wantIDs: []string{"TKT-0002", "TKT-0003", "TKT-0004"}, wantTotal: 40,
		},
		{
			name: "global reader filters in the store", user: "alice",
			q:       visibility.PageQuery{Props: openStatus, Limit: 3, Offset: 1},
			wantIDs: []string{"TKT-0002", "TKT-0004", "TKT-0006"}, wantTotal: 20,
		},
		{
			name: "offset past the end", user: "alice",
			q:         visibility.PageQuery{Limit: 5, Offset: 40},
			wantTotal: 40,
		},
		{
			name: "relation reader reads its rows only", user: "bob",
			q:       visibility.PageQuery{Limit: 2, Offset: 10},
			wantIDs: []string{"TKT-0010", "TKT-0011"}, wantTotal: pageAssigned,
		},
		{
			name: "a filter on a field hidden on some rows is refused", user: "bob",
			q:       visibility.PageQuery{Props: openStatus, Limit: 5},
			wantErr: visibility.ErrPropsNotPushable,
		},
		{
			name: "no read grant reads nothing", user: "carol",
			q: visibility.PageQuery{Limit: 5},
		},
		{
			name: "a world serves each family at its first face", user: "alice",
			q:       visibility.PageQuery{Type: "policy", Faces: world, Limit: 5},
			wantIDs: []string{"POL-1@published", "POL-2@draft"}, wantTotal: 2,
		},
		{
			name: "a face the reader may not read falls through before the rank", user: "dave",
			q:       visibility.PageQuery{Type: "policy", Faces: world, Limit: 5},
			wantIDs: []string{"POL-1@draft", "POL-2@draft"}, wantTotal: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.q.Type == "" {
				tc.q.Type, tc.q.Faces = "ticket", store.AllFaces()
			}
			page, err := f.sr.ListEntityPage(f.as(t, tc.user), tc.q)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got := pageIDs(page); !slices.Equal(got, tc.wantIDs) && len(got)+len(tc.wantIDs) > 0 {
				t.Errorf("rows = %v, want %v", got, tc.wantIDs)
			}
			if page.Total != tc.wantTotal {
				t.Errorf("total = %d, want %d", page.Total, tc.wantTotal)
			}
			for _, e := range page.Rows {
				if e.Content != "" {
					t.Errorf("%s: a page row carries its body", e.ID)
				}
				if tc.user == "bob" && (e.Properties["status"] != nil || e.Properties["secret"] != nil) {
					t.Errorf("%s: bob sees hidden fields: %v", e.ID, e.Properties)
				}
			}
		})
	}
}

// A page query needs a type and a positive limit: a page read never reads
// the whole type.
func TestListEntityPage_RejectsUnboundedQueries(t *testing.T) {
	t.Parallel()
	f := newPageFixture(t, 1)
	readers := map[string]interface {
		ListEntityPage(context.Context, visibility.PageQuery) (visibility.EntityPage, error)
	}{
		"script":       f.sr,
		"unrestricted": visibility.Unrestricted(f.st),
	}
	for name, r := range readers {
		for _, q := range []visibility.PageQuery{
			{Type: "ticket", Faces: store.AllFaces()},
			{Faces: store.AllFaces(), Limit: 5},
		} {
			if _, err := r.ListEntityPage(f.as(t, "alice"), q); err == nil {
				t.Errorf("%s: %+v was accepted", name, q)
			}
		}
	}
	if _, err := (visibility.DenyReader{}).ListEntityPage(context.Background(), visibility.PageQuery{
		Type: "ticket", Faces: store.AllFaces(), Limit: 5,
	}); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("DenyReader err = %v, want ErrReaderUnavailable", err)
	}
}

// The unrestricted reader pages and filters in the store, with no gate.
func TestUnrestrictedReader_ListEntityPage(t *testing.T) {
	t.Parallel()
	f := newPageFixture(t, 10)
	page, err := visibility.Unrestricted(f.st).ListEntityPage(context.Background(), visibility.PageQuery{
		Type: "ticket", Faces: store.AllFaces(), Props: openStatus, Limit: 2, Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageIDs(page); !slices.Equal(got, []string{"TKT-0002", "TKT-0004"}) || page.Total != 5 {
		t.Errorf("page = %v of %d, want [TKT-0002 TKT-0004] of 5", got, page.Total)
	}
}

// A page costs the same store reads over 100 tickets as over 1,000, for a
// global reader with a filter and for a relation-conferred reader whose
// field gate checks local roles.
func TestScriptReader_ListEntityPage_ScaleFree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, user string
		props      []store.PropPredicate
	}{
		{"alice", "alice", nil},
		{"alice filtered", "alice", openStatus},
		{"bob", "bob", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				f := newPageFixture(t, n)
				ctx := f.as(t, tc.user)
				f.st.Reset()
				page, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{
					Type: "ticket", Faces: store.AllFaces(), Props: tc.props, Limit: 10, Offset: 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(page.Rows) != 10 {
					t.Fatalf("page has %d rows, want 10", len(page.Rows))
				}
				return f.st.Cost(), f.st.String()
			})
		})
	}
}
