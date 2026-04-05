// Package openapi provides OpenAPI 3.1 spec generation from rela metamodels.
package openapi

// Spec represents an OpenAPI 3.1 specification.
type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths"`
	Components *Components         `json:"components,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// Server represents an API server.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Components holds reusable schema definitions.
type Components struct {
	Schemas map[string]*Schema `json:"schemas,omitempty"`
}

// PathItem describes operations available on a single path.
type PathItem struct {
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Get         *Operation  `json:"get,omitempty"`
	Post        *Operation  `json:"post,omitempty"`
	Patch       *Operation  `json:"patch,omitempty"`
	Delete      *Operation  `json:"delete,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	OperationID string              `json:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // "path", "query", "header", "cookie"
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody describes a single request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
}

// MediaType provides schema and examples for a media type.
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Header describes a single header.
type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// Schema represents a JSON Schema (OpenAPI 3.1 uses JSON Schema draft 2020-12).
type Schema struct {
	// Reference to another schema
	Ref string `json:"$ref,omitempty"`

	// Type information
	Type   string   `json:"type,omitempty"`
	Format string   `json:"format,omitempty"`
	Enum   []string `json:"enum,omitempty"`

	// Object properties
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`

	// Array items
	Items *Schema `json:"items,omitempty"`

	// Composition
	AllOf []*Schema `json:"allOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`

	// Metadata
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`

	// Numeric constraints
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
}

// Ref creates a schema reference to a component.
func Ref(name string) *Schema {
	return &Schema{Ref: "#/components/schemas/" + name}
}

// StringSchema returns a basic string schema.
func StringSchema() *Schema {
	return &Schema{Type: "string"}
}

// IntegerSchema returns a basic integer schema.
func IntegerSchema() *Schema {
	return &Schema{Type: "integer"}
}

// BooleanSchema returns a basic boolean schema.
func BooleanSchema() *Schema {
	return &Schema{Type: "boolean"}
}

// ArraySchema returns an array schema with the given item schema.
func ArraySchema(items *Schema) *Schema {
	return &Schema{Type: "array", Items: items}
}

// ObjectSchema returns an object schema with the given properties.
func ObjectSchema(props map[string]*Schema, required []string) *Schema {
	return &Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}
