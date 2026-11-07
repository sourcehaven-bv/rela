// Package pgflowrun is the PostgreSQL [flowrun.Store]: the sqlflowrun
// implementation over the pgx pool the composition root shares with
// pgstore.
//
// Several rela-server processes may start and move runs at once. The locks
// are transaction-scoped advisory locks whose keys include the schema, so
// tenants sharing a database do not contend: one per subject id, and one
// per schema over the definitions, taken shared by a create and exclusive
// by a prune.
package pgflowrun

import (
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/pgexec"
)

// DBTX is the database handle: a *pgxpool.Pool in production.
type DBTX = pgexec.DBTX

const (
	lockDefinitionsShared = `SELECT pg_advisory_xact_lock_shared(hashtextextended('rela-flowdefs:' || current_schema(), 0))`
	lockDefinitions       = `SELECT pg_advisory_xact_lock(hashtextextended('rela-flowdefs:' || current_schema(), 0))`
	lockSubject           = `SELECT pg_advisory_xact_lock(hashtextextended('rela-flowsubject:' || current_schema() || ':' || ?, 0))`
)

// New returns a run store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DBTX) (*sqlflowrun.Store, error) {
	h, err := pgexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlflowrun.New(h, sqlflowrun.Dialect{
		Name:                  "pgflowrun",
		LockDefinitionsShared: lockDefinitionsShared,
		LockDefinitions:       lockDefinitions,
		LockSubject:           lockSubject,
	})
}
