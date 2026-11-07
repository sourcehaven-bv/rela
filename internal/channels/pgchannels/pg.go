// Package pgchannels is the PostgreSQL [channels.Store]: the sqlchannels
// implementation over the pgx pool the composition root shares with
// pgstore, as pgcomments does.
//
// Several rela-server processes may write one channel at once. Every
// mutation takes a transaction-scoped advisory lock on the entity first, so
// the writes of one entity serialize across processes and the seq stays
// gapless; writes to different entities do not wait on each other. The lock
// key includes the schema, so tenants sharing a database do not contend.
//
// This package does not depend on internal/store (arch-lint enforces it):
// a channel message is not graph content.
package pgchannels

import (
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/pgexec"
)

// DBTX is the database handle: a *pgxpool.Pool in production.
type DBTX = pgexec.DBTX

// lockEntity takes the advisory lock of one entity's channels for the rest
// of the transaction.
const lockEntity = `SELECT pg_advisory_xact_lock(hashtextextended('rela-channels:' || current_schema() || ':' || ?, 0))`

// New returns a channel store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DBTX) (*sqlchannels.Store, error) {
	h, err := pgexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlchannels.New(h, sqlchannels.Dialect{Name: "pgchannels", LockEntity: lockEntity})
}
