package metamodel

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ScopeChange is the `on_change:` of a scoped relation (TKT-5GK80Q9): what a
// write that changes the source's scope does to an edge it leaves out of
// scope.
type ScopeChange string

const (
	// ScopeChangeClear deletes the stale edge in the same write.
	ScopeChangeClear ScopeChange = "clear"
	// ScopeChangeReject refuses the write.
	ScopeChangeReject ScopeChange = "reject"
)

// IsValid reports whether c is a declared on_change value.
func (c ScopeChange) IsValid() bool {
	return c == ScopeChangeClear || c == ScopeChangeReject
}

// ScopeOwner returns the owning relation through which an entity of
// targetType, at the far end of the scoped relation relType, has the owner
// that is compared with the source's scope. The loader guarantees exactly
// one for every `to` type of a scoped relation, so ok is false only when
// relType is not scoped or targetType is not one of its `to` types.
//
// A package function rather than a method: Metamodel's exported API is capped
// (plimsoll).
//
// Nil: accepted for m; a nil metamodel declares no scoped relation.
func ScopeOwner(m *Metamodel, relType, targetType string) (string, bool) {
	if m == nil {
		return "", false
	}
	def, ok := m.Relations[relType]
	if !ok || def.Within == "" || !slices.Contains(def.To, targetType) {
		return "", false
	}
	owners := scopeOwners(m, def, targetType)
	if len(owners) != 1 {
		return "", false
	}
	return owners[0], true
}

// scopeOwners lists, sorted, the owning relations that can own an entity of
// targetType from an entity the scope relation of def points to.
func scopeOwners(m *Metamodel, def RelationDef, targetType string) []string {
	scope, ok := m.Relations[def.Within]
	if !ok {
		return nil
	}
	var out []string
	for _, name := range slices.Sorted(maps.Keys(m.Relations)) {
		o := m.Relations[name]
		if !o.Owning || !slices.Contains(o.To, targetType) {
			continue
		}
		if slices.ContainsFunc(o.From, func(t string) bool { return slices.Contains(scope.To, t) }) {
			out = append(out, name)
		}
	}
	return out
}

// validateRelationWithin checks the `within:` and `on_change:` declarations
// (TKT-5GK80Q9). Scoping is one level deep: a scope relation is not scoped
// itself. That is what lets the write path clear stale edges in one pass:
// clearing a scoped edge can never make another scoped edge stale.
func validateRelationWithin(m *Metamodel) []string {
	var errs []string
	reported := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(m.Relations)) {
		rel := m.Relations[name]
		if rel.Within == "" {
			if rel.OnChange != "" {
				errs = append(errs, fmt.Sprintf("relation %q: on_change is set without within", name))
			}
			continue
		}
		if cycle := withinCycle(m, name); cycle != nil {
			if !reported[name] {
				for _, n := range cycle {
					reported[n] = true
				}
				errs = append(errs, fmt.Sprintf("relation %q: within forms a cycle: %s",
					name, strings.Join(append(cycle, cycle[0]), " -> ")))
			}
			continue
		}
		errs = append(errs, checkWithin(m, name, rel)...)
	}
	return errs
}

// withinCycle returns the relations of the within chain that starts at name,
// in order, when the chain comes back to name. Nil otherwise.
func withinCycle(m *Metamodel, name string) []string {
	chain := []string{name}
	for cur := name; ; {
		next := m.Relations[cur].Within
		switch {
		case next == "":
			return nil
		case next == name:
			return chain
		}
		if _, ok := m.Relations[next]; !ok || slices.Contains(chain, next) {
			// A dangling name, or a cycle that does not pass through name.
			return nil
		}
		chain = append(chain, next)
		cur = next
	}
}

// checkWithin checks one scoped relation that is in no within cycle.
func checkWithin(m *Metamodel, name string, rel RelationDef) []string {
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf("relation %q: "+format, append([]any{name}, args...)...))
	}
	switch {
	case rel.OnChange == "":
		add("within requires on_change (clear or reject)")
	case !rel.OnChange.IsValid():
		add("invalid on_change value %q (allowed: clear, reject)", string(rel.OnChange))
	}
	if rel.Owning {
		add("within cannot be combined with owning")
	}
	if rel.Symmetric {
		add("within cannot be combined with symmetric")
	}
	if rel.Scope.IsContent() {
		add("within is not supported on a content-scoped relation")
	}
	scope, ok := m.Relations[rel.Within]
	if !ok {
		add("within names unknown relation %q", rel.Within)
		return errs
	}
	if scope.Within != "" {
		add("within names %q, which is scoped itself; scoping is one level deep", rel.Within)
	}
	if scope.Symmetric {
		add("within names %q, which is symmetric", rel.Within)
	}
	if scope.Scope.IsContent() {
		add("within names %q, which is content-scoped", rel.Within)
	}
	if scope.MaxOutgoing == nil || *scope.MaxOutgoing != 1 {
		add("within names %q, which must be single-valued (max_outgoing: 1)", rel.Within)
	}
	for _, t := range rel.From {
		if !slices.Contains(scope.From, t) {
			add("within names %q, whose from does not include %q", rel.Within, t)
		}
	}
	for _, t := range rel.To {
		switch owners := scopeOwners(m, rel, t); len(owners) {
		case 1:
		case 0:
			add("type %q cannot resolve to an owner: no owning relation from a target of %q owns it",
				t, rel.Within)
		default:
			add("type %q cannot resolve to an owner: owning relations %s each own it from a target of %q",
				t, strings.Join(owners, ", "), rel.Within)
		}
	}
	return errs
}
