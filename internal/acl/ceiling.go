package acl

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Client attenuation (TKT-IAC8TX) restricts a non-interactive client BELOW the
// user it acts as.
//
// The invariant, and the reason this is safe:
//
//	effective = user_grants ∩ (baseline ∪ matched scope_grants)
//
// Two properties fall out, and both are load-bearing:
//
//  1. A ceiling NEVER grants. Intersection with the acting user's own grants
//     means a token cannot exceed its user whatever it claims — a read-only
//     user holding a write-scoped token still cannot write.
//  2. Scopes widen only WITHIN the ceiling. `baseline ∪ scopes` is a union, so
//     more scopes means more capability (matching OAuth everywhere else), but
//     still clamped by (1).
//
// # Why this compiles at load time
//
// A restriction block is static config, so it is fully resolvable when the
// policy loads: `redact: {person: [salary]}` becomes "the allowed field set for
// person, minus salary" against the metamodel. The compiled result is an
// ordinary allowlist, which means the RUNTIME EVALUATOR NEVER LEARNS THE WORD
// "DENY" — decideFromAttrs, readQuery, grantsPermission and FieldVerdicts keep
// seeing the same plain allowlists they always did, and DEC-RG878's additive
// union semantics are untouched.
//
// That is not an implementation detail, it is the design. A runtime denial
// primitive would have to be re-derived in every evaluation path, INCLUDING
// readQuery — which compiles to a store.GraphQuery pushed down into SQL, so a
// deny would have to become a SQL predicate. Do not reintroduce one.
//
// # Selection has no combination rule
//
// [ClientBaseline.AppliesTo] sets must be DISJOINT ([Policy.Validate] rejects
// overlap), so exactly one baseline matches any principal_type. There is
// deliberately no baseline-vs-baseline precedence to specify, document, or get
// wrong. A principal_type matching no baseline is unrestricted.

// ClientBaseline is the ceiling applied to a client of a given principal_type.
// It is keyed by NAME in `acl.yaml` (so it can be talked about in diagnostics
// and `rela acl map --as`), and selects on the verified `principal_type` claim
// via AppliesTo.
type ClientBaseline struct {
	// Description is optional operator-facing prose. Documentation only,
	// mirroring [Policy.Description] and [RoleDef.Description].
	Description string `yaml:"description,omitempty"`

	// AppliesTo lists the verified principal_type values this baseline covers
	// (e.g. [app, pat, service]). Must be disjoint across all baselines —
	// overlap is a load error, not a precedence puzzle. An empty AppliesTo
	// matches nothing and is flagged by `rela acl audit` as dead config.
	AppliesTo []string `yaml:"applies_to"`

	Restriction `yaml:",inline"`
}

// ScopeGrant re-opens capability a baseline closed. It is keyed by the verified
// scope string in `acl.yaml`. Every scope on the token that names a grant
// contributes; the results union. Since a union of re-openings is still bounded
// by the acting user's grants, a scope can never escalate.
//
// Only the ALLOW spellings are meaningful on a scope grant: a scope exists to
// re-open, and a "deny" on a re-opener would be a second, redundant way to
// spell "leave it closed". [Policy.Validate] rejects the deny spellings here so
// the asymmetry is explicit rather than silently ignored.
//
// # A baseline denial is a default, not a floor
//
// Any scope grant naming a capability re-opens it, including one the baseline
// explicitly denied — a scope-granted exception is checked ahead of the denial
// (see [verbCeiling.except]). There is deliberately no way to mark a baseline
// denial un-carve-out-able.
//
// That is safe because the floor is the ACTING USER, not the baseline:
// everything here is still intersected with the user's own grants, so no scope
// can reach past what the user holds. But an operator should read a baseline
// denial as "off unless a scope turns it on", not as a hard minimum. If a
// capability must never reach a client class at all, do not write a scope grant
// that names it.
type ScopeGrant struct {
	Description string `yaml:"description,omitempty"`

	Restriction `yaml:",inline"`
}

