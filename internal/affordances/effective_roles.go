package affordances

import (
	"context"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// globalRoles returns the principal's global (assignment-based) role
// set as a lookup map: Assignments[user] plus [acl.EveryoneRole] when
// the policy declares it. The "everyone" name is shared with
// acl.Declarative via the acl package constant so write-grant and
// affordance composition can't drift.
func (r *PolicyResolver) globalRoles(p principal.Principal) map[string]bool {
	out := map[string]bool{}
	if r.policy == nil {
		return out
	}
	if role, ok := r.policy.Assignments[p.User]; ok {
		if _, declared := r.policy.Roles[role]; declared {
			out[role] = true
		}
	}
	if _, ok := r.policy.Roles[acl.EveryoneRole]; ok {
		out[acl.EveryoneRole] = true
	}
	return out
}

// effectiveRoles returns the role names that apply to (principal,
// entity): global roles plus direct local roles conferred on the
// entity by a role-relation edge from the principal. The result is a
// stable-order-independent slice (callers iterate for union semantics).
func (r *PolicyResolver) effectiveRoles(
	ctx context.Context, p principal.Principal, e *entity.Entity, global map[string]bool,
) []string {
	set := map[string]bool{}
	for role := range global {
		set[role] = true
	}
	for confersRole, relTypes := range r.localRoleRelations {
		for _, rt := range relTypes {
			if r.lookup.HasEdge(ctx, p.User, rt, e.ID) {
				set[confersRole] = true
				break
			}
		}
	}
	out := make([]string, 0, len(set))
	for role := range set {
		out = append(out, role)
	}
	// Deterministic order so that attribution (which role is credited
	// for a deny) is stable across runs, not map-iteration-dependent
	// (DR S3 / RR-QV18).
	sort.Strings(out)
	return out
}

// localRoleRelations returns the role-relation types that confer role,
// used by the bindingContext's has_role local-role resolution.
func (bc *bindingContext) localRoleRelations(role string) []string {
	return bc.resolver.localRoleRelations[role]
}
