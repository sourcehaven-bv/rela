package openapi

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// buildEntitySchema builds a JSON Schema for an entity type.
func (g *Generator) buildEntitySchema(typeName string, def metamodel.EntityDef) *Schema {
	props := make(map[string]*Schema)
	required := []string{"id", "type"}

	// Core fields
	props["id"] = &Schema{Type: "string", Description: "Unique entity identifier"}
	props["type"] = &Schema{Type: "string", Enum: []string{typeName}, Description: "Entity type"}
	props["content"] = &Schema{Type: "string", Description: "Markdown body content"}
	props["_self"] = &Schema{Type: "string", Format: "uri-reference", Description: "Self link"}

	// Properties object
	propSchema := g.buildPropertiesSchema(def)
	props["properties"] = propSchema

	// Relations map (relation type -> array of target IDs)
	props["relations"] = &Schema{
		Type: "object",
		AdditionalProperties: &Schema{
			Type:  "array",
			Items: StringSchema(),
		},
		Description: "Outgoing relations by type",
	}

	// Included entities map (for sideloading)
	props["included"] = &Schema{
		Type:                 "object",
		AdditionalProperties: Ref("Entity"),
		Description:          "Sideloaded related entities",
	}

	// Actions
	props["_actions"] = Ref("EntityActions")

	return &Schema{
		Type:        "object",
		Title:       def.Label,
		Description: def.Description,
		Properties:  props,
		Required:    required,
	}
}