// Restriction is the shared body of a [ClientBaseline] and a [ScopeGrant]: what
// an axis permits, in either an allowlist or a denylist spelling.
//
// Per axis, per entity type, an operator picks ONE spelling:
//
//	allowlist (fail-closed)              denylist (low-effort)
//	read:    [ticket, person]            deny_read:    [audit-record]
//	create/update/delete: [ticket]       deny_create/update/delete: ["*"]
//	visible: {person: [name, email]}     redact:       {person: [salary]}
//	permissions: [history:read]          deny_permissions: [history:read]
//
// Declaring both spellings for the same axis (or, for fields, the same TYPE) in
// one block is a load error — a merge rule there would be a coin-flip an
// operator has to memorize.
//
// An OMITTED axis is INHERITED: the block does not narrow it. That is what lets
// a baseline that only hides two fields stay two lines long, instead of
// restating the whole schema.
type Restriction struct {
	Read   []string `yaml:"read,omitempty"`
	Create []string `yaml:"create,omitempty"`
	Update []string `yaml:"update,omitempty"`
	Delete []string `yaml:"delete,omitempty"`

	DenyRead   []string `yaml:"deny_read,omitempty"`
	DenyCreate []string `yaml:"deny_create,omitempty"`
	DenyUpdate []string `yaml:"deny_update,omitempty"`
	DenyDelete []string `yaml:"deny_delete,omitempty"`

	// DenyWrite is shorthand expanding at load to deny_create + deny_update +
	// deny_delete. "read-only client" is the single most common ceiling and
	// should be one line, not three. There is deliberately NO allowlist `write:`
	// counterpart: an allowlist names types, and "the types I may write" is
	// already spelled per-verb.
	DenyWrite []string `yaml:"deny_write,omitempty"`

	// Visible is the closed-world allowlist form: naming a type here asserts a
	// COMPLETE field list for it, so anything unnamed — including a property
	// added to the metamodel later — is redacted. This is where the fail-closed
	// property comes from, and it reuses the existing per-role `visible:`
	// semantics rather than inventing a parallel one.
	Visible map[string][]string `yaml:"visible,omitempty"`

	// Redact is the open-world denylist form: only the named fields are hidden,
	// everything else passes through, including fields added later. Cheaper to
	// write, weaker guarantee — the right pick when a type has two sensitive
	// columns and an open-ended set of harmless ones.
	Redact map[string][]string `yaml:"redact,omitempty"`

	Permissions     []string `yaml:"permissions,omitempty"`
	DenyPermissions []string `yaml:"deny_permissions,omitempty"`
}

// Narrows reports whether the block restricts anything at all. Exported for
// `rela acl audit`, which flags a baseline that narrows nothing: it reads like
// protection, enforces none, and — unlike a wrong grant — produces no symptom
// anyone would report.
func (r Restriction) Narrows() bool {
	return len(r.Read) > 0 || len(r.Create) > 0 || len(r.Update) > 0 ||
		len(r.Delete) > 0 || len(r.DenyRead) > 0 || len(r.DenyCreate) > 0 ||
		len(r.DenyUpdate) > 0 || len(r.DenyDelete) > 0 || len(r.DenyWrite) > 0 ||
		len(r.Visible) > 0 || len(r.Redact) > 0 ||
		len(r.Permissions) > 0 || len(r.DenyPermissions) > 0
}

// denySpellings reports which deny-form axes are set, for the [ScopeGrant]
// rejection. Returns names in a stable order so the error message is
// deterministic.
func (r Restriction) denySpellings() []string {
	var out []string
	for _, c := range []struct {
		name string
		set  bool
	}{
		{"deny_read", len(r.DenyRead) > 0},
		{"deny_create", len(r.DenyCreate) > 0},
		{"deny_update", len(r.DenyUpdate) > 0},
		{"deny_delete", len(r.DenyDelete) > 0},
		{"deny_write", len(r.DenyWrite) > 0},
		{"redact", len(r.Redact) > 0},
		{"deny_permissions", len(r.DenyPermissions) > 0},
	} {
		if c.set {
			out = append(out, c.name)
		}
	}
	return out
}

// validate checks a restriction block. label names the containing block for the
// error message ("client_baselines.apps"). Split into three passes so each
// stays independently readable — they answer different questions.
func (r Restriction) validate(label string) error {
	if err := r.validateSpellings(label); err != nil {
		return err
	}
	return r.validateNoBlanks(label)
}

