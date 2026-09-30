package visibility

import (
	"context"
	"errors"
	"iter"
	"reflect"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// A face grant is the second half of a read permission, and it can only be
// applied AFTER the row is loaded — an entity's face is not knowable from its
// (type, id). [RowGate] answers only the first half, so before [FaceGate]
// existed every read-out path that used a Reader was face-blind.
//
// That was a cross-principal content leak in production shape: the view
// surface served a draft body to a principal granted only `policy@published`,
// while the entity route 404'd the very same face (TKT-O7R2A1). The gate lives
// here, in the one reader all those paths already route through, rather than
// being hand-placed at each call site — four such sites had accumulated and
// two of them were still ungated when this was written.

// faceRowGate is a RowGate that also declares permitted faces per type.
type faceRowGate struct {
	// permitted maps entity type -> readable faces. A type absent from the
	// map is unrestricted, matching the "empty means every face" contract.
	permitted map[string][]entity.Face
	err       error
}

func (g faceRowGate) PermitsRead(context.Context, string, string) (bool, error) {
	return true, nil
}

func (g faceRowGate) PermitsReadMany(
	_ context.Context, _ string, ids []string,
) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (g faceRowGate) PermittedFaces(_ context.Context, entityType string) ([]entity.Face, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.permitted[entityType], nil
}

// plainRowGate permits every row and does NOT implement FaceGate — the shape
// of every pre-existing gate and test double.
type plainRowGate struct{}

func (plainRowGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

func (plainRowGate) PermitsReadMany(
	_ context.Context, _ string, ids []string,
) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// faceGetter serves one entity, at a face the test chooses.
type faceGetter struct{ e *entity.Entity }

func (g faceGetter) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(func(*entity.Entity, error) bool) {}
}

// readFace reads e's own address through r's resolver, in the default world.
func readFace(r *PolicyReader, e *entity.Entity) (*entity.Entity, bool, error) {
	res, ok, err := r.Resolver().Address(context.Background(), WorldOf(store.TrivialScope()), "policy", e.Ref().String())
	return res.Entity, ok, err
}

func (g faceGetter) GetEntity(context.Context, entity.Ref) (*entity.Entity, error) {
	if g.e == nil {
		return nil, errors.New("not found")
	}
	return g.e, nil
}

const published = entity.Face("published")

func draftPolicy() *entity.Entity {
	return &entity.Entity{ID: "POL-1", Type: "policy", Content: "draft body"}
}

func publishedPolicy() *entity.Entity {
	return &entity.Entity{ID: "POL-1", Type: "policy", Face: published, Content: "published body"}
}

func TestFaceGate_GetDeniesAnUngrantedFace(t *testing.T) {
	// The grant names `published`; the store holds the DRAFT face, which is
	// the empty/zero Face. That pairing is the leak's exact shape: the bare
	// face serializes as "" so a naive check passes it.
	r, err := NewPolicyReader(
		faceRowGate{permitted: map[string][]entity.Face{"policy": {published}}},
		NopRedactor{},
		faceGetter{e: draftPolicy()},
	)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}

	got, ok, gerr := readFace(r, draftPolicy())
	if gerr != nil {
		t.Fatalf("Get: %v", gerr)
	}
	if ok {
		t.Fatalf("Get served face %q to a principal granted only %q; content=%q",
			got.Face, published, got.Content)
	}
}

func TestFaceGate_GetServesTheGrantedFace(t *testing.T) {
	// The mirror of the test above. Without it, a gate that denied
	// EVERYTHING would pass — an outage reads as a successful denial.
	r, err := NewPolicyReader(
		faceRowGate{permitted: map[string][]entity.Face{"policy": {published}}},
		NopRedactor{},
		faceGetter{e: publishedPolicy()},
	)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}

	got, ok, gerr := readFace(r, publishedPolicy())
	if gerr != nil {
		t.Fatalf("Get: %v", gerr)
	}
	if !ok {
		t.Fatal("Get denied the granted face")
	}
	if got.Face != published {
		t.Errorf("got face %q, want %q", got.Face, published)
	}
}

