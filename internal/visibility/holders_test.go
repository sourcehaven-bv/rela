package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// holderID is the identity the holder tests look for.
const holderID = "USR-7"

// hideOwnerOf is a FieldRedactor that hides "owner" on the listed ids.
type hideOwnerOf map[string]bool

func (h hideOwnerOf) HiddenProperties(_ context.Context, e *entity.Entity) map[string]struct{} {
	if e != nil && h[e.ID] {
		return map[string]struct{}{"owner": {}}
	}
	return nil
}

// holderFixture is a seeded store with its policy.
type holderFixture struct {
	st store.Store
	d  *acl.Declarative
}

// seedHolders seeds faced policies whose owner names holderID in several
// shapes, and a policy with global, face-restricted and relation-conferred
// readers.
func seedHolders(t *testing.T, st store.Store) holderFixture {
	t.Helper()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Properties: map[string]any{"owner": holderID}},
		// The published face names someone else, the draft face names the
		// holder: a world that serves published first does not match.
		{ID: "POL-2", Type: "policy", Face: "published", Properties: map[string]any{"owner": "USR-9"}},
		{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"owner": holderID}},
		{ID: "POL-3", Type: "policy", Face: "published", Properties: map[string]any{"owner": []any{"USR-1", holderID}}},
		// The owner is hidden from every gated caller.
		{ID: "POL-4", Type: "policy", Face: "published", Properties: map[string]any{"owner": holderID}},
		{ID: "POL-5", Type: "policy", Face: "published", Properties: map[string]any{"owner": ""}},
		{ID: "POL-6", Type: "policy", Face: "published", Properties: map[string]any{"owner": "USR-70"}},
		{ID: "POL-7", Type: "policy", Face: "published"},
		{ID: "alice", Type: "user"},
		{ID: "bob", Type: "user"},
		{ID: "carol", Type: "user"},
		{ID: "dave", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, to := range []string{"POL-1", "POL-2", "POL-3"} {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "reviews", To: to}, nil); err != nil {
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
	}, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	return holderFixture{st: st, d: d}
}

// scriptReaderFor builds the production script reader over f; withGate
// false drops the binder, so the reader has no pushdown.
func (f holderFixture) scriptReaderFor(t *testing.T, redact FieldRedactor, withGate bool) *ScriptReader {
	t.Helper()
	gate, err := NewDeclarativeGate(f.d, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := NewPolicyReader(gate, redact, f.st)
	if err != nil {
		t.Fatal(err)
	}
	var binder Binder
	if withGate {
		binder = gate
	}
	sr, err := NewScriptReader(reader, f.st, binder)
	if err != nil {
		t.Fatal(err)
	}
	return sr
}

func refStrings(refs []entity.Ref) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = entity.FormatStateRef(r.ID, r.Face)
	}
	return out
}

// listOracle is the answer a holder query must give: a gated list read of
// the type, filtered in Go, in (id, face) order.
func listOracle(ctx context.Context, t *testing.T, sr *ScriptReader, faces store.FaceSelection) []string {
	t.Helper()
	var refs []entity.Ref
	for e, err := range sr.ListEntities(ctx, store.EntityQuery{Type: "policy", Faces: faces}) {
		if err != nil {
			t.Fatal(err)
		}
		if namesIdentity(e.Properties["owner"], holderID) {
			refs = append(refs, e.Ref())
		}
	}
	slices.SortFunc(refs, store.CompareRefs)
	return refStrings(refs)
}

func publishedFirst() store.FaceSelection {
	return store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	}))
}

// TestHolderLookup_GatedLikeList pins AC 5 on memstore (the sqlite twin
// runs under the sqlite tag): the gated lookup, with and without pushdown,
// equals a gated list read filtered in Go, for every kind of grant and face
// selection; the unrestricted lookup returns the stored matches; the
// refusing lookup refuses.
func TestHolderLookup_GatedLikeList(t *testing.T) {
	assertHoldersLikeList(t, memstore.New())
}

