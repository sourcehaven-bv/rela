package acl

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// globalRoles computes the global role set of a principal whose membership
// closure is already resolved (see [IdentityResolver]). It unions
// Assignments[m] for every member m, the roles mapped from the principal's
// verified assertion claims, and the "everyone" role if declared.
//
// It is a pure function of its arguments and the policy: no graph traffic.
// [Declarative.ForIdentity] calls it once per Request.
func (d *Declarative) globalRoles(p principal.Principal, members []string) GlobalRoles {
	var attrs []RoleAttribution
	seen := map[attrKey]bool{}
	add := func(role string, source Source) {
		k := attrKey{Role: role, Source: source}
		if seen[k] {
			return
		}
		seen[k] = true
		attrs = append(attrs, RoleAttribution{Role: role, Source: source})
	}

	policy := d.policy
	for _, m := range members {
		role, ok := policy.Assignments[m]
		if !ok {
			continue
		}
		if _, defined := policy.Roles[role]; !defined {
			continue
		}
		if m == p.User {
			add(role, Source{Kind: SourceGlobal})
		} else {
			add(role, Source{Kind: SourceGroup, Group: m})
		}
	}
	// Roles asserted by a verified identity assertion, mapped through the
	// operator's allowlist. Like the everyone role below, these land without
	// a graph walk — the principal need not exist as an entity at all, which
	// is what makes an SSO-provisioned user's first request work.
	//
	// The claim value is NOT a role name: it only ever selects an entry the
	// operator wrote in asserted_role_assignments, so an IdP cannot name a
	// rela role the deployment did not choose to grant. Matching is exact
	// after TrimSpace (Validate rejects blank keys); an undeclared target
	// role is dropped silently, matching the Assignments guard above.
	for _, claim := range p.Roles() {
		claim = strings.TrimSpace(claim)
		for _, role := range policy.AssertedRoles[claim] {
			if _, defined := policy.Roles[role]; !defined {
				continue
			}
			add(role, Source{Kind: SourceAsserted, Claim: claim})
		}
	}
	if _, ok := policy.Roles[EveryoneRole]; ok {
		add(EveryoneRole, Source{Kind: SourceGlobal})
	}
	return GlobalRoles{Attributions: attrs, Members: slices.Clone(members)}
}

// computeForEntity computes the per-entity attribution set: cached
// globals plus local-role probes (direct edges from any group-set
// member to the entity, and when inherit_roles_through is configured,
// per-ancestor probes).
//
// The membership closure comes from the [Identity] the Request was opened
// with; only the ancestor walk runs here. The two are crossed against the
// policy's role-relations:
//
//	principal.User                            target entity
//	     │                                          │
//	     │ member-of                                │ inherit_roles_through
//	     ▼ (resolved Identity, see identity.go)     ▼ (ancestors, depth-capped)
//	┌─────────┐                                ┌─────────┐
//	│ Members │                                │ Chain   │  (entity + ancestors)
//	│  set M  │                                │  C      │
//	└────┬────┘                                └────┬────┘
//	     │                                          │
//	     └──────────────┬───────────────────────────┘
//	                    ▼
//	       for each role-relation rel ∈ policy.RoleRelations (sorted):
//	       for each (m ∈ M, target ∈ C):
//	           if graph.HasEdge(m, rel, target):
//	               attribute Confers(rel) with Source picked from
//	               {Local, LocalViaGroup, LocalViaAncestor,
//	                LocalViaGroupAndAncestor} by (m==user?, target==entity?)
//
// The cross-product is bounded by depthCap on both walks, the
// member-of and inherit_roles_through closures usually being tiny
// (under 10 nodes each in practice). Graph errors on either walk
// abort the surrounding loop — under-attribution is safer than
// over-granting.
func (r *Request) computeForEntity(ctx context.Context, entityID string) []RoleAttribution {
	globals := r.Globals()
	if local, ok := primedLocalRoles(ctx, r, entityID); ok {
		return mergeAttributions(globals.Attributions, local)
	}
	chain := r.ancestors(ctx, entityID)
	local := r.localRoles(entityID, chain, globals.Members, func(member, relType, target string) bool {
		return r.d.graph.HasEdge(ctx, member, relType, target)
	})
	return mergeAttributions(globals.Attributions, local)
}

// localRoles returns the roles conferred on the principal for entityID by
// role-relation edges from a member to an entity on chain (entityID first,
// then its inherit_roles_through ancestors), deduplicated, in the resolver's
// deterministic order. hasEdge answers one (member, relType, target) probe;
// the per-entity path asks the graph, a primed page asks the edges it loaded
// in one batch ([PrimeLocalRoles]), and both share this walk so they cannot
// disagree about which roles an edge confers or in what order.
func (r *Request) localRoles(
	entityID string, chain, members []string, hasEdge func(member, relType, target string) bool,
) []RoleAttribution {
	policy := r.d.policy
	var out []RoleAttribution
	seen := map[attrKey]bool{}
	// RR-MBK0: iterate role-relation types in sorted order so the local-
	// role attributions land in deterministic order — Decision.Attributions
	// otherwise reflects Go's randomized map iteration, making
	// formatDeniedSummary output non-reproducible.
	for _, relType := range r.conferringRelations() {
		role := policy.RoleRelations[relType].Confers
		for _, member := range members {
			for _, target := range chain {
				if !hasEdge(member, relType, target) {
					continue
				}
				src := buildLocalSource(member, r.principal.User, target, entityID, relType)
				a := RoleAttribution{Role: role, Source: src}
				if k := attrKey(a); !seen[k] {
					seen[k] = true
					out = append(out, a)
				}
			}
		}
	}
	return out
}

