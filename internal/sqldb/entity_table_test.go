package sqldb

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/types"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestNewEntityTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)

	if table == nil {
		t.Fatal("expected non-nil table")
	}
	if table.Name() != "requirements" {
		t.Errorf("expected table name 'requirements', got %s", table.Name())
	}
	if table.String() != "requirements" {
		t.Errorf("expected String() to return 'requirements', got %s", table.String())
	}
}

func TestEntityTable_Schema(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)
	schema := table.Schema()

	if len(schema) == 0 {
		t.Fatal("expected non-empty schema")
	}

	// First column should be id
	if schema[0].Name != "id" {
		t.Errorf("expected first column to be 'id', got %s", schema[0].Name)
	}
	if schema[0].PrimaryKey != true {
		t.Error("expected id column to be primary key")
	}

	// Check that title, status, priority, and content columns exist
	columnNames := make(map[string]bool)
	for _, col := range schema {
		columnNames[col.Name] = true
	}

	requiredColumns := []string{"id", "title", "status", "priority", "content"}
	for _, name := range requiredColumns {
		if !columnNames[name] {
			t.Errorf("expected column %s in schema", name)
		}
	}
}

func TestEntityTable_SchemaTypes(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)
	schema := table.Schema()

	// Find columns by name
	colTypes := make(map[string]sql.Type)
	for _, col := range schema {
		colTypes[col.Name] = col.Type
	}

	// id should be text
	if colTypes["id"] != types.Text {
		t.Errorf("expected id to be Text, got %v", colTypes["id"])
	}

	// title (string) should be text
	if colTypes["title"] != types.Text {
		t.Errorf("expected title to be Text, got %v", colTypes["title"])
	}

	// content should be text
	if colTypes["content"] != types.Text {
		t.Errorf("expected content to be Text, got %v", colTypes["content"])
	}
}

func TestEntityTable_Collation(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)

	if table.Collation() != sql.Collation_Default {
		t.Errorf("expected default collation, got %v", table.Collation())
	}
}

func TestEntityTable_Partitions(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)
	ctx := sql.NewContext(context.Background())

	partIter, err := table.Partitions(ctx)
	if err != nil {
		t.Fatalf("Partitions failed: %v", err)
	}

	// Should have exactly one partition
	part, err := partIter.Next(ctx)
	if err != nil {
		t.Fatalf("expected partition, got error: %v", err)
	}

	if string(part.Key()) != "all" {
		t.Errorf("expected partition key 'all', got %s", string(part.Key()))
	}

	// No more partitions
	_, err = partIter.Next(ctx)
	if !errors.Is(err, io.EOF) {
		t.Error("expected EOF after first partition")
	}

	partIter.Close(ctx)
}

func TestEntityTable_PartitionRows(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Count rows
	count := 0
	for {
		_, err := rowIter.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		count++
	}

	// Should have 3 requirements in testGraph
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}
}

func TestEntityTable_RowData(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("requirement", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Get first row
	row, err := rowIter.Next(ctx)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}

	// Row should have id as first element
	id, ok := row[0].(string)
	if !ok {
		t.Fatalf("expected id to be string, got %T", row[0])
	}

	// ID should be one of the requirements
	validIDs := map[string]bool{"REQ-001": true, "REQ-002": true, "REQ-003": true}
	if !validIDs[id] {
		t.Errorf("unexpected id: %s", id)
	}
}

func TestEntityTable_NilProperty(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	// Add an entity without all properties
	sparse := model.NewEntity("REQ-999", "requirement")
	sparse.Properties["title"] = "Sparse"
	// Don't set status or priority
	g.AddNode(sparse)

	table := NewEntityTable("requirement", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Iterate to find REQ-999
	for {
		row, err := rowIter.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}

		if row[0] == "REQ-999" {
			// Found our sparse entity - some properties should be nil
			// Just verify it doesn't crash
			return
		}
	}

	t.Error("REQ-999 not found in results")
}

func TestEntityTable_Content(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewEntityTable("function", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Look for FUNC-001 which has content
	schema := table.Schema()
	contentIdx := -1
	for i, col := range schema {
		if col.Name == "content" {
			contentIdx = i
			break
		}
	}

	if contentIdx == -1 {
		t.Fatal("content column not found in schema")
	}

	for {
		row, err := rowIter.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}

		if row[0] == "FUNC-001" {
			content := row[contentIdx]
			if content != "Handles user login" {
				t.Errorf("expected content 'Handles user login', got %v", content)
			}
			return
		}
	}

	t.Error("FUNC-001 not found in results")
}
