package appbuild

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
)

// warnQueryScopes logs the two load-time diagnostics a query scope can earn.
//
// Both are WARNINGS rather than refusals, for opposite reasons. The identity
// one describes a configuration that is correct but surprising, and whose
// failure mode (every page of that type erroring on an anonymous deployment)
// is loud enough already — the warning only supplies the connection to the
// declaration that caused it. The `visible:` one describes a configuration
// that is merely inconsistent; see [QueryScopeVisibilityConflicts].
//
// Nil: a nil policy skips the visibility report and keeps the identity one.
func warnQueryScopes(compiled *scopes.Compiled, meta *metamodel.Metamodel, policy *acl.Policy) {
	for _, entityType := range compiled.RequiresIdentity() {
		slog.Warn("appbuild: default query scope reads current_user, so every "+
			"presentation surface for this type needs a resolved principal; "+
			"on a deployment without identity these pages will error rather than render",
			"entity_type", entityType,
			"scope", metamodel.DefaultQueryScopeName)
	}
	for _, msg := range QueryScopeVisibilityConflicts(compiled, meta, policy) {
		slog.Warn("appbuild: query scope reads a field-restricted property", "detail", msg)
	}
}

// QueryScopeVisibilityConflicts reports query scopes that read a property some
// role can hide by field-level ACL (`visible:`).
//
// # Why this warns rather than refuses
//
// A scope decides MEMBERSHIP, and membership is visible to the reader as the
// presence or absence of a row. Read alone, that looks like a field-ACL
// bypass: `entity.salary == '100000'` would disclose, one bit at a time, a
// value `visible:` exists to withhold.
//
// It is not one, because a scope is never the only thing excluding a row. The
// ACL read gate runs independently and FIRST, so every row a scope can act on
// is one the principal was already entitled to read. A scope narrows that set
// further for presentation reasons. The bit an operator leaks this way is a
// bit about a row the reader may already fetch — so the overlap is a
// configuration smell, not a confidentiality boundary being crossed.
//
// What makes it a smell worth naming is that the two declarations disagree
// about the same property: one says "this value is not for that role", the
// other builds a screen out of it. Whichever the operator meant, they almost
// certainly did not mean both.
//
// The warning is also the only way to notice. Nothing fails, no page errors;
// a role simply sees a row list shaped by a field it cannot read.
//
// # Pessimistic on purpose
//
// The check asks whether ANY role could hide the property, not whether some
// particular principal's roles do — it runs at load, where there is no
// principal. A role nobody currently holds still counts: an unassigned role
// is a misconfiguration to fix, not a reason to stay quiet until someone is
// assigned to it.
//
// Returns one message per (scope, property) conflict, sorted.
//
// Nil: a nil metamodel or policy yields no conflicts (nothing to compare).
func QueryScopeVisibilityConflicts(
	compiled *scopes.Compiled, meta *metamodel.Metamodel, policy *acl.Policy,
) []string {
	if meta == nil || policy == nil {
		return nil
	}
	restricted := restrictedFields(policy, meta)
	if len(restricted) == 0 {
		return nil
	}

	var out []string
	compiled.Each(func(key scopes.Key, prog *predicate.Program) {
		hidden := restricted[key.EntityType]
		if len(hidden) == 0 {
			return
		}
		for _, attr := range prog.Attributes(predicatefns.VarEntity) {
			roles, ok := hidden[attr]
			if !ok {
				continue
			}
			out = append(out, fmt.Sprintf(
				"entity %q: query scope %q reads %q, which role %s can hide with `visible:` — "+
					"the scope shapes a row list out of a value that role may not read; "+
					"scope on a property no role restricts, or widen the role's visible: list",
				key.EntityType, key.Name, attr, quotedRoleList(roles)))
		}
	})
	sort.Strings(out)
	return out
}

// restrictedFields maps entityType → property → the roles that can hide it.
//
// `visible:` is a CLOSED WORLD: naming a type asserts a complete field list,
// so the restricted set is the COMPLEMENT of what a role grants, not a
// denylist. A role declaring `visible: [name]` on `person` restricts every
// other declared property — including ones added to the metamodel later,
// which is where the fail-closed property comes from. Computing this as
// "fields explicitly denied" would make the check look like it works while
// catching nothing.
func restrictedFields(policy *acl.Policy, meta *metamodel.Metamodel) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for roleName, role := range policy.Roles {
		for entityType, grants := range role.Visible {
			def, ok := meta.GetEntityDef(entityType)
			if !ok {
				continue
			}
			granted := make(map[string]bool, len(grants))
			for _, g := range grants {
				// A conditional grant (`when:`) is NOT treated as granting:
				// it hides the field for SOME rows, which is precisely the
				// row-shaped disagreement this check exists to name.
				if g.When == "" {
					granted[g.Field] = true
				}
			}
			byField := out[entityType]
			if byField == nil {
				byField = map[string][]string{}
				out[entityType] = byField
			}
			for property := range def.Properties {
				if !granted[property] {
					byField[property] = append(byField[property], roleName)
				}
			}
		}
	}
	for _, byField := range out {
		for property := range byField {
			slices.Sort(byField[property])
		}
	}
	return out
}

// quotedRoleList renders role names for a diagnostic, naming every one rather
// than the first: an operator widening the wrong role's `visible:` list would
// see no change and conclude the check is broken.
func quotedRoleList(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	if len(out) == 1 {
		return out[0]
	}
	return "s " + joinWithAnd(out)
}

func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return parts[0] + ", " + joinWithAnd(parts[1:])
}
