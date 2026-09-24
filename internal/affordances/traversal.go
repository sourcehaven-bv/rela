package affordances

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// TraversalBinder answers the `related(...)` traversals of `when:` programs
// for a batch of entities of one type (TKT-205V2N). Defined at the consumer;
// the wiring site supplies a raw-store binder. A grant is an authorization
// decision, so it reads the graph as it is, like has_relation does, and not
// through the reader it is deciding for.
type TraversalBinder interface {
	Bind(ctx context.Context, entityType string, ids []string, progs ...*predicate.Program) (
		func(rowID string) predicate.TraversalFunc, error)
}

// Option configures [New].
type Option func(*PolicyResolver)

// WithTraversals wires the binder that answers `related(...)` in `when:`.
// Without it, a `when:` using related() is a compile error.
func WithTraversals(b TraversalBinder) Option {
	return func(r *PolicyResolver) { r.traversals = b }
}

// errNotAnswerable is why a traversing grant is denied without a store call.
var errNotAnswerable = errors.New("related(...) cannot be answered here")

// traversalMemo holds answers primed for a batch of rows, keyed by entity
// type then id. It lives on the ctx of ONE operation for ONE principal and is
// never shared beyond it.
type traversalMemo struct {
	mu   sync.Mutex
	rows map[string]map[string]rowAnswer
}

type rowAnswer struct {
	fn  predicate.TraversalFunc
	err error
}

type traversalMemoKey struct{}

// PrimeTraversals answers, for every row, the `related(...)` of every grant
// on its type, with one store query per distinct traversal and type, and
// returns a ctx carrying the answers. [PolicyResolver.FieldVerdicts],
// RelationVerdicts and TransitionVerdicts on that ctx then read them instead
// of querying per row. A row that was not primed is answered live, so priming
// is an optimization and never a correctness condition.
//
// Priming again on the returned ctx adds to the same memo, so a caller that
// works in chunks primes each chunk.
func (r *PolicyResolver) PrimeTraversals(ctx context.Context, rows []*entity.Entity) context.Context {
	if r.traversals == nil || len(r.traversalProgs) == 0 {
		return ctx
	}
	memo, ok := ctx.Value(traversalMemoKey{}).(*traversalMemo)
	if !ok {
		memo = &traversalMemo{rows: map[string]map[string]rowAnswer{}}
		ctx = context.WithValue(ctx, traversalMemoKey{}, memo)
	}
	idsByType := map[string][]string{}
	for _, e := range rows {
		if e == nil || e.Face != "" || e.ID == "" || len(r.traversalProgs[e.Type]) == 0 {
			continue // answered (or refused) live
		}
		idsByType[e.Type] = append(idsByType[e.Type], e.ID)
	}
	for typ, ids := range idsByType {
		bound, err := r.traversals.Bind(ctx, typ, ids, r.traversalProgs[typ]...)
		memo.mu.Lock()
		if memo.rows[typ] == nil {
			memo.rows[typ] = map[string]rowAnswer{}
		}
		for _, id := range ids {
			if err != nil {
				memo.rows[typ][id] = rowAnswer{err: err}
			} else {
				memo.rows[typ][id] = rowAnswer{fn: bound(id)}
			}
		}
		memo.mu.Unlock()
	}
	return ctx
}

// traversalFor returns the answers to e's grant traversals: primed if ctx
// carries them, otherwise from one live bind for e alone.
//
// It refuses a historical subject (the live graph does not describe the
// entity as of that version) and a row on a named face (the store answers
// from the default face's edges). The caller denies the grant, which is the
// closed direction.
func (r *PolicyResolver) traversalFor(ctx context.Context, e *entity.Entity) (predicate.TraversalFunc, error) {
	switch {
	case isHistoricalSubject(ctx):
		return nil, fmt.Errorf("%w: historical subject", errNotAnswerable)
	case e.Face != "":
		return nil, fmt.Errorf("%w: face %q", errNotAnswerable, e.Face)
	case r.traversals == nil:
		// coverage-ignore: invariant: New refuses a traversing `when:` without a binder
		return nil, fmt.Errorf("%w: no store is wired", errNotAnswerable)
	}
	if memo, ok := ctx.Value(traversalMemoKey{}).(*traversalMemo); ok {
		memo.mu.Lock()
		ans, hit := memo.rows[e.Type][e.ID]
		memo.mu.Unlock()
		if hit {
			return ans.fn, ans.err
		}
	}
	bound, err := r.traversals.Bind(ctx, e.Type, []string{e.ID}, r.traversalProgs[e.Type]...)
	if err != nil {
		return nil, err
	}
	return bound(e.ID), nil
}

// warnConditionallyVisible logs a load warning for a grant traversal that
// filters on a property the policy does not show to everyone. The grant is
// kept: it reads the raw graph, so its verdict is correct, but a principal
// who cannot see that property can learn its value from which grants they
// get. The same filter in a view or next-action condition is refused.
func (r *PolicyResolver) warnConditionallyVisible(
	roleName, entityType, block string, idx int, prog *predicate.Program,
) {
	for _, spec := range prog.Traversals() {
		hops, err := predicatefns.ResolveTraversal(r.meta, entityType, spec)
		if err != nil || len(hops) == 0 {
			continue // compile reports it
		}
		target := hops[len(hops)-1].Target
		for _, prop := range spec.PropNames() {
			if r.policy.ConditionallyVisible(target, prop) {
				slog.Warn("acl: a when: related() filters on a property not visible to every role",
					"grant", fmt.Sprintf("roles.%s.%s.%s[%d]", roleName, block, entityType, idx),
					"type", target, "property", prop)
			}
		}
	}
}
