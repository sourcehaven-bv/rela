package metamodel

import (
	"fmt"
	"strings"
)

// ReservedPrefix starts every type, relation and property name that rela
// manages itself, such as the `_flow-run` type and its `_carries` and
// `_produced` relations (FEAT-J0O10QI) and the order properties of an
// orderable relation. A schema may not declare a name that starts with it,
// so a rela-managed name never collides with an operator's.
const ReservedPrefix = "_"

// IsReservedName reports whether name, or its local part after a
// namespace (`isms._run`), starts with [ReservedPrefix].
func IsReservedName(name string) bool {
	if strings.HasPrefix(name, ReservedPrefix) {
		return true
	}
	_, local, ok := strings.Cut(name, NamespaceSeparator)
	return ok && strings.HasPrefix(local, ReservedPrefix)
}

// validateReservedNames refuses a declared entity type, type alias,
// relation type, inverse relation or property whose name starts with
// [ReservedPrefix]. The managed order properties have their own, more
// specific message in [validateRelationProperties], so they are skipped
// here.
func validateReservedNames(m *Metamodel) []string {
	var errs []string
	reserved := func(what, name string) {
		errs = append(errs, fmt.Sprintf("%s %q: names starting with %q are reserved for rela", what, name, ReservedPrefix))
	}
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		if IsReservedName(name) {
			reserved("entity type", name)
		}
		for _, alias := range def.Aliases {
			if IsReservedName(alias) {
				reserved(fmt.Sprintf("entity %q: alias", name), alias)
			}
		}
		for _, prop := range sortedKeys(def.Properties) {
			if IsReservedName(prop) {
				reserved(fmt.Sprintf("entity %q: property", name), prop)
			}
		}
	}
	for _, name := range sortedKeys(m.Relations) {
		rel := m.Relations[name]
		if IsReservedName(name) {
			reserved("relation type", name)
		}
		if rel.Inverse != nil && IsReservedName(rel.Inverse.ID) {
			reserved(fmt.Sprintf("relation %q: inverse", name), rel.Inverse.ID)
		}
		for _, prop := range sortedKeys(rel.Properties) {
			if prop == OrderPropertyOut || prop == OrderPropertyIn {
				continue
			}
			if IsReservedName(prop) {
				reserved(fmt.Sprintf("relation %q: property", name), prop)
			}
		}
	}
	return errs
}
