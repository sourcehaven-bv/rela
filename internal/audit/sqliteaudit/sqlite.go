// Package sqliteaudit is the SQLite audit log: the sqlaudit implementation
// over the *sql.DB that sqlitedb owns. The tables live in rela.db with the
// graph.
//
// The sqlite tier is single-process (DEC-LFSYNY) and every write is a
// BEGIN IMMEDIATE transaction (see sqliteexec), so the dialect takes no
// chain lock.
package sqliteaudit

import (
	"database/sql"

	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB = sqliteexec.DB

// New returns a log over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlaudit.Log, error) {
	h, err := sqliteexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlaudit.New(h, sqlaudit.Dialect{Name: "sqliteaudit"})
}

// NewInTx returns a log over conn, the connection of an open sqlitestore
// transaction (sqlitestore.TxConn). An append runs in a savepoint of that
// transaction, so it commits or rolls back with the write it records. The
// log is valid only until the transaction ends.
//
// Nil: conn rejected.
func NewInTx(conn *sql.Conn) (*sqlaudit.Log, error) {
	h, err := sqliteexec.NewInTx(conn, "audit")
	if err != nil {
		return nil, err
	}
	return sqlaudit.New(h, sqlaudit.Dialect{Name: "sqliteaudit"})
}
