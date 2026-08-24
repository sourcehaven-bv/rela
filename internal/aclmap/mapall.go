package aclmap

import (
	"context"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// MapAllResult is the whole-graph inventory: every enumerated principal's
// effective access, aggregated by type with per-entity exceptions — the
// union of one [MapPrincipalResult] per principal. It fixes neither the
// principal nor the entity (the O(P·E) case), which is why it is a
// separate slice from map --principal.
//
// EveryoneBaseline is reported ONCE here, not repeated inside every
// principal: the built-in everyone role grants the same access to all
// principals, so folding it into each principal's baseline would bloat
// the report and obscure which principals hold a PERSONAL grant. Each
// per-principal entry therefore reports only its non-everyone access;
// EveryoneOnly on a principal is preserved so "cut off except for
// everyone" is still visible.
type MapAllResult struct {
	SchemaVersion int      `json:"schema_version"`
	Verbs         []string `json:"verbs"`
	// EveryoneBaseline lists the types (and verbs) the everyone role grants
	// every principal, reported once. Sorted by type.
	EveryoneBaseline []EveryoneType `json:"everyone_baseline,omitempty"`
	// Principals is every enumerated principal that holds ANY access (even
	// everyone-only), sorted by principal ID.
	Principals []MapPrincipalResult `json:"principals"`
	// PrincipalCount and GrantSourceCount are the headline summary: how many
	// principals were enumerated and how many distinct (principal, type,
	// verb, route) grant-sources exist across the graph — the number an
	// operator reads to gauge the whole-graph posture.
	PrincipalCount   int `json:"principal_count"`
	GrantSourceCount int `json:"grant_source_count"`
}

// EveryoneType is the everyone role's grant on one entity type: the verbs
// it confers on every principal.
type EveryoneType struct {
	Type  string   `json:"type"`
	Verbs []string `json:"verbs"`
}

// MapAll computes the whole-graph effective-access inventory: it
// enumerates the principal universe and runs [Engine.MapPrincipal] for
// each, then lifts the everyone baseline out to a single top-level entry.
//
// This is the O(P·E) slice. The per-type baseline inside MapPrincipal is
// O(types) via the empty-entity probe, so the dominant cost is the
// exception scan (entities carrying a local/inherited edge). See the
// package cost note; a reverse index bounding the exception scan to
// edge-reachable entities is the optimization this slice's ticket scopes.
func (e *Engine) MapAll(
	ctx context.Context, verbFilter acl.Verb, typeFilter string, entityTypes []string,
) (*MapAllResult, error) {
	verbs, err := resolveVerbs(verbFilter)
	if err != nil {
		return nil, err
	}

	candidates, err := e.enumeratePrincipals(ctx)
	if err != nil {
		return nil, err
	}

	// Merge by EFFECTIVE principal — a single human can surface as several
	// candidate keys (raw UPN + resolved entity). Running MapPrincipal on
	// each and keying the result by its resolved Principal collapses them.
	//
	// First-key-wins is safe because access depends only on the effective
	// User: the CLI never sets verified role claims and RawUser is
	// audit-only, so every candidate key resolving to one effective
	// principal computes IDENTICAL access. (This deliberately differs from
	// who-can, which UNIONS routes across duplicate keys via mergeRoutes;
	// here there is nothing to union.) A blank key yields an empty-Principal
	// result — skip it so a malformed key adds no phantom row.
	byPrincipal := map[string]*MapPrincipalResult{}
	for _, raw := range candidates {
		res, err := e.MapPrincipal(ctx, raw, verbFilter, typeFilter, entityTypes)
		if err != nil {
			return nil, err
		}
		if res.Principal == "" {
			continue
		}
		if _, seen := byPrincipal[res.Principal]; !seen {
			byPrincipal[res.Principal] = res
		}
	}

	result := &MapAllResult{
		SchemaVersion:    schemaVersion,
		Verbs:            verbStrings(verbs),
		EveryoneBaseline: e.everyoneBaseline(verbs, mapTypes(typeFilter, entityTypes)),
	}

	ids := make([]string, 0, len(byPrincipal))
	for id := range byPrincipal {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		pr := byPrincipal[id]
		stripEveryoneBaseline(pr)
		result.Principals = append(result.Principals, *pr)
		result.GrantSourceCount += grantSources(pr)
	}
	result.PrincipalCount = len(result.Principals)
	return result, nil
}

// everyoneBaseline lists the everyone role's type-wide grants, once, for
// the top-level report. It consults the policy directly (EveryoneGrants),
// independent of any principal, so it is exact even for a type with zero
// entities.
func (e *Engine) everyoneBaseline(verbs []acl.Verb, types []string) []EveryoneType {
	var out []EveryoneType
	for _, typ := range types {
		var granted []string
		for _, verb := range verbs {
			if e.resolver.EveryoneGrants(verb, typ).Granted {
				granted = append(granted, string(verb))
			}
		}
		if len(granted) > 0 {
			sort.Strings(granted)
			out = append(out, EveryoneType{Type: typ, Verbs: granted})
		}
	}
	return out
}

// stripEveryoneBaseline removes the everyone-role routes from a
// per-principal result's type baselines, since MapAll reports the everyone
// baseline once at the top level. A verb whose baseline becomes empty is
// dropped; a type left with no baseline and no exceptions is dropped.
// EveryoneOnly is left untouched — it still signals "no personal grant".
func stripEveryoneBaseline(pr *MapPrincipalResult) {
	kept := pr.Types[:0]
	for _, ta := range pr.Types {
		for verb, routes := range ta.Baseline {
			filtered := routes[:0]
			for _, r := range routes {
				if r.Role == acl.EveryoneRole && r.Kind == acl.SourceGlobal.String() {
					continue
				}
				filtered = append(filtered, r)
			}
			if len(filtered) == 0 {
				delete(ta.Baseline, verb)
			} else {
				ta.Baseline[verb] = filtered
			}
		}
		if len(ta.Baseline) > 0 || len(ta.Exceptions) > 0 {
			kept = append(kept, ta)
		}
	}
	pr.Types = kept
}

// grantSources counts the distinct grant-sources in a per-principal
// result — every route across every type baseline and every entity
// exception. It is the per-principal contribution to the whole-graph
// GrantSourceCount headline.
func grantSources(pr *MapPrincipalResult) int {
	n := 0
	for _, ta := range pr.Types {
		for _, routes := range ta.Baseline {
			n += len(routes)
		}
		for _, ex := range ta.Exceptions {
			for _, routes := range ex.Extra {
				n += len(routes)
			}
		}
	}
	return n
}
