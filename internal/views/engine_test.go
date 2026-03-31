package views

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestEngineExecute(t *testing.T) {
	// Create a simple metamodel
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document": {
				Label:      "Document",
				IDPrefixes: []string{"DOC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"section": {
				Label:      "Section",
				IDPrefixes: []string{"SEC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"contains": {
				Label: "contains",
				From:  []string{"document"},
				To:    []string{"section"},
			},
		},
	}

	// Create a graph with test data
	g := graph.New()

	doc := testutil.EntityFor(meta, "document").
		ID("DOC-001").
		With("title", "Test Document").
		WithContent("Document content").
		Build()
	g.AddNode(doc)

	sec1 := testutil.EntityFor(meta, "section").
		ID("SEC-001").
		With("title", "Section 1").
		WithContent("Section 1 content").
		Build()
	g.AddNode(sec1)

	sec2 := testutil.EntityFor(meta, "section").
		ID("SEC-002").
		With("title", "Section 2").
		WithContent("Section 2 content").
		Build()
	g.AddNode(sec2)

	// Add relations
	g.AddEdge(testutil.NewRelation("DOC-001", "contains", "SEC-001").
		WithContent("Relation content 1").
		Build())
	g.AddEdge(testutil.NewRelation("DOC-001", "contains", "SEC-002").
		WithContent("Relation content 2").
		Build())

	// Create a view definition
	view := ViewDef{
		Entry: EntryDef{
			Type:      "document",
			Parameter: "doc_id",
		},
		Output: OutputDef{
			IncludeContent:        true,
			ResolveRelationTitles: true,
			IncludeEntry:          true,
		},
		Traverse: []TraverseRule{
			{
				From:      "entry",
				Follow:    "contains",
				CollectAs: "sections",
			},
		},
	}

	// Execute the view
	engine := NewEngine(g, meta)
	result, err := engine.Execute(view, "DOC-001")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify results
	if result.Entry == nil {
		t.Fatal("Entry is nil")
	}
	if result.Entry.ID != "DOC-001" {
		t.Errorf("Entry ID = %s, want DOC-001", result.Entry.ID)
	}

	sections, ok := result.Collections["sections"]
	if !ok {
		t.Fatal("sections collection not found")
	}
	if len(sections) != 2 {
		t.Errorf("sections count = %d, want 2", len(sections))
	}

	// Verify content is included
	if result.Entry.Content != "Document content" {
		t.Errorf("Entry content = %q, want %q", result.Entry.Content, "Document content")
	}
}

func TestEngineTraverseRecursive(t *testing.T) {
	// Create a metamodel with recursive relations
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"component": {
				Label:      "Component",
				IDPrefixes: []string{"COMP-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"dependsOn": {
				Label: "depends on",
				From:  []string{"component"},
				To:    []string{"component"},
			},
		},
	}

	// Create a graph with dependencies
	g := graph.New()

	comp1 := testutil.EntityFor(meta, "component").
		ID("COMP-001").
		With("title", "Component 1").
		Build()
	g.AddNode(comp1)

	comp2 := testutil.EntityFor(meta, "component").
		ID("COMP-002").
		With("title", "Component 2").
		Build()
	g.AddNode(comp2)

	comp3 := testutil.EntityFor(meta, "component").
		ID("COMP-003").
		With("title", "Component 3").
		Build()
	g.AddNode(comp3)

	// COMP-001 -> COMP-002 -> COMP-003 (chain)
	g.AddEdge(testutil.NewRelation("COMP-001", "dependsOn", "COMP-002").Build())
	g.AddEdge(testutil.NewRelation("COMP-002", "dependsOn", "COMP-003").Build())

	// Create a view with recursive traversal
	view := ViewDef{
		Entry: EntryDef{
			Type:      "component",
			Parameter: "comp_id",
		},
		Traverse: []TraverseRule{
			{
				From:      "entry",
				Follow:    "dependsOn",
				CollectAs: "dependencies",
				Recursive: true,
				MaxDepth:  3,
			},
		},
	}

	// Execute the view
	engine := NewEngine(g, meta)
	result, err := engine.Execute(view, "COMP-001")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify we got all dependencies transitively
	deps, ok := result.Collections["dependencies"]
	if !ok {
		t.Fatal("dependencies collection not found")
	}

	// Should have COMP-002 and COMP-003
	if len(deps) < 2 {
		t.Errorf("dependencies count = %d, want at least 2", len(deps))
	}
}

