package affordances

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// FieldVerdicts carries per-entity field-level affordance decisions.
// All maps are sparse: an absent key means the permissive default
// (writable / visible / option allowed). The data-entry adapter maps
// this onto its own wire-shape verdict type.
type FieldVerdicts struct {
	Writable map[string]bool
	Visible  map[string]bool
	Options  map[string]map[string]bool
	// Attribution maps a denied field (or "field=option") to the
	// role/grant that produced the deny, for the audit Summary channel
	// (DR-C5). Sparse — only denies appear; never surfaced on the wire.
	Attribution map[string]string
}

// RelationVerdicts carries per-entity relation-level decisions, sparse
// by relation type.
type RelationVerdicts struct {
	Types map[string]RelationVerdict
}

// RelationVerdict is the decision for one relation type.
type RelationVerdict struct {
	Creatable bool
	Removable bool
	Fields    map[string]bool
	// Attribution maps a denied dimension ("create", "remove",
	// "fields.<name>") to the role/grant that denied it.
	Attribution map[string]string
}

// PolicyResolver answers field/option/relation affordance queries from
// a compiled acl.yaml policy. Construct with [New]; safe for concurrent
// use.
type PolicyResolver struct {
	policy *acl.Policy
	meta   *metamodel.Metamodel
	lookup RelationLookup

	// envs holds the compiled predicate env per entity type, reused
	// across grants of that type.
	envs map[string]*predicate.Env

	// grants is indexed by (role, entityType) → compiled grant blocks.
	grants map[grantKey]*compiledGrants

	// localRoleRelations maps a conferred role name → the relation
	// types that confer it (from policy.RoleRelations).
	localRoleRelations map[string][]string
}

type grantKey struct {
	role       string
	entityType string
}

// compiledGrants is the per-(role, type) bundle of compiled grants. A
// nil slice value distinguishes "block declared (opt-in)" from "block
// absent"; the presence flags record which blocks the role declared
// for this type.
type compiledGrants struct {
	fields    []compiledFieldGrant
	visible   []compiledFieldGrant
	options   []compiledOptionGrant
	relations []compiledRelationGrant

	declaredFields    bool
	declaredVisible   bool
	declaredOptions   bool
	declaredRelations bool
}

