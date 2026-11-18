//go:build sqlite && !postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqliteaudit"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// joinAuditFor returns how an audit append joins a transaction of st: a
// sqliteaudit log on the transaction's connection, which appends under a
// savepoint. An append on another connection would wait on the
// transaction's lock until it timed out.
//
// Nil: returned when st is not a sqlitestore.
func joinAuditFor(st store.Store) joinAudit {
	if _, db := store.Innermost(st).(*sqlitestore.Store); !db {
		return nil
	}
	return func(view store.Store) (*sqlaudit.Log, error) {
		conn, ok := sqlitestore.TxConn(store.Innermost(view))
		if !ok {
			return nil, errAuditNotTxView
		}
		return sqliteaudit.NewInTx(conn)
	}
}

// errAuditNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the write back.
var errAuditNotTxView = errors.New("appbuild: audit: the store is not a transaction view of the database store")
