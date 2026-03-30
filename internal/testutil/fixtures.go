package testutil

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// EntityBuilder provides a fluent interface for building test entities.
// Builders should not be reused after Build() - each call creates a fresh builder.
type EntityBuilder struct {
	entity     *model.Entity
	meta       *metamodel.Metamodel
	skip       map[string]bool // properties to skip during auto-fill
	hasID      bool            // whether ID was explicitly set
	entityType string          // original entity type for metamodel lookup
}

// NewEntity creates a new entity builder with the given ID and type.
func NewEntity(id, entityType string) *EntityBuilder {
	return &EntityBuilder{
		entity:     model.NewEntity(id, entityType),
		skip:       make(map[string]bool),
		hasID:      true,
		entityType: entityType,
	}
}

// Entity creates a simple entity builder with a random ID.
// The entity will have a random ID unless set explicitly with ID().
func Entity(entityType string) *EntityBuilder {
	return &EntityBuilder{
		entity:     model.NewEntity("", entityType),
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
		entity:     model.NewEntity("", entityType),
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
func (b *EntityBuilder) WithProperty(key string, value interface{}) *EntityBuilder {
	b.entity.Properties[key] = value
	return b
}

// With is an alias for WithProperty for more concise syntax.
func (b *EntityBuilder) With(key string, value interface{}) *EntityBuilder {
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

// WithStatus adds a status property to the entity.
func (b *EntityBuilder) WithStatus(status model.Status) *EntityBuilder {
	return b.WithProperty("status", string(status))
}

// WithDescription adds a description property to the entity.
func (b *EntityBuilder) WithDescription(desc string) *EntityBuilder {
	return b.WithProperty("description", desc)
}

// WithPriority adds a priority property to the entity.
func (b *EntityBuilder) WithPriority(priority model.Priority) *EntityBuilder {
	return b.WithProperty("priority", string(priority))
}

// WithContent sets the content of the entity.
func (b *EntityBuilder) WithContent(content string) *EntityBuilder {
	b.entity.Content = content
	return b
}

// Build returns the built entity.
func (b *EntityBuilder) Build() *model.Entity {
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
func (b *EntityBuilder) generatePropertyValue(_ string, prop metamodel.PropertyDef) interface{} {
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
	case metamodel.PropertyTypeBoolean:
		return RandomBool()
	case metamodel.PropertyTypeDate:
		return RandomDate()
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
	relation *model.Relation
}

// NewRelation creates a new relation builder with from, type, and to already set.
func NewRelation(from, relationType, to string) *RelationBuilder {
	return &RelationBuilder{
		relation: model.NewRelation(from, relationType, to),
	}
}

// Relation creates a new relation builder with just the relation type.
// Use From() and To() to set the source and target entity IDs.
func Relation(relationType string) *RelationBuilder {
	return &RelationBuilder{
		relation: model.NewRelation("", relationType, ""),
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

// WithProperty adds a property to the relation.
func (b *RelationBuilder) WithProperty(key string, value interface{}) *RelationBuilder {
	if b.relation.Properties == nil {
		b.relation.Properties = make(map[string]interface{})
	}
	b.relation.Properties[key] = value
	return b
}

// WithContent sets the relation's markdown content.
func (b *RelationBuilder) WithContent(content string) *RelationBuilder {
	b.relation.Content = content
	return b
}

// Build returns the built relation.
// Panics if From or To are not set.
func (b *RelationBuilder) Build() *model.Relation {
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

// WithEntity adds an entity definition to the metamodel.
func (b *MetamodelBuilder) WithEntity(name, label string, idPatterns []string) *MetamodelBuilder {
	def := metamodel.EntityDef{
		Label:      label,
		Properties: make(map[string]metamodel.PropertyDef),
	}
	// Convert idPatterns to id_prefix or id_prefixes based on length
	if len(idPatterns) == 1 {
		def.IDPrefix = idPatterns[0]
	} else if len(idPatterns) > 1 {
		def.IDPrefixes = idPatterns
	}
	b.meta.Entities[name] = def
	return b
}

// WithEntityProperty adds a property to an entity definition.
func (b *MetamodelBuilder) WithEntityProperty(entityName, propName, propType string, required bool) *MetamodelBuilder {
	entity := b.meta.Entities[entityName]
	entity.Properties[propName] = metamodel.PropertyDef{
		Type:     propType,
		Required: required,
	}
	b.meta.Entities[entityName] = entity
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

// Build returns the built metamodel.
func (b *MetamodelBuilder) Build() *metamodel.Metamodel {
	return b.meta
}

// ProjectContext holds project context paths for testing.
type ProjectContext struct {
	Root         string
	EntitiesDir  string
	RelationsDir string
	CacheDir     string
}

// ProjectBuilder provides a fluent interface for building test projects.
type ProjectBuilder struct {
	t       *testing.T
	tmpDir  string
	ctx     *ProjectContext
	meta    *metamodel.Metamodel
	graph   *graph.Graph
	hasInit bool
}

// NewProject creates a new project builder.
func NewProject(t *testing.T) *ProjectBuilder {
	t.Helper()

	tmpDir := TempDirWithCleanup(t)

	return &ProjectBuilder{
		t:      t,
		tmpDir: tmpDir,
		ctx: &ProjectContext{
			Root:         tmpDir,
			EntitiesDir:  filepath.Join(tmpDir, "entities"),
			RelationsDir: filepath.Join(tmpDir, "relations"),
			CacheDir:     filepath.Join(tmpDir, ".rela"),
		},
		meta:  NewMetamodel().Build(),
		graph: graph.New(),
	}
}

// WithMetamodel sets the metamodel for the project.
func (b *ProjectBuilder) WithMetamodel(meta *metamodel.Metamodel) *ProjectBuilder {
	b.meta = meta
	return b
}

// WithMetamodelYAML writes a metamodel.yaml file to the project.
func (b *ProjectBuilder) WithMetamodelYAML(yaml string) *ProjectBuilder {
	b.t.Helper()

	metamodelPath := filepath.Join(b.tmpDir, "metamodel.yaml")
	CreateFile(b.t, metamodelPath, yaml)

	// Parse the metamodel
	meta, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		b.t.Fatalf("failed to parse metamodel: %v", err)
	}

	b.meta = meta
	return b
}

// Init initializes project directories.
func (b *ProjectBuilder) Init() *ProjectBuilder {
	b.t.Helper()

	if b.hasInit {
		return b
	}

	CreateDir(b.t, b.ctx.EntitiesDir)
	CreateDir(b.t, b.ctx.RelationsDir)
	CreateDir(b.t, b.ctx.CacheDir)

	b.hasInit = true
	return b
}

// WithEntity adds an entity file to the project.
func (b *ProjectBuilder) WithEntity(entity *model.Entity) *ProjectBuilder {
	b.t.Helper()

	b.Init()

	// Create type directory
	typeDir := filepath.Join(b.ctx.EntitiesDir, entity.Type)
	CreateDir(b.t, typeDir)

	// Create entity file
	entityPath := filepath.Join(typeDir, entity.ID+".md")

	// Build content
	content := "---\n"
	content += "id: " + entity.ID + "\n"
	content += "type: " + entity.Type + "\n"

	// Add properties
	for key, value := range entity.Properties {
		content += key + ": " + toString(value) + "\n"
	}

	content += "---\n"

	if entity.Content != "" {
		content += "\n" + entity.Content + "\n"
	}

	CreateFile(b.t, entityPath, content)

	return b
}

// WithRelation adds a relation file to the project.
func (b *ProjectBuilder) WithRelation(relation *model.Relation) *ProjectBuilder {
	b.t.Helper()

	b.Init()

	// Create relation file
	filename := relation.From + "--" + relation.Type + "--" + relation.To + ".md"
	relationPath := filepath.Join(b.ctx.RelationsDir, filename)

	// Build content
	content := "---\n"
	content += "from: " + relation.From + "\n"
	content += "relation: " + relation.Type + "\n"
	content += "to: " + relation.To + "\n"

	// Add properties
	for key, value := range relation.Properties {
		content += key + ": " + toString(value) + "\n"
	}

	content += "---\n"

	CreateFile(b.t, relationPath, content)

	return b
}

// Build returns the project context, metamodel, and graph.
func (b *ProjectBuilder) Build() (*ProjectContext, *metamodel.Metamodel, *graph.Graph) {
	b.t.Helper()

	b.Init()

	return b.ctx, b.meta, b.graph
}

// BuildContext returns just the project context.
func (b *ProjectBuilder) BuildContext() *ProjectContext {
	b.t.Helper()

	b.Init()

	return b.ctx
}

// toString converts a value to a string representation for YAML.
func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []string:
		// Format as YAML list
		if len(v) == 0 {
			return "[]"
		}
		var sb strings.Builder
		sb.WriteString("[")
		for i, s := range v {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(s)
		}
		sb.WriteString("]")
		return sb.String()
	default:
		return ""
	}
}

// SimpleMetamodel returns a simple metamodel for testing with requirement and decision types.
func SimpleMetamodel() *metamodel.Metamodel {
	return NewMetamodel().
		WithEntity("requirement", "Requirement", []string{"REQ-"}).
		WithEntityProperty("requirement", "title", metamodel.PropertyTypeString, true).
		WithEntityProperty("requirement", "status", "status", false).
		WithEntity("decision", "Decision", []string{"DEC-"}).
		WithEntityProperty("decision", "title", metamodel.PropertyTypeString, true).
		WithEntityProperty("decision", "status", "status", false).
		WithRelation("addresses", "Addresses", []string{"decision"}, []string{"requirement"}).
		WithCustomType("status", []string{"draft", "proposed", "accepted", "rejected", "deprecated", "retired"}).
		Build()
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
