package markdown

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// ErrTemplateNotFound is returned when a template file does not exist.
var ErrTemplateNotFound = fmt.Errorf("template not found")

// TemplateRelation represents a pre-filled relation in a template.
type TemplateRelation struct {
	Relation string `yaml:"relation"`
	Target   string `yaml:"target"`
}

// EntityTemplate represents a parsed entity template with optional variant name.
type EntityTemplate struct {
	Name       string                 // "" for default, "epic" for --epic variant
	EntityType string                 // The entity type this template is for
	Properties map[string]interface{} // Property defaults (excludes _template_relations)
	Content    string                 // Markdown body content
	Relations  []TemplateRelation     // Pre-filled relations
}

// LoadEntityTemplate reads an entity template file and returns the parsed document.
// Returns nil, nil if the template file does not exist.
func (f *FileIO) LoadEntityTemplate(ctx *project.Context, entityType string) (*Document, error) {
	path := ctx.EntityTemplatePath(entityType)
	doc, err := f.loadTemplate(path)
	if errors.Is(err, ErrTemplateNotFound) {
		return nil, nil //nolint:nilnil // nil,nil is intentional when template doesn't exist
	}
	return doc, err
}

// LoadRelationTemplate reads a relation template file and returns the parsed document.
// Returns nil, nil if the template file does not exist.
func (f *FileIO) LoadRelationTemplate(ctx *project.Context, relationType string) (*Document, error) {
	path := ctx.RelationTemplatePath(relationType)
	doc, err := f.loadTemplate(path)
	if errors.Is(err, ErrTemplateNotFound) {
		return nil, nil //nolint:nilnil // nil,nil is intentional when template doesn't exist
	}
	return doc, err
}

// DiscoverEntityTemplates returns all templates for an entity type, including variants.
// Templates are discovered by looking for files matching:
//   - <entityType>.md (default template)
//   - <entityType>--<variant>.md (variant templates)
//
// Returns templates sorted by name (default first, then alphabetically).
// Returns an empty slice if no templates exist (not an error).
func (f *FileIO) DiscoverEntityTemplates(ctx *project.Context, entityType string) ([]*EntityTemplate, error) {
	dir := ctx.EntityTemplatesDir

	// Check if templates directory exists
	if _, err := f.FS.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	// Read directory entries
	entries, err := f.FS.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	templates := make([]*EntityTemplate, 0, len(entries))
	prefix := entityType + "--"
	defaultFile := entityType + ".md"

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		var variantName string
		switch {
		case name == defaultFile:
			variantName = "" // default template
		case strings.HasPrefix(name, prefix):
			// Extract variant name: requirement--epic.md -> epic
			variantName = strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".md")
		default:
			continue // not a template for this entity type
		}

		// Load and parse the template
		path := filepath.Join(dir, name)
		tmpl, err := f.loadEntityTemplate(path, entityType, variantName)
		if err != nil {
			return nil, fmt.Errorf("failed to load template %s: %w", name, err)
		}
		templates = append(templates, tmpl)
	}

	// Sort: default first, then alphabetically by variant name
	sort.Slice(templates, func(i, j int) bool {
		// Default template (empty name) always comes first
		if templates[i].Name == "" {
			return true
		}
		if templates[j].Name == "" {
			return false
		}
		return natsort.Less(templates[i].Name, templates[j].Name)
	})

	return templates, nil
}

// loadEntityTemplate loads a single entity template file and parses it into an EntityTemplate.
func (f *FileIO) loadEntityTemplate(path, entityType, variantName string) (*EntityTemplate, error) {
	content, err := f.FS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	doc, err := ParseDocument(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Extract _template_relations from frontmatter
	relations := extractTemplateRelations(doc.Frontmatter)

	// Remove _template_relations from properties
	properties := make(map[string]interface{})
	for k, v := range doc.Frontmatter {
		if k != "_template_relations" {
			properties[k] = v
		}
	}

	return &EntityTemplate{
		Name:       variantName,
		EntityType: entityType,
		Properties: properties,
		Content:    doc.Content,
		Relations:  relations,
	}, nil
}

// extractTemplateRelations parses the _template_relations field from frontmatter.
func extractTemplateRelations(frontmatter map[string]interface{}) []TemplateRelation {
	raw, ok := frontmatter["_template_relations"]
	if !ok {
		return nil
	}

	// _template_relations should be a list of maps with "relation" and "target" keys
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	var relations []TemplateRelation
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rel := TemplateRelation{}
		if r, ok := m["relation"].(string); ok {
			rel.Relation = r
		}
		if t, ok := m["target"].(string); ok {
			rel.Target = t
		}
		if rel.Relation != "" && rel.Target != "" {
			relations = append(relations, rel)
		}
	}
	return relations
}

