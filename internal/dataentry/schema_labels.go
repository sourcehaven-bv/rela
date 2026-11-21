package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A column or field without its own `label:` shows the label the schema
// gives its property or relation, so a list, a view and a form name a
// property the same way unless the operator chose otherwise. Forms and the
// Changes list always used the schema label; columns and view fields used
// the raw property name, so operators repeated labels in data-entry.yaml
// and the copies drifted ("Status" in a form, "Account status" on the
// page). The schema label is authored too, so this keeps DEC-6C1NAA: no
// label is derived. The raw name stays the last resort.

// schemaPropertyLabel is the schema label of property on the first of
// types that declares it with one, or "" when none does.
func schemaPropertyLabel(meta *metamodel.Metamodel, types []string, property string) string {
	if meta == nil || property == "" {
		return ""
	}
	for _, t := range types {
		def, ok := meta.GetEntityDef(t)
		if !ok {
			continue
		}
		if pd, ok := def.Properties[property]; ok && pd.Label != "" {
			return pd.Label
		}
	}
	return ""
}

// schemaRelationLabel is the schema label of relation as seen from
// entityType: the inverse label for an incoming column when the schema
// declares one, otherwise the relation's own label. "" when neither is set.
func schemaRelationLabel(meta *metamodel.Metamodel, entityType, relation string, dir dataentryconfig.Direction) string {
	if meta == nil || relation == "" {
		return ""
	}
	def, ok := meta.GetRelationDef(relation)
	if !ok {
		return ""
	}
	if dir == "" {
		// An ambiguous binding is refused at config load; reading it as
		// outgoing here only picks which schema label to show.
		if inferred, res := dataentryconfig.InferDirection(entityType, relation, meta); res == dataentryconfig.DirectionResolved {
			dir = inferred
		}
	}
	if dir.IsIncoming() &&
		def.Inverse != nil && def.Inverse.Label != "" {
		return def.Inverse.Label
	}
	return def.Label
}

// withSchemaLabels returns cols with each empty label filled from the
// schema, read on the first of types that knows the property or relation.
// The input is not changed: it is the loaded config.
func withSchemaLabels(meta *metamodel.Metamodel, types []string, cols []dataentryconfig.ListColumn) []dataentryconfig.ListColumn {
	if len(cols) == 0 {
		return cols
	}
	out := make([]dataentryconfig.ListColumn, len(cols))
	copy(out, cols)
	for i := range out {
		c := &out[i]
		if c.Label != "" {
			continue
		}
		switch {
		case c.Property != "":
			c.Label = schemaPropertyLabel(meta, types, c.Property)
		case c.Relation != "":
			for _, t := range types {
				if l := schemaRelationLabel(meta, t, c.Relation, c.Direction); l != "" {
					c.Label = l
					break
				}
			}
		}
	}
	return out
}

// entityTypesOf returns the distinct types of entities, in first-seen order.
func entityTypesOf(entities []*entity.Entity) []string {
	var types []string
	seen := map[string]bool{}
	for _, e := range entities {
		if !seen[e.Type] {
			seen[e.Type] = true
			types = append(types, e.Type)
		}
	}
	return types
}
