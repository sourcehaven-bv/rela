package aclmap

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// WhoCan reports every principal who can perform verb on the entity
// entityID, each with the route(s) that grant it, plus a single global
// entry when the built-in everyone role grants the verb.
//
// It gates on entity existence first (a missing entity errors with
// [ErrEntityNotFound] rather than returning a misleading global-only
// reader set), resolves the entity's type from the store (the resolver
// does not know it), then:
//
//   - Records the everyone grant once, globally, when policy grants the
//     verb to everyone (directly or via "*").
//   - Records any asserted-claim grants in their own Conditional section.
//     These are NOT enumerable as principals (the holders live in the IdP,
//     not the graph) and are deliberately not folded into the everyone
//     grant — see [ConditionalGrant].
//   - Enumerates the principal universe (resolvable user entities ∪
//     assignment keys ∪ membership-relation sources ∪ role-relation
//     sources) and, for each, asks the resolver for the routes granting
//     the verb on this entity. Read is decided by the runtime read path
//     (acl.Request.AccessRoutes gates read on PermitsRead), so a
//     reader is never dropped.
//
// Principals are returned sorted by ID; each principal's routes are
// sorted deterministically. The everyone role is never listed as a
// principal (it is the global entry).
func (e *Engine) WhoCan(ctx context.Context, verb acl.Verb, entityID string) (*WhoCanResult, error) {
	ent, err := e.src.GetEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, entityID)
		}
		return nil, fmt.Errorf("aclmap: load entity %q: %w", entityID, err)
	}
	entityType := ent.Type

	result := &WhoCanResult{
		SchemaVersion: schemaVersion,
		Verb:          string(verb),
		Entity:        entityID,
		EntityType:    entityType,
	}

	eg := e.resolver.EveryoneGrants(verb, entityType)
	result.Everyone = Everyone{Granted: eg.Granted, Wildcard: eg.Wildcard}

	// Reported in their own section, never merged into Everyone: the holders
	// of a claim are not enumerable from the graph, so claiming they are
	// "everyone" would be a false — and dangerously reassuring — statement.
	for _, ag := range e.resolver.AssertedGrants(verb, entityType) {
		result.Conditional = append(result.Conditional, ConditionalGrant{
			Claim: ag.Claim, Role: ag.Role, Wildcard: ag.Wildcard,
		})
	}

	candidates, err := e.enumeratePrincipals(ctx)
	if err != nil {
		return nil, err
	}

	// Merge by EFFECTIVE principal: a single human can appear as several
	// candidate keys (a raw-UPN assignment key AND the resolved user
	// entity it maps to), which must collapse to one row with a unioned
	// route set — otherwise the same principal is reported twice and the
	// diff artifact this feeds is corrupt (RR-XC2NTO).
	byPrincipal := map[string]*PrincipalAccess{}
	var order []string
	for _, raw := range candidates {
		access, ok, err := e.accessFor(ctx, raw, verb, entityType, entityID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		existing, seen := byPrincipal[access.Principal]
		if !seen {
			acc := access
			byPrincipal[access.Principal] = &acc
			order = append(order, access.Principal)
			continue
		}
		existing.Routes = mergeRoutes(existing.Routes, access.Routes)
		if existing.Raw == "" {
			existing.Raw = access.Raw
		}
	}
	sort.Strings(order)
	for _, id := range order {
		result.Principals = append(result.Principals, *byPrincipal[id])
	}
	return result, nil
}

// mergeRoutes unions two route sets, dropping exact duplicates, and
// returns them re-sorted so the output stays deterministic regardless of
// which candidate key surfaced each route.
func mergeRoutes(a, b []Route) []Route {
	seen := make(map[Route]struct{}, len(a)+len(b))
	out := make([]Route, 0, len(a)+len(b))
	for _, rs := range [][]Route{a, b} {
		for _, r := range rs {
			if _, dup := seen[r]; dup {
				continue
			}
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessRoute(out[i], out[j]) })
	return out
}

// accessFor resolves one raw principal key, asks the resolver for the
// routes granting verb on the entity. The bool is false (with a zero
// PrincipalAccess) when the principal has no non-everyone route — a
// normal "not a grantee" outcome, distinct from an error. A raw key that
// resolves to a user entity ID uses the resolved ID as the effective
// principal and records the raw value when it differs.
func (e *Engine) accessFor(
	ctx context.Context, raw string, verb acl.Verb, entityType, entityID string,
) (PrincipalAccess, bool, error) {
	user := strings.TrimSpace(raw)
	if user == "" {
		// A blank assignment key can't be a principal (ForPrincipal would
		// reject it as unstamped); skip it, not an error.
		return PrincipalAccess{}, false, nil
	}
	rawShown := ""
	// Resolve raw → entity ID when principal_property is configured. An
	// ambiguous or errored lookup fails the report loud rather than
	// silently mis-attributing.
	if id, err := e.resolver.ResolvePrincipal(ctx, raw); err != nil {
		return PrincipalAccess{}, false, err
	} else if id != "" && id != raw {
		user = id
		rawShown = raw
	}

	req, err := e.resolver.ForPrincipal(
		principal.Principal{User: user, Tool: principal.ToolCLI, RawUser: rawShown})
	if err != nil {
		return PrincipalAccess{}, false, fmt.Errorf("aclmap: open resolver for %q: %w", user, err)
	}

	attrs, err := req.AccessRoutes(ctx, verb, entityType, entityID)
	if err != nil {
		return PrincipalAccess{}, false, fmt.Errorf("aclmap: access routes for %q: %w", user, err)
	}
	if len(attrs) == 0 {
		return PrincipalAccess{}, false, nil
	}
	return PrincipalAccess{
		Principal: user,
		Raw:       rawShown,
		Routes:    routesFromAttributions(attrs),
	}, true, nil
}

// routesFromAttributions converts resolver attributions to the wire
// Route shape (terminal facts) and sorts them deterministically.
func routesFromAttributions(attrs []acl.RoleAttribution) []Route {
	routes := make([]Route, 0, len(attrs))
	for _, a := range attrs {
		routes = append(routes, Route{
			Kind:     a.Source.Kind.String(),
			Role:     a.Role,
			Group:    a.Source.Group,
			Ancestor: a.Source.Ancestor,
			Relation: a.Source.Relation,
			Claim:    a.Source.Claim,
		})
	}
	sort.Slice(routes, func(i, j int) bool { return lessRoute(routes[i], routes[j]) })
	return routes
}

func lessRoute(a, b Route) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Role != b.Role {
		return a.Role < b.Role
	}
	if a.Group != b.Group {
		return a.Group < b.Group
	}
	if a.Ancestor != b.Ancestor {
		return a.Ancestor < b.Ancestor
	}
	if a.Relation != b.Relation {
		return a.Relation < b.Relation
	}
	// Mirrors the Claim tiebreak in acl.lessSource — without it two asserted
	// routes differing only by claim compare equal and the artifact ordering
	// becomes input-dependent.
	return a.Claim < b.Claim
}
