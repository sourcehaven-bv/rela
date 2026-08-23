package acl

import (
	"fmt"
	"slices"
)

// UngatedPrivilegedRoleRelationOpen reports whether any role-relation
// confers a declared, escalation-relevant role while carrying no
// `requires_permission` gate — i.e. whether a principal who can write one
// edge of that type can self-grant that role.
//
// This is the predicate behind the linter's A2-ungated-role-relation
// finding, extracted so the finding and the load refusal
// ([Policy.WorldGrantRefusalReason]) can never disagree about what "ungated"
// means — the property TKT-T31NKT established for A1 and
// [Policy.MembershipSelfPromotionOpen], extended to A2 (Ruling 8).
//
// "Escalation-relevant" is [Policy.roleIsEscalationRelevant], NOT
// [RoleDef.IsPrivileged]. The two differ by exactly one term — a role
// holding a non-default WORLD read grant — and that term is the reason this
// predicate exists as a refusal rather than only as an advisory finding.
// See roleIsEscalationRelevant for why widening IsPrivileged itself would
// be the wrong fix.
//
// Returns the relation type and the role it confers so the caller can name
// both in a diagnostic; relations are scanned in sorted order so the same
// policy always reports the same one.
func (p *Policy) UngatedPrivilegedRoleRelationOpen() (relType, confers string, open bool) {
	rels := make([]string, 0, len(p.RoleRelations))
	for rel := range p.RoleRelations {
		rels = append(rels, rel)
	}
	slices.Sort(rels)

	for _, rel := range rels {
		if p.RoleRelationEscalates(rel) {
			return rel, p.RoleRelations[rel].Confers, true
		}
	}
	return "", "", false
}

// RoleRelationEscalates reports whether writing one edge of relType would
// self-grant an escalation-relevant role: relType is a declared
// role-relation, it carries no `requires_permission` gate, and the role it
// confers is declared and escalation-relevant.
//
// This is the per-relation predicate behind BOTH the linter's
// A2-ungated-role-relation finding and
// [Policy.UngatedPrivilegedRoleRelationOpen] (the refusal's second arm), so
// the advisory and the enforcement view cannot disagree — TKT-T31NKT's
// shared-predicate property, which Ruling 8 requires extending to A2.
//
// The sharing matters concretely: "escalation-relevant" includes a
// non-default world read grant, which [RoleDef.IsPrivileged] does not. A
// policy refused at load for `owns → viewer{read: [world:published]}` must
// also produce an A2 finding, or `rela acl audit` reports clean on a policy
// the server will not boot — and the refusal's own error text tells the
// operator to run that audit.
func (p *Policy) RoleRelationEscalates(relType string) bool {
	def, declared := p.RoleRelations[relType]
	if !declared || def.RequiresPermission != "" {
		return false
	}
	role, ok := p.Roles[def.Confers]
	return ok && p.roleIsEscalationRelevant(role)
}

// roleIsEscalationRelevant reports whether self-granting this role would
// gain the grantee something a policy author would not want handed out by
// an unauthenticated edge write.
//
// It is [RoleDef.IsPrivileged] — any write verb or any permission — PLUS
// one term: a read grant on a NON-DEFAULT WORLD.
//
// # Why the extra term exists
//
// IsPrivileged deliberately excludes Read, on the reasoning that "a
// read-everything role is a visibility choice, not an escalation path"
// (RR-LXI3NW, RR-UR0LJU). That reasoning holds for the default world and
// is why A1/A2 do not fire on read-only groups — the false positive the
// audit design fought.
//
// It stops holding the moment world-shaped read grants exist. A role
// holding `read: [world:published]` and nothing else is not privileged by
// IsPrivileged's definition, yet self-granting it is EXACTLY the leak this
// ticket's refusal exists to prevent:
//
//	role_relations:
//	  member-of: { requires_permission: delegate-admin }  # gated: A1 says no
//	  owns:      { confers: viewer }                      # ungated
//	roles:
//	  viewer: { read: [world:published] }                 # IsPrivileged: false
//
// One `owns` edge write self-grants a published-world read. A refusal
// keyed on IsPrivileged alone does not fire, and would ship as a
// guarantee it does not deliver. Pinned by
// TestRefusal_ReadOnlyRoleHoldingWorldGrantCountsAsEscalation.
//
// # Why not widen IsPrivileged itself
//
// IsPrivileged is consumed by A1, A2 and A3 as their severity criterion,
// and by the boot warning. Widening it would make every read-only-but-
// world-holding role trip all four, changing the meaning of three existing
// audit findings and the warning for policies that are not doing anything
// wrong. The extra term belongs to the REFUSAL, which is scoped to
// policies that already grant a non-default world read.
func (p *Policy) roleIsEscalationRelevant(role RoleDef) bool {
	if role.IsPrivileged() {
		return true
	}
	for _, w := range role.Worlds {
		if w != DefaultWorldName {
			return true
		}
	}
	return false
}

