package testutil

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// EntityBuilder provides a fluent interface for building test entities.
// Builders should not be reused after Build() - each call creates a fresh builder.
type EntityBuilder struct {
	entity     *entity.Entity
	meta       *metamodel.Metamodel
	skip       map[string]bool // properties to skip during auto-fill
	hasID      bool            // whether ID was explicitly set
	entityType string          // original entity type for metamodel lookup
}

// NewEntity creates a new entity builder with the given ID and type.
func NewEntity(id, entityType string) *EntityBuilder {
	return &EntityBuilder{
		entity:     entity.New(id, entityType),
		skip:       make(map[string]bool),
		hasID:      true,
		entityType: entityType,
	}
}

// Entity creates a simple entity builder with a random ID.
// The entity will have a random ID unless set explicitly with ID().
func Entity(entityType string) *EntityBuilder {
	return &EntityBuilder{
		entity:     entity.New("", entityType),
		skip:       make(map[string]bool),
		hasID:      false,
		entityType: entityType,
	}
}

// EntityFor creates a metamodel-aware entity builder that auto-fills required properties.
// Panics if meta is nil or entityType is not defined in the metamodel.
func EntityFor(meta *metamodel.Metamodel, entityType string) *EntityBuilder {
	if meta == nil {
		panic("EntityFor: metamodel cannot be nil")
	}
	if _, ok := meta.Entities[entityType]; !ok {
		panic("EntityFor: unknown entity type: " + entityType)
	}
	return &EntityBuilder{
		entity:     entity.New("", entityType),
		meta:       meta,
		skip:       make(map[string]bool),
		hasID:      false,
		entityType: entityType,
	}
}

// ID sets the entity ID explicitly.
func (b *EntityBuilder) ID(id string) *EntityBuilder {
	b.entity.ID = id
	b.hasID = true
	return b
}

// WithProperty adds a property to the entity.
func (b *EntityBuilder) WithProperty(key string, value any) *EntityBuilder {
	b.entity.Properties[key] = value
	return b
}

// With is an alias for WithProperty for more concise syntax.
func (b *EntityBuilder) With(key string, value any) *EntityBuilder {
	return b.WithProperty(key, value)
}

// WithList sets a list property value.
func (b *EntityBuilder) WithList(key string, values ...string) *EntityBuilder {
	b.entity.Properties[key] = values
	return b
}

// Without marks a property to be skipped during auto-fill (only relevant for EntityFor).
func (b *EntityBuilder) Without(key string) *EntityBuilder {
	b.skip[key] = true
	return b
}

// WithTitle adds a title property to the entity.
func (b *EntityBuilder) WithTitle(title string) *EntityBuilder {
	return b.WithProperty("title", title)
}

// WithContent sets the content of the entity.
func (b *EntityBuilder) WithContent(content string) *EntityBuilder {
	b.entity.Content = content
	return b
}

// Build returns the built entity.
func (b *EntityBuilder) Build() *entity.Entity {
	// Generate ID if not set
	if !b.hasID {
		b.entity.ID = b.generateID()
	}

	// Auto-fill required properties if metamodel-aware
	if b.meta != nil {
		b.autoFillProperties()
	}

	return b.entity
}

// generateID generates an appropriate ID based on metamodel or defaults.
func (b *EntityBuilder) generateID() string {
	if b.meta != nil {
		if def, ok := b.meta.Entities[b.entityType]; ok {
			// Use first ID prefix if available
			if len(def.IDPrefixes) > 0 {
				return RandomID(def.IDPrefixes[0])
			}
			if def.IDPrefix != "" {
				return RandomID(def.IDPrefix)
			}
		}
	}
	// Default: use uppercase entity type as prefix
	return RandomID(b.entityType)
}

// autoFillProperties fills required properties with random values.
func (b *EntityBuilder) autoFillProperties() {
	def := b.meta.Entities[b.entityType]

	for propName, propDef := range def.Properties {
		// Skip if explicitly set or marked to skip
		if _, hasExplicit := b.entity.Properties[propName]; hasExplicit {
			continue
		}
		if b.skip[propName] {
			continue
		}

		// Only auto-fill required properties
		if !propDef.Required {
			continue
		}

		value := b.generatePropertyValue(propName, propDef)
		if value != nil {
			b.entity.Properties[propName] = value
		}
	}
}

