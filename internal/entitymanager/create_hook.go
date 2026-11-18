package entitymanager

import (
	"context"
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
	// EntityCreated runs after the row and its audit record are written,
	// on view, the transaction's store; ctx carries the transaction too.
	// An error fails the create and, on a store that rolls back, undoes
	// it. afterCommit, when not nil, runs once the transaction has
	// committed, on the caller's ctx. It does not run when the create is
	// part of a transaction the manager did not open, which then holds no
	// list of them.
	EntityCreated(ctx context.Context, view store.Store, e *entity.Entity) (afterCommit func(context.Context), err error)
}

// watchesCreate reports whether deps.CreateHook watches creates of typ.
//
// Nil: deps.CreateHook accepted (nothing watches).
func watchesCreate(deps Deps, typ string) bool {
	return deps.CreateHook != nil && deps.CreateHook.Watches(typ)
}

// runCreateHook runs deps.CreateHook for e, created on view, and queues its
// afterCommit on the transaction ctx belongs to.
func runCreateHook(ctx context.Context, deps Deps, view store.Store, e *entity.Entity) error {
	if !watchesCreate(deps, e.Type) {
		return nil
	}
	after, err := deps.CreateHook.EntityCreated(ctx, view, e)
	if err != nil || after == nil {
		return err
	}
	if !onCommit(ctx, after) {
		slog.DebugContext(ctx, "entitymanager: create hook follow-up left to its owner: no commit list",
			"entity", e.ID)
	}
	return nil
}

// commitList holds what runs once the transaction [auditTx] opened has
// committed.
type commitList struct{ fns []func(context.Context) }

type commitListKey struct{}

// withCommitList returns ctx with a fresh commit list.
func withCommitList(ctx context.Context) (context.Context, *commitList) {
	l := &commitList{}
	return context.WithValue(ctx, commitListKey{}, l), l
}

// onCommit queues fn on the commit list of ctx. It reports false when ctx
// carries none.
func onCommit(ctx context.Context, fn func(context.Context)) bool {
	l, ok := ctx.Value(commitListKey{}).(*commitList)
	if !ok || l == nil {
		return false
	}
	l.fns = append(l.fns, fn)
	return true
}

// run runs the queued functions on ctx, in order.
func (l *commitList) run(ctx context.Context) {
	for _, fn := range l.fns {
		fn(ctx)
	}
}

// afterCommitCtx returns ctx, taken inside a transaction [auditTx] opened,
// for work that runs once that transaction has committed: the same
// principal, request and other values, without the transaction's view,
// its held audit records, its record batch or its commit list.
func afterCommitCtx(ctx context.Context) context.Context {
	ctx = store.ContextOutsideTx(ctx)
	ctx = context.WithValue(ctx, heldAuditKey{}, (*heldAudit)(nil))
	ctx = context.WithValue(ctx, txBatchKey{}, (*txBatch)(nil))
	return context.WithValue(ctx, commitListKey{}, (*commitList)(nil))
}
