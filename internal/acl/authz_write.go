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
	return r.decideFromAttrs(attrs, op, s.Type, "no role grants %s on type %q")
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
	//
	// Globals only, never computeForEntity: sourcing this from locally-conferred
	// roles would let a role conferred ON an entity authorize edges FROM it,
	// which is the delegate-X inversion in a different dress.
	attrs := r.Globals(ctx).Attributions

	// The ceiling is checked before any grant, exactly as on the entity path —
	// see ceilingDenial for why roleFor alone is not enough. Note it keys on
	// FromType while the relation grant below keys on s.Type: entity types are
	// the ceiling's vocabulary, so `deny_write: ["*"]` denies every relation
	// write, while `deny_write: [person]` does not deny an edge from a
	// non-person. That asymmetry is deliberate.
	if deny := r.ceilingDenial(attrs, op, s.FromType); deny != nil {
		return *deny
	}

	// `relation_grants:` — an ALTERNATIVE SATISFIER of the source-type verb
	// grant below, never a short-circuit: both the delegate-X gate above and
	// the ceiling just above have already had their say, and neither can be
	// reached from here.
	//
	// Gated on a resolved FromType. Four of the five RelationSubject call sites
	// leave it empty when the source entity is missing or unreadable, and today
	// that fails closed because no role lists "". Honoring a relation grant on
	// an empty FromType would silently turn "source unresolvable ⇒ deny" into
	// "⇒ allow" — the grant keys on the caller-supplied relation type, which is
	// always populated, so nothing else would be checked.
	if s.FromType != "" {
		perm, ok := r.d.policy.relationPermissionFor(s.Type, op)
		if ok && r.grantsPermission(attrs, perm) {
			return Decision{
				Allow:        true,
				RuleKind:     "relation-grant",
				RuleID:       perm,
				Attributions: attrs,
			}
		}
	}

	d := r.decideFromAttrs(attrs, op, s.FromType,
		"no role grants %s on relations from type %q")
	if !d.Allow {
		d.Reason = r.explainRelationDenial(d.Reason, s, op)
	}
	return d
}

// explainRelationDenial appends the relation_grants path to a denial reason
// when the policy declares one for this relation type. Without it an operator
// who configured `relation_grants:` and then got denied reads a message about
// the SOURCE TYPE and has no hint that a second, closer rule was consulted.
//
// That is the incident's second root cause in miniature: the gate knew exactly
// why it said no, and said something else.
func (r *Request) explainRelationDenial(reason string, s RelationSubject, op Op) string {
	perm, ok := r.d.policy.relationPermissionFor(s.Type, op)
	if !ok {
		return reason
	}
	if s.FromType == "" {
		return fmt.Sprintf("%s; relation_grants.%s would accept permission %q, but the "+
			"source entity's type could not be resolved", reason, s.Type, perm)
	}
	return fmt.Sprintf("%s; nor does any role grant permission %q "+
		"(relation_grants.%s)", reason, perm, s.Type)
}

// decideFromAttrs returns an allow Decision when any role in `attrs`
// grants the verb `op` on `target`; otherwise a structured deny with the
// reason templated against the verb and target.
//
// The full attribution set propagates into the returned Decision on
// both branches so audit consumers can record every (role, source)
// the resolver considered (AC7). The wire 403 path
// ([ForbiddenError.Error]) ignores Attributions — only audit reads it.
func (r *Request) decideFromAttrs(attrs []RoleAttribution, op Op, target, denyFmt string) Decision {
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
	if deny := r.ceilingDenial(attrs, op, target); deny != nil {
		return *deny
	}
	for _, a := range attrs {
		role, ok := r.roleFor(a.Role)
		if !ok {
			continue
		}
		if grantsVerb(role, op, target) {
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

// ceilingDenial returns the structured client-ceiling denial when the ceiling
// withholds op on target, or nil when it permits. Extracted so both write paths
// apply the clamp in the same position without decideFromAttrs having to know
// which grant sources each path can satisfy.
//
// It must run before any grant is consulted, and its result must be able to
// deny even when some other source would allow: filterTypes deliberately
// PRESERVES a role's "*" under a pure denial (a plain allowlist cannot spell
// "all except X"), so the RoleDef returned by roleFor still looks permissive
// under the common `deny_write: ["*"]` shape. permitsVerb is what actually
// gates. Any new allow source that skips this call escapes the ceiling
// entirely, which would make a ceiling GRANT — see ceiling.go.
func (r *Request) ceilingDenial(attrs []RoleAttribution, op Op, target string) *Decision {
	if r.ceiling.permitsVerb(op, target) {
		return nil
	}
	return &Decision{
		Allow:    false,
		RuleKind: "client-ceiling",
		RuleID:   r.ceiling.name,
		Reason: fmt.Sprintf(
			"client baseline %q does not permit %s on %q for principal type %q",
			r.ceiling.name, op, target, r.principal.PrincipalType()),
		Attributions: attrs,
	}
}
