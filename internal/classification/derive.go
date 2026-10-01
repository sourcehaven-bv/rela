package classification

import (
	"slices"
	"strings"
)

// Derivation is a derived label and the fields that made its rule hold.
type Derivation struct {
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
}

// Derivations returns the derived labels at scope for fields, each with its
// contributing fields, in declaration order.
func (f *File) Derivations(scope Scope, fields []Field) []Derivation {
	labels := f.Derive(scope, fields)
	out := make([]Derivation, 0, len(labels))
	for _, label := range labels {
		out = append(out, Derivation{Label: label, Fields: f.contributors(label, fields)})
	}
	return out
}

// contributors returns the refs of the fields that satisfy label's rule,
// sorted. A contribution through another derived label is expanded to that
// label's own contributors, so the result names only real fields.
func (f *File) contributors(label string, fields []Field) []string {
	l := f.Labels[label]
	if l == nil || l.When == nil {
		return nil
	}
	work := slices.Clone(fields)
	for _, name := range f.Derive(l.When.Scope, fields) {
		work = append(work, Field{Ref: "@" + name, Labels: []string{name}, Derived: true})
	}
	seen := map[string]bool{}
	var refs []string
	var add func(c cond)
	visiting := map[string]bool{label: true}
	add = func(c cond) {
		for _, field := range f.satisfying(c, work) {
			if !field.Derived || field.Sources != nil {
				for _, ref := range sourcesOf(field) {
					if !seen[ref] {
						seen[ref] = true
						refs = append(refs, ref)
					}
				}
				continue
			}
			name := field.Labels[0]
			if visiting[name] {
				continue
			}
			visiting[name] = true
			add(f.Labels[name].When.cond)
			visiting[name] = false
		}
	}
	add(l.When.cond)
	slices.Sort(refs)
	return refs
}

// satisfying returns the fields that take part in c holding: the matching
// fields of a selector or count, every child of all_of, and only the
// children of any_of that hold.
func (f *File) satisfying(c cond, fields []Field) []Field {
	var out []Field
	switch c.kind {
	case condSelector, condCount:
		for _, field := range fields {
			if f.matches(c.sel, field) {
				out = append(out, field)
			}
		}
	case condAllOf:
		for _, child := range c.children {
			out = append(out, f.satisfying(child, fields)...)
		}
	case condAnyOf:
		for _, child := range c.children {
			if f.holds(child, fields) {
				out = append(out, f.satisfying(child, fields)...)
			}
		}
	}
	return out
}

// satisfyingSources returns the real fields behind the fields that make c
// hold. Every derived field in fields already carries its Sources.
func (f *File) satisfyingSources(c cond, fields []Field) []string {
	seen := map[string]bool{}
	var out []string
	for _, field := range f.satisfying(c, fields) {
		for _, ref := range sourcesOf(field) {
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	slices.Sort(out)
	return out
}

func sourcesOf(field Field) []string {
	if field.Derived {
		return field.Sources
	}
	return []string{field.Ref}
}

func joinRefs(refs []string) string {
	return strings.Join(refs, ", ")
}
