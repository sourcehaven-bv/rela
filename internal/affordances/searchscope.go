package affordances

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// blockVisible is the acl.yaml key of the property-visibility block.
const blockVisible = "visible"

// loadCheckIdentity stands in for a principal when the load-time rule lowers
// a `when:`. Any non-empty id works: the check asks whether the condition has
// a store form, and a `current_user.id` reference has one for every caller
// who has an identity.
const loadCheckIdentity = "load-check"

// requireLowerable enforces the load-time rule for `visible:` conditions:
// every `when:` must lower exactly to store predicates through
// [queryplan.LowerScope]. Search applies `visible:` inside the store query,
// so a condition the store cannot answer exactly would have to be answered
// per row in Go, by calling back into the ACL from the store. Refusing it at
// load keeps that callback impossible.
//
// What lowers: an AND of property equalities (`entity.x == 'v'`,
// `entity.x == current_user.id`, `is_current_user(entity.x)`) and
// `related(...)` terms. What does not: or, not, ordered comparisons, ~=, and
// the host functions has_role, has_global_role, has_relation, count_relations
// and has_current_user. Grant per-entity roles with role_relations, which
// `visible:` already honors.
func requireLowerable(
	meta *metamodel.Metamodel, roleName, entityType string, idx int, field string, prog *predicate.Program,
) error {
	if prog == nil {
		return nil
	}
	if _, ok := queryplan.LowerScope(prog, meta, entityType, loadCheckIdentity); ok {
		return nil
	}
	return fmt.Errorf("roles.%s.visible.%s[%d] (field %q).when: the condition cannot be answered by a store query; "+
		"a visible: condition must be an AND of property equalities and related(...) terms "+
		"(or, not, ordered comparisons, ~=, has_role, has_global_role, has_relation, count_relations and "+
		"has_current_user are refused; "+
		"grant per-entity access with role_relations)",
		roleName, entityType, idx, field)
}

// SearchFieldScope compiles the `visible:` grants of entityType for the
// request on ctx into a [search.FieldScope], the form a search backend
// applies to every row at once. The scope's Gated list is the type's declared
// properties plus any property a client ceiling names, so an undeclared
// property stays visible, as it does in [PolicyResolver.FieldVerdicts]. A nil
// scope means nothing is hidden on any row of the type.
//
// It reproduces [PolicyResolver.FieldVerdicts] exactly for live rows. A field
// is visible on a row when some role that applies to the row shows it, the
// union of roles (BUG-Y5UF3K):
//
//   - A role that declares `visible:` shows what its grants allow, each with
//     its own condition. Held globally, it applies on every row; held only
//     through a role relation, on the rows where it is held. Where it is held
//     is [acl.Request.LocalRolePredicate], the same predicate the row gate
//     uses.
//   - A role that declares no `visible:` block shows every field on the rows
//     it applies to whose face it reads ([acl.Request.RoleGrantsVerbOnFace]).
//   - A row where no role that declares the block applies shows every field.
//     When no global role declares it, those rows are matched by negating
//     each local-role predicate.
//   - A client ceiling then narrows every grant and can close the type on
//     its own.
//
// A condition that fails to lower here (only an empty identity can cause
// that, after the load-time rule) contributes nothing, which hides more and
// never less. The error is a missing or mismatched request, or a historical
// subject; callers fail closed.
func (r *PolicyResolver) SearchFieldScope(ctx context.Context, entityType string) (*search.FieldScope, error) {
	if r.policy == nil {
		return nil, nil //nolint:nilnil // nil scope is the documented "nothing hidden"
	}
	if IsHistoricalSubject(ctx) {
		return nil, errors.New("affordances: SearchFieldScope: historical subjects have no search scope")
	}
	req, err := r.requestFor(ctx)
	if err != nil {
		return nil, err
	}

	c := fieldScopeCompiler{
		meta: r.meta, policy: r.policy, grants: r.grants,
		req: req, entityType: entityType, isGlobal: globalRoleSet(req),
	}
	globalRoles, locals := c.heldVisibleRoles()
	ceiling := req.FieldCeilingFor(entityType)
	if len(globalRoles) == 0 && len(locals) == 0 && !ceiling.Constrains() {
		return nil, nil //nolint:nilnil // nil scope is the documented "nothing hidden"
	}

	universe := r.declaredFields(entityType)
	slices.Sort(universe)
	blockless, openEverywhere := c.blocklessFieldGrants(universe)
	if openEverywhere && !ceiling.Constrains() {
		return nil, nil //nolint:nilnil // a global block-less reader shows every field on every row
	}
	if c.identity = principal.From(ctx).User; c.identity == principal.Unknown {
		c.identity = ""
	}
	grants := c.globalFieldGrants(globalRoles)
	grants = append(grants, c.localFieldGrants(locals)...)
	grants = append(grants, blockless...)
	if len(globalRoles) == 0 {
		grants = append(grants, openFieldGrant(entityType, universe, locals))
	}

	scope := &search.FieldScope{Gated: gatedFields(universe, ceiling)}
	narrow := ceilingNarrower(ceiling)
	for _, g := range grants {
		if g.Fields = narrow(g.Fields); len(g.Fields) > 0 {
			scope.Grants = append(scope.Grants, g)
		}
	}
	return scope, nil
}

