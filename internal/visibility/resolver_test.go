package visibility_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
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

const (
	faceDraft     = entity.Face("draft")
	facePublished = entity.Face("published")
)

// resolverGate is a RowGate and FaceSetGate whose verdicts a test sets.
type resolverGate struct {
	deny    map[string]bool // ids the row gate refuses
	err     error           // returned by the row gate
	faces   map[string]visibility.FaceSet
	faceErr error // returned by ReadableFaces
	rowHits *int
}

func (g resolverGate) PermitsRead(ctx context.Context, typ, id string) (bool, error) {
	m, err := g.permitsReadMany(ctx, typ, []string{id})
	return m[id], err
}

func (g resolverGate) permitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	if g.rowHits != nil {
		*g.rowHits++
	}
	if g.err != nil {
		return nil, g.err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = !g.deny[id]
	}
	return out, nil
}

// ReadableFacesMany implements the row gate over permitsReadMany.
func (g resolverGate) ReadableFacesMany(ctx context.Context, entityType string, ids []string) (acl.FaceVerdicts, error) {
	return visibilitytest.IDVerdicts(g.permitsReadMany(ctx, entityType, ids))
}

func (g resolverGate) ReadableFaces(_ context.Context, entityType string) (visibility.FaceSet, error) {
	if g.faceErr != nil {
		return visibility.NoFaces(), g.faceErr
	}
	if s, ok := g.faces[entityType]; ok {
		return s, nil
	}
	return visibility.AllFaces(), nil
}

// resolverStore seeds a faced policy (draft and published), a faceless
// ticket, and a faceless feature.
func resolverStore(t *testing.T) *storetest.Counting {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: faceDraft, Properties: map[string]any{"title": "draft", "salary": 1}},
		{ID: "POL-1", Type: "policy", Face: facePublished, Properties: map[string]any{"title": "published"}},
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "ticket", "salary": 2}},
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "feature"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	return storetest.NewCounting(st)
}

func publishedWorld() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{facePublished, faceDraft}, Fallback: store.FallbackExclude},
	})
}

func mustResolver(t *testing.T, gate visibility.RowGate, red visibility.FieldRedactor, load visibility.Loader) *visibility.Resolver {
	t.Helper()
	r, err := visibility.NewResolver(gate, red, load)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	return r
}

