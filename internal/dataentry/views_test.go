package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// testViewApp creates an App with a graph suitable for view traversal tests.
//
//	TKT-001 --depends_on--> TKT-002 --depends_on--> TKT-003
//	TKT-001 --belongs_to--> CMP-001
func testViewApp() *App {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type": {Values: []string{"open", "closed"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status_type"},
				},
			},
			"component": {
				Label: "Component",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"depends_on": {From: []string{"ticket"}, To: []string{"ticket"}},
			"belongs_to": {From: []string{"ticket"}, To: []string{"component"}},
		},
	}

	cfg := &Config{
		App: AppConfig{Name: "Test"},
	}

	g := graph.New()
	e1 := model.NewEntity("TKT-001", "ticket")
	e1.SetString("title", "First")
	e1.SetString("status", "open")
	g.AddNode(e1)

	e2 := model.NewEntity("TKT-002", "ticket")
	e2.SetString("title", "Second")
	e2.SetString("status", "closed")
	g.AddNode(e2)

	e3 := model.NewEntity("TKT-003", "ticket")
	e3.SetString("title", "Third")
	g.AddNode(e3)

	c1 := model.NewEntity("CMP-001", "component")
	c1.SetString("name", "Frontend")
	g.AddNode(c1)

	g.AddEdge(model.NewRelation("TKT-001", "depends_on", "TKT-002"))
	g.AddEdge(model.NewRelation("TKT-002", "depends_on", "TKT-003"))
	g.AddEdge(model.NewRelation("TKT-001", "belongs_to", "CMP-001"))

	styleMap, styledTypes := buildStyleMap(cfg, meta)
	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}
}

func TestCountViewEntities(t *testing.T) {
	t.Run("empty collections", func(t *testing.T) {
		got := countViewEntities(map[string][]*model.Entity{})
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("counts unique entities", func(t *testing.T) {
		e1 := &model.Entity{ID: "A"}
		e2 := &model.Entity{ID: "B"}
		collections := map[string][]*model.Entity{
			"col1": {e1, e2},
			"col2": {e1}, // duplicate
		}
		got := countViewEntities(collections)
		if got != 2 {
			t.Errorf("expected 2, got %d", got)
		}
	})
}

func TestExecuteView(t *testing.T) {
	app := testViewApp()

	t.Run("basic outgoing traversal", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "dependencies"},
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Entry.ID != "TKT-001" {
			t.Errorf("expected entry TKT-001, got %s", result.Entry.ID)
		}
		deps := result.Collections["dependencies"]
		if len(deps) != 1 || deps[0].ID != "TKT-002" {
			ids := collectIDs(deps)
			t.Errorf("expected [TKT-002], got %v", ids)
		}
	})

	t.Run("incoming traversal", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", FollowIncoming: "depends_on", CollectAs: "dependents"},
			},
		}
		result, err := app.executeView(view, "TKT-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dependents := result.Collections["dependents"]
		if len(dependents) != 1 || dependents[0].ID != "TKT-001" {
			ids := collectIDs(dependents)
			t.Errorf("expected [TKT-001], got %v", ids)
		}
	})

	t.Run("recursive traversal", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "all_deps", Recursive: true},
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		allDeps := result.Collections["all_deps"]
		if len(allDeps) != 2 {
			ids := collectIDs(allDeps)
			t.Errorf("expected 2 recursive dependencies [TKT-002, TKT-003], got %v", ids)
		}
	})

	t.Run("recursive with max depth", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "limited_deps", Recursive: true, MaxDepth: 1},
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		limitedDeps := result.Collections["limited_deps"]
		if len(limitedDeps) != 1 || limitedDeps[0].ID != "TKT-002" {
			ids := collectIDs(limitedDeps)
			t.Errorf("expected [TKT-002] with depth limit, got %v", ids)
		}
	})

	t.Run("wildcard from collects from all", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "deps"},
				{From: "*", Follow: "depends_on", CollectAs: "transitive_deps"},
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// "*" collects from entry (TKT-001) and deps (TKT-002)
		// TKT-001 -> TKT-002, TKT-002 -> TKT-003
		transDeps := result.Collections["transitive_deps"]
		if len(transDeps) != 2 {
			ids := collectIDs(transDeps)
			t.Errorf("expected 2 transitive deps, got %v", ids)
		}
	})

	t.Run("entry not found", func(t *testing.T) {
		view := ViewConfig{Entry: ViewEntry{Type: "ticket"}}
		_, err := app.executeView(view, "NONEXISTENT")
		if err == nil {
			t.Error("expected error for nonexistent entry")
		}
	})

	t.Run("wrong entry type", func(t *testing.T) {
		view := ViewConfig{Entry: ViewEntry{Type: "component"}}
		_, err := app.executeView(view, "TKT-001")
		if err == nil {
			t.Error("expected error for wrong entry type")
		}
	})

	t.Run("entry collection removed from result", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := result.Collections["entry"]; ok {
			t.Error("expected 'entry' collection to be removed from result")
		}
	})

	t.Run("multiple traversals", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "deps"},
				{From: "entry", Follow: "belongs_to", CollectAs: "components"},
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Collections["deps"]) != 1 {
			t.Errorf("expected 1 dep, got %d", len(result.Collections["deps"]))
		}
		if len(result.Collections["components"]) != 1 {
			t.Errorf("expected 1 component, got %d", len(result.Collections["components"]))
		}
		if result.Collections["components"][0].ID != "CMP-001" {
			t.Errorf("expected CMP-001, got %s", result.Collections["components"][0].ID)
		}
	})

	t.Run("deduplication within collection", func(t *testing.T) {
		// Add a second edge TKT-001 --belongs_to--> CMP-001 won't happen,
		// but multiple rules collecting into same collection should deduplicate.
		view := ViewConfig{
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "depends_on", CollectAs: "collected"},
				{From: "entry", Follow: "depends_on", CollectAs: "collected"}, // same rule again
			},
		}
		result, err := app.executeView(view, "TKT-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Collections["collected"]) != 1 {
			t.Errorf("expected 1 (deduplicated), got %d", len(result.Collections["collected"]))
		}
	})
}

func collectIDs(entities []*model.Entity) []string {
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}
