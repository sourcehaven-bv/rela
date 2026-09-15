package appbuild

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
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
func QueryScopes(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (*QueryScopeResolver, []string) {
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

// Evaluate applies a resolved scope to one row.
func (r *QueryScopeResolver) Evaluate(
	ctx context.Context, scope any, entityType, id string, props map[string]any,
) (bool, error) {
	prog, ok := scope.(*predicate.Program)
	if !ok {
		// The handle came from Resolve, so a mismatch means two resolvers
		// were mixed. Fail rather than skip: skipping serves unscoped rows.
		return false, fmt.Errorf("appbuild: query scope handle has unexpected type %T", scope)
	}
	return r.eval.MatchesAs(ctx, prog, entityType, id, props)
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
