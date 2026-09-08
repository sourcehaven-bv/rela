package acl

import (
	"slices"
	"strings"
)

// compiledCeiling is the resolved, per-request ceiling: what a client of a
// given principal_type, carrying a given scope set, may do — BEFORE
// intersecting with the acting user's own grants.
//
// # Why this is a matcher, not an expanded set
//
// The obvious implementation is to expand every wildcard against the metamodel
// at load ("read: [*] minus person" → the literal list of every other type) and
// store the result. This does NOT do that, for two reasons:
//
//  1. Expansion is the one step that can fail OPEN. Getting the type universe
//     wrong — stale metamodel, a type added later, a subtly wrong set
//     difference — silently produces a ceiling that permits more than written.
//     A matcher has no universe to get wrong: `deny_read: [person]` is checked
//     by asking "is this type person?", which cannot over-permit.
//  2. The verb predicates ([grantsVerb], [roleGrantsRead]) already match
//     wildcards rather than expanding them, so an expanded set would have to be
//     re-collapsed to interoperate. Matching composes directly.
//
// Fields are absent here on purpose: the closed-world `visible:` universe is
// resolved at EVALUATION time by internal/affordances against the entity's
// declared properties (its declaredFields), so the field axis of a ceiling is
// applied there in the same vocabulary, not compiled here.
type compiledCeiling struct {
	// name identifies the matched baseline, for diagnostics and attribution.
	// Empty when no baseline matched (i.e. the principal is unrestricted).
	name string

	// active is false when no baseline matched. Distinguished from a
	// zero-valued ceiling because "no baseline" (unrestricted) and "a baseline
	// that permits nothing" are opposite outcomes and must not be confused.
	active bool

	// baseline is the matched block, kept so the field axis
	// ([Request.FieldCeilingFor]) reads the SAME resolution the verb axis was
	// compiled from rather than re-deriving it per call. One source of truth,
	// and no map walk on every field verdict in a list response.
	baseline ClientBaseline

	read, create, update, del verbCeiling
	permissions               permissionCeiling

	// worlds narrows the world read axis (TKT-DN37J2). Reuses verbCeiling
	// because the allow/deny/except semantics are identical — only the
	// vocabulary differs (world names, not entity types) and there is no
	// wildcard to expand.
	worlds verbCeiling
}

// verbCeiling decides whether one verb is permitted on one entity type.
// Exactly one of allow/deny is populated, mirroring the one-spelling-per-axis
// rule enforced at load.
type verbCeiling struct {
	// allow, when non-nil, is the closed allowlist: only these types (or "*").
	// A non-nil EMPTY slice means "nothing permitted" — distinct from nil,
	// which means the axis was omitted and is therefore inherited.
	allow []string
	// deny lists types removed from whatever the user holds. "*" removes all.
	deny []string
	// except lists types a scope grant carved back OUT of deny. It exists
	// because `deny_write: ["*"]` — the "read-only client" one-liner, and the
	// single most common ceiling — cannot be widened by subtraction: removing
	// "ticket" from ["*"] leaves ["*"], which still denies everything. So a
	// re-opened type is recorded here and checked ahead of the wildcard.
	except []string
}

// permits reports whether the ceiling admits target. An omitted axis (both nil)
// inherits — it does not narrow.
func (v verbCeiling) permits(target string) bool {
	// A scope-granted exception wins over the denial it was carved from. This
	// is checked FIRST so it can override a `deny: ["*"]` wildcard.
	if slices.Contains(v.except, target) {
		return true
	}
	if v.allow != nil {
		return matchesTypeList(v.allow, target)
	}
	if len(v.deny) > 0 {
		return !matchesTypeList(v.deny, target)
	}
	return true
}

// narrows reports whether this axis constrains anything at all.
func (v verbCeiling) narrows() bool { return v.allow != nil || len(v.deny) > 0 }

