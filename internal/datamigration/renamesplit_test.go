package datamigration

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// splitFrom is a schema with one task type; splitTo replaces it with two
// variants, the way an abstract type with `extends:` loads.
func splitFrom() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{"soort": {Values: []string{"ontwikkeling", "onderhoud", "audit"}}},
		Entities: map[string]metamodel.EntityDef{
			"taak": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
				"soort": {Type: "soort"},
				"prio":  {Type: "integer"},
				"klaar": {Type: "boolean"},
				"tags":  {Type: "string", List: true},
				"due":   {Type: "date"},
			}},
			"person": {Properties: map[string]metamodel.PropertyDef{"name": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"bewijs": {From: []string{"taak"}, To: []string{"person"}},
			"ziet":   {From: []string{"person"}, To: []string{"taak"}},
		},
	}
}

func splitTo() *metamodel.Metamodel {
	m := splitFrom()
	taak := m.Entities["taak"]
	delete(m.Entities, "taak")
	m.Entities["isms.taak"] = taak
	m.Entities["projecten.taak"] = taak
	// Evidence is an ISMS concern: only isms.taak may carry it.
	m.Relations["bewijs"] = metamodel.RelationDef{From: []string{"isms.taak"}, To: []string{"person"}}
	m.Relations["ziet"] = metamodel.RelationDef{From: []string{"person"}, To: []string{"isms.taak", "projecten.taak"}}
	return m
}

const splitSteps = `  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: [ontwikkeling, onderhoud]}}
  - rename_entity_type: {from: taak, to: isms.taak}
`

func seedSplit(t *testing.T) store.Store {
	t.Helper()
	ctx := t.Context()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "taak", Properties: map[string]any{"title": "a", "soort": "ontwikkeling", "prio": 1, "klaar": true}},
		{ID: "T-2", Type: "taak", Properties: map[string]any{"title": "b", "soort": "audit", "prio": 2}},
		{ID: "T-3", Type: "taak", Properties: map[string]any{"title": "c", "soort": "onderhoud", "prio": float64(1)}},
		{ID: "T-4", Type: "taak", Properties: map[string]any{"title": "d"}},
		{ID: "P-1", Type: "person", Properties: map[string]any{"name": "Ada"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "T-2", Type: "bewijs", To: "P-1"},
		{From: "P-1", Type: "ziet", To: "T-1"},
		{From: "P-1", Type: "ziet", To: "T-2"},
	} {
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func typesOf(t *testing.T, st store.Store, ids ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, id := range ids {
		out[id] = getEntity(t, st, id).Type
	}
	return out
}

func TestRenameSplit_MovesSubsetKeepsIDsAndRelations(t *testing.T) {
	st := seedSplit(t)
	ctx := t.Context()
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(), splitSteps))
	r := newTestRunner(t, Deps{Store: st, Meta: splitTo()})

	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := map[string]string{"T-1": "projecten.taak", "T-2": "isms.taak", "T-3": "projecten.taak", "T-4": "isms.taak"}
	if got := typesOf(t, st, "T-1", "T-2", "T-3", "T-4"); !maps.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
	if got := getEntity(t, st, "T-1").Properties["title"]; got != "a" {
		t.Errorf("T-1 lost its content: title = %v", got)
	}
	if steps := res.Files[0].Steps; steps[0].Affected != 2 || steps[1].Affected != 2 {
		t.Errorf("affected = %d, %d; want 2, 2", steps[0].Affected, steps[1].Affected)
	}
	n, err := st.CountRelations(ctx, store.RelationQuery{})
	if err != nil || n != 3 {
		t.Fatalf("relations = %d (%v), want all 3 kept", n, err)
	}
	if _, gerr := st.GetRelation(ctx, entity.RelationKey{From: "P-1", Type: "ziet", To: "T-1"}); gerr != nil {
		t.Errorf("edge to a moved entity is gone: %v", gerr)
	}

	// Re-running the applied file is safe: nothing is left at taak.
	again, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	for _, sr := range again.Files[0].Steps {
		if sr.Affected != 0 {
			t.Errorf("re-run %s affected %d, want 0", sr.Target, sr.Affected)
		}
	}
	if notes := again.Files[0].Steps[0].Notes; !slices.Contains(notes, "matched no entities") {
		t.Errorf("re-run notes = %v, want an explicit empty-match note", notes)
	}
	if got := typesOf(t, st, "T-1", "T-2", "T-3", "T-4"); !maps.Equal(got, want) {
		t.Errorf("re-run changed types: %v", got)
	}
}

