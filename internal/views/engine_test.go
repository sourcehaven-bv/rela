package views

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestEngineExecute(t *testing.T) {
	// Create a simple metamodel
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document": {
				Label:      "Document",
				IDPatterns: []string{"DOC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"section": {
				Label:      "Section",
				IDPatterns: []string{"SEC-"},
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

	doc := &model.Entity{
		ID:   "DOC-001",
		Type: "document",
		Properties: map[string]interface{}{
			"title": "Test Document",
		},
		Content: "Document content",
	}
	g.AddNode(doc)

	sec1 := &model.Entity{
		ID:   "SEC-001",
		Type: "section",
		Properties: map[string]interface{}{
			"title": "Section 1",
		},
		Content: "Section 1 content",
	}
	g.AddNode(sec1)

	sec2 := &model.Entity{
		ID:   "SEC-002",
		Type: "section",
		Properties: map[string]interface{}{
			"title": "Section 2",
		},
		Content: "Section 2 content",
	}
	g.AddNode(sec2)

	// Add relations
	g.AddEdge(&model.Relation{
		From:    "DOC-001",
		Type:    "contains",
		To:      "SEC-001",
		Content: "Relation content 1",
	})
	g.AddEdge(&model.Relation{
		From:    "DOC-001",
		Type:    "contains",
		To:      "SEC-002",
		Content: "Relation content 2",
	})

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
				IDPatterns: []string{"COMP-"},
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

	comp1 := &model.Entity{
		ID:   "COMP-001",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "Component 1",
		},
	}
	g.AddNode(comp1)

	comp2 := &model.Entity{
		ID:   "COMP-002",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "Component 2",
		},
	}
	g.AddNode(comp2)

	comp3 := &model.Entity{
		ID:   "COMP-003",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "Component 3",
		},
	}
	g.AddNode(comp3)

	// COMP-001 -> COMP-002 -> COMP-003 (chain)
	g.AddEdge(&model.Relation{From: "COMP-001", Type: "dependsOn", To: "COMP-002"})
	g.AddEdge(&model.Relation{From: "COMP-002", Type: "dependsOn", To: "COMP-003"})

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
				IDPatterns: []string{"DOC-"},
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
