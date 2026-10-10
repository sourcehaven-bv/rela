package openapi

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// addAttachmentPaths adds the upload, download and delete operations for a
// type's `file` properties. A type without one gets none.
//
// The upload accepts the file as the raw request body as well as a multipart
// form, so a generic client can send a file without building an envelope:
// restish turns an application/octet-stream body into `@file` input.
func (g *Generator) addAttachmentPaths(spec *Spec, typeName string, def metamodel.EntityDef, basePath string) {
	fileProps := fileProperties(def)
	if len(fileProps) == 0 {
		return
	}
	tag := def.Label
	idParam := Parameter{Name: "id", In: "path", Required: true, Description: "Entity ID", Schema: StringSchema()}
	propParam := Parameter{
		Name: "property", In: "path", Required: true, Description: "The file property",
		Schema: &Schema{Type: "string", Enum: fileProps},
	}
	binary := &Schema{Type: "string", Format: "binary"}

	spec.Paths[basePath+"/{id}/_attachments/{property}"] = PathItem{
		Parameters: []Parameter{idParam, propParam},
		Put: &Operation{
			OperationID: "put" + capitalize(typeName) + "Attachment",
			Summary:     "Upload a file to a " + def.Label,
			Description: "Attaches a file to a file property, appending up to the property's `max` " +
				"(replacing the file when `max` is 1). Send the file as the raw body with `filename`, " +
				"or as a multipart form with the file in field `file`. The upload policy of the " +
				"property (MIME allowlist, scan, size limit) applies.",
			Tags: []string{tag},
			Parameters: []Parameter{{
				Name: "filename", In: "query",
				Description: "The file's name; required with a raw body, ignored with a multipart form",
				Schema:      StringSchema(),
			}},
			RequestBody: &RequestBody{
				Required: true,
				Content: map[string]MediaType{
					"application/octet-stream": {Schema: binary},
					"multipart/form-data": {
						Schema: ObjectSchema(map[string]*Schema{"file": binary}, []string{"file"}),
					},
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "File attached; the updated " + def.Label,
					Content:     jsonContent(g.buildEntitySchema(typeName, def)),
				},
				"400": {Description: "No file, or no file name with a raw body", Content: problemContent()},
				"403": {Description: "Not allowed to update the entity", Content: problemContent()},
				"404": {Description: "Entity or file property not found", Content: problemContent()},
				"409": {Description: "The property already holds its maximum number of files", Content: problemContent()},
				"413": {Description: "File exceeds the size limit", Content: problemContent()},
				"415": {Description: "Raw body sent with a Content-Encoding", Content: problemContent()},
				"422": {Description: "File rejected by the upload policy", Content: problemContent()},
				"503": {Description: "Upload capacity reached; retry later", Content: problemContent()},
			},
		},
	}

	spec.Paths[basePath+"/{id}/_attachments/{property}/{fileName}"] = PathItem{
		Parameters: []Parameter{
			idParam, propParam,
			{Name: "fileName", In: "path", Required: true, Description: "The attached file's name", Schema: StringSchema()},
		},
		Get: &Operation{
			OperationID: "get" + capitalize(typeName) + "Attachment",
			Summary:     "Download a file attached to a " + def.Label,
			Tags:        []string{tag},
			Responses: map[string]Response{
				"200": {
					Description: "The file's bytes, with a Content-Type inferred from its name",
					Content:     map[string]MediaType{"application/octet-stream": {Schema: binary}},
				},
				"404": {Description: "Entity, property or file not found", Content: problemContent()},
			},
		},
		Delete: &Operation{
			OperationID: "delete" + capitalize(typeName) + "Attachment",
			Summary:     "Remove a file from a " + def.Label,
			Tags:        []string{tag},
			Responses: map[string]Response{
				"204": {Description: "File removed (also when it was not attached)"},
				"403": {Description: "Not allowed to update the entity", Content: problemContent()},
				"404": {Description: "Entity or file property not found", Content: problemContent()},
			},
		},
	}
}

// fileProperties returns the names of def's `file` properties, sorted.
func fileProperties(def metamodel.EntityDef) []string {
	var names []string
	for name, prop := range def.Properties {
		if prop.Type == metamodel.PropertyTypeFile {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
