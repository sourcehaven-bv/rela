package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// QueryScopes compiles the metamodel's `query_scopes:` and adapts them to the
// seam internal/dataentry declares.
//
// Lives at the composition root for the reason [NextActionMatchers] does: the
// condition engine sits above the data-entry app, so dataentry takes a seam
// and this bridges the two.
//
// Takes config + metamodel rather than returning a prebuilt resolver because
// both reload at runtime — a resolver captured at boot would keep applying a
// scope the operator has since edited.
//
// Returns a STRUCTURAL match for dataentry.QueryScopeResolver rather than the
// named interface: internal/dataentry's tests import this package, so naming
// its types here would close an import cycle (arch-lint forbids the edge).
// The composition root adapts it — see dataentry.AdaptQueryScopes.
// cfg is unused today and kept because the seam's shape is the contract: the
// same signature serves NextActionMatchers, which does read the config, and a
// resolver that later needs it (a per-view override, say) must not change
// every wiring site to get it.
func QueryScopes(
	_ *dataentryconfig.Config, meta *metamodel.Metamodel,
) (resolver *QueryScopeResolver, problems []string) {
	compiled, err := scopes.Compile(meta)
	if err != nil {
		return nil, []string{err.Error()}
	}
	return &QueryScopeResolver{
		compiled: compiled,
		meta:     meta,
		eval:     predicatefns.NewEvaluator(meta),
	}, nil
}

// QueryScopeResolver adapts scopes.Compiled to dataentry's seam.
type QueryScopeResolver struct {
	compiled scopes.Compiled
	meta     *metamodel.Metamodel
	eval     *predicatefns.Evaluator
}

// Resolve looks up a scope by name, or the type's default when name is empty.
//
// ok=false ONLY for a name that is genuinely undeclared. "No scope applies"
// is reported as (nil, nil, true) — a distinction the caller depends on,
// since it must refuse the first and read everything for the second.
func (r *QueryScopeResolver) Resolve(
	entityType, name string,
) (any, []store.PropPredicate, bool) {
	var (
		prog *predicate.Program
		ok   bool
	)
	if name == "" {
		prog, ok = r.compiled.Default(entityType)
		if !ok {
			// No default declared: an unscoped read, not an error.
			return nil, nil, true
		}
	} else if prog, ok = r.compiled.Lookup(entityType, name); !ok {
		return nil, nil, false
	}
	if prog == nil {
		// The implicit `all` scope resolves to no predicate.
		return nil, nil, true
	}
	// Lower the store-safe conjuncts so the read narrows in SQL. This is a
	// SUPERSET: whatever does not lower (negation, ordered comparison,
	// disjunction) is left to the Go-side Evaluate, which stays
	// authoritative.
	//
	// The empty identity is deliberate and load-bearing. It makes
	// ConditionPrefilters skip every `current_user` conjunct, so an identity
	// scope pushes NOTHING and applyScope decides it alone — the pushdown
	// stays a strict superset. Passing a resolved identity here would be the
	// natural way to earn the index listScopeIndexProperties derives for
	// `mijn:`, and it is the one change to make carefully: this resolver is
	// built per (cfg, meta), NOT per request, so an identity held on the
	// struct would be one principal's identity reused in another principal's
	// query. Thread it as an argument from a request-bound caller or not at
	// all.
	props := queryplan.ConditionPrefilters(prog, r.meta, []string{entityType}, "")
	return prog, props, true
}

