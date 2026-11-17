// Package pgrecords is the PostgreSQL [records.Store]: the sqlrecords
// implementation over the pgx pool the composition root shares with
// pgstore.
//
// This package does not depend on internal/store: a record is not graph
// content.
package pgrecords

import (
	"github.com/Sourcehaven-BV/rela/internal/records/sqlrecords"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/pgexec"
)

// DBTX is the database handle: a *pgxpool.Pool in production.
type DBTX = pgexec.DBTX

// New returns a record store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DBTX) (*sqlrecords.Store, error) {
	h, err := pgexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlrecords.New(h, "pgrecords")
}
