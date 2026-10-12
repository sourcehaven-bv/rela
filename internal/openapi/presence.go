package openapi

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// addPresencePath adds the presence read for a type (TKT-MHXH87).
func (g *Generator) addPresencePath(spec *Spec, typeName string, def metamodel.EntityDef, basePath string) {
	spec.Paths[basePath+"/{id}/_presence"] = PathItem{
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID, or ID@face", Schema: StringSchema()},
			{Name: "world", In: "query", Description: "World that picks the face", Schema: StringSchema()},
		},
		Get: &Operation{
			OperationID: "getPresence" + capitalize(typeName),
			Summary:     "List who else has a " + def.Label + " open",
			Description: "The other people with a live subscription to this path, oldest first, at most 20. " +
				"A viewer stays listed for 15 seconds after they close it. You are never listed, from any " +
				"tool. This GET does not add you; subscribe to the path on the live event stream for that.",
			Tags: []string{def.Label},
			Responses: map[string]Response{
				"200": {
					Description: "The viewers",
					Content: jsonContent(ObjectSchema(map[string]*Schema{
						"viewers": ArraySchema(ObjectSchema(map[string]*Schema{
							"user": StringSchema(), "tool": StringSchema(), "since": dateTimeSchema(),
						}, []string{"user", "tool", "since"})),
						"more": {Type: "integer", Description: "Viewers past the 20 listed"},
					}, []string{"viewers", "more"})),
				},
				"404": {Description: "Entity not found", Content: problemContent()},
				"405": {Description: "Method not allowed", Content: problemContent()},
			},
		},
	}
}