// validateSpellings enforces one spelling per axis (and, for fields, per type).
func (r Restriction) validateSpellings(label string) error {
	for _, c := range []struct {
		allow, deny       string
		allowSet, denySet bool
	}{
		{"read", "deny_read", len(r.Read) > 0, len(r.DenyRead) > 0},
		{"create", "deny_create", len(r.Create) > 0, len(r.DenyCreate) > 0},
		{"update", "deny_update", len(r.Update) > 0, len(r.DenyUpdate) > 0},
		{"delete", "deny_delete", len(r.Delete) > 0, len(r.DenyDelete) > 0},
		{"permissions", "deny_permissions", len(r.Permissions) > 0, len(r.DenyPermissions) > 0},
	} {
		if c.allowSet && c.denySet {
			return fmt.Errorf(
				"%s: declares both %q and %q — pick one spelling per axis "+
					"(an allowlist states what is permitted; a denylist states what is removed)",
				label, c.allow, c.deny)
		}
	}

	// deny_write is shorthand for the three write verbs, so it collides with
	// each of them in either spelling. Checked separately because the collision
	// is one-to-many.
	if len(r.DenyWrite) > 0 {
		for _, c := range []struct {
			name string
			set  bool
		}{
			{"create", len(r.Create) > 0}, {"deny_create", len(r.DenyCreate) > 0},
			{"update", len(r.Update) > 0}, {"deny_update", len(r.DenyUpdate) > 0},
			{"delete", len(r.Delete) > 0}, {"deny_delete", len(r.DenyDelete) > 0},
		} {
			if c.set {
				return fmt.Errorf(
					"%s: declares both \"deny_write\" and %q — deny_write already "+
						"covers create, update and delete", label, c.name)
			}
		}
	}

	// Fields collide per TYPE, not per axis: `visible` on person and `redact`
	// on ticket is a legitimate mix (each type picks its own posture).
	for t := range r.Visible {
		if _, both := r.Redact[t]; both {
			return fmt.Errorf(
				"%s: declares both \"visible\" and \"redact\" for type %q — "+
					"pick one spelling per type", label, t)
		}
	}
	return nil
}

// validateNoBlanks rejects empty/whitespace entries. A blank type, field or
// permission name can never match anything, so it is silently inert — the
// failure mode an operator is least likely to notice.
func (r Restriction) validateNoBlanks(label string) error {
	for _, c := range []struct {
		field string
		vals  []string
	}{
		{"read", r.Read}, {"create", r.Create}, {"update", r.Update}, {"delete", r.Delete},
		{"deny_read", r.DenyRead}, {"deny_create", r.DenyCreate},
		{"deny_update", r.DenyUpdate}, {"deny_delete", r.DenyDelete},
		{"deny_write", r.DenyWrite},
		{"permissions", r.Permissions}, {"deny_permissions", r.DenyPermissions},
	} {
		for i, v := range c.vals {
			if isBlank(v) {
				return fmt.Errorf("%s.%s[%d]: must not be empty or whitespace", label, c.field, i)
			}
		}
	}
	for _, m := range []struct {
		field string
		vals  map[string][]string
	}{{"visible", r.Visible}, {"redact", r.Redact}} {
		for t, fields := range m.vals {
			if isBlank(t) {
				return fmt.Errorf("%s.%s: entity type key must not be empty or whitespace", label, m.field)
			}
			for i, f := range fields {
				if isBlank(f) {
					return fmt.Errorf("%s.%s.%s[%d]: field must not be empty or whitespace",
						label, m.field, t, i)
				}
			}
		}
	}
	return nil
}

// validateClientAttenuation checks the baseline/scope blocks. Split out of
// [Policy.Validate] to keep that function's cognitive complexity in bounds,
// matching validateUnmatchedPrincipal.
func (p *Policy) validateClientAttenuation() error {
	// A principal_type may be claimed by at most ONE baseline. This is what
	// buys the "no combination rule" property; without it we would need a
	// precedence order, and a more-specific baseline could silently WIDEN a
	// narrower one.
	claimedBy := map[string]string{}
	for _, name := range sortedBaselineNames(p.ClientBaselines) {
		b := p.ClientBaselines[name]
		if isBlank(name) {
			return errors.New("client_baselines: name key must not be empty or whitespace")
		}
		if err := b.validate("client_baselines." + name); err != nil {
			return err
		}
		for i, t := range b.AppliesTo {
			if isBlank(t) {
				return fmt.Errorf(
					"client_baselines.%s.applies_to[%d]: principal type must not be empty or whitespace",
					name, i)
			}
			t = strings.TrimSpace(t)
			if prev, dup := claimedBy[t]; dup {
				return fmt.Errorf(
					"client_baselines: principal type %q is claimed by both %q and %q — "+
						"applies_to sets must be disjoint so exactly one baseline matches",
					t, prev, name)
			}
			claimedBy[t] = name
		}
	}

	for _, name := range sortedScopeNames(p.ScopeGrants) {
		g := p.ScopeGrants[name]
		if isBlank(name) {
			return errors.New("scope_grants: scope key must not be empty or whitespace")
		}
		if err := g.validate("scope_grants." + name); err != nil {
			return err
		}
		// A scope re-opens; a deny on it would mean "re-open nothing", which is
		// what omitting the scope already says. Rejecting rather than ignoring
		// keeps an operator from believing a scope tightens something.
		if deny := g.denySpellings(); len(deny) > 0 {
			return fmt.Errorf(
				"scope_grants.%s: declares %s — a scope grant may only RE-OPEN "+
					"capability a baseline closed, never remove it; express removals "+
					"in the client_baselines block instead",
				name, strings.Join(quoteAll(deny), ", "))
		}
	}

	// Only now that every collision has been rejected is it safe to rewrite the
	// deny_write shorthand — see Restriction.expanded.
	p.expandClientAttenuation()
	return nil
}

