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
	// A client ceiling that denies reads of this type short-circuits every
	// grant path below. Checked first because the ceiling can only ever remove:
	// if it says no, no role — global, conferred or inherited — can say yes.
	// This also closes the wildcard gap roleFor cannot express in a plain list
	// (a role holding `read: ["*"]` under `deny_read: [person]` keeps its
	// wildcard; see filterTypes).
	if !r.ceiling.permitsRead(entityType) {
		return ReadQueryResult{DenyAll: true}
	}

	globals := r.Globals(ctx)
	for _, a := range globals.Attributions {
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		if roleGrantsRead(role, entityType) {
			return ReadQueryResult{AllowAll: true}
		}
	}

	var conferring []string
	for relType, def := range r.d.policy.RoleRelations {
		role, ok := r.roleFor(def.Confers)
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

	// Fail closed on an empty member set. `Endpoints: nil` means "ANY
	// endpoint" to store.GraphQuery (the absence-query widening), so
	// handing it an empty set would degrade this gate from "reachable
	// from ME" to "reachable from ANYONE" — a read bypass, not a
	// narrowing. Today walkMembers only returns empty for an unstamped
	// principal, which Declarative.ForPrincipal already rejects; this
	// guard states the invariant HERE so the gate does not depend on a
	// validation living in another file to stay safe.
	if len(globals.Members) == 0 {
		return ReadQueryResult{DenyAll: true}
	}

	// World is deliberately left ZERO here, which means the DEFAULT world
	// (store.WorldScope's zero value). This is NOT an assertion that the
	// ACL path is world-safe: a world-scoped read must have its scope
	// INJECTED from above by the world-resolved reader (internal/worldreader,
	// TKT-WAV8XP PR-D), which is the layer that owns a compiled WorldScope.
	//
	// internal/acl structurally cannot resolve one itself — arch-lint
	// forbids it from importing internal/metamodel, and a WorldScope is
	// compiled from the metamodel. So the seam belongs above this package,
	// not in it. Until PR-D wires that injection, store.GraphQuery.World is
	// honored by the backends (PR-B) but never set on this path.
	//
	// Do NOT "fix" this by giving internal/acl a metamodel dependency: that
	// breaks the boundary that keeps ACL evaluable without a schema.
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
