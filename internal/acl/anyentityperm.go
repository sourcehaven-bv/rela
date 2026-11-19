package acl

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// HoldsPermissionOnAnyEntity reports whether r's principal holds perm through
// a global role, or through a role that a role relation confers on some
// entity ([heldRelationRoles]).
//
// Only a gate that decides what to render may use it; the reasoning is on
// access.Mode.PermitsGatedUIElement, its one caller. A gate that allows an
// operation uses [Request.HoldsPermission], which counts global roles only,
// and a check about one entity uses [Request.HoldsPermissionForEntity].
//
// A failed read of the role relation edges counts no conferred role, so the
// answer is the global one. The client ceiling applies as it does for every
// permission check.
//
// A function rather than a method: [Request] is at its plimsoll method load
// line.
//
// Nil: r is rejected (it panics); it must come from [Declarative.ForIdentity].
//
//archplan:guard=authz reason="checks a config-declared UX permission"
func HoldsPermissionOnAnyEntity(ctx context.Context, r *Request, perm string) bool {
	if r.holdsPermission(perm) {
		return true
	}
	if !r.grantsPermission(conferrableRoles(r), perm) {
		return false // no role relation could grant it: skip the read
	}
	held, err := r.relationRoles.get(ctx, r)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.WarnContext(ctx, "acl: reading role relation edges failed; counting global roles only",
				"user", r.principal.User, "err", err)
		}
		return false
	}
	return r.grantsPermission(held, perm)
}

// heldRelationRoles is the set of roles a [Request]'s principal holds
// through a role relation on some entity: the principal, or a group it
// belongs to, is the source of at least one edge of the relation whose
// `when:` holds. The target does not matter. It answers the questions that
// have no subject entity: [Request.PermitsWorld] and
// [HoldsPermissionOnAnyEntity].
//
// It is read on first use and kept for the Request, so a sidebar costs the
// same whatever its size: one read per role relation that confers a role,
// each stopping at the first edge. A read that failed because the caller's
// ctx ended is not kept, so a later caller with a live ctx reads again. Any
// other error is kept: the Request does not retry it.
type heldRelationRoles struct {
	mu    sync.Mutex
	done  bool
	roles []RoleAttribution
	err   error
}

// get returns the set, reading it for r on the first call.
func (h *heldRelationRoles) get(ctx context.Context, r *Request) ([]RoleAttribution, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.done {
		return h.roles, h.err
	}
	roles, err := readRelationRoles(ctx, r)
	if err != nil && ctx.Err() != nil {
		return nil, err
	}
	h.done, h.roles, h.err = true, roles, err
	return roles, err
}

// conferrableRoles returns one attribution per role relation that confers a
// role, as if the principal held every one. A question that no role in it
// answers needs no read of the edges.
func conferrableRoles(r *Request) []RoleAttribution {
	rels := r.conferringRelations()
	out := make([]RoleAttribution, 0, len(rels))
	for _, rel := range rels {
		role := r.d.policy.RoleRelations[rel].Confers
		out = append(out, RoleAttribution{Role: role, Source: Source{Kind: SourceLocal, Relation: rel}})
	}
	return out
}

// readRelationRoles returns one attribution per role relation of which a
// member of r's closure holds at least one live edge. Any read error fails
// the whole set, so a partial answer never counts.
func readRelationRoles(ctx context.Context, r *Request) ([]RoleAttribution, error) {
	members := r.Globals().Members
	if len(members) == 0 {
		return nil, nil
	}
	var held []RoleAttribution
	for _, rel := range r.conferringRelations() {
		cond, _ := r.conds.edge(rel) // conferringRelations keeps live types only
		found, err := r.d.graph.HasOutgoingEdge(ctx, rel, cond, members)
		if err != nil {
			return nil, err
		}
		if found {
			role := r.d.policy.RoleRelations[rel].Confers
			held = append(held, RoleAttribution{Role: role, Source: Source{Kind: SourceLocal, Relation: rel}})
		}
	}
	return held, nil
}
