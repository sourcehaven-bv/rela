package analysis_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// BUG-95W7MV: a faced type has no row at the bare id, so every check that
// read only the default face left faced types out of `rela analyze`. These
// tests run each check on a project that mixes faced and faceless types and
// assert the faced type appears, with the coverage the design names.

// mixedFacedService seeds:
//   - POL-1 at draft and published, no edges: an orphan family.
//   - POL-2 at draft, with a draft-tailed edge to CTL-1: connected.
//   - POL-3 at published only, sharing POL-1's title and code.
//   - POL-5 at published only, so POL-4 is a gap.
//   - NOTE-1, faceless and unconnected: an orphan with a title.
func mixedFacedService(t *testing.T) *analysis.Service {
	t.Helper()
	meta := facedPolicyMeta()
	def := meta.Entities["policy"]
	def.Properties["code"] = metamodel.PropertyDef{Type: "string", Unique: true}
	meta.Entities["policy"] = def
	meta.Entities["control"] = metamodel.EntityDef{Label: "Control"}
	meta.Entities["note"] = metamodel.EntityDef{Label: "Note"}
	meta.Relations = map[string]metamodel.RelationDef{"implements": {
		From: []string{"policy"}, To: []string{"control"}, Scope: metamodel.ScopeContent,
	}}
	return newServiceWith(t, meta, func(st store.Store) {
		ctx := context.Background()
		for _, e := range []*entity.Entity{
			{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Access", "code": "A"}},
			{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "Access", "code": "A"}},
			{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Backup"}},
			{ID: "POL-3", Type: "policy", Face: "published", Properties: map[string]any{"title": "Access", "code": "A"}},
			{ID: "POL-5", Type: "policy", Face: "published", Properties: map[string]any{"title": "Crypto"}},
			{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Control"}},
			{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "Loose note"}},
		} {
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
			}
		}
		if _, err := st.CreateRelation(ctx, "POL-2", "implements", "CTL-1",
			&store.RelationData{FromFace: "draft"}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	})
}

func TestFindOrphans_ReportsFacedFamilies(t *testing.T) {
	got, err := mixedFacedService(t).FindOrphansWithScope(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []analysis.Orphan{
		{ID: "NOTE-1", Type: "note", Title: "Loose note"},
		{ID: "POL-1", Type: "policy", Faces: []entity.Face{"draft", "published"}},
		{ID: "POL-3", Type: "policy", Faces: []entity.Face{"published"}},
		{ID: "POL-5", Type: "policy", Faces: []entity.Face{"published"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orphans =\n %+v\nwant\n %+v", got, want)
	}
}

func TestFindDuplicates_EveryFaceDifferentIDs(t *testing.T) {
	got, err := mixedFacedService(t).FindDuplicates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d groups, want 1 (POL-1's two faces alone are not duplicates): %+v", len(got), got)
	}
	var refs []string
	for _, e := range got[0].Entities {
		refs = append(refs, e.Ref().String())
	}
	want := []string{"POL-1@draft", "POL-1@published", "POL-3@published"}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("group refs = %v, want %v", refs, want)
	}
}

func TestFindUniqueViolations_PerFace(t *testing.T) {
	got, err := mixedFacedService(t).FindUniqueViolations(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Draft holds code A on POL-1 only, so only the published face collides.
	if len(got) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(got), got)
	}
	v := got[0]
	if v.Face != "published" || v.Value != "A" || len(v.Entities) != 2 {
		t.Fatalf("violation = %+v, want published/A over POL-1 and POL-3", v)
	}
	if v.Entities[0].ID != "POL-1" || v.Entities[1].ID != "POL-3" {
		t.Fatalf("violation ids = %s, %s", v.Entities[0].ID, v.Entities[1].ID)
	}
}

func TestFindGaps_CountsFacedIDsOnce(t *testing.T) {
	got, err := mixedFacedService(t).FindGaps(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var pol *analysis.GapResult
	for i := range got {
		if got[i].Prefix == "POL-" {
			pol = &got[i]
		}
	}
	if pol == nil {
		t.Fatalf("faced prefix POL- absent from gaps: %+v", got)
	}
	if !reflect.DeepEqual(pol.Missing, []string{"POL-004"}) {
		t.Fatalf("POL- gaps = %v, want [POL-004]", pol.Missing)
	}
}

func TestCheckCardinality_NamesTheFace(t *testing.T) {
	one := 1
	meta := facedPolicyMeta()
	meta.Entities["control"] = metamodel.EntityDef{Label: "Control"}
	meta.Relations = map[string]metamodel.RelationDef{"implements": {
		From: []string{"policy"}, To: []string{"control"},
		MinOutgoing: &one, Scope: metamodel.ScopeContent,
	}}
	svc := newServiceWith(t, meta, func(st store.Store) {
		ctx := context.Background()
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy", Face: "draft"})
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy", Face: "published"})
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "CTL-1", Type: "control"})
		if _, err := st.CreateRelation(ctx, "POL-1", "implements", "CTL-1",
			&store.RelationData{FromFace: "draft"}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	})
	got, err := svc.CheckCardinality(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Face != "published" {
		t.Fatalf("violations = %+v, want one on POL-1@published", got)
	}
}
