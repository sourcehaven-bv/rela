package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/visibility/visibilitytest"
)

// TestResolver_ResolveHeaders walks the gates of the batch read. Each case
// names the refs it asks and, per ref, the face served ("-" for none) and
// whether the family is readable. A ref absent from want is a full miss.
func TestResolver_ResolveHeaders(t *testing.T) {
	onlyPublished := map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}
	pol := entity.Ref{ID: "POL-1"}
	polDraft := entity.Ref{ID: "POL-1", Face: faceDraft}
	polPub := entity.Ref{ID: "POL-1", Face: facePublished}
	tkt := entity.Ref{ID: "TKT-1"}
	missing := entity.Ref{ID: "TKT-404"}
	all := []entity.Ref{pol, polDraft, polPub, tkt, missing}

	type hit struct {
		face   string // served face, or "-" for none
		family bool
	}
	for _, tc := range []struct {
		name  string
		gate  resolverGate
		world visibility.World
		want  map[entity.Ref]hit
	}{
		{
			name:  "default world: named faces served, bare faced id by family only",
			world: visibility.WorldOf(store.TrivialScope()),
			want: map[entity.Ref]hit{
				pol: {"-", true}, polDraft: {"draft", true}, polPub: {"published", true}, tkt: {"", true},
			},
		},
		{
			name:  "a world serves the bare id its first readable chain face",
			world: visibility.WorldOf(publishedWorld()),
			want: map[entity.Ref]hit{
				pol: {"published", true}, polDraft: {"draft", true}, polPub: {"published", true}, tkt: {"", true},
			},
		},
		{
			name:  "the ACL trims candidates before the world ranks them",
			gate:  resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(faceDraft)}},
			world: visibility.WorldOf(publishedWorld()),
			want: map[entity.Ref]hit{
				pol: {"draft", true}, polDraft: {"draft", true}, polPub: {"-", true}, tkt: {"", true},
			},
		},
		{
			name:  "a hidden face is not served",
			world: visibility.WorldOf(store.TrivialScope()),
			gate:  resolverGate{faces: onlyPublished},
			want: map[entity.Ref]hit{
				pol: {"-", true}, polDraft: {"-", true}, polPub: {"published", true}, tkt: {"", true},
			},
		},
		{
			name:  "a denied row is a full miss",
			world: visibility.WorldOf(store.TrivialScope()),
			gate:  resolverGate{deny: map[string]bool{"POL-1": true}},
			want:  map[entity.Ref]hit{tkt: {"", true}},
		},
		{
			name:  "a denied world serves nothing but still answers the family",
			world: visibility.DeniedWorld(),
			want: map[entity.Ref]hit{
				pol: {"-", true}, polDraft: {"-", true}, polPub: {"-", true}, tkt: {"-", true},
			},
		},
		{
			name:  "a gate error hides the type",
			world: visibility.WorldOf(store.TrivialScope()),
			gate:  resolverGate{faceErr: errors.New("down")},
			want:  map[entity.Ref]hit{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captureWarn(t)
			st := resolverStore(t)
			r := mustResolver(t, tc.gate, visibility.NopRedactor{}, st)
			got := r.ResolveHeaders(context.Background(), tc.world, all)
			for _, ref := range all {
				res, ok := got[ref]
				want, wantOK := tc.want[ref]
				if ok != wantOK {
					t.Errorf("%s: present = %v, want %v", ref, ok, wantOK)
					continue
				}
				if !ok {
					continue
				}
				face := "-"
				if res.Served() {
					face = res.Header.Face.String()
					if res.Header.ID != ref.ID {
						t.Errorf("%s: served id %q", ref, res.Header.ID)
					}
				}
				if face != want.face || res.Family != want.family {
					t.Errorf("%s: got (face %q, family %v), want (%q, %v)", ref, face, res.Family, want.face, want.family)
				}
			}
			if calls := st.Calls(); st.Reads() != 1 || calls["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
		})
	}
}

