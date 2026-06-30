package aclaudit

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// tierA runs the pure-policy checks: escalation foot-guns and dead/inert
// config that need no metamodel. Order within the slice doesn't matter —
// Audit sorts the combined result.
func tierA(p *acl.Policy) []Finding {
	f := make([]Finding, 0, 8)
	f = append(f, checkUngatedMembership(p)...)     // A1 / A1b
	f = append(f, checkUngatedRoleRelations(p)...)  // A2
	f = append(f, checkEveryonePrivileged(p)...)    // A3
	f = append(f, checkAssignmentsToUnknown(p)...)  // A4
	f = append(f, checkConfersUnknown(p)...)        // A5
	f = append(f, checkUngrantablePermission(p)...) // A6
	f = append(f, checkDeadPermissions(p)...)       // A7
	f = append(f, checkWildcardWriteSprawl(p)...)   // A9
	f = append(f, checkNameWhitespace(p)...)        // A10
	return f
}

// A1 / A1b — the membership relation confers group roles via assignments but
// is not gated by requires_permission (self-promotion), or has no assignments
// to confer (inert). Covers the DEFAULT member-of too, via the same effective
// accessor the resolver uses (single source of truth).
func checkUngatedMembership(p *acl.Policy) []Finding {
	rel := p.EffectiveMembershipRelation()
	var f []Finding

	// A1b — configured a non-default membership relation but no assignments,
	// so it confers no group-level roles. (Only for non-default: the default
	// member-of with no assignments is the common "groups unused" case, not
	// worth a finding.)
	if rel != defaultMembershipRelationName() && len(p.Assignments) == 0 {
		f = append(f, Finding{
			Rule: "A1b-inert-membership", Severity: Low, Subject: rel,
			Detail: fmt.Sprintf("membership_relation %q is configured but assignments is empty, "+
				"so it confers no group-level roles", rel),
			Fix: "add an assignments mapping, or remove membership_relation if groups are unused",
		})
	}

	// A1 — the membership relation can confer an assigned role (there is at
	// least one assignment to a declared role) but writes to it are not gated.
	if len(p.Assignments) > 0 && assignsAnyDeclaredRole(p) && requiresPermissionFor(p, rel) == "" {
		f = append(f, Finding{
			Rule: "A1-ungated-membership", Severity: High, Subject: rel,
			Detail: fmt.Sprintf("membership relation %q confers group roles via assignments but is not "+
				"gated by requires_permission; any principal who can write a %q edge can grant "+
				"themselves any assigned role", rel, rel),
			Fix: fmt.Sprintf("add role_relations.%s.requires_permission and grant that permission "+
				"only to admins (see docs/security.md)", rel),
		})
	}
	return f
}

// assignsAnyDeclaredRole reports whether at least one assignment targets a
// declared role (so the membership walk actually confers something).
func assignsAnyDeclaredRole(p *acl.Policy) bool {
	for _, role := range p.Assignments {
		if roleDeclared(p, role) {
			return true
		}
	}
	return false
}

// A2 — a role-relation confers a privileged role but is not gated by
// requires_permission: anyone who can write that edge self-grants the role.
func checkUngatedRoleRelations(p *acl.Policy) []Finding {
	var f []Finding
	for _, rel := range sortedRelTypes(p) {
		def := p.RoleRelations[rel]
		role, ok := p.Roles[def.Confers]
		if !ok || !isPrivileged(role) {
			continue
		}
		if def.RequiresPermission != "" {
			continue // gated — fine
		}
		f = append(f, Finding{
			Rule: "A2-ungated-role-relation", Severity: High, Subject: rel,
			Detail: fmt.Sprintf("role-relation %q confers the privileged role %q but is not gated by "+
				"requires_permission; any principal who can write a %q edge self-grants %q",
				rel, def.Confers, rel, def.Confers),
			Fix: fmt.Sprintf("add role_relations.%s.requires_permission gating the edge to a delegate role", rel),
		})
	}
	return f
}

// A3 — the built-in `everyone` role grants write/permissions: every principal
// (including anonymous) holds it. Critical. Read-only everyone is fine.
func checkEveryonePrivileged(p *acl.Policy) []Finding {
	role, ok := p.Roles[acl.EveryoneRole]
	if !ok || !isPrivileged(role) {
		return nil
	}
	return []Finding{{
		Rule: "A3-everyone-privileged", Severity: Critical, Subject: acl.EveryoneRole,
		Detail: fmt.Sprintf("role %q is held by every principal (including anonymous) and grants write "+
			"or permissions; this hands privileged capabilities to everyone", acl.EveryoneRole),
		Fix: "remove write/permission grants from the everyone role; keep only read grants there",
	}}
}

// A4 — an assignment names a role not declared in roles: silently inert.
func checkAssignmentsToUnknown(p *acl.Policy) []Finding {
	var f []Finding
	for _, key := range sortedAssignmentKeys(p) {
		role := p.Assignments[key]
		if roleDeclared(p, role) {
			continue
		}
		f = append(f, Finding{
			Rule: "A4-assignment-unknown-role", Severity: Medium, Subject: key,
			Detail: fmt.Sprintf("assignment %q -> %q names a role not declared in roles; the assignment "+
				"has no effect", key, role),
			Fix: fmt.Sprintf("declare role %q under roles, or fix the assignment", role),
		})
	}
	return f
}