// buildPropertiesSchema builds the properties object schema for an entity type.
func (g *Generator) buildPropertiesSchema(def metamodel.EntityDef) *Schema {
	props := make(map[string]*Schema)
	var required []string

	// Sort property names for deterministic output
	propNames := make([]string, 0, len(def.Properties))
	for name := range def.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, name := range propNames {
		propDef := def.Properties[name]
		props[name] = g.propertyToSchema(propDef)
		if propDef.Required {
			required = append(required, name)
		}
	}

	return &Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// propertyToSchema converts a metamodel property definition to a JSON Schema.
func (g *Generator) propertyToSchema(prop metamodel.PropertyDef) *Schema {
	var base *Schema

	switch prop.Type {
	case metamodel.PropertyTypeString:
		base = &Schema{Type: "string"}
	case metamodel.PropertyTypeDate:
		base = &Schema{Type: "string", Format: "date"}
	case metamodel.PropertyTypeInteger:
		base = &Schema{Type: "integer"}
	case metamodel.PropertyTypeBoolean:
		base = &Schema{Type: "boolean"}
	case metamodel.PropertyTypeFile:
		base = &Schema{Type: "string", Format: "uri-reference"}
	default:
		// Check if it's a custom enum type
		if ct, ok := g.meta.Types[prop.Type]; ok {
			base = &Schema{Type: "string", Enum: ct.Values}
			if ct.Default != "" {
				base.Default = ct.Default
			}
		} else {
			// Unknown type, default to string
			base = &Schema{Type: "string"}
		}
	}

	// Handle inline enum values (override custom type values)
	if len(prop.Values) > 0 {
		base.Enum = prop.Values
	}

	// Add description
	if prop.Description != "" {
		base.Description = prop.Description
	}

	// Add default
	if prop.Default != "" && base.Default == nil {
		base.Default = prop.Default
	}

	// Handle list properties (multi-select)
	if prop.List {
		return &Schema{
			Type:        "array",
			Items:       base,
			Description: base.Description,
		}
	}

	return base
}

// buildCreateEntitySchema builds the request body schema for creating an entity.
func (g *Generator) buildCreateEntitySchema(typeName string, def metamodel.EntityDef) *Schema {
	props := make(map[string]*Schema)

	// Optional ID (auto-generated if omitted)
	props["id"] = &Schema{
		Type:        "string",
		Description: "Custom entity ID (auto-generated if omitted)",
	}

	// Properties (the inner properties object already has its own Required array)
	propSchema := g.buildPropertiesSchema(def)
	props["properties"] = propSchema

	// Content
	props["content"] = &Schema{
		Type:        "string",
		Description: "Markdown body content",
	}

	// Properties object is required at the request level
	return &Schema{
		Type:        "object",
		Title:       "Create" + typeName,
		Description: "Request body for creating a " + def.Label,
		Properties:  props,
		Required:    []string{"properties"},
	}
}

// buildUpdateEntitySchema builds the request body schema for updating an entity.
//
// Wire format spans three concerns:
//   - properties / properties_unset / content: entity-level upsert
//     semantics (mirrors PATCH semantics for entity fields).
//   - relations: JSON:API §9-shaped — `{<type>: {data: [{type, id,
//     meta?, meta_unset?, content?}]}}`. Replacement at the list level;
//     upsert at the per-edge level. See the data-entry API reference
//     for the full contract (omit-vs-empty rules, propagation, etc.).
func (g *Generator) buildUpdateEntitySchema(typeName string, def metamodel.EntityDef) *Schema {
	props := make(map[string]*Schema)

	// Properties (partial update — upsert).
	propSchema := g.buildPropertiesSchema(def)
	props["properties"] = propSchema

	// properties_unset: explicit clear list.
	props["properties_unset"] = &Schema{
		Type:        "array",
		Items:       &Schema{Type: "string"},
		Description: "Property names to clear (delete the key). Must reference declared properties on this entity type.",
	}

	props["content"] = &Schema{
		Type:        "string",
		Description: "Markdown body content (upsert; absent leaves existing content alone).",
	}

	// relations: JSON:API §9-shaped per-relation-type wrapper.
	resourceIdentifier := &Schema{
		Type:        "object",
		Description: "JSON:API §5.2.1-shaped resource identifier with rela's per-edge upsert semantics.",
		Properties: map[string]*Schema{
			"type": {Type: "string", Description: "Target entity type (required)."},
			"id":   {Type: "string", Description: "Target entity ID (required)."},
			"meta": {
				Type:                 "object",
				AdditionalProperties: &Schema{},
				Description:          "Per-edge properties to merge into existing meta (upsert). Keys must be declared on the relation type.",
			},
			"meta_unset": {
				Type:        "array",
				Items:       &Schema{Type: "string"},
				Description: "Per-edge property keys to clear after the merge.",
			},
			"content": {
				Type:        "string",
				Description: "Per-edge markdown body. Only meaningful for relation types declared with content: true.",
			},
		},
		Required: []string{"type", "id"},
	}
	relationsUpdate := &Schema{
		Type:        "object",
		Description: "Wrapper for a single relation type's desired state. data field is required when this object appears.",
		Properties: map[string]*Schema{
			"data": {
				Type:        "array",
				Items:       resourceIdentifier,
				Description: "Full desired set of edges of this type. Empty array removes all; null is treated as empty.",
			},
		},
		Required: []string{"data"},
	}
	props["relations"] = &Schema{
		Type:                 "object",
		AdditionalProperties: relationsUpdate,
		Description: "Per-relation-type updates, keyed by relation type. Omitting a relation type leaves its edges untouched. " +
			"`data: []` removes all edges of that type. WARNING: clients should fetch entity state before constructing a PATCH body — " +
			"sending `data: []` from an unfetched form silently deletes all edges.",
	}

	return &Schema{
		Type:        "object",
		Title:       "Update" + typeName,
		Description: "Request body for updating a " + def.Label + " (PATCH - partial update). See the data-entry API reference for full semantics.",
		Properties:  props,
	}
}

// buildRelationRequestSchema builds the request body schema for creating a relation.
func (g *Generator) buildRelationRequestSchema() *Schema {
	return &Schema{
		Type:  "object",
		Title: "CreateRelation",
		Properties: map[string]*Schema{
			"id": {
				Type:        "string",
				Description: "Target entity ID",
			},
			"meta": {
				Type:                 "object",
				AdditionalProperties: StringSchema(),
				Description:          "Relation metadata properties",
			},
		},
		Required: []string{"id"},
	}
}

// addCommonSchemas adds shared schema definitions to the spec.
//
//nolint:funlen // Long function is acceptable for declarative schema definitions
func (g *Generator) addCommonSchemas(spec *Spec) {
	// Generic Entity (union type)
	spec.Components.Schemas["Entity"] = &Schema{
		Type:        "object",
		Description: "A rela entity",
		Properties: map[string]*Schema{
			"id":         {Type: "string"},
			"type":       {Type: "string"},
			"properties": {Type: "object", AdditionalProperties: &Schema{}},
			"content":    {Type: "string"},
			"relations": {
				Type:                 "object",
				AdditionalProperties: ArraySchema(StringSchema()),
			},
			"_self": {Type: "string", Format: "uri-reference"},
		},
		Required: []string{"id", "type"},
	}

	// List response wrapper
	spec.Components.Schemas["ListResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"data": ArraySchema(Ref("Entity")),
			"meta": Ref("ListMeta"),
		},
		Required: []string{"data", "meta"},
	}

	// List metadata
	spec.Components.Schemas["ListMeta"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"total":    IntegerSchema(),
			"page":     IntegerSchema(),
			"per_page": IntegerSchema(),
			"has_more": BooleanSchema(),
		},
		Required: []string{"total", "page", "per_page", "has_more"},
	}

	// Entity actions
	spec.Components.Schemas["EntityActions"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"delete": {
				Type: "object",
				Properties: map[string]*Schema{
					"allowed": BooleanSchema(),
					"reason":  StringSchema(),
				},
			},
			"transitions": ArraySchema(StringSchema()),
		},
	}

	// Error response (RFC 7807)
	spec.Components.Schemas["Error"] = &Schema{
		Type:        "object",
		Title:       "Problem Details",
		Description: "RFC 7807 Problem Details for HTTP APIs",
		Properties: map[string]*Schema{
			"type":     {Type: "string", Format: "uri", Description: "Error type URI"},
			"title":    {Type: "string", Description: "Short human-readable summary"},
			"status":   {Type: "integer", Description: "HTTP status code"},
			"detail":   {Type: "string", Description: "Detailed explanation"},
			"instance": {Type: "string", Description: "URI reference to the specific occurrence"},
			"errors":   ArraySchema(Ref("FieldError")),
		},
		Required: []string{"type", "title", "status"},
	}

	// Field error for validation errors
	spec.Components.Schemas["FieldError"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"source": {
				Type: "object",
				Properties: map[string]*Schema{
					"pointer": StringSchema(),
				},
			},
			"code":   StringSchema(),
			"detail": StringSchema(),
		},
	}

	// Schema response
	spec.Components.Schemas["Schema"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"entities":  {Type: "object", AdditionalProperties: Ref("EntityType")},
			"relations": {Type: "object", AdditionalProperties: Ref("RelationType")},
			"types":     {Type: "object", AdditionalProperties: Ref("CustomType")},
		},
	}

	spec.Components.Schemas["EntityType"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"label":       StringSchema(),
			"plural":      StringSchema(),
			"description": StringSchema(),
			"primary":     StringSchema(),
			"id_type":     StringSchema(),
			"id_prefix":   StringSchema(),
			"properties":  {Type: "object", AdditionalProperties: Ref("PropertyDef")},
		},
	}

	spec.Components.Schemas["PropertyDef"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"type":        StringSchema(),
			"required":    BooleanSchema(),
			"default":     StringSchema(),
			"values":      ArraySchema(StringSchema()),
			"description": StringSchema(),
			"list":        BooleanSchema(),
		},
	}

	spec.Components.Schemas["RelationType"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"label":        StringSchema(),
			"description":  StringSchema(),
			"from":         ArraySchema(StringSchema()),
			"to":           ArraySchema(StringSchema()),
			"min_outgoing": IntegerSchema(),
			"max_outgoing": IntegerSchema(),
			"min_incoming": IntegerSchema(),
			"max_incoming": IntegerSchema(),
		},
	}

	spec.Components.Schemas["CustomType"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"values":  ArraySchema(StringSchema()),
			"default": StringSchema(),
		},
	}

	// Config response
	spec.Components.Schemas["Config"] = &Schema{
		Type:        "object",
		Description: "UI configuration",
		Properties: map[string]*Schema{
			"app":        {Type: "object"},
			"forms":      {Type: "object"},
			"lists":      {Type: "object"},
			"views":      {Type: "object"},
			"kanbans":    {Type: "object"},
			"dashboard":  {Type: "object"},
			"navigation": {Type: "array"},
			"documents":  {Type: "object"},
		},
	}

	// Search response
	spec.Components.Schemas["SearchResponse"] = Ref("ListResponse")

	// Analysis response
	spec.Components.Schemas["AnalysisResult"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"errors":   IntegerSchema(),
			"warnings": IntegerSchema(),
			"issues":   ArraySchema(Ref("AnalysisIssue")),
			"by_check": {Type: "object", AdditionalProperties: IntegerSchema()},
		},
	}

	spec.Components.Schemas["AnalysisIssue"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"entity_id":   StringSchema(),
			"entity_type": StringSchema(),
			"message":     StringSchema(),
			"severity":    StringSchema(),
			"check_type":  StringSchema(),
		},
	}

	// Create relation request
	spec.Components.Schemas["CreateRelationRequest"] = g.buildRelationRequestSchema()
}
