package visibility

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// countingRedactor records how many rows it redacted and strips "secret".
type countingRedactor struct{ calls int }

func (c *countingRedactor) redact(_ context.Context, e *entity.Entity) *entity.Entity {
	c.calls++
	if e == nil || e.Properties["secret"] == nil {
		return e
	}
	out := *e
	out.Properties = map[string]any{}
	for k, v := range e.Properties {
		if k != "secret" {
			out.Properties[k] = v
		}
	}
	return &out
}

// stubProvider returns a fixed ReadQueryResult.
type stubProvider struct {
	res acl.ReadQueryResult
	err error
}

func (s stubProvider) ReadQueryFor(context.Context, string) (acl.ReadQueryResult, error) {
	return s.res, s.err
}

// graphSpy is a store.Store stand-in that records whether GraphQuery ran.
type graphSpy struct {
	store.Store
	graphCalls int
	listCalls  int
	rows       []*entity.Entity
}

func (g *graphSpy) GraphQuery(context.Context, store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	g.graphCalls++
	return g.seq()
}

func (g *graphSpy) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	g.listCalls++
	return g.seq()
}

func (g *graphSpy) seq() iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range g.rows {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// GraphCount and MatchingFaces complete store.GraphQueryer. Neither is on the
// pushdown path (it uses GraphQuery only), so they are inert here — a
// non-zero return would misrepresent them as participating.
func (g *graphSpy) GraphCount(context.Context, store.GraphQuery) (matched, total int, err error) {
	return 0, 0, nil
}

func (g *graphSpy) MatchingFaces(
	context.Context, store.GraphQuery, []string,
) (map[string][]entity.Face, error) {
	return map[string][]entity.Face{}, nil
}

func seededSpy() *graphSpy {
	return &graphSpy{rows: []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "A", "secret": "S"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "B"}},
	}}
}

func drain(t *testing.T, seq iter.Seq2[*entity.Entity, error]) ([]*entity.Entity, error) {
	t.Helper()
	var out []*entity.Entity
	for e, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, nil
}

// TestListPushdown_QueryBranchUsesGraphQuery pins that a composed ACL scope
// actually reaches the store as a query. Without this, every other test here
// could pass while the pushdown silently fell back to load-then-Filter.
func TestListPushdown_QueryBranchUsesGraphQuery(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	p := stubProvider{res: acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket", Faces: store.InWorld(store.TrivialScope())}}}

	seq, ok := listPushdown(context.Background(), p, spy, red.redact, store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())})
	if !ok {
		t.Fatal("pushdown declined a composable query")
	}
	rows, err := drain(t, seq)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if spy.graphCalls != 1 {
		t.Errorf("GraphQuery calls = %d, want 1 — the ACL predicate never reached the store", spy.graphCalls)
	}
	if spy.listCalls != 0 {
		t.Errorf("ListEntities calls = %d, want 0 — pushdown must not also do a full scan", spy.listCalls)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}

// TestListPushdown_RedactsOnEveryBranch is the RR-1W1G6K regression test.
// The pushdown replaces the ROW gate; dropping field redaction with it would
// return every `visible:`-hidden property to scripts — the #1188 finding.
//
// AllowAll is called out separately (RR-OXE47R): "may read every row" is not
// "may see every field", and it is the branch where a return-rows-straight-
// through shortcut is most tempting.
func TestListPushdown_RedactsOnEveryBranch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		res  acl.ReadQueryResult
	}{
		{"AllowAll", acl.ReadQueryResult{AllowAll: true}},
		{"Query", acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket", Faces: store.InWorld(store.TrivialScope())}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spy := seededSpy()
			red := &countingRedactor{}
			seq, ok := listPushdown(
				context.Background(), stubProvider{res: tc.res}, spy, red.redact,
				store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())})
			if !ok {
				t.Fatal("pushdown declined")
			}
			rows, err := drain(t, seq)
			if err != nil {
				t.Fatalf("drain: %v", err)
			}
			if red.calls != len(rows) {
				t.Errorf("redactor ran %d times for %d rows — every row must be redacted",
					red.calls, len(rows))
			}
			for _, e := range rows {
				if _, leaked := e.Properties["secret"]; leaked {
					t.Errorf("row %s leaked a hidden property: %v", e.ID, e.Properties)
				}
			}
		})
	}
}

