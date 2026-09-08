package aclaudit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// Client-attenuation audit rules (TKT-IAC8TX).
//
// These exist because of one specific failure mode: a ceiling that reads like
// protection and enforces nothing. Unlike a wrong grant — which denies
// something and gets noticed — a baseline that never matches, or a scope that
// re-opens something no baseline closed, is completely silent. Nobody files a
// bug about access they still have.
//
// The severities reflect that. An inert baseline is Medium, not Low: an
// operator who wrote it believes a client is restricted when it is not.

// checkCeilings runs the pure-policy ceiling checks (tier A).
func checkCeilings(p *acl.Policy) []Finding {
	f := make([]Finding, 0, 4)
	f = append(f, checkInertBaseline(p)...)         // A11
	f = append(f, checkUnreachableScopeGrant(p)...) // A12
	f = append(f, checkEmptyAppliesTo(p)...)        // A13
	return f
}

// A11 — a baseline that narrows nothing. The operator wrote a restriction block
// and got no restriction; every client of that principal_type keeps its acting
// user's full grants.
func checkInertBaseline(p *acl.Policy) []Finding {
	var f []Finding
	for _, name := range sortedBaselineNames(p) {
		b := p.ClientBaselines[name]
		if b.Narrows() {
			continue
		}
		f = append(f, Finding{
			Rule: "A11-inert-client-baseline", Severity: Medium, Subject: name,
			Detail: fmt.Sprintf("client baseline %q declares no restriction, so clients of "+
				"principal type(s) %s keep their acting user's full access",
				name, quoteJoin(b.AppliesTo)),
			Fix: "add a restriction (deny_write, redact, visible, deny_read, deny_permissions), " +
				"or remove the baseline if the client is meant to be unrestricted",
		})
	}
	return f
}

// A13 — a baseline with no applies_to matches no principal_type at all. Same
// class as A11 but a different mistake: the restriction is real, the selector
// is missing.
func checkEmptyAppliesTo(p *acl.Policy) []Finding {
	var f []Finding
	for _, name := range sortedBaselineNames(p) {
		if len(p.ClientBaselines[name].AppliesTo) > 0 {
			continue
		}
		f = append(f, Finding{
			Rule: "A13-baseline-matches-nothing", Severity: Medium, Subject: name,
			Detail: fmt.Sprintf("client baseline %q has an empty applies_to, so it matches no "+
				"principal type and never applies", name),
			Fix: "list the principal_type values this baseline covers " +
				"(e.g. applies_to: [app, pat, service])",
		})
	}
	return f
}

// A12 — a scope grant that re-opens something no baseline ever closes.
//
// This is the subtle one, and it is worth a finding precisely because the
// symptom is invisible: the scope appears to work (the capability IS present),
// so an operator concludes the mechanism is wired up correctly. Then they write
// a second scope expecting the same and find it inert, because THAT one really
// did depend on a baseline closing the capability first.
//
// Only reported when at least one baseline exists — a policy with scope_grants
// and no baselines at all is a different (and more obvious) mistake, already
// covered by the scope being wholly decorative.
func checkUnreachableScopeGrant(p *acl.Policy) []Finding {
	if len(p.ClientBaselines) == 0 || len(p.ScopeGrants) == 0 {
		return nil
	}
	var f []Finding
	for _, scope := range sortedScopeGrantNames(p) {
		g := p.ScopeGrants[scope]
		unreachable := unreachableTargets(p, g)
		if len(unreachable) == 0 {
			continue
		}
		f = append(f, Finding{
			Rule: "A12-scope-reopens-nothing", Severity: Low, Subject: scope,
			Detail: fmt.Sprintf("scope grant %q re-opens %s, which no client baseline closes; "+
				"presenting this scope changes nothing",
				scope, strings.Join(unreachable, ", ")),
			Fix: "close the capability in a client_baselines block first, or drop the " +
				"unreachable entries from the scope grant",
		})
	}
	return f
}

