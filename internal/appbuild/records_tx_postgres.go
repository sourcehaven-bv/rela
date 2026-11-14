//go:build postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/pgrecords"
	"github.com/Sourcehaven-BV/rela/internal/records/sqlrecords"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// joinRecordsFor returns how a record write joins a transaction of st: a
// pgrecords store over the transaction's handle, whose own transactions
// become savepoints. The write then holds the store's advisory lock.
//
// Nil: returned when st is not a pgstore or backend is not the database
// record store; record writes are then not ordered against entity writes.
func joinRecordsFor(st store.Store, backend records.Store) joinRecords {
	if _, db := backend.(*sqlrecords.Store); !db || pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(view store.Store) (records.Store, error) {
		h := pgstore.HandleFor(view)
		if h == nil {
			return nil, errRecordsNotTxView
		}
		return pgrecords.New(h)
	}
}

// errRecordsNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the record write back.
var errRecordsNotTxView = errors.New("appbuild: records: the transaction view is not a database store")