// permissionCeiling decides whether one named permission is withheld.
type permissionCeiling struct {
	allow  []string // non-nil: only these permissions survive
	deny   []string // these permissions are withheld
	except []string // scope-granted carve-outs; see [verbCeiling.except]
}

func (p permissionCeiling) permits(name string) bool {
	if slices.Contains(p.except, name) {
		return true
	}
	if p.allow != nil {
		return slices.Contains(p.allow, name) || slices.Contains(p.allow, "*")
	}
	if len(p.deny) > 0 {
		return !slices.Contains(p.deny, name) && !slices.Contains(p.deny, "*")
	}
	return true
}

func (p permissionCeiling) narrows() bool { return p.allow != nil || len(p.deny) > 0 }

// matchesTypeList reports whether target is covered by list, honoring the "*"
// wildcard the verb predicates already use.
func matchesTypeList(list []string, target string) bool {
	for _, t := range list {
		if t == "*" || t == target {
			return true
		}
	}
	return false
}

// ceilingFor resolves the ceiling for a principal: the baseline matching its
// principal_type, widened by every scope grant its scope claim names.
//
// Returns an inactive ceiling when no baseline matches — an unrecognized (or
// absent) principal_type is UNRESTRICTED, which is what makes an interactive
// `user` token, a `--principal-header` deployment and a provider that models no
// principal type all work without special-casing. The audit surfaces a
// principal_type no baseline covers, since that is a policy gap rather than a
// runtime error.
func (p *Policy) ceilingFor(principalType string, scopes []string) compiledCeiling {
	name, baseline, ok := p.baselineFor(principalType)
	if !ok {
		return compiledCeiling{}
	}

	c := compiledCeiling{
		name:     name,
		active:   true,
		baseline: baseline,
		read:     verbCeiling{allow: nilIfUnset(baseline.Read), deny: baseline.DenyRead},
		create:   verbCeiling{allow: nilIfUnset(baseline.Create), deny: baseline.DenyCreate},
		update:   verbCeiling{allow: nilIfUnset(baseline.Update), deny: baseline.DenyUpdate},
		del:      verbCeiling{allow: nilIfUnset(baseline.Delete), deny: baseline.DenyDelete},
		worlds:   verbCeiling{allow: nilIfUnset(baseline.Worlds), deny: baseline.DenyWorlds},
		permissions: permissionCeiling{
			allow: nilIfUnset(baseline.Permissions),
			deny:  baseline.DenyPermissions,
		},
	}

	// Every scope the token carries that names a grant re-opens its listed
	// capability. Scopes UNION: more scopes means more capability, which is how
	// OAuth scopes behave everywhere else. This cannot escalate, because the
	// result is still intersected with the acting user's own grants downstream.
	//
	// An unknown scope contributes nothing — matching the Assignments guard,
	// and meaning an IdP cannot invent capability by minting a scope name the
	// operator never wrote.
	for _, scope := range scopes {
		g, found := p.ScopeGrants[strings.TrimSpace(scope)]
		if !found {
			continue
		}
		c.read.reopen(g.Read)
		c.create.reopen(g.Create)
		c.update.reopen(g.Update)
		c.del.reopen(g.Delete)
		c.worlds.reopen(g.Worlds)
		c.permissions.reopen(g.Permissions)
	}
	return c
}

// baselineFor finds the single baseline covering principalType. Load-time
// validation guarantees applies_to sets are disjoint, so the first match is the
// only match and iteration order cannot change the answer.
func (p *Policy) baselineFor(principalType string) (string, ClientBaseline, bool) {
	principalType = strings.TrimSpace(principalType)
	if principalType == "" || len(p.ClientBaselines) == 0 {
		return "", ClientBaseline{}, false
	}
	for _, name := range sortedBaselineNames(p.ClientBaselines) {
		b := p.ClientBaselines[name]
		if slices.Contains(b.AppliesTo, principalType) {
			return name, b, true
		}
	}
	return "", ClientBaseline{}, false
}

