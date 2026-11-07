package acl

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// FlowEngineRole is the role [Policy.ValidateAgainstMetamodel] adds when the
// schema declares [entity.FlowRunType], assigned to [principal.UserFlow].
// It is the only role that may write a flow run or its managed edges
// (TKT-BC5G8RP). acl.yaml may not declare it, assign it, or name the
// managed types in any other role, so the grant is an allowlist compiled at
// load, not a runtime deny.
const FlowEngineRole = entity.ManagedPrefix + "flow-engine"

// wildcardCovers reports whether the grant entry "*" covers target. It
// covers every operator type and never a rela-managed one
// ([entity.IsManagedName]): a role reaches a managed type only by naming
// it, which Validate refuses to every role but [FlowEngineRole].
//
// Every type-matching site in this package uses it or [coversType], so a
// new site that compares against "*" directly fails the guard test
// (TestWildcardSitesExcludeManagedTypes).
func wildcardCovers(entry, target string) bool {
	return entry == "*" && !entity.IsManagedName(target)
}

// coversType reports whether a grant entry names target or is a wildcard
// that covers it.
func coversType(entry, target string) bool {
	return entry == target || wildcardCovers(entry, target)
}

// flowEngineRoleDef is the role [FlowEngineRole] names, in normalized
// form: Read holds entity types only, so Validate's world split leaves it
// unchanged.
func flowEngineRoleDef() RoleDef {
	return RoleDef{
		Description: "rela's flow engine: writes flow runs and their managed edges.",
		Create:      []string{entity.FlowRunType},
		Update:      []string{entity.FlowRunType},
		Delete:      []string{entity.FlowRunType},
		Read:        []string{"*", entity.FlowRunType},
	}
}

// validateManagedNames refuses an acl.yaml that names a rela-managed type,
// relation or role, or that assigns [principal.UserFlow]. Only
// injectFlowEngine may add those, so a policy it already
// processed skips exactly the role and assignment it added.
func validateManagedNames(p *Policy) error {
	for _, name := range slices.Sorted(maps.Keys(p.Roles)) {
		if name == FlowEngineRole && p.flowEngine {
			continue
		}
		if entity.IsManagedName(name) {
			return managedErr("roles", name, "a role name")
		}
		if err := validateRoleManagedNames(name, p.Roles[name]); err != nil {
			return err
		}
	}
	if err := validateManagedAssignments(p); err != nil {
		return err
	}
	for _, rel := range slices.Sorted(maps.Keys(p.RoleRelations)) {
		if entity.IsManagedName(rel) {
			return managedErr("role_relations", rel, "a relation type")
		}
		if entity.IsManagedName(p.RoleRelations[rel].Confers) {
			return managedErr("role_relations."+rel+".confers", p.RoleRelations[rel].Confers, "a role")
		}
	}
	for _, rel := range p.InheritRolesThrough {
		if entity.IsManagedName(rel) {
			return managedErr("inherit_roles_through", rel, "a relation type")
		}
	}
	for _, rel := range slices.Sorted(maps.Keys(p.RelationWriteGrants)) {
		if entity.IsManagedName(rel) {
			return managedErr("relation_grants", rel, "a relation type")
		}
	}
	if entity.IsManagedName(p.UserEntityType) {
		return managedErr("user_entity_type", p.UserEntityType, "an entity type")
	}
	if entity.IsManagedName(p.MembershipRelation) {
		return managedErr("membership_relation", p.MembershipRelation, "a relation type")
	}
	return nil
}

// validateManagedAssignments refuses an assignment of a managed role, and
// any assignment of [principal.UserFlow] except the one
// injectFlowEngine added.
func validateManagedAssignments(p *Policy) error {
	for _, who := range slices.Sorted(maps.Keys(p.Assignments)) {
		role := p.Assignments[who]
		if who == principal.UserFlow {
			if p.flowEngine && role == FlowEngineRole {
				continue
			}
			return fmt.Errorf("assignments.%s: rela assigns this identity itself; it cannot be assigned in acl.yaml",
				who)
		}
		if entity.IsManagedName(role) {
			return managedErr("assignments."+who, role, "a role")
		}
	}
	for _, who := range slices.Sorted(maps.Keys(p.AssertedRoles)) {
		if who == principal.UserFlow {
			return fmt.Errorf("asserted_role_assignments.%s: rela assigns this identity itself", who)
		}
		for _, role := range p.AssertedRoles[who] {
			if entity.IsManagedName(role) {
				return managedErr("asserted_role_assignments."+who, role, "a role")
			}
		}
	}
	return nil
}

// validateRoleManagedNames refuses a role grant that names a managed type
// or relation, in any verb list or keyed block.
func validateRoleManagedNames(name string, role RoleDef) error {
	lists := []struct {
		key  string
		list []string
	}{
		{"create", role.Create}, {"update", role.Update}, {"delete", role.Delete},
		{"convert", role.Convert}, {"rename", role.Rename}, {"read", role.Read},
	}
	for _, l := range lists {
		for _, entry := range l.list {
			if entity.IsManagedName(strings.TrimSpace(entry)) {
				return managedErr("roles."+name+"."+l.key, entry, "an entity type")
			}
		}
	}
	blocks := []struct {
		key  string
		keys []string
	}{
		{"fields", slices.Collect(maps.Keys(role.Fields))},
		{"visible", slices.Collect(maps.Keys(role.Visible))},
		{"options", slices.Collect(maps.Keys(role.Options))},
		{"relations", slices.Collect(maps.Keys(role.Relations))},
	}
	for _, b := range blocks {
		slices.Sort(b.keys)
		for _, k := range b.keys {
			if entity.IsManagedName(k) {
				return managedErr("roles."+name+"."+b.key, k, "an entity type")
			}
		}
	}
	for _, typ := range slices.Sorted(maps.Keys(role.Relations)) {
		for _, g := range role.Relations[typ] {
			if entity.IsManagedName(g.Relation) {
				return managedErr("roles."+name+".relations."+typ, g.Relation, "a relation type")
			}
		}
	}
	return nil
}

