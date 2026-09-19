package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// reversedMeta is metaV1 with assigned-to pointing the other way.
func reversedMeta() *metamodel.Metamodel {
	m := metaV1()
	rel := m.Relations["assigned-to"]
	rel.From, rel.To = rel.To, rel.From
	m.Relations["assigned-to"] = rel
	return m
}

// relStore seeds entities and the named edges.
func relStore(t *testing.T, edges ...[2]string) store.Store {
	t.Helper()
	st := memstore.New()
	ctx := t.Context()
	for _, id := range []string{"TSK-1", "TSK-2"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "task",
			Properties: map[string]any{"title": id}}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	for _, id := range []string{"PER-1", "PER-2"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "person",
			Properties: map[string]any{"name": id}}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	for _, e := range edges {
		if _, err := st.CreateRelation(ctx, e[0], "assigned-to", e[1],
			&store.RelationData{Properties: map[string]any{"weight": e[0] + "->" + e[1]}}); err != nil {
			t.Fatalf("seed edge %v: %v", e, err)
		}
	}
	return st
}

// runReverse applies a one-step migration file reversing assigned-to.
func runReverse(t *testing.T, st store.Store, apply bool) (*RunResult, error) {
	t.Helper()
	data := mustFileYAML(t, metaV1(), reversedMeta(),
		"  - reverse_relation: {type: assigned-to}\n")
	f, err := ParseFile("0001-reverse.yaml", data)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	r := newTestRunner(t, Deps{
		Store: st, Meta: reversedMeta(), State: newFakeKV(), Audit: audit.NewMemory(),
	})
	return r.Run(t.Context(), []*File{f}, apply)
}

func TestReverseRelation_RewritesEveryEdge(t *testing.T) {
	st := relStore(t, [2]string{"TSK-1", "PER-1"}, [2]string{"TSK-2", "PER-1"})
	ctx := t.Context()

	res, err := runReverse(t, st, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Files[0].Steps[0].Affected; got != 2 {
		t.Fatalf("affected = %d, want 2", got)
	}

	// The edge exists reversed, carrying its properties.
	got, err := st.GetRelation(ctx, "PER-1", "assigned-to", "TSK-1")
	if err != nil {
		t.Fatalf("reversed edge missing: %v", err)
	}
	if got.Properties["weight"] != "TSK-1->PER-1" {
		t.Errorf("properties lost: %v", got.Properties)
	}
	if _, err := st.GetRelation(ctx, "TSK-1", "assigned-to", "PER-1"); err == nil {
		t.Error("the original direction still exists")
	}
}

// Dry-run must report the count an apply would change, and write nothing.
func TestReverseRelation_DryRunCountsWithoutWriting(t *testing.T) {
	st := relStore(t, [2]string{"TSK-1", "PER-1"})
	res, err := runReverse(t, st, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Files[0].Steps[0].Affected; got != 1 {
		t.Fatalf("affected = %d, want 1", got)
	}
	if _, err := st.GetRelation(t.Context(), "TSK-1", "assigned-to", "PER-1"); err != nil {
		t.Error("dry run rewrote an edge")
	}
}

// A content-scoped edge has no reversed representation: the head has no face
// slot, so the tail would be dropped and every edge differing only by it would
// merge. Refused against the DATA, before any write.
func TestReverseRelation_RefusesAStateTailedEdge(t *testing.T) {
	st := relStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "TSK-1", Type: "task", Face: "draft",
		Properties: map[string]any{"title": "draft"}}); err != nil {
		t.Fatalf("seed face: %v", err)
	}
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "PER-1",
		&store.RelationData{FromFace: "draft"}); err != nil {
		t.Fatalf("seed tailed edge: %v", err)
	}

	_, err := runReverse(t, st, true)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "tailed at face") {
		t.Errorf("not the tail refusal: %v", err)
	}
	face := entity.Face("draft")
	found := false
	for r, gErr := range st.ListRelations(ctx, store.RelationQuery{Type: "assigned-to", FromFace: &face}) {
		if gErr != nil {
			t.Fatalf("list: %v", gErr)
		}
		if r.From == "TSK-1" && r.To == "PER-1" {
			found = true
		}
	}
	if !found {
		t.Error("the refused edge was modified")
	}
}