func TestRenameSplit_DryRunCountsEachStepOnce(t *testing.T) {
	st := seedSplit(t)
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(), splitSteps))
	r := newTestRunner(t, Deps{Store: st, Meta: splitTo()})

	res, err := r.Run(t.Context(), []*File{f}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	steps := res.Files[0].Steps
	if steps[0].Affected != 2 || steps[1].Affected != 2 {
		t.Errorf("dry-run affected = %d, %d; want 2, 2 (the rest excludes what the first step takes)",
			steps[0].Affected, steps[1].Affected)
	}
	if !slices.Contains(steps[0].Notes, "selected: T-1, T-3") {
		t.Errorf("notes = %v, want the selected ids listed", steps[0].Notes)
	}
	if !slices.Contains(steps[1].Notes, "selected: T-2, T-4") {
		t.Errorf("catch-all notes = %v, want the rest listed", steps[1].Notes)
	}
	if got := getEntity(t, st, "T-1").Type; got != "taak" {
		t.Errorf("dry run wrote: T-1 type = %s", got)
	}
}

func TestRenameSplit_RefusesEdgeTheNewTypeMayNotHave(t *testing.T) {
	st := seedSplit(t)
	// T-2 carries evidence but is selected for projecten.taak.
	steps := `  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}}
  - rename_entity_type: {from: taak, to: isms.taak}
`
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(), steps))
	r := newTestRunner(t, Deps{Store: st, Meta: splitTo()})

	_, err := r.Run(t.Context(), []*File{f}, true)
	if err == nil || !strings.Contains(err.Error(), "T-2 -bewijs-> P-1") {
		t.Fatalf("err = %v, want the offending edge named", err)
	}
	if got := getEntity(t, st, "T-2").Type; got != "taak" {
		t.Errorf("T-2 type = %s, want taak: nothing of the step may be written", got)
	}
}

func TestRenameSplit_CatchAllIsCheckedToo(t *testing.T) {
	st := seedSplit(t)
	to := splitTo()
	// Now evidence is allowed only on projecten.taak, and T-2 is left for
	// the unfiltered rest, which goes to isms.taak.
	to.Relations["bewijs"] = metamodel.RelationDef{From: []string{"projecten.taak"}, To: []string{"person"}}
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), to, splitSteps))
	r := newTestRunner(t, Deps{Store: st, Meta: to})

	_, err := r.Run(t.Context(), []*File{f}, true)
	if err == nil || !strings.Contains(err.Error(), "not allowed as source") {
		t.Fatalf("err = %v, want the catch-all refused", err)
	}
	if got := getEntity(t, st, "T-4").Type; got != "taak" {
		t.Errorf("T-4 type = %s, want taak", got)
	}
}

func TestRenameSplit_Selection(t *testing.T) {
	tests := []struct {
		name  string
		step  string
		want  []string // ids that move
		notes []string
	}{
		{"scalar value", `{from: taak, to: projecten.taak, where: {soort: audit}}`, []string{"T-2"}, nil},
		{"integer", `{from: taak, to: projecten.taak, where: {prio: 1}}`, []string{"T-1", "T-3"}, nil},
		{"boolean", `{from: taak, to: projecten.taak, where: {klaar: "true"}}`, []string{"T-1"}, nil},
		{"and across properties", `{from: taak, to: projecten.taak, where: {soort: [ontwikkeling, onderhoud], prio: 1}}`,
			[]string{"T-1", "T-3"}, nil},
		{"absent property never matches", `{from: taak, to: projecten.taak, where: {klaar: "false"}}`,
			nil, []string{"matched no entities"}},
		{"ids", `{from: taak, to: projecten.taak, ids: [T-4, T-9]}`, []string{"T-4"},
			[]string{`T-9: not of type "taak" (already renamed or absent)`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := seedSplit(t)
			// Edges are not the point here; drop them.
			for _, k := range []entity.RelationKey{
				{From: "T-2", Type: "bewijs", To: "P-1"},
			} {
				if err := st.DeleteRelation(t.Context(), k); err != nil {
					t.Fatal(err)
				}
			}
			f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(),
				"  - rename_entity_type: "+tc.step+"\n  - rename_entity_type: {from: taak, to: isms.taak}\n"))
			x := &Exec{Store: st, Apply: true}
			res, err := f.Steps[0].Run(t.Context(), x)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			var moved []string
			for _, id := range []string{"T-1", "T-2", "T-3", "T-4"} {
				if getEntity(t, st, id).Type == "projecten.taak" {
					moved = append(moved, id)
				}
			}
			if !slices.Equal(moved, tc.want) {
				t.Errorf("moved = %v, want %v", moved, tc.want)
			}
			for _, n := range tc.notes {
				if !slices.Contains(res.Notes, n) {
					t.Errorf("notes = %v, want %q", res.Notes, n)
				}
			}
		})
	}
}

