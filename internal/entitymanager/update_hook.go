package entitymanager

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// UpdateHook is told of each update inside the store transaction that
// writes it. The flow runtime is one: a write that moves a flow run or
// starts one stores the run in that transaction, so the run commits or
// rolls back with the write.
//
// It covers every update that goes through updateCore: [Manager.UpdateEntity],
// [Manager.PatchEntity] and a granted patch. It does not cover a rename,
// a conversion, a restore, a copy, a cascade write or a write straight to
// the store.
type UpdateHook interface {
	// Watches reports whether an update of an entity of type typ concerns
	// the hook. An update the hook watches always runs in a store
	// transaction.
	Watches(typ string) bool
	// PrepareUpdate runs before the store transaction of a watched update,
	// outside any transaction, with old the row as read before it, and
	// returns the ctx the update goes on with. An update that joins a
	// transaction its caller opened skips it, so EntityUpdated must also
	// work without it.
	PrepareUpdate(ctx context.Context, old, e *entity.Entity) (context.Context, error)
	// EntityUpdated runs after the row and its audit record are written,
	// on view, the transaction's store, with old the row as read inside
	// the transaction before the write and e the row as written. An error
	// fails the update. A
	// non-nil afterCommit runs once the update has committed and run its
	// cascade, on the update's ctx outside the transaction.
	EntityUpdated(
		ctx context.Context, view store.Store, old, e *entity.Entity,
	) (afterCommit func(context.Context), err error)
}

// watchesUpdate reports whether deps.TxHooks.UpdateHook watches updates of
// typ.
//
// Nil: deps.TxHooks.UpdateHook accepted (nothing watches).
func watchesUpdate(deps Deps, typ string) bool {
	return deps.TxHooks.UpdateHook != nil && deps.TxHooks.UpdateHook.Watches(typ)
}

// updateHookError is an [UpdateHook.EntityUpdated] failure.
type updateHookError struct{ err error }

func (e updateHookError) Error() string { return e.err.Error() }
func (e updateHookError) Unwrap() error { return e.err }

// noTxCheck is a transaction check that checks nothing. A watched update
// chains it to make sure its write runs in a transaction.
func noTxCheck(context.Context) error { return nil }

// beginUpdate prepares a watched update of e on ctx. It returns the ctx
// the update runs on, the operation whose finish runs the follow-ups
// queued during the write, and txCheck, chained so that a watched update
// always runs in a transaction. An update that is not watched, joins a
// transaction, or whose ctx already carries a commit list, gets an
// operation that runs nothing: the owner of the list runs it.
//
// Nil: txCheck accepted (no check).
func beginUpdate(ctx context.Context, deps Deps, old, e *entity.Entity, txCheck func(context.Context) error) (
	context.Context, *createOp, func(context.Context) error, error) {
	op := &createOp{}
	if !watchesUpdate(deps, e.Type) {
		return ctx, op, txCheck, nil
	}
	txCheck = chainTxChecks(txCheck, noTxCheck)
	if store.TxView(ctx) != nil {
		return ctx, op, txCheck, nil
	}
	ctx, err := deps.TxHooks.UpdateHook.PrepareUpdate(ctx, old, e)
	if err != nil {
		return nil, nil, nil, err
	}
	outer := scopeOf(ctx)
	if outer.commits != nil {
		return ctx, op, txCheck, nil
	}
	op.commits = &commitList{}
	scope := *outer
	scope.commits = op.commits
	return withTxScope(ctx, &scope), op, txCheck, nil
}

// runUpdateHook runs deps.TxHooks.UpdateHook for e, written on view, and
// queues its afterCommit on the commit list of ctx. prior is the row as
// read on view before the write: a writer that committed after the caller
// read its row is then not judged against the older row.
func runUpdateHook(ctx context.Context, deps Deps, view store.Store, prior, e *entity.Entity) error {
	if !watchesUpdate(deps, e.Type) {
		return nil
	}
	after, err := deps.TxHooks.UpdateHook.EntityUpdated(ctx, view, prior, e)
	if err != nil {
		return updateHookError{err}
	}
	if after == nil {
		return nil
	}
	if !onCommit(ctx, after) {
		slog.InfoContext(ctx, "entitymanager: update hook follow-up left to its owner: no commit list",
			"entity", e.ID)
	}
	return nil
}

// priorRow reads e's row on view before a watched update writes it, or
// returns fallback for an update the hook does not watch.
func priorRow(ctx context.Context, deps Deps, view store.Store, e, fallback *entity.Entity) (*entity.Entity, error) {
	if !watchesUpdate(deps, e.Type) {
		return fallback, nil
	}
	return view.GetEntity(ctx, e.Ref())
}
