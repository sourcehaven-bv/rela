package acl

import (
	"context"
	"fmt"
)

// authorizeWrite implements per-Request write authz. v1 only:
//   - subject-aware dispatch (EntitySubject vs RelationSubject)
//   - entity-id-aware local-role evaluation for EntitySubject
//   - primary-source attribution surfaced in the Decision
//
// A nil Subject is a programmer error: every production caller in
// entitymanager / dataentry populates Subject, and tests use
// EntitySubject{} / RelationSubject{} explicitly. The legacy
// "fall through to global-roles-only" path that briefly existed
// during the v0→v1 migration was removed (RR-X1TE) because it could
// silently downgrade an unstamped principal to v0 semantics without
// the isUnstamped check.
func (r *Request) authorizeWrite(ctx context.Context, req WriteRequest) Decision {
	switch s := req.Subject.(type) {
	case EntitySubject:
		return r.authorizeEntityWrite(ctx, s)
	case RelationSubject:
		return r.authorizeRelationWrite(ctx, s)
	default:
		// Sealed sum (incl. nil Subject): unreachable from any well-formed
		// caller. Panic so a missing case or a forgotten Subject surfaces
		// in CI rather than as silent-deny in production.
		panic(fmt.Sprintf("acl: unhandled Subject variant %T", s))
	}
}

func (r *Request) authorizeEntityWrite(ctx context.Context, s EntitySubject) Decision {
	// With an ID, fold in local-role probes; without, globals-only
	// (Op=Create has no ID yet at authz time).
	var attrs []RoleAttribution
	if s.ID != "" {
		attrs = r.computeForEntity(ctx, s.ID)
	} else {
		attrs = r.Globals(ctx).Attributions
	}
	return r.decideFromAttrs(attrs, s.Type, "no role grants write on type %q")
}

func (r *Request) authorizeRelationWrite(ctx context.Context, s RelationSubject) Decision {
	// Delegate-X gate for role-relation writes.
	if rr, ok := r.d.policy.RoleRelations[s.Type]; ok && rr.RequiresPermission != "" {
		if !r.holdsPermission(ctx, rr.RequiresPermission) {
			return Decision{
				Allow:    false,
				RuleKind: "delegate-permission",
				RuleID:   rr.RequiresPermission,
				Reason: fmt.Sprintf("writing %q relations requires permission %q",
					s.Type, rr.RequiresPermission),
			}
		}
	}
	// Type-level gate: principal needs write on the source entity's
	// type. The To side is not part of RelationSubject — see that
	// type's godoc for the rationale (RR-F9M9).
	attrs := r.Globals(ctx).Attributions
	return r.decideFromAttrs(attrs, s.FromType,
		"no role grants write on relations from type %q")
}

// decideFromAttrs returns an allow Decision when any role in `attrs`
// grants write on `target`; otherwise a structured deny with the
// reason templated against `target`.
//
// The full attribution set propagates into the returned Decision on
// both branches so audit consumers can record every (role, source)
// the resolver considered (AC7). The wire 403 path
// ([ForbiddenError.Error]) ignores Attributions — only audit reads it.
func (r *Request) decideFromAttrs(attrs []RoleAttribution, target, denyFmt string) Decision {
	for _, a := range attrs {
		role, ok := r.d.policy.Roles[a.Role]
		if !ok {
			continue
		}
		if roleGrantsWrite(role, target) {
			return Decision{
				Allow:        true,
				RuleKind:     "role-grant",
				RuleID:       a.Role,
				Attributions: attrs,
			}
		}
	}
	return Decision{
		Allow:        false,
		RuleKind:     "role-grant",
		RuleID:       "-",
		Reason:       fmt.Sprintf(denyFmt, target),
		Attributions: attrs,
	}
}