// captureWarn swaps the default logger for the duration of the test and
// returns the buffer it writes to.
func captureWarn(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestNewResolver_RejectsNil(t *testing.T) {
	st := memstore.New()
	for name, build := range map[string]func() (*visibility.Resolver, error){
		"gate":     func() (*visibility.Resolver, error) { return visibility.NewResolver(nil, visibility.NopRedactor{}, st) },
		"redactor": func() (*visibility.Resolver, error) { return visibility.NewResolver(visibility.NopGate{}, nil, st) },
		"loader": func() (*visibility.Resolver, error) {
			return visibility.NewResolver(visibility.NopGate{}, visibility.NopRedactor{}, nil)
		},
		"allow-all loader": func() (*visibility.Resolver, error) {
			return visibility.NewAllowAllResolver(nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if r, err := build(); err == nil || r != nil {
				t.Fatalf("got (%v, %v), want an error", r, err)
			}
		})
	}
}

// TestResolver_GateOrder walks every gate of every mode. Each case states the
// outcome and how many store reads it may cost, so a gate that moves after
// the load fails on the count even when the verdict is unchanged.
func TestResolver_GateOrder(t *testing.T) {
	onlyPublished := map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}
	none := map[string]visibility.FaceSet{"policy": visibility.NoFaces(), "ticket": visibility.NoFaces()}
	gateDown := errors.New("gate down")

	type call func(r *visibility.Resolver) (visibility.Resolved, bool, error)
	addr := func(w visibility.World, typ, a string) call {
		return func(r *visibility.Resolver) (visibility.Resolved, bool, error) {
			return r.Address(context.Background(), w, typ, a)
		}
	}
	def := visibility.WorldOf(store.TrivialScope())
	pub := visibility.WorldOf(publishedWorld())

	for _, tc := range []struct {
		name      string
		gate      resolverGate
		call      call
		wantRef   entity.Ref // zero: a miss
		wantErr   bool
		wantReads int
		wantRows  int // row-gate calls
	}{
		{name: "denied world, bare id", call: addr(visibility.DeniedWorld(), "ticket", "TKT-1")},
		{name: "denied world, named face", call: addr(visibility.DeniedWorld(), "policy", "POL-1@published")},
		{name: "unparseable address", call: addr(def, "ticket", "TKT-1@"), wantRows: 0},
		{name: "empty id", call: addr(def, "ticket", ""), wantRows: 0},
		{name: "row gate denies", gate: resolverGate{deny: map[string]bool{"TKT-1": true}},
			call: addr(def, "ticket", "TKT-1"), wantRows: 1},
		{name: "row gate errors", gate: resolverGate{err: gateDown},
			call: addr(def, "ticket", "TKT-1"), wantErr: true, wantRows: 1},
		{name: "readable faces error", gate: resolverGate{faceErr: gateDown},
			call: addr(def, "ticket", "TKT-1"), wantErr: true, wantRows: 1},
		{name: "no readable face, default world", gate: resolverGate{faces: none},
			call: addr(def, "ticket", "TKT-1"), wantRows: 1},
		{name: "no readable face, named face", gate: resolverGate{faces: none},
			call: addr(def, "policy", "POL-1@published"), wantRows: 1},
		{name: "no readable face, world", gate: resolverGate{faces: none},
			call: addr(pub, "policy", "POL-1"), wantRows: 1},
		{name: "named face denied", gate: resolverGate{faces: onlyPublished},
			call: addr(def, "policy", "POL-1@draft"), wantRows: 1},
		{name: "named face granted", gate: resolverGate{faces: onlyPublished},
			call: addr(def, "policy", "POL-1@published"), wantRef: entity.Ref{ID: "POL-1", Face: facePublished},
			wantReads: 1, wantRows: 1},
		{name: "faceless type, default world", call: addr(def, "ticket", "TKT-1"),
			wantRef: entity.Ref{ID: "TKT-1"}, wantReads: 1, wantRows: 1},
		{name: "faced type, bare id, default world", call: addr(def, "policy", "POL-1"),
			wantReads: 1, wantRows: 1},
		{name: "faced type, bare id, default world, implicit face not granted",
			gate: resolverGate{faces: onlyPublished}, call: addr(def, "policy", "POL-1"), wantRows: 1},
		{name: "missing id", call: addr(def, "ticket", "TKT-404"), wantReads: 1, wantRows: 1},
		{name: "stored type differs", call: addr(def, "ticket", "FEAT-1"), wantReads: 1, wantRows: 1},
		{name: "world ranks the chain", call: addr(pub, "policy", "POL-1"),
			wantRef: entity.Ref{ID: "POL-1", Face: facePublished}, wantReads: 1, wantRows: 1},
		{name: "world ranks within the readable faces",
			gate: resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(faceDraft)}},
			call: addr(pub, "policy", "POL-1"), wantRef: entity.Ref{ID: "POL-1", Face: faceDraft},
			wantReads: 1, wantRows: 1},
		{name: "world reads an unscoped type at its implicit face",
			call: addr(pub, "feature", "FEAT-1"), wantRef: entity.Ref{ID: "FEAT-1"}, wantReads: 1, wantRows: 1},
		{name: "named face read literally under a world", call: addr(pub, "policy", "POL-1@draft"),
			wantRef: entity.Ref{ID: "POL-1", Face: faceDraft}, wantReads: 1, wantRows: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resolverStore(t)
			rows := 0
			tc.gate.rowHits = &rows
			r := mustResolver(t, tc.gate, visibility.NopRedactor{}, st)

			res, ok, err := tc.call(r)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantRef.IsZero() {
				if ok || res.Entity != nil {
					t.Fatalf("got a hit %v, want a miss", res.Entity.Ref())
				}
			} else {
				if !ok || res.Entity == nil {
					t.Fatalf("got a miss (err %v), want %v", err, tc.wantRef)
				}
				if got := res.Entity.Ref(); got != tc.wantRef {
					t.Fatalf("served %v, want %v", got, tc.wantRef)
				}
			}
			if got := st.Reads(); got != tc.wantReads {
				t.Errorf("store reads = %d (%s), want %d", got, st, tc.wantReads)
			}
			if rows != tc.wantRows {
				t.Errorf("row-gate calls = %d, want %d", rows, tc.wantRows)
			}
		})
	}
}

