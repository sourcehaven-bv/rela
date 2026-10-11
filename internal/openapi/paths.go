package openapi

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// addSystemPaths adds the static system endpoint paths.
func (g *Generator) addSystemPaths(spec *Spec) {
	// GET /api/v1/_openapi.json
	spec.Paths["/api/v1/_openapi.json"] = PathItem{
		Get: &Operation{
			OperationID: "getOpenAPISpec",
			Summary:     "Get OpenAPI specification",
			Description: "Returns this OpenAPI 3.1 specification",
			Tags:        []string{"System"},
			Responses: map[string]Response{
				"200": {
					Description: "OpenAPI specification",
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{Type: "object"}},
					},
				},
			},
		},
	}

	// The SSE stream and the git routes are left out: they are browser
	// surfaces outside the non-browser CSRF exemption, so a generic client
	// would get 403 origin_missing from every call.
}

// addEntityPaths adds CRUD paths for an entity type.
//
//nolint:funlen // Long function is acceptable for declarative spec building
func (g *Generator) addEntityPaths(spec *Spec, typeName string, def metamodel.EntityDef) {
	plural := def.GetPlural(typeName)
	tag := def.Label
	basePath := "/api/v1/" + plural

	// Collection path: GET (list) and POST (create)
	spec.Paths[basePath] = PathItem{
		Summary: fmt.Sprintf("Collection of %s entities", def.Label),
		Get: &Operation{
			OperationID: "list" + capitalize(plural),
			Summary:     "List " + def.GetLabelPlural(),
			Description: def.Description,
			Tags:        []string{tag},
			Parameters:  g.listParameters(),
			Responses: map[string]Response{
				"200": {
					Description: "List of " + def.GetLabelPlural(),
					Content:     jsonContent(Ref("ListResponse")),
					Headers: map[string]Header{
						"X-Total-Count": {Description: "Total number of entities", Schema: IntegerSchema()},
						"X-Page":        {Description: "Current page number", Schema: IntegerSchema()},
						"X-Per-Page":    {Description: "Items per page", Schema: IntegerSchema()},
						"Link":          {Description: "Pagination links (RFC 5988)", Schema: StringSchema()},
					},
				},
			},
		},
		Post: &Operation{
			OperationID: "create" + capitalize(typeName),
			Summary:     "Create a " + def.Label,
			Tags:        []string{tag},
			RequestBody: &RequestBody{
				Required: true,
				Content:  jsonContent(g.buildCreateEntitySchema(typeName, def)),
			},
			Responses: map[string]Response{
				"201": {
					Description: def.Label + " created",
					Content:     jsonContent(g.buildEntitySchema(typeName, def)),
					Headers: map[string]Header{
						"Location": {Description: "URL of created entity", Schema: StringSchema()},
					},
				},
				"422": {Description: "Validation failed", Content: problemContent()},
			},
		},
	}

	// Single entity path: GET, PATCH, DELETE
	entityPath := basePath + "/{id}"
	spec.Paths[entityPath] = PathItem{
		Summary: fmt.Sprintf("Single %s entity", def.Label),
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID", Schema: StringSchema()},
		},
		Get: &Operation{
			OperationID: "get" + capitalize(typeName),
			Summary:     "Get a " + def.Label,
			Tags:        []string{tag},
			Parameters: []Parameter{
				{Name: "include", In: "query", Description: "Comma-separated relation types to include", Schema: StringSchema()},
			},
			Responses: map[string]Response{
				"200": {
					Description: def.Label + " details",
					Content:     jsonContent(g.buildEntitySchema(typeName, def)),
					Headers: map[string]Header{
						"ETag": {Description: "Entity version tag", Schema: StringSchema()},
					},
				},
				"304": {Description: "Not modified (ETag match)"},
				"404": {Description: "Entity not found", Content: problemContent()},
			},
		},
		Patch: &Operation{
			OperationID: "update" + capitalize(typeName),
			Summary:     "Update a " + def.Label,
			Tags:        []string{tag},
			Parameters: []Parameter{
				{Name: "If-Match", In: "header", Description: "ETag for optimistic locking", Schema: StringSchema()},
			},
			RequestBody: &RequestBody{
				Required: true,
				Content:  jsonContent(g.buildUpdateEntitySchema(typeName, def)),
			},
			Responses: map[string]Response{
				"200": {
					Description: def.Label + " updated",
					Content:     jsonContent(g.buildEntitySchema(typeName, def)),
					Headers: map[string]Header{
						"ETag": {Description: "New entity version tag", Schema: StringSchema()},
					},
				},
				"404": {Description: "Entity not found", Content: problemContent()},
				"412": {Description: "Precondition failed (ETag mismatch)", Content: problemContent()},
				"422": {Description: "Validation failed", Content: problemContent()},
			},
		},
		Delete: &Operation{
			OperationID: "delete" + capitalize(typeName),
			Summary:     "Delete a " + def.Label,
			Tags:        []string{tag},
			Responses: map[string]Response{
				"204": {Description: "Entity deleted"},
				"404": {Description: "Entity not found", Content: problemContent()},
				"409": {Description: "Cannot delete (has incoming relations)", Content: problemContent()},
			},
		},
	}

	// Relations path
	relationsPath := basePath + "/{id}/relations"
	spec.Paths[relationsPath] = PathItem{
		Summary: "Relations for a " + def.Label,
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID", Schema: StringSchema()},
		},
		Get: &Operation{
			OperationID: fmt.Sprintf("get%sRelations", capitalize(typeName)),
			Summary:     "Get all relations for a " + def.Label,
			Tags:        []string{tag},
			Responses: map[string]Response{
				"200": {
					Description: "Relations grouped by type",
					Content: jsonContent(&Schema{
						Type:                 "object",
						AdditionalProperties: ArraySchema(&Schema{Type: "object"}),
					}),
				},
				"404": {Description: "Entity not found", Content: problemContent()},
			},
		},
	}

	// Add relation type paths for valid outgoing relations
	g.addRelationTypePaths(spec, typeName, def, basePath)

	g.addAttachmentPaths(spec, typeName, def, basePath)

	// Clone action
	clonePath := basePath + "/{id}/_actions/clone"
	spec.Paths[clonePath] = PathItem{
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID", Schema: StringSchema()},
		},
		Post: &Operation{
			OperationID: "clone" + capitalize(typeName),
			Summary:     "Clone a " + def.Label,
			Description: "Creates a copy of the entity with a new ID",
			Tags:        []string{tag},
			Responses: map[string]Response{
				"201": {
					Description: "Cloned " + def.Label,
					Content:     jsonContent(g.buildEntitySchema(typeName, def)),
					Headers: map[string]Header{
						"Location": {Description: "URL of cloned entity", Schema: StringSchema()},
					},
				},
				"404": {Description: "Entity not found", Content: problemContent()},
			},
		},
	}
}