// New compiles the policy's affordance grants against the metamodel
// and returns a PolicyResolver. Every grant's `when:` predicate is
// compiled up front; all compile errors are collected and joined so an
// operator sees every failure in one pass (DR-S2). meta and lookup
// must be non-nil; policy may be nil (yields an all-permissive
// resolver).
//
// The full *metamodel.Metamodel is required (not a narrower slice):
// the resolver can be asked about any entity type at runtime, and for
// each it needs that type's property defs (to build the predicate env
// and coerce values) and the relation defs (to validate relation
// grant targets). Narrowing to a per-type view would just move the
// whole-metamodel dependency to the caller.
func New(policy *acl.Policy, meta *metamodel.Metamodel, lookup RelationLookup) (*PolicyResolver, error) {
	if meta == nil {
		return nil, errors.New("affordances: New: meta must be non-nil")
	}
	if lookup == nil {
		return nil, errors.New("affordances: New: lookup must be non-nil")
	}
	r := &PolicyResolver{
		policy:             policy,
		meta:               meta,
		lookup:             lookup,
		envs:               map[string]*predicate.Env{},
		grants:             map[grantKey]*compiledGrants{},
		localRoleRelations: map[string][]string{},
	}
	if policy == nil {
		return r, nil
	}

	for confRel, def := range policy.RoleRelations {
		if def.Confers != "" {
			r.localRoleRelations[def.Confers] = append(r.localRoleRelations[def.Confers], confRel)
		}
	}

	var errs []error
	for roleName, role := range policy.Roles {
		errs = append(errs, r.compileRole(roleName, role)...)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return r, nil
}

// env returns (compiling on first use) the predicate env for an entity
// type. Envs are cached so every grant of a type shares one.
func (r *PolicyResolver) env(entityType string) (*predicate.Env, error) {
	if e, ok := r.envs[entityType]; ok {
		return e, nil
	}
	e, err := buildEnv(r.meta, entityType)
	if err != nil {
		return nil, err
	}
	r.envs[entityType] = e
	return e, nil
}

// compileRole compiles every grant block of one role across all the
// entity types it mentions, recording per-(role, type) compiled
// grants. Returns the collected compile errors (path-prefixed).
func (r *PolicyResolver) compileRole(roleName string, role acl.RoleDef) []error {
	var errs []error
	get := func(entityType string) *compiledGrants {
		k := grantKey{roleName, entityType}
		g := r.grants[k]
		if g == nil {
			g = &compiledGrants{}
			r.grants[k] = g
		}
		return g
	}

	for et, grants := range role.Fields {
		g := get(et)
		g.declaredFields = true
		errs = append(errs, r.compileFieldBlock(roleName, et, "fields", grants, &g.fields)...)
	}
	for et, grants := range role.Visible {
		g := get(et)
		g.declaredVisible = true
		errs = append(errs, r.compileFieldBlock(roleName, et, "visible", grants, &g.visible)...)
	}
	for et, grants := range role.Options {
		g := get(et)
		g.declaredOptions = true
		errs = append(errs, r.compileOptionBlock(roleName, et, grants, &g.options)...)
	}
	for et, grants := range role.Relations {
		g := get(et)
		g.declaredRelations = true
		for i, rg := range grants {
			errs = append(errs, r.compileRelationGrant(g, roleName, et, i, rg)...)
		}
	}
	return errs
}

// compileFieldBlock validates + compiles a fields/visible block,
// appending compiled grants to out. block is the YAML key for error
// paths ("fields" or "visible").
func (r *PolicyResolver) compileFieldBlock(
	roleName, entityType, block string, grants []acl.FieldGrant, out *[]compiledFieldGrant,
) []error {
	var errs []error
	for i, fg := range grants {
		if verr := r.validateField(roleName, entityType, block, i, fg.Field); verr != nil {
			errs = append(errs, verr)
			continue
		}
		prog, err := r.compile(roleName, entityType, block, i, fg.When)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*out = append(*out, compiledFieldGrant{field: fg.Field, program: prog})
	}
	return errs
}

// compileOptionBlock validates + compiles an options block.
func (r *PolicyResolver) compileOptionBlock(
	roleName, entityType string, grants []acl.OptionGrant, out *[]compiledOptionGrant,
) []error {
	var errs []error
	for i, og := range grants {
		if verr := r.validateOption(roleName, entityType, i, og.Field, og.Option); verr != nil {
			errs = append(errs, verr)
			continue
		}
		prog, err := r.compile(roleName, entityType, "options", i, og.When)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*out = append(*out, compiledOptionGrant{field: og.Field, option: og.Option, program: prog})
	}
	return errs
}

func (r *PolicyResolver) compileRelationGrant(
	g *compiledGrants, roleName, entityType string, i int, rg acl.RelationGrant,
) []error {
	var errs []error
	if verr := r.validateRelation(roleName, entityType, i, rg.Relation); verr != nil {
		// Unknown relation type: report and skip the whole grant —
		// none of its dimensions can meaningfully gate anything.
		return append(errs, verr)
	}
	prog, err := r.compile(roleName, entityType, "relations", i, rg.When)
	if err != nil {
		errs = append(errs, err)
	}
	cr := compiledRelationGrant{
		relation: rg.Relation,
		create:   rg.Create,
		remove:   rg.Remove,
		program:  prog,
	}
	metaFailed := false
	for j, fg := range rg.Fields {
		fprog, ferr := r.compile(roleName, entityType,
			fmt.Sprintf("relations[%d].fields", i), j, fg.When)
		if ferr != nil {
			errs = append(errs, ferr)
			metaFailed = true
			continue
		}
		cr.fields = append(cr.fields, compiledFieldGrant{field: fg.Field, program: fprog})
	}
	// Append the grant only when it compiled cleanly end-to-end. A
	// grant that lost a meta field to a compile error must not be
	// half-installed (S4): silently dropping the field would flip a
	// closed-world meta deny into permissive if New ever relaxed to
	// warn-and-continue. New currently hard-fails on any collected
	// error, so this is belt-and-suspenders.
	if err == nil && !metaFailed {
		g.relations = append(g.relations, cr)
	}
	return errs
}

// compile compiles one grant predicate. An empty `when` yields a nil
// program, which the evaluator (passes) treats as an unconditional
// grant — nil is the intended sentinel here, not an error-absent
// invalid value.
//
//nolint:nilnil // nil program is the documented "unconditional" sentinel
func (r *PolicyResolver) compile(roleName, entityType, block string, idx int, when string) (*predicate.Program, error) {
	if when == "" {
		return nil, nil
	}
	env, err := r.env(entityType)
	if err != nil {
		return nil, fmt.Errorf("roles.%s.%s.%s[%d]: %w", roleName, block, entityType, idx, err)
	}
	prog, err := predicate.Compile(env, when)
	if err != nil {
		return nil, fmt.Errorf("roles.%s.%s.%s[%d].when: %w", roleName, block, entityType, idx, err)
	}
	return prog, nil
}

// FieldVerdicts computes the sparse field-level verdicts for e against
// the principal carried on ctx.
func (r *PolicyResolver) FieldVerdicts(ctx context.Context, e *entity.Entity) FieldVerdicts {
	out := FieldVerdicts{}
	if e == nil || r.policy == nil {
		return out
	}
	bc, roles := r.bindingFor(ctx, e)
	if bc == nil {
		return out
	}

	writable := newDimension()
	visible := newDimension()
	options := newOptionDimension()

	for _, role := range roles {
		g := r.grants[grantKey{role, e.Type}]
		if g == nil {
			continue
		}
		if g.declaredFields {
			r.applyFieldGrants(bc, role, "read-only", g.fields, writable)
		}
		if g.declaredVisible {
			r.applyFieldGrants(bc, role, "hidden", g.visible, visible)
		}
		if g.declaredOptions {
			r.applyOptionGrants(bc, role, g.options, options)
		}
	}

	fieldUniverse := r.declaredFields(e.Type)
	out.Writable, out.Attribution = writable.deny(fieldUniverse, out.Attribution)
	visMap, attr := visible.deny(fieldUniverse, out.Attribution)
	out.Visible = visMap
	out.Attribution = attr
	out.Options, out.Attribution = options.deny(r.enumOptions(e.Type), out.Attribution)
	return out
}

// declaredFields returns the metamodel-declared property names for an
// entity type — the closed-world universe a fields/visible block
// denies from. Unknown type yields an empty set.
func (r *PolicyResolver) declaredFields(entityType string) []string {
	def, ok := r.meta.Entities[entityType]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(def.Properties))
	for name := range def.Properties {
		out = append(out, name)
	}
	return out
}

