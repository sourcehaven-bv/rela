package views

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TestIssue1_CollectAsTypeFiltering tests that collect_as filters by entity type
func TestIssue1_CollectAsTypeFiltering(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"bouwblok": {
				Label:      "Bouwblok",
				IDPatterns: []string{"BB-"},
			},
			"function": {
				Label:      "Function",
				IDPatterns: []string{"FUNC-"},
			},
			"usecase": {
				Label:      "Use Case",
				IDPatterns: []string{"UC-"},
			},
			"scenario": {
				Label:      "Scenario",
				IDPatterns: []string{"SC-"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"partOfBouwblok": {
				Label: "part of bouwblok",
				From:  []string{"function", "usecase", "scenario"},
				To:    []string{"bouwblok"},
			},
		},
	}

	g := graph.New()

	// Create a bouwblok
	bb := &model.Entity{ID: "BB-001", Type: "bouwblok"}
	g.AddNode(bb)

	// Create mixed entity types
	func1 := &model.Entity{ID: "FUNC-001", Type: "function"}
	func2 := &model.Entity{ID: "FUNC-002", Type: "function"}
	uc1 := &model.Entity{ID: "UC-001", Type: "usecase"}
	uc2 := &model.Entity{ID: "UC-002", Type: "usecase"}
	sc1 := &model.Entity{ID: "SC-001", Type: "scenario"}

	g.AddNode(func1)
	g.AddNode(func2)
	g.AddNode(uc1)
	g.AddNode(uc2)
	g.AddNode(sc1)

	// Link them all to the bouwblok
	g.AddEdge(&model.Relation{From: "FUNC-001", Type: "partOfBouwblok", To: "BB-001"})
	g.AddEdge(&model.Relation{From: "FUNC-002", Type: "partOfBouwblok", To: "BB-001"})
	g.AddEdge(&model.Relation{From: "UC-001", Type: "partOfBouwblok", To: "BB-001"})
	g.AddEdge(&model.Relation{From: "UC-002", Type: "partOfBouwblok", To: "BB-001"})
	g.AddEdge(&model.Relation{From: "SC-001", Type: "partOfBouwblok", To: "BB-001"})

	view := ViewDef{
		Entry: EntryDef{Type: "bouwblok"},
		Traverse: []TraverseRule{
			{
				From:           "entry",
				FollowIncoming: "partOfBouwblok",
				CollectAs:      []interface{}{"functions", "usecases", "scenarios"},
			},
		},
	}

	engine := NewEngine(g, meta)
	result, err := engine.Execute(view, "BB-001")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify type filtering
	functions := result.Collections["functions"]
	usecases := result.Collections["usecases"]
	scenarios := result.Collections["scenarios"]

	if len(functions) != 2 {
		t.Errorf("functions count = %d, want 2", len(functions))
		for _, e := range functions {
			t.Logf("  function: %s (type=%s)", e.ID, e.Type)
		}
	}

	if len(usecases) != 2 {
		t.Errorf("usecases count = %d, want 2", len(usecases))
		for _, e := range usecases {
			t.Logf("  usecase: %s (type=%s)", e.ID, e.Type)
		}
	}

	if len(scenarios) != 1 {
		t.Errorf("scenarios count = %d, want 1", len(scenarios))
		for _, e := range scenarios {
			t.Logf("  scenario: %s (type=%s)", e.ID, e.Type)
		}
	}

	// Verify each collection only contains the correct type
	for _, fn := range functions {
		if fn.Type != "function" {
			t.Errorf("functions collection contains %s with type %s", fn.ID, fn.Type)
		}
	}

	for _, uc := range usecases {
		if uc.Type != "usecase" {
			t.Errorf("usecases collection contains %s with type %s", uc.ID, uc.Type)
		}
	}

	for _, sc := range scenarios {
		if sc.Type != "scenario" {
			t.Errorf("scenarios collection contains %s with type %s", sc.ID, sc.Type)
		}
	}
}