// reopen widens a verb axis by the types a scope grant names.
//
// The two spellings widen differently, and both directions are "permit more":
//
//   - allowlist: append the types, so they now pass the closed list.
//   - denylist: record them as exceptions, so they stop being blocked.
//
// Exceptions rather than list-subtraction because the most common denial is
// `deny_write: ["*"]`, and subtracting "ticket" from ["*"] leaves ["*"] — the
// scope would silently fail to re-open anything. See [verbCeiling.except].
//
// A scope naming a type on an axis that is already unrestricted (both nil) is a
// no-op: there is nothing to widen.
func (v *verbCeiling) reopen(types []string) {
	if len(types) == 0 {
		return
	}
	if v.allow != nil {
		v.allow = append(slices.Clone(v.allow), types...)
	}
	if len(v.deny) > 0 {
		v.except = append(slices.Clone(v.except), types...)
	}
}

// reopen widens the permission axis, mirroring [verbCeiling.reopen].
func (p *permissionCeiling) reopen(names []string) {
	if len(names) == 0 {
		return
	}
	if p.allow != nil {
		p.allow = append(slices.Clone(p.allow), names...)
	}
	if len(p.deny) > 0 {
		p.except = append(slices.Clone(p.except), names...)
	}
}

// nilIfUnset preserves the load-time distinction between an omitted axis (nil →
// inherit) and an explicitly empty one (non-nil empty → permit nothing). YAML
// decodes `read: []` to an empty non-nil slice and an absent key to nil, so the
// distinction survives to here for free — but it is load-bearing enough to name.
func nilIfUnset(v []string) []string {
	if v == nil {
		return nil
	}
	return slices.Clone(v)
}

// clamp returns role narrowed to what this ceiling permits, or false when the
// ceiling leaves the role with nothing at all.
//
// This is THE clamp point. Every evaluation path in this package resolves a
// role name to a RoleDef and then asks a predicate of it; narrowing the RoleDef
// here means read gating, write authorization, permission checks and the
// affordance resolver all inherit the ceiling without a single call site
// knowing it exists.
//
// The result is a plain RoleDef holding plain allowlists — the runtime never
// learns the word "deny". See the commentary in ceiling.go for why that is the
// design and not an accident.
func (c compiledCeiling) clamp(role RoleDef) RoleDef {
	if !c.active {
		return role
	}
	role.Read = filterTypes(role.Read, c.read)
	role.Create = filterTypes(role.Create, c.create)
	role.Update = filterTypes(role.Update, c.update)
	role.Delete = filterTypes(role.Delete, c.del)
	role.Worlds = filterWorlds(role.Worlds, c.worlds)
	role.Permissions = filterPermissions(role.Permissions, c.permissions)
	return role
}

// filterWorlds intersects a role's world grants with the ceiling's world
// axis. Simpler than [filterTypes] because neither side can hold a
// wildcard (normalizeWorldGrants rejects `world:*`, and a ceiling world
// list is plain names), so there is no "everything" to collapse.
//
// This handles the ALLOW direction only. A ceiling that DENIES the default
// world cannot be expressed here at all: the default world is the absence
// of an entry, so an empty Worlds stays empty under any intersection and
// still means "default world". That denial lives in
// [compiledCeiling.permitsWorld], which every world check consults after
// the role predicate says yes.
func filterWorlds(roleWorlds []string, w verbCeiling) []string {
	if !w.narrows() || len(roleWorlds) == 0 {
		return roleWorlds
	}
	out := make([]string, 0, len(roleWorlds))
	for _, name := range roleWorlds {
		if w.permits(name) {
			out = append(out, name)
		}
	}
	return out
}