// Filter applies a resolved scope to a batch of rows.
//
// Any `related(...)` in the scope is answered first, once per distinct
// traversal over all candidates, then each row is evaluated against those
// answers. The answers live in this call only; see [relresolve.Answers].
func (r *QueryScopeResolver) Filter(
	ctx context.Context, scope any, entityType string, headers []store.EntityHeader,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
) ([]store.EntityHeader, error) {
	prog, ok := scope.(*predicate.Program)
	if !ok {
		// The handle came from Resolve, so a mismatch means two resolvers
		// were mixed. Fail rather than skip: skipping serves unscoped rows.
		return nil, fmt.Errorf("appbuild: query scope handle has unexpected type %T", scope)
	}
	ids := make([]string, len(headers))
	for i, h := range headers {
		ids[i] = h.ID
	}
	answers, err := relresolve.Answer(ctx, r.meta, gate, match, entityType, prog.Traversals(), ids)
	if err != nil {
		return nil, err
	}
	out := make([]store.EntityHeader, 0, len(headers))
	for _, h := range headers {
		if h.Type != entityType {
			// The traversals were resolved and answered from entityType; a
			// row of another type would be evaluated against the wrong walk.
			return nil, fmt.Errorf("appbuild: query scope on %q got a row of type %q", entityType, h.Type)
		}
		ok, err := r.eval.MatchesWithTraversals(ctx, prog, h.Type, h.ID, h.Properties, answers.For(h.ID))
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, h)
		}
	}
	return out, nil
}

// BindRequest stamps the query identity onto ctx once per request, so a scope
// using `current_user` evaluates against the caller.
//
// Reuses nextActionRequestScope: the identity semantics are identical, and
// two derivations that could disagree about who the caller is would be worse
// than one shared with a different feature.
func (r *QueryScopeResolver) BindRequest(ctx context.Context) (context.Context, error) {
	return nextActionRequestScope(ctx)
}

// Lower rewrites a resolved scope as store predicates, for a list the store
// can then page by itself (TKT-XKCNCL). The outcome is one of:
//
//   - ok=false: the scope does not lower exactly, or a traversal cannot be
//     gated for this principal. The caller takes the [QueryScopeResolver.Filter]
//     path, which reproduces the same rows or the same error.
//   - ok=true, empty=true: a traversal is denied outright and none is
//     refused, so no row can match. This is the answer Filter gives, reached
//     without reading the type.
//   - ok=true: frag carries Props and Related to AND onto the read query.
//
// Every traversal is gated here, per request, by the caller's gate; nothing
// is cached on the resolver, which is shared across principals. The identity
// comes from ctx, which BindRequest has stamped.
func (r *QueryScopeResolver) Lower(
	ctx context.Context, scope any, entityType string,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
) (frag store.GraphQuery, empty, ok bool) {
	prog, isProg := scope.(*predicate.Program)
	if !isProg || gate == nil {
		return store.GraphQuery{}, false, false
	}
	var identity string
	if id, bound := predicatefns.QueryIdentityFrom(ctx); bound {
		identity = id.ID()
	}
	lowered, ok := queryplan.LowerScope(prog, r.meta, entityType, identity)
	if !ok {
		return store.GraphQuery{}, false, false
	}
	frag.Props = lowered.Props
	seen := make(map[string]bool, len(lowered.Traversals))
	for _, spec := range lowered.Traversals {
		// A repeated traversal adds nothing to a conjunction; skip it as
		// relresolve.Answer does, rather than gate and join it twice.
		if seen[spec.Key()] {
			continue
		}
		seen[spec.Key()] = true
		hop, err := relresolve.Hop(r.meta, entityType, spec)
		if err != nil {
			return store.GraphQuery{}, false, false
		}
		pred, err := gate(ctx, entityType, hop)
		switch {
		case errors.Is(err, acl.ErrTraversalDenied):
			// Keep gating the rest: a refused term must still surface as
			// the error Filter reports, so "denied" wins only if nothing
			// is refused.
			empty = true
			continue
		case err != nil, pred == nil:
			// A nil predicate would match every row; fall back rather than
			// widen or dereference it.
			return store.GraphQuery{}, false, false
		}
		frag.Related = append(frag.Related, store.DirectedRelation{Incoming: hop.Incoming, Pred: *pred})
	}
	if empty {
		return store.GraphQuery{}, true, true
	}
	return frag, false, true
}
