// Package templating: fsloader.go contains the filesystem primitives
// used by FSTemplater to read, discover, and generate entity/relation
// template files. It is the former markdown.FileIO template methods,
// lifted into this package so FSTemplater owns its own I/O.
package templating

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ErrTemplateNotFound is returned when a template file does not exist.
var ErrTemplateNotFound = fmt.Errorf("template not found")

// loadEntityTemplateDoc reads an entity template file and returns the parsed document.
// Returns (nil, nil) if the template file does not exist.
func loadEntityTemplateDoc(fs storage.FS, templatePath string) (*markdown.Document, error) {
	doc, err := loadTemplateDoc(fs, templatePath)
	if errors.Is(err, ErrTemplateNotFound) {
		return nil, nil //nolint:nilnil // nil,nil is intentional when template doesn't exist
	}
	return doc, err
}

// loadRelationTemplateDoc reads a relation template file and returns the parsed document.
// Returns (nil, nil) if the template file does not exist.
func loadRelationTemplateDoc(fs storage.FS, templatePath string) (*markdown.Document, error) {
	doc, err := loadTemplateDoc(fs, templatePath)
	if errors.Is(err, ErrTemplateNotFound) {
		return nil, nil //nolint:nilnil // nil,nil is intentional when template doesn't exist
	}
	return doc, err
}

// discoverEntityTemplates returns every template for an entity type,
// including variants. Templates are discovered by file name:
//   - <entityType>.md         → default template
//   - <entityType>--<variant>.md → variant template
//
// Returns templates sorted by name (default first, then alphabetically)
// and an empty slice (not an error) when the templates directory is
// missing.
func discoverEntityTemplates(
	fs storage.FS, templatesDir, entityType string,
) ([]*Template, error) {
	if _, err := fs.Stat(templatesDir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := fs.ReadDir(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	templates := make([]*Template, 0, len(entries))
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
			variantName = strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".md")
		default:
			continue
		}

		path := filepath.Join(templatesDir, name)
		tmpl, err := loadEntityTemplate(fs, path, entityType, variantName)
		if err != nil {
			return nil, fmt.Errorf("failed to load template %s: %w", name, err)
		}
		templates = append(templates, tmpl)
	}

	// Default first, then alphabetically by variant name.
	sort.Slice(templates, func(i, j int) bool {
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

// loadEntityTemplate loads a single entity template file and parses it
// into a Template.
func loadEntityTemplate(
	fs storage.FS, path, entityType, variantName string,
) (*Template, error) {
	content, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	doc, err := markdown.ParseDocument(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	relations := extractTemplateRelations(doc.Frontmatter)
	properties := make(map[string]interface{})
	for k, v := range doc.Frontmatter {
		if k != "_template_relations" {
			properties[k] = v
		}
	}
	return &Template{
		Name:       variantName,
		EntityType: entityType,
		Properties: properties,
		Content:    doc.Content,
		Relations:  relations,
	}, nil
}

// extractTemplateRelations parses the _template_relations field from
// frontmatter into a slice of Relation.
func extractTemplateRelations(frontmatter map[string]interface{}) []Relation {
	raw, ok := frontmatter["_template_relations"]
	if !ok {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var relations []Relation
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rel := Relation{}
		if r, ok := m["relation"].(string); ok {
			rel.Type = r
		}
		if t, ok := m["target"].(string); ok {
			rel.Target = t
		}
		if rel.Type != "" && rel.Target != "" {
			relations = append(relations, rel)
		}
	}
	return relations
}

// loadTemplateDoc reads and parses a template file. Returns
// ErrTemplateNotFound if the file does not exist.
func loadTemplateDoc(fs storage.FS, path string) (*markdown.Document, error) {
	content, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	doc, err := markdown.ParseDocument(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return doc, nil
}

// generateEntityTemplate creates a template file for an entity type.
// Returns true if the file was created, false if it already existed
// (and force is false).
func generateEntityTemplate(
	fs storage.FS,
	templatePath string,
	meta *metamodel.Metamodel,
	entityType string,
	force bool,
) (bool, error) {
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return false, fmt.Errorf("unknown entity type: %s", entityType)
	}
	if !force {
		if _, err := fs.Stat(templatePath); err == nil {
			return false, nil
		}
	}

	frontmatter := make(map[string]interface{})
	propNames := make([]string, 0, len(entityDef.Properties))
	for name := range entityDef.Properties {
		propNames = append(propNames, name)
	}
	natsort.Strings(propNames)
	for _, name := range propNames {
		prop := entityDef.Properties[name]
		frontmatter[name] = propertyDefault(prop, meta)
	}

	label := entityDef.Label
	if label == "" {
		label = entityType
	}
	content := fmt.Sprintf("# Description\n\nDescribe your %s here.\n", strings.ToLower(label))

	output, err := markdown.FormatDocument(frontmatter, content)
	if err != nil {
		return false, fmt.Errorf("failed to format template: %w", err)
	}

	dir := filepath.Dir(templatePath)
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}
	if err := fs.WriteFile(templatePath, []byte(output), 0644); err != nil {
		return false, fmt.Errorf("failed to write template: %w", err)
	}
	return true, nil
}

// generateRelationTemplate creates a template file for a relation type.
// Returns true if the file was created, false if it already existed
// (and force is false).
func generateRelationTemplate(
	fs storage.FS,
	templatePath string,
	meta *metamodel.Metamodel,
	relationType string,
	force bool,
) (bool, error) {
	relDef, ok := meta.GetRelationDef(relationType)
	if !ok {
		return false, fmt.Errorf("unknown relation type: %s", relationType)
	}
	if !force {
		if _, err := fs.Stat(templatePath); err == nil {
			return false, nil
		}
	}

	label := relDef.Label
	if label == "" {
		label = relationType
	}
	content := fmt.Sprintf("# Rationale\n\nExplain why this %s relation exists.\n", strings.ToLower(label))

	output, err := markdown.FormatDocument(map[string]interface{}{}, content)
	if err != nil {
		return false, fmt.Errorf("failed to format template: %w", err)
	}

	dir := filepath.Dir(templatePath)
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}
	if err := fs.WriteFile(templatePath, []byte(output), 0644); err != nil {
		return false, fmt.Errorf("failed to write template: %w", err)
	}
	return true, nil
}

// propertyDefault returns the default value for a property based on
// its type.
func propertyDefault(prop metamodel.PropertyDef, meta *metamodel.Metamodel) interface{} {
	if prop.Default != "" {
		return prop.Default
	}
	switch prop.Type {
	case metamodel.PropertyTypeBoolean:
		return false
	case metamodel.PropertyTypeInteger:
		return 0
	}
	if len(prop.Values) > 0 {
		return prop.Values[0]
	}
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
