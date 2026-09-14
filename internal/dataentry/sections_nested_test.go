package dataentry

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// nestedFixture builds a viewResult shaped like a two-step traverse:
// `parents` collected from the entry, `children` collected from the parents,
// with the parent→child edges recorded in Parents as applyViewTraverse would.
//
// Both collections hold entities that already passed the row gate, because
// that is the state a section builder always sees (executeView filters once,
// before any builder runs). A child the caller may not read is modeled by
// omitting it from the collection while LEAVING its id in the edge map — which
// is exactly what the gate does upstream.
func nestedFixture(edges map[string][]string, visibleChildren ...string) *viewResult {
	visible := map[string]bool{}
	for _, id := range visibleChildren {
		visible[id] = true
	}

	var parents, children []*entity.Entity
	seenParent := map[string]bool{}
	for parentID, childIDs := range edges {
		if !seenParent[parentID] {
			parents = append(parents, &entity.Entity{
				ID: parentID, Type: "ticket",
				Properties: map[string]any{"title": parentID, "status": "open"},
			})
			seenParent[parentID] = true
		}
		for _, childID := range childIDs {
			if visible[childID] {
				children = append(children, &entity.Entity{
					ID: childID, Type: "ticket",
					Properties: map[string]any{"title": childID, "status": "closed"},
				})
			}
		}
	}

	return &viewResult{
		Collections: map[string][]*entity.Entity{"parents": parents, "children": children},
		Parents:     map[string]map[string][]string{"children": edges},
	}
}

func nestedSection() ViewSection {
	return ViewSection{
		Heading: "Children", Source: "parents", Display: dataentryconfig.DisplayNested, Children: "children",
		// Both levels are `ticket` here, with DIFFERENT columns — the case a
		// single type-keyed map could not express.
		ParentColumns: map[string][]ListColumn{"ticket": {{Property: "status"}}},
		ChildColumns:  map[string][]ListColumn{"ticket": {{Property: "title"}, {Property: "status"}}},
	}
}

// buildNestedFor runs buildSections and returns the single nested section.
func buildNestedFor(t *testing.T, app *App, sec ViewSection, result *viewResult) SectionData {
	t.Helper()
	out := app.views.buildSections(context.Background(), []ViewSection{sec}, result)
	if len(out) != 1 {
		t.Fatalf("buildSections: got %d sections, want 1", len(out))
	}
	return out[0]
}

// treeIDs maps each parent id to its emitted child ids, for order-independent
// assertions about attribution.
func treeIDs(sd SectionData) map[string][]string {
	got := map[string][]string{}
	for _, node := range sd.Tree {
		ids := []string{}
		for _, child := range node.Children {
			ids = append(ids, child.ID)
		}
		got[node.ID] = ids
	}
	return got
}

// TestNestedSection_AttributesChildrenToTheirOwnParent is the core contract:
// Collections alone cannot say which child belongs to which parent, so the
// section reads viewResult.Parents. A bug there would most likely show up as
// every parent listing every child, which this catches.
func TestNestedSection_AttributesChildrenToTheirOwnParent(t *testing.T) {
	app := testViewApp()
	result := nestedFixture(
		map[string][]string{
			"TKT-P1": {"TKT-C1", "TKT-C2"},
			"TKT-P2": {"TKT-C3"},
		},
		"TKT-C1", "TKT-C2", "TKT-C3",
	)

	sd := buildNestedFor(t, app, nestedSection(), result)

	if len(sd.Tree) != 2 {
		t.Fatalf("got %d parent rows, want 2", len(sd.Tree))
	}
	got := treeIDs(sd)
	want := map[string][]string{
		"TKT-P1": {"TKT-C1", "TKT-C2"},
		"TKT-P2": {"TKT-C3"},
	}
	for parent, wantIDs := range want {
		gotIDs := got[parent]
		if len(gotIDs) != len(wantIDs) {
			t.Errorf("%s: got children %v, want %v", parent, gotIDs, wantIDs)
			continue
		}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Errorf("%s: got children %v, want %v", parent, gotIDs, wantIDs)
				break
			}
		}
	}
	if sd.Truncated {
		t.Error("Truncated set on a tree that fit the budget")
	}
}

// TestNestedSection_HiddenChildIsAbsent pins the row gate. The child's id is
// still in the edge map (the walk ran on raw store rows, before gating), so
// the ONLY thing keeping it out of the response is the lookup against the
// filtered collection. That is the property a future rollup would depend on.
func TestNestedSection_HiddenChildIsAbsent(t *testing.T) {
	app := testViewApp()
	// TKT-C2 is in the edges but NOT in the visible collection.
	result := nestedFixture(
		map[string][]string{"TKT-P1": {"TKT-C1", "TKT-C2"}},
		"TKT-C1",
	)

	sd := buildNestedFor(t, app, nestedSection(), result)

	if len(sd.Tree) != 1 {
		t.Fatalf("got %d parent rows, want 1", len(sd.Tree))
	}
	node := sd.Tree[0]
	if len(node.Children) != 1 || node.Children[0].ID != "TKT-C1" {
		t.Fatalf("got children %v, want only TKT-C1", treeIDs(sd)["TKT-P1"])
	}
	// ChildCount counts only readable children, so it is safe to render.
	if node.ChildCount != 1 {
		t.Errorf("ChildCount = %d, want 1 (hidden children must not be counted)", node.ChildCount)
	}
	if node.HasMoreChildren {
		t.Error("HasMoreChildren set by a HIDDEN child: that leaks its existence")
	}
}

