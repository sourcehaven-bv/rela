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
	members := r.Globals().Members
	if len(members) == 0 {
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
	pred = store.RelationPredicate{Endpoints: members, OfTypes: conferring}
	if len(r.d.policy.InheritRolesThrough) > 0 {
		pred.EntityInheritThrough = append([]string(nil), r.d.policy.InheritRolesThrough...)
		pred.EntityDepth = depthCap
	}
	return pred, true
}
