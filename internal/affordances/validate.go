package affordances

import "fmt"

// validateField checks that field is a declared property of entityType.
// A typo'd field name would otherwise allow the bogus name and
// closed-world DENY the real one (the inverse of intent), with no
// startup error — so this fails loudly like a predicate compile error
// (RR-RTJE / S2).
func (r *PolicyResolver) validateField(roleName, entityType, block string, idx int, field string) error {
	def, ok := r.meta.Entities[entityType]
	if !ok {
		return fmt.Errorf("roles.%s.%s.%s[%d]: unknown entity type %q",
			roleName, block, entityType, idx, entityType)
	}
	if _, ok := def.Properties[field]; !ok {
		return fmt.Errorf("roles.%s.%s.%s[%d]: unknown field %q on type %q",
			roleName, block, entityType, idx, field, entityType)
	}
	return nil
}

// validateOption checks that field is an enum-typed property of
// entityType and option is one of its declared values.
func (r *PolicyResolver) validateOption(roleName, entityType string, idx int, field, option string) error {
	def, ok := r.meta.Entities[entityType]
	if !ok {
		return fmt.Errorf("roles.%s.options.%s[%d]: unknown entity type %q",
			roleName, entityType, idx, entityType)
	}
	prop, ok := def.Properties[field]
	if !ok {
		return fmt.Errorf("roles.%s.options.%s[%d]: unknown field %q on type %q",
			roleName, entityType, idx, field, entityType)
	}
	values := prop.Values
	if len(values) == 0 {
		if ct, ok := r.meta.Types[prop.Type]; ok {
			values = ct.Values
		}
	}
	if len(values) == 0 {
		return fmt.Errorf("roles.%s.options.%s[%d]: field %q on type %q is not enum-typed",
			roleName, entityType, idx, field, entityType)
	}
	for _, v := range values {
		if v == option {
			return nil
		}
	}
	return fmt.Errorf("roles.%s.options.%s[%d]: option %q is not a declared value of field %q",
		roleName, entityType, idx, option, field)
}

// validateRelation checks that relType is a declared relation type
// valid as an outgoing edge from entityType.
func (r *PolicyResolver) validateRelation(roleName, entityType string, idx int, relType string) error {
	rel, ok := r.meta.Relations[relType]
	if !ok {
		return fmt.Errorf("roles.%s.relations.%s[%d]: unknown relation type %q",
			roleName, entityType, idx, relType)
	}
	// A relation grant on an entity type only gates outgoing edges of
	// that type; require entityType to be a valid source.
	if len(rel.From) > 0 {
		for _, from := range rel.From {
			if from == entityType {
				return nil
			}
		}
		return fmt.Errorf("roles.%s.relations.%s[%d]: relation %q does not originate from type %q",
			roleName, entityType, idx, relType, entityType)
	}
	return nil
}
