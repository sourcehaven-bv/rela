// Package pgaudit is the PostgreSQL audit log: the sqlaudit implementation
// over the pgx pool the composition root shares with pgstore. The tables
// live in the store's schema (pgstore migration 0051), so each tenant has
// its own chain.
//
// Several rela-server processes may append at once. The chain lock is a
// transaction-scoped advisory lock whose key includes the schema, so
// tenants sharing a database do not contend.
package pgaudit

import (
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/pgexec"
)

// DBTX is the database handle: a *pgxpool.Pool in production.
type DBTX = pgexec.DBTX

const lockChain = `SELECT pg_advisory_xact_lock(hashtextextended('rela-audit:' || current_schema(), 0))`

// New returns a log over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DBTX) (*sqlaudit.Log, error) {
	h, err := pgexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlaudit.New(h, sqlaudit.Dialect{Name: "pgaudit", LockChain: lockChain})
}
