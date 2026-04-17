// Package templating provides a Templater service for entity and relation
// templates.
//
// Templates are read-mostly defaults used during entity and relation
// creation. They live outside the store (typically as files in a
// templates/ directory) and are a pluggable service so backends can swap
// (local filesystem, remote source, database) without touching call sites.
package templating

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Relation is a pre-filled relation in a template.
type Relation struct {
	Type   string
	Target string
}

// Template represents an entity or relation template.
//
// Name is "" for the default template; for entity templates it can hold a
// variant name (e.g. "epic" for a <type>--epic.md variant). Relation
// templates don't have variants; Name is always "" for them.
type Template struct {
	Name       string                 // "" for default, e.g. "epic" for a variant
	EntityType string                 // the entity type this template applies to (empty for relation templates)
	Properties map[string]interface{} // property defaults
	Content    string                 // markdown body
	Relations  []Relation             // pre-filled relations (entity templates only)
}

// Templater provides access to entity and relation templates.
type Templater interface {
	// EntityTemplate returns a specific named variant. variant="" returns the default.
	// Returns nil if no template exists.
	EntityTemplate(ctx context.Context, entityType, variant string) (*Template, error)

	// EntityTemplates returns all templates (default + variants) for a type.
	EntityTemplates(ctx context.Context, entityType string) ([]*Template, error)

	// RelationTemplate returns the template for a relation type.
	// Returns nil if no template exists.
	RelationTemplate(ctx context.Context, relationType string) (*Template, error)

	// GenerateEntity writes a template file for the given entity type and
	// optional variant name. Returns (created=true) when a new file was
	// written, (created=false) when skipped because one existed and force
	// is false.
	GenerateEntity(ctx context.Context, meta *metamodel.Metamodel, entityType, variant string, force bool) (bool, error)

	// GenerateRelation writes a template file for the given relation type.
	GenerateRelation(ctx context.Context, meta *metamodel.Metamodel, relationType string, force bool) (bool, error)
}

// ApplyEntity merges template defaults into the target property map and
// content. Pass the entity's current properties and content; the function
// fills in defaults for keys that are not already present, and sets
// content only when content is currently empty.
//
// Returns the (possibly updated) properties map and content. The caller
// should write both back onto its entity.
func ApplyEntity(
	properties map[string]interface{}, content string, t *Template,
) (mergedProperties map[string]interface{}, mergedContent string) {
	if t == nil {
		return properties, content
	}
	if properties == nil {
		properties = make(map[string]interface{})
	}
	for k, v := range t.Properties {
		if _, exists := properties[k]; !exists {
			properties[k] = v
		}
	}
	if content == "" && t.Content != "" {
		content = t.Content
	}
	return properties, content
}

// ApplyRelation merges relation template defaults into the target property
// map. Content is intentionally not propagated — the relation-creation
// path does not carry a body through templates today.
func ApplyRelation(properties map[string]interface{}, t *Template) map[string]interface{} {
	if t == nil {
		return properties
	}
	if properties == nil {
		properties = make(map[string]interface{})
	}
	for k, v := range t.Properties {
		if _, exists := properties[k]; !exists {
			properties[k] = v
		}
	}
	return properties
}
