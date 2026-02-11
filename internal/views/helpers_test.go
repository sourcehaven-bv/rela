package views

import (
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// setupDepsTestGraph creates a test graph with documents, sections, and components.
//
//	DOC-001 --contains--> SEC-001 --describes--> COMP-001
//	DOC-001 --contains--> SEC-002 --describes--> COMP-002
//	DOC-002 --contains--> SEC-003 --describes--> COMP-001
func setupDepsTestGraph() (*graph.Graph, *metamodel.Metamodel) {
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
			"component": {
				Label:      "Component",
				IDPrefixes: []string{"COMP-"},
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
			"describes": {
				Label: "describes",
				From:  []string{"section"},
				To:    []string{"component"},
			},
		},
	}

	g := graph.New()

	g.AddNode(&model.Entity{ID: "DOC-001", Type: "document", Properties: map[string]interface{}{"title": "Doc 1"}})
	g.AddNode(&model.Entity{ID: "DOC-002", Type: "document", Properties: map[string]interface{}{"title": "Doc 2"}})
	g.AddNode(&model.Entity{ID: "SEC-001", Type: "section", Properties: map[string]interface{}{"title": "Section 1"}})
	g.AddNode(&model.Entity{ID: "SEC-002", Type: "section", Properties: map[string]interface{}{"title": "Section 2"}})
	g.AddNode(&model.Entity{ID: "SEC-003", Type: "section", Properties: map[string]interface{}{"title": "Section 3"}})
	g.AddNode(&model.Entity{ID: "COMP-001", Type: "component", Properties: map[string]interface{}{"title": "Component 1"}})
	g.AddNode(&model.Entity{ID: "COMP-002", Type: "component", Properties: map[string]interface{}{"title": "Component 2"}})

	g.AddEdge(&model.Relation{From: "DOC-001", Type: "contains", To: "SEC-001"})
	g.AddEdge(&model.Relation{From: "DOC-001", Type: "contains", To: "SEC-002"})
	g.AddEdge(&model.Relation{From: "DOC-002", Type: "contains", To: "SEC-003"})
	g.AddEdge(&model.Relation{From: "SEC-001", Type: "describes", To: "COMP-001"})
	g.AddEdge(&model.Relation{From: "SEC-002", Type: "describes", To: "COMP-002"})
	g.AddEdge(&model.Relation{From: "SEC-003", Type: "describes", To: "COMP-001"})

	return g, meta
}

func makeDocView() ViewDef {
	return ViewDef{
		Entry: EntryDef{
			Type:      "document",
			Parameter: "doc_id",
		},
		Traverse: []TraverseRule{
			{
				From:      "entry",
				Follow:    "contains",
				CollectAs: "sections",
			},
			{
				From:      "sections",
				Follow:    "describes",
				CollectAs: "components",
			},
		},
	}
}

func entityIDs(entities []*model.Entity) []string {
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}