// TestListPushdown_DenyAllYieldsNothingWithoutTouchingTheStore pins that a
// denied scope short-circuits rather than reading and filtering.
func TestListPushdown_DenyAllYieldsNothingWithoutTouchingTheStore(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	seq, ok := listPushdown(
		context.Background(), stubProvider{res: acl.ReadQueryResult{DenyAll: true}},
		spy, red.redact, store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())})
	if !ok {
		t.Fatal("pushdown declined DenyAll")
	}
	rows, err := drain(t, seq)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("DenyAll yielded %d rows, want 0", len(rows))
	}
	if spy.graphCalls+spy.listCalls != 0 {
		t.Errorf("DenyAll touched the store (%d graph, %d list), want 0",
			spy.graphCalls, spy.listCalls)
	}
}

// TestListPushdown_ScopeErrorFailsClosed pins that a scope we cannot compose
// surfaces as an error rather than degrading to an ungated read. An empty
// list would be the other tempting choice and is wrong: it is
// indistinguishable from "you may see nothing" (the TKT-FVQ4 ambiguity).
func TestListPushdown_ScopeErrorFailsClosed(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	boom := errors.New("gate down")
	seq, ok := listPushdown(
		context.Background(), stubProvider{err: boom}, spy, red.redact,
		store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())})
	if !ok {
		t.Fatal("a scope error must be reported, not silently declined to the fallback")
	}
	rows, err := drain(t, seq)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the gate failure surfaced", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows leaked past a failed scope: %v", rows)
	}
	if spy.graphCalls+spy.listCalls != 0 {
		t.Errorf("store was read despite an uncomposable scope")
	}
}

// TestListPushdown_DeclinesWhenNotApplicable pins the fallback conditions.
// Declining is a PERFORMANCE regression only — the caller then runs
// load-then-Filter, which gates on the same policy.
func TestListPushdown_DeclinesWhenNotApplicable(t *testing.T) {
	t.Parallel()
	red := &countingRedactor{}
	q := store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())}
	allow := stubProvider{res: acl.ReadQueryResult{AllowAll: true}}

	if _, ok := listPushdown(context.Background(), nil, seededSpy(), red.redact, q); ok {
		t.Error("pushdown ran without a provider")
	}
	if _, ok := listPushdown(
		context.Background(), allow, seededSpy(), red.redact, store.EntityQuery{Faces: store.InWorld(store.TrivialScope())},
	); ok {
		t.Error("pushdown ran for a type-less query; the ACL scope is composed per type")
	}
	// Zero ReadQueryResult — neither allow, deny, nor query. Unrepresentable;
	// must fall back rather than guess.
	if _, ok := listPushdown(
		context.Background(), stubProvider{}, seededSpy(), red.redact, q,
	); ok {
		t.Error("pushdown accepted a zero ReadQueryResult instead of falling back")
	}
}

// An AllFaces list through the pushdown, for a principal with a scoped,
// face-restricted grant, returns exactly the face rows that satisfy the scope
// and sit inside the grant's faces. This is the access delta of TKT-KQXVF7:
// before face selections were required, the template ran in the default world
// and a faced entity with no default row was never listed.
func TestListPushdown_AllFacesScopedPrincipalGetsGrantedFaceRowsOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy"},
		{ID: "POL-2", Type: "policy", Face: "published"},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "POL-3", Type: "policy", Face: "published"},
		{ID: "alice", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, to := range []string{"POL-1", "POL-2"} {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "reviews", To: to}, nil); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:         map[string]acl.RoleDef{"reviewer": {Read: []string{"policy@published"}}},
		RoleRelations: map[string]acl.RoleRelationDef{"reviews": {Confers: "reviewer"}},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	req, err := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatal(err)
	}
	rqr := req.ReadQuery(ctx, "policy")
	if rqr.Query == nil {
		t.Fatalf("want a scoped verdict, got %+v", rqr)
	}
	identity := func(_ context.Context, e *entity.Entity) *entity.Entity { return e }
	seq, ok := listPushdown(ctx, stubProvider{res: rqr}, st, identity,
		store.EntityQuery{Type: "policy", Faces: store.AllFaces()})
	if !ok {
		t.Fatal("pushdown declined a composable query")
	}
	rows, err := drain(t, seq)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range rows {
		got = append(got, e.ID+"@"+e.Face.String())
	}
	if len(got) != 1 || got[0] != "POL-2@published" {
		t.Fatalf("rows = %v, want [POL-2@published]: POL-1 has no granted face, POL-2@draft is "+
			"outside the grant's faces, POL-3 is outside the scope", got)
	}
	// The ACL result is a template: it carries no selection, and the
	// pushdown stamps its own onto a copy.
	if !rqr.Query.Faces.IsZero() {
		t.Fatalf("template left with selection %s after pushdown", rqr.Query.Faces)
	}
}

