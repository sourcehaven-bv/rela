package aclmap

import (
	"context"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// enumeratePrincipals returns the deduplicated, sorted set of candidate
// principal identifiers that could hold a non-everyone grant:
//
//   - every entity of the policy's user_entity_type (the resolvable
//     human principals), when that type is configured;
//   - every key in Assignments (direct + group grant keys, incl.
//     raw-UPN break-glass keys);
//   - the source (From) of every membership-relation edge — a member of
//     a group, who inherits the group's assigned role;
//   - the source (From) of every role-relation edge — a principal
//     granted a role by a graph edge.
//
// The membership-relation sources matter because a person typically
// holds NO direct assignment and is NOT of user_entity_type (which may
// be unset): they gain their role purely by being a member of a group
// that is an assignment key. Omitting them drops exactly the
// group-conferred principals — the false negative the who-can tool must
// never produce.
//
// The everyone role is not a principal and is reported globally. This is
// the O(principals) universe the ticket's cost note describes; a reverse
// principal→entity index is a later optimization.
func (e *Engine) enumeratePrincipals(ctx context.Context) ([]string, error) {
	policy := e.resolver.Policy()
	set := map[string]struct{}{}

	// Assignment keys.
	for key := range policy.Assignments {
		set[key] = struct{}{}
	}

	// User-entity-type entities.
	if ut := policy.UserEntityType; ut != "" {
		for ent, err := range e.src.ListEntities(ctx, store.EntityQuery{Type: ut}) {
			if err != nil {
				return nil, fmt.Errorf("aclmap: list %s entities: %w", ut, err)
			}
			set[ent.ID] = struct{}{}
		}
	}

	// Sources of membership and role-relation edges. Both are read via
	// the relation index keyed by type; the membership relation is the
	// effective one (default member-of) so it honors a policy override.
	// Membership TARGETS (groups) are collected separately: a group is a
	// role container, not an actor — it must not be listed as a principal
	// even though, resolved as one, it would carry its own assigned role.
	// Excluding groups keeps the report to real actors and avoids
	// double-reporting a grant against both the group and its members.
	groups := map[string]struct{}{}
	membershipRel := policy.EffectiveMembershipRelation()
	relTypes := map[string]struct{}{membershipRel: {}}
	for relType := range policy.RoleRelations {
		relTypes[relType] = struct{}{}
	}
	for relType := range relTypes {
		for rel, err := range e.src.ListRelations(ctx, store.RelationQuery{
			Type:      relType,
			Direction: store.DirectionOutgoing,
		}) {
			if err != nil {
				return nil, fmt.Errorf("aclmap: list %s relations: %w", relType, err)
			}
			set[rel.From] = struct{}{}
			if relType == membershipRel {
				groups[rel.To] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		if _, isGroup := groups[k]; isGroup {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// resolvePrincipal maps a raw principal key to a user entity ID via the
// policy's principal_property lookup. Returns "" when the lookup is
// disabled or no entity matches (the caller keeps the raw key). An
// ambiguous or errored lookup is surfaced so the report fails loud
// rather than silently mis-attributing.
func (e *Engine) resolvePrincipal(ctx context.Context, raw string) (string, error) {
	return e.resolver.ResolvePrincipal(ctx, raw)
}
