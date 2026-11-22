package appbuild

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
//   - ok=true: frag carries Props, Related and Where to AND onto the read
//     query. A traversal inside Where that is denied reads as false there,
//     and a Where those denials make false on every row is empty=true.
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
	lowered, ok := lowerConditionTerms(ctx, meta, prog, entityType)
	if !ok {
		return store.GraphQuery{}, false, false
	}
	frag.Props = lowered.props
	if lowered.where != nil {
		// Gated first, so a refused hop in it wins over a denied term below.
		w, err := visibility.GateCond(ctx, gate, entityType, *lowered.where)
		if err != nil {
			return store.GraphQuery{}, false, false
		}
		// A tree its denied hops made constant needs no query: false
		// matches no row, true narrows nothing.
		switch v, known := w.Constant(); {
		case !known:
			frag.Where = &w
		case !v:
			empty = true
		}
	}
	for _, term := range lowered.terms {
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

// conditionTerms is a lowered condition whose traversals are hops, not yet
// gated: the AND of props, terms and where.
type conditionTerms struct {
	props []store.PropPredicate
	terms []visibility.PageTraversal
	where *visibility.PageCond
}

// lowerConditionTerms lowers prog exactly ([queryplan.LowerCondition]) with
// the query identity stamped on ctx, and resolves each distinct traversal
// to the hop it walks. It gates nothing: a caller gates the terms, or hands
// them to a reader that gates them itself. ok=false: prog does not lower,
// or a traversal has no hop.
func lowerConditionTerms(
	ctx context.Context, meta *metamodel.Metamodel, prog *predicate.Program, entityType string,
) (out conditionTerms, ok bool) {
	out, err := lowerConditionTermsExplained(ctx, meta, prog, entityType)
	return out, err == nil
}

// lowerConditionTermsExplained is [lowerConditionTerms] that says why a
// condition does not lower.
func lowerConditionTermsExplained(
	ctx context.Context, meta *metamodel.Metamodel, prog *predicate.Program, entityType string,
) (conditionTerms, error) {
	if prog == nil || meta == nil {
		return conditionTerms{}, errors.New("no condition to lower")
	}
	var identity string
	if id, bound := predicatefns.QueryIdentityFrom(ctx); bound {
		identity = id.ID()
	}
	lowered, err := queryplan.LowerConditionExplained(prog, meta, entityType, identity)
	if err != nil {
		return conditionTerms{}, err
	}
	var out conditionTerms
	if lowered.Where != nil {
		w, err := pageCond(meta, entityType, *lowered.Where)
		if err != nil {
			return conditionTerms{}, err
		}
		out.where = &w
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
			return conditionTerms{}, fmt.Errorf("related(...): %w", err)
		}
		out.terms = append(out.terms, visibility.PageTraversal{Hop: hop, Negate: term.Negated})
	}
	out.props = lowered.Props
	return out, nil
}

// pageCond resolves the traversals of c to the hops they walk.
func pageCond(meta *metamodel.Metamodel, entityType string, c queryplan.LoweredCond) (visibility.PageCond, error) {
	out := visibility.PageCond{Op: c.Op, Prop: c.Prop}
	if c.Op == store.CondRelated {
		hop, err := relresolve.Hop(meta, entityType, c.Traversal)
		if err != nil {
			return visibility.PageCond{}, fmt.Errorf("related(...): %w", err)
		}
		out.Hop = hop
		return out, nil
	}
	for _, a := range c.Args {
		pa, err := pageCond(meta, entityType, a)
		if err != nil {
			return visibility.PageCond{}, err
		}
		out.Args = append(out.Args, pa)
	}
	return out, nil
}

// QueryConditions lowers a caller's query condition for a page read, for an
// identity-bearing consumer outside the data-entry API: MCP list_entities
// (TKT-Z099FGY). It holds only the metamodel. It gates nothing: the
// traversals it returns are hops, which the page reader gates itself with
// the caller's traversal gate ([visibility.PageTraversal]). One value
// serves every request.
//
// It also stamps the query identity, because `current_user` must bind to
// the caller; a consumer that lowers a condition is the one that needs it.
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
// on the STORED row, the traversals for the reader to gate, and a Where tree
// for the rest. The filter is the AND of all three; a caller that drops one
// widens it. An error means prog does not lower exactly and names the part
// that does not ([queryplan.UnsupportedConditionError] for most).
//
// Every entity field prog reads is the property of a predicate in props or
// in where: a condition that reads a field none names, or uses the entity
// record whole other than as a traversal subject, does not lower. So a
// reader whose callers expect the REDACTED row judged can guard exactly
// those properties, which [visibility.ScriptReader.ListEntityPage] does.
// The zero value lowers nothing.
func (q QueryConditions) Lower(
	ctx context.Context, prog *predicate.Program, entityType string,
) (visibility.PageQuery, error) {
	lowered, err := lowerConditionTermsExplained(ctx, q.meta, prog, entityType)
	if err != nil {
		return visibility.PageQuery{}, err
	}
	filter := visibility.PageQuery{Props: lowered.props, Traversals: lowered.terms, Where: lowered.where}
	named := map[string]bool{}
	for _, p := range visibilityProps(filter) {
		named[p.Property] = true
	}
	for _, attr := range prog.Attributes("entity") {
		if !named[attr] {
			return visibility.PageQuery{}, &queryplan.UnsupportedConditionError{
				Construct: "entity." + attr,
				Reason:    "read in a way the store cannot compare",
			}
		}
	}
	return filter, nil
}

// visibilityProps returns every property predicate of a page filter.
func visibilityProps(f visibility.PageQuery) []store.PropPredicate {
	if f.Where == nil {
		return f.Props
	}
	return append(slices.Clone(f.Props), f.Where.Props()...)
}