// assignmentSelfPromotionEscalates reports the A1 shape widened by the same
// world term as [Policy.roleIsEscalationRelevant]: an ungated membership
// relation whose assignments confer a role that is escalation-relevant.
//
// [Policy.MembershipSelfPromotionOpen] stays exactly as TKT-T31NKT wrote it
// — it is the ADVISORY predicate, shared with A1 and the boot warning, and
// widening it would change what those report. This is the REFUSAL's view of
// the same shape.
func (p *Policy) assignmentSelfPromotionEscalates() (role string, open bool) {
	if p.RoleRelations[p.EffectiveMembershipRelation()].RequiresPermission != "" {
		return "", false
	}
	assigned := make(map[string]bool, len(p.Assignments))
	for _, role := range p.Assignments {
		assigned[role] = true
	}
	// Sorted so the same policy always names the same role, rather than
	// whichever one Go's map iteration reached first.
	for _, name := range sortedRoleNames(p.Roles) {
		if assigned[name] && p.roleIsEscalationRelevant(p.Roles[name]) {
			return name, true
		}
	}
	return "", false
}

// WorldGrantRefusalReason returns a human-readable reason this policy must
// be REFUSED at load, or "" when it may load.
//
// The refusal fires only when the policy grants read on a non-default world
// (design doc §8.1) AND leaves a self-promotion path open. Both halves
// matter: a deployment that does not use worlds keeps today's
// warn-and-boot behavior verbatim (docs/acl-security.md promises exactly
// that in writing), and a deployment that has gated its role-relations is
// unaffected.
//
// # Why a refusal here rather than a warning
//
// The ungated-membership hole is pre-existing and, inside a trusted team
// where everyone who can write to the project is already trusted, it may be
// a risk an operator has consciously accepted — so it boots with a warning
// ([appbuild.warnUngatedMembership]). Adding a non-default world read grant
// changes the stakes: the same hole becomes a working mechanism for reading
// unpublished content, one relation write away. That is the case
// docs/acl-security.md § "This becomes a hard requirement with content
// states" told operators would be refused.
//
// # What this guarantees, stated honestly
//
// Arm (b) is a NECESSARY-CONDITION refusal, not a proof of safety. Every
// two-write chain (write an ungated edge to gain a role holding the
// permission that gates membership, then write the membership edge) must
// contain an ungated escalation-relevant role-relation, so refusing on that
// closes every such chain — without a reachability search inside a load
// path. It also refuses some policies that have an ungated role-relation
// not actually reaching the membership gate. That over-refusal is
// deliberate: those policies already carry a High `rela acl audit` finding
// whose fix is one `requires_permission` line.
//
// What it does NOT model: transitive escalation generally, and roles
// reachable only through `asserted_role_assignments` (an IdP claim rather
// than a graph write) — neither predicate scans AssertedRoles (RR-8ZOICR).
func (p *Policy) WorldGrantRefusalReason() string {
	if !p.GrantsAnyNonDefaultWorldRead() {
		return ""
	}
	rel := p.EffectiveMembershipRelation()
	if role, open := p.assignmentSelfPromotionEscalates(); open {
		return fmt.Sprintf(
			"this policy grants read on a non-default world, but the membership "+
				"relation %q carries no requires_permission gate while assignments "+
				"confer the role %q — any principal who can write a %q edge can grant "+
				"themselves that role and read the world. "+
				"Fix: set role_relations.%s.requires_permission and grant that "+
				"permission only to admins (see docs/acl-security.md, and run "+
				"`rela acl audit` for the full picture)",
			rel, role, rel, rel)
	}
	if relType, confers, open := p.UngatedPrivilegedRoleRelationOpen(); open {
		return fmt.Sprintf(
			"this policy grants read on a non-default world, but the role-relation "+
				"%q confers the role %q while carrying no requires_permission gate — "+
				"any principal who can write a %q edge self-grants %q, which is one "+
				"write away from reading a world they were not granted. "+
				"Fix: set role_relations.%s.requires_permission gating the edge to a "+
				"delegate role (see docs/acl-security.md, and run `rela acl audit` "+
				"for the full picture)",
			relType, confers, relType, confers, relType)
	}
	return ""
}
