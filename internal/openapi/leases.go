package openapi

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const leasesTag = "Leases"

// leaseSchema describes `_lease`: the advisory lease on an entity, on the
// single-entity GET and list rows only.
func leaseSchema() *Schema {
	return &Schema{
		Type:        "object",
		Description: "The advisory lease on this entity; absent when nobody holds it",
		Properties: map[string]*Schema{
			"user": StringSchema(), "tool": StringSchema(), "label": StringSchema(),
			"since": dateTimeSchema(), "status": StringSchema(),
		},
		Required: []string{"user", "tool", "since"},
	}
}

func dateTimeSchema() *Schema { return &Schema{Type: "string", Format: "date-time"} }

// leaseIDSchema is the lease_id a claim returns.
func leaseIDSchema() *Schema {
	return &Schema{Type: "string", Description: "The lease id the claim returned; a secret"}
}

// leaseHeldSchema is the 409 body of a claim: the problem fields, plus the
// live lease when the problem type is lease_held.
func leaseHeldSchema() *Schema {
	return &Schema{
		Description: "A problem. When the type ends in lease_held, it also names the holder and the live " +
			"lease; a lease_limit problem has the problem fields only.",
		AllOf: []*Schema{Ref("Error"), ObjectSchema(map[string]*Schema{
			"holder": ObjectSchema(map[string]*Schema{
				"user": StringSchema(), "tool": StringSchema(), "label": StringSchema(),
			}, []string{"user", "tool"}),
			"since":      dateTimeSchema(),
			"expires_at": dateTimeSchema(),
			"status":     StringSchema(),
		}, nil)},
	}
}

// addLeasePath adds the claim operation for a type. Renew, release and the
// caller's lease list are type-independent; addLeaseRoutes adds them.
func (g *Generator) addLeasePath(spec *Spec, typeName string, def metamodel.EntityDef, basePath string) {
	spec.Paths[basePath+"/{id}/_lease"] = PathItem{
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Entity ID, or ID@face", Schema: StringSchema()},
		},
		Post: &Operation{
			OperationID: "claim" + capitalize(typeName),
			Summary:     "Claim an advisory lease on a " + def.Label,
			Description: "Tells people and agents you are working on the entity. Needs read and update " +
				"access. Leases do not block writes. Keep lease_id: it is returned only here, and " +
				"POST /api/v1/_leases/renew and POST /api/v1/_leases/release need it.",
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
						"expires_at": dateTimeSchema(),
					}, []string{"lease_id", "token", "expires_at"})),
				},
				"400": {Description: "Invalid JSON, an unknown field, or invalid input", Content: problemContent()},
				"403": {Description: "Not allowed to update the entity, or no signed-in user", Content: problemContent()},
				"404": {Description: "Entity not found", Content: problemContent()},
				"409": {
					Description: "The entity is held (lease_held, naming the holder), or you hold too many " +
						"leases (lease_limit)",
					Content: map[string]MediaType{"application/problem+json": {Schema: leaseHeldSchema()}},
				},
				"413": {Description: "Request body too large", Content: problemContent()},
				"422": {Description: "A bare id of a faced type; address one face", Content: problemContent()},
				"503": {Description: "The lease store is full; retry later", Content: problemContent()},
			},
		},
	}
}

// addLeaseRoutes adds renew, release and the caller's lease list.
func (g *Generator) addLeaseRoutes(spec *Spec) {
	spec.Paths["/api/v1/_leases"] = PathItem{
		Get: &Operation{
			OperationID: "listOwnLeases",
			Summary:     "List your advisory leases",
			Description: "Your live leases on entities you may read now, oldest first. Lease ids are not returned.",
			Tags:        []string{leasesTag},
			Responses: map[string]Response{
				"200": {
					Description: "Your leases",
					Content: jsonContent(ObjectSchema(map[string]*Schema{
						"data": ArraySchema(ObjectSchema(map[string]*Schema{
							"id":         {Type: "string", Description: "Entity ID"},
							"since":      dateTimeSchema(),
							"expires_at": dateTimeSchema(),
							"status":     StringSchema(),
							"label":      StringSchema(),
						}, []string{"id", "since", "expires_at"})),
					}, []string{"data"})),
				},
				"403": {Description: "No signed-in user", Content: problemContent()},
			},
		},
	}
	spec.Paths["/api/v1/_leases/renew"] = PathItem{
		Post: &Operation{
			OperationID: "renewLease",
			Summary:     "Renew an advisory lease",
			Description: "Extends the lease and optionally replaces its status. An absent status keeps it; " +
				"an empty one clears it.",
			Tags: []string{leasesTag},
			RequestBody: &RequestBody{
				Required: true,
				Content: jsonContent(ObjectSchema(map[string]*Schema{
					"lease_id":    leaseIDSchema(),
					"ttl_seconds": {Type: "integer", Description: "Lease duration, 30 to 3600 (default 900)"},
					"status":      {Type: "string", Description: "Short note on what you are doing"},
				}, []string{"lease_id"})),
			},
			Responses: map[string]Response{
				"200": {
					Description: "Lease renewed",
					Content: jsonContent(ObjectSchema(map[string]*Schema{
						"token":      IntegerSchema(),
						"expires_at": dateTimeSchema(),
						"status":     StringSchema(),
					}, []string{"token", "expires_at"})),
				},
				"400": {Description: "Invalid JSON, an unknown field, or invalid input", Content: problemContent()},
				"403": {Description: "No signed-in user", Content: problemContent()},
				"409": {
					Description: "lease_lost: the lease expired, was released, or you lost access; claim again",
					Content:     problemContent(),
				},
				"413": {Description: "Request body too large", Content: problemContent()},
			},
		},
	}
	spec.Paths["/api/v1/_leases/release"] = PathItem{
		Post: &Operation{
			OperationID: "releaseLease",
			Summary:     "Release an advisory lease",
			Description: "Answers 204 whether or not you held the lease.",
			Tags:        []string{leasesTag},
			RequestBody: &RequestBody{
				Required: true,
				Content: jsonContent(ObjectSchema(map[string]*Schema{
					"lease_id": leaseIDSchema(),
				}, []string{"lease_id"})),
			},
			Responses: map[string]Response{
				"204": {Description: "Released, or not held"},
				"400": {Description: "Invalid JSON, an unknown field, or a malformed lease id", Content: problemContent()},
				"403": {Description: "No signed-in user", Content: problemContent()},
				"413": {Description: "Request body too large", Content: problemContent()},
			},
		},
	}
}
