package model

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
