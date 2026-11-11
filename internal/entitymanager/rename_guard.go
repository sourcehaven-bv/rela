package entitymanager

import (
	"context"
	"errors"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// RenameGuard refuses a rename that state kept outside the graph forbids.
// The flow runtime is one: its runs are keyed by subject id, so a subject an
// unfinished run holds keeps its id (TKT-XI7IDBS).
type RenameGuard interface {
	// CheckRename returns an error when the entity id must keep its id
	// now: one wrapping [ErrRenameBlocked] for a refusal, any other when
	// the guard could not decide. Either refuses the rename.
	//
	// The manager calls it inside the store transaction of a rename or a
	// conversion (and once, outside any, for a rename's dry run or a
	// conversion's preview). A writer that changes what the guard
	// reads under the same transaction is therefore ordered against the
	// rename.
	CheckRename(ctx context.Context, id string) error
}

// checkRenameGuard asks deps.RenameGuard whether id may be renamed.
//
// Nil: deps.RenameGuard accepted (nothing refuses).
func checkRenameGuard(ctx context.Context, deps Deps, id string) error {
	if deps.RenameGuard == nil {
		return nil
	}
	return deps.RenameGuard.CheckRename(ctx, id)
}

// recordRenameRefusal records a [RenameGuard] refusal of a new id for id as
// a denied write, as a field gate's refusal is. It runs after the
// transaction, so no audit write happens while the write lock is held.
//
// Nil: err accepted (nothing recorded); only an [ErrRenameBlocked] is.
func recordRenameRefusal(ctx context.Context, m *Manager, id string, err error) {
	if !errors.Is(err, ErrRenameBlocked) {
		return
	}
	m.deps.Audit.Record(audit.Stamp(ctx, audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     &audit.Subject{Kind: "entity", ID: id},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary:     "denied: " + err.Error(),
	}))
}