// TestResolver_InWorldRefusesAnUnsetWorld pins design A4 at the resolver: the
// zero World is refused before any read, while a denied world still misses
// quietly.
func TestResolver_InWorldRefusesAnUnsetWorld(t *testing.T) {
	st := resolverStore(t)
	r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
	if _, _, err := r.InWorld(context.Background(), visibility.World{}, "ticket", "TKT-1"); !errors.Is(err, store.ErrInvalidQuery) {
		t.Errorf("InWorld(unset) err = %v, want ErrInvalidQuery", err)
	}
	if _, ok, err := r.InWorld(context.Background(), visibility.DeniedWorld(), "ticket", "TKT-1"); ok || err != nil {
		t.Errorf("InWorld(denied) = (%v, %v), want a quiet miss", ok, err)
	}
	if st.Reads() != 0 {
		t.Errorf("neither call may read the store: %s", st)
	}
}

// TestResolver_WorldQueryCarriesTheFaceSet pins the RR-Z23T2T shape: FaceIn is
// nil only for "every face", and the list of readable faces otherwise.
func TestResolver_WorldQueryCarriesTheFaceSet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		faces visibility.FaceSet
		want  []entity.Face
	}{
		{"every face", visibility.AllFaces(), nil},
		{"some faces", visibility.SomeFaces(faceDraft), []entity.Face{faceDraft}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingLoader{Loader: resolverStore(t)}
			r := mustResolver(t, resolverGate{faces: map[string]visibility.FaceSet{"policy": tc.faces}},
				visibility.NopRedactor{}, rec)
			if _, _, err := r.InWorld(context.Background(), visibility.WorldOf(publishedWorld()), "policy", "POL-1"); err != nil {
				t.Fatal(err)
			}
			if len(rec.queries) != 1 {
				t.Fatalf("queries = %d, want 1", len(rec.queries))
			}
			q := rec.queries[0]
			if (q.FaceIn == nil) != (tc.want == nil) || !slices.Equal(q.FaceIn, tc.want) {
				t.Errorf("FaceIn = %#v, want %#v", q.FaceIn, tc.want)
			}
			if w, ok := q.Faces.World(); !ok || w.IsTrivial() || !slices.Equal(q.IDs, []string{"POL-1"}) {
				t.Errorf("query = %+v, want the world and the one id", q)
			}
		})
	}
}

// TestResolver_LoadFailureIsAMissAndOneWarning pins ruling 2 (RR-FE1EGP): a
// store fault is the same miss a missing row is, plus one warning, in every
// mode. A genuine miss logs nothing.
func TestResolver_LoadFailureIsAMissAndOneWarning(t *testing.T) {
	boom := errors.New("backend down")
	for _, tc := range []struct {
		name string
		run  func(r *visibility.Resolver) bool
	}{
		{"ref", func(r *visibility.Resolver) bool {
			_, ok, err := r.Address(context.Background(), visibility.WorldOf(store.TrivialScope()), "policy", "POL-1@draft")
			return ok || err != nil
		}},
		{"default world", func(r *visibility.Resolver) bool {
			_, ok, err := r.Address(context.Background(), visibility.WorldOf(store.TrivialScope()), "ticket", "TKT-1")
			return ok || err != nil
		}},
		{"world", func(r *visibility.Resolver) bool {
			_, ok, err := r.Address(context.Background(), visibility.WorldOf(publishedWorld()), "policy", "POL-1")
			return ok || err != nil
		}},
		{"family", func(r *visibility.Resolver) bool {
			_, ok, err := r.Family(context.Background(), "policy", "POL-1")
			return ok || err != nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureWarn(t)
			r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, failingLoader{err: boom})
			if tc.run(r) {
				t.Fatal("a load failure must be a clean miss")
			}
			if n := strings.Count(buf.String(), "resolver load failed"); n != 1 {
				t.Errorf("warnings = %d, want 1:\n%s", n, buf)
			}
		})
	}
	t.Run("not found logs nothing", func(t *testing.T) {
		buf := captureWarn(t)
		r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, resolverStore(t))
		if _, ok, _ := r.Address(context.Background(), visibility.WorldOf(store.TrivialScope()), "policy", "POL-9@draft"); ok {
			t.Fatal("a missing row read")
		}
		if buf.Len() != 0 {
			t.Errorf("a genuine miss logged: %s", buf)
		}
	})
}