// expandClientAttenuation applies Restriction.expanded to every block. Called
// at the tail of validation, never before it.
func (p *Policy) expandClientAttenuation() {
	for name, b := range p.ClientBaselines {
		b.Restriction = b.expanded()
		p.ClientBaselines[name] = b
	}
	for name, g := range p.ScopeGrants {
		g.Restriction = g.expanded()
		p.ScopeGrants[name] = g
	}
}

// normalizeClientAttenuation trims whitespace from every lookup key and value
// so a padded `acl.yaml` entry cannot load clean yet stay permanently
// unmatchable (the RR-IK355A trap on asserted_role_assignments). Runs before
// validation, so the blank checks see the normalized form.
func (p *Policy) normalizeClientAttenuation() {
	if len(p.ClientBaselines) > 0 {
		out := make(map[string]ClientBaseline, len(p.ClientBaselines))
		for name, b := range p.ClientBaselines {
			b.AppliesTo = trimAll(b.AppliesTo)
			b.Restriction = b.normalized()
			out[strings.TrimSpace(name)] = b
		}
		p.ClientBaselines = out
	}
	if len(p.ScopeGrants) > 0 {
		out := make(map[string]ScopeGrant, len(p.ScopeGrants))
		for name, g := range p.ScopeGrants {
			g.Restriction = g.normalized()
			out[strings.TrimSpace(name)] = g
		}
		p.ScopeGrants = out
	}
}

// normalized returns r with every type name, field name and permission trimmed.
//
// It deliberately does NOT expand deny_write — that is Restriction.expanded,
// and the ordering matters: expanding before validation would rewrite
// deny_write into the three per-verb lists, and the "deny_write alongside
// deny_update" collision check would then find deny_write already gone and pass
// a policy whose deny_update was silently discarded.
func (r Restriction) normalized() Restriction {
	r.Read, r.Create, r.Update, r.Delete =
		trimAll(r.Read), trimAll(r.Create), trimAll(r.Update), trimAll(r.Delete)
	r.DenyRead, r.DenyCreate, r.DenyUpdate, r.DenyDelete =
		trimAll(r.DenyRead), trimAll(r.DenyCreate), trimAll(r.DenyUpdate), trimAll(r.DenyDelete)
	r.DenyWrite = trimAll(r.DenyWrite)
	r.Permissions, r.DenyPermissions = trimAll(r.Permissions), trimAll(r.DenyPermissions)
	r.Visible, r.Redact = trimFieldMap(r.Visible), trimFieldMap(r.Redact)
	return r
}

// expanded rewrites the deny_write shorthand into the three write verbs, so
// every downstream consumer sees one canonical shape instead of re-deriving the
// shorthand. Runs AFTER validation has confirmed deny_write does not collide
// with a per-verb spelling, so these assignments cannot clobber operator input.
func (r Restriction) expanded() Restriction {
	if len(r.DenyWrite) == 0 {
		return r
	}
	w := r.DenyWrite
	// Each verb gets its OWN backing array: a later in-place edit of one
	// (a compiler step filtering wildcards, say) must not silently mutate the
	// other two.
	r.DenyCreate = slices.Clone(w)
	r.DenyUpdate = slices.Clone(w)
	r.DenyDelete = slices.Clone(w)
	r.DenyWrite = nil
	return r
}

// trimAll trims every element, PRESERVING the nil-vs-empty distinction: an
// absent key decodes to nil ("axis omitted → inherit") while `read: []` decodes
// to a non-nil empty slice ("permit nothing"). Collapsing the latter to nil
// would silently turn a hard lockout into no restriction at all — the fail-open
// direction.
func trimAll(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, strings.TrimSpace(v))
	}
	return out
}

func trimFieldMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for t, fields := range in {
		out[strings.TrimSpace(t)] = trimAll(fields)
	}
	return out
}

func quoteAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return out
}

// sortedBaselineNames / sortedScopeNames give deterministic iteration so a
// policy with two conflicting entries always reports the SAME pair, rather than
// whichever the map happened to yield first.
func sortedBaselineNames(m map[string]ClientBaseline) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedScopeNames(m map[string]ScopeGrant) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
