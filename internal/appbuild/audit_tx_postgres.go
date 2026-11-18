//go:build postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/audit/pgaudit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// joinAuditFor returns how an audit append joins a transaction of st: a
// pgaudit log over the transaction's handle, whose own transaction becomes a
// savepoint. A view's handle must be the transaction itself; the pool would
// append outside the write and commit whatever became of it.
//
// Nil: returned when st is not a pgstore.
func joinAuditFor(st store.Store) joinAudit {
	if pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(view store.Store) (*sqlaudit.Log, error) {
		h := pgstore.TxHandleFor(view)
		if h == nil {
			return nil, errAuditNotTxView
		}
		return pgaudit.New(h)
	}
}

// errAuditNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the write back.
var errAuditNotTxView = errors.New("appbuild: audit: the store is not a transaction view of the database store")