// TestResolver_ResolveHeadersTypeAndRedaction checks that a family stored
// under two types is a miss, and that a served header is redacted once for
// the whole batch without touching the stored row.
func TestResolver_ResolveHeadersTypeAndRedaction(t *testing.T) {
	ctx := context.Background()
	base := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "MIX-1", Type: "policy", Face: faceDraft},
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t", "salary": 2}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "u", "salary": 3}},
	} {
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	primer := &countingPrimer{hideRedactor: hideRedactor{names: []string{"salary"}}}
	// No backend stores one id under two types, so the loader adds the row.
	load := extraRowLoader{Loader: base, extra: &entity.Entity{ID: "MIX-1", Type: "ticket", Face: facePublished}}
	r := mustResolver(t, visibility.NopGate{}, primer, load)

	got := r.ResolveHeaders(ctx, visibility.WorldOf(store.TrivialScope()), []entity.Ref{
		{ID: "MIX-1", Face: faceDraft}, {ID: "TKT-1"}, {ID: "TKT-2"},
	})
	if _, ok := got[entity.Ref{ID: "MIX-1", Face: faceDraft}]; ok {
		t.Error("a family stored under two types was served")
	}
	for _, id := range []string{"TKT-1", "TKT-2"} {
		h := got[entity.Ref{ID: id}].Header
		if _, leaked := h.Properties["salary"]; leaked || !slices.Equal(h.Redacted, []string{"salary"}) {
			t.Errorf("%s: header = %+v, want salary redacted", id, h)
		}
	}
	if primer.primed != 1 {
		t.Errorf("primed %d times, want once per batch", primer.primed)
	}
	stored, _ := base.GetEntity(ctx, entity.Ref{ID: "TKT-1"})
	if _, kept := stored.Properties["salary"]; !kept {
		t.Error("redaction mutated the stored row")
	}
}

func TestResolver_ResolveHeadersEmptyInput(t *testing.T) {
	st := resolverStore(t)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
	if got := r.ResolveHeaders(context.Background(), visibility.WorldOf(store.TrivialScope()), []entity.Ref{{}}); len(got) != 0 {
		t.Errorf("got %v, want no hits", got)
	}
	if st.Reads() != 0 {
		t.Errorf("an empty batch read the store: %s", st)
	}
}

// TestResolver_ResolveHeadersUnsetWorldServesNothing pins that the zero World
// fails closed, as InWorld refuses it, rather than reading the trivial world.
func TestResolver_ResolveHeadersUnsetWorldServesNothing(t *testing.T) {
	buf := captureWarn(t)
	st := resolverStore(t)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
	if got := r.ResolveHeaders(context.Background(), visibility.World{}, []entity.Ref{{ID: "TKT-1"}}); len(got) != 0 {
		t.Errorf("got %v, want no hits", got)
	}
	if st.Reads() != 0 {
		t.Errorf("an unset world read the store: %s", st)
	}
	if !strings.Contains(buf.String(), "unset world") {
		t.Errorf("the wiring bug was not logged: %s", buf)
	}
}

func TestResolver_ResolveHeadersReadFailureIsAMiss(t *testing.T) {
	buf := captureWarn(t)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, failingLoader{err: errors.New("disk")})
	if got := r.ResolveHeaders(context.Background(), visibility.WorldOf(store.TrivialScope()), []entity.Ref{{ID: "TKT-1"}}); len(got) != 0 {
		t.Errorf("got %v, want no hits", got)
	}
	if !strings.Contains(buf.String(), "header read failed") {
		t.Errorf("the failure was not logged: %s", buf)
	}
}

// TestResolver_ResolveHeadersBudget pins RR-XD7YN9: the batch costs one
// header query and one gate round per type, at 10 refs and at 50.
func TestResolver_ResolveHeadersBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			var refs []entity.Ref
			for i := range n {
				id := fmt.Sprintf("TKT-%d", i)
				e := &entity.Entity{ID: id, Type: "ticket", Content: strings.Repeat("body ", 100)}
				if err := base.CreateEntity(ctx, e); err != nil {
					t.Fatal(err)
				}
				refs = append(refs, entity.Ref{ID: id})
			}
			st := storetest.NewCounting(base)
			gate := &countingGate{}
			r := mustResolver(t, gate, visibility.NopRedactor{}, st)
			if got := r.ResolveHeaders(ctx, visibility.WorldOf(store.TrivialScope()), refs); len(got) != n {
				t.Fatalf("got %d hits, want %d", len(got), n)
			}
			if calls := st.Calls(); st.Reads() != 1 || calls["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
			if gate.many != 1 || gate.single != 0 {
				t.Errorf("gate calls = %d many, %d single; want 1 and 0", gate.many, gate.single)
			}
		})
	}
}

// extraRowLoader yields one more row after every ListEntities query that
// names its id. It hides the store's header projection, so header reads go
// through ListEntities too.
type extraRowLoader struct {
	visibility.Loader
	extra *entity.Entity
}

func (l extraRowLoader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range l.Loader.ListEntities(ctx, q) {
			if !yield(e, err) {
				return
			}
		}
		if slices.Contains(q.IDs, l.extra.ID) {
			yield(l.extra, nil)
		}
	}
}

