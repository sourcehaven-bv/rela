package acl

import (
	"context"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Declarative is the policy-driven [ACL] implementation. It composes
// a [Policy] (loaded from `acl.yaml`) with the [principal.Principal]
// carried on each request's context to answer [ACL.AuthorizeWrite].
//
// v0 scope:
//
//   - **Global roles only.** A principal's effective role set is
//     `Assignments[user]` plus the `default` role (if defined). Groups
//     and transitive `member-of` expansion are deferred to v1.
//   - **Union semantics.** Any role granting write on the target
//     entity type → allow. The returned RuleID names the first role
//     that matched, for debuggability.
//   - **Delegate-X tamper resistance.** Writes to a relation type
//     listed in [Policy.RoleRelations] require the writer to hold the
//     declared permission. See [RoleRelationDef.RequiresPermission].
//
// PR 2 wires this in unit tests only; PR 3 makes [appbuild.Discover]
// load it.
type Declarative struct {
	policy *Policy
}

// NewDeclarative wraps a [Policy] as an [ACL]. The Policy must be
// non-nil; the caller (typically appbuild) loads it via
// [LoadPolicy] and falls back to [NopACL] when no `acl.yaml` exists.
func NewDeclarative(p *Policy) *Declarative {
	return &Declarative{policy: p}
}

// AuthorizeWrite implements [ACL.AuthorizeWrite].
//
// Order of checks:
//
//  1. If `req.RelationType` names a role-relation that declares a
//     `requires_permission`, the writer must hold that permission.
//     Deny with `RuleKind="delegate-permission"` on miss.
//  2. Otherwise (and for entity writes), any role in the principal's
//     effective set whose `write` list contains the target type — or
//     the wildcard `"*"` — allows. The matching role's name surfaces
//     as `RuleID`.
//  3. No role granted → deny with `RuleKind="role-grant"`,
//     `RuleID="-"`.
//
// For relation writes, `req.EntityType` carries the source entity's
// type (the caller in entitymanager looks it up). Empty EntityType
// is treated as the literal target `"relation"` — denied by default
// since no role would grant write on a synthetic type, surfacing the
// misuse in the deny reason.
func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
	p := principal.From(ctx)
	roles := d.effectiveRoles(p)

	// 1. Delegate-X gate for role-relation writes.
	if req.RelationType != "" {
		if rr, ok := d.policy.RoleRelations[req.RelationType]; ok && rr.RequiresPermission != "" {
			if !d.holdsPermission(roles, rr.RequiresPermission) {
				return Decision{
					Allow:    false,
					RuleKind: "delegate-permission",
					RuleID:   rr.RequiresPermission,
					Reason: fmt.Sprintf("writing '%s' relations requires permission '%s'",
						req.RelationType, rr.RequiresPermission),
				}
			}
		}
	}

	// 2. Type-level write grant. Union across roles; first hit wins
	//    for RuleID (deterministic via effectiveRoles ordering).
	target := req.EntityType
	if target == "" {
		target = "relation"
	}
	for _, name := range roles {
		role, ok := d.policy.Roles[name]
		if !ok {
			continue
		}
		if roleGrantsWrite(role, target) {
			return Decision{Allow: true, RuleKind: "role-grant", RuleID: name}
		}
	}

	return Decision{
		Allow:    false,
		RuleKind: "role-grant",
		RuleID:   "-",
		Reason:   fmt.Sprintf("no role grants write on type '%s'", target),
	}
}

// effectiveRoles returns the role names applicable to p, in priority
// order: explicit assignment first, then the `default` role (when
// defined). The order is stable so [AuthorizeWrite] deterministically
// reports the same RuleID for a given Allow.
//
// v0 is global-roles-only: no group expansion. If
// `Assignments[user]` names an undefined role, it is dropped (a
// load-time warning will have been emitted in v1; v0 silently
// ignores — operators discover the typo via `analyze_*` style
// follow-up tickets). [EveryoneRole] is always appended last if defined.
func (d *Declarative) effectiveRoles(p principal.Principal) []string {
	var out []string
	if name, ok := d.policy.Assignments[p.User]; ok {
		if _, defined := d.policy.Roles[name]; defined {
			out = append(out, name)
		}
	}
	if _, ok := d.policy.Roles[EveryoneRole]; ok {
		// Only append it if it isn't already the principal's explicit
		// role (defensive against `assignments: {alice: everyone}`).
		if !slices.Contains(out, EveryoneRole) {
			out = append(out, EveryoneRole)
		}
	}
	return out
}

// holdsPermission reports whether any of `roles` lists `perm` in its
// Permissions. Union semantics — one match is enough.
func (d *Declarative) holdsPermission(roles []string, perm string) bool {
	for _, name := range roles {
		role, ok := d.policy.Roles[name]
		if !ok {
			continue
		}
		if slices.Contains(role.Permissions, perm) {
			return true
		}
	}
	return false
}

// roleGrantsWrite reports whether `role.Write` covers `target` —
// either by an exact match or by the wildcard `"*"`.
func roleGrantsWrite(role RoleDef, target string) bool {
	for _, w := range role.Write {
		if w == "*" || w == target {
			return true
		}
	}
	return false
}