// enumOptions returns, per enum-typed field, its declared option
// values — the closed-world universe an options block denies from.
func (r *PolicyResolver) enumOptions(entityType string) map[string][]string {
	def, ok := r.meta.Entities[entityType]
	if !ok {
		return nil
	}
	out := map[string][]string{}
	for name, prop := range def.Properties {
		values := prop.Values
		if len(values) == 0 {
			if ct, ok := r.meta.Types[prop.Type]; ok {
				values = ct.Values
			}
		}
		if len(values) > 0 {
			out[name] = values
		}
	}
	return out
}

// RelationVerdicts computes the sparse relation-level verdicts for e.
func (r *PolicyResolver) RelationVerdicts(ctx context.Context, e *entity.Entity) RelationVerdicts {
	out := RelationVerdicts{}
	if e == nil || r.policy == nil {
		return out
	}
	bc, roles := r.bindingFor(ctx, e)
	if bc == nil {
		return out
	}

	acc := newRelationAccumulator()
	for _, role := range roles {
		g := r.grants[grantKey{role, e.Type}]
		if g == nil || !g.declaredRelations {
			continue
		}
		for _, rg := range g.relations {
			grantPassed := r.passes(bc, rg.program, role)
			metaPassed := r.metaFieldResults(bc, role, rg, grantPassed)
			acc.observe(role, rg, grantPassed, metaPassed)
		}
	}
	out.Types = acc.verdicts()
	return out
}

