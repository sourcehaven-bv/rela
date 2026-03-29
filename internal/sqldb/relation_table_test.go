package sqldb

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dolthub/go-mysql-server/sql"
	sqltypes "github.com/dolthub/go-mysql-server/sql/types"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestNewRelationTable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)

	if table == nil {
		t.Fatal("expected non-nil table")
	}
	if table.Name() != "implements" {
		t.Errorf("expected table name 'implements', got %s", table.Name())
	}
	if table.String() != "implements" {
		t.Errorf("expected String() to return 'implements', got %s", table.String())
	}
}

func TestRelationTable_Schema(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
	schema := table.Schema()

	// Should have from_id, to_id, content
	if len(schema) != 3 {
		t.Errorf("expected 3 columns, got %d", len(schema))
	}

	// Check column names
	expectedCols := map[string]bool{"from_id": true, "to_id": true, "content": true}
	for _, col := range schema {
		if !expectedCols[col.Name] {
			t.Errorf("unexpected column: %s", col.Name)
		}
	}
}

func TestRelationTable_SchemaTypes(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
	schema := table.Schema()

	for _, col := range schema {
		if col.Type != sqltypes.Text {
			t.Errorf("expected column %s to be Text, got %v", col.Name, col.Type)
		}
	}
}

func TestRelationTable_SchemaNullable(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
	schema := table.Schema()

	for _, col := range schema {
		switch col.Name {
		case "from_id", "to_id":
			if col.Nullable {
				t.Errorf("expected %s to be non-nullable", col.Name)
			}
		case "content":
			if !col.Nullable {
				t.Error("expected content to be nullable")
			}
		}
	}
}

func TestRelationTable_Collation(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)

	if table.Collation() != sql.Collation_Default {
		t.Errorf("expected default collation, got %v", table.Collation())
	}
}

func TestRelationTable_Partitions(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
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

func TestRelationTable_PartitionRows(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
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

	// Should have 2 implements relations in testGraph
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

func TestRelationTable_RowData(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	table := NewRelationTable("implements", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Collect all rows
	rows := make([]sql.Row, 0)
	for {
		row, err := rowIter.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		rows = append(rows, row)
	}

	// Should have 2 rows
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Check that rows have correct structure (from_id, to_id, content)
	for _, row := range rows {
		if len(row) != 3 {
			t.Errorf("expected 3 columns, got %d", len(row))
		}

		fromID, ok := row[0].(string)
		if !ok {
			t.Errorf("expected from_id to be string, got %T", row[0])
		}

		toID, ok := row[1].(string)
		if !ok {
			t.Errorf("expected to_id to be string, got %T", row[1])
		}

		// Verify the relation makes sense
		validPairs := map[string]string{
			"FUNC-001": "REQ-001",
			"FUNC-002": "REQ-002",
		}

		expectedTo, exists := validPairs[fromID]
		if !exists {
			t.Errorf("unexpected from_id: %s", fromID)
		} else if toID != expectedTo {
			t.Errorf("for from_id %s, expected to_id %s, got %s", fromID, expectedTo, toID)
		}
	}
}

func TestRelationTable_EmptyRelations(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	// Create a relation type that has no instances
	// (belongs-to has relations but let's test an empty one)
	meta.Relations["empty"] = metamodel.RelationDef{
		Label: "Empty",
		From:  []string{"function"},
		To:    []string{"component"},
	}

	table := NewRelationTable("empty", g, meta)
	ctx := sql.NewContext(context.Background())

	rowIter, err := table.PartitionRows(ctx, &partition{})
	if err != nil {
		t.Fatalf("PartitionRows failed: %v", err)
	}
	defer rowIter.Close(ctx)

	// Should have 0 rows for empty relation
	_, err = rowIter.Next(ctx)
	if !errors.Is(err, io.EOF) {
		t.Error("expected EOF for empty relation table")
	}
}

func TestRelationTable_Content(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	// Add a relation with content
	relWithContent := model.NewRelation("FUNC-001", "belongs-to", "COMP-001")
	relWithContent.Content = "Primary assignment"
	// Note: testGraph already has this relation without content,
	// so we'll query the existing relations which have empty content

	table := NewRelationTable("belongs-to", g, meta)
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

	// Content should be at index 2 (it's the third column)
	// and should be empty string or nil for our test data
	content := row[2]
	if content != "" && content != nil {
		t.Errorf("expected empty content, got %v", content)
	}
}

func TestRelationTable_DifferentTypes(t *testing.T) {
	meta := testMetamodel()
	g := testGraph()

	// Test both relation types
	relTypes := []string{"implements", "belongs-to"}

	for _, relType := range relTypes {
		t.Run(relType, func(t *testing.T) {
			table := NewRelationTable(relType, g, meta)

			if table.Name() != relType {
				t.Errorf("expected table name %s, got %s", relType, table.Name())
			}

			ctx := sql.NewContext(context.Background())
			rowIter, err := table.PartitionRows(ctx, &partition{})
			if err != nil {
				t.Fatalf("PartitionRows failed: %v", err)
			}
			defer rowIter.Close(ctx)

			// Should have 2 relations of each type in testGraph
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

			if count != 2 {
				t.Errorf("expected 2 rows for %s, got %d", relType, count)
			}
		})
	}
}
