package openapi

const reconcileTag = "Reconcile"

// addReconcileRoute adds POST /api/v1/_reconcile (TKT-VP02WHG).
func (g *Generator) addReconcileRoute(spec *Spec) {
	rng := ObjectSchema(map[string]*Schema{
		"kind": {Type: "string", Enum: []string{"entity", "relation"}},
		"entities": {Type: "array", Items: StringSchema(),
			Description: "Entity types in scope; also the endpoint scope of a relation range"},
		"relations": {Type: "array", Items: StringSchema(), Description: "Relation types, for a relation range"},
		"lo":        {Type: "string", Description: "Inclusive lower address bound"},
		"after":     {Type: "string", Description: "Exclusive lower bound: the last of a previous list answer"},
		"hi":        {Type: "string", Description: "Exclusive upper bound; absent means unbounded"},
		"op":        {Type: "string", Enum: []string{"fingerprint", "list"}},
		"limit":     {Type: "integer", Description: "Page size of a list, 1 to 2000"},
	}, []string{"kind", "entities", "op"})
	element := ObjectSchema(map[string]*Schema{
		"address": StringSchema(),
		"hash":    {Type: "string", Description: "View hash, 64 hex digits"},
	}, []string{"address", "hash"})
	result := ObjectSchema(map[string]*Schema{
		"status": {Type: "string", Enum: []string{"ok", "too_large", "not_run"}},
		"count":  IntegerSchema(),
		"fingerprint": {Type: "string",
			Description: "Sum of the element fingerprints modulo 2^128, 32 hex digits"},
		"elements": ArraySchema(element),
		"last":     {Type: "string", Description: "Address of the last row the page covered"},
		"more":     BooleanSchema(),
	}, []string{"status"})
	retry := map[string]Header{"Retry-After": {Description: "Seconds to wait", Schema: IntegerSchema()}}

	spec.Paths["/api/v1/_reconcile"] = PathItem{
		Post: &Operation{
			OperationID: "reconcileRanges",
			Summary:     "Fingerprint or list element ranges for rela-to-rela sync",
			Description: "Answers each range with the count and fingerprint of the elements you may read, " +
				"or with a page of them. At most 32 ranges, 256 type names, 2000 listed rows and 512 KiB " +
				"per request. A range that holds too many rows answers too_large: split it by type, then by " +
				"address. Ranges cut off by the deadline answer not_run. The answer is yours alone and is " +
				"never cached.",
			Tags: []string{reconcileTag},
			RequestBody: &RequestBody{
				Required: true,
				Content: jsonContent(ObjectSchema(map[string]*Schema{"ranges": ArraySchema(rng)},
					[]string{"ranges"})),
			},
			Responses: map[string]Response{
				"200": {
					Description: "One result per range, in request order",
					Content: jsonContent(ObjectSchema(map[string]*Schema{"ranges": ArraySchema(result)},
						[]string{"ranges"})),
				},
				"400": {Description: "Invalid JSON, an unknown or repeated type, a bad range, or a limit exceeded",
					Content: problemContent()},
				"401": {Description: "Not authenticated", Content: problemContent()},
				"413": {Description: "Request body too large", Content: problemContent()},
				"422": {Description: "A relation type whose meta the policy can hide", Content: problemContent()},
				"429": {Description: "Rate limited; see Retry-After", Headers: retry, Content: problemContent()},
				"501": {Description: "This server does not serve element ranges", Content: problemContent()},
				"503": {Description: "Too many reconcile requests are running; see Retry-After",
					Headers: retry, Content: problemContent()},
			},
		},
	}
}
