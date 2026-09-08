package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// Recursive traversal is breadth-first since TKT-1U8XYN (one relation query
// per level instead of one per node). The collection order follows: every
// direct neighbor before any second-level one. This pins that order so a
// later change to the walk is a deliberate one, not a silent reorder.
//
//	TKT-001 --blocks--> TKT-002 --blocks--> TKT-004
//	TKT-001 --blocks--> TKT-003
//
// Depth-first would have given [TKT-002, TKT-004, TKT-003].
func TestViewTraversal_RecursiveIsBreadthFirst(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"blocks": {From: []string{"ticket"}, To: []string{"ticket"}},
		},
	}
	g := newFixture()
	for _, id := range []string{"TKT-001", "TKT-002", "TKT-003", "TKT-004"} {
		g.AddNode(testutil.EntityFor(meta, "ticket").ID(id).With("title", id).Build())
	}
	g.AddEdge(testutil.NewRelation("TKT-001", "blocks", "TKT-002").Build())
	g.AddEdge(testutil.NewRelation("TKT-001", "blocks", "TKT-003").Build())
	g.AddEdge(testutil.NewRelation("TKT-002", "blocks", "TKT-004").Build())
	app := newAppFromParts(nil, meta, g)

	view := ViewConfig{
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", Follow: "blocks", CollectAs: "all", Recursive: true}},
	}
	result, err := app.views.executeView(context.Background(), view, "TKT-001", defaultViewWorld())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, 3)
	for _, e := range result.Collections["all"] {
		got = append(got, e.ID)
	}
	want := []string{"TKT-002", "TKT-003", "TKT-004"}
	if len(got) != len(want) {
		t.Fatalf("collection = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collection = %v, want breadth-first %v", got, want)
		}
	}
	var _ *entity.Entity
}
