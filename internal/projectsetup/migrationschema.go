package projectsetup

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// migrationSchema adapts a loaded metamodel to migration.MetamodelProvider.
// The adapter lives at this wiring site because internal/migration cannot
// import internal/metamodel (metamodel imports migration). The zero value is
// not usable; build it in loadMigrationMetamodel.
type migrationSchema struct {
	meta *metamodel.Metamodel
}

// property looks up a property definition; entityType may be an alias. An
// unknown type or property yields the zero PropertyDef.
func (s migrationSchema) property(entityType, name string) metamodel.PropertyDef {
	entDef, ok := s.meta.GetEntityDef(entityType)
	if !ok {
		return metamodel.PropertyDef{}
	}
	return entDef.Properties[name]
}

// GetPropertyType returns the type of a property for an entity type (empty if not found).
func (s migrationSchema) GetPropertyType(entityType, property string) string {
	return s.property(entityType, property).Type
}

// IsPropertyRequired returns whether a property is required.
func (s migrationSchema) IsPropertyRequired(entityType, property string) bool {
	return s.property(entityType, property).Required
}

// GetPropertyDefault returns the default value for a property.
func (s migrationSchema) GetPropertyDefault(entityType, property string) string {
	return s.property(entityType, property).Default
}

// GetTypeDefault returns the default value for a custom type.
func (s migrationSchema) GetTypeDefault(typeName string) string {
	if ct, ok := s.meta.Types[typeName]; ok {
		return ct.Default
	}
	return ""
}

// GetRelationLabel returns the label for a relation (empty if not found).
func (s migrationSchema) GetRelationLabel(relation string) string {
	relDef, ok := s.meta.GetRelationDef(relation)
	if !ok {
		return ""
	}
	return relDef.Label
}

// GetRelationFrom returns the "from" entity types for a relation.
func (s migrationSchema) GetRelationFrom(relation string) []string {
	relDef, ok := s.meta.GetRelationDef(relation)
	if !ok {
		return nil
	}
	return relDef.From
}

// GetRelationTo returns the "to" entity types for a relation.
func (s migrationSchema) GetRelationTo(relation string) []string {
	relDef, ok := s.meta.GetRelationDef(relation)
	if !ok {
		return nil
	}
	return relDef.To
}

// ResolveWidgetFromType returns the table-cell widget for a property type.
func (s migrationSchema) ResolveWidgetFromType(propType string) string {
	return metamodel.ResolveWidgetFromType(s.meta, propType)
}
