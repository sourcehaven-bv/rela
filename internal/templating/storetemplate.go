// Package templating provides a Templater service for entity templates.
//
// Templates are read-only defaults used during entity creation. They live
// outside the store (typically as files in a templates/ directory). This
// package defines the service interface and a Template type that doesn't
// depend on the model package.
package templating

import "context"

// Relation is a pre-filled relation in a template.
type Relation struct {
	Type   string
	Target string
}

// Template represents an entity template with optional variant name.
type Template struct {
	Name       string                 // "" for default, e.g. "epic" for a variant
	EntityType string                 // the entity type this template applies to
	Properties map[string]interface{} // property defaults
	Content    string                 // markdown body
	Relations  []Relation             // pre-filled relations
}

// Templater provides read-only access to entity templates.
type Templater interface {
	// EntityTemplates returns all templates (default + variants) for a type.
	EntityTemplates(ctx context.Context, entityType string) ([]*Template, error)

	// EntityTemplate returns a specific named variant. variant="" returns the default.
	// Returns nil if no template exists.
	EntityTemplate(ctx context.Context, entityType, variant string) (*Template, error)
}
