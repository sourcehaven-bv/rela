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

// RelationTable exposes relations of a specific type as a SQL table.
type RelationTable struct {
	relName string
	graph   *graph.Graph
	meta    *metamodel.Metamodel
	schema  sql.Schema
}

// NewRelationTable creates a new relation table for the given relation type.
func NewRelationTable(relName string, g *graph.Graph, meta *metamodel.Metamodel) *RelationTable {
	t := &RelationTable{
		relName: relName,
		graph:   g,
		meta:    meta,
	}
	t.schema = t.buildSchema()
	return t
}

// Name returns the table name (relation type name).
func (t *RelationTable) Name() string {
	return t.relName
}

// String returns a string representation.
func (t *RelationTable) String() string {
	return t.Name()
}

// Schema returns the SQL schema for this relation type.
func (t *RelationTable) Schema() sql.Schema {
	return t.schema
}

// Collation returns the default collation.
func (t *RelationTable) Collation() sql.CollationID {
	return sql.Collation_Default
}

// Partitions returns a single partition.
func (t *RelationTable) Partitions(_ *sql.Context) (sql.PartitionIter, error) {
	return sql.PartitionsToPartitionIter(&partition{}), nil
}

// PartitionRows returns an iterator over all relations of this type.
func (t *RelationTable) PartitionRows(_ *sql.Context, _ sql.Partition) (sql.RowIter, error) {
	return &relationRowIter{
		table:     t,
		relations: t.graph.RelationsOfType(t.relName),
		pos:       0,
	}, nil
}

// buildSchema creates a SQL schema from the metamodel relation definition.
func (t *RelationTable) buildSchema() sql.Schema {
	// Standard relation columns: from_id, to_id, content
	// Note: relations don't have typed properties yet in the metamodel
	return sql.Schema{
		{Name: "from_id", Type: types.Text, Nullable: false, Source: t.Name()},
		{Name: "to_id", Type: types.Text, Nullable: false, Source: t.Name()},
		{Name: "content", Type: types.Text, Nullable: true, Source: t.Name()},
	}
}

// relationRowIter iterates over relations.
type relationRowIter struct {
	table     *RelationTable
	relations []*model.Relation
	pos       int
}

// Next returns the next row.
func (i *relationRowIter) Next(_ *sql.Context) (sql.Row, error) {
	if i.pos >= len(i.relations) {
		return nil, io.EOF
	}

	rel := i.relations[i.pos]
	i.pos++

	// Build row matching schema order
	row := make(sql.Row, len(i.table.schema))
	row[0] = rel.From
	row[1] = rel.To

	// Set content
	row[2] = rel.Content

	return row, nil
}

// Close closes the iterator.
func (i *relationRowIter) Close(_ *sql.Context) error {
	return nil
}

// Ensure RelationTable implements required interfaces.
var _ sql.Table = (*RelationTable)(nil)