// An empty permitted set means EVERY face, not "no faces".
//
// This is the backward-compatibility contract and it is deliberately NOT the
// fail-closed direction: a world resolves each entity through its chain and
// never serves the default face, so a bare grant clamped to the default would
// read nothing at all under any world — a total outage rather than a
// narrowing. Writes differ, and do fail closed, because they address a face by
// id and never pass through a world.
func TestFaceGate_EmptyGrantMeansEveryFace(t *testing.T) {
	for _, tc := range []struct {
		name string
		e    *entity.Entity
	}{
		{"default face", draftPolicy()},
		{"named face", publishedPolicy()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewPolicyReader(
				faceRowGate{permitted: map[string][]entity.Face{}},
				NopRedactor{},
				faceGetter{e: tc.e},
			)
			if err != nil {
				t.Fatalf("NewPolicyReader: %v", err)
			}
			if _, ok, gerr := readFace(r, tc.e); gerr != nil || !ok {
				t.Errorf("an unrestricted grant must read every face; ok=%v err=%v", ok, gerr)
			}
		})
	}
}

// A gate that does not implement FaceGate grants every face — the behavior a
// project declaring no faces has always had, and what keeps every existing
// RowGate implementation working unchanged.
func TestFaceGate_NonFaceGateIsUnrestricted(t *testing.T) {
	r, err := NewPolicyReader(plainRowGate{}, NopRedactor{}, faceGetter{e: publishedPolicy()})
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	if _, ok, gerr := readFace(r, publishedPolicy()); gerr != nil || !ok {
		t.Errorf("a gate without FaceGate must not restrict faces; ok=%v err=%v", ok, gerr)
	}
}

// A gate failure HIDES, matching the package's fail-closed contract. An error
// here must not be mistaken for "no restriction" — that is the direction that
// leaks.
func TestFaceGate_GateErrorFailsClosed(t *testing.T) {
	r, err := NewPolicyReader(
		faceRowGate{err: errors.New("gate down")},
		NopRedactor{},
		faceGetter{e: publishedPolicy()},
	)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	if _, ok, _ := readFace(r, publishedPolicy()); ok {
		t.Error("a gate error must hide the row, not reveal it")
	}
}

// Filter and FilterHeaders enforce the same verdict as Get.
//
// Both are asserted because they are separate loops over separate types, and
// the leak that motivated this reached production through the collection path
// while the single-entity path was already gated.
func TestFaceGate_FilterAndHeadersAgreeWithGet(t *testing.T) {
	gate := faceRowGate{permitted: map[string][]entity.Face{"policy": {published}}}
	r, err := NewPolicyReader(gate, NopRedactor{}, faceGetter{e: publishedPolicy()})
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	ctx := context.Background()

	kept := r.Filter(ctx, []*entity.Entity{draftPolicy(), publishedPolicy()})
	if len(kept) != 1 {
		t.Fatalf("Filter kept %d rows, want 1 (the published face only)", len(kept))
	}
	if kept[0].Face != published {
		t.Errorf("Filter kept face %q; the denied draft leaked", kept[0].Face)
	}

	heads := r.FilterHeaders(ctx, []store.EntityHeader{
		{ID: "POL-1", Type: "policy"},
		{ID: "POL-1", Type: "policy", Face: published},
	})
	if len(heads) != 1 {
		t.Fatalf("FilterHeaders kept %d rows, want 1", len(heads))
	}
	if heads[0].Face != published {
		t.Errorf("FilterHeaders kept face %q; the denied draft leaked", heads[0].Face)
	}
}

