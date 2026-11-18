package entitymanager

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CreateHook is told of each new entity inside the store transaction that
// creates it. The flow runtime is one: a flow that starts on create stores
// its run in that transaction, so the run commits or rolls back with the
// entity.
//
// It covers every create that goes through createCore: a direct create, a
// linked create, a batch, a mapped import and a cascade. It does not cover
// a copy's new face, a conversion or a history restore, which bring an
// existing entity to a new place rather than create one, nor `rela import`
// of an export and `rela db load`, which write to the store directly.
type CreateHook interface {
	// Watches reports whether a create of an entity of type typ concerns
	// the hook. A create the hook watches always runs in a store
	// transaction, so the hook has one to join.
	Watches(typ string) bool
	// PrepareCreate runs before the store transaction of a watched
	// create, outside any transaction, and returns the ctx the create
	// goes on with. It is where the hook does the reads it needs (binding
	// an identity), so that they do not run while the transaction holds
	// its connection and, on postgres, the write lock. A create that joins
	// a transaction its caller opened, and a cascade, skip it, so
	// EntityCreated must also work without it.
	PrepareCreate(ctx context.Context) (context.Context, error)
	// EntityCreated runs after the row and its audit record are written,
	// on view, the transaction's store; ctx carries the transaction too.
	// An error fails the create and, on a store that rolls back, undoes
	// it. afterCommit, when not nil, runs once the create's entry point
	// has committed and run the entity's own follow-ups (alias
	// notification, automations), on that entry point's ctx. It does not
	// run when the create is part of a transaction the manager did not
	// open, which then holds no list of them.
	EntityCreated(
		ctx context.Context, view store.Store, e *entity.Entity,
	) (afterCommit func(context.Context), err error)
}

// watchesCreate reports whether deps.CreateHook watches creates of typ.
//
// Nil: deps.CreateHook accepted (nothing watches).
func watchesCreate(deps Deps, typ string) bool {
	return deps.CreateHook != nil && deps.CreateHook.Watches(typ)
}

// createHookError is a [CreateHook.EntityCreated] failure, written after
// the entity row.
type createHookError struct{ err error }

func (e createHookError) Error() string { return e.err.Error() }
func (e createHookError) Unwrap() error { return e.err }

// runCreateHook runs deps.CreateHook for e, created on view, and queues its
// afterCommit on the commit list of ctx.
func runCreateHook(ctx context.Context, deps Deps, view store.Store, e *entity.Entity) error {
	if !watchesCreate(deps, e.Type) {
		return nil
	}
	after, err := deps.CreateHook.EntityCreated(ctx, view, e)
	if err != nil {
		return createHookError{err}
	}
	if after == nil {
		return nil
	}
	if !onCommit(ctx, after) {
		slog.InfoContext(ctx, "entitymanager: create hook follow-up left to its owner: no commit list",
			"entity", e.ID)
	}
	return nil
}

// isCreateHookError reports whether err is a create hook's failure. The
// entity row was written before it, so on a store that cannot roll back
// the row stays, and createCore returns the entity with the error.
func isCreateHookError(err error) bool {
	var hookErr createHookError
	return errors.As(err, &hookErr)
}

// createOp is one call of a create entry point ([Manager.CreateEntity],
// [CreateLinked], [CreateBatch]) that runs its own transaction. It owns the
// commit list of that transaction, and runs it after the entry point's own
// follow-ups, so a hook's follow-up sees the entity as its automations left
// it.
type createOp struct {
	commits *commitList
}

// beginCreate starts a create of entities of types on ctx. It returns the
// ctx the create runs on and the operation, whose finish runs the commit
// list. A ctx inside a transaction joins it: the opener's list and
// preparation apply, and the operation's finish does nothing.
func beginCreate(ctx context.Context, deps Deps, types ...string) (context.Context, *createOp, error) {
	op := &createOp{}
	if store.TxView(ctx) != nil {
		return ctx, op, nil
	}
	for _, typ := range types {
		if watchesCreate(deps, typ) {
			var err error
			if ctx, err = deps.CreateHook.PrepareCreate(ctx); err != nil {
				return nil, nil, err
			}
			break
		}
	}
	op.commits = &commitList{}
	return withTxScope(ctx, &txScope{commits: op.commits}), op, nil
}

// finish runs the commit list after a create that wrote something that
// stayed: one that committed, or one that failed on a store that cannot
// roll back. ctx is the caller's ctx, the one the follow-ups run on.
func (op *createOp) finish(ctx context.Context, deps Deps, err error) {
	if op.commits == nil || (err != nil && store.TxRollsBack(deps.Store)) {
		return
	}
	op.commits.run(ctx)
}

// commitList holds what runs once a create's transaction has committed.
type commitList struct{ fns []func(context.Context) }

// onCommit queues fn on the commit list of ctx. It reports false when ctx
// carries none.
func onCommit(ctx context.Context, fn func(context.Context)) bool {
	s := scopeOf(ctx)
	if s.commits == nil {
		return false
	}
	s.commits.fns = append(s.commits.fns, fn)
	return true
}

// run runs the queued functions on ctx, in order, and empties the list.
func (l *commitList) run(ctx context.Context) {
	fns := l.fns
	l.fns = nil
	for _, fn := range fns {
		fn(ctx)
	}
}

// txScope is everything this package keeps on a ctx for the transaction
// that ctx belongs to: the commit list, and the audit records a
// [TxAudit] sink appends at the end ([txBatch]) or a best-effort sink
// receives after the commit ([heldAudit]). It sits under one key so that
// [afterCommitCtx] can leave all of it with one value.
type txScope struct {
	commits *commitList
	batch   *txBatch
	held    *heldAudit
}

type txScopeKey struct{}

// noScope is the scope of a ctx outside any transaction.
var noScope = &txScope{}

func withTxScope(ctx context.Context, s *txScope) context.Context {
	return context.WithValue(ctx, txScopeKey{}, s)
}

// scopeOf returns the scope of ctx. Nil: never returned.
func scopeOf(ctx context.Context) *txScope {
	if s, ok := ctx.Value(txScopeKey{}).(*txScope); ok && s != nil {
		return s
	}
	return noScope
}

// afterCommitCtx returns ctx, taken inside a transaction, for work that
// runs once that transaction has committed: the same principal, request
// and other values, outside the transaction. Each package keeps its
// transaction state under one key, and this leaves both: the store's view
// and this package's [txScope].
func afterCommitCtx(ctx context.Context) context.Context {
	return withTxScope(store.ContextOutsideTx(ctx), noScope)
}