// TestResolver_RedactsOnceAndReportsTheRule checks the served row is
// redacted and carries the world rule for its face.
func TestResolver_RedactsOnceAndReportsTheRule(t *testing.T) {
	st := resolverStore(t)
	primer := &countingPrimer{hideRedactor: hideRedactor{names: []string{"salary"}}}
	r := mustResolver(t, visibility.NopGate{}, primer, st)

	res, ok, err := r.Address(context.Background(), visibility.WorldOf(publishedWorld()), "policy", "POL-1@draft")
	if err != nil || !ok {
		t.Fatalf("read = (%v, %v)", ok, err)
	}
	if _, leaked := res.Entity.Properties["salary"]; leaked {
		t.Error("a hidden property was served")
	}
	if !slices.Equal(res.Entity.Redacted, []string{"salary"}) {
		t.Errorf("Redacted = %v, want [salary]", res.Entity.Redacted)
	}
	if primer.primed != 1 {
		t.Errorf("primed %d times, want 1", primer.primed)
	}
	if res.Via != store.ResolutionChain || res.ChainPosition != 1 {
		t.Errorf("provenance = (%v, %d), want (chain, 1)", res.Via, res.ChainPosition)
	}
	stored, _ := st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: faceDraft})
	if _, kept := stored.Properties["salary"]; !kept {
		t.Error("redaction mutated the stored row")
	}

	res, ok, _ = r.Address(context.Background(), visibility.WorldOf(store.TrivialScope()), "ticket", "TKT-1")
	if !ok || res.Via != store.ResolutionUnscoped || res.ChainPosition != 0 {
		t.Errorf("default world provenance = (%v, %d, ok=%v), want (unscoped, 0)", res.Via, res.ChainPosition, ok)
	}
}

func TestResolver_Family(t *testing.T) {
	onlyPublished := map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}
	for _, tc := range []struct {
		name      string
		gate      resolverGate
		typ, id   string
		want      []entity.Face // nil: a miss
		wantErr   bool
		wantReads int
	}{
		{name: "every face, sorted", typ: "policy", id: "POL-1",
			want: []entity.Face{faceDraft, facePublished}, wantReads: 1},
		{name: "a hidden face is left out", gate: resolverGate{faces: onlyPublished}, typ: "policy", id: "POL-1",
			want: []entity.Face{facePublished}, wantReads: 1},
		{name: "no readable face stored is a miss",
			gate: resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces("archived")}},
			typ:  "policy", id: "POL-1", wantReads: 1},
		{name: "faceless type", typ: "ticket", id: "TKT-1", want: []entity.Face{""}, wantReads: 1},
		{name: "stored type differs", typ: "ticket", id: "FEAT-1", wantReads: 1},
		{name: "missing id", typ: "ticket", id: "TKT-404", wantReads: 1},
		{name: "row gate denies", gate: resolverGate{deny: map[string]bool{"POL-1": true}}, typ: "policy", id: "POL-1"},
		{name: "row gate errors", gate: resolverGate{err: errors.New("down")}, typ: "policy", id: "POL-1", wantErr: true},
		{name: "no readable face before the store",
			gate: resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.NoFaces()}}, typ: "policy", id: "POL-1"},
		{name: "empty id", typ: "policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resolverStore(t)
			r := mustResolver(t, tc.gate, visibility.NopRedactor{}, st)
			fam, ok, err := r.Family(context.Background(), tc.typ, tc.id)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.want == nil {
				if ok {
					t.Fatalf("got family %+v, want a miss", fam)
				}
			} else if !ok || !slices.Equal(fam.Faces, tc.want) || fam.ID != tc.id || fam.Type != tc.typ {
				t.Fatalf("got (%+v, %v), want faces %v", fam, ok, tc.want)
			}
			if st.Reads() != tc.wantReads {
				t.Errorf("store reads = %d (%s), want %d", st.Reads(), st, tc.wantReads)
			}
			if calls := st.Calls(); calls["ListEntities"]+calls["GetEntity"] != 0 {
				t.Errorf("Family loaded a body: %s", st)
			}
		})
	}
}

