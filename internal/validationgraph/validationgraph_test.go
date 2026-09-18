package validationgraph_test

import (
	"context"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/validation"
	"github.com/Sourcehaven-BV/rela/internal/validationgraph"
)

// fixture builds a memstore holding A --r--> B and returns a Graph over it.
func fixture(t *testing.T) *validationgraph.Graph {
	t.Helper()
	ctx := t.Context()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "A", Type: "ticket"},
		{ID: "B", Type: "review-checklist", Properties: map[string]any{"status": "done"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "A", "has-review", "B", nil); err != nil {
		t.Fatalf("create relation: %v", err)
	}
	g, err := validationgraph.New(st)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

// Direction must select the edge SET, not merely which endpoint is read from
// an already-chosen set.
//
// This is the regression that motivated the package. store.RelationQuery
// honors Direction for EntityID/EntityIDs only — From and To are matched
// unconditionally — so the natural-looking `{From: id, Direction: Incoming}`
// silently returns the entity's OUTGOING edges. A test that only checked
// "which end did we read" would pass against that bug, because the endpoint
// logic would be correct and the edge set wrong. So this asserts both
// directions from both ends.
func TestRelatedEntities_DirectionSelectsTheEdgeSet(t *testing.T) {
	g := fixture(t)
	ctx := t.Context()

	tests := []struct {
		name    string
		subject string
		dir     validation.Direction
		wantIDs []string
	}{
		// A is the SOURCE of the only edge.
		{"outgoing from source finds the target", "A", validation.DirectionOutgoing, []string{"B"}},
		{"incoming from source finds nothing", "A", validation.DirectionIncoming, nil},
		// B is the TARGET. Its incoming edge is the one the atlas case needs.
		{"incoming from target finds the source", "B", validation.DirectionIncoming, []string{"A"}},
		{"outgoing from target finds nothing", "B", validation.DirectionOutgoing, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.RelatedEntities(ctx, tc.subject, "has-review", tc.dir, true)
			if err != nil {
				t.Fatalf("RelatedEntities: %v", err)
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d edges, want %d (%v)", len(got), len(tc.wantIDs), tc.wantIDs)
			}
			for i, want := range tc.wantIDs {
				if got[i].ID != want {
					t.Errorf("edge %d: far end = %q, want %q", i, got[i].ID, want)
				}
				if !got[i].Resolved {
					t.Errorf("edge %d: want Resolved, the entity exists", i)
				}
			}
		})
	}
}

// The far entity's properties must reach the caller, since `where` matches
// against them.
func TestRelatedEntities_CarriesTargetProperties(t *testing.T) {
	got, err := fixture(t).RelatedEntities(t.Context(), "A", "has-review", validation.DirectionOutgoing, true)
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d edges, want 1", len(got))
	}
	if got[0].Type != "review-checklist" {
		t.Errorf("Type = %q, want review-checklist", got[0].Type)
	}
	if got[0].Properties["status"] != "done" {
		t.Errorf("Properties[status] = %v, want done", got[0].Properties["status"])
	}
}

// A dangling edge must come back UNRESOLVED rather than be dropped.
//
// Dropping it would undercount, which a `max:` bound reads as success — so a
// `max: 0` gate would report satisfied precisely because the thing it guards
// against could not be read. The evaluator can only make that choice if the
// edge survives the trip.
func TestRelatedEntities_UnreadableTargetSurvivesAsUnresolved(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "A", Type: "ticket"}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	// memstore permits an edge to an absent entity, which is exactly the
	// dangling-reference case.
	if _, err := st.CreateRelation(ctx, "A", "has-review", "GONE", nil); err != nil {
		t.Skipf("backend refuses a dangling edge (%v); nothing to assert here", err)
	}
	g, err := validationgraph.New(st)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := g.RelatedEntities(ctx, "A", "has-review", validation.DirectionOutgoing, true)
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d edges, want 1 — an unreadable target must not remove the edge", len(got))
	}
	if got[0].Resolved {
		t.Error("want Resolved=false for a target that could not be read")
	}
	if got[0].ID != "GONE" {
		t.Errorf("ID = %q, want GONE — the edge must still name its far end", got[0].ID)
	}
}

// A nil reader is a wiring mistake and must be refused at construction, not
// silently produce a graph that counts nothing.
func TestNew_RejectsNilReader(t *testing.T) {
	if _, err := validationgraph.New(nil); err == nil {
		t.Fatal("New(nil) must fail: a graph that counts nothing satisfies every max gate")
	}
}

// A typed nil survives a plain `!= nil` check at the wiring site, so the
// constructor has to catch it here. Otherwise it panics on first use — a
// crash in whichever request happened to evaluate a gate, rather than the
// wiring error it actually is.
func TestNew_RejectsTypedNilReader(t *testing.T) {
	var nilStore *memstore.MemStore
	if _, err := validationgraph.New(nilStore); err == nil {
		t.Fatal("New must reject a typed-nil reader")
	}
}

// countingReader counts entity reads so a budget test can assert that a
// constraint needing no far-entity data performs none.
type countingReader struct {
	inner validationgraph.Reader
	gets  int
}

func (c *countingReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	c.gets++
	return c.inner.GetEntity(ctx, id)
}

func (c *countingReader) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return c.inner.ListRelations(ctx, q)
}

// A constraint that inspects nothing about the far end must read nothing.
//
// Before resolveFar existed, the adapter resolved every edge eagerly, so a
// bare `min: 1` gate did one lookup per edge where the pre-seam code did
// none — a per-row read on what is a whole-graph scan, and invisible to every
// behavioral test because the count was still correct.
func TestRelatedEntities_NoFarReadsWhenNotRequested(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "A", Type: "ticket"},
		{ID: "B", Type: "review-checklist"},
		{ID: "C", Type: "review-checklist"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create %s: %v", e.ID, err)
		}
	}
	for _, to := range []string{"B", "C"} {
		if _, err := st.CreateRelation(ctx, "A", "has-review", to, nil); err != nil {
			t.Fatalf("create relation to %s: %v", to, err)
		}
	}
	cr := &countingReader{inner: st}
	g, err := validationgraph.New(cr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := g.RelatedEntities(ctx, "A", "has-review", validation.DirectionOutgoing, false)
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d edges, want 2 — the edges must still be counted", len(got))
	}
	if cr.gets != 0 {
		t.Errorf("performed %d entity reads with resolveFar=false; want 0", cr.gets)
	}

	// And it still reads when asked.
	cr.gets = 0
	if _, err := g.RelatedEntities(ctx, "A", "has-review", validation.DirectionOutgoing, true); err != nil {
		t.Fatalf("RelatedEntities(resolveFar): %v", err)
	}
	if cr.gets != 2 {
		t.Errorf("performed %d entity reads with resolveFar=true; want 2", cr.gets)
	}
}
