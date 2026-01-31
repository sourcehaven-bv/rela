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
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// ErrTemplateNotFound is returned when a template file does not exist.
var ErrTemplateNotFound = fmt.Errorf("template not found")

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
// Returns true if the file was created, false if it already existed (and force is false).
func (f *FileIO) GenerateEntityTemplate(
	ctx *project.Context,
	meta *metamodel.Metamodel,
	entityType string,
	force bool,
) (bool, error) {
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return false, fmt.Errorf("unknown entity type: %s", entityType)
	}

	path := ctx.EntityTemplatePath(entityType)

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
	sort.Strings(propNames)

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
