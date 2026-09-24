package dataentry

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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

	// Filter applies a resolved scope to a batch of rows; see
	// [QueryScopeFilter].
	Filter(
		ctx context.Context, scope QueryScopeHandle, entityType string, headers []store.EntityHeader,
		gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
	) ([]store.EntityHeader, error)

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
// names one. A nil cfg or meta is an ERROR, which is a different case — see
// below.
func viewQueryScope(
	fn QueryScopeResolverFunc, cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
	entityType, name string,
) (resolved resolvedQueryScope, err error) {
	if fn == nil {
		// No resolver was ever wired, so no scope can be declared: config
		// validation refuses any view naming one, and a type's scopes cannot
		// compile without a compiler. Unscoped is the truthful answer.
		return resolvedQueryScope{}, nil
	}
	if cfg == nil || meta == nil {
		// NOT the same case. A nil metamodel does not mean "no scopes are
		// declared", it means "I cannot tell whether any are" — and the
		// unscoped read this used to return is the one answer that is wrong
		// either way. Refuse instead; every caller maps this to a 500, which
		// is what a half-built App deserves.
		return resolvedQueryScope{}, errors.New(
			"dataentry: cannot resolve query scope without config and metamodel")
	}
	resolver, problems := fn(cfg, meta)
	if resolver == nil {
		if len(problems) > 0 {
			// Compilation failed at startup and was reported there; refusing
			// here too keeps a broken scope from degrading to unfiltered.
			return resolvedQueryScope{}, errors.New("dataentry: query scopes failed to compile")
		}
		return resolvedQueryScope{}, nil
	}
	scope, props, ok := resolver.Resolve(entityType, name)
	if !ok {
		// Config validation refuses an undeclared name at load, so reaching
		// here means the config and the compiled scopes disagree — a
		// hot-reload race, say. Refuse rather than serve the unscoped set.
		return resolvedQueryScope{}, fmt.Errorf(
			"%w: %q is not declared on %q", errBadQueryScope, name, entityType)
	}
	if scope == nil {
		// No scope applies, but still carry the binder: the identity it
		// stamps is request-scoped, not scope-scoped, and a view's
		// `condition:` needs it whether or not a scope resolved. Dropping it
		// here is what made `is_current_user(...)` in a condition fail with
		// ErrNoCurrentUser on an unscoped list. Scope stays nil, so every
		// "is there a scope" check still reads this as unscoped.
		return resolvedQueryScope{Bind: resolver.BindRequest}, nil
	}
	return resolvedQueryScope{
		Scope: scope, Props: props, Filter: resolver.Filter, Bind: resolver.BindRequest,
	}, nil
}

// resolvedQueryScope is one view's scope, ready to apply.
//
// The four fields travel together because they are one contract: the program
// was compiled by the resolver that supplied Filter, and Filter can only evaluate
// it against an identity Bind stamped. Returning them separately invited
// exactly the bug that motivated this struct — Bind was declared on the seam,
// implemented at the composition root, adapted through two layers, and then
// never called, so `is_current_user(...)` in a scope failed every page of its
// type with ErrNoCurrentUser instead of scoping it.
//
// The zero value means "no scope", which every field check treats as unscoped.
type resolvedQueryScope struct {
	Scope  QueryScopeHandle
	Props  []store.PropPredicate
	Filter QueryScopeFilter
	Bind   func(context.Context) (context.Context, error)
}

// bind stamps the request-scoped evaluation state a scope needs, once, before
// any row is evaluated.
//
// Called even when the scope does not reference the identity: the resolver
// decides per program whether the binding is needed, and the cost of a
// needless bind is one store lookup, while the cost of a missing one is a
// failed page. Never call it per row — resolution reads the store.
func (r resolvedQueryScope) bind(ctx context.Context) (context.Context, error) {
	if r.Scope == nil || r.Bind == nil {
		return ctx, nil
	}
	return r.Bind(ctx)
}

// bindQueryIdentity stamps the request identity for ANY narrowing that may
// name `current_user`, not only a resolved scope.
//
// Separate from [resolvedQueryScope.bind] because the two answer different
// questions. bind asks "does this scope need binding", and correctly says no
// when there is no scope. This asks "does this REQUEST need an identity", and
// a list whose `condition:` names current_user needs one with no scope in
// play — the case that returned 500 before.
//
// Binding twice is safe: the resolver honors an existing stamp and refuses
// only a conflicting one, so a caller that already bound loses nothing.
func bindQueryIdentity(ctx context.Context, r resolvedQueryScope) (context.Context, error) {
	if r.Bind == nil {
		return ctx, nil
	}
	return r.Bind(ctx)
}

// QueryScopeParam is the query parameter that selects a query scope on the
// read API: `?query_scope=archief`.
//
// Absent means the entity type's `default` scope (if it declares one). The
// reserved `all` withdraws that default and reads everything.
//
// Client-supplied like `?world=`, and for the same reason: the list endpoint
// is keyed by entity TYPE, so the server cannot tell which configured list is
// on screen. The SPA attaches it from the view's `query_scope:` — see the
// queryParams computed in EntityList.vue and boardParams in KanbanView.vue,
// each guarded so a view declaring no scope sends no parameter rather than an
// empty one, since "" is a name that does not resolve.
//
// That the client is a required participant is worth stating plainly: a
// `query_scope:` in data-entry.yaml validates, and derives an index, whether
// or not anything sends it. It shipped inert once for exactly that reason.
const QueryScopeParam = "query_scope"

// errBadQueryScope classifies every query-scope failure a REQUEST can cause,
// so the list handler can answer 400 rather than letting it fall through to
// the pipeline's catch-all 500.
//
// The distinction matters to whoever is debugging: a mistyped `?query_scope=`
// is the caller's error and is fixed by changing the URL, whereas the
// catch-all reports "Free-text search failed" — naming a subsystem the
// request never reached. Scope names are operator-authored configuration, not
// secrets (see the CLAUDE.md rule), so the detail may name the scope.
var errBadQueryScope = errors.New("invalid query_scope")

// errQueryScopeDuplicated reports a repeated `?query_scope=`.
var errQueryScopeDuplicated = fmt.Errorf(
	"%w: query_scope may be given at most once", errBadQueryScope)

// queryScopeParam extracts the requested scope name from a request's query.
//
// A REPEATED parameter is an error rather than first-wins: Get() would take
// the first, so `?query_scope=all&query_scope=archief` would read everything
// under a request that also asked for the archive — a client-side
// param-append bug becoming a silently wider read. Same defense, same reason,
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
	Filter(
		ctx context.Context, scope any, entityType string, headers []store.EntityHeader,
		gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
	) ([]store.EntityHeader, error)
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
	Filter(
		ctx context.Context, scope any, entityType string, headers []store.EntityHeader,
		gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
	) ([]store.EntityHeader, error)
	BindRequest(ctx context.Context) (context.Context, error)
}] struct{ r R }

func (a adaptedQueryScopes[R]) Resolve(
	entityType, name string,
) (QueryScopeHandle, []store.PropPredicate, bool) {
	return a.r.Resolve(entityType, name)
}

func (a adaptedQueryScopes[R]) Filter(
	ctx context.Context, scope QueryScopeHandle, entityType string, headers []store.EntityHeader,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
) ([]store.EntityHeader, error) {
	return a.r.Filter(ctx, scope, entityType, headers, gate, match)
}

func (a adaptedQueryScopes[R]) BindRequest(ctx context.Context) (context.Context, error) {
	return a.r.BindRequest(ctx)
}
