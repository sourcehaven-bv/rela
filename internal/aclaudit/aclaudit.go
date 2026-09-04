// Package aclaudit is an on-demand linter for an [acl.Policy] loaded from
// `acl.yaml`. It reports authorization-misconfiguration findings —
// escalation foot-guns, dead/inert config, and policy-vs-metamodel drift —
// ranked by severity. It is deliberately SEPARATE from [acl.Policy.Validate]:
//
//   - Validate is a boot gate: it hard-errors on a few security-critical
//     structural invariants and runs on every policy load. It sees only the
//     policy.
//   - aclaudit is an on-demand linter: operators (or CI) run `rela acl audit`.
//     It is advisory (never blocks boot), ranks findings by severity, and
//     cross-checks the metamodel — none of which fits a boot gate.
//
// The package lives outside internal/acl because the metamodel cross-checks
// need the schema, and arch-lint forbids acl→metamodel. aclaudit defines a
// narrow consumer-side [MetamodelReader] interface; the concrete adapter over
// `*metamodel.Metamodel` is supplied by the caller (the CLI), so aclaudit
// itself depends only on internal/acl.
//
// See TKT-TS0J5K and the research RES-VWNN2T for the tier model and the full
// finding taxonomy. v1 implements Tier A (pure-policy) and Tier B (metamodel).
// Tier C (graph-aware) and Tier D (reachability) are future work.
package aclaudit

