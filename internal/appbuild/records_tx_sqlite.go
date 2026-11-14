//go:build sqlite && !postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/sqliterecords"
	"github.com/Sourcehaven-BV/rela/internal/records/sqlrecords"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// joinRecordsFor returns how a record write joins a transaction of st: a
// sqliterecords store on the transaction's connection, which writes under a
// savepoint. A write on another connection would wait on the transaction's
// lock until it timed out.
//
// Nil: returned when st is not a sqlitestore or backend is not the database
// record store; record writes are then not ordered against entity writes.
func joinRecordsFor(st store.Store, backend records.Store) joinRecords {
	_, dbRecords := backend.(*sqlrecords.Store)
	if _, db := store.Innermost(st).(*sqlitestore.Store); !dbRecords || !db {
		return nil
	}
	return func(view store.Store) (records.Store, error) {
		conn, ok := sqlitestore.TxConn(store.Innermost(view))
		if !ok {
			return nil, errRecordsNotTxView
		}
		return sqliterecords.NewInTx(conn)
	}
}

// errRecordsNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the record write back.
var errRecordsNotTxView = errors.New("appbuild: records: the transaction view is not a database store")