// loadTemplate reads and parses a template file.
// Returns ErrTemplateNotFound if the file does not exist.
func (f *FileIO) loadTemplate(path string) (*Document, error) {
	content, err := f.FS.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	doc, err := ParseDocument(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return doc, nil
}

// ApplyEntityTemplate merges template defaults with the entity.
// CLI-provided values (already set in entity) take precedence over template values.
// Template content is applied to entity.Content if entity.Content is empty.
func ApplyEntityTemplate(entity *model.Entity, template *Document) {
	if template == nil {
		return
	}

	// Apply template frontmatter as defaults (only for properties not already set)
	for key, value := range template.Frontmatter {
		// Skip id and type - these are managed by the create command
		if key == "id" || key == "type" {
			continue
		}

		// Only set if property is not already set
		if _, exists := entity.Properties[key]; !exists {
			entity.Properties[key] = value
		}
	}

	// Apply template content if entity has no content
	if entity.Content == "" && template.Content != "" {
		entity.Content = template.Content
	}
}

// ApplyRelationTemplate merges template defaults with the relation.
// CLI-provided values (already set in relation) take precedence over template values.
func ApplyRelationTemplate(relation *model.Relation, template *Document) {
	if template == nil {
		return
	}

	// Initialize properties map if nil
	if relation.Properties == nil {
		relation.Properties = make(map[string]interface{})
	}

	// Apply template frontmatter as defaults (only for properties not already set)
	for key, value := range template.Frontmatter {
		// Skip from, relation, to - these are managed by the link command
		if key == "from" || key == "relation" || key == "to" {
			continue
		}

		// Only set if property is not already set
		if _, exists := relation.Properties[key]; !exists {
			relation.Properties[key] = value
		}
	}
}

// GenerateEntityTemplate creates a template file for an entity type.
// If variant is non-empty, creates a variant template (e.g., type--variant.md).
// Returns true if the file was created, false if it already existed (and force is false).
func (f *FileIO) GenerateEntityTemplate(
	ctx *project.Context,
	meta *metamodel.Metamodel,
	entityType string,
	variant string,
	force bool,
) (bool, error) {
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return false, fmt.Errorf("unknown entity type: %s", entityType)
	}

	path := ctx.EntityTemplateVariantPath(entityType, variant)

	// Check if file exists
	if !force {
		if _, err := f.FS.Stat(path); err == nil {
			return false, nil // File exists, skip
		}
	}

	// Build frontmatter with all properties
	frontmatter := make(map[string]interface{})

	// Get sorted property names for deterministic output
	propNames := make([]string, 0, len(entityDef.Properties))
	for name := range entityDef.Properties {
		propNames = append(propNames, name)
	}
	natsort.Strings(propNames)

	// Add properties with defaults or empty values
	for _, name := range propNames {
		prop := entityDef.Properties[name]
		frontmatter[name] = getPropertyDefault(prop, meta)
	}

	// Generate placeholder content
	label := entityDef.Label
	if label == "" {
		label = entityType
	}
	content := fmt.Sprintf("# Description\n\nDescribe your %s here.\n", strings.ToLower(label))

	// Format document
	output, err := FormatDocument(frontmatter, content)
	if err != nil {
		return false, fmt.Errorf("failed to format template: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := f.FS.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := f.FS.WriteFile(path, []byte(output), 0644); err != nil {
		return false, fmt.Errorf("failed to write template: %w", err)
	}

	return true, nil
}

// getPropertyDefault returns the default value for a property based on its type.
func getPropertyDefault(prop metamodel.PropertyDef, meta *metamodel.Metamodel) interface{} {
	// Explicit default takes precedence
	if prop.Default != "" {
		return prop.Default
	}

	// Handle built-in types
	switch prop.Type {
	case metamodel.PropertyTypeBoolean:
		return false
	case metamodel.PropertyTypeInteger:
		return 0
	}

	// Inline enum values
	if len(prop.Values) > 0 {
		return prop.Values[0]
	}

	// Custom type - use default or first value
	if customType, ok := meta.Types[prop.Type]; ok {
		if customType.Default != "" {
			return customType.Default
		}
		if len(customType.Values) > 0 {
			return customType.Values[0]
		}
	}

	return ""
}

// GenerateRelationTemplate creates a template file for a relation type.
// Returns true if the file was created, false if it already existed (and force is false).
func (f *FileIO) GenerateRelationTemplate(
	ctx *project.Context,
	meta *metamodel.Metamodel,
	relationType string,
	force bool,
) (bool, error) {
	relDef, ok := meta.GetRelationDef(relationType)
	if !ok {
		return false, fmt.Errorf("unknown relation type: %s", relationType)
	}

	path := ctx.RelationTemplatePath(relationType)

	// Check if file exists
	if !force {
		if _, err := f.FS.Stat(path); err == nil {
			return false, nil // File exists, skip
		}
	}

	// Build frontmatter - relations typically don't have many properties
	// Just create an empty frontmatter for now
	frontmatter := make(map[string]interface{})

	// Generate placeholder content
	label := relDef.Label
	if label == "" {
		label = relationType
	}
	content := fmt.Sprintf("# Rationale\n\nExplain why this %s relation exists.\n", strings.ToLower(label))

	// Format document
	output, err := FormatDocument(frontmatter, content)
	if err != nil {
		return false, fmt.Errorf("failed to format template: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := f.FS.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := f.FS.WriteFile(path, []byte(output), 0644); err != nil {
		return false, fmt.Errorf("failed to write template: %w", err)
	}

	return true, nil
}
