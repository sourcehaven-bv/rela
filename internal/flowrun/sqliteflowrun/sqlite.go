// Package sqliteflowrun is the SQLite [flowrun.Store]: the sqlflowrun
// implementation over the *sql.DB that sqlitedb owns. The tables live in
// rela.db with the graph.
//
// The sqlite tier is single-process (DEC-LFSYNY) and every write that
// reads before it writes is a BEGIN IMMEDIATE transaction (see
// sqliteexec), so the dialect takes no locks. Delete is one statement.
package sqliteflowrun

import (
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB = sqliteexec.DB

// New returns a run store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlflowrun.Store, error) {
	h, err := sqliteexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlflowrun.New(h, sqlflowrun.Dialect{Name: "sqliteflowrun"})
}