// A5 — a role-relation confers a role not declared in roles: silently inert.
func checkConfersUnknown(p *acl.Policy) []Finding {
	var f []Finding
	for _, rel := range sortedRelTypes(p) {
		role := p.RoleRelations[rel].Confers
		if role == "" || roleDeclared(p, role) {
			continue
		}
		f = append(f, Finding{
			Rule: "A5-confers-unknown-role", Severity: Medium, Subject: rel,
			Detail: fmt.Sprintf("role_relations.%s confers %q which is not declared in roles; the "+
				"role-relation confers nothing", rel, role),
			Fix: fmt.Sprintf("declare role %q under roles, or fix the confers value", role),
		})
	}
	return f
}

// A6 — a requires_permission names a permission no role grants: no principal
// can write that relation. Often an intentional lockdown, so this is low and
// phrased as a question (RR-O7H3GY).
func checkUngrantablePermission(p *acl.Policy) []Finding {
	var f []Finding
	for _, rel := range sortedRelTypes(p) {
		perm := p.RoleRelations[rel].RequiresPermission
		if perm == "" || permissionGranted(p, perm) {
			continue
		}
		f = append(f, Finding{
			Rule: "A6-ungrantable-permission", Severity: Low, Subject: rel,
			Detail: fmt.Sprintf("role_relations.%s requires permission %q which no role grants, so no "+
				"principal can write a %s edge — is this an intentional lockdown?", rel, perm, rel),
			Fix: fmt.Sprintf("grant %q to a role, or remove the requires_permission gate if the "+
				"lockdown is unintended", perm),
		})
	}
	return f
}

// A7 — a role declares a permission that no requires_permission references:
// dead config, possibly a typo of a real gate.
func checkDeadPermissions(p *acl.Policy) []Finding {
	// Collect every permission referenced by a requires_permission gate.
	used := map[string]bool{}
	for _, def := range p.RoleRelations {
		if def.RequiresPermission != "" {
			used[def.RequiresPermission] = true
		}
	}
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		perms := append([]string(nil), p.Roles[name].Permissions...)
		sort.Strings(perms)
		for _, perm := range perms {
			if used[perm] {
				continue
			}
			f = append(f, Finding{
				Rule: "A7-dead-permission", Severity: Low, Subject: name,
				Detail: fmt.Sprintf("role %q grants permission %q which no role_relations.requires_permission "+
					"references; the permission is dead", name, perm),
				Fix: fmt.Sprintf("reference %q in a requires_permission gate, or remove it (check for a typo)", perm),
			})
		}
	}
	return f
}

// A9 — a non-`everyone` role grants a WRITE wildcard ("*" on create/update/
// delete). read:["*"] is NEVER flagged — read-everything is an intentional
// visibility choice, not least-privilege sprawl (RR-UR0LJU).
func checkWildcardWriteSprawl(p *acl.Policy) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		if name == acl.EveryoneRole {
			continue
		}
		if !hasWildcardWrite(p.Roles[name]) {
			continue
		}
		f = append(f, Finding{
			Rule: "A9-wildcard-write", Severity: Medium, Subject: name,
			Detail: fmt.Sprintf("role %q grants a write wildcard (\"*\") on create/update/delete; confirm "+
				"this role is meant to mutate every entity type", name),
			Fix: "list the specific entity types the role may write, or confirm the wildcard is intended for an admin role",
		})
	}
	return f
}

// A10 — a relation/role/type name carries leading/trailing whitespace, so it
// silently matches nothing. Checks the membership relation, role-relation
// keys, assignment values, and inherit_roles_through entries.
func checkNameWhitespace(p *acl.Policy) []Finding {
	var f []Finding
	add := func(subject, kind string) {
		f = append(f, Finding{
			Rule: "A10-name-whitespace", Severity: Low, Subject: subject,
			Detail: fmt.Sprintf("%s %q has leading/trailing whitespace and will not match what the "+
				"operator intended", kind, subject),
			Fix: "remove the surrounding whitespace",
		})
	}
	if hasLeadingTrailingSpace(p.MembershipRelation) {
		add(p.MembershipRelation, "membership_relation")
	}
	for _, rel := range sortedRelTypes(p) {
		if hasLeadingTrailingSpace(rel) {
			add(rel, "role_relations key")
		}
	}
	for _, key := range sortedAssignmentKeys(p) {
		if hasLeadingTrailingSpace(p.Assignments[key]) {
			add(p.Assignments[key], "assignment role")
		}
	}
	for _, rel := range p.InheritRolesThrough {
		if hasLeadingTrailingSpace(rel) {
			add(rel, "inherit_roles_through entry")
		}
	}
	return f
}

// ---- deterministic iteration helpers -----------------------------------

func sortedRelTypes(p *acl.Policy) []string {
	out := make([]string, 0, len(p.RoleRelations))
	for r := range p.RoleRelations {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func sortedAssignmentKeys(p *acl.Policy) []string {
	out := make([]string, 0, len(p.Assignments))
	for k := range p.Assignments {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// defaultMembershipRelationName returns the default membership relation name.
// aclaudit can't see acl's unexported const, so it derives it from a
// zero-value Policy's effective relation (the single source of truth).
func defaultMembershipRelationName() string {
	return (&acl.Policy{}).EffectiveMembershipRelation()
}
