// Package sqliterecords is the SQLite [records.Store]: the sqlrecords
// implementation over the *sql.DB that sqlitedb owns, so records travel
// with the database file like the entities they belong to.
package sqliterecords

import (
	"github.com/Sourcehaven-BV/rela/internal/records/sqlrecords"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB = sqliteexec.DB

// New returns a record store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlrecords.Store, error) {
	h, err := sqliteexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlrecords.New(h, "sqliterecords")
}