// generatePropertyValue generates a random value for a property based on its type.
func (b *EntityBuilder) generatePropertyValue(_ string, prop metamodel.PropertyDef) any {
	// Check for enum values (inline or from custom type)
	values := prop.Values
	if len(values) == 0 && b.meta != nil {
		if customType, ok := b.meta.Types[prop.Type]; ok {
			values = customType.Values
		}
	}

	// If we have enum values, pick one
	if len(values) > 0 {
		if prop.List {
			// For list properties, return a slice with one random value
			return []string{RandomEnumValue(values)}
		}
		return RandomEnumValue(values)
	}

	// Generate based on built-in type
	switch prop.Type {
	case metamodel.PropertyTypeString, "":
		return RandomString()
	case metamodel.PropertyTypeInteger:
		return RandomInt(1, 100)
	case metamodel.PropertyTypeDecimal:
		const maxCents = 99
		return fmt.Sprintf("%d.%02d", RandomInt(1, 100), RandomInt(0, maxCents))
	case metamodel.PropertyTypeBoolean:
		return RandomBool()
	case metamodel.PropertyTypeDate:
		return RandomDate()
	case metamodel.PropertyTypeDatetime:
		return RandomDatetime()
	case metamodel.PropertyTypeFile:
		return "test-file.txt"
	default:
		// Custom type without enum values - treat as string
		return RandomString()
	}
}

// RelationBuilder provides a fluent interface for building test relations.
// Builders should not be reused after Build() - each call creates a fresh builder.
type RelationBuilder struct {
	relation *entity.Relation
}

// NewRelation creates a new relation builder with from, type, and to already set.
func NewRelation(from, relationType, to string) *RelationBuilder {
	return &RelationBuilder{
		relation: entity.NewRelation(from, relationType, to),
	}
}

// Relation creates a new relation builder with just the relation type.
// Use From() and To() to set the source and target entity IDs.
func Relation(relationType string) *RelationBuilder {
	return &RelationBuilder{
		relation: entity.NewRelation("", relationType, ""),
	}
}

// From sets the source entity ID.
func (b *RelationBuilder) From(id string) *RelationBuilder {
	b.relation.From = id
	return b
}

// To sets the target entity ID.
func (b *RelationBuilder) To(id string) *RelationBuilder {
	b.relation.To = id
	return b
}

// WithContent sets the relation's markdown content.
func (b *RelationBuilder) WithContent(content string) *RelationBuilder {
	b.relation.Content = content
	return b
}

// Build returns the built relation.
// Panics if From or To are not set.
func (b *RelationBuilder) Build() *entity.Relation {
	if b.relation.From == "" {
		panic("RelationBuilder.Build: From is required")
	}
	if b.relation.To == "" {
		panic("RelationBuilder.Build: To is required")
	}
	return b.relation
}

// MetamodelBuilder provides a fluent interface for building test metamodels.
type MetamodelBuilder struct {
	meta *metamodel.Metamodel
}

// NewMetamodel creates a new metamodel builder.
func NewMetamodel() *MetamodelBuilder {
	return &MetamodelBuilder{
		meta: &metamodel.Metamodel{
			Version:   "1.0",
			Entities:  make(map[string]metamodel.EntityDef),
			Relations: make(map[string]metamodel.RelationDef),
			Types:     make(map[string]metamodel.CustomType),
		},
	}
}

// EntityDefBuilder provides a fluent interface for building entity definitions.
type EntityDefBuilder struct {
	parent     *MetamodelBuilder
	name       string
	def        metamodel.EntityDef
	propOrder  []string
	properties map[string]metamodel.PropertyDef
}

// DefineEntity starts building an entity definition with the fluent API. Call End() to finish.
func (b *MetamodelBuilder) DefineEntity(name string) *EntityDefBuilder {
	return &EntityDefBuilder{
		parent:     b,
		name:       name,
		def:        metamodel.EntityDef{},
		properties: make(map[string]metamodel.PropertyDef),
	}
}

// WithEntity adds an entity definition with common defaults (simple 3-arg form).
func (b *MetamodelBuilder) WithEntity(name, label string, idPatterns []string) *MetamodelBuilder {
	def := metamodel.EntityDef{
		Label:      label,
		Properties: make(map[string]metamodel.PropertyDef),
	}
	if len(idPatterns) == 1 {
		def.IDPrefix = idPatterns[0]
	} else if len(idPatterns) > 1 {
		def.IDPrefixes = idPatterns
	}
	b.meta.Entities[name] = def
	return b
}