// unreachableTargets lists the (axis, type) pairs a scope grant re-opens that
// no baseline restricts. Returns human-readable labels, deduped and sorted.
func unreachableTargets(p *acl.Policy, g acl.ScopeGrant) []string {
	seen := map[string]bool{}
	var out []string
	note := func(axis, target string) {
		label := axis + " " + target
		if !seen[label] {
			seen[label] = true
			out = append(out, label)
		}
	}

	for _, c := range []struct {
		axis    string
		targets []string
		closed  func(acl.ClientBaseline, string) bool
	}{
		{"read", g.Read, func(b acl.ClientBaseline, t string) bool {
			return restrictsVerb(b.Read, b.DenyRead, t)
		}},
		{"create", g.Create, func(b acl.ClientBaseline, t string) bool {
			return restrictsVerb(b.Create, b.DenyCreate, t)
		}},
		{"update", g.Update, func(b acl.ClientBaseline, t string) bool {
			return restrictsVerb(b.Update, b.DenyUpdate, t)
		}},
		{"delete", g.Delete, func(b acl.ClientBaseline, t string) bool {
			return restrictsVerb(b.Delete, b.DenyDelete, t)
		}},
	} {
		for _, target := range c.targets {
			if !anyBaseline(p, func(b acl.ClientBaseline) bool { return c.closed(b, target) }) {
				note(c.axis, target)
			}
		}
	}

	for _, perm := range g.Permissions {
		closed := anyBaseline(p, func(b acl.ClientBaseline) bool {
			return restrictsVerb(b.Permissions, b.DenyPermissions, perm)
		})
		if !closed {
			note("permission", perm)
		}
	}

	for entityType, fields := range g.Visible {
		for _, field := range fields {
			closed := anyBaseline(p, func(b acl.ClientBaseline) bool {
				return restrictsField(b, entityType, field)
			})
			if !closed {
				note("field", entityType+"."+field)
			}
		}
	}

	sort.Strings(out)
	return out
}

// restrictsVerb reports whether an allow/deny pair constrains target — i.e.
// whether re-opening target could make any difference.
func restrictsVerb(allow, deny []string, target string) bool {
	if allow != nil && !containsOrWildcard(allow, target) {
		return true // closed allowlist that omits target
	}
	return containsOrWildcard(deny, target)
}

// restrictsField reports whether a baseline hides entityType.field.
func restrictsField(b acl.ClientBaseline, entityType, field string) bool {
	if v, declared := b.Visible[entityType]; declared {
		// Closed world: a field the block omits is hidden.
		return !containsOrWildcard(v, field)
	}
	return containsOrWildcard(b.Redact[entityType], field)
}

func containsOrWildcard(list []string, target string) bool {
	for _, v := range list {
		if v == "*" || v == target {
			return true
		}
	}
	return false
}

func anyBaseline(p *acl.Policy, pred func(acl.ClientBaseline) bool) bool {
	for _, b := range p.ClientBaselines {
		if pred(b) {
			return true
		}
	}
	return false
}

// B8 — a ceiling names an entity type or field the metamodel doesn't declare,
// so it restricts nothing. The same drift class B1/B4 catch for roles: a typo'd
// type in a DENY position is worse than in a grant, because it silently fails
// to protect rather than silently failing to permit.
func checkCeilingsAgainstMetamodel(p *acl.Policy, m MetamodelReader) []Finding {
	f := make([]Finding, 0, 4)
	for _, name := range sortedBaselineNames(p) {
		b := p.ClientBaselines[name]
		f = append(f, ceilingTypeFindings(
			"client baseline", name, b.Restriction, m)...)
		f = append(f, ceilingWorldFindings(
			"client baseline", name, b.Restriction, m)...)
	}
	for _, scope := range sortedScopeGrantNames(p) {
		g := p.ScopeGrants[scope]
		f = append(f, ceilingTypeFindings(
			"scope grant", scope, g.Restriction, m)...)
		f = append(f, ceilingWorldFindings(
			"scope grant", scope, g.Restriction, m)...)
	}
	return f
}