// fieldScopeCompiler compiles one type's `visible:` grants for one request.
// It holds the parts of [PolicyResolver] the compilation reads.
type fieldScopeCompiler struct {
	meta       *metamodel.Metamodel
	policy     *acl.Policy
	grants     map[grantKey]*compiledGrants
	req        *acl.Request
	entityType string
	identity   string          // the principal's user id, "" when unknown
	isGlobal   map[string]bool // the roles req holds globally
}

// localRole is a role that declares `visible:` and is held on some rows
// through a role relation: held matches those rows.
type localRole struct {
	name string
	held store.RelationPredicate
}

// globalRoleSet returns the names of the roles req holds globally.
func globalRoleSet(req *acl.Request) map[string]bool {
	isGlobal := map[string]bool{}
	for _, a := range req.Globals().Attributions {
		isGlobal[a.Role] = true
	}
	return isGlobal
}

// heldVisibleRoles splits the roles declaring `visible:` for the type into
// those the request holds globally and those it may hold through a role
// relation. A role it can hold neither way plays no part.
func (c *fieldScopeCompiler) heldVisibleRoles() (global []string, locals []localRole) {
	for _, role := range c.visibleRoles() {
		if c.isGlobal[role] {
			global = append(global, role)
			continue
		}
		if pred, ok := c.req.LocalRolePredicate(role); ok {
			locals = append(locals, localRole{name: role, held: pred})
		}
	}
	return global, locals
}

// globalFieldGrants is the grants of roles held globally: they apply on
// every row, each with its own condition. The unconditional fields merge
// into one grant.
func (c *fieldScopeCompiler) globalFieldGrants(roles []string) []search.FieldGrant {
	var out []search.FieldGrant
	var unconditional []string
	for _, role := range roles {
		for _, fg := range c.grants[grantKey{role, c.entityType}].visible {
			if fg.program == nil {
				unconditional = append(unconditional, fg.field)
				continue
			}
			if when, ok := c.lowerWhen(fg.program); ok {
				out = append(out, search.FieldGrant{Fields: []string{fg.field}, When: when})
			}
		}
	}
	if len(unconditional) > 0 {
		out = append(out, search.FieldGrant{Fields: dedupe(unconditional)})
	}
	return out
}

// localFieldGrants is the grants of roles held through a role relation:
// each applies where the role is held AND its condition passes. A role's
// unconditional fields merge into one grant, as [fieldScopeCompiler.globalFieldGrants]
// merges a global role's, so they cost one predicate per role, not per field.
func (c *fieldScopeCompiler) localFieldGrants(locals []localRole) []search.FieldGrant {
	var out []search.FieldGrant
	for _, lr := range locals {
		held := store.DirectedRelation{Incoming: true, Pred: lr.held}
		var unconditional []string
		for _, fg := range c.grants[grantKey{lr.name, c.entityType}].visible {
			if fg.program == nil {
				unconditional = append(unconditional, fg.field)
				continue
			}
			when, ok := c.lowerWhen(fg.program)
			if !ok {
				continue
			}
			when.Related = append(when.Related, held)
			out = append(out, search.FieldGrant{Fields: []string{fg.field}, When: when})
		}
		if len(unconditional) > 0 {
			out = append(out, search.FieldGrant{
				Fields: dedupe(unconditional),
				When: &store.GraphQuery{
					EntityType: c.entityType, Faces: store.AllFaces(), Related: []store.DirectedRelation{held},
				},
			})
		}
	}
	return out
}

