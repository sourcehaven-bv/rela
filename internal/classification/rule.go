package classification

import (
	"slices"
	"strconv"
	"strings"
)

// Rule is a derived label's `when:` clause: a condition over the labels
// present in a set of fields, evaluated at one scope.
type Rule struct {
	Scope Scope
	cond  cond
}

type condKind int

const (
	condSelector condKind = iota
	condAllOf
	condAnyOf
	condCount
)

// cond is one node of a rule. Every condition is positive (no negation), so
// adding a label to a field set can only make more rules match. Derive relies
// on that to reach a fixpoint.
type cond struct {
	kind     condKind
	sel      selector // condSelector, condCount
	children []cond   // condAllOf, condAnyOf
	min      int      // condCount
}

// selector matches a label by name, or every label with a role ("@role").
type selector struct {
	role  Role
	label string
}

func (s selector) String() string {
	if s.role != RoleNone {
		return "@" + string(s.role)
	}
	return s.label
}

// Field is one field in an evaluated set: a reference such as
// "person.email" and the labels it carries.
type Field struct {
	Ref    string
	Labels []string
	// Derived marks the pseudo-field that carries a derived label once its
	// rule matched. Pseudo-fields never count as breakers.
	Derived bool
	// Sources are the real fields behind a Derived field, when they are
	// known up front (a record-scope label carried into a subject profile).
	Sources []string
}

// matches reports whether field carries a label the selector selects.
func (f *File) matches(s selector, field Field) bool {
	for _, name := range field.Labels {
		if s.label != "" && name == s.label {
			return true
		}
		if s.role != RoleNone {
			if l, ok := f.Labels[name]; ok && l.Role == s.role {
				return true
			}
		}
	}
	return false
}

func (f *File) holds(c cond, fields []Field) bool {
	switch c.kind {
	case condSelector:
		for _, field := range fields {
			if f.matches(c.sel, field) {
				return true
			}
		}
		return false
	case condAllOf:
		for _, child := range c.children {
			if !f.holds(child, fields) {
				return false
			}
		}
		return true
	case condAnyOf:
		for _, child := range c.children {
			if f.holds(child, fields) {
				return true
			}
		}
		return false
	case condCount:
		// Count distinct real fields. A derived label stands for fields that
		// are already present, so counting it too would let "3 or more"
		// hold on two fields.
		seen := map[string]bool{}
		for _, field := range fields {
			if f.matches(c.sel, field) {
				for _, ref := range sourcesOf(field) {
					seen[ref] = true
				}
			}
		}
		return len(seen) >= c.min
	}
	return false
}

// Derive returns the derived labels whose rule at scope matches fields, in
// declaration order. Derived labels may build on each other (a derived label
// with role direct-identifier satisfies "@direct-identifier" in another rule),
// so evaluation repeats until nothing new matches. Rules are positive, so the
// loop ends after at most one pass per derived label.
//
// The returned pseudo-fields are appended to fields for evaluation only; the
// caller's slice is not modified.
func (f *File) Derive(scope Scope, fields []Field) []string {
	work := slices.Clone(fields)
	have := map[string]bool{}
	for _, field := range fields {
		for _, name := range field.Labels {
			have[name] = true
		}
	}
	var derived []string
	for changed := true; changed; {
		changed = false
		for _, name := range f.LabelOrder {
			l := f.Labels[name]
			if l.When == nil || l.When.Scope != scope || have[name] {
				continue
			}
			if f.holds(l.When.cond, work) {
				have[name] = true
				derived = append(derived, name)
				work = append(work, Field{
					Ref: "@" + name, Labels: []string{name}, Derived: true,
					Sources: f.satisfyingSources(l.When.cond, work),
				})
				changed = true
			}
		}
	}
	return f.inDeclarationOrder(derived)
}

func (f *File) inDeclarationOrder(names []string) []string {
	pos := make(map[string]int, len(f.LabelOrder))
	for i, n := range f.LabelOrder {
		pos[n] = i
	}
	slices.SortFunc(names, func(a, b string) int { return pos[a] - pos[b] })
	return names
}

// referencedLabels lists every label a condition names, for the
// undefined-label check.
func (c cond) referencedLabels() []string {
	var out []string
	if (c.kind == condSelector || c.kind == condCount) && c.sel.label != "" {
		out = append(out, c.sel.label)
	}
	for _, child := range c.children {
		out = append(out, child.referencedLabels()...)
	}
	return out
}

func (c cond) String() string {
	switch c.kind {
	case condSelector:
		return c.sel.String()
	case condCount:
		return "count(" + c.sel.String() + ") >= " + strconv.Itoa(c.min)
	case condAllOf, condAnyOf:
	}
	parts := make([]string, 0, len(c.children))
	for _, child := range c.children {
		parts = append(parts, child.String())
	}
	sep := " and "
	if c.kind == condAnyOf {
		sep = " or "
	}
	return "(" + strings.Join(parts, sep) + ")"
}
