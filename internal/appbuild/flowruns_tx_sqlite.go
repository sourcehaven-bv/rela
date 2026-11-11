//go:build sqlite && !postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqliteflowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// joinFlowRunsFor returns how a run write joins a transaction of st: a
// sqliteflowrun store on the transaction's connection, which writes under a
// savepoint. A write on another connection would wait on the transaction's
// lock until it timed out.
//
// Nil: returned when st is not a sqlitestore or runs is not the database
// run store; run writes are then not ordered against field writes.
func joinFlowRunsFor(st store.Store, runs flowrun.Store) joinFlowRuns {
	_, dbRuns := runs.(*sqlflowrun.Store)
	if _, db := store.Innermost(st).(*sqlitestore.Store); !dbRuns || !db {
		return nil
	}
	return func(view store.Store) (flowrun.Store, error) {
		conn, ok := sqlitestore.TxConn(store.Innermost(view))
		if !ok {
			return nil, errFlowRunsNotTxView
		}
		return sqliteflowrun.NewInTx(conn)
	}
}

// errFlowRunsNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the run write back.
var errFlowRunsNotTxView = errors.New("appbuild: flow runs: the transaction view is not a database store")
