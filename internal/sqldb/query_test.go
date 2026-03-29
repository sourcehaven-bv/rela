package sqldb

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// testMetamodel creates a test metamodel with multiple entity types and relations.
func testMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "approved", "accepted", "rejected"},
				Default: "draft",
			},
			"priority": {
				Values:  []string{"low", "medium", "high"},
				Default: "medium",
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPrefixes: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:   "status",
						Values: []string{"draft", "approved", "rejected"},
					},
					"priority": {
						Type:   "priority",
						Values: []string{"low", "medium", "high"},
					},
				},
			},
			"component": {
				Label:      "Component",
				IDPrefixes: []string{"COMP-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"active": {
						Type:    "boolean",
						Default: "true",
					},
				},
			},
			"function": {
				Label:      "Function",
				IDPrefixes: []string{"FUNC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "Implements",
				From:  []string{"function"},
				To:    []string{"requirement"},
			},
			"belongs-to": {
				Label: "Belongs to",
				From:  []string{"function"},
				To:    []string{"component"},
			},
		},
	}
}

// testGraph creates a test graph with sample entities and relations.
func testGraph() *graph.Graph {
	g := graph.New()

	// Add requirements
	req1 := model.NewEntity("REQ-001", "requirement")
	req1.Properties["title"] = "User authentication"
	req1.Properties["status"] = "approved"
	req1.Properties["priority"] = "high"
	g.AddNode(req1)

	req2 := model.NewEntity("REQ-002", "requirement")
	req2.Properties["title"] = "Data encryption"
	req2.Properties["status"] = "draft"
	req2.Properties["priority"] = "medium"
	g.AddNode(req2)

	req3 := model.NewEntity("REQ-003", "requirement")
	req3.Properties["title"] = "Logging"
	req3.Properties["status"] = "approved"
	req3.Properties["priority"] = "low"
	g.AddNode(req3)

	// Add components
	comp1 := model.NewEntity("COMP-001", "component")
	comp1.Properties["title"] = "Auth Module"
	comp1.Properties["active"] = "true"
	g.AddNode(comp1)

	comp2 := model.NewEntity("COMP-002", "component")
	comp2.Properties["title"] = "Security Module"
	comp2.Properties["active"] = "false"
	g.AddNode(comp2)

	// Add functions
	func1 := model.NewEntity("FUNC-001", "function")
	func1.Properties["title"] = "Login"
	func1.Content = "Handles user login"
	g.AddNode(func1)

	func2 := model.NewEntity("FUNC-002", "function")
	func2.Properties["title"] = "Encrypt"
	g.AddNode(func2)

	// Add relations
	rel1 := model.NewRelation("FUNC-001", "implements", "REQ-001")
	g.AddEdge(rel1)

	rel2 := model.NewRelation("FUNC-002", "implements", "REQ-002")
	g.AddEdge(rel2)

	rel3 := model.NewRelation("FUNC-001", "belongs-to", "COMP-001")
	g.AddEdge(rel3)

	rel4 := model.NewRelation("FUNC-002", "belongs-to", "COMP-002")
	g.AddEdge(rel4)

	return g
}

func TestQuery_Select(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id, title FROM requirements")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Check columns
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0] != "id" {
		t.Errorf("expected first column to be 'id', got %s", result.Columns[0])
	}
	if result.Columns[1] != "title" {
		t.Errorf("expected second column to be 'title', got %s", result.Columns[1])
	}

	// Check rows
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}

func TestQuery_SelectAll(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT * FROM components")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have id, title, active, content columns
	if len(result.Columns) < 3 {
		t.Errorf("expected at least 3 columns, got %d", len(result.Columns))
	}

	// Check rows
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestQuery_Where(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id, title FROM requirements WHERE status = 'approved'")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Check rows - should only have 2 approved requirements (REQ-001 and REQ-003)
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestQuery_WhereMultiple(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id FROM requirements WHERE status = 'approved' AND priority = 'high'")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Only REQ-001 is both approved and high priority
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestQuery_Join(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	query := `
		SELECT f.id, f.title, r.id, r.title
		FROM functions f
		JOIN implements i ON f.id = i.from_id
		JOIN requirements r ON i.to_id = r.id
	`
	result, err := Query(context.Background(), g, meta, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Check columns
	if len(result.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(result.Columns))
	}

	// Should have 2 rows (FUNC-001 -> REQ-001, FUNC-002 -> REQ-002)
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestQuery_Count(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT COUNT(*) FROM requirements")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	// Count should be 3
	count, ok := result.Rows[0][0].(int64)
	if !ok {
		t.Fatalf("expected count to be int64, got %T", result.Rows[0][0])
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestQuery_CountWithWhere(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT COUNT(*) FROM requirements WHERE status = 'approved'")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	count, ok := result.Rows[0][0].(int64)
	if !ok {
		t.Fatalf("expected count to be int64, got %T", result.Rows[0][0])
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestQuery_Limit(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id FROM requirements LIMIT 2")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestQuery_OrderBy(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id FROM requirements ORDER BY id ASC")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}

	// First row should be REQ-001
	if result.Rows[0][0] != "REQ-001" {
		t.Errorf("expected first row to be REQ-001, got %v", result.Rows[0][0])
	}
}

func TestQuery_RelationTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT from_id, to_id FROM implements")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Check columns
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}

	// Should have 2 implements relations
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestQuery_ShowTables(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SHOW TABLES")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have entity tables (requirements, components, functions) + relation tables (implements, belongs-to)
	if len(result.Rows) < 5 {
		t.Errorf("expected at least 5 tables, got %d", len(result.Rows))
	}
}

func TestQuery_Describe(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "DESCRIBE requirements")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have columns: id, title, status, priority, content
	if len(result.Rows) < 4 {
		t.Errorf("expected at least 4 columns in requirements table, got %d", len(result.Rows))
	}
}

func TestQuery_InvalidSQL(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	_, err := Query(context.Background(), g, meta, "INVALID SQL QUERY")
	if err == nil {
		t.Error("expected error for invalid SQL")
	}
}

func TestQuery_NonExistentTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	_, err := Query(context.Background(), g, meta, "SELECT * FROM nonexistent")
	if err == nil {
		t.Error("expected error for non-existent table")
	}
}

func TestQuery_EmptyResult(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id FROM requirements WHERE status = 'nonexistent'")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

func TestQuery_Content(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT id, content FROM functions WHERE id = 'FUNC-001'")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	// FUNC-001 has content "Handles user login"
	content := result.Rows[0][1]
	if content != "Handles user login" {
		t.Errorf("expected content 'Handles user login', got %v", content)
	}
}

func TestQuery_GroupBy(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	result, err := Query(context.Background(), g, meta, "SELECT status, COUNT(*) FROM requirements GROUP BY status")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have 2 groups: approved (2) and draft (1)
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Rows))
	}
}

func TestQuery_ComplexJoin(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	// Join functions to both requirements and components
	query := `
		SELECT f.id, r.title AS req_title, c.title AS comp_title
		FROM functions f
		JOIN implements i ON f.id = i.from_id
		JOIN requirements r ON i.to_id = r.id
		JOIN ` + "`belongs-to`" + ` b ON f.id = b.from_id
		JOIN components c ON b.to_id = c.id
	`
	result, err := Query(context.Background(), g, meta, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}