// TestVisibleTracer_IsFaceGated: the base tracer reads every node's DEFAULT
// face, so the row gate alone surfaced draft titles and properties to a
// principal granted only `ticket@published` — while a single-entity read on the
// same entity correctly reported not-found. The decorator now applies the face
// gate to the default face per type.
func TestVisibleTracer_IsFaceGated(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "SECRET DRAFT"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "SECRET DRAFT TWO"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "blocks", To: "TKT-2"}, nil); err != nil {
		t.Fatal(err)
	}
	base := tracer.New(st, store.TrivialScope())

	// Control: with every face permitted the trace carries the draft title,
	// so the absence below is the gate's doing and not an empty fixture.
	openRes, err := NewResolver(faceRowGate{}, NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	open, err := NewVisibleTracer(base, openRes, st, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	if res := open.TraceFrom(ctx, "TKT-1", 3); res == nil || res.Title != "SECRET DRAFT" {
		t.Fatalf("precondition: an unrestricted trace must show the draft; got %+v", res)
	}

	gatedRes, err := NewResolver(
		faceRowGate{permitted: map[string][]entity.Face{"ticket": {"published"}}},
		NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	gated, err := NewVisibleTracer(base, gatedRes, st, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	res := gated.TraceFrom(ctx, "TKT-1", 3)
	if res != nil {
		t.Errorf("a published-only principal must see nothing of a draft-only "+
			"trace; got root %q with %d children", res.Title, len(res.Children))
	}
}

// A principal who cannot read one face of a family (A4, BUG-95W7MV): the
// hidden face is absent from every face list, and an edge hung from it
// neither connects the family nor appears in a trace or path.
func TestVisibleTracer_HiddenFace(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Draft"}},
		{ID: "POL-1", Type: "policy", Face: published, Properties: map[string]any{"title": "Published"}},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Control"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "implements", To: "CTL-1"}, &store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	base := tracer.New(st, store.TrivialScope())
	build := func(g faceRowGate) *VisibleTracer {
		t.Helper()
		res, err := NewResolver(g, NopRedactor{}, st)
		if err != nil {
			t.Fatal(err)
		}
		tr, err := NewVisibleTracer(base, res, st, store.TrivialScope())
		if err != nil {
			t.Fatal(err)
		}
		return tr
	}
	open := build(faceRowGate{})
	gated := build(faceRowGate{permitted: map[string][]entity.Face{"policy": {published}}})

	t.Run("orphans", func(t *testing.T) {
		// Control: with every face readable the draft edge connects both.
		if got, err := open.FindOrphans(ctx); err != nil || len(got) != 0 {
			t.Fatalf("open orphans = %+v, %v; want none", got, err)
		}
		got, err := gated.FindOrphans(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := []tracer.Orphan{
			{ID: "CTL-1", Type: "control", Title: "Control"},
			{ID: "POL-1", Type: "policy", Faces: []entity.Face{published}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("gated orphans = %+v, want %+v", got, want)
		}
	})

	t.Run("trace", func(t *testing.T) {
		if res := open.TraceFrom(ctx, "POL-1", 3); res == nil || len(res.Children) != 1 ||
			!reflect.DeepEqual(res.Faces, []entity.Face{"draft", published}) {

			t.Fatalf("open trace = %+v; want both faces and the draft edge", res)
		}
		res := gated.TraceFrom(ctx, "POL-1", 3)
		if res == nil {
			t.Fatal("the published face is readable, so the family must trace")
		}
		if !reflect.DeepEqual(res.Faces, []entity.Face{published}) {
			t.Errorf("faces = %v, want [published]", res.Faces)
		}
		if len(res.Children) != 0 {
			t.Errorf("an edge hung from the hidden draft leaked: %+v", res.Children[0])
		}
		if up := gated.TraceTo(ctx, "CTL-1", 3); up == nil || len(up.Children) != 0 {
			t.Errorf("TraceTo leaked the draft edge: %+v", up)
		}
	})

	t.Run("path", func(t *testing.T) {
		if steps := open.FindPath(ctx, "POL-1", "CTL-1"); len(steps) != 2 {
			t.Fatalf("open path = %+v; want 2 steps", steps)
		}
		if steps := gated.FindPath(ctx, "POL-1", "CTL-1"); steps != nil {
			t.Errorf("path over the hidden draft edge leaked: %+v", steps)
		}
	})
}

// The traversal follows only readable edges, so a visible node first
// reachable through a hidden one still shows its visible subtree. A
// post-hoc filter lost it: the base expands each id once, under the hidden
// branch that is then pruned, and the visible occurrence was a bare leaf.
//
// From R, outgoing edges are walked first: R -> H -> X -> Y would expand X
// under the hidden H. X -> R (incoming to R) is the visible route.
func TestVisibleTracer_TraversalSkipsHiddenEdges(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "R", Type: "note"},
		{ID: "H", Type: "policy", Face: "draft"},
		{ID: "X", Type: "note"},
		{ID: "Y", Type: "note"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range [][2]string{{"R", "H"}, {"H", "X"}, {"X", "R"}, {"X", "Y"}} {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: r[0], Type: "links", To: r[1]}, nil); err != nil {
			t.Fatal(err)
		}
	}
	res, err := NewResolver(faceRowGate{permitted: map[string][]entity.Face{"policy": {published}}}, NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := NewVisibleTracer(tracer.New(st, store.TrivialScope()), res, st, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}

	root := tr.TraceFrom(ctx, "R", 0)
	if root == nil || len(root.Children) != 1 || root.Children[0].ID != "X" {
		t.Fatalf("trace = %+v, want R with the single visible child X", root)
	}
	var kids []string
	for _, c := range root.Children[0].Children {
		kids = append(kids, c.ID)
	}
	if !slices.Contains(kids, "Y") {
		t.Fatalf("X lost its visible child Y: children %v", kids)
	}
	if steps := tr.FindPath(ctx, "R", "Y"); len(steps) != 3 {
		t.Errorf("path R..Y = %+v, want the visible route R, X, Y", steps)
	}
	if tr.HasCycle(ctx, "X") {
		t.Error("the only cycle through X passes through the hidden H")
	}
}
