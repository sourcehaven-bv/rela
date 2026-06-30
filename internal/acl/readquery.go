package acl

import (
	"context"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ReadQueryResult is the response from Request.ReadQuery. Exactly one
// of (AllowAll, DenyAll, Query) is meaningful:
//
//	AllowAll → caller runs an unfiltered list of EntityType.
//	DenyAll  → caller returns an empty list of EntityType.
//	Query    → caller runs the composed store.GraphQuery to filter.
type ReadQueryResult struct {
	AllowAll bool
	DenyAll  bool
	Query    *store.GraphQuery
}

// readQuery composes a ReadQueryResult. AllowAll when any effective
// global role grants read on entityType; otherwise compose a
// GraphQuery whose HasInbound predicate matches entities reachable
// via the role-relations whose confers-role grants read on the type.
// DenyAll when no role grants any kind of read.
func (r *Request) readQuery(ctx context.Context, entityType string) ReadQueryResult {
	globals := r.Globals(ctx)
	for _, a := range globals.Attributions {
		role, ok := r.d.policy.Roles[a.Role]
		if !ok {
			continue
		}
		if roleGrantsRead(role, entityType) {
			return ReadQueryResult{AllowAll: true}
		}
	}

	var conferring []string
	for relType, def := range r.d.policy.RoleRelations {
		role, ok := r.d.policy.Roles[def.Confers]
		if !ok {
			continue
		}
		if roleGrantsRead(role, entityType) {
			conferring = append(conferring, relType)
		}
	}
	if len(conferring) == 0 {
		return ReadQueryResult{DenyAll: true}
	}
	sort.Strings(conferring)

	q := &store.GraphQuery{
		EntityType: entityType,
		HasInbound: &store.RelationPredicate{
			// Endpoints is the principal's group-member set, already
			// expanded by walkMembers over the *configured* membership
			// relation (Policy.membershipRelation). This is the deliberate
			// seam: group expansion happens once, upstream, so this read
			// path inherits the configured relation for free. Do NOT inline
			// the membership relation here as another RelationPredicate —
			// that would re-hardcode "member-of" and bypass the single
			// accessor guard (TKT-Z8A62F).
			Endpoints: globals.Members,
			OfTypes:   conferring,
		},
	}
	if len(r.d.policy.InheritRolesThrough) > 0 {
		q.HasInbound.EntityInheritThrough = append([]string(nil), r.d.policy.InheritRolesThrough...)
		q.HasInbound.EntityDepth = depthCap
	}
	return ReadQueryResult{Query: q}
}

// roleGrantsRead reports whether `role.Read` covers `target` — exact
// match or wildcard `"*"`.
func roleGrantsRead(role RoleDef, target string) bool {
	for _, t := range role.Read {
		if t == "*" || t == target {
			return true
		}
	}
	return false
}
