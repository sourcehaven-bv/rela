package metamodel

import "strings"

// rebuildAliasMap rebuilds the alias map from all entity definitions.
// Called after merging includes to ensure aliases from included files are registered.
func (m *Metamodel) rebuildAliasMap() {
	m.aliasMap = make(map[string]string)
	for name, def := range m.Entities {
		m.aliasMap[strings.ToLower(name)] = name
		for _, alias := range def.Aliases {
			m.aliasMap[strings.ToLower(alias)] = name
		}
	}
}

// InitAliases initializes the alias map from entity definitions.
// Call this after programmatically constructing a Metamodel (e.g., via testutil builders)
// to enable alias resolution. Not needed when using Parse() which calls this automatically.
func (m *Metamodel) InitAliases() {
	m.rebuildAliasMap()
}

// ResolveAlias returns the canonical entity type name for an alias
func (m *Metamodel) ResolveAlias(alias string) string {
	if m.aliasMap == nil {
		return alias
	}
	if canonical, ok := m.aliasMap[alias]; ok {
		return canonical
	}
	return alias
}

// GetEntityDef returns the entity definition for a type (resolving aliases)
func (m *Metamodel) GetEntityDef(entityType string) (*EntityDef, bool) {
	// First try direct lookup
	if def, ok := m.Entities[entityType]; ok {
		return &def, true
	}
	// Try alias resolution
	canonical := m.ResolveAlias(entityType)
	if def, ok := m.Entities[canonical]; ok {
		return &def, true
	}
	return nil, false
}

// DisplayTitle returns the display title for an entity using its type's primary property.
// Falls back to entity ID if no entity definition found or no primary property value is set.
func (m *Metamodel) DisplayTitle(id, entityType string, properties map[string]interface{}) string {
	if def, ok := m.GetEntityDef(entityType); ok {
		return def.DisplayTitle(id, properties)
	}
	return id
}

// GetRelationDef returns the relation definition
func (m *Metamodel) GetRelationDef(name string) (*RelationDef, bool) {
	if def, ok := m.Relations[name]; ok {
		return &def, true
	}
	return nil, false
}

// InferEntityType tries to determine the entity type from an ID
func (m *Metamodel) InferEntityType(id string) string {
	for name, def := range m.Entities {
		if def.MatchesID(id) {
			return name
		}
	}
	return ""
}

// ValidateRelation checks if a relation is valid between two entity types
func (m *Metamodel) ValidateRelation(relationType, fromType, toType string) error {
	rel, ok := m.GetRelationDef(relationType)
	if !ok {
		return &RelationNotFoundError{Name: relationType}
	}

	fromValid := false
	for _, t := range rel.From {
		if t == fromType {
			fromValid = true
			break
		}
	}
	if !fromValid {
		return &InvalidRelationError{
			Relation: relationType,
			From:     fromType,
			To:       toType,
			Message:  "source entity type not allowed",
		}
	}

	toValid := false
	for _, t := range rel.To {
		if t == toType {
			toValid = true
			break
		}
	}
	if !toValid {
		return &InvalidRelationError{
			Relation: relationType,
			From:     fromType,
			To:       toType,
			Message:  "target entity type not allowed",
		}
	}

	return nil
}

// EntityTypes returns all entity type names
func (m *Metamodel) EntityTypes() []string {
	types := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		types = append(types, name)
	}
	return types
}

// RelationTypes returns all relation type names
func (m *Metamodel) RelationTypes() []string {
	types := make([]string, 0, len(m.Relations))
	for name := range m.Relations {
		types = append(types, name)
	}
	return types
}

// HasEntityType returns true if the entity type exists in the metamodel.
func (m *Metamodel) HasEntityType(entityType string) bool {
	_, ok := m.GetEntityDef(entityType)
	return ok
}

// HasValidationRule returns true if a validation rule with the given name exists.
func (m *Metamodel) HasValidationRule(ruleName string) bool {
	for _, rule := range m.Validations {
		if rule.Name == ruleName {
			return true
		}
	}
	return false
}
