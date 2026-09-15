package dataentry

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// QueryScopeResolver resolves a view's declared scope into the pieces
// [scopedHeaders] needs to apply it.
//
// The consumer-side seam for the scope compiler: this package must not import
// it (arch-lint keeps the condition engine above the data-entry app), so the
// composition root supplies an implementation — the same arrangement as
// [NextActionMatcherFunc].
//
// Resolving by (entityType, name) rather than by view id keeps the seam
// independent of which config surface asked: a list, a kanban and a future
// caller all resolve the same scope to the same program.
type QueryScopeResolver interface {
	// Resolve returns the scope a view should apply. name is the view's
	// `query_scope:`, empty meaning "use the type's default".
	//
	// ok=false means the name is not declared and the caller must REFUSE.
	// It must never be reported for a resolvable scope, because the caller
	// cannot distinguish "unknown" from "unscoped" any other way, and
	// guessing yields the unfiltered set.
	Resolve(entityType, name string) (scope QueryScopeHandle, props []store.PropPredicate, ok bool)

	// Evaluate applies a resolved scope to one row.
	Evaluate(ctx context.Context, scope QueryScopeHandle, entityType, id string, props map[string]any) (bool, error)

	// BindRequest stamps request-scoped evaluation state — today the
	// identity `current_user` resolves to — onto ctx ONCE per request,
	// before any row is evaluated. Mirrors NextActionRequestScope.
	BindRequest(ctx context.Context) (context.Context, error)
}

// QueryScopeResolverFunc builds a [QueryScopeResolver] for the current config
// and metamodel, plus one message per problem.
//
// Takes cfg and meta rather than a prebuilt resolver because both reload at
// runtime (the config watcher), so a resolver captured at wiring time would
// serve a stale scope after an operator edits schema.yaml.
type QueryScopeResolverFunc func(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (QueryScopeResolver, []string)

// SetQueryScopeResolver injects the compiler backing `query_scopes:`.
//
// Rejects nil for the reason [App.SetNextActionMatchers] does: a silently
// absent resolver would leave every scope unapplied, showing rows the
// operator's default explicitly hides — and nothing on screen would say so.
func (a *App) SetQueryScopeResolver(fn QueryScopeResolverFunc) error {
	if fn == nil {
		return errors.New("dataentry.SetQueryScopeResolver: func must be non-nil")
	}
	a.queryScopes = fn
	return nil
}

// viewQueryScope resolves the scope for one view, returning the fields to
// copy onto a [scopeRequest].
//
// A package function rather than a method to keep App under its plimsoll load
// line — the precedent TKT-WRLDAPI records.
//
// Nil: a nil resolver func (no wiring) yields an unscoped read, which is
// correct: a deployment whose composition root never supplied a resolver has
// no compiled scopes, and config validation has already refused any view that
// names one.
func viewQueryScope(
	fn QueryScopeResolverFunc, cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
	entityType, name string,
) (scope QueryScopeHandle, props []store.PropPredicate, eval QueryScopeEvaluator, err error) {
	if fn == nil || cfg == nil || meta == nil {
		return nil, nil, nil, nil
	}
	resolver, problems := fn(cfg, meta)
	if resolver == nil {
		if len(problems) > 0 {
			// Compilation failed at startup and was reported there; refusing
			// here too keeps a broken scope from degrading to unfiltered.
			return nil, nil, nil, errors.New("dataentry: query scopes failed to compile")
		}
		return nil, nil, nil, nil
	}
	scope, props, ok := resolver.Resolve(entityType, name)
	if !ok {
		// Config validation refuses an undeclared name at load, so reaching
		// here means the config and the compiled scopes disagree — a
		// hot-reload race, say. Refuse rather than serve the unscoped set.
		return nil, nil, nil, errors.New("dataentry: query scope " + name + " is not declared on " + entityType)
	}
	if scope == nil {
		return nil, nil, nil, nil
	}
	return scope, props, resolver.Evaluate, nil
}

// QueryScopeParam is the query parameter that selects a query scope on the
// read API: `?query_scope=archief`.
//
// Absent means the entity type's `default` scope (if it declares one). The
// reserved `all` withdraws that default and reads everything.
//
// Client-supplied like `?world=`, and for the same reason: the list endpoint
// is keyed by entity TYPE, so the server cannot tell which configured list is
// on screen. The SPA attaches it from the list's `query_scope:` at the call
// sites that have one, never blanket-applied.
const QueryScopeParam = "query_scope"

// errQueryScopeDuplicated reports a repeated `?query_scope=`.
var errQueryScopeDuplicated = errors.New(
	"query_scope may be given at most once")

// queryScopeParam extracts the requested scope name from a request's query.
//
// A REPEATED parameter is an error rather than first-wins: Get() would take
// the first, so `?query_scope=all&query_scope=archief` would read everything
// under a request that also asked for the archive — a client-side
// param-append bug becoming a silently wider read. Same defence, same reason,
// as resolveWorld's errWorldDuplicated.
func queryScopeParam(query map[string][]string) (string, error) {
	values := query[QueryScopeParam]
	if len(values) > 1 {
		return "", errQueryScopeDuplicated
	}
	if len(values) == 0 {
		return "", nil
	}
	return values[0], nil
}

// AdaptQueryScopes turns a composition-root scope resolver into the seam this
// package declares.
//
// Generic over the concrete type because internal/appbuild cannot name
// [QueryScopeResolver] — dataentry's own tests import appbuild, so the edge
// would close a cycle (arch-lint forbids it). The constraint restates the
// interface structurally, so a root that satisfies the shape satisfies the
// seam without either package importing the other's types.
func AdaptQueryScopes[R interface {
	Resolve(entityType, name string) (any, []store.PropPredicate, bool)
	Evaluate(ctx context.Context, scope any, entityType, id string, props map[string]any) (bool, error)
	BindRequest(ctx context.Context) (context.Context, error)
}](build func(*dataentryconfig.Config, *metamodel.Metamodel) (R, []string)) QueryScopeResolverFunc {
	return func(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) (QueryScopeResolver, []string) {
		r, problems := build(cfg, meta)
		if len(problems) > 0 {
			return nil, problems
		}
		// A typed-nil R wrapped in a non-nil interface is the classic trap
		// NextActionMatchers documents; compare against the zero value so a
		// builder that returned nothing yields a nil seam, not a live one
		// that panics on first use.
		var zero R
		if any(r) == any(zero) {
			return nil, nil
		}
		return adaptedQueryScopes[R]{r: r}, nil
	}
}

type adaptedQueryScopes[R interface {
	Resolve(entityType, name string) (any, []store.PropPredicate, bool)
	Evaluate(ctx context.Context, scope any, entityType, id string, props map[string]any) (bool, error)
	BindRequest(ctx context.Context) (context.Context, error)
}] struct{ r R }

func (a adaptedQueryScopes[R]) Resolve(
	entityType, name string,
) (QueryScopeHandle, []store.PropPredicate, bool) {
	return a.r.Resolve(entityType, name)
}

func (a adaptedQueryScopes[R]) Evaluate(
	ctx context.Context, scope QueryScopeHandle, entityType, id string, props map[string]any,
) (bool, error) {
	return a.r.Evaluate(ctx, scope, entityType, id, props)
}

func (a adaptedQueryScopes[R]) BindRequest(ctx context.Context) (context.Context, error) {
	return a.r.BindRequest(ctx)
}