// TestNestedSection_CappedParentIsDistinguishableFromChildless covers the
// reason HasMoreChildren exists at all: Children is omitempty, so without the
// flag a truncated parent and a genuinely childless one are byte-identical.
func TestNestedSection_CappedParentIsDistinguishableFromChildless(t *testing.T) {
	app := testViewApp()

	manyIDs := make([]string, 0, nestedChildPreview+5)
	for i := range nestedChildPreview + 5 {
		manyIDs = append(manyIDs, fmt.Sprintf("TKT-C%03d", i))
	}
	result := nestedFixture(
		map[string][]string{"TKT-BIG": manyIDs, "TKT-EMPTY": nil},
		manyIDs...,
	)

	sd := buildNestedFor(t, app, nestedSection(), result)

	var big, empty *SectionTreeNode
	for i := range sd.Tree {
		switch sd.Tree[i].ID {
		case "TKT-BIG":
			big = &sd.Tree[i]
		case "TKT-EMPTY":
			empty = &sd.Tree[i]
		}
	}
	if big == nil || empty == nil {
		t.Fatalf("expected both parents in the tree, got %v", treeIDs(sd))
	}

	if len(big.Children) != nestedChildPreview {
		t.Errorf("capped parent emitted %d children, want %d", len(big.Children), nestedChildPreview)
	}
	if !big.HasMoreChildren {
		t.Error("capped parent must report HasMoreChildren")
	}
	if big.ChildCount != len(manyIDs) {
		t.Errorf("ChildCount = %d, want the true total %d", big.ChildCount, len(manyIDs))
	}
	if empty.HasMoreChildren {
		t.Error("childless parent must NOT report HasMoreChildren")
	}
	if len(empty.Children) != 0 {
		t.Errorf("childless parent emitted %d children", len(empty.Children))
	}
	if !sd.Truncated {
		t.Error("Truncated must be set when a visible child was withheld")
	}
}

// TestNestedSection_CarriesNoContent pins that a nested row ships no markdown
// body. `include_content` is a list-path parameter and does not exist on the
// views path, so content is opt-in by display mode; a nested tree of many rows
// shipping every body is the cost that rule exists to avoid.
func TestNestedSection_CarriesNoContent(t *testing.T) {
	app := testViewApp()
	result := nestedFixture(map[string][]string{"TKT-P1": {"TKT-C1"}}, "TKT-C1")
	for _, list := range result.Collections {
		for _, e := range list {
			e.Content = "# body that must not ship"
		}
	}

	sd := buildNestedFor(t, app, nestedSection(), result)

	for _, node := range sd.Tree {
		if node.Content != "" || node.HasContent {
			t.Errorf("parent %s carries content", node.ID)
		}
		for _, child := range node.Children {
			if child.Content != "" || child.HasContent {
				t.Errorf("child %s carries content", child.ID)
			}
		}
	}
}

// TestNestedSection_EmptyParentsIsEmptySection checks the empty case reaches
// the SPA's generic empty state rather than an empty tree that renders as a
// bare heading.
func TestNestedSection_EmptyParentsIsEmptySection(t *testing.T) {
	app := testViewApp()
	result := &viewResult{Collections: map[string][]*entity.Entity{"parents": {}, "children": {}}}

	sd := buildNestedFor(t, app, nestedSection(), result)

	if !sd.IsEmpty {
		t.Error("section with no parents must set IsEmpty")
	}
	if len(sd.Tree) != 0 {
		t.Errorf("got %d parent rows, want 0", len(sd.Tree))
	}
}

// TestNestedSection_LevelsCarryTheirOwnColumns pins the per-level, per-type
// column split: a parent and a child of the SAME entity type render different
// columns, which is the case a single type-keyed map could not express.
func TestNestedSection_LevelsCarryTheirOwnColumns(t *testing.T) {
	app := testViewApp()
	result := nestedFixture(map[string][]string{"TKT-P1": {"TKT-C1"}}, "TKT-C1")

	sd := buildNestedFor(t, app, nestedSection(), result)

	node := sd.Tree[0]
	// The parent shows its own column set, the child a different one, even
	// though both are `ticket`.
	if len(node.Columns) != 1 || node.Columns[0].Property != "status" {
		t.Fatalf("parent columns: %+v", node.Columns)
	}
	if len(node.Row.Cells) != 1 || node.Row.Cells[0].Values[0] != "open" {
		t.Fatalf("parent cells: %+v", node.Row.Cells)
	}
	child := node.Children[0]
	if len(child.Columns) != 2 {
		t.Fatalf("child columns: %+v", child.Columns)
	}
	if got := child.Row.Cells[0].Values[0]; got != "TKT-C1" {
		t.Errorf("child title cell = %q, want %q", got, "TKT-C1")
	}
	if got := child.Row.Cells[1].Values[0]; got != "closed" {
		t.Errorf("child status cell = %q, want %q", got, "closed")
	}
}

