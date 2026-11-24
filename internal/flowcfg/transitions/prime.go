package transitions

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// primedTraversals is one page-level bind of the `related(...)` traversals of
// every transition `when:` of one entity type (TKT-FJG779Q). It is carried on
// ctx by [Transitions.PrimeTraversals] and consumed by bindTraversals.
type primedTraversals struct {
	owner      *Transitions
	entityType string
	ids        map[string]bool
	bound      func(rowID string) predicate.TraversalFunc
	err        error
}

type primedTraversalsKey struct{}

// PrimeTraversals binds the `related(...)` traversals of every transition
// `when:` of entityType for all ids in one call, and returns a ctx that
// carries the answer. [Transitions.Performable] on one of those rows, under
// the returned ctx, reads the primed answer instead of binding again, so a
// page of rows costs one bind rather than one per row.
//
// It returns ctx unchanged when no `when:` of the type traverses, when no
// binder is wired, or when ids is empty. A bind error is carried and fails
// the precondition of every primed row, as a per-row bind error would.
func (s *Transitions) PrimeTraversals(ctx context.Context, entityType string, ids []string) context.Context {
	if s.Empty() || s.traversals == nil || len(ids) == 0 {
		return ctx
	}
	var whens []*predicate.Program
	for _, typeName := range s.propType[entityType] {
		m := s.machines[typeName]
		if m == nil {
			continue
		}
		m.eachBranch(func(_ string, b headlessBranch) {
			if b.when != nil && len(b.when.Traversals()) > 0 {
				whens = append(whens, b.when)
			}
		})
	}
	if len(whens) == 0 {
		return ctx
	}
	p := &primedTraversals{owner: s, entityType: entityType, ids: make(map[string]bool, len(ids))}
	for _, id := range ids {
		p.ids[id] = true
	}
	p.bound, p.err = s.traversals.Bind(ctx, entityType, ids, whens...)
	return context.WithValue(ctx, primedTraversalsKey{}, p)
}

// primed returns the page-level answer for the row id of entityType, if ctx
// carries one bound by s that covers it.
func (s *Transitions) primed(ctx context.Context, entityType, id string) (edgeTraversal, bool) {
	p, ok := ctx.Value(primedTraversalsKey{}).(*primedTraversals)
	if !ok || p.owner != s || p.entityType != entityType || !p.ids[id] {
		return edgeTraversal{}, false
	}
	if p.err != nil {
		return edgeTraversal{err: p.err}, true
	}
	return edgeTraversal{fn: p.bound(id)}, true
}