// filterTypes intersects a role's type list with a verb ceiling.
//
// The wildcard case is the subtle one. A role granting `["*"]` under a ceiling
// of `allow: [ticket]` must become `[ticket]`, NOT stay `["*"]` — otherwise the
// ceiling would be silently ignored, which is the fail-open direction. So when
// the role holds a wildcard and the ceiling has a closed allowlist, the
// ceiling's list becomes the result: it is by construction a subset of "every
// type".
//
// A wildcard under a pure DENY ceiling is different: a plain list cannot spell
// "everything except person", and expanding it would reintroduce exactly the
// metamodel-universe fail-open risk this design avoids. So the wildcard is
// preserved and the denial re-checked at match time via
// [compiledCeiling.permitsVerb] / [compiledCeiling.permitsRead].
func filterTypes(roleTypes []string, v verbCeiling) []string {
	if !v.narrows() || len(roleTypes) == 0 {
		return roleTypes
	}
	if v.allow != nil && slices.Contains(roleTypes, "*") {
		// "everything" ∩ "exactly these" = "exactly these".
		return slices.Clone(v.allow)
	}
	out := make([]string, 0, len(roleTypes))
	for _, t := range roleTypes {
		if t == "*" {
			// Wildcard under a pure denial: keep it. The denial is enforced by
			// permitsVerb, which every predicate consults after the role lists
			// have been checked — a plain list cannot express "all except X".
			out = append(out, t)
			continue
		}
		// Match on the TYPE half. A ceiling names entity TYPES, so it
		// clamps at type granularity and leaves the face to
		// [GrantsVerbOnState]; comparing the whole literal would drop a
		// state grant entirely, because "page@draft" never equals "page".
		//
		// That failure had an inversion in it worth remembering: the
		// wildcard branch above returns early, so `update: ["*"]` KEPT the
		// state grant while the narrower `update: ["page"]` destroyed it —
		// a grant surviving the broad ceiling and dying under the specific
		// one. And the resulting denial named the ROLE, not the ceiling,
		// so an operator reading the audit log was sent to inspect an
		// acl.yaml whose grant was plainly present.
		//
		// Still fail-closed: a DENIAL reaches every face regardless,
		// because permitsVerb runs before the role loop against the
		// subject's bare type.
		if v.permits(grantTypeOf(t)) {
			out = append(out, t)
		}
	}
	return out
}

// filterPermissions intersects a role's permission list with the ceiling.
func filterPermissions(rolePerms []string, p permissionCeiling) []string {
	if !p.narrows() || len(rolePerms) == 0 {
		return rolePerms
	}
	if p.allow != nil && slices.Contains(rolePerms, "*") {
		return slices.Clone(p.allow)
	}
	out := make([]string, 0, len(rolePerms))
	for _, name := range rolePerms {
		if name == "*" {
			out = append(out, name)
			continue
		}
		if p.permits(name) {
			out = append(out, name)
		}
	}
	return out
}

// permitsVerb re-checks a resolved verb against the ceiling, closing the
// wildcard gap [filterTypes] leaves open: a role holding `["*"]` under a
// `deny_read: [person]` ceiling keeps its wildcard (a list cannot spell "all
// except person"), so the denial must be applied at match time instead.
//
// Callers apply this AFTER the ordinary role predicates say yes. It can only
// turn a yes into a no.
func (c compiledCeiling) permitsVerb(op Op, target string) bool {
	if !c.active {
		return true
	}
	switch op {
	case OpCreate:
		return c.create.permits(target)
	case OpUpdate, OpRename:
		return c.update.permits(target)
	case OpDelete:
		return c.del.permits(target)
	default:
		return true
	}
}

// permitsRead re-checks a read against the ceiling, for the same wildcard
// reason as [compiledCeiling.permitsVerb].
func (c compiledCeiling) permitsRead(target string) bool {
	if !c.active {
		return true
	}
	return c.read.permits(target)
}