import (
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// Severity ranks a [Finding]. Critical/High are the gating tier for
// `rela acl audit --exit-code`; Medium/Low/Nit are advisory.
//
// This is a richer model than the codebase's usual "error"/"warning" string
// convention (see internal/validation) — deliberately, because the audit's
// value is ranking a self-promotion path above a dead-permission nit, and the
// CI exit-code gate needs that distinction.
type Severity int

const (
	// Critical: anyone (incl. anonymous) gets a privileged capability.
	Critical Severity = iota
	// High: a self-promotion path, or a grant that silently mis-fires in a
	// security-relevant way.
	High
	// Medium: dead/inert config or drift that weakens intent.
	Medium
	// Low: cosmetic or likely-intentional config worth a second look.
	Low
	// Nit: stylistic.
	Nit
)

// String renders the severity as a lowercase label.
func (s Severity) String() string {
	switch s {
	case Critical:
		return "critical"
	case High:
		return "high"
	case Medium:
		return "medium"
	case Low:
		return "low"
	case Nit:
		return "nit"
	default:
		return "unknown"
	}
}

// ParseSeverity maps a lowercase label to a Severity. The special label
// "any" maps to [Nit] — the least-severe level — so a threshold of "any"
// matches every finding (used by `--fail-on=any`). Returns false for an
// unknown label.
func ParseSeverity(s string) (Severity, bool) {
	switch s {
	case "critical":
		return Critical, true
	case "high":
		return High, true
	case "medium":
		return Medium, true
	case "low":
		return Low, true
	case "nit", "any":
		return Nit, true
	default:
		return 0, false
	}
}

// Finding is one authorization-misconfiguration the audit reports. Each
// analyzer-style finding owns these fields (the codebase has no shared base
// finding type; see internal/analysis / internal/validation).
type Finding struct {
	// Rule is a stable identifier (e.g. "A1-ungated-membership") so findings
	// are greppable and a future suppress-list can reference them.
	Rule string `json:"rule"`
	// Severity ranks the finding.
	Severity Severity `json:"severity"`
	// Subject names the policy/schema element the finding is about (a role,
	// relation, type, or assignment key).
	Subject string `json:"subject"`
	// Detail is the human explanation of what's wrong.
	Detail string `json:"detail"`
	// Fix is a one-line remediation hint.
	Fix string `json:"fix"`
}

// RelationView is the narrow metamodel view of a relation type the audit
// needs: the entity types its edges may originate from.
type RelationView struct {
	From []string
}

// MetamodelReader is the narrow consumer-side view of the metamodel the Tier-B
// checks need. The concrete implementation (an adapter over
// *metamodel.Metamodel) is supplied by the caller; defining the interface here
// keeps aclaudit free of a metamodel import and bounded to the 3 lookups it
// actually uses (rather than the whole, plimsoll-fat Metamodel surface).
//
// A nil MetamodelReader is valid: [Audit] then runs only the Tier-A
// pure-policy checks and skips Tier B.
type MetamodelReader interface {
	// HasEntityType reports whether t is a declared entity type.
	HasEntityType(t string) bool
	// GetRelation returns the relation type's view, or false if undeclared.
	GetRelation(name string) (RelationView, bool)
	// HasField reports whether entity type t declares a property named field.
	HasField(t, field string) bool
	// EnumOptions returns the allowed values for an enum field on type t, and
	// false if the field is absent or not an enum.
	EnumOptions(t, field string) ([]string, bool)
	// HasWorld reports whether name is a DECLARED world. The implicit
	// default world is not declared and reports false; callers that accept
	// it must say so themselves (see checkUndeclaredWorlds).
	HasWorld(name string) bool
	// HasFace reports whether entity type t declares the content state
	// named face.
	HasFace(t, face string) bool
	// BareFace returns the declared name of the face stored under the bare
	// id (`bare_face:`), or "" when the type declares none.
	BareFace(t string) string
}

// PermissionConsumer reports permissions referenced OUTSIDE acl.yaml. The
// policy knows every place a permission is *granted* (a role's `permissions:`
// list) but only one place it is *consumed*
// (`role_relations.requires_permission`), so a permission gating a data-entry
// UI surface — a document, a dashboard card, a navigation entry, a command —
// is invisible to the audit. Reporting those as dead config, with a hint to
// remove them, is exactly the false positive this interface exists to prevent.
//
// Like [MetamodelReader], the concrete implementation is supplied by the
// caller, so aclaudit stays free of a dataentryconfig import and bounded to
// the one lookup it needs.
//
// Unlike [MetamodelReader], a nil PermissionConsumer is NOT simply "run fewer
// checks": see [Audit] for why it must suppress the dead-permission finding
// rather than let it run blind.
type PermissionConsumer interface {
	// UsedPermissions returns every permission name referenced by a
	// non-policy surface. Order and duplicates don't matter.
	UsedPermissions() []string
}

// Audit runs every v1 check against the policy and returns the findings
// sorted deterministically (severity, then rule, then subject). A clean,
// well-gated policy returns an empty slice.
//
// meta may be nil — Tier-B (metamodel cross-check) findings are then skipped
// and only Tier-A pure-policy findings are returned.
//
// perms may be nil, but nil is NOT equivalent to "no permissions are used
// elsewhere". A nil consumer means the caller could not tell us what the UI
// gates reference, so A7 (dead permission) is SKIPPED entirely rather than
// asserting config is dead on incomplete information. This deliberately
// differs from meta's nil handling, where a missing metamodel can only make a
// check unformable, never wrong.
// PRECONDITION: p must have passed [acl.Policy.Validate] (which
// [acl.LoadPolicy] runs). World grants are read from RoleDef.Worlds, which
// only the validating load populates by splitting `read: [world:X]` out of
// Read — so on an unvalidated policy B10 would examine nothing and B1 would
// instead report the world token as an undeclared ENTITY TYPE, which is
// misleading advice at High severity. checkUndeclaredWorlds re-scans Read
// for a residual world prefix so that mistake is diagnosed correctly rather
// than merely undetected, but callers should validate first regardless.
func Audit(p *acl.Policy, meta MetamodelReader, perms PermissionConsumer) []Finding {
	if p == nil {
		return nil
	}
	var f []Finding
	f = append(f, tierA(p, perms)...)
	if meta != nil {
		f = append(f, tierB(p, meta)...)
	}
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].Severity != f[j].Severity {
			return f[i].Severity < f[j].Severity
		}
		if f[i].Rule != f[j].Rule {
			return f[i].Rule < f[j].Rule
		}
		return f[i].Subject < f[j].Subject
	})
	return f
}

