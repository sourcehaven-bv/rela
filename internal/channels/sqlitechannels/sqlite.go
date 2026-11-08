// Package sqlitechannels is the SQLite [channels.Store]: the sqlchannels
// implementation over the *sql.DB that sqlitedb owns.
//
// Channels live in rela.db for the reason comments do (see the
// sqlitecomments package doc): a copied or shipped database carries the
// conversation with the entities it is about.
//
// The sqlite tier is single-process (DEC-LFSYNY), so writes serialize on an
// in-process mutex and then on SQLite's own write lock (see sqliteexec).
package sqlitechannels

import (
	"database/sql"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB = sqliteexec.DB

// dialect is the SQLite dialect: no entity lock, since the write
// transaction already excludes every other writer.
var dialect = sqlchannels.Dialect{Name: "sqlitechannels"}

// New returns a channel store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlchannels.Store, error) {
	h, err := sqliteexec.New(db)
	if err != nil {
		return nil, err
	}
	return sqlchannels.New(h, dialect)
}

// NewInTx returns a channel store whose writes join the transaction open on
// conn: each write runs in a SAVEPOINT on conn, so it commits or rolls back
// with that transaction. It is for a write that must change the entity graph
// and a channel atomically; conn is the one sqlitestore.TxConn returns
// inside a store transaction. The store is valid only until that
// transaction ends.
//
// Nil: rejected, so a wiring mistake fails at construction.
func NewInTx(conn *sql.Conn) (*sqlchannels.Store, error) {
	h, err := sqliteexec.NewInTx(conn, "channels")
	if err != nil {
		return nil, err
	}
	return sqlchannels.New(h, dialect)
}
