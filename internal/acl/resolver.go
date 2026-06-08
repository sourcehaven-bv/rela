package acl

import (
	"context"
	"sort"
)

// computeGlobals walks member-of from the principal and unions in
// Assignments[m] for every member m, plus the "everyone" role if
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
	if _, ok := policy.Roles[EveryoneRole]; ok {
		add(EveryoneRole, Source{Kind: SourceGlobal})
	}
	return GlobalRoles{Attributions: attrs, Members: members}
}

// walkMembers returns {principal.User} ∪ transitive member-of closure.
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
			tos, err := r.d.graph.OutgoingRelations(ctx, n, "member-of")
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

// holdsPermission reports whether any role in the principal's global
// role set grants the given permission. Used by the delegate-X gate
// on role-relation writes; permissions are global-only by design.
func (r *Request) holdsPermission(ctx context.Context, perm string) bool {
	for _, a := range r.Globals(ctx).Attributions {
		role, ok := r.d.policy.Roles[a.Role]
		if !ok {
			continue
		}
		for _, p := range role.Permissions {
			if p == perm {
				return true
			}
		}
	}
	return false
}