// conferringRelations returns, sorted, the role-relation types whose
// conferred role the policy defines: the only ones the local-role walk
// probes.
func (r *Request) conferringRelations() []string {
	policy := r.d.policy
	relTypes := make([]string, 0, len(policy.RoleRelations))
	for relType, def := range policy.RoleRelations {
		if def.Confers == "" {
			continue
		}
		if _, defined := policy.Roles[def.Confers]; !defined {
			continue
		}
		relTypes = append(relTypes, relType)
	}
	sort.Strings(relTypes)
	return relTypes
}

// mergeAttributions appends local to globals, dropping any (role, source)
// pair already present. globals is not modified.
func mergeAttributions(globals, local []RoleAttribution) []RoleAttribution {
	out := append([]RoleAttribution(nil), globals...)
	seen := make(map[attrKey]bool, len(out))
	for _, a := range out {
		seen[attrKey(a)] = true
	}
	for _, a := range local {
		if k := attrKey(a); !seen[k] {
			seen[k] = true
			out = append(out, a)
		}
	}
	return out
}

// buildLocalSource picks the right Source variant for an
// (group-member, ancestor, role-relation) match.
func buildLocalSource(member, principalUser, target, entityID, relType string) Source {
	inheritedAncestor := target != entityID
	viaGroup := member != principalUser
	switch {
	case !inheritedAncestor && !viaGroup:
		return Source{Kind: SourceLocal, Relation: relType}
	case !inheritedAncestor && viaGroup:
		return Source{Kind: SourceLocalViaGroup, Group: member, Relation: relType}
	case inheritedAncestor && !viaGroup:
		return Source{Kind: SourceLocalViaAncestor, Ancestor: target, Relation: relType}
	default:
		return Source{Kind: SourceLocalViaGroupAndAncestor, Group: member, Ancestor: target, Relation: relType}
	}
}

// ancestors returns entityID plus any ancestors reachable via the
// configured inherit_roles_through relation types (union across all
// listed types). depthCap-bounded BFS with visited-set termination.
// The entity itself is always at index 0.
func (r *Request) ancestors(ctx context.Context, entityID string) []string {
	if entityID == "" || len(r.d.policy.InheritRolesThrough) == 0 {
		return []string{entityID}
	}
	visited := map[string]bool{entityID: true}
	order := []string{entityID}
	frontier := []string{entityID}
	for depth := 0; depth < depthCap && len(frontier) > 0; depth++ {
		var next []string
		for _, n := range frontier {
			for _, relType := range r.d.policy.InheritRolesThrough {
				tos, err := r.d.graph.OutgoingRelations(ctx, n, relType)
				if err != nil {
					return order
				}
				for _, to := range tos {
					if visited[to] {
						continue
					}
					visited[to] = true
					order = append(order, to)
					next = append(next, to)
				}
			}
		}
		frontier = next
	}
	return order
}

// HoldsPermission reports whether the principal holds the given global
// named permission. It is the exported entry point for consumers outside
// the write-side delegate-X gate — e.g. the data-entry history read path
// gating deleted-entity history on [PermHistoryRead]. Permissions are
// global-only by design (see [holdsPermission]).
func (r *Request) HoldsPermission(_ context.Context, perm string) bool {
	return r.holdsPermission(perm)
}

// HoldsPermissionForEntity reports whether the principal holds the given named
// permission for the entity identified by entityID, considering BOTH global
// roles AND roles conferred locally by graph relations to that entity (and its
// ancestors via inherit_roles_through). This is the subject-aware sibling of
// [Request.HoldsPermission]: it lets a caller express "the assignee may perform
// X on their own entity" where the assignee role is conferred by an ownership
// relation, not a global assignment.
//
// It is the entry point the statemachine transition guard uses (TKT-E4LW2):
// the guard permission is a coarse capability noun (e.g. "establish"), and the
// per-subject scope comes from whether a role-relation confers the granting
// role for this entity — resolved here, not baked into the permission.
func (r *Request) HoldsPermissionForEntity(ctx context.Context, entityID, perm string) bool {
	return r.grantsPermission(r.computeForEntity(ctx, entityID), perm)
}

// holdsPermission reports whether any role in the principal's global
// role set grants the given permission. Used by the delegate-X gate
// on role-relation writes; permissions are global-only by design.
func (r *Request) holdsPermission(perm string) bool {
	return r.grantsPermission(r.Globals().Attributions, perm)
}

// grantsPermission reports whether any role in attrs grants perm.
func (r *Request) grantsPermission(attrs []RoleAttribution, perm string) bool {
	// A client ceiling withholding this permission short-circuits every role,
	// and closes the wildcard gap (a role holding `permissions: ["*"]` keeps
	// its wildcard through roleFor under a pure denial).
	if !r.ceiling.permitsPermission(perm) {
		return false
	}
	for _, a := range attrs {
		role, ok := r.roleFor(a.Role)
		// coverage-ignore: defensive: attrs come from Globals/computeForEntity which already skip roles not defined in
		// policy.Roles, so ok is
		// always true here
		if !ok {
			continue
		}
		if slices.Contains(role.Permissions, perm) {
			return true
		}
	}
	return false
}
