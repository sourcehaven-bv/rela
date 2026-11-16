package metamodel

import (
	"maps"
	"slices"
)

// The queries below answer schema questions that config files ask about the
// entity types they name: flows.yaml triggers and data-entry.yaml both scope
// settings by type, face and copy. Each takes a nil Metamodel as an empty
// schema, so a caller that checks before the schema loads gets "not
// declared" rather than a panic.

// ExpandTypeNames returns names with every abstract type replaced by its
// variants, in order and without duplicates. An abstract type stores no
// entities, so a setting that names one means all of its variants. Nil:
// accepted for names, returned as nil so "no list" stays distinct from
// "an empty list".
func ExpandTypeNames(m *Metamodel, names []string) []string {
	if names == nil {
		return nil
	}
	out := make([]string, 0, len(names))
	add := func(name string) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, name := range names {
		variants, ok := VariantsOf(m, name)
		if !ok {
			add(name)
			continue
		}
		for _, v := range variants {
			add(v)
		}
	}
	return out
}

// EntityTypeNames returns the declared entity type names, sorted.
func EntityTypeNames(m *Metamodel) []string {
	if m == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(m.Entities))
}

// DeclaresFace reports whether the entity type typ declares face.
func DeclaresFace(m *Metamodel, typ, face string) bool {
	if m == nil {
		return false
	}
	def, ok := m.Entities[typ]
	if !ok {
		return false
	}
	_, declared := def.Faces[face]
	return declared
}

// HasFaces reports whether the entity type typ declares any `faces:`.
func HasFaces(m *Metamodel, typ string) bool {
	if m == nil {
		return false
	}
	def, ok := m.Entities[typ]
	return ok && len(def.Faces) > 0
}

// CopyTargetOf returns where the declared copy name writes. declared is
// false when no copy has that name; ok is false when it is declared but its
// target does not parse, which the schema loader reports.
func CopyTargetOf(m *Metamodel, name string) (target CopyTarget, declared, ok bool) {
	if m == nil {
		return CopyTarget{}, false, false
	}
	def, declared := m.Copies[name]
	if !declared {
		return CopyTarget{}, false, false
	}
	to, err := ParseCopyTarget(def.To)
	if err != nil {
		return CopyTarget{}, true, false
	}
	return to, true, true
}
