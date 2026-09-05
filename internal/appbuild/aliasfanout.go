package appbuild

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// aliasFanout dispatches one identity-change notification to several
// subscribers.
//
// [entitymanager.Deps.AliasRewriter] is a single field because it was
// introduced for a single consumer (the CalDAV alias service). More than one
// subsystem now holds references BY ENTITY ID — comments are keyed on the
// target's id too — and both must learn about a rename. Fanning out here keeps
// that a wiring concern: entitymanager still depends on the two methods it
// calls, and does not grow a registry.
//
// Errors from every subscriber are joined rather than short-circuited, so one
// failing subscriber cannot mask another's failure or stop it being notified.
// The Manager logs what it gets back; see the AliasRewriter doc for why a
// post-write hook cannot fail the write that already happened.
type aliasFanout struct {
	subscribers []entitymanager.AliasRewriter
}

var _ entitymanager.AliasRewriter = (*aliasFanout)(nil)

// newAliasFanout returns a rewriter over the non-nil subscribers.
//
// Nil: returns nil when nothing subscribes, so the Manager's `if
// AliasRewriter == nil` fast path still applies and an unused hook costs
// nothing. Returns the single subscriber unwrapped when there is exactly one,
// keeping the common case free of an indirection.
func newAliasFanout(subs ...entitymanager.AliasRewriter) entitymanager.AliasRewriter {
	live := make([]entitymanager.AliasRewriter, 0, len(subs))
	for _, s := range subs {
		if s != nil {
			live = append(live, s)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	default:
		return &aliasFanout{subscribers: live}
	}
}

// EntityRenamed notifies every subscriber that an entity's id changed.
func (f *aliasFanout) EntityRenamed(ctx context.Context, oldID, newID string) error {
	var errs []error
	for _, s := range f.subscribers {
		if err := s.EntityRenamed(ctx, oldID, newID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// EntityDeleted notifies every subscriber that an entity left the graph.
func (f *aliasFanout) EntityDeleted(ctx context.Context, entityID string) error {
	var errs []error
	for _, s := range f.subscribers {
		if err := s.EntityDeleted(ctx, entityID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