// countingGate allows everything and counts its row-gate calls.
type countingGate struct{ many, single int }

func (g *countingGate) PermitsRead(context.Context, string, string) (bool, error) {
	g.single++
	return true, nil
}

func (g *countingGate) permitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	g.many++
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// ReadableFacesMany implements the row gate over permitsReadMany.
func (g *countingGate) ReadableFacesMany(ctx context.Context, entityType string, ids []string) (acl.FaceVerdicts, error) {
	return visibilitytest.IDVerdicts(g.permitsReadMany(ctx, entityType, ids))
}

// typeErrGate is resolverGate whose row gate fails for one type.
type typeErrGate struct {
	resolverGate
	failType string
}

func (g typeErrGate) permitsReadMany(ctx context.Context, typ string, ids []string) (map[string]bool, error) {
	if typ == g.failType {
		return nil, errors.New("row gate down")
	}
	return g.resolverGate.permitsReadMany(ctx, typ, ids)
}

// ReadableFacesMany implements the row gate over permitsReadMany.
func (g typeErrGate) ReadableFacesMany(ctx context.Context, entityType string, ids []string) (acl.FaceVerdicts, error) {
	return visibilitytest.IDVerdicts(g.permitsReadMany(ctx, entityType, ids))
}

// TestResolver_ResolveHeadersRowGateErrorHidesOnlyItsType checks that a row
// gate failure for one type hides that type's refs and leaves the others.
func TestResolver_ResolveHeadersRowGateErrorHidesOnlyItsType(t *testing.T) {
	buf := captureWarn(t)
	st := resolverStore(t)
	r := mustResolver(t, typeErrGate{failType: "policy"}, visibility.NopRedactor{}, st)
	got := r.ResolveHeaders(context.Background(), visibility.WorldOf(store.TrivialScope()), []entity.Ref{
		{ID: "POL-1", Face: faceDraft}, {ID: "TKT-1"},
	})
	if _, ok := got[entity.Ref{ID: "POL-1", Face: faceDraft}]; ok {
		t.Error("a ref of the failing type was answered")
	}
	if !got[entity.Ref{ID: "TKT-1"}].Served() {
		t.Error("a ref of another type was hidden")
	}
	if !strings.Contains(buf.String(), "gate failed") {
		t.Errorf("the failure was not logged: %s", buf)
	}
}

// TestResolver_ReadableTypes checks that the typed batch returns a fault
// instead of folding it into a miss, and that it reports the stored type of a
// family whose only readable face is not the one a caller might ask for.
func TestResolver_ReadableTypes(t *testing.T) {
	t.Run("a gate error is returned", func(t *testing.T) {
		r := mustResolver(t, typeErrGate{failType: "policy"}, visibility.NopRedactor{}, resolverStore(t))
		got, err := r.ReadableTypes(context.Background(), []string{"POL-1", "TKT-1"})
		if err == nil || got != nil {
			t.Errorf("got %v, %v; want nil and the gate error", got, err)
		}
	})
	t.Run("a read error is returned", func(t *testing.T) {
		r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, failingLoader{err: errors.New("disk")})
		if _, err := r.ReadableTypes(context.Background(), []string{"TKT-1"}); err == nil {
			t.Error("a failed header read was not returned")
		}
	})
	t.Run("types of readable families only", func(t *testing.T) {
		gate := resolverGate{
			faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)},
			deny:  map[string]bool{"FEAT-1": true},
		}
		r := mustResolver(t, gate, visibility.NopRedactor{}, resolverStore(t))
		got, err := r.ReadableTypes(context.Background(), []string{"POL-1", "TKT-1", "FEAT-1", "TKT-404", ""})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"POL-1": "policy", "TKT-1": "ticket"}
		if !maps.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("the lenient batch still folds a gate error into a miss", func(t *testing.T) {
		captureWarn(t)
		r := mustResolver(t, typeErrGate{failType: "policy"}, visibility.NopRedactor{}, resolverStore(t))
		polDraft := entity.Ref{ID: "POL-1", Face: faceDraft}
		got := r.ResolveHeaders(context.Background(), visibility.WorldOf(store.TrivialScope()), []entity.Ref{polDraft, {ID: "TKT-1"}})
		if _, ok := got[polDraft]; ok || !got[entity.Ref{ID: "TKT-1"}].Served() {
			t.Errorf("got %v, want the policy ref hidden and the ticket served", got)
		}
	})
}