func TestViewDefValidation(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document": {
				Label:      "Document",
				IDPrefixes: []string{"DOC-"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"contains": {
				Label: "contains",
				From:  []string{"document"},
				To:    []string{"section"},
			},
		},
	}

	tests := []struct {
		name    string
		view    ViewDef
		wantErr bool
	}{
		{
			name: "valid view",
			view: ViewDef{
				Entry: EntryDef{Type: "document"},
				Traverse: []TraverseRule{
					{From: "entry", Follow: "contains", CollectAs: "sections"},
				},
			},
			wantErr: false,
		},
		{
			name: "unknown entry type",
			view: ViewDef{
				Entry: EntryDef{Type: "unknown"},
			},
			wantErr: true,
		},
		{
			name: "unknown relation type",
			view: ViewDef{
				Entry: EntryDef{Type: "document"},
				Traverse: []TraverseRule{
					{From: "entry", Follow: "unknownRelation", CollectAs: "sections"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing collect_as",
			view: ViewDef{
				Entry: EntryDef{Type: "document"},
				Traverse: []TraverseRule{
					{From: "entry", Follow: "contains"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.view.Validate(meta, "test_view")
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestViewResultEntities(t *testing.T) {
	entry := testutil.NewEntity("DOC-001", "document").Build()
	sec1 := testutil.NewEntity("SEC-001", "section").Build()
	sec2 := testutil.NewEntity("SEC-002", "section").Build()

	tests := []struct {
		name    string
		result  *ViewResult
		wantIDs []string
	}{
		{
			name: "entry only",
			result: &ViewResult{
				Entry:       entry,
				Collections: map[string][]*model.Entity{},
			},
			wantIDs: []string{"DOC-001"},
		},
		{
			name: "entry and collections",
			result: &ViewResult{
				Entry: entry,
				Collections: map[string][]*model.Entity{
					"sections": {sec1, sec2},
				},
			},
			wantIDs: []string{"DOC-001", "SEC-001", "SEC-002"},
		},
		{
			name: "deduplication across collections",
			result: &ViewResult{
				Entry: entry,
				Collections: map[string][]*model.Entity{
					"sections":  {sec1, sec2},
					"all_items": {entry, sec1}, // entry and sec1 duplicated
				},
			},
			wantIDs: []string{"DOC-001", "SEC-001", "SEC-002"},
		},
		{
			name: "nil entry",
			result: &ViewResult{
				Entry: nil,
				Collections: map[string][]*model.Entity{
					"sections": {sec1, sec2},
				},
			},
			wantIDs: []string{"SEC-001", "SEC-002"},
		},
		{
			name: "empty result - nil entry and empty collections",
			result: &ViewResult{
				Entry:       nil,
				Collections: map[string][]*model.Entity{},
			},
			wantIDs: []string{},
		},
		{
			name: "empty result - nil entry and nil collections",
			result: &ViewResult{
				Entry:       nil,
				Collections: nil,
			},
			wantIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotIDs []string
			for e := range tt.result.Entities() {
				gotIDs = append(gotIDs, e.ID)
			}

			if len(gotIDs) != len(tt.wantIDs) {
				t.Errorf("Entities() returned %d items, want %d", len(gotIDs), len(tt.wantIDs))
				return
			}

			// Check all expected IDs are present (order may vary due to map iteration)
			gotSet := make(map[string]bool)
			for _, id := range gotIDs {
				gotSet[id] = true
			}
			for _, wantID := range tt.wantIDs {
				if !gotSet[wantID] {
					t.Errorf("Entities() missing expected ID %s", wantID)
				}
			}
		})
	}
}

func TestViewResultEntityIDs(t *testing.T) {
	entry := testutil.NewEntity("DOC-001", "document").Build()
	sec1 := testutil.NewEntity("SEC-001", "section").Build()

	result := &ViewResult{
		Entry: entry,
		Collections: map[string][]*model.Entity{
			"sections": {sec1},
		},
	}

	ids := result.EntityIDs()

	if len(ids) != 2 {
		t.Errorf("EntityIDs() returned %d IDs, want 2", len(ids))
	}
	if !ids["DOC-001"] {
		t.Error("EntityIDs() missing DOC-001")
	}
	if !ids["SEC-001"] {
		t.Error("EntityIDs() missing SEC-001")
	}
}