// countPushdown counts exactly the rows listPushdown lists, for every kind of
// read grant (TKT-QZTROQ): global, face-restricted global, relation-conferred
// and none. The count runs in the store, not over the listed rows.
func TestCountPushdown_EqualsListPushdown(t *testing.T) {
	t.Parallel()
	assertCountMatchesList(t, memstore.New())
}

// assertCountMatchesList seeds st with faced policies and checks, per
// principal and world, that countPushdown counts the rows listPushdown
// yields. Each backend renders the count in its own SQL, so each runs it.
func assertCountMatchesList(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy"},
		{ID: "POL-2", Type: "policy", Face: "published"},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "POL-3", Type: "policy", Face: "published"},
		{ID: "alice", Type: "user"},
		{ID: "bob", Type: "user"},
		{ID: "carol", Type: "user"},
		{ID: "dave", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []entity.RelationKey{
		{From: "alice", Type: "reviews", To: "POL-1"},
		{From: "alice", Type: "reviews", To: "POL-2"},
	} {
		if _, err := st.CreateRelation(ctx, rel, nil); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reviewer":         {Read: []string{"policy@published"}},
			"readers":          {Read: []string{"policy"}},
			"publishedreaders": {Read: []string{"policy@published"}},
		},
		Assignments:   map[string]string{"bob": "readers", "carol": "publishedreaders"},
		RoleRelations: map[string]acl.RoleRelationDef{"reviews": {Confers: "reviewer"}},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	identity := func(_ context.Context, e *entity.Entity) *entity.Entity { return e }
	// draftFirst prefers draft over published and drops POL-1, which has
	// neither face. A face-restricted grant must rank among its own faces
	// only, in the count as in the list.
	draftFirst := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	}))

	for _, tc := range []struct {
		user  string
		faces store.FaceSelection
		world string
		want  int
	}{
		{"alice", store.AllFaces(), "all", 1},   // relation-conferred, published only: POL-2@published
		{"bob", store.AllFaces(), "all", 4},     // global read on every face
		{"carol", store.AllFaces(), "all", 2},   // global read on published: POL-2, POL-3
		{"dave", store.AllFaces(), "all", 0},    // no read
		{"alice", draftFirst, "draft-first", 1}, // POL-2 at published, its only readable face
		{"bob", draftFirst, "draft-first", 2},   // POL-2 at draft, POL-3
		{"carol", draftFirst, "draft-first", 2}, // POL-2 at published, POL-3
		{"dave", draftFirst, "draft-first", 0},
	} {
		t.Run(tc.user+"/"+tc.world, func(t *testing.T) {
			t.Parallel()
			req, err := d.ForPrincipal(principal.Principal{User: tc.user, Tool: principal.ToolDataEntry})
			if err != nil {
				t.Fatal(err)
			}
			p := stubProvider{res: req.ReadQuery(ctx, "policy")}
			q := store.EntityQuery{Type: "policy", Faces: tc.faces}

			seq, ok := listPushdown(ctx, p, st, identity, q)
			if !ok {
				t.Fatal("list pushdown declined a composable query")
			}
			rows, err := drain(t, seq)
			if err != nil {
				t.Fatal(err)
			}
			n, ok, err := countPushdown(ctx, p, st, q)
			if !ok || err != nil {
				t.Fatalf("countPushdown = (%d, %v, %v)", n, ok, err)
			}
			if n != len(rows) || n != tc.want {
				t.Errorf("count = %d, list = %d, want %d", n, len(rows), tc.want)
			}
		})
	}
}

// A DenyAll count is 0 without a store read, and a scope error is an error,
// never a raw count.
func TestCountPushdown_DenyAndScopeError(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	q := store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())}

	n, ok, err := countPushdown(context.Background(), stubProvider{res: acl.ReadQueryResult{DenyAll: true}}, spy, q)
	if !ok || err != nil || n != 0 {
		t.Errorf("DenyAll = (%d, %v, %v), want (0, true, nil)", n, ok, err)
	}
	if spy.graphCalls != 0 || spy.listCalls != 0 {
		t.Errorf("DenyAll touched the store: graph=%d list=%d", spy.graphCalls, spy.listCalls)
	}

	boom := errors.New("boom")
	_, ok, err = countPushdown(context.Background(), stubProvider{err: boom}, spy, q)
	if !ok || !errors.Is(err, boom) {
		t.Errorf("scope error = (ok=%v, err=%v), want (true, boom)", ok, err)
	}
}
