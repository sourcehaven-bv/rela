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
// group-conferred principals — a false negative.
//
// No candidate is excluded by graph TOPOLOGY. An earlier version dropped
// membership-relation targets as "groups", but "is a membership target"
// is not the same predicate as "is a non-actor container": a manager who
// has reports (a membership target) is also a real principal, and
// excluding them dropped a genuine grantee (RR-C5Q743). Correctness — the
// runtime's own answer for every candidate — beats hiding group entities.
// A candidate that carries no grant for the queried entity/verb simply
// yields no row; only the "everyone" role is special-cased, and it is
// reported globally, not per candidate.
//
// This is the O(principals) universe the ticket's cost note describes; a
// reverse principal→entity index is a later optimization.
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
	relTypes := map[string]struct{}{policy.EffectiveMembershipRelation(): {}}
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
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
