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
	if prog == nil || gate == nil {
		return store.GraphQuery{}, false, false
	}
	var identity string
	if id, bound := predicatefns.QueryIdentityFrom(ctx); bound {
		identity = id.ID()
	}
	lowered, ok := queryplan.LowerCondition(prog, meta, entityType, identity)
	if !ok {
		return store.GraphQuery{}, false, false
	}
	frag.Props = lowered.Props
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
			return store.GraphQuery{}, false, false
		}
		pred, err := gate(ctx, entityType, hop)
		switch {
		case errors.Is(err, acl.ErrTraversalDenied):
			// Keep gating the rest: a refused term must still surface as
			// the error the Go path reports, so "denied" wins only if
			// nothing is refused.
			if !term.Negated {
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
		rel.Pred.Negate = term.Negated
		frag.Related = append(frag.Related, rel)
	}
	if empty {
		return store.GraphQuery{}, true, true
	}
	return frag, false, true
}
