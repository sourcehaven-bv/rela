package acl

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
		return r.authorizeEntityWrite(ctx, req.Op, s)
	case RelationSubject:
		return r.authorizeRelationWrite(ctx, req.Op, s)
	default:
		// Sealed sum (incl. nil Subject): unreachable from any well-formed
		// caller. Panic so a missing case or a forgotten Subject surfaces
		// in CI rather than as silent-deny in production.
		panic(fmt.Sprintf("acl: unhandled Subject variant %T", s))
	}
}

func (r *Request) authorizeEntityWrite(ctx context.Context, op Op, s EntitySubject) Decision {
	// With an ID, fold in local-role probes; without, globals-only
	// (Op=Create has no ID yet at authz time).
	var attrs []RoleAttribution
	if s.ID != "" {
		attrs = r.computeForEntity(ctx, s.ID)
	} else {
		attrs = r.Globals(ctx).Attributions
	}
	return r.decideFromAttrs(attrs, op, s.Type, s.Pointer, "no role grants %s on type %q")
}

func (r *Request) authorizeRelationWrite(ctx context.Context, op Op, s RelationSubject) Decision {
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
	// Type-level gate: principal needs the matching verb grant on the source
	// entity's type. A relation create checks the source type's `create` grant
	// (consistent with entity create); the To side is not part of
	// RelationSubject — see that type's godoc for the rationale (RR-F9M9).
	attrs := r.Globals(ctx).Attributions
	// Relation writes address the DEFAULT tail today: RelationSubject carries
	// no pointer, and store.RelationData.FromPointer is consumed by
	// CreateRelation only. State-tailed relation writes arrive with the copy
	// kernel; until then the zero pointer here is the accurate statement, not
	// a placeholder.
	return r.decideFromAttrs(attrs, op, s.FromType, entity.Pointer(""),
		"no role grants %s on relations from type %q")
}

// decideFromAttrs returns an allow Decision when any role in `attrs`
// grants the verb `op` on `target`; otherwise a structured deny with the
// reason templated against the verb and target.
//
// The full attribution set propagates into the returned Decision on
// both branches so audit consumers can record every (role, source)
// the resolver considered (AC7). The wire 403 path
// ([ForbiddenError.Error]) ignores Attributions — only audit reads it.
func (r *Request) decideFromAttrs(
	attrs []RoleAttribution, op Op, target string, ptr entity.Pointer, denyFmt string,
) Decision {
	// The client ceiling is checked before any role, because it can only
	// remove: if it denies this verb on this type, no role can grant it. The
	// deny names the ceiling rather than a role, so an operator reading the
	// audit log can tell "your policy grants nothing" apart from "your policy
	// grants it, but this client is attenuated" — two very different fixes.
	//
	// It also closes the wildcard gap: a role holding `update: ["*"]` under a
	// `deny_update: [person]` ceiling keeps its wildcard through roleFor (a
	// plain list cannot spell "all except person"), so the denial must be
	// applied here.
	if !r.ceiling.permitsVerb(op, target) {
		return Decision{
			Allow:    false,
			RuleKind: "client-ceiling",
			RuleID:   r.ceiling.name,
			Reason: fmt.Sprintf(
				"client baseline %q does not permit %s on %q for principal type %q",
				r.ceiling.name, op, target, r.principal.PrincipalType()),
			Attributions: attrs,
		}
	}
	for _, a := range attrs {
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		if GrantsVerbOnState(role, op, target, ptr) {
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
		Reason:       fmt.Sprintf(denyFmt, op, target),
		Attributions: attrs,
	}
}