func managedErr(where, name, what string) error {
	return fmt.Errorf("%s: %q: %s starting with %q is reserved for rela", where, name, what, entity.ManagedPrefix)
}

// injectFlowEngine adds [FlowEngineRole] and its assignment to
// [principal.UserFlow] when meta declares [entity.FlowRunType], and records
// the subject types a run's `_carries` edge may point at.
//
// It is idempotent: a second call on the same policy rebuilds the same role
// and writes nothing when the targets are unchanged. Validate refused an
// operator role or assignment with these names before it runs.
func injectFlowEngine(p *Policy, meta MetamodelView) {
	if !meta.HasEntityType(entity.FlowRunType) {
		return
	}
	var targets []string
	for _, t := range meta.RelationInfo(entity.CarriesRelation).To {
		// RR-1QKRQPB: a managed type is never a subject, so the run gate
		// cannot recurse into itself.
		if !entity.IsManagedName(t) {
			targets = append(targets, t)
		}
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if p.flowEngine && slices.Equal(p.flowRunTargets, targets) {
		return
	}
	if p.Roles == nil {
		p.Roles = map[string]RoleDef{}
	}
	if p.Assignments == nil {
		p.Assignments = map[string]string{}
	}
	p.Roles[FlowEngineRole] = flowEngineRoleDef()
	p.Assignments[principal.UserFlow] = FlowEngineRole
	p.flowRunTargets = targets
	p.flowEngine = true
}

// holdsGlobalRead reports whether a global role of the principal names
// entityType in its read list. For a managed type that is only
// [FlowEngineRole], because "*" does not cover it.
func holdsGlobalRead(r *Request, entityType string) bool {
	for _, a := range r.Globals().Attributions {
		if role, ok := r.roleFor(a.Role); ok && roleGrantsRead(role, entityType) {
			return true
		}
	}
	return false
}

// flowRunReadQuery is the read gate of [entity.FlowRunType] for a principal
// without a global grant on it: a run is readable exactly when its subject
// is, through its `_carries` edge (TKT-BC5G8RP).
//
// Each subject type the principal may read contributes one branch whose
// endpoint carries that type's row gate, the fold `related(...)` already
// uses (gateHop). A subject type whose gate cannot fold into an
// endpoint (a read through inheritance or a face-split role relation)
// drops its branch: the answer is an allowlist, so dropping fails closed
// (RR-YHDXSPL, TKT-9N9Z17R). A run with no `_carries` edge, or whose
// subject was deleted, matches no branch and is hidden.
//
// The type-level client ceiling is not consulted for the run type itself:
// each branch's subject read query applies the ceiling, so an attenuated
// client reads the runs of the subjects it may read (RR-1SYZD5J).
//
// Every branch keeps its endpoint's type, even when the type is fully
// readable: an untyped `_carries` predicate would admit a run whose stored
// edge points at a type outside the targets (a type that later gained
// faces), which the reader may not read. A single branch needs no `Any`.
func flowRunReadQuery(ctx context.Context, r *Request) ReadQueryResult {
	carries := func(m *store.EndpointPredicate) *store.RelationPredicate {
		return &store.RelationPredicate{OfTypes: []string{entity.CarriesRelation}, EndpointMatch: m}
	}
	var branches []store.GraphBranch
	for _, t := range r.d.policy.flowRunTargets {
		m, err := r.gateHop(ctx, TraversalHop{EntityType: t})
		if err != nil {
			continue
		}
		branches = append(branches, store.GraphBranch{HasOutbound: carries(m)})
	}
	q := &store.GraphQuery{EntityType: entity.FlowRunType, SkipUnreadable: true}
	switch len(branches) {
	case 0:
		return ReadQueryResult{DenyAll: true}
	case 1:
		q.HasOutbound = branches[0].HasOutbound
	default:
		q.Any = branches
	}
	return ReadQueryResult{Query: q}
}

// flowRunReadRoutes reports why a reader may read a flow run: a run is read
// through its subject (flowRunReadQuery), so its routes are the routes by
// which the reader reads each subject its `_carries` edge points at. No role
// grants read on the run type itself except the engine's, so crediting the
// run type's own grants would report no route for a reader the runtime
// admits.
func flowRunReadRoutes(ctx context.Context, r *Request, runID string) ([]RoleAttribution, error) {
	subjects, err := r.d.graph.OutgoingRelations(ctx, runID, entity.CarriesRelation)
	if err != nil {
		return nil, fmt.Errorf("acl: flow run %q subjects: %w", runID, err)
	}
	seen := map[attrKey]bool{}
	var out []RoleAttribution
	for _, id := range subjects {
		for _, t := range r.d.policy.flowRunTargets {
			faces, err := r.d.faceMatcher.MatchingFaces(ctx,
				store.GraphQuery{EntityType: t, Faces: store.AllFaces()}, []string{id})
			if err != nil {
				return nil, fmt.Errorf("acl: flow run %q subject type: %w", runID, err)
			}
			if len(faces[id]) == 0 {
				continue
			}
			ok, err := r.PermitsRead(ctx, t, id)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			for _, a := range r.grantingAttributions(ctx, VerbRead, t, id) {
				if k := attrKey(a); !seen[k] {
					seen[k] = true
					out = append(out, a)
				}
			}
		}
	}
	return out, nil
}