// TestNestedSection_AttributionSurvivesFixpoint drives the REAL traversal
// rather than a hand-built fixture, because the defect it guards is in
// applyViewTraverse, not in the section builder.
//
// executeView runs every traverse rule up to 10 times until the collections
// stop growing. Collections tolerate that by deduping on merge; the edge map
// has to do the same, or each pass appends the same child again and every row
// renders duplicated. A single-pass view would never show it, which is why
// this uses a two-step traverse over a chain (TKT-001 → TKT-002 → TKT-003)
// that keeps the fixpoint iterating.
func TestNestedSection_AttributionSurvivesFixpoint(t *testing.T) {
	app := testViewApp()
	view := ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "depends_on", CollectAs: "mid"},
			{From: "mid", Follow: "depends_on", CollectAs: "leaf"},
		},
	}

	result, err := app.views.executeView(context.Background(), view, "TKT-001", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	// TKT-002 is the only mid; its single depends_on edge points at TKT-003.
	// Recorded once no matter how many passes the fixpoint took.
	edges := result.Parents["leaf"]
	if got := len(edges["TKT-002"]); got != 1 {
		t.Errorf("TKT-002 has %d recorded children %v, want exactly 1 (duplicated across fixpoint passes?)",
			got, edges["TKT-002"])
	}

	sec := ViewSection{
		Heading: "Leaves", Source: "mid", Display: dataentryconfig.DisplayNested, Children: "leaf",
	}
	sd := buildNestedFor(t, app, sec, result)

	for _, node := range sd.Tree {
		seen := map[string]int{}
		for _, child := range node.Children {
			seen[child.ID]++
		}
		for id, n := range seen {
			if n != 1 {
				t.Errorf("parent %s renders child %s %d times, want once", node.ID, id, n)
			}
		}
	}
}

// TestNestedSection_RecursiveWalkRecordsNoAttribution documents why config
// validation refuses `recursive: true` under a nested section: the breadth-first
// walk reports ids level by level without retaining which node each came from,
// so the tree would render flat with no children at all.
//
// Pinned as a test so that if someone later teaches the recursive walk to
// retain edges, this fails and points at the validation rule that can then be
// relaxed — rather than the rule silently outliving its reason.
func TestNestedSection_RecursiveWalkRecordsNoAttribution(t *testing.T) {
	app := testViewApp()
	view := ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "depends_on", CollectAs: "all", Recursive: true},
		},
	}

	result, err := app.views.executeView(context.Background(), view, "TKT-001", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}
	if len(result.Collections["all"]) == 0 {
		t.Fatal("recursive walk collected nothing; fixture no longer exercises the path")
	}
	if edges := result.Parents["all"]; len(edges) != 0 {
		t.Errorf("recursive walk recorded attribution %v; if this now works, relax "+
			"validateNestedSection's recursive refusal", edges)
	}
}

// TestNestedSection_SectionBudgetDropsParentsAndFlags covers the case the
// per-parent cap does not: when the SECTION budget runs out, later parents are
// dropped entirely, so there is no row left to carry HasMoreChildren.
//
// Only the section-level Truncated flag can report it — which is why the SPA
// has to render that flag (RR-O1VNNN: it initially did not, and rows vanished
// silently).
func TestNestedSection_SectionBudgetDropsParentsAndFlags(t *testing.T) {
	app := testViewApp()

	// Sized to overrun nestedNodeBudget: each parent contributes itself plus
	// at most nestedChildPreview children.
	perParent := nestedChildPreview
	parentCount := (nestedNodeBudget / (perParent + 1)) + 20

	edges := map[string][]string{}
	var visible []string
	for p := range parentCount {
		pid := fmt.Sprintf("TKT-P%04d", p)
		for c := range perParent {
			cid := fmt.Sprintf("TKT-C%04d-%04d", p, c)
			edges[pid] = append(edges[pid], cid)
			visible = append(visible, cid)
		}
	}

	sd := buildNestedFor(t, app, nestedSection(), nestedFixture(edges, visible...))

	emitted := 0
	for _, n := range sd.Tree {
		emitted += 1 + len(n.Children)
	}
	if emitted > nestedNodeBudget {
		t.Errorf("emitted %d nodes, budget is %d", emitted, nestedNodeBudget)
	}
	if len(sd.Tree) >= parentCount {
		t.Errorf("emitted all %d parents; expected the budget to drop some", len(sd.Tree))
	}
	if !sd.Truncated {
		t.Error("dropping whole parents must set Truncated: no row survives to carry hasMoreChildren")
	}
}