// Label sets the entity label.
func (e *EntityDefBuilder) Label(label string) *EntityDefBuilder {
	e.def.Label = label
	return e
}

// IDPrefix sets a single ID prefix.
func (e *EntityDefBuilder) IDPrefix(prefix string) *EntityDefBuilder {
	e.def.IDPrefix = prefix
	return e
}

// Aliases sets entity type aliases.
func (e *EntityDefBuilder) Aliases(aliases ...string) *EntityDefBuilder {
	e.def.Aliases = aliases
	return e
}

// Prop adds a simple property with just type and required flag.
func (e *EntityDefBuilder) Prop(name, propType string, required bool) *EntityDefBuilder {
	e.propOrder = append(e.propOrder, name)
	e.properties[name] = metamodel.PropertyDef{
		Type:     propType,
		Required: required,
	}
	return e
}

// End finishes building the entity and adds it to the metamodel.
func (e *EntityDefBuilder) End() *MetamodelBuilder {
	e.def.Properties = e.properties
	e.def.PropertyOrder = e.propOrder
	e.parent.meta.Entities[e.name] = e.def
	return e.parent
}

// WithEntityProperty adds a property to an entity definition.
func (b *MetamodelBuilder) WithEntityProperty(entityName, propName, propType string, required bool) *MetamodelBuilder {
	entity := b.meta.Entities[entityName]
	if entity.Properties == nil {
		entity.Properties = make(map[string]metamodel.PropertyDef)
	}
	entity.Properties[propName] = metamodel.PropertyDef{
		Type:     propType,
		Required: required,
	}
	b.meta.Entities[entityName] = entity
	return b
}

// WithValidation appends a validation rule to the metamodel. Use for
// tests whose subject is the rule itself (when/then or Lua) — the rule
// stays at the call site while the entity-shape boilerplate comes from
// the builder.
func (b *MetamodelBuilder) WithValidation(rule metamodel.ValidationRule) *MetamodelBuilder {
	b.meta.Validations = append(b.meta.Validations, rule)
	return b
}

// WithRelation adds a relation definition to the metamodel.
func (b *MetamodelBuilder) WithRelation(name, label string, from, to []string) *MetamodelBuilder {
	b.meta.Relations[name] = metamodel.RelationDef{
		Label: label,
		From:  from,
		To:    to,
	}
	return b
}

// WithCustomType adds a custom type to the metamodel.
func (b *MetamodelBuilder) WithCustomType(name string, values []string) *MetamodelBuilder {
	b.meta.Types[name] = metamodel.CustomType{
		Values: values,
	}
	return b
}

// WithCustomTypeDefault adds a custom type with a default value.
func (b *MetamodelBuilder) WithCustomTypeDefault(name string, values []string, defaultVal string) *MetamodelBuilder {
	b.meta.Types[name] = metamodel.CustomType{
		Values:  values,
		Default: defaultVal,
	}
	return b
}

// Build returns the built metamodel, initializing the alias map.
func (b *MetamodelBuilder) Build() *metamodel.Metamodel {
	b.meta.InitAliases()
	return b.meta
}

// SimpleMetamodelYAML returns a simple metamodel YAML for testing.
func SimpleMetamodelYAML() string {
	return `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    properties:
      title:
        type: string
        required: true
      status:
        type: status
  decision:
    label: Decision
    id_prefix: "DEC-"
    properties:
      title:
        type: string
        required: true
      status:
        type: status
relations:
  addresses:
    label: Addresses
    from: [decision]
    to: [requirement]
types:
  status:
    values: [draft, proposed, accepted, rejected, deprecated, retired]
`
}

// AliasMetamodelYAML returns the YAML for AliasMetamodel().
func AliasMetamodelYAML() string {
	return `version: "1.0"
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_prefix: "REQ-"
    properties:
      title:
        type: string
        required: true
      status:
        type: status
        required: true
  control:
    label: Control
    aliases: [ctrl]
    id_prefix: "CTRL-"
    properties:
      title:
        type: string
        required: true
types:
  status:
    values: [draft, accepted]
    default: draft
`
}
