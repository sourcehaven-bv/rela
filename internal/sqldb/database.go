// Package sqldb provides a MySQL-compatible SQL interface to rela graphs.
package sqldb

import (
	"github.com/dolthub/go-mysql-server/sql"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Database wraps a rela graph as a SQL database.
type Database struct {
	name  string
	graph *graph.Graph
	meta  *metamodel.Metamodel
}

// NewDatabase creates a new SQL database backed by a rela graph.
func NewDatabase(name string, g *graph.Graph, meta *metamodel.Metamodel) *Database {
	return &Database{
		name:  name,
		graph: g,
		meta:  meta,
	}
}

// Name returns the database name.
func (db *Database) Name() string {
	return db.name
}

// GetTableInsensitive returns a table by name (case-insensitive).
func (db *Database) GetTableInsensitive(ctx *sql.Context, name string) (sql.Table, bool, error) {
	table, ok := db.GetTable(ctx, name)
	return table, ok, nil
}

// GetTableNames returns all table names.
func (db *Database) GetTableNames(_ *sql.Context) ([]string, error) {
	names := make([]string, 0, len(db.meta.Entities)+len(db.meta.Relations))

	// Entity type tables (pluralized)
	for typeName := range db.meta.Entities {
		names = append(names, pluralize(typeName))
	}

	// Relation type tables
	for relName := range db.meta.Relations {
		names = append(names, relName)
	}

	return names, nil
}

// GetTable returns a table by name.
func (db *Database) GetTable(_ *sql.Context, name string) (sql.Table, bool) {
	// Check if it's an entity table (pluralized name)
	for typeName := range db.meta.Entities {
		if pluralize(typeName) == name {
			return NewEntityTable(typeName, db.graph, db.meta), true
		}
	}

	// Check if it's a relation table
	if _, ok := db.meta.Relations[name]; ok {
		return NewRelationTable(name, db.graph, db.meta), true
	}

	return nil, false
}

// IsReadOnly returns true - rela SQL is read-only.
func (db *Database) IsReadOnly() bool {
	return true
}

// Helper to pluralize entity type names.
func pluralize(name string) string {
	// Simple pluralization rules
	switch {
	case name == "glossaryterm":
		return "glossaryterms"
	case name == "exampledata":
		return "exampledata"
	case name == "infrastructurecomponent":
		return "infrastructurecomponents"
	case name != "" && name[len(name)-1] == 's':
		return name + "es"
	case name != "" && name[len(name)-1] == 'y':
		return name[:len(name)-1] + "ies"
	default:
		return name + "s"
	}
}

// ViewDatabase interface - we don't support SQL views but need to implement this

// CreateView is not supported.
func (db *Database) CreateView(_ *sql.Context, _, _, _ string) error {
	return sql.ErrReadOnly.New()
}

// DropView is not supported.
func (db *Database) DropView(_ *sql.Context, name string) error {
	return sql.ErrViewDoesNotExist.New(name)
}

// GetViewDefinition returns no views.
func (db *Database) GetViewDefinition(_ *sql.Context, _ string) (sql.ViewDefinition, bool, error) {
	return sql.ViewDefinition{}, false, nil
}

// AllViews returns an empty list.
func (db *Database) AllViews(_ *sql.Context) ([]sql.ViewDefinition, error) {
	return nil, nil
}

// Ensure Database implements required interfaces.
var (
	_ sql.Database     = (*Database)(nil)
	_ sql.ViewDatabase = (*Database)(nil)
)