// The refusal must fire on a DRY RUN too, or an operator reviewing the plan is
// told the migration is fine and only discovers otherwise on apply.
func TestReverseRelation_TailRefusalFiresOnDryRun(t *testing.T) {
	st := relStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "TSK-1", Type: "task", Face: "draft",
		Properties: map[string]any{"title": "draft"}}); err != nil {
		t.Fatalf("seed face: %v", err)
	}
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "PER-1",
		&store.RelationData{FromFace: "draft"}); err != nil {
		t.Fatalf("seed tailed edge: %v", err)
	}
	if _, err := runReverse(t, st, false); err == nil {
		t.Fatal("dry run reported no problem for a migration that cannot apply")
	}
}

// Two edges pointing at each other would swap onto one triple. Refused whole:
// a half-applied rewrite loses one and leaves the other carrying its content.
func TestReverseRelation_RefusesBothDirections(t *testing.T) {
	st := relStore(t)
	ctx := t.Context()
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "TSK-2", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := st.CreateRelation(ctx, "TSK-2", "assigned-to", "TSK-1", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := runReverse(t, st, true); err == nil {
		t.Fatal("expected a refusal")
	}
	for _, e := range [][2]string{{"TSK-1", "TSK-2"}, {"TSK-2", "TSK-1"}} {
		if _, err := st.GetRelation(ctx, e[0], "assigned-to", e[1]); err != nil {
			t.Errorf("edge %v was destroyed by a refused run: %v", e, err)
		}
	}
}

// The dry-run must reach the same refusal an apply would. Without this an
// operator reviews "would change 2 records", approves it, and the apply fails —
// and on a backend whose refusal comes from a unique constraint there is no
// read-only path to that answer, so the step has to compute it.
func TestReverseRelation_DryRunRefusesBothDirections(t *testing.T) {
	st := relStore(t)
	ctx := t.Context()
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "TSK-2", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := st.CreateRelation(ctx, "TSK-2", "assigned-to", "TSK-1", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := runReverse(t, st, false)
	if err == nil {
		t.Fatal("dry run reported no problem for a migration that cannot apply")
	}
	if !strings.Contains(err.Error(), "merge two distinct edges") {
		t.Errorf("not the collision refusal: %v", err)
	}
	// And it must not claim a count it will not deliver.
	if len(res.Files) > 0 && len(res.Files[0].Steps) > 0 {
		if got := res.Files[0].Steps[0].Affected; got != 0 {
			t.Errorf("refused dry run still reported %d affected", got)
		}
	}
}

// A self-edge reverses to itself. The regression: create-then-delete conflicts
// on the create and then deletes the only copy.
func TestReverseRelation_SelfEdgeSurvives(t *testing.T) {
	st := relStore(t)
	ctx := t.Context()
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "TSK-1", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := runReverse(t, st, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := st.GetRelation(ctx, "TSK-1", "assigned-to", "TSK-1"); err != nil {
		t.Errorf("the self-edge was destroyed: %v", err)
	}
}

func TestReverseRelation_ValidateRefusals(t *testing.T) {
	swapped := func(m *metamodel.Metamodel) *metamodel.Metamodel { return m }
	tests := []struct {
		name    string
		from    func() *metamodel.Metamodel
		to      func() *metamodel.Metamodel
		wantErr string
	}{
		{
			name: "symmetric type",
			from: func() *metamodel.Metamodel {
				m := metaV1()
				r := m.Relations["assigned-to"]
				r.Symmetric = true
				m.Relations["assigned-to"] = r
				return m
			},
			to: func() *metamodel.Metamodel {
				m := reversedMeta()
				r := m.Relations["assigned-to"]
				r.Symmetric = true
				m.Relations["assigned-to"] = r
				return m
			},
			wantErr: "symmetric",
		},
		{
			name: "overlapping endpoints",
			from: func() *metamodel.Metamodel {
				m := metaV1()
				r := m.Relations["assigned-to"]
				r.From = []string{"task"}
				r.To = []string{"task", "person"}
				m.Relations["assigned-to"] = r
				return m
			},
			to: func() *metamodel.Metamodel {
				m := metaV1()
				r := m.Relations["assigned-to"]
				r.From = []string{"task", "person"}
				r.To = []string{"task"}
				m.Relations["assigned-to"] = r
				return m
			},
			wantErr: "overlapping endpoints",
		},
		{
			name:    "schema does not declare a swap",
			from:    metaV1,
			to:      func() *metamodel.Metamodel { return swapped(metaV1()) },
			wantErr: "not reversed between the two schemas",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			step := &reverseRelationStep{Type: "assigned-to"}
			err := step.Validate(tc.from().ShapeProjection(), tc.to().ShapeProjection())
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

// The enforcement the review found would silently never fire: a file spanning
// the delta with no step must be refused, and one WITH the step must parse.
func TestReverseRelation_FileMustCarryTheStep(t *testing.T) {
	t.Run("missing step is refused", func(t *testing.T) {
		data := mustFileYAML(t, metaV1(), reversedMeta(), "  []\n")
		_, err := ParseFile("0001-reverse.yaml", data)
		if err == nil {
			t.Fatal("a file spanning the swap with no step must be refused")
		}
		if !strings.Contains(err.Error(), "reverse_relation") {
			t.Errorf("error does not name the required step: %v", err)
		}
	})

	t.Run("correct file parses", func(t *testing.T) {
		data := mustFileYAML(t, metaV1(), reversedMeta(),
			"  - reverse_relation: {type: assigned-to}\n")
		if _, err := ParseFile("0001-reverse.yaml", data); err != nil {
			t.Fatalf("a file carrying the step must parse: %v", err)
		}
	})
}

// The generator must draft the step live, not the old "no declarative step can
// fix this" comment.
func TestReverseRelation_GeneratorDraftsTheStep(t *testing.T) {
	out, err := Generate(metaV1().ShapeProjection(), reversedMeta().ShapeProjection(), nil, "reverse")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out == nil {
		t.Fatal("no file drafted")
	}
	body := string(out.Content)
	if !strings.Contains(body, "- reverse_relation: {type: assigned-to}") {
		t.Errorf("draft does not carry a live reverse_relation step:\n%s", body)
	}
	if strings.Contains(body, "no declarative step can fix this") {
		t.Errorf("draft still claims nothing can fix this:\n%s", body)
	}
	if _, err := ParseFile(out.FileName, out.Content); err != nil {
		t.Errorf("the drafted file does not parse: %v", err)
	}
}

// Swapping the endpoints without exchanging the bounds leaves the reversed data
// violating the schema it was migrated to satisfy.
func TestReverseRelation_WarnsWhenCardinalityWasNotSwapped(t *testing.T) {
	from := metaV1()
	r := from.Relations["assigned-to"]
	one := 1
	r.MaxOutgoing = &one
	from.Relations["assigned-to"] = r

	to := reversedMeta()
	r2 := to.Relations["assigned-to"]
	r2.MaxOutgoing = &one // left behind: should have become MaxIncoming
	to.Relations["assigned-to"] = r2

	out, err := Generate(from.ShapeProjection(), to.ShapeProjection(), nil, "reverse")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out == nil {
		t.Fatal("no file drafted")
	}
	if !strings.Contains(string(out.Content), "bounds were not") {
		t.Errorf("no cardinality warning in the draft:\n%s", out.Content)
	}
}

// One delta, not two narrowings — the noise the ticket exists to remove.
func TestReverseRelation_SwapIsOneDelta(t *testing.T) {
	report := metamodel.CompareShapes(metaV1().ShapeProjection(), reversedMeta().ShapeProjection())
	kinds := map[string]int{}
	for _, d := range report.Deltas {
		kinds[d.Kind]++
	}
	if kinds["relation_endpoints_swapped"] != 1 {
		t.Errorf("want exactly one swap delta, got %v", kinds)
	}
	if kinds["relation_endpoint_narrowed"] != 0 {
		t.Errorf("swap still reported as narrowing: %v", kinds)
	}
}
