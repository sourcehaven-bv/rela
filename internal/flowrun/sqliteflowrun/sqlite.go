// Package sqliteflowrun is the SQLite [flowrun.Store]: the sqlflowrun
// implementation over the *sql.DB that sqlitedb owns. The tables live in
// rela.db with the graph.
//
// The sqlite tier is single-process (DEC-LFSYNY) and every write that
// reads before it writes is a BEGIN IMMEDIATE transaction (see
// sqliteexec), so the dialect takes no locks. Delete is one statement.
package sqliteflowrun

import (
	"database/sql"

	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB = sqliteexec.DB

// dialect is the SQLite dialect: no locks (see the package doc).
var dialect = sqlflowrun.Dialect{Name: "sqliteflowrun"}

// New returns a run store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlflowrun.Store, error) {
	h, err := sqliteexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlflowrun.New(h, dialect)
}

// NewInTx returns a run store whose writes join the transaction open on
// conn, each under a savepoint, so they commit or roll back with it. conn is
// the one sqlitestore.TxConn returns inside a store transaction; the store
// is valid only until that transaction ends.
//
// Nil: rejected, so a wiring mistake fails at construction.
func NewInTx(conn *sql.Conn) (*sqlflowrun.Store, error) {
	h, err := sqliteexec.NewInTx(conn, "flowruns")
	if err != nil {
		return nil, err
	}
	return sqlflowrun.New(h, dialect)
}
