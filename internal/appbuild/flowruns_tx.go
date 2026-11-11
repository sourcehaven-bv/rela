package appbuild

import (
	"context"
	"errors"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// joinFlowRuns returns a run store whose writes join the open store
// transaction view, on the view's own database handle.
type joinFlowRuns func(view store.Store) (flowrun.Store, error)

// txFlowRuns writes a run start or a run's new state inside a write
// transaction of the entity store, on that transaction's handle
// (TKT-XI7IDBS). The manager asks the flow edit ceiling again inside the same
// kind of transaction before it persists a field write (see
// entitymanager.FieldWriteGate). The store's write lock orders the two:
// postgres takes its schema-wide advisory lock, sqlite its write mutex and
// BEGIN IMMEDIATE. A run start or step therefore commits before the check,
// which then sees it, or waits until the write has committed.
//
// List reads on the transaction's handle when ctx carries its view
// ([store.ContextWithTxView]): the manager's check inside the transaction
// and the rename guard then take no second pool connection while the write
// lock is held, and on sqlite they read on the connection that holds it.
// Postgres sees the run under its default READ COMMITTED isolation, which
// takes each statement's snapshot after the advisory lock was granted.
//
// Delete is not wrapped: ending a run only lifts rules. The fs and memory
// tiers have no transaction to join (see joinFlowRunsFor), so there the
// check and a run start are not ordered.
type txFlowRuns struct {
	flowrun.Store
	st   store.Store
	join joinFlowRuns
}

// lockFlowRuns returns runs with its starts and steps ordered against field
// writes to st, or runs itself when st has no transaction a run write can
// join.
func lockFlowRuns(st store.Store, runs flowrun.Store) flowrun.Store {
	join := joinFlowRunsFor(st, runs)
	if join == nil {
		return runs
	}
	return txFlowRuns{Store: runs, st: st, join: join}
}

// errFlowRunsNested is a run write asked for inside a store transaction. It
// would wait for the write lock its own caller holds.
var errFlowRunsNested = errors.New("appbuild: flow runs: a run write cannot run inside a store transaction")

// List implements [flowrun.Store], on the transaction's handle when ctx
// carries its view.
func (r txFlowRuns) List(ctx context.Context, q flowrun.Query) ([]flowrun.Run, error) {
	view := store.TxView(ctx)
	if view == nil {
		return r.Store.List(ctx, q)
	}
	in, err := r.join(view)
	if err != nil {
		return nil, err
	}
	return in.List(ctx, q)
}

// Create implements [flowrun.Store] inside a store transaction.
func (r txFlowRuns) Create(ctx context.Context, nr flowrun.NewRun) (flowrun.Run, error) {
	if store.InTx(ctx) {
		return flowrun.Run{}, errFlowRunsNested
	}
	var out flowrun.Run
	err := r.st.Tx(ctx, func(view store.Store) error {
		in, err := r.join(view)
		if err != nil {
			return err
		}
		out, err = in.Create(store.ContextInTx(ctx), nr)
		return err
	})
	return out, err
}

// Update implements [flowrun.Store] inside a store transaction.
func (r txFlowRuns) Update(
	ctx context.Context, id flowrun.RunID, expectRev int64, st flowrun.State, at time.Time,
) (flowrun.Run, error) {
	if store.InTx(ctx) {
		return flowrun.Run{}, errFlowRunsNested
	}
	var out flowrun.Run
	err := r.st.Tx(ctx, func(view store.Store) error {
		in, err := r.join(view)
		if err != nil {
			return err
		}
		out, err = in.Update(store.ContextInTx(ctx), id, expectRev, st, at)
		return err
	})
	return out, err
}
