package openapi

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// leaseSchema describes `_lease`: the advisory lease on an entity, on the
// single-entity GET and list rows only.
func leaseSchema() *Schema {
	return &Schema{
		Type:        "object",
		Description: "The advisory lease on this entity; absent when nobody holds it",
		Properties: map[string]*Schema{
			"user": StringSchema(), "tool": StringSchema(), "label": StringSchema(),
			"since": {Type: "string", Format: "date-time"}, "status": StringSchema(),
		},
	}
}

// addLeasePath adds the claim operation for a type. Renew, release and the
// caller's lease list live under /api/v1/_leases, a browser surface outside
// this spec: a client without a browser origin cannot call it.
func (g *Generator) addLeasePath(spec *Spec, typeName string, def metamodel.EntityDef, basePath string) {
	spec.Paths[basePath+"/{id}/_lease"] = PathItem{
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID, or ID@face", Schema: StringSchema()},
		},
		Post: &Operation{
			OperationID: "claim" + capitalize(typeName),
			Summary:     "Claim an advisory lease on a " + def.Label,
			Description: "Tells people and agents you are working on the entity. Needs read and update " +
				"access. Leases do not block writes. Keep lease_id: it renews and releases the lease " +
				"and is returned only here.",
			Tags: []string{def.Label},
			RequestBody: &RequestBody{
				Content: jsonContent(ObjectSchema(map[string]*Schema{
					"ttl_seconds": {Type: "integer", Description: "Lease duration, 30 to 3600 (default 900)"},
					"label":       {Type: "string", Description: "Tells apart agents running as the same user"},
					"status":      {Type: "string", Description: "Short note on what you are doing"},
				}, nil)),
			},
			Responses: map[string]Response{
				"201": {
					Description: "Lease claimed",
					Content: jsonContent(ObjectSchema(map[string]*Schema{
						"lease_id":   StringSchema(),
						"token":      IntegerSchema(),
						"expires_at": {Type: "string", Format: "date-time"},
					}, []string{"lease_id", "token", "expires_at"})),
				},
				"400": {Description: "Invalid JSON or input", Content: problemContent()},
				"403": {Description: "Not allowed to update the entity", Content: problemContent()},
				"404": {Description: "Entity not found", Content: problemContent()},
				"409": {Description: "The entity is held, or you hold too many leases", Content: problemContent()},
				"422": {Description: "A bare id of a faced type; address one face", Content: problemContent()},
				"503": {Description: "The lease store is full; retry later", Content: problemContent()},
			},
		},
	}
}
