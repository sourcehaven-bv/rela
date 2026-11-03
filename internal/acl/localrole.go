package acl

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LocalRolePredicate returns the relation predicate that matches the entities
// on which the principal holds role through a role relation. It is the store
// form of the local half of [Request.ForEntity]: an incoming edge of a type
// that confers role, from the principal or one of its groups, on the entity or
// on an ancestor reached through inherit_roles_through.
//
// It is the same predicate readQuery builds for a conferred read grant, so a
// role held per entity is answered by the store the way the row gate is.
//
// ok is false when no role relation confers role, or when the principal has
// no member set. The role is then held through an edge on no entity. An
// empty member set must never become a predicate, because empty Endpoints
// means "any endpoint" to the store.
func (r *Request) LocalRolePredicate(role string) (pred store.RelationPredicate, ok bool) {
	if _, defined := r.roleFor(role); !defined {
		return store.RelationPredicate{}, false
	}
	var conferring []string
	for relType, def := range r.d.policy.RoleRelations {
		if def.Confers == role {
			conferring = append(conferring, relType)
		}
	}
	if len(conferring) == 0 {
		return store.RelationPredicate{}, false
	}
	sort.Strings(conferring)
	p, ok := conferredEdge(r.d.policy, r.Globals().Members, conferring)
	if !ok {
		return store.RelationPredicate{}, false
	}
	return *p, true
}

// conferredEdge returns the predicate that matches an entity with an incoming
// edge of one of relTypes from one of members, on the entity itself or on an
// ancestor reached through inherit_roles_through. It is the one place the
// conferred-role edge is spelled, shared by the row gate ([Request.ReadQuery])
// and [Request.LocalRolePredicate].
//
// ok is false for an empty members: empty Endpoints means "any endpoint" to
// the store, so an empty member set must never become a predicate.
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