// blocklessFieldGrants is the contribution of the roles that declare no
// `visible:` block for the type (BUG-Y5UF3K): every declared property, on
// the rows the role applies to whose face it reads. A global role applies on
// every row; a role held through a role relation, where it is held.
// openEverywhere reports that a global role reads every face, so every field
// is visible on every row before the client ceiling.
func (c *fieldScopeCompiler) blocklessFieldGrants(
	universe []string,
) (grants []search.FieldGrant, openEverywhere bool) {
	entityType := c.entityType
	faces := declaredFaces(c.meta.Entities[entityType].Faces)
	for _, role := range sortedRoleNames(c.policy) {
		if g := c.grants[grantKey{role, entityType}]; g != nil && g.declaredVisible {
			continue
		}
		read := slices.DeleteFunc(slices.Clone(faces), func(f entity.Face) bool {
			return !c.req.RoleGrantsVerbOnFace(role, acl.VerbRead, entityType, f)
		})
		if len(read) == 0 {
			continue
		}
		var faceIn []entity.Face
		if len(read) < len(faces) {
			faceIn = read
		}
		if c.isGlobal[role] {
			if faceIn == nil {
				openEverywhere = true
				grants = append(grants, search.FieldGrant{Fields: universe})
				continue
			}
			grants = append(grants, search.FieldGrant{Fields: universe, When: &store.GraphQuery{
				EntityType: entityType, Faces: store.AllFaces(), FaceIn: faceIn,
			}})
			continue
		}
		held, ok := c.req.LocalRolePredicate(role)
		if !ok {
			continue
		}
		grants = append(grants, search.FieldGrant{Fields: universe, When: &store.GraphQuery{
			EntityType: entityType, Faces: store.AllFaces(), FaceIn: faceIn,
			Related: []store.DirectedRelation{{Incoming: true, Pred: held}},
		}})
	}
	return grants, openEverywhere
}

// sortedRoleNames returns the policy's role names in a stable order.
func sortedRoleNames(p *acl.Policy) []string {
	out := make([]string, 0, len(p.Roles))
	for name := range p.Roles {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// openFieldGrant shows every declared property on the rows where no role
// declaring `visible:` is held: no role is held globally, so those are the
// rows where none of the local roles is held either.
func openFieldGrant(entityType string, universe []string, locals []localRole) search.FieldGrant {
	g := search.FieldGrant{Fields: universe}
	for _, lr := range locals {
		if g.When == nil {
			g.When = &store.GraphQuery{EntityType: entityType, Faces: store.AllFaces()}
		}
		notHeld := lr.held
		notHeld.Negate = true
		g.When.Related = append(g.When.Related, store.DirectedRelation{Incoming: true, Pred: notHeld})
	}
	return g
}

// visibleRoles lists, sorted, the roles that declare `visible:` for the
// type.
func (c *fieldScopeCompiler) visibleRoles() []string {
	var out []string
	for k, g := range c.grants {
		if k.entityType == c.entityType && g.declaredVisible {
			out = append(out, k.role)
		}
	}
	slices.Sort(out)
	return out
}

// lowerWhen turns a grant condition into the store query that matches the
// rows it passes on. ok is false when it does not lower for the identity;
// the grant then passes nowhere, which is also what PolicyResolver.passes
// answers for a condition on current_user with no identity.
//
// A condition with related(...) is answered as the grant's traversal binder
// answers it: ungated, every face of the endpoint, and never for a row on a
// named face (traversalFor refuses those).
func (c *fieldScopeCompiler) lowerWhen(prog *predicate.Program) (*store.GraphQuery, bool) {
	entityType := c.entityType
	lowered, ok := queryplan.LowerScope(prog, c.meta, entityType, c.identity)
	if !ok {
		return nil, false
	}
	q := &store.GraphQuery{EntityType: entityType, Faces: store.AllFaces(), Props: lowered.Props}
	for _, spec := range lowered.Traversals {
		hop, err := relresolve.Hop(c.meta, entityType, spec)
		if err != nil {
			return nil, false
		}
		pred, err := acl.UngatedTraversal(hop)
		if err != nil || pred == nil {
			return nil, false
		}
		if pred.EndpointMatch != nil {
			pred.EndpointMatch.Faces = store.AllFaces()
		}
		q.Related = append(q.Related, store.DirectedRelation{Incoming: hop.Incoming, Pred: *pred})
	}
	if len(q.Related) > 0 {
		q.FaceIn = []entity.Face{""}
	}
	return q, true
}

// gatedFields is the universe a scope's grants decide: the declared
// properties plus every property a ceiling names.
func gatedFields(universe []string, ceiling acl.FieldCeiling) []string {
	out := slices.Clone(universe)
	out = append(out, ceiling.Visible...)
	out = append(out, ceiling.Redact...)
	return dedupe(out)
}

// ceilingNarrower returns the function that removes what a client ceiling
// withholds from a grant's field list. A ceiling only ever removes.
func ceilingNarrower(c acl.FieldCeiling) func([]string) []string {
	switch {
	case c.Visible != nil:
		return func(fields []string) []string {
			return slices.DeleteFunc(slices.Clone(fields), func(f string) bool {
				return !slices.Contains(c.Visible, f)
			})
		}
	case len(c.Redact) > 0:
		return func(fields []string) []string {
			return slices.DeleteFunc(slices.Clone(fields), func(f string) bool { return slices.Contains(c.Redact, f) })
		}
	default:
		return func(fields []string) []string { return fields }
	}
}

func dedupe(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}