// TestResolver_ReadableTypesBudget pins the typed batch to one header query
// and one gate round per type, at 10 ids and at 50.
func TestResolver_ReadableTypesBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			var ids []string
			for i := range n {
				id := fmt.Sprintf("TKT-%d", i)
				if err := base.CreateEntity(ctx, &entity.Entity{ID: id, Type: "ticket"}); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			st := storetest.NewCounting(base)
			gate := &countingGate{}
			r := mustResolver(t, gate, visibility.NopRedactor{}, st)
			got, err := r.ReadableTypes(ctx, ids)
			if err != nil || len(got) != n {
				t.Fatalf("got %d types, %v; want %d", len(got), err, n)
			}
			if calls := st.Calls(); st.Reads() != 1 || calls["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
			if gate.many != 1 || gate.single != 0 {
				t.Errorf("gate calls = %d many, %d single; want 1 and 0", gate.many, gate.single)
			}
		})
	}
}

// TestResolver_ResolveHeadersRefusesMalformedRefs checks that a ref the
// address grammar refuses is a miss and does not disturb the others.
func TestResolver_ResolveHeadersRefusesMalformedRefs(t *testing.T) {
	st := resolverStore(t)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
	bad := entity.Ref{ID: "TKT-1@draft"}
	got := r.ResolveHeaders(context.Background(), visibility.WorldOf(store.TrivialScope()), []entity.Ref{bad, {ID: "TKT-1"}})
	if _, ok := got[bad]; ok {
		t.Error("a malformed ref was answered")
	}
	if !got[entity.Ref{ID: "TKT-1"}].Served() {
		t.Error("a well-formed ref was not served")
	}
}

// TestResolver_ResolveHeadersAgreesWithInWorld pins the batch's world ranking
// to the single read, which the store ranks, for a chain with each fallback.
func TestResolver_ResolveHeadersAgreesWithInWorld(t *testing.T) {
	ctx := context.Background()
	base := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: faceDraft},
		{ID: "POL-1", Type: "policy", Face: facePublished},
		{ID: "POL-2", Type: "policy", Face: faceDraft},
		{ID: "DOC-1", Type: "doc"},
		{ID: "DOC-1", Type: "doc", Face: faceDraft},
		{ID: "DOC-2", Type: "doc"},
		{ID: "DOC-2", Type: "doc", Face: facePublished},
		{ID: "TKT-1", Type: "ticket"},
	} {
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{facePublished}, Fallback: store.FallbackExclude},
		"doc":    {Chain: []entity.Face{facePublished}, Fallback: store.FallbackDefaultState},
	})
	w := visibility.WorldOf(scope)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, base)
	types := map[string]string{"POL-1": "policy", "POL-2": "policy", "DOC-1": "doc", "DOC-2": "doc", "TKT-1": "ticket"}
	var refs []entity.Ref
	for id := range types {
		refs = append(refs, entity.Ref{ID: id})
	}
	got := r.ResolveHeaders(ctx, w, refs)
	for id, typ := range types {
		single, ok, err := r.InWorld(ctx, w, typ, id)
		if err != nil {
			t.Fatal(err)
		}
		batch := got[entity.Ref{ID: id}]
		if batch.Served() != ok {
			t.Errorf("%s: batch served %v, single %v", id, batch.Served(), ok)
			continue
		}
		if ok && batch.Header.Face != single.Entity.Face {
			t.Errorf("%s: batch face %q, single %q", id, batch.Header.Face, single.Entity.Face)
		}
	}
}

// TestScriptReader_ResolveHeadersBudget pins the production path: the bound
// script reader answers a batch over two types with one header read and one
// row-gate round per type, at 10 refs and at 50.
func TestScriptReader_ResolveHeadersBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			var refs []entity.Ref
			for i := range n {
				for _, typ := range []string{"ticket", "feature"} {
					id := fmt.Sprintf("%s-%d", strings.ToUpper(typ[:3]), i)
					if err := base.CreateEntity(ctx, &entity.Entity{ID: id, Type: typ}); err != nil {
						t.Fatal(err)
					}
					refs = append(refs, entity.Ref{ID: id})
				}
			}
			st := storetest.NewCounting(base)
			gate := &countingGate{}
			pr, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			sr, err := visibility.NewScriptReader(pr, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := sr.ResolveHeaders(ctx, refs); len(got) != 2*n {
				t.Fatalf("got %d hits, want %d", len(got), 2*n)
			}
			if calls := st.Calls(); st.Reads() != 1 || calls["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
			if gate.many != 2 || gate.single != 0 {
				t.Errorf("gate calls = %d many, %d single; want 2 and 0", gate.many, gate.single)
			}
		})
	}
}