func TestRenameSplit_FacesMustAgree(t *testing.T) {
	st := seedSplit(t)
	if err := st.CreateEntity(t.Context(), &entity.Entity{
		ID: "T-1", Type: "taak", Face: entity.Face("nl"),
		Properties: map[string]any{"title": "a", "soort": "audit"},
	}); err != nil {
		t.Fatal(err)
	}
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(), splitSteps))
	_, err := f.Steps[0].Run(t.Context(), &Exec{Store: st, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "T-1") || !strings.Contains(err.Error(), "ids:") {
		t.Fatalf("err = %v, want the mixed id named with the ids: remedy", err)
	}
	if got := getEntity(t, st, "T-3").Type; got != "taak" {
		t.Errorf("T-3 type = %s, want taak: nothing may be written", got)
	}
}

func TestRenameSplit_ParseErrors(t *testing.T) {
	keep := splitFrom()
	keep.Entities["projecten.taak"] = keep.Entities["taak"]
	tests := []struct {
		name  string
		to    *metamodel.Metamodel
		steps string
		want  string
	}{
		{"unknown property", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {kind: x}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"not in the from-schema"},
		{"undeclared enum value", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: ontwikeling}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"is not a value of taak.soort"},
		{"list property", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {tags: x}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"list-typed"},
		{"unsupported type", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {due: 2026-01-01}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"only string, enum, boolean and integer"},
		{"bad integer", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {prio: hoog}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"must be one"},
		{"filter-string form", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: [\"soort=audit\"]}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"filter-string list form"},
		{"where and ids", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}, ids: [T-1]}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"cannot be combined"},
		{"empty ids", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, ids: []}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"at least one id"},
		{"empty where", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {}}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"at least one property"},
		{"null where", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: }\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"where: is empty"},
		{"null ids", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, ids: ~}\n  - rename_entity_type: {from: taak, to: isms.taak}\n",
			"ids: is empty"},
		{"filtered after unfiltered", splitTo(),
			"  - rename_entity_type: {from: taak, to: isms.taak}\n  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}}\n",
			"would match nothing"},
		{"no catch-all for a removed type", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}}\n",
			"orphaned"},
		{"drop is not a catch-all", splitTo(),
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}}\n  - drop_entities: {type: taak}\n",
			"orphaned"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile(testName("split"), mustFileYAML(t, splitFrom(), tc.to, tc.steps))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}

	t.Run("partial rename of a kept type needs no catch-all", func(t *testing.T) {
		mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), keep,
			"  - rename_entity_type: {from: taak, to: projecten.taak, where: {soort: audit}}\n"))
	})
}

func TestRenameSplit_RenamesLeavesSplitTypeOut(t *testing.T) {
	f := mustParse(t, testName("split"), mustFileYAML(t, splitFrom(), splitTo(), splitSteps))
	if got := f.Renames(); len(got) != 0 {
		t.Errorf("Renames() = %v, want none for a split type", got)
	}
}

func TestRenameEntityType_UnfilteredIsNotSplit(t *testing.T) {
	to := splitFrom()
	to.Entities["job"] = to.Entities["taak"]
	delete(to.Entities, "taak")
	to.Relations["bewijs"] = metamodel.RelationDef{From: []string{"job"}, To: []string{"person"}}
	to.Relations["ziet"] = metamodel.RelationDef{From: []string{"person"}, To: []string{"job"}}
	f := mustParse(t, testName("plain"), mustFileYAML(t, splitFrom(), to,
		"  - rename_entity_type: {from: taak, to: job}\n"))
	if s := f.Steps[0].(*renameEntityTypeStep); s.split {
		t.Error("an unfiltered rename with no filtered sibling took the split path")
	}
	if got := f.Renames(); len(got) != 1 {
		t.Errorf("Renames() = %v, want the plain rename", got)
	}
}