func assertHoldersLikeList(t *testing.T, st store.Store) {
	t.Helper()
	f := seedHolders(t, st)
	redact := hideOwnerOf{"POL-4": true}
	pushed := f.scriptReaderFor(t, redact, true)
	fallback := f.scriptReaderFor(t, redact, false)
	draftFirst := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	}))
	for _, tc := range []struct {
		user  string
		world string
		faces store.FaceSelection
		want  []string
	}{
		{"alice", "all", store.AllFaces(), []string{"POL-3@published"}},
		{"bob", "all", store.AllFaces(), []string{"POL-1", "POL-2@draft", "POL-3@published"}},
		{"carol", "all", store.AllFaces(), []string{"POL-3@published"}},
		{"dave", "all", store.AllFaces(), []string{}},
		{"bob", "published-first", publishedFirst(), []string{"POL-3@published"}},
		{"bob", "draft-first", draftFirst, []string{"POL-2@draft", "POL-3@published"}},
		{"carol", "draft-first", draftFirst, []string{"POL-3@published"}},
		{"alice", "trivial", store.InWorld(store.TrivialScope()), []string{}},
		{"bob", "trivial", store.InWorld(store.TrivialScope()), []string{"POL-1"}},
	} {
		t.Run(tc.user+"/"+tc.world, func(t *testing.T) {
			ctx := principal.With(context.Background(), principal.Principal{User: tc.user, Tool: principal.ToolDataEntry})
			ctx = acltest.Bind(ctx, t, f.d)
			oracle := listOracle(ctx, t, pushed, tc.faces)
			if !slices.Equal(oracle, tc.want) {
				t.Fatalf("list oracle = %v, want %v", oracle, tc.want)
			}
			for name, sr := range map[string]*ScriptReader{"pushdown": pushed, "fallback": fallback} {
				h, err := NewHolderLookup(sr)
				if err != nil {
					t.Fatal(err)
				}
				refs, more, err := h.Matching(ctx, HolderQuery{
					EntityType: "policy", Property: "owner", Identity: holderID, Faces: tc.faces, Limit: 10,
				})
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if got := refStrings(refs); !slices.Equal(got, tc.want) || more {
					t.Errorf("%s: got %v (more %v), want %v", name, got, more, tc.want)
				}
			}
		})
	}

	t.Run("limit reports more", func(t *testing.T) {
		ctx := acltest.Bind(principal.With(context.Background(),
			principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, f.d)
		h, err := NewHolderLookup(pushed)
		if err != nil {
			t.Fatal(err)
		}
		refs, more, err := h.Matching(ctx, HolderQuery{
			EntityType: "policy", Property: "owner", Identity: holderID, Faces: store.AllFaces(), Limit: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := refStrings(refs); !slices.Equal(got, []string{"POL-1", "POL-2@draft"}) || !more {
			t.Errorf("got %v (more %v), want the first two and more", got, more)
		}
	})

	t.Run("empty identity matches nothing", func(t *testing.T) {
		ctx := acltest.Bind(principal.With(context.Background(),
			principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, f.d)
		h, err := NewHolderLookup(pushed)
		if err != nil {
			t.Fatal(err)
		}
		refs, more, err := h.Matching(ctx, HolderQuery{
			EntityType: "policy", Property: "owner", Faces: store.AllFaces(), Limit: 10,
		})
		if err != nil || len(refs) != 0 || more {
			t.Errorf("got %v (more %v, err %v), want none", refStrings(refs), more, err)
		}
	})

	t.Run("unrestricted", func(t *testing.T) {
		h, err := NewUnrestrictedHolderLookup(st)
		if err != nil {
			t.Fatal(err)
		}
		for faces, want := range map[string]struct {
			sel  store.FaceSelection
			refs []string
		}{
			"all":             {store.AllFaces(), []string{"POL-1", "POL-2@draft", "POL-3@published", "POL-4@published"}},
			"published-first": {publishedFirst(), []string{"POL-3@published", "POL-4@published"}},
		} {
			refs, _, err := h.Matching(context.Background(), HolderQuery{
				EntityType: "policy", Property: "owner", Identity: holderID, Faces: want.sel, Limit: 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := refStrings(refs); !slices.Equal(got, want.refs) {
				t.Errorf("%s: got %v, want %v", faces, got, want.refs)
			}
		}
	})

	t.Run("refusing", func(t *testing.T) {
		_, _, err := NewRefusingHolderLookup().Matching(context.Background(), HolderQuery{
			EntityType: "policy", Property: "owner", Identity: holderID, Faces: store.AllFaces(), Limit: 10,
		})
		if !errors.Is(err, ErrReaderUnavailable) {
			t.Errorf("err = %v, want ErrReaderUnavailable", err)
		}
	})
}

// TestHolderLookup_HiddenPropertyNotCounted pins that the limit counts rows
// after redaction: three rows whose hidden owner names the holder do not
// use up a limit of 2, and two visible matches give more = false.
func TestHolderLookup_HiddenPropertyNotCounted(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, id := range []string{"H-1", "H-2", "H-3", "V-1", "V-2"} {
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: id, Type: "policy", Properties: map[string]any{"owner": holderID},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "bob", Type: "user"}); err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:       map[string]acl.RoleDef{"readers": {Read: []string{"policy"}}},
		Assignments: map[string]string{"bob": "readers"},
	}, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	f := holderFixture{st: st, d: d}
	sr := f.scriptReaderFor(t, hideOwnerOf{"H-1": true, "H-2": true, "H-3": true}, true)
	h, err := NewHolderLookup(sr)
	if err != nil {
		t.Fatal(err)
	}
	as := acltest.Bind(principal.With(ctx, principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, d)
	refs, more, err := h.Matching(as, HolderQuery{
		EntityType: "policy", Property: "owner", Identity: holderID, Faces: store.AllFaces(), Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := refStrings(refs); !slices.Equal(got, []string{"V-1", "V-2"}) || more {
		t.Errorf("got %v (more %v), want [V-1 V-2] and no more", got, more)
	}
}

// stubBinder is a Binder that also composes a fixed read scope, so a
// ScriptReader built on it pushes down with that scope.
type stubBinder struct{ stubProvider }

func (stubBinder) Bind(ctx context.Context) (context.Context, error) { return ctx, nil }

// holderSpy is a store whose GraphQuery records each query and returns no
// rows, and whose policy reader allows every row.
func spyLookup(t *testing.T, res acl.ReadQueryResult) (*HolderLookup, *worldCapturingSpy) {
	t.Helper()
	spy := &worldCapturingSpy{Store: memstore.New()}
	reader, err := NewPolicyReader(NopGate{}, NopRedactor{}, spy)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := NewScriptReader(reader, spy, stubBinder{stubProvider{res: res}})
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHolderLookup(sr)
	if err != nil {
		t.Fatal(err)
	}
	return h, spy
}

func holderQuery() HolderQuery {
	return HolderQuery{EntityType: "policy", Property: "owner", Identity: holderID, Faces: store.AllFaces(), Limit: 5}
}

// TestHolderLookup_PropsOnAllowAll pins RR-GQWRLD for the holder query:
// the property test reaches the store on the AllowAll branch with the face
// allowlist, and on the query branch it is added to a copy of the ACL
// query, never to the template itself.
func TestHolderLookup_PropsOnAllowAll(t *testing.T) {
	want := holderProp(holderQuery())
	t.Run("allow all", func(t *testing.T) {
		h, spy := spyLookup(t, acl.ReadQueryResult{AllowAll: true, Faces: []entity.Face{"published"}})
		if _, _, err := h.Matching(context.Background(), holderQuery()); err != nil {
			t.Fatal(err)
		}
		if len(spy.graphQueries) != 1 {
			t.Fatalf("graph queries = %d, want 1", len(spy.graphQueries))
		}
		q := spy.graphQueries[0]
		if !slices.Contains(q.Props, want) || !slices.Equal(q.FaceIn, []entity.Face{"published"}) {
			t.Errorf("query = %+v, want the holder predicate and the face allowlist", q)
		}
		if q.Limit != 6 || q.Offset != 0 {
			t.Errorf("page = limit %d offset %d, want limit 6 offset 0", q.Limit, q.Offset)
		}
	})
	t.Run("query", func(t *testing.T) {
		tmpl := &store.GraphQuery{EntityType: "policy", Props: []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "live"},
		}}
		h, spy := spyLookup(t, acl.ReadQueryResult{Query: tmpl})
		if _, _, err := h.Matching(context.Background(), holderQuery()); err != nil {
			t.Fatal(err)
		}
		q := spy.graphQueries[0]
		if len(q.Props) != 2 || q.Props[1] != want {
			t.Errorf("props = %+v, want the ACL predicate and the holder predicate", q.Props)
		}
		if len(tmpl.Props) != 1 || !tmpl.Faces.IsZero() {
			t.Errorf("the ACL template was changed: %+v", tmpl)
		}
	})
	t.Run("deny", func(t *testing.T) {
		h, spy := spyLookup(t, acl.ReadQueryResult{DenyAll: true})
		refs, more, err := h.Matching(context.Background(), holderQuery())
		if err != nil || len(refs) != 0 || more || len(spy.graphQueries) != 0 {
			t.Errorf("deny: refs %v more %v err %v queries %d; want none and no read", refs, more, err,
				len(spy.graphQueries))
		}
	})
}

// TestHolderLookup_ZeroResultErrors pins that a read scope with no verdict
// and no query is an error, never a fallback or an ungated read; so is a
// scope that cannot be composed, and a malformed query.
func TestHolderLookup_ZeroResultErrors(t *testing.T) {
	h, spy := spyLookup(t, acl.ReadQueryResult{})
	if _, _, err := h.Matching(context.Background(), holderQuery()); !errors.Is(err, errZeroReadQuery) {
		t.Errorf("zero result: err = %v, want errZeroReadQuery", err)
	}
	if len(spy.graphQueries)+len(spy.entityQueries) != 0 {
		t.Errorf("a zero result read the store")
	}
	boom := errors.New("boom")
	h, _ = spyLookup(t, acl.ReadQueryResult{})
	h.sr.provider = stubProvider{err: boom}
	if _, _, err := h.Matching(context.Background(), holderQuery()); !errors.Is(err, boom) {
		t.Errorf("scope error: err = %v, want it passed on", err)
	}
	for name, q := range map[string]HolderQuery{
		"no type":     {Property: "owner", Identity: holderID, Faces: store.AllFaces(), Limit: 1},
		"no property": {EntityType: "policy", Identity: holderID, Faces: store.AllFaces(), Limit: 1},
		"no limit":    {EntityType: "policy", Property: "owner", Identity: holderID, Faces: store.AllFaces()},
		"no faces":    {EntityType: "policy", Property: "owner", Identity: holderID, Limit: 1},
	} {
		if _, _, err := h.Matching(context.Background(), q); !errors.Is(err, store.ErrInvalidQuery) {
			t.Errorf("%s: err = %v, want ErrInvalidQuery", name, err)
		}
	}
	if _, err := NewHolderLookup(nil); err == nil {
		t.Error("NewHolderLookup(nil) succeeded")
	}
	if _, err := NewUnrestrictedHolderLookup(nil); err == nil {
		t.Error("NewUnrestrictedHolderLookup(nil) succeeded")
	}
}

// TestHolderLookup_ReadsNoBodies pins that the lookup reads headers only:
// it needs the id, the face and one property, so no body crosses the store
// seam on the gated or the unrestricted path.
func TestHolderLookup_ReadsNoBodies(t *testing.T) {
	ctx := context.Background()
	raw := memstore.New()
	f := seedHolders(t, raw)
	for _, id := range []string{"BODY-1", "BODY-2"} {
		e := &entity.Entity{
			ID: id, Type: "policy", Content: "a long body", Properties: map[string]any{"owner": holderID},
		}
		if err := raw.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	watch := storetest.NewBodyWatch(raw)
	f.st = watch
	gated, err := NewHolderLookup(f.scriptReaderFor(t, hideOwnerOf{}, true))
	if err != nil {
		t.Fatal(err)
	}
	open, err := NewUnrestrictedHolderLookup(watch)
	if err != nil {
		t.Fatal(err)
	}
	q := holderQuery()
	q.Limit = 10
	bob := acltest.Bind(principal.With(ctx, principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, f.d)
	for name, h := range map[string]*HolderLookup{"gated": gated, "unrestricted": open} {
		watch.Reset()
		refs, _, err := h.Matching(bob, q)
		if err != nil {
			t.Fatal(err)
		}
		if got := refStrings(refs); !slices.Contains(got, "BODY-1") {
			t.Errorf("%s: refs %v, want BODY-1 among them", name, got)
		}
		if n := watch.Bodies(); n != 0 {
			t.Errorf("%s: %d bodies read, want none (%s)", name, n, watch)
		}
	}
}
