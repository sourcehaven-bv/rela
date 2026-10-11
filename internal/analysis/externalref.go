package analysis

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// hasExternalRefs reports whether any entity type declares an external ref.
func hasExternalRefs(meta *metamodel.Metamodel) bool {
	for typeName := range meta.Entities {
		if len(metamodel.ExternalRefPropertyNames(meta, typeName)) > 0 {
			return true
		}
	}
	return false
}

// externalRefViolations groups entities that hold the same (system, id) in
// one face (TKT-SM20FG, D12). The write path refuses a new duplicate across
// every type declaring the system; this reports ones already in the data,
// such as rows imported before the property was declared, or written to the
// files by hand.
func externalRefViolations(meta *metamodel.Metamodel, entities []*entity.Entity) []UniqueViolation {
	type key struct {
		system string
		face   entity.Face
		id     string
	}
	groups := make(map[key][]*entity.Entity)
	var order []key
	for _, e := range entities {
		for _, prop := range metamodel.ExternalRefPropertyNames(meta, e.Type) {
			id, ok := metamodel.ExternalRefID(e.Properties[prop])
			if !ok {
				continue
			}
			system, _ := metamodel.ExternalRefSystem(meta, e.Type, prop)
			k := key{system, e.Face, id}
			if _, seen := groups[k]; !seen {
				order = append(order, k)
			}
			groups[k] = append(groups[k], e)
		}
	}
	var out []UniqueViolation
	for _, k := range order {
		group := groups[k]
		if distinctIDs(group) < 2 {
			continue
		}
		sortRows(group)
		out = append(out, UniqueViolation{System: k.system, Face: k.face, Value: k.id, Entities: group})
	}
	return out
}
