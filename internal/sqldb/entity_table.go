// Package sqldb provides a MySQL-compatible SQL interface to rela graphs.
package sqldb

import (
	"io"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/types"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// EntityTable exposes entities of a specific type as a SQL table.
type EntityTable struct {
	typeName string
	graph    *graph.Graph
	meta     *metamodel.Metamodel
	schema   sql.Schema
}

// NewEntityTable creates a new entity table for the given type.
func NewEntityTable(typeName string, g *graph.Graph, meta *metamodel.Metamodel) *EntityTable {
	t := &EntityTable{
		typeName: typeName,
		graph:    g,
		meta:     meta,
	}
	t.schema = t.buildSchema()
	return t
}

// Name returns the table name (pluralized entity type).
func (t *EntityTable) Name() string {
	return pluralize(t.typeName)
}

// String returns a string representation.
func (t *EntityTable) String() string {
	return t.Name()
}

// Schema returns the SQL schema for this entity type.
func (t *EntityTable) Schema() sql.Schema {
	return t.schema
}

// Collation returns the default collation.
func (t *EntityTable) Collation() sql.CollationID {
	return sql.Collation_Default
}

// Partitions returns a single partition.
func (t *EntityTable) Partitions(_ *sql.Context) (sql.PartitionIter, error) {
	return sql.PartitionsToPartitionIter(&partition{}), nil
}

// PartitionRows returns an iterator over all entities of this type.
func (t *EntityTable) PartitionRows(_ *sql.Context, _ sql.Partition) (sql.RowIter, error) {
	return &entityRowIter{
		table:    t,
		entities: t.graph.NodesByType(t.typeName),
		pos:      0,
	}, nil
}

// buildSchema creates a SQL schema from the metamodel entity definition.
func (t *EntityTable) buildSchema() sql.Schema {
	entityDef := t.meta.Entities[t.typeName]

	// Start with id column
	schema := sql.Schema{
		{Name: "id", Type: types.Text, Nullable: false, Source: t.Name(), PrimaryKey: true},
	}

	// Add property columns based on metamodel
	for propName, propDef := range entityDef.Properties {
		col := &sql.Column{
			Name:     propName,
			Source:   t.Name(),
			Nullable: !propDef.Required,
		}

		// Map property type to SQL type
		switch propDef.Type {
		case "boolean":
			col.Type = types.Boolean
		case "integer":
			col.Type = types.Int64
		case "date":
			col.Type = types.Date
		default:
			// string, enum, text, url, etc. all map to text
			col.Type = types.Text
		}

		schema = append(schema, col)
	}

	// Add content column for markdown body
	schema = append(schema, &sql.Column{
		Name:     "content",
		Type:     types.Text,
		Source:   t.Name(),
		Nullable: true,
	})

	return schema
}

// entityRowIter iterates over entities.
type entityRowIter struct {
	table    *EntityTable
	entities []*model.Entity
	pos      int
}

// Next returns the next row.
func (i *entityRowIter) Next(_ *sql.Context) (sql.Row, error) {
	if i.pos >= len(i.entities) {
		return nil, io.EOF
	}

	entity := i.entities[i.pos]
	i.pos++

	// Build row matching schema order
	row := make(sql.Row, len(i.table.schema))
	row[0] = entity.ID

	// Map properties to columns
	for idx, col := range i.table.schema {
		if idx == 0 {
			continue // id already set
		}
		if col.Name == "content" {
			row[idx] = entity.Content
			continue
		}

		// Get property value
		if val, ok := entity.Properties[col.Name]; ok {
			row[idx] = val
		} else {
			row[idx] = nil
		}
	}

	return row, nil
}

// Close closes the iterator.
func (i *entityRowIter) Close(_ *sql.Context) error {
	return nil
}

// partition implements sql.Partition for a single partition.
type partition struct{}

func (p *partition) Key() []byte {
	return []byte("all")
}

// Ensure EntityTable implements required interfaces.
var _ sql.Table = (*EntityTable)(nil)
