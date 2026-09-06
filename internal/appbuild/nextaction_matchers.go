package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/conditionlint"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// nextActionMatcher adapts one compiled condition to the two seams dataentry
// declares: [nextaction.Matcher] (the authoritative per-candidate pass) and
// dataentry.ConditionPrefilterer (the store-side pre-filter). appbuild cannot
// name the latter — dataentry's tests import appbuild, so the edge would
// cycle — which is why dataentry's own test suite asserts the adapter
// satisfies it (TestNextAction_CurrentUserConditionEndToEnd).
//
// It is also where "who is the current user?" is answered for next actions.
// Both methods derive the identity from the principal on ctx through ONE
// function, queryIdentityFor, so the pre-filter and the Go pass cannot
// disagree about whose rows they are selecting.
type nextActionMatcher struct {
	m *conditionlint.NextActionMatcher
}

// Match evaluates the whole condition, with the identity stamped on ctx if
// the request boundary has not already done so.
func (w nextActionMatcher) Match(ctx context.Context, e *entity.Entity) (bool, error) {
	if _, stamped := predicatefns.QueryIdentityFrom(ctx); !stamped {
		if q, ok := queryIdentityFor(ctx); ok {
			ctx = predicatefns.WithQueryIdentity(ctx, q)
		}
	}
	ok, err := w.m.Match(ctx, e)
	if errors.Is(err, predicatefns.ErrNoCurrentUser) {
		// Translate to the engine's sentinel so the HTTP layer can name the
		// misconfiguration without importing the predicate engine.
		return false, fmt.Errorf("%w: %w", nextaction.ErrIdentityRequired, err)
	}
	return ok, err
}

// Prefilters lowers the condition's store-evaluable conjuncts for the types
// the source's query names (queryplan.ConditionPrefilters). With no
// resolvable identity the current-user conjuncts push nothing; Match then
// fails the same condition closed, so the request is refused rather than
// answered with the wrong rows.
func (w nextActionMatcher) Prefilters(
	ctx context.Context, meta *metamodel.Metamodel, types []string,
) []store.PropPredicate {
	if len(types) == 0 {
		return nil
	}
	prog, ok := w.m.Program(types[0])
	if !ok {
		return nil
	}
	var identity string
	if q, stamped := predicatefns.QueryIdentityFrom(ctx); stamped {
		identity = q.ID()
	} else if q, ok := queryIdentityFor(ctx); ok {
		identity = q.ID()
	}
	return queryplan.ConditionPrefilters(prog, meta, types, identity)
}

// queryIdentityFor derives the query identity from the principal on ctx.
//
// It does NOT call the ACL resolver: on the data-entry API the router has
// already run resolvePrincipalEntity, so principal.User IS the resolved user
// entity id when the policy has a user_entity_type and the principal matched
// one, with RawUser holding the original identifier. When no resolution
// happened, User is the raw identifier and that is what graph data can be
// compared against (predicatefns.QueryIdentity documents the fallback).
//
// Two states yield no identity, and both must: an unstamped ctx, and the
// "unknown" attribution placeholder the data-entry server stamps when it has
// no identity source. Neither is a user, and comparing either against
// `entity.assignee` would be a match against nobody at best and against an
// entity literally assigned to "unknown" at worst.
func queryIdentityFor(ctx context.Context) (predicatefns.QueryIdentity, bool) {
	p, ok := principal.Stamped(ctx)
	if !ok || p.User == "" || p.User == principal.Unknown {
		return predicatefns.QueryIdentity{}, false
	}
	q := predicatefns.QueryIdentity{Raw: p.User, Tool: p.Tool}
	if p.RawUser != "" && p.RawUser != p.User {
		q.EntityID = p.User
		q.Raw = p.RawUser
	}
	return q, true
}

// NextActionMatchers compiles the `condition:` of every next-action source,
// adapting conditionlint's compiler to the seam internal/dataentry declares.
//
// Lives at the composition root for the same reason the userstate backend
// does: the condition/policy engine sits above the data-entry app, so
// dataentry takes a seam and this bridges the two.
//
// Takes config + metamodel rather than returning a prebuilt lookup because
// both reload at runtime — a lookup captured at boot would keep evaluating a
// condition the operator has since edited.
func NextActionMatchers(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (lookup func(string) (nextaction.Matcher, bool), problems []string) {
	compiled, issues := conditionlint.NextActionMatchers(cfg, meta)
	if len(issues) > 0 || compiled == nil {
		return nil, issues
	}
	return func(id string) (nextaction.Matcher, bool) {
		m, ok := compiled(id)
		if !ok {
			// Returning m directly would hand back a non-nil interface
			// wrapping a nil *NextActionMatcher — the classic typed-nil trap.
			return nil, false
		}
		return nextActionMatcher{m: m}, true
	}, nil
}