// addRelationTypePaths adds paths for specific relation types valid for an entity.
func (g *Generator) addRelationTypePaths(spec *Spec, typeName string, def metamodel.EntityDef, basePath string) {
	// Find all relations where this entity type can be the source
	validRelations := make(map[string]metamodel.RelationDef)
	for relName, relDef := range g.meta.Relations {
		if slices.Contains(relDef.From, typeName) {
			validRelations[relName] = relDef
		}
	}

	// Sort for deterministic output
	relNames := make([]string, 0, len(validRelations))
	for name := range validRelations {
		relNames = append(relNames, name)
	}
	sort.Strings(relNames)

	for _, relName := range relNames {
		relDef := validRelations[relName]
		relPath := fmt.Sprintf("%s/{id}/relations/%s", basePath, relName)

		// Build description from metamodel + target constraint
		pathDesc := relDef.Description
		targetConstraint := "Target must be one of: " + strings.Join(relDef.To, ", ")
		if pathDesc == "" {
			pathDesc = targetConstraint
		} else {
			pathDesc = pathDesc + ". " + targetConstraint
		}

		spec.Paths[relPath] = PathItem{
			Summary:     fmt.Sprintf("%s relations for %s", relDef.Label, def.Label),
			Description: relDef.Description,
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "Source entity ID", Schema: StringSchema()},
			},
			Get: &Operation{
				OperationID: fmt.Sprintf("get%s%sRelations", capitalize(typeName), capitalize(relName)),
				Summary:     fmt.Sprintf("List %s relations", relDef.Label),
				Description: relDef.Description,
				Tags:        []string{def.Label, "Relations"},
				Responses: map[string]Response{
					"200": {
						Description: "List of related entity IDs",
						Content: jsonContent(ArraySchema(&Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"id":   StringSchema(),
								"meta": {Type: "object"},
							},
						})),
					},
					"404": {Description: "Entity not found", Content: problemContent()},
				},
			},
			Post: &Operation{
				OperationID: fmt.Sprintf("create%s%sRelation", capitalize(typeName), capitalize(relName)),
				Summary:     fmt.Sprintf("Create %s relation", relDef.Label),
				Description: pathDesc,
				Tags:        []string{def.Label, "Relations"},
				RequestBody: &RequestBody{
					Required: true,
					Content:  jsonContent(Ref("CreateRelationRequest")),
				},
				Responses: map[string]Response{
					"201": {Description: "Relation created"},
					"404": {Description: "Source or target entity not found", Content: problemContent()},
					"422": {Description: "Invalid relation", Content: problemContent()},
				},
			},
		}

		// Delete specific relation
		deleteRelPath := relPath + "/{targetId}"
		spec.Paths[deleteRelPath] = PathItem{
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "Source entity ID", Schema: StringSchema()},
				{Name: "targetId", In: "path", Required: true, Description: "Target entity ID", Schema: StringSchema()},
			},
			Delete: &Operation{
				OperationID: fmt.Sprintf("delete%s%sRelation", capitalize(typeName), capitalize(relName)),
				Summary:     fmt.Sprintf("Delete %s relation", relDef.Label),
				Tags:        []string{def.Label, "Relations"},
				Responses: map[string]Response{
					"204": {Description: "Relation deleted"},
					"404": {Description: "Relation not found", Content: problemContent()},
				},
			},
		}
	}
}

