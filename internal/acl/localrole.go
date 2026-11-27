package acl

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LocalRolePredicates returns the relation predicates that match the entities
// on which the principal holds role through a role relation: an entity
// matches when ANY of them does. It is the store form of the local half of
// [Request.ForEntity]: an incoming edge of a type that confers role, from the
// principal or one of its groups, on the entity or on an ancestor reached
// through inherit_roles_through.
//
// The unconditional conferring types share one predicate. Each conditional
// type (`when:`, TKT-TKSCSBT) gets its own, carrying its condition bound to
// the request's read time; a type no edge can satisfy at that time is left
// out. These are the predicates readQuery builds for a conferred read grant,
// so a role held per entity is answered by the store the way the row gate is.
//
// ok is false when no role relation confers role, or when the principal has
// no member set. The role is then held through an edge on no entity. An
// empty member set must never become a predicate, because empty Endpoints
// means "any endpoint" to the store.
func (r *Request) LocalRolePredicates(role string) (preds []store.RelationPredicate, ok bool) {
	if _, defined := r.roleFor(role); !defined {
		return nil, false
	}
	var plain, conditional []string
	for relType, def := range r.d.policy.RoleRelations {
		if def.Confers != role {
			continue
		}
		switch _, live := r.conds.edge(relType); {
		case !live:
		case r.conds.conditional(relType):
			conditional = append(conditional, relType)
		default:
			plain = append(plain, relType)
		}
	}
	slices.Sort(plain)
	slices.Sort(conditional)
	if len(plain) > 0 {
		p, ok := conferredEdge(r.d.policy, r.Globals().Members, plain)
		if !ok {
			return nil, false
		}
		preds = append(preds, *p)
	}
	for _, relType := range conditional {
		p, ok := conferredEdge(r.d.policy, r.Globals().Members, []string{relType})
		if !ok {
			return nil, false
		}
		p.EdgeProps, _ = r.conds.edge(relType)
		preds = append(preds, *p)
	}
	return preds, len(preds) > 0
}

// conferredEdge returns the predicate that matches an entity with an incoming
// edge of one of relTypes from one of members, on the entity itself or on an
// ancestor reached through inherit_roles_through. It is the one place the
// conferred-role edge is spelled, shared by the row gate ([Request.ReadQuery])
// and [Request.LocalRolePredicates].
//
// ok is false for an empty members: empty Endpoints means "any endpoint" to
// the store, so an empty member set must never become a predicate.
// The caller sets EdgeProps for a conditional type; conferredEdge leaves it
// empty.
func conferredEdge(policy *Policy, members, relTypes []string) (*store.RelationPredicate, bool) {
	if len(members) == 0 {
		return nil, false
	}
	p := &store.RelationPredicate{Endpoints: members, OfTypes: relTypes}
	if len(policy.InheritRolesThrough) > 0 {
		p.EntityInheritThrough = append([]string(nil), policy.InheritRolesThrough...)
		p.EntityDepth = depthCap
	}
	return p, true
}