// permitsWorld reports whether the ceiling admits reading the named world.
//
// MANDATORY, not an optional re-check. [filterWorlds] can only narrow the
// list of NAMED worlds a role holds; it structurally cannot express a
// denial of the DEFAULT world, because the default world is spelled as the
// absence of a grant. `deny_worlds: [default]` against a role whose Worlds
// is empty intersects to empty — which still means the default world, so
// the denial would be a silent no-op.
//
// So every world check calls this after the role predicate
// ([roleGrantsWorldRead]) says yes. It can only turn a yes into a no,
// which is the same contract [compiledCeiling.permitsRead] has.
func (c compiledCeiling) permitsWorld(name string) bool {
	if !c.active {
		return true
	}
	if name == "" {
		name = DefaultWorldName
	}
	// The DEFAULT world needs an EXPLICIT denial; an allowlist does not
	// take it away. `worlds: [published]` reads as "this client may also
	// reach the published world", not "published and nothing else,
	// including the default face it could already read" — and an operator
	// who meant the latter has `deny_worlds: [default]` to say so.
	//
	// Silently revoking it would be the failure the world axis exists to
	// prevent, pointed the other way: the default world is the DRAFT face
	// under the design doc's layout, so a client scoped to `published`
	// would find its ordinary reads disappearing for a reason nothing in
	// the config states. A ceiling narrows what it NAMES; the default
	// world is the one world a grant never names.
	// Delegated to worlds.permits when the default world IS explicitly
	// denied, so a scope grant can still re-open it via `except`.
	if name == DefaultWorldName && !slices.Contains(c.worlds.deny, DefaultWorldName) {
		return true
	}
	return c.worlds.permits(name)
}

// permitsPermission re-checks a named permission against the ceiling.
func (c compiledCeiling) permitsPermission(name string) bool {
	if !c.active {
		return true
	}
	return c.permissions.permits(name)
}

// FieldCeiling is the field-visibility half of a client ceiling, for one entity
// type. It is the only part of client attenuation that crosses out of this
// package: the closed-world `visible:` universe is resolved against an entity's
// declared properties, which lives in internal/affordances, so the field axis
// is applied there rather than compiled here.
//
// Exactly one of Visible / Redact is meaningful, mirroring the
// one-spelling-per-type rule enforced at load:
//
//   - Visible non-nil → CLOSED WORLD. Only these fields survive; everything
//     else on the type is hidden, including properties added later. This is
//     where the fail-closed property comes from.
//   - Redact non-empty → open world. Only these fields are hidden.
//   - Both empty → the ceiling does not constrain this type's fields.
type FieldCeiling struct {
	Baseline string
	Visible  []string
	Redact   []string
}

// Constrains reports whether this ceiling hides anything for the type.
func (f FieldCeiling) Constrains() bool {
	return f.Visible != nil || len(f.Redact) > 0
}

// FieldCeilingFor returns the field ceiling this request's principal is subject
// to for entityType. A zero value (Constrains() == false) means unrestricted —
// either no baseline matched, or the baseline says nothing about this type.
//
// Scope grants re-open fields the same way they re-open verbs: a scope naming
// fields under `visible:` ADDS them to the closed world, and one naming fields
// already hidden by `redact:` removes them from the denial.
func (r *Request) FieldCeilingFor(entityType string) FieldCeiling {
	if !r.ceiling.active {
		return FieldCeiling{}
	}
	// Read the baseline resolved at construction rather than re-deriving it.
	// Re-deriving would be a second lookup keyed off the same principal, and
	// the point of computing the ceiling once is that there is nothing to
	// invalidate — two derivations is two things that can drift.
	baseline := r.ceiling.baseline

	out := FieldCeiling{Baseline: r.ceiling.name}
	if v, declared := baseline.Visible[entityType]; declared {
		out.Visible = slices.Clone(v)
	}
	out.Redact = slices.Clone(baseline.Redact[entityType])

	for _, scope := range r.principal.Scopes() {
		g, found := r.d.policy.ScopeGrants[strings.TrimSpace(scope)]
		if !found {
			continue
		}
		reopened := g.Visible[entityType]
		if len(reopened) == 0 {
			continue
		}
		if out.Visible != nil {
			out.Visible = append(out.Visible, reopened...)
		}
		if len(out.Redact) > 0 {
			out.Redact = slices.DeleteFunc(out.Redact, func(f string) bool {
				return slices.Contains(reopened, f)
			})
		}
	}
	return out
}
