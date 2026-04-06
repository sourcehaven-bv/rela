package metamodel

// Schema output interface methods for Metamodel

// GetVersion returns the metamodel version
func (m *Metamodel) GetVersion() string {
	return m.Version
}

// GetNamespace returns the metamodel namespace
func (m *Metamodel) GetNamespace() string {
	return m.Namespace
}

// GetEntities returns the entities map for JSON output
func (m *Metamodel) GetEntities() interface{} {
	return m.Entities
}

// GetRelations returns the relations map for JSON output
func (m *Metamodel) GetRelations() interface{} {
	return m.Relations
}

// GetTypes returns the custom types map for JSON output
func (m *Metamodel) GetTypes() interface{} {
	return m.Types
}

// Methods implementing migration.MetamodelProvider interface

// GetPropertyType returns the type of a property for an entity type (empty if not found).
func (m *Metamodel) GetPropertyType(entityType, property string) string {
	entDef, ok := m.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	propDef, ok := entDef.Properties[property]
	if !ok {
		return ""
	}
	return propDef.Type
}

// IsPropertyRequired returns whether a property is required.
func (m *Metamodel) IsPropertyRequired(entityType, property string) bool {
	entDef, ok := m.GetEntityDef(entityType)
	if !ok {
		return false
	}
	propDef, ok := entDef.Properties[property]
	if !ok {
		return false
	}
	return propDef.Required
}

// GetPropertyDefault returns the default value for a property.
func (m *Metamodel) GetPropertyDefault(entityType, property string) string {
	entDef, ok := m.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	propDef, ok := entDef.Properties[property]
	if !ok {
		return ""
	}
	return propDef.Default
}

// GetTypeDefault returns the default value for a custom type.
func (m *Metamodel) GetTypeDefault(typeName string) string {
	if ct, ok := m.Types[typeName]; ok {
		return ct.Default
	}
	return ""
}

// IsEnumType returns whether a type is an enum-like type (has values).
func (m *Metamodel) IsEnumType(typeName string) bool {
	if ct, ok := m.Types[typeName]; ok {
		return len(ct.Values) > 0
	}
	return false
}

// GetRelationLabel returns the label for a relation (empty if not found).
func (m *Metamodel) GetRelationLabel(relation string) string {
	relDef, ok := m.GetRelationDef(relation)
	if !ok {
		return ""
	}
	return relDef.Label
}

// GetRelationFrom returns the "from" entity types for a relation.
func (m *Metamodel) GetRelationFrom(relation string) []string {
	relDef, ok := m.GetRelationDef(relation)
	if !ok {
		return nil
	}
	return relDef.From
}

// GetRelationTo returns the "to" entity types for a relation.
func (m *Metamodel) GetRelationTo(relation string) []string {
	relDef, ok := m.GetRelationDef(relation)
	if !ok {
		return nil
	}
	return relDef.To
}

// ResolveWidgetFromType returns the canonical widget for a property type.
// This is the single source of truth for the type→widget mapping used by
// the data entry app and the migration system.
func (m *Metamodel) ResolveWidgetFromType(propType string) string {
	switch propType {
	case PropertyTypeString:
		return "text"
	case PropertyTypeDate:
		return "date"
	case PropertyTypeInteger:
		return "number"
	case PropertyTypeBoolean:
		return "checkbox"
	case PropertyTypeEnum:
		return "select"
	case PropertyTypeRrule:
		return "rrule"
	default:
		if ct, ok := m.Types[propType]; ok && len(ct.Values) > 0 {
			return "select"
		}
		return "text"
	}
}

// Schema output interface methods for EntityDef

// GetLabel returns the entity label
func (e *EntityDef) GetLabel() string {
	return e.Label
}

// GetAliases returns the entity aliases
func (e *EntityDef) GetAliases() []string {
	return e.Aliases
}

// GetIDPatterns returns the entity ID prefixes.
// Deprecated: Use GetIDPrefixes instead.
func (e *EntityDef) GetIDPatterns() []string {
	return e.GetIDPrefixes()
}

// GetProperties returns the entity properties for JSON output.
// Note: This returns interface{} to satisfy the SchemaEntityDef interface.
// For typed access, use PropertyDefs() which implements PropertySchema.
func (e *EntityDef) GetProperties() interface{} {
	return e.Properties
}

// GetRDFType returns the RDF type
func (e *EntityDef) GetRDFType() string {
	return e.RDFType
}

// GetColor returns the color
func (e *EntityDef) GetColor() string {
	return e.Color
}

// GetBorderColor returns the border color
func (e *EntityDef) GetBorderColor() string {
	return e.BorderColor
}

// Schema output interface methods for RelationDef

// GetLabel returns the relation label
func (r *RelationDef) GetLabel() string {
	return r.Label
}

// GetFrom returns the source entity types
func (r *RelationDef) GetFrom() []string {
	return r.From
}

// GetTo returns the target entity types
func (r *RelationDef) GetTo() []string {
	return r.To
}

// GetDescription returns the relation description
func (r *RelationDef) GetDescription() string {
	return r.Description
}

// GetInverse returns the inverse definition for JSON output
func (r *RelationDef) GetInverse() interface{} {
	if r.Inverse == nil {
		return nil
	}
	return r.Inverse
}

// IsSymmetric returns whether the relation is symmetric
func (r *RelationDef) IsSymmetric() bool {
	return r.Symmetric
}

// GetMinOutgoing returns the minimum outgoing cardinality (from-side constraint)
func (r *RelationDef) GetMinOutgoing() *int {
	return r.MinOutgoing
}

// GetMaxOutgoing returns the maximum outgoing cardinality (from-side constraint)
func (r *RelationDef) GetMaxOutgoing() *int {
	return r.MaxOutgoing
}

// GetMinIncoming returns the minimum incoming cardinality (to-side constraint)
func (r *RelationDef) GetMinIncoming() *int {
	return r.MinIncoming
}

// GetMaxIncoming returns the maximum incoming cardinality (to-side constraint)
func (r *RelationDef) GetMaxIncoming() *int {
	return r.MaxIncoming
}