// ceilingWorldFindings reports a client-attenuation block naming a world the
// metamodel does not declare (B10, sharing the rule id with the role-grant
// check — same mistake, same fix, and an operator should not have to learn
// two rule names for "that world does not exist").
//
// A ceiling entry naming a nonexistent world is inert in the direction that
// matters: an allowlist naming only unknown worlds permits nothing, and a
// denylist denies nothing. Neither leaks, but both read as protection.
func ceilingWorldFindings(
	kind, name string, r acl.Restriction, m MetamodelReader,
) []Finding {
	var f []Finding
	seen := map[string]bool{}
	for _, c := range []struct {
		axis   string
		worlds []string
	}{{"worlds", r.Worlds}, {"deny_worlds", r.DenyWorlds}} {
		for _, world := range c.worlds {
			if world == acl.DefaultWorldName || m.HasWorld(world) || seen[world] {
				continue
			}
			seen[world] = true
			if cv, isCaseVariant := defaultWorldCaseVariant(name, world); isCaseVariant {
				f = append(f, cv)
				continue
			}
			f = append(f, Finding{
				Rule: "B10-undeclared-world", Severity: High, Subject: name,
				Detail: fmt.Sprintf("%s %q names world %q (under %s) which the metamodel does "+
					"not declare; the restriction matches nothing", kind, name, world, c.axis),
				Fix: fmt.Sprintf("declare world %q under `worlds:` in schema.yaml, or fix the "+
					"%s entry (check for a typo)", world, c.axis),
			})
		}
	}
	return f
}

// ceilingTypeFindings reports undeclared entity types and fields in one block.
func ceilingTypeFindings(
	kind, name string, r acl.Restriction, m MetamodelReader,
) []Finding {
	var f []Finding
	seen := map[string]bool{}

	for _, c := range []struct {
		axis  string
		types []string
	}{
		{"read", r.Read}, {"create", r.Create}, {"update", r.Update}, {"delete", r.Delete},
		{"deny_read", r.DenyRead}, {"deny_create", r.DenyCreate},
		{"deny_update", r.DenyUpdate}, {"deny_delete", r.DenyDelete},
		// worlds/deny_worlds are deliberately ABSENT: their entries are
		// world names, not entity types, so HasEntityType is the wrong
		// question and would report every one of them as undeclared.
		// Checked separately by ceilingWorldFindings (B10).
	} {
		for _, entry := range c.types {
			// TYPE half only — a ceiling axis may name a state-shaped
			// grant for the same reason a role may (TKT-DN37J2).
			t := grantEntityType(entry)
			if t == "*" || m.HasEntityType(t) || seen[t] {
				continue
			}
			seen[t] = true
			f = append(f, Finding{
				Rule: "B8-ceiling-undeclared-type", Severity: High, Subject: name,
				Detail: fmt.Sprintf("%s %q names entity type %q (under %s) which the metamodel "+
					"does not declare; the restriction matches nothing",
					kind, name, t, c.axis),
				Fix: fmt.Sprintf("declare entity type %q, or fix the %s entry (check for a typo)",
					t, c.axis),
			})
		}
	}

	for _, m2 := range []struct {
		axis   string
		fields map[string][]string
	}{{"visible", r.Visible}, {"redact", r.Redact}} {
		for _, entityType := range sortedFieldMapKeys(m2.fields) {
			if !m.HasEntityType(entityType) {
				if !seen[entityType] {
					seen[entityType] = true
					f = append(f, Finding{
						Rule: "B8-ceiling-undeclared-type", Severity: High, Subject: name,
						Detail: fmt.Sprintf("%s %q names entity type %q (under %s) which the "+
							"metamodel does not declare; the restriction matches nothing",
							kind, name, entityType, m2.axis),
						Fix: fmt.Sprintf("declare entity type %q, or fix the %s key",
							entityType, m2.axis),
					})
				}
				continue
			}
			for _, field := range m2.fields[entityType] {
				if field == "*" || m.HasField(entityType, field) {
					continue
				}
				f = append(f, Finding{
					Rule: "B9-ceiling-undeclared-field", Severity: Medium, Subject: name,
					Detail: fmt.Sprintf("%s %q names field %q on %q (under %s) which the metamodel "+
						"does not declare", kind, name, field, entityType, m2.axis),
					Fix: fmt.Sprintf("declare property %q on %q, or fix the %s entry",
						field, entityType, m2.axis),
				})
			}
		}
	}
	return f
}

func sortedBaselineNames(p *acl.Policy) []string {
	out := make([]string, 0, len(p.ClientBaselines))
	for k := range p.ClientBaselines {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedScopeGrantNames(p *acl.Policy) []string {
	out := make([]string, 0, len(p.ScopeGrants))
	for k := range p.ScopeGrants {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedFieldMapKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func quoteJoin(in []string) string {
	if len(in) == 0 {
		return "(none)"
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return strings.Join(out, ", ")
}