// TestIssue2_FilterExpand tests that expand mode adds entities from the graph
func TestIssue2_FilterExpand(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
			},
			"document": {
				Label:      "Document",
				IDPatterns: []string{"DOC-"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "addresses",
				From:  []string{"document"},
				To:    []string{"requirement"},
			},
		},
	}

	g := graph.New()

	// Create a document
	doc := &model.Entity{ID: "DOC-001", Type: "document"}
	g.AddNode(doc)

	// Create requirements with different prefixes
	req1 := &model.Entity{ID: "REQ-001", Type: "requirement"} // Not connected
	req2 := &model.Entity{ID: "LRZA-001", Type: "requirement"} // Connected
	req3 := &model.Entity{ID: "LRZA-002", Type: "requirement"} // Not connected
	req4 := &model.Entity{ID: "GF-001", Type: "requirement"}   // Not connected
	req5 := &model.Entity{ID: "GF-002", Type: "requirement"}   // Not connected

	g.AddNode(req1)
	g.AddNode(req2)
	g.AddNode(req3)
	g.AddNode(req4)
	g.AddNode(req5)

	// Only connect one requirement
	g.AddEdge(&model.Relation{From: "DOC-001", Type: "addresses", To: "LRZA-001"})

	view := ViewDef{
		Entry: EntryDef{Type: "document"},
		Traverse: []TraverseRule{
			{
				From:      "entry",
				Follow:    "addresses",
				CollectAs: "requirements",
			},
		},
		Filters: map[string]Filter{
			"requirements": {
				Expand:   true,
				IDPrefix: []string{"LRZA-", "GF-"},
			},
		},
	}

	engine := NewEngine(g, meta)
	result, err := engine.Execute(view, "DOC-001")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	requirements := result.Collections["requirements"]

	// Should have 4 requirements: LRZA-001 (traversed) + LRZA-002, GF-001, GF-002 (expanded)
	if len(requirements) != 4 {
		t.Errorf("requirements count = %d, want 4", len(requirements))
		for _, req := range requirements {
			t.Logf("  requirement: %s", req.ID)
		}
	}

	// Verify all are the correct type and have the right prefix
	foundLRZA1 := false
	foundLRZA2 := false
	foundGF1 := false
	foundGF2 := false

	for _, req := range requirements {
		if req.Type != "requirement" {
			t.Errorf("requirements collection contains %s with type %s", req.ID, req.Type)
		}

		switch req.ID {
		case "LRZA-001":
			foundLRZA1 = true
		case "LRZA-002":
			foundLRZA2 = true
		case "GF-001":
			foundGF1 = true
		case "GF-002":
			foundGF2 = true
		case "REQ-001":
			t.Errorf("Should not include REQ-001 (wrong prefix)")
		}
	}

	if !foundLRZA1 || !foundLRZA2 || !foundGF1 || !foundGF2 {
		t.Errorf("Missing expected requirements: LRZA1=%v, LRZA2=%v, GF1=%v, GF2=%v",
			foundLRZA1, foundLRZA2, foundGF1, foundGF2)
	}
}

// TestIssue3_MultiPassTraversal tests that multi-pass finds indirectly connected entities
func TestIssue3_MultiPassTraversal(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"persona": {
				Label:      "Persona",
				IDPatterns: []string{"PER-"},
			},
			"function": {
				Label:      "Function",
				IDPatterns: []string{"FUNC-"},
			},
			"component": {
				Label:      "Component",
				IDPatterns: []string{"COMP-"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"usesFunction": {
				Label: "uses function",
				From:  []string{"persona"},
				To:    []string{"function"},
			},
			"realizes": {
				Label: "realizes",
				From:  []string{"component"},
				To:    []string{"function"},
			},
		},
	}

	g := graph.New()

	// Create a chain: persona -> function -> component
	persona := &model.Entity{ID: "PER-001", Type: "persona"}
	fn := &model.Entity{ID: "FUNC-001", Type: "function"}
	comp := &model.Entity{ID: "COMP-001", Type: "component"}

	g.AddNode(persona)
	g.AddNode(fn)
	g.AddNode(comp)

	g.AddEdge(&model.Relation{From: "PER-001", Type: "usesFunction", To: "FUNC-001"})
	g.AddEdge(&model.Relation{From: "COMP-001", Type: "realizes", To: "FUNC-001"})

	view := ViewDef{
		Entry: EntryDef{Type: "persona"},
		Traverse: []TraverseRule{
			// First: get functions from persona
			{
				From:      "entry",
				Follow:    "usesFunction",
				CollectAs: "functions",
			},
			// Second: get components from functions (depends on first rule)
			{
				From:           "functions",
				FollowIncoming: "realizes",
				CollectAs:      "components",
			},
		},
	}

	engine := NewEngine(g, meta)
	result, err := engine.Execute(view, "PER-001")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	functions := result.Collections["functions"]
	components := result.Collections["components"]

	if len(functions) != 1 {
		t.Errorf("functions count = %d, want 1", len(functions))
	}

	if len(components) != 1 {
		t.Errorf("components count = %d, want 1", len(components))
		t.Logf("functions found: %d", len(functions))
		for _, f := range functions {
			t.Logf("  function: %s", f.ID)
		}
	}

	if len(components) > 0 && components[0].ID != "COMP-001" {
		t.Errorf("component ID = %s, want COMP-001", components[0].ID)
	}
}