// HasAtLeast reports whether any finding is at or above the given severity
// (lower Severity value == more severe). Used by `--exit-code` to gate on
// Critical/High.
func HasAtLeast(findings []Finding, threshold Severity) bool {
	for _, f := range findings {
		if f.Severity <= threshold {
			return true
		}
	}
	return false
}

// ---- Shared policy predicates ------------------------------------------

// isPrivileged reports whether a role confers escalation-relevant power.
// Delegates to [acl.RoleDef.IsPrivileged] so A2/A3 and the shared
// membership predicate ([acl.Policy.MembershipSelfPromotionOpen], which A1
// calls) agree on what "privileged" means — one definition, not two
// (TKT-T31NKT). Kept as a local function so the call sites read unchanged.
func isPrivileged(r acl.RoleDef) bool {
	return r.IsPrivileged()
}

// roleDeclared reports whether the policy declares a role by this name.
func roleDeclared(p *acl.Policy, role string) bool {
	_, ok := p.Roles[role]
	return ok
}

// permissionGranted reports whether any declared role grants perm.
func permissionGranted(p *acl.Policy, perm string) bool {
	for _, r := range p.Roles {
		if slices.Contains(r.Permissions, perm) {
			return true
		}
	}
	return false
}

// grantEntityType returns the entity type an ACL grant entry addresses:
// the whole entry for a plain type grant, the part before "@" for a
// state-shaped write grant (`page@draft`, TKT-DN37J2).
//
// Mirrors internal/acl's unexported grantTypeOf. Duplicated rather than
// exported from there because it is one strings.Cut and the audit already
// keeps its own narrow view of the policy — but the two must agree, which
// is what TestGrantEntityType_MatchesACLSplit pins.
func grantEntityType(entry string) string {
	typeName, _, _ := splitStateGrant(entry)
	return typeName
}

// splitStateGrant splits a `type@face` write grant into its parts,
// reporting isState=false for a plain type grant.
//
// Mirrors internal/acl's parseStateGrant, minus the grammar validation —
// acl rejects a malformed face at policy load, so by the time the audit
// runs the only question left is whether the metamodel declares it.
func splitStateGrant(entry string) (typeName, face string, isState bool) {
	typeName, face, isState = strings.Cut(entry, "@")
	if !isState {
		return entry, "", false
	}
	return typeName, face, true
}

// verbLists returns the four grant lists of a role for iteration.
func verbLists(r acl.RoleDef) map[string][]string {
	return map[string][]string{
		"create": r.Create,
		"update": r.Update,
		"delete": r.Delete,
		"read":   r.Read,
	}
}

// hasWildcardWrite reports whether a role grants "*" on any write verb.
func hasWildcardWrite(r acl.RoleDef) bool {
	for _, list := range [][]string{r.Create, r.Update, r.Delete} {
		if slices.Contains(list, "*") {
			return true
		}
	}
	return false
}

// affordanceTypeKeys returns the entity-type keys across all affordance maps
// of a role (fields/visible/options/relations). These keys are entity types
// with no wildcard, so they can be checked against the schema directly
// (RR-TZ2S3G).
func affordanceTypeKeys(r acl.RoleDef) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m map[string][]acl.FieldGrant) {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	add(r.Fields)
	add(r.Visible)
	for k := range r.Options {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range r.Relations {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// sortedRoleNames returns role names in deterministic order.
func sortedRoleNames(p *acl.Policy) []string {
	names := make([]string, 0, len(p.Roles))
	for n := range p.Roles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// hasLeadingTrailingSpace reports whether s differs from its trimmed form.
func hasLeadingTrailingSpace(s string) bool {
	return s != "" && strings.TrimSpace(s) != s
}
