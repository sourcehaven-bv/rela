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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"dependencies"}},
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
				{From: StringOrSlice{"entry"}, FollowIncoming: "depends_on", CollectAs: StringOrSlice{"dependents"}},
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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"all_deps"}, Recursive: true},
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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"limited_deps"}, Recursive: true, MaxDepth: 1},
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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"deps"}},
				{From: StringOrSlice{"*"}, Follow: "depends_on", CollectAs: StringOrSlice{"transitive_deps"}},
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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"deps"}},
				{From: StringOrSlice{"entry"}, Follow: "belongs_to", CollectAs: StringOrSlice{"components"}},
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
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"collected"}},
				{From: StringOrSlice{"entry"}, Follow: "depends_on", CollectAs: StringOrSlice{"collected"}}, // same rule again
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

// testViewAppWithMixedTypes creates an App with mixed entity types for where clause tests.
//
//	BOUWBLOK-001
//	  <--partOfBouwblok-- FUNC-001 (function)
//	  <--partOfBouwblok-- FUNC-002 (function)
//	  <--partOfBouwblok-- UC-001 (usecase)
//	  <--partOfBouwblok-- SCEN-001 (scenario)
func testViewAppWithMixedTypes() *App {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type": {Values: []string{"draft", "active", "done"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"bouwblok": {
				Label: "Bouwblok",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"function": {
				Label: "Function",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status_type"},
				},
			},
			"usecase": {
				Label: "Use Case",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status_type"},
				},
			},
			"scenario": {
				Label: "Scenario",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"partOfBouwblok": {
				From: []string{"function", "usecase", "scenario"},
				To:   []string{"bouwblok"},
			},
		},
	}

	cfg := &Config{
		App: AppConfig{Name: "Test"},
	}

	g := graph.New()

	// Bouwblok
	bb := model.NewEntity("BOUWBLOK-001", "bouwblok")
	bb.SetString("title", "Main Bouwblok")
	g.AddNode(bb)

	// Functions
	f1 := model.NewEntity("FUNC-001", "function")
	f1.SetString("title", "Function One")
	f1.SetString("status", "active")
	g.AddNode(f1)

	f2 := model.NewEntity("FUNC-002", "function")
	f2.SetString("title", "Function Two")
	f2.SetString("status", "draft")
	g.AddNode(f2)

	// Use case
	uc := model.NewEntity("UC-001", "usecase")
	uc.SetString("title", "Use Case One")
	uc.SetString("status", "active")
	g.AddNode(uc)

	// Scenario
	sc := model.NewEntity("SCEN-001", "scenario")
	sc.SetString("title", "Scenario One")
	g.AddNode(sc)

	// Relations: all point to bouwblok
	g.AddEdge(model.NewRelation("FUNC-001", "partOfBouwblok", "BOUWBLOK-001"))
	g.AddEdge(model.NewRelation("FUNC-002", "partOfBouwblok", "BOUWBLOK-001"))
	g.AddEdge(model.NewRelation("UC-001", "partOfBouwblok", "BOUWBLOK-001"))
	g.AddEdge(model.NewRelation("SCEN-001", "partOfBouwblok", "BOUWBLOK-001"))

	styleMap, styledTypes := buildStyleMap(cfg, meta)
	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}
}

func TestExecuteViewWithWhere(t *testing.T) {
	app := testViewAppWithMixedTypes()

	t.Run("filter by type - functions only", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"functions"},
					Where:          "type = function",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		functions := result.Collections["functions"]
		if len(functions) != 2 {
			ids := collectIDs(functions)
			t.Errorf("expected 2 functions, got %v", ids)
		}
		for _, f := range functions {
			if f.Type != "function" {
				t.Errorf("expected type function, got %s", f.Type)
			}
		}
	})

	t.Run("filter by type - usecases only", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"usecases"},
					Where:          "type = usecase",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		usecases := result.Collections["usecases"]
		if len(usecases) != 1 || usecases[0].ID != "UC-001" {
			ids := collectIDs(usecases)
			t.Errorf("expected [UC-001], got %v", ids)
		}
	})

	t.Run("filter by type - exclude with !=", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"not_functions"},
					Where:          "type != function",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		notFunctions := result.Collections["not_functions"]
		if len(notFunctions) != 2 {
			ids := collectIDs(notFunctions)
			t.Errorf("expected 2 non-functions (UC-001, SCEN-001), got %v", ids)
		}
		for _, e := range notFunctions {
			if e.Type == "function" {
				t.Errorf("should not contain functions, but found %s", e.ID)
			}
		}
	})

	t.Run("filter by property - status", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"active_items"},
					Where:          "status = active",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		activeItems := result.Collections["active_items"]
		// FUNC-001 (active) and UC-001 (active) should match
		// FUNC-002 (draft) and SCEN-001 (no status) should not
		if len(activeItems) != 2 {
			ids := collectIDs(activeItems)
			t.Errorf("expected 2 active items, got %v", ids)
		}
	})

	t.Run("multiple traverse rules with different type filters", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"functions"},
					Where:          "type = function",
				},
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"usecases"},
					Where:          "type = usecase",
				},
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"scenarios"},
					Where:          "type = scenario",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Collections["functions"]) != 2 {
			t.Errorf("expected 2 functions, got %d", len(result.Collections["functions"]))
		}
		if len(result.Collections["usecases"]) != 1 {
			t.Errorf("expected 1 usecase, got %d", len(result.Collections["usecases"]))
		}
		if len(result.Collections["scenarios"]) != 1 {
			t.Errorf("expected 1 scenario, got %d", len(result.Collections["scenarios"]))
		}
	})

	t.Run("invalid where expression - continues with unfiltered", func(t *testing.T) {
		view := ViewConfig{
			Entry: ViewEntry{Type: "bouwblok"},
			Traverse: []ViewTraverse{
				{
					From:           StringOrSlice{"entry"},
					FollowIncoming: "partOfBouwblok",
					CollectAs:      StringOrSlice{"all"},
					Where:          "invalid expression without operator",
				},
			},
		}
		result, err := app.executeView(view, "BOUWBLOK-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// On invalid expression, should continue with unfiltered results
		all := result.Collections["all"]
		if len(all) != 4 {
			ids := collectIDs(all)
			t.Errorf("expected 4 unfiltered entities, got %v", ids)
		}
	})
}

func TestFilterEntities(t *testing.T) {
	app := testViewAppWithMixedTypes()

	entities := []*model.Entity{
		{ID: "FUNC-001", Type: "function", Properties: map[string]interface{}{"status": "active"}},
		{ID: "FUNC-002", Type: "function", Properties: map[string]interface{}{"status": "draft"}},
		{ID: "UC-001", Type: "usecase", Properties: map[string]interface{}{"status": "active"}},
		{ID: "SCEN-001", Type: "scenario", Properties: map[string]interface{}{}},
	}

	t.Run("filter by type", func(t *testing.T) {
		result, err := app.filterEntities(entities, "type = function")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 functions, got %d", len(result))
		}
	})

	t.Run("filter by type not equal", func(t *testing.T) {
		result, err := app.filterEntities(entities, "type != function")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 non-functions, got %d", len(result))
		}
	})

	t.Run("invalid expression returns error", func(t *testing.T) {
		_, err := app.filterEntities(entities, "no operator here")
		if err == nil {
			t.Error("expected error for invalid expression")
		}
	})
}