// TestResolver_FamilyBudget pins RR-S4S8ZG: Family costs one header query
// whatever the number of faces.
func TestResolver_FamilyBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			for i := range n {
				e := &entity.Entity{ID: "POL-1", Type: "policy", Face: entity.Face(fmt.Sprintf("f%03d", i)),
					Content: strings.Repeat("body ", 100)}
				if err := base.CreateEntity(ctx, e); err != nil {
					t.Fatal(err)
				}
			}
			st := storetest.NewCounting(base)
			r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
			fam, ok, err := r.Family(ctx, "policy", "POL-1")
			if err != nil || !ok || len(fam.Faces) != n {
				t.Fatalf("Family = (%d faces, %v, %v), want %d faces", len(fam.Faces), ok, err, n)
			}
			if got := st.Calls(); st.Reads() != 1 || got["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
		})
	}
}

func TestAllowAllResolver_ReadsEveryFaceAndChecksType(t *testing.T) {
	r, err := visibility.NewAllowAllResolver(resolverStore(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res, ok, err := r.Address(ctx, visibility.WorldOf(store.TrivialScope()), "policy", "POL-1@draft")
	if err != nil || !ok || res.Entity.Properties["salary"] != 1 {
		t.Fatalf("allow-all read = (%v, %v, %v), want the unredacted draft", res.Entity, ok, err)
	}
	if _, ok, _ := r.Address(ctx, visibility.WorldOf(store.TrivialScope()), "policy", "TKT-1"); ok {
		t.Error("allow-all must keep the stored-type check")
	}
}

func TestFaceSet(t *testing.T) {
	if !visibility.NoFaces().IsNone() || !(visibility.FaceSet{}).IsNone() || !visibility.SomeFaces().IsNone() {
		t.Error("the zero, NoFaces and empty SomeFaces sets must all be none")
	}
	if visibility.AllFaces().IsNone() || !visibility.AllFaces().Contains("anything") ||
		!queryFaces(visibility.AllFaces(), nil, true) {

		t.Error("AllFaces must hold every face and push no FaceIn")
	}
	if !queryFaces(visibility.NoFaces(), nil, false) {
		t.Error("NoFaces must refuse to become a query")
	}
	some := visibility.SomeFaces(facePublished)
	if some.Contains(faceDraft) || !some.Contains(facePublished) ||
		!queryFaces(some, []entity.Face{facePublished}, true) {

		t.Error("SomeFaces must hold exactly its list")
	}
}

func queryFaces(s visibility.FaceSet, want []entity.Face, wantOK bool) bool {
	got, ok := s.QueryFaces()
	return ok == wantOK && (got == nil) == (want == nil) && slices.Equal(got, want)
}

// failingLoader fails every read with err.
type failingLoader struct{ err error }

func (f failingLoader) GetEntity(context.Context, entity.Ref) (*entity.Entity, error) {
	return nil, f.err
}

func (f failingLoader) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) { yield(nil, f.err) }
}

// recordingLoader records every ListEntities query.
type recordingLoader struct {
	visibility.Loader
	queries []store.EntityQuery
}

func (r *recordingLoader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	r.queries = append(r.queries, q)
	return r.Loader.ListEntities(ctx, q)
}

// countingPrimer is a redactor that counts PrimeTraversals calls.
type countingPrimer struct {
	hideRedactor
	primed int
}

func (c *countingPrimer) PrimeTraversals(ctx context.Context, _ []*entity.Entity) context.Context {
	c.primed++
	return ctx
}
