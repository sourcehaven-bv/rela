//go:build postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/pgflowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// joinFlowRunsFor returns how a run write joins a transaction of st: a
// pgflowrun store over the transaction's handle, whose own transactions
// become savepoints. The write then holds the store's advisory lock.
//
// Nil: returned when st is not a pgstore or runs is not the database run
// store; run writes are then not ordered against field writes.
func joinFlowRunsFor(st store.Store, runs flowrun.Store) joinFlowRuns {
	if _, db := runs.(*sqlflowrun.Store); !db || pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(view store.Store) (flowrun.Store, error) {
		h := pgstore.HandleFor(view)
		if h == nil {
			return nil, errFlowRunsNotTxView
		}
		return pgflowrun.New(h)
	}
}

// errFlowRunsNotTxView is a join of a store that is not a transaction view of
// the database store. It rolls the run write back.
var errFlowRunsNotTxView = errors.New("appbuild: flow runs: the transaction view is not a database store")