// bindingFor resolves the effective role set for (principal, entity)
// and builds the binding context shared across grant evaluations for
// this call. Returns nil bc when no policy roles apply.
func (r *PolicyResolver) bindingFor(ctx context.Context, e *entity.Entity) (bc *bindingContext, roles []string) {
	p := principal.From(ctx)
	global := r.globalRoles(p)
	roles = r.effectiveRoles(ctx, p, e, global)
	if len(roles) == 0 {
		return nil, nil
	}
	bc = &bindingContext{
		principal:   p,
		entity:      e,
		globalRoles: global,
		lookup:      r.lookup,
		userID:      p.User,
		resolver:    r,
	}
	return bc, roles
}

// passes reports whether a grant's predicate evaluates true. A nil
// program is unconditional (true). A predicate runtime error fails
// closed (false) with a slog.Warn for operator visibility (DR-S5).
func (r *PolicyResolver) passes(bc *bindingContext, prog *predicate.Program, role string) bool {
	if prog == nil {
		return true
	}
	b, err := bc.newBindings(r.meta)
	if err != nil {
		slog.Warn("affordances: binding build failed; denying grant",
			"role", role, "entity", bc.entity.ID, "error", err)
		return false
	}
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		slog.Warn("affordances: predicate eval failed; denying grant",
			"role", role, "entity", bc.entity.ID, "error", err)
		return false
	}
	boolV, ok := v.(predicate.Bool)
	if !ok {
		slog.Warn("affordances: predicate did not return bool; denying grant",
			"role", role, "entity", bc.entity.ID)
		return false
	}
	return boolV.Bool()
}

// applyFieldGrants marks each granted field allowed when its predicate
// passes. ruleKind is the denial-rule label ("read-only" or "hidden")
// recorded in attribution for fields that end up denied.
func (r *PolicyResolver) applyFieldGrants(
	bc *bindingContext, role, ruleKind string, grants []compiledFieldGrant, dim *dimension,
) {
	dim.optIn(ruleKind)
	for _, fg := range grants {
		if r.passes(bc, fg.program, role) {
			dim.allow(fg.field)
		} else {
			dim.observeDeny(fg.field, role)
		}
	}
}

func (r *PolicyResolver) applyOptionGrants(
	bc *bindingContext, role string, grants []compiledOptionGrant, dim *optionDimension,
) {
	for _, og := range grants {
		dim.optIn(og.field)
		if r.passes(bc, og.program, role) {
			dim.allow(og.field, og.option)
		} else {
			dim.observeDeny(og.field, og.option, role)
		}
	}
}

// metaFieldResults evaluates each meta-field grant's own predicate.
// A meta field is allowed only when BOTH the whole-grant predicate
// AND the meta-field predicate pass — a meta field can be more
// restrictive than its relation grant, never less. Returns
// field → passed.
func (r *PolicyResolver) metaFieldResults(
	bc *bindingContext, role string, rg compiledRelationGrant, grantPassed bool,
) map[string]bool {
	if len(rg.fields) == 0 {
		return nil
	}
	out := make(map[string]bool, len(rg.fields))
	for _, fg := range rg.fields {
		out[fg.field] = grantPassed && r.passes(bc, fg.program, role)
	}
	return out
}
