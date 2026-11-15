package appbuild

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// lowerQueryCondition rewrites a query condition on entityType as a store
// query fragment for the principal gate authorizes (TKT-I39Y7N4). A list
// query scope and a list or nav status `condition:` both come here. The
// outcome is one of:
//
//   - ok=false: the condition does not lower exactly
//     ([queryplan.LowerCondition]), or a traversal cannot be gated for this
//     principal. The caller keeps its Go path, which gives the same rows or
//     the same error.
//   - ok=true, empty=true: a positive traversal is denied outright and none
//     is refused, so no row can match. The Go path gives that answer too,
//     after reading the type.
//   - ok=true: frag carries Props and Related to AND onto the read query.
//
// A denied NEGATED traversal drops out: `related(...)` is false on every row
// for a principal who may not read the far type, so `not related(...)` is
// true on every row and narrows nothing. Every traversal is gated here, per
// request; nothing is cached on the caller, which is shared across
// principals. The identity comes from ctx, which the caller's binder stamped.
func lowerQueryCondition(
	ctx context.Context, meta *metamodel.Metamodel, prog *predicate.Program, entityType string,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
) (frag store.GraphQuery, empty, ok bool) {
	if gate == nil {
		return store.GraphQuery{}, false, false
	}
	props, terms, ok := lowerConditionTerms(ctx, meta, prog, entityType)
	if !ok {
		return store.GraphQuery{}, false, false
	}
	frag.Props = props
	for _, term := range terms {
		hop := term.Hop
		pred, err := gate(ctx, entityType, hop)
		switch {
		case errors.Is(err, acl.ErrTraversalDenied):
			// Keep gating the rest: a refused term must still surface as
			// the error the Go path reports, so "denied" wins only if
			// nothing is refused.
			if !term.Negate {
				empty = true
			}
			continue
		case err != nil, pred == nil:
			// A nil predicate would match every row; fall back rather than
			// widen or dereference it.
			return store.GraphQuery{}, false, false
		case pred.Negate:
			// The gate's predicate is a positive edge test. One that is
			// already negated has a meaning this lowering does not
			// compose; leave it to the Go path.
			return store.GraphQuery{}, false, false
		}
		rel := store.DirectedRelation{Incoming: hop.Incoming, Pred: *pred}
		rel.Pred.Negate = term.Negate
		frag.Related = append(frag.Related, rel)
	}
	if empty {
		return store.GraphQuery{}, true, true
	}
	return frag, false, true
}

// lowerConditionTerms lowers prog exactly ([queryplan.LowerCondition]) with
// the query identity stamped on ctx, and resolves each distinct traversal
// to the hop it walks. It gates nothing: a caller gates the terms, or hands
// them to a reader that gates them itself. ok=false: prog does not lower,
// or a traversal has no hop.
func lowerConditionTerms(
	ctx context.Context, meta *metamodel.Metamodel, prog *predicate.Program, entityType string,
) (props []store.PropPredicate, terms []visibility.PageTraversal, ok bool) {
	if prog == nil || meta == nil {
		return nil, nil, false
	}
	var identity string
	if id, bound := predicatefns.QueryIdentityFrom(ctx); bound {
		identity = id.ID()
	}
	lowered, ok := queryplan.LowerCondition(prog, meta, entityType, identity)
	if !ok {
		return nil, nil, false
	}
	seen := make(map[string]bool, len(lowered.Traversals))
	for _, term := range lowered.Traversals {
		// A repeated term adds nothing to a conjunction; skip it as
		// relresolve.Answer does, rather than gate and join it twice. The
		// key includes the sign, so `related(x) and not related(x)` keeps
		// both and matches nothing, as in Go.
		key := term.Spec.Key()
		if term.Negated {
			key = "not " + key
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		hop, err := relresolve.Hop(meta, entityType, term.Spec)
		if err != nil {
			return nil, nil, false
		}
		terms = append(terms, visibility.PageTraversal{Hop: hop, Negate: term.Negated})
	}
	return lowered.Props, terms, true
}

// QueryConditions lowers a caller's query condition for a page read, for an
// identity-bearing consumer outside the data-entry API: MCP list_entities
// (TKT-Z099FGY). It holds only the metamodel. It gates nothing: the
// traversals it returns are hops, which the page reader gates itself with
// the caller's traversal gate ([visibility.PageTraversal]), the same gate
// the [GatedReadBundle]'s Traversals binder answers the condition with in
// Go. One value serves every request.
//
// It also stamps the query identity, because `current_user` must bind to
// the same caller in the store and in Go; a consumer that lowers a
// condition is the one that needs it.
type QueryConditions struct {
	meta *metamodel.Metamodel
}

// StampQueryIdentity stamps the caller's query identity onto ctx, so
// `current_user` lowers and evaluates against the caller. It is the
// derivation query scopes use ([QueryScopeResolver.StampQueryIdentity]).
func (q QueryConditions) StampQueryIdentity(ctx context.Context) (context.Context, error) {
	return nextActionRequestScope(ctx)
}

// Lower rewrites prog, compiled against entityType, as a page filter: props
// on the STORED row, and the traversals for the reader to gate.
// ok=false: prog does not lower exactly, and the caller answers it in Go.
//
// Every entity field prog reads is the property of one of props: a
// condition that reads a field no prop names, or uses the entity record
// whole other than as a traversal subject, does not lower. So a caller
// whose Go path judges REDACTED rows decides whether the store may answer by
// checking the visibility of the props' properties alone. The zero value
// lowers nothing.
func (q QueryConditions) Lower(
	ctx context.Context, prog *predicate.Program, entityType string,
) (props []store.PropPredicate, traversals []visibility.PageTraversal, ok bool) {
	props, traversals, ok = lowerConditionTerms(ctx, q.meta, prog, entityType)
	if !ok {
		return nil, nil, false
	}
	named := make(map[string]bool, len(props))
	for _, p := range props {
		named[p.Property] = true
	}
	for _, attr := range prog.Attributes("entity") {
		if !named[attr] {
			return nil, nil, false
		}
	}
	return props, traversals, true
}