// listParameters returns common query parameters for list endpoints.
func (g *Generator) listParameters() []Parameter {
	return []Parameter{
		{Name: "page", In: "query", Description: "Page number (default: 1)", Schema: IntegerSchema()},
		{Name: "per_page", In: "query", Description: "Items per page (default: 25, max: 100)", Schema: IntegerSchema()},
		{Name: "sort", In: "query", Description: "Sort fields (comma-separated, prefix with - for descending)", Schema: StringSchema()},
		{Name: "filter[property]", In: "query", Description: "Filter by property value (e.g., filter[status]=active)", Schema: StringSchema()},
		{Name: "filter[property][operator]", In: "query", Description: "Filter with operator: eq, ne, contains, in", Schema: StringSchema()},
		{Name: "include_content", In: "query", Description: "Include each row's markdown body (default: false; collection rows are content-free)", Schema: BooleanSchema()},
		relationContextParam, directionContextParam,
	}
}

// relationContextParam and directionContextParam are the relation context of
// a collection read: with both, every row carries `linkable`.
var (
	relationContextParam = Parameter{
		Name: "relation", In: "query", Schema: StringSchema(),
		Description: "Canonical relation type; with direction=incoming, each row carries " +
			"`linkable`: whether the caller may create that relation from the row's face",
	}
	directionContextParam = Parameter{
		Name: "direction", In: "query", Schema: StringSchema(),
		Description: "Required with relation; only `incoming` is accepted",
	}
)

// problemContent is the body of an error response from a handler: RFC 7807
// problem details, served as application/problem+json. (A refusal by the
// same-origin middleware, before any handler, is plain JSON.)
func problemContent() map[string]MediaType {
	return map[string]MediaType{
		"application/problem+json": {Schema: Ref("Error")},
	}
}

// jsonContent creates a JSON media type map with the given schema.
func jsonContent(schema *Schema) map[string]MediaType {
	return map[string]MediaType{
		"application/json": {Schema: schema},
	}
}

// capitalize returns the string with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
