package acl

import (
	"context"
	"slices"
	"sort"
	"strings"
)

// computeGlobals walks the configured membership relation (default
// `member-of`, see [Policy.MembershipRelation]) from the principal and
// unions in Assignments[m] for every member m, plus any roles mapped from
// the principal's verified assertion claims, plus the "everyone" role if
// declared.
//
// Called once per Request via Globals(); the result is cached for the
// lifetime of the Request.
func (r *Request) computeGlobals(ctx context.Context) GlobalRoles {
	members := r.walkMembers(ctx)

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

	policy := r.d.policy
	for _, m := range members {
		role, ok := policy.Assignments[m]
		if !ok {
			continue
		}
		if _, defined := policy.Roles[role]; !defined {
			continue
		}
		if m == r.principal.User {
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
	for _, claim := range r.principal.Roles() {
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
	return GlobalRoles{Attributions: attrs, Members: members}
}

// walkMembers returns {principal.User} ∪ the transitive closure over
// the configured membership relation (default `member-of`, see
// [Policy.MembershipRelation]).
// Visited-set primary; depthCap as backstop. Errors from the graph
// abort the surrounding walk — under-counting members is safer than
// over-granting, but a partial-data principal-resolution is worse
// than failing loud.
func (r *Request) walkMembers(ctx context.Context) []string {
	user := r.principal.User
	if user == "" {
		return nil
	}
	visited := map[string]bool{user: true}
	order := []string{user}
	frontier := []string{user}
	for depth := 0; depth < depthCap && len(frontier) > 0; depth++ {
		var next []string
		for _, n := range frontier {
			tos, err := r.d.graph.OutgoingRelations(ctx, n, r.d.policy.EffectiveMembershipRelation())
			if err != nil {
				// Abort the walk loud rather than silently undercount.
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
		frontier = next
	}
	return order
}

// computeForEntity computes the per-entity attribution set: cached
// globals plus local-role probes (direct edges from any group-set
// member to the entity, and when inherit_roles_through is configured,
// per-ancestor probes).
//
// The resolver runs two independent graph walks and crosses them
// against the policy's role-relations:
//
//	principal.User                            target entity
//	     │                                          │
//	     │ member-of                                │ inherit_roles_through
//	     ▼ (walkMembers, depth-capped)              ▼ (ancestors, depth-capped)
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
	globals := r.Globals(ctx)
	out := append([]RoleAttribution(nil), globals.Attributions...)
	seen := map[attrKey]bool{}
	for _, a := range out {
		seen[attrKey(a)] = true
	}
	add := func(role string, source Source) {
		k := attrKey{Role: role, Source: source}
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, RoleAttribution{Role: role, Source: source})
	}

	chain := r.ancestors(ctx, entityID)
	policy := r.d.policy
	// RR-MBK0: iterate role-relation types in sorted order so the local-
	// role attributions land in deterministic order — Decision.Attributions
	// otherwise reflects Go's randomized map iteration, making
	// formatDeniedSummary output non-reproducible.
	relTypes := make([]string, 0, len(policy.RoleRelations))
	for relType := range policy.RoleRelations {
		relTypes = append(relTypes, relType)
	}
	sort.Strings(relTypes)
	for _, relType := range relTypes {
		def := policy.RoleRelations[relType]
		role := def.Confers
		if role == "" {
			continue
		}
		if _, defined := policy.Roles[role]; !defined {
			continue
		}
		for _, member := range globals.Members {
			for _, target := range chain {
				if !r.d.graph.HasEdge(ctx, member, relType, target) {
					continue
				}
				add(role, buildLocalSource(member, r.principal.User, target, entityID, relType))
			}
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
func (r *Request) HoldsPermission(ctx context.Context, perm string) bool {
	return r.holdsPermission(ctx, perm)
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
func (r *Request) holdsPermission(ctx context.Context, perm string) bool {
	return r.grantsPermission(r.Globals(ctx).Attributions, perm)
}

// grantsPermission reports whether any role in attrs grants perm.
func (r *Request) grantsPermission(attrs []RoleAttribution, perm string) bool {
	for _, a := range attrs {
		role, ok := r.d.policy.Roles[a.Role]
		if !ok {
			continue
		}
		if slices.Contains(role.Permissions, perm) {
			return true
		}
	}
	return false
}
