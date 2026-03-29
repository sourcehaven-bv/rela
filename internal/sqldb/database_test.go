package sqldb

import (
	"context"
	"sort"
	"testing"

	"github.com/dolthub/go-mysql-server/sql"
)

func TestNewDatabase(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	if db == nil {
		t.Fatal("expected non-nil database")
	}
	if db.Name() != "testdb" {
		t.Errorf("expected database name 'testdb', got %s", db.Name())
	}
}

func TestDatabase_GetTableNames(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	ctx := sql.NewContext(context.Background())
	names, err := db.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	// Should have entity tables (requirements, components, functions) + relation tables (implements, belongs-to)
	if len(names) != 5 {
		t.Errorf("expected 5 tables, got %d: %v", len(names), names)
	}

	// Check that expected tables are present
	sort.Strings(names)
	expected := []string{"belongs-to", "components", "functions", "implements", "requirements"}
	sort.Strings(expected)

	for i, exp := range expected {
		if names[i] != exp {
			t.Errorf("expected table %s at position %d, got %s", exp, i, names[i])
		}
	}
}

func TestDatabase_GetTable_EntityTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	ctx := sql.NewContext(context.Background())

	// Get entity table by pluralized name
	table, ok := db.GetTable(ctx, "requirements")
	if !ok {
		t.Fatal("expected to find requirements table")
	}

	if table.Name() != "requirements" {
		t.Errorf("expected table name 'requirements', got %s", table.Name())
	}

	// Verify it's an EntityTable
	_, isEntityTable := table.(*EntityTable)
	if !isEntityTable {
		t.Error("expected table to be an EntityTable")
	}
}

func TestDatabase_GetTable_RelationTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	ctx := sql.NewContext(context.Background())

	// Get relation table by name
	table, ok := db.GetTable(ctx, "implements")
	if !ok {
		t.Fatal("expected to find implements table")
	}

	if table.Name() != "implements" {
		t.Errorf("expected table name 'implements', got %s", table.Name())
	}

	// Verify it's a RelationTable
	_, isRelationTable := table.(*RelationTable)
	if !isRelationTable {
		t.Error("expected table to be a RelationTable")
	}
}

func TestDatabase_GetTable_NotFound(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	ctx := sql.NewContext(context.Background())

	_, ok := db.GetTable(ctx, "nonexistent")
	if ok {
		t.Error("expected not to find nonexistent table")
	}
}

func TestDatabase_GetTableInsensitive(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	ctx := sql.NewContext(context.Background())

	// GetTableInsensitive should work like GetTable
	table, ok, err := db.GetTableInsensitive(ctx, "requirements")
	if err != nil {
		t.Fatalf("GetTableInsensitive failed: %v", err)
	}
	if !ok {
		t.Fatal("expected to find requirements table")
	}
	if table.Name() != "requirements" {
		t.Errorf("expected table name 'requirements', got %s", table.Name())
	}
}

func TestDatabase_IsReadOnly(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)

	if !db.IsReadOnly() {
		t.Error("expected database to be read-only")
	}
}

func TestDatabase_ViewOperations(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	db := NewDatabase("testdb", g, meta)
	ctx := sql.NewContext(context.Background())

	// CreateView should return read-only error
	err := db.CreateView(ctx, "testview", "SELECT 1", "CREATE VIEW testview AS SELECT 1")
	if err == nil {
		t.Error("expected error from CreateView")
	}

	// DropView should return view not found error
	err = db.DropView(ctx, "testview")
	if err == nil {
		t.Error("expected error from DropView")
	}

	// GetViewDefinition should return false
	_, ok, err := db.GetViewDefinition(ctx, "testview")
	if err != nil {
		t.Fatalf("GetViewDefinition failed: %v", err)
	}
	if ok {
		t.Error("expected view not to be found")
	}

	// AllViews should return empty list
	views, err := db.AllViews(ctx)
	if err != nil {
		t.Fatalf("AllViews failed: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("expected 0 views, got %d", len(views))
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"requirement", "requirements"},
		{"entity", "entities"},
		{"class", "classes"},
		{"bus", "buses"},
		{"glossaryterm", "glossaryterms"},
		{"exampledata", "exampledata"},
		{"infrastructurecomponent", "infrastructurecomponents"},
		{"component", "components"},
		{"function", "functions"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := pluralize(tt.input)
			if result != tt.expected {
				t.Errorf("pluralize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
